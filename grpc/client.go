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
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
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

	// m guards calls to the Wasm module.
	m         sync.Mutex
	mallocFn  api.Function
	commandFn api.Function
	// buf is the buffer used to communicate with the Wasm module.
	buf []byte
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
		return nil, fmt.Errorf("failed to get malloc function: %w", err)
	}

	commandFn, err := getExportedFunction(module, commandFunctionDefinition)
	if err != nil {
		return nil, fmt.Errorf("failed to get command function: %w", err)
	}

	c := &ClientConn{
		opts:      opts,
		module:    module,
		mallocFn:  mallocFn,
		commandFn: commandFn,
	}

	return c, nil
}

//nolint:lll // This method is just a stub to satisfy the grpc.ClientConnInterface interface.
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

	return c.invoke(ctx, method, reqMsg, respMsg, opts...)
}

func (c *ClientConn) invoke(
	ctx context.Context,
	method string,
	req proto.Message,
	resp proto.Message,
	_ ...grpc.CallOption, // Options are currently ignored.
) error {
	c.m.Lock()
	defer c.m.Unlock()

	logger := c.opts.logger.With("method", method)

	// Step 1: Allocate memory in the Wasm module if needed.
	if msgSize := proto.Size(req) + len(method); cap(c.buf) < msgSize {
		logger.DebugContext(ctx, "memory buffer is too small, reallocating using malloc function")

		err := c.invokeMalloc(ctx, msgSize)
		if err != nil {
			return err
		}
	}

	// Step 2: Write request to the buffer in the Wasm module.
	err := c.writeRequestToModule(method, req)
	if err != nil {
		return err
	}

	// Step 3: Call the Wasm command function.
	err = c.invokeCommand(ctx, method, resp)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientConn) invokeMalloc(ctx context.Context, msgSize int) error {
	results, err := c.mallocFn.Call(
		ctx,
		api.EncodeU32(uint32(msgSize)), //nolint:gosec // no risk of overflow
	)
	if err != nil {
		return fmt.Errorf("failed to call Wasm function %q: %w", c.mallocFn.Definition().Name(), err)
	}

	c.modulePointer = api.DecodeU32(results[0])

	if cap(c.buf) < msgSize {
		c.buf = make([]byte, msgSize)
	}

	return nil
}

func (c *ClientConn) writeRequestToModule(method string, req proto.Message) error {
	c.buf = c.buf[:len(method)]
	copy(c.buf, method)

	reqBytes, err := proto.MarshalOptions{}.MarshalAppend(c.buf, req)
	if err != nil {
		return fmt.Errorf("failed to marshal protobuf command request: %w", err)
	}

	c.buf = reqBytes

	if !c.module.Memory().Write(c.modulePointer, c.buf) {
		return fmt.Errorf("failed to write to Wasm module memory at pointer %d with size %d", c.modulePointer, len(c.buf))
	}

	return nil
}

func (c *ClientConn) invokeCommand(ctx context.Context, method string, resp proto.Message) error {
	results, err := c.commandFn.Call(
		ctx,
		api.EncodeU32(c.modulePointer),
		api.EncodeU32(uint32(len(method))), //nolint:gosec // no risk of overflow
		api.EncodeU32(uint32(len(c.buf))),  //nolint:gosec // no risk of overflow
	)
	if err != nil {
		return fmt.Errorf("failed to call Wasm function %q: %w", c.commandFn.Definition().Name(), err)
	}

	// Check the results of the function call.
	ptrSize := results[0]
	ptr := uint32(ptrSize >> 32) //nolint:gosec // higher 32 bits
	size := uint32(ptrSize)      //nolint:gosec // lower 32 bits

	// Read the byte slice from the module's memory.
	respBytes, ok := c.module.Memory().Read(ptr, size)
	if !ok {
		return fmt.Errorf("failed to read from Wasm module memory at pointer %d with size %d", ptr, size)
	}

	if len(respBytes) == 0 {
		return fmt.Errorf("received empty response from Wasm module")
	}

	if respBytes[0] == 1 {
		// Error response.
		var st spb.Status
		if err := proto.Unmarshal(respBytes[1:], &st); err != nil {
			return fmt.Errorf("failed to unmarshal protobuf error response: %w", err)
		}
		return status.ErrorProto(&st)
	}

	if err := proto.Unmarshal(respBytes[1:], resp); err != nil {
		return fmt.Errorf("failed to unmarshal protobuf command response: %w", err)
	}

	return nil
}

func (c *ClientConn) Close(ctx context.Context) error {
	err := c.module.Close(ctx)
	if err != nil {
		return fmt.Errorf("failed to close module: %w", err)
	}

	return nil
}
