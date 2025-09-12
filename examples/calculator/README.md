# Calculator Example - Hornet Plugin SDK Pattern

This comprehensive example demonstrates how to build a complete SDK layer on top
of Hornet, showcasing best practices for creating WebAssembly plugins that can
also function as regular Go libraries.

## Run it

The makefile automates building the plugin and running it using the host:

```bash
make run
```

The host will load the WebAssembly plugin and perform various calculations,
demonstrating concurrent access and error handling.

## Overview

This example illustrates a **three-layer architecture**:

1. **SDK Layer** (`sdk/`) - Abstracts away WebAssembly and Protocol Buffer details
2. **Plugin Implementation** (`plugin/`) - The actual business logic
3. **Host Application** (`host/`) - Consumes the plugin through the clean SDK interface

```
calculator/
├── host/main.go        # Host application
├── plugin/main.go      # Plugin implementation
└── sdk/                # SDK abstraction layer
    ├── sdk.go          # Public Calculator interface
    ├── grpc.go         # gRPC client/server adapters
    └── proto/...       # Protocol Buffer definitions and generated code
```

### SDK

The SDK completely abstracts away the underlying technologies, allowing plugin
developers to focus solely on implementing business logic. Plugin developers
only need to implement a simple Go interface and register it in the `init`
function.

```go
// Plugin implementation is just a simple Go interface
type Calculator interface {
    Add(ctx context.Context, a, b int64) (int64, error)
    Sub(ctx context.Context, a, b int64) (int64, error)
    Mul(ctx context.Context, a, b int64) (int64, error)
    Div(ctx context.Context, a, b int64) (int64, error)
}
```

Note this interface is meant to be used both on the host and in the plugin. 
That's why it's recommended that all interface methods receive a `context.Context`
parameter and return an `error`, even though the business logic might not
necessarily need these arguments. The host can use these arguments for
cancellation and handling of errors that might happen in the gRPC layer.

### Error Type Preservation

The SDK demonstrates how error types can be preserved across the WebAssembly
boundary. In this example, we use the gRPC error code `InvalidArgument` to
indicate a division by zero attempt. A more sophisticated approach could define
a structured error message and use that to reconstruct the error type on the host.

```go
// 1. The plugin implementation returns an SDK error type.
func (c *Calculator) Div(ctx context.Context, a, b int64) (int64, error) {
    if b == 0 {
        return 0, sdk.ErrDivisionByZero  // SDK-defined error type
    }
    // ...
}

// 2. The SDK translates the error to/from a gRPC status error
func (c *calculatorServer) Div(ctx context.Context, req *calculatorv1.DivRequest) (*calculatorv1.DivResponse, error) {
    out, err := c.impl.Div(ctx, req.GetA(), req.GetB())
    if err != nil {
        if errors.Is(err, ErrDivisionByZero) {
            // Return a gRPC InvalidArgument error if division by zero is attempted.
            // The client can then check for this specific code.
            return nil, status.Error(codes.InvalidArgument, err.Error())
        }
        return nil, err
    }
    return &calculatorv1.DivResponse{C: out}, nil
}

// ...

func (c *calculatorClient) Div(ctx context.Context, a, b int64) (int64, error) {
    out, err := c.client.Div(ctx, &calculatorv1.DivRequest{A: a, B: b})
    if err != nil {
        // Check if the error is a gRPC error with code InvalidArgument,
        // which indicates a division by zero attempt.
        st, ok := status.FromError(err)
        if ok && st.Code() == codes.InvalidArgument {
            return 0, ErrDivisionByZero
        }
        return 0, err
    }
    return out.GetC(), nil
}

// 3. The host can use errors.Is() for type checking.
c, err := calc.Div(ctx, a, b)
if errors.Is(err, sdk.ErrDivisionByZero) {
    fmt.Printf("Division by zero: %v\n", err)
}
```

This pattern ensures that plugin developers and host applications can work with
strongly-typed errors without needing to understand the underlying gRPC error
mechanism.

### Concurrent Access

The host demonstrates concurrent plugin calls:

```go
var wg sync.WaitGroup
// Simulate concurrent access from 10 goroutines
for range 10 {
    wg.Add(1)
    go func() {
        defer wg.Done()
        add(ctx, calc)
        sub(ctx, calc)
        mul(ctx, calc)
        div(ctx, calc)
    }()
}
wg.Wait()
```

**Important**: While the host can make concurrent calls to the plugin,
**WebAssembly does not support concurrent access to memory**. Hornet
automatically **serializes all calls** to the WebAssembly module under the hood.

If true parallelism is required, the host can create multiple plugin instances
and distribute calls among them.

### Native Go Integration

Using the approach with hiding implementation details behind a SDK layer allows
you to use the same plugin either as a Wasm binary or import it as a native Go
library (e.g. for testing/development or for providing built-in plugins):

```go
package main

import (
    "github.com/yourorg/yourplugin/sdk"
    "github.com/yourorg/yourplugin/plugin" // Import plugin directly
)

func main() {
    calc := &plugin.Calculator{} // Use directly without WebAssembly
    result, err := calc.Add(ctx, 10, 32)
    // ...
}
```

This example demonstrates the power and flexibility of the Hornet library
combined with a well-designed SDK layer, enabling both powerful plugin
capabilities and excellent developer experience.

## Creating Your Own SDK

To create a similar SDK for your use case:

1. **Define the interface** (`sdk.go`):
   ```go
   type YourPlugin interface {
       DoSomething(ctx context.Context, input string) (string, error)
   }
   ```

2. **Create a matching protocol buffer definition** (`proto/yourservice.proto`):
   ```protobuf
   service YourServicePlugin {
     rpc DoSomething(DoSomethingRequest) returns (DoSomethingResponse);
   }
   ```

3. **Implement adapters** (`grpc.go`):
   - Client adapter: converts interface calls to gRPC calls
   - Server adapter: converts gRPC calls to interface calls
   - Error translation: map errors to/from gRPC status errors

4. **Provide registration helpers**:
   ```go
   func RegisterYourPlugin(srv grpc.ServiceRegistrar, impl YourPlugin)
   func NewYourPluginFromClient(client YourServicePluginClient) YourPlugin
   ```
