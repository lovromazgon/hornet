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
	logger                *slog.Logger
	maxConcurrentRequests int
}

var defaultClientOptions = clientOptions{
	logger:                slog.Default(),
	maxConcurrentRequests: 2,
}

var _ grpc.ClientConnInterface = &ClientConn{}

type ClientConn struct {
	opts   clientOptions
	module api.Module

	invoker invoker
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

	ivk, err := newAsyncInvoker(module, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create async invoker: %w", err)
	}

	c := &ClientConn{
		opts:    opts,
		module:  module,
		invoker: ivk,
	}

	return c, nil
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

	if c.module.IsClosed() {
		return errors.New("module is closed")
	}

	return c.invoker.invoke(ctx, method, reqMsg, respMsg, opts...)
}

type invoker interface {
	invoke(ctx context.Context, method string, req proto.Message, resp proto.Message, opts ...grpc.CallOption) error
}

type syncInvoker struct {
	m sync.Mutex

	module api.Module
	opts   clientOptions

	mallocFn  api.Function
	commandFn api.Function

	// buf is the buffer used to communicate with the Wasm module.
	buf []byte
	// modulePointer is the pointer to the buffer in the Wasm module. It is used
	// to write data to the Wasm module.
	modulePointer uint32
}

func (i *syncInvoker) invoke(ctx context.Context, method string, req proto.Message, resp proto.Message, opts ...grpc.CallOption) error {
	// TODO respect options if applicable

	i.m.Lock()
	defer i.m.Unlock()

	logger := i.opts.logger.With("method", method)

	// Step 1: Allocate memory in the Wasm module if needed.
	if msgSize := proto.Size(req) + len(method) + 1; cap(i.buf) < msgSize {
		logger.DebugContext(ctx, "memory buffer is too small, reallocating using malloc function")

		results, err := i.mallocFn.Call(ctx, api.EncodeI32(int32(msgSize)))
		if err != nil {
			logger.ErrorContext(ctx, "failed to call Wasm function", "function", i.mallocFn.Definition().Name(), "error", err)
			return fmt.Errorf("failed to call Wasm function %q: %w", i.mallocFn.Definition().Name(), err)
		}

		i.modulePointer = api.DecodeU32(results[0])

		if cap(i.buf) < msgSize {
			i.buf = make([]byte, msgSize)
		}
	}

	// Step 2: Marshal the method into the buffer.
	i.buf = i.buf[:len(method)]
	copy(i.buf, method)

	// Step 3: Marshal the request into the buffer.
	reqBytes, err := proto.MarshalOptions{}.MarshalAppend(i.buf, req)
	if err != nil {
		logger.ErrorContext(ctx, "failed marshalling protobuf command request", "error", err)
		return fmt.Errorf("failed to marshal protobuf command request: %w", err)
	}

	// Step 4: Write the request to the Wasm module's memory.
	if !i.module.Memory().Write(i.modulePointer, reqBytes) {
		logger.ErrorContext(ctx, "failed to write to Wasm module memory", "ptr", i.modulePointer, "size", len(reqBytes))
		return fmt.Errorf("failed to write to Wasm module memory at pointer %d with size %d", i.modulePointer, len(reqBytes))
	}

	// Step 5: Call the Wasm function with the pointer and size of the buffer.
	results, err := i.commandFn.Call(
		ctx,
		api.EncodeU32(i.modulePointer),
		api.EncodeU32(uint32(len(method))),
		api.EncodeU32(uint32(len(reqBytes))),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to call Wasm function", "function", i.commandFn.Definition().Name(), "error", err)
		return fmt.Errorf("failed to call Wasm function %q: %w", i.commandFn.Definition().Name(), err)
	}

	// Step 6: Check the results of the function call.
	ptrSize := results[0]
	ptr := uint32(ptrSize >> 32)
	size := uint32(ptrSize)

	// Read the byte slice from the module's memory.
	respBytes, ok := i.module.Memory().Read(ptr, size)
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

type asyncInvoker struct {
	opts   clientOptions
	module api.Module

	// workers is a channel of available worker contexts.
	// The size of the channel is the maximum number of concurrent requests.
	workers chan *asyncInvokerContext

	// m protects access to mallocFn.
	m        sync.Mutex
	mallocFn api.Function
}

type asyncInvokerContext struct {
	id        int
	commandFn api.Function

	// buf is the buffer used to communicate with the Wasm module.
	buf []byte
	// modulePointer is the pointer to the buffer in the Wasm module. It is used
	// to write data to the Wasm module.
	modulePointer uint32
}

func newAsyncInvoker(module api.Module, opts clientOptions) (*asyncInvoker, error) {
	mallocFn, err := getExportedFunction(module, mallocFunctionDefinition)
	if err != nil {
		return nil, fmt.Errorf("failed to get malloc function: %w", err)
	}

	workers := make(chan *asyncInvokerContext, opts.maxConcurrentRequests)
	for i := range opts.maxConcurrentRequests {
		workCtx, err := newAsyncInvokerContext(i, module)
		if err != nil {
			return nil, fmt.Errorf("failed to create async invoker context: %w", err)
		}
		workers <- workCtx
	}

	return &asyncInvoker{
		opts:     opts,
		module:   module,
		workers:  workers,
		mallocFn: mallocFn,
	}, nil
}

func newAsyncInvokerContext(id int, module api.Module) (*asyncInvokerContext, error) {
	commandFn, err := getExportedFunction(module, commandFunctionDefinition)
	if err != nil {
		return nil, fmt.Errorf("failed to get command function: %w", err)
	}

	return &asyncInvokerContext{
		id:        id,
		commandFn: commandFn,
	}, nil
}

func (i *asyncInvoker) invoke(ctx context.Context, method string, req proto.Message, resp proto.Message, opts ...grpc.CallOption) error {
	// TODO respect options if applicable

	workCtx := <-i.workers
	defer func() { i.workers <- workCtx }()

	logger := i.opts.logger.With("method", method)

	// Step 1: Allocate memory in the Wasm module if needed.
	if msgSize := proto.Size(req) + len(method) + 1; cap(workCtx.buf) < msgSize {
		logger.DebugContext(ctx, "memory buffer is too small, reallocating using malloc function")
		if err := i.malloc(ctx, workCtx, msgSize); err != nil {
			return fmt.Errorf("failed to allocate memory in Wasm module: %w", err)
		}
	}

	// Step 2: Marshal the method into the buffer.
	workCtx.buf = workCtx.buf[:len(method)]
	copy(workCtx.buf, method)

	// Step 3: Marshal the request into the buffer.
	reqBytes, err := proto.MarshalOptions{}.MarshalAppend(workCtx.buf, req)
	if err != nil {
		logger.ErrorContext(ctx, "failed marshalling protobuf command request", "error", err)
		return fmt.Errorf("failed to marshal protobuf command request: %w", err)
	}

	// Step 4: Write the request to the Wasm module's memory.
	if !i.module.Memory().Write(workCtx.modulePointer, reqBytes) {
		logger.ErrorContext(ctx, "failed to write to Wasm module memory", "ptr", workCtx.modulePointer, "size", len(reqBytes))
		return fmt.Errorf("failed to write to Wasm module memory at pointer %d with size %d", workCtx.modulePointer, len(reqBytes))
	}

	// Step 5: Call the Wasm function with the pointer and size of the buffer.
	results, err := workCtx.commandFn.Call(
		ctx,
		api.EncodeU32(workCtx.modulePointer),
		api.EncodeU32(uint32(len(method))),
		api.EncodeU32(uint32(len(reqBytes))),
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to call Wasm function", "function", workCtx.commandFn.Definition().Name(), "error", err)
		return fmt.Errorf("failed to call Wasm function %q: %w", workCtx.commandFn.Definition().Name(), err)
	}

	// Step 6: Check the results of the function call.
	ptrSize := results[0]
	ptr := uint32(ptrSize >> 32)
	size := uint32(ptrSize)

	// Read the byte slice from the module's memory.
	respBytes, ok := i.module.Memory().Read(ptr, size)
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

func (i *asyncInvoker) malloc(ctx context.Context, workCtx *asyncInvokerContext, msgSize int) error {
	i.m.Lock()
	defer i.m.Unlock()

	results, err := i.mallocFn.Call(
		ctx,
		api.EncodeU32(workCtx.modulePointer),
		api.EncodeI32(int32(msgSize)),
	)
	if err != nil {
		return fmt.Errorf("failed to call Wasm function %q: %w", i.mallocFn.Definition().Name(), err)
	}

	workCtx.modulePointer = api.DecodeU32(results[0])

	if cap(workCtx.buf) < msgSize {
		workCtx.buf = make([]byte, msgSize)
	}
	return nil
}
