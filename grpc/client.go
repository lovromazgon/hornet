package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type clientOptions struct {
	logger *slog.Logger
}

var defaultClientOptions = clientOptions{
	logger: slog.Default(),
}

var _ grpc.ClientConnInterface = &ClientConn{}

type ClientConn struct {
	opts   clientOptions
	module api.Module

	mallocFn  api.Function
	commandFn api.Function

	m sync.Mutex
	// buf is the buffer used to communicate with the Wasm module.
	buf []byte
	// lastMemorySize is the size of the memory when we last allocated the
	// buffer in the Wasm module. This is used to detect if the memory has grown,
	// and the pointer might be invalidated.
	lastMemorySize uint32
	// modulePointer is the pointer to the buffer in the Wasm module. It is used
	// to write data to the Wasm module.
	modulePointer uint32
}

func InstantiateModuleAndClient[T any](
	ctx context.Context,
	runtime wazero.Runtime,
	source []byte,
	newClient func(grpc.ClientConnInterface) T,
	opt ...ClientOption,
) (api.Module, T, error) {
	var zeroT T

	// Configure the module to initialize the reactor.
	config := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithStartFunctions("_initialize")

	// Instantiate the module.
	wasmModule, err := runtime.InstantiateWithConfig(ctx, source, config)
	if err != nil {
		return nil, zeroT, fmt.Errorf("failed to instantiate Wasm module: %w", err)
	}

	// Instantiate client.
	client, err := NewClient(wasmModule, opt...)
	if err != nil {
		_ = wasmModule.Close(ctx)
		return nil, zeroT, fmt.Errorf("failed to instantiate grpc client: %w", err)
	}

	return wasmModule, newClient(client), nil
}

func NewClient(module api.Module, opt ...ClientOption) (*ClientConn, error) {
	opts := defaultClientOptions
	for _, o := range opt {
		o.applyClient(&opts)
	}

	mallocFn, err := getExportedFunction(module, mallocFunctionDefinition)
	if err != nil {
		return nil, err
	}

	commandFn, err := getExportedFunction(module, commandFunctionDefinition)
	if err != nil {
		return nil, err
	}

	return &ClientConn{
		opts:   opts,
		module: module,

		mallocFn:  mallocFn,
		commandFn: commandFn,
	}, nil
}

func (c *ClientConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, errors.New("streams are not supported by Wasm")
}

func (c *ClientConn) Invoke(
	ctx context.Context,
	method string,
	req, resp any,
	opts ...grpc.CallOption,
) error {
	reqMsg, ok := req.(proto.Message)
	if !ok {
		return fmt.Errorf("invalid request type: expected proto.Message, got %T", req)
	}

	respMsg, ok := resp.(proto.Message)
	if !ok {
		return fmt.Errorf("invalid response type: expected proto.Message, got %T", req)
	}

	// TODO respect options if applicable
	return c.executeCall(ctx, method, reqMsg, respMsg)
}

// executeCall executes a function call to the Wasm module, ensuring it allocates
// enough memory and collects the response. It returns the response, or an error
// if the response is an error.
func (c *ClientConn) executeCall(
	ctx context.Context,
	method string,
	req proto.Message,
	resp proto.Message,
) error {
	c.m.Lock()
	defer c.m.Unlock()

	if c.module.IsClosed() {
		return errors.New("module is closed")
	}

	logger := c.opts.logger.With("method", method)

	// Step 1: Allocate memory in the Wasm module if needed.
	if msgSize := proto.Size(req) + len(method) + 1; cap(c.buf) < msgSize ||
		c.lastMemorySize != c.module.Memory().Size() {
		logger.DebugContext(ctx, "memory buffer is too small or memory size has changed, reallocating using malloc function")

		results, err := c.mallocFn.Call(ctx, api.EncodeI32(int32(msgSize)))
		if err != nil {
			logger.ErrorContext(ctx, "failed to call Wasm function", "function", c.mallocFn.Definition().Name(), "error", err)
			return fmt.Errorf("failed to call Wasm function %q: %w", c.mallocFn.Definition().Name(), err)
		}

		c.modulePointer = api.DecodeU32(results[0])
		c.lastMemorySize = c.module.Memory().Size()

		if cap(c.buf) < msgSize {
			c.buf = make([]byte, msgSize)
		}
	}

	// Step 2: Marshal the method into the buffer + separator (NUL).
	c.buf = c.buf[:len(method)+1]
	copy(c.buf, method)
	c.buf[len(method)] = '\u0000'

	// Step 3: Marshal the request into the buffer.
	reqBytes, err := proto.MarshalOptions{}.MarshalAppend(c.buf, req)
	if err != nil {
		logger.ErrorContext(ctx, "failed marshalling protobuf command request", "error", err)
		return fmt.Errorf("failed to marshal protobuf command request: %w", err)
	}

	// Step 4: Write the request to the Wasm module's memory.
	if !c.module.Memory().Write(c.modulePointer, reqBytes) {
		logger.ErrorContext(ctx, "failed to write to Wasm module memory", "ptr", c.modulePointer, "size", len(reqBytes))
		return fmt.Errorf("failed to write to Wasm module memory at pointer %d with size %d", c.modulePointer, len(reqBytes))
	}

	// Step 5: Call the Wasm function with the pointer and size of the buffer.
	results, err := c.commandFn.Call(
		ctx,
		api.EncodeU32(c.modulePointer),
		api.EncodeU32(uint32(len(reqBytes))),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to call Wasm function", "function", c.commandFn.Definition().Name(), "error", err)
		return fmt.Errorf("failed to call Wasm function %q: %w", c.commandFn.Definition().Name(), err)
	}

	// Step 6: Check the results of the function call.
	ptrSize := results[0]
	ptr := uint32(ptrSize >> 32)
	size := uint32(ptrSize)

	// Read the byte slice from the module's memory.
	respBytes, ok := c.module.Memory().Read(ptr, size)
	if !ok {
		logger.ErrorContext(ctx, "failed to read from Wasm module memory", "ptr", ptr, "size", size)
		return fmt.Errorf("failed to read from Wasm module memory at pointer %d with size %d", ptr, size)
	}

	if err := proto.Unmarshal(respBytes, resp); err != nil {
		logger.ErrorContext(ctx, "failed to unmarshal protobuf command response", "error", err)
		return fmt.Errorf("failed to unmarshal protobuf command response: %w", err)
	}

	return nil
}
