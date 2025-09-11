# Hornet

[![License](https://img.shields.io/github/license/lovromazgon/hornet)](https://github.com/lovromazgon/hornet/blob/main/LICENSE)
[![Test](https://github.com/lovromazgon/hornet/actions/workflows/test.yml/badge.svg)](https://github.com/lovromazgon/hornet/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/lovromazgon/hornet)](https://goreportcard.com/report/github.com/lovromazgon/hornet)

Hornet is a Go library that provides a simple way to build plugins in Go
applications using WebAssembly (Wasm), Wazero, and gRPC. Write your plugins in
Go, compile them to Wasm, and communicate using familiar gRPC patterns with full
type safety.

## Features

- **Pure Go**: Host and plugin can be written entirely in Go, compiled using the
  standard Go toolchain.
- **gRPC**: Use standard gRPC services and protobuf messages.
- **Type-safe**: Full compile-time type checking with generated protobuf code.
- **Easy development**: Familiar Go patterns for both host and plugin development.
- **High-performance**: Built on the [wazero](https://wazero.io/) runtime.

## Installation

Import Hornet in your Go project:

```bash
go get github.com/lovromazgon/hornet
```

## Quick Start

### 1. Define Your Service

First, define your plugin interface using protobuf and gRPC:

```protobuf
// calculator.proto
syntax = "proto3";

package calculator.v1;

service CalculatorPlugin {
  rpc Add(AddRequest) returns (AddResponse);
  rpc Multiply(MultiplyRequest) returns (MultiplyResponse);
}

message AddRequest {
  int64 a = 1;
  int64 b = 2;
}

message AddResponse {
  int64 result = 1;
}

// ... other messages
```

Generate the Go code using `protoc` or [`buf`](https://buf.build/docs/cli/):

```bash
protoc --go_out=. --go-grpc_out=. calculator.proto
```

### 2. Create the Plugin

Implementing the plugin is essentially the same as writing a gRPC server in Go.
Additionally, you need to initialize the Hornet plugin server in the `init`
function.

```go
//go:build wasm

package main

import (
    "context"
    "github.com/lovromazgon/hornet/grpc"
    "github.com/lovromazgon/hornet/wasm"
    // Your generated protobuf code
    calculatorv1 "your-project/proto/calculator/v1"
)

func main() {
    // Required by the compiler but never called
}

func init() {
    // Initialize the plugin gRPC server
    srv := grpc.NewServer()
    calculatorv1.RegisterCalculatorPluginServer(srv, &Calculator{})
    wasm.Init(srv)
}

type Calculator struct {
    calculatorv1.UnimplementedCalculatorPluginServer
}

func (c *Calculator) Add(ctx context.Context, req *calculatorv1.AddRequest) (*calculatorv1.AddResponse, error) {
    result := req.GetA() + req.GetB()
    return &calculatorv1.AddResponse{Result: result}, nil
}

func (c *Calculator) Multiply(ctx context.Context, req *calculatorv1.MultiplyRequest) (*calculatorv1.MultiplyResponse, error) {
    result := req.GetA() * req.GetB()
    return &calculatorv1.MultiplyResponse{Result: result}, nil
}
```

### 3. Build the Wasm Plugin

Build the plugin with the standard Go toolchain by targeting the `wasip1` OS and
`wasm` architecture:

```bash
GOOS=wasip1 GOARCH=wasm go build -o calculator.wasm ./plugin
```

### 4. Create the Host Application

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/lovromazgon/hornet/grpc"
    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
    calculatorv1 "your-project/proto/calculator/v1"
)

func main() {
    ctx := context.Background()
    
    // Create Wasm runtime
    r := wazero.NewRuntime(ctx)
    defer r.Close(ctx)
    
    // Set up WASI (WebAssembly System Interface)
    wasi_snapshot_preview1.MustInstantiate(ctx, r)
    
    // Load the compiled Wasm plugin
    wasmBytes, err := os.ReadFile("calculator.wasm")
    if err != nil {
        panic(err)
    }
    
    // Instantiate the plugin
    module, client, err := grpc.InstantiateModuleAndClient(
        ctx, r, wasmBytes, 
        calculatorv1.NewCalculatorPluginClient,
    )
    if err != nil {
        panic(err)
    }
    defer module.Close(ctx)
    
    // Use the plugin
    result, err := client.Add(ctx, &calculatorv1.AddRequest{A: 10, B: 32})
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("10 + 32 = %d\n", result.GetResult()) // Output: 10 + 32 = 42
}
```

## Calculator Example

See the [`examples/calculator`](./examples/calculator) directory for a complete
working example of a Hornet plugin and host application. That example also shows
how an SDK layer can be built on top of Hornet to simplify plugin implementation
for end users.

## Error Handling

Hornet propagates gRPC errors between host and plugin:

```go
// In plugin
func (c *Calculator) Divide(ctx context.Context, req *calculatorv1.DivideRequest) (*calculatorv1.DivideResponse, error) {
    if req.GetB() == 0 {
        return nil, status.Error(codes.InvalidArgument, "division by zero")
    }
    // ... rest of implementation
}

// In host
result, err := client.Divide(ctx, &calculatorv1.DivideRequest{A: 10, B: 0})
if err != nil {
    // Handle gRPC error
    if s, ok := status.FromError(err); ok {
        fmt.Printf("Error: %s (code: %s)\n", s.Message(), s.Code())
    }
}
```

## Limitations

- **No streaming**: gRPC streaming is not supported in a Wasm environment.
- **Single-threaded**: Wasm plugins run in a single-threaded context. If multiple
  concurrent calls are made, they will be serialized. If you need true concurrency,
  consider running multiple plugin instances.
- **Memory constraints**: Wasm has a 4GB memory limit (though this is rarely a
  practical concern)
- **Buffer size**: The buffer used for exchanging messages between host and plugin
  grows as needed. However, the buffer currently doesn't shrink, so if your plugin
  processes a large message once, the buffer will remain large for the lifetime
  of the plugin instance.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major
changes, please open an issue first to discuss what you would like to change.

## Acknowledgments

- Built on the [wazero](https://wazero.io/) WebAssembly runtime.
- Inspired by the [Conduit Processor SDK](https://github.com/ConduitIO/conduit-processor-sdk).
