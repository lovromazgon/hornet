//go:build wasm

package hornet

import (
	"unsafe"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// mallocBuffer is a reusable buffer for exchanging data with the Wasm host.
// It is initialized with a size of 1024 bytes and will grow as needed.
var mallocBuffer = newBuffer(1024)

// malloc gets called by the host to allocate a memory buffer of the given size
// in the Wasm plugin. It returns a pointer to the allocated buffer.
//
//go:wasmexport hornet-v1-malloc
func malloc(size uint32) uintptr {
	// Allocate a buffer of the specified size.
	mallocBuffer.Grow(int(size))
	return mallocBuffer.Pointer()
}

// command gets called by the host to execute a command in the Wasm plugin.
// It receives a pointer to a memory buffer that contains the method name and
// the request payload. The method name is of length methodSize, and the rest
// of the buffer is the request payload of length bufferSize - methodSize.
// It returns a pointer to a memory buffer that contains the response payload
// and its size in a uint64 value, where the higher 32 bits are the pointer
// and the lower 32 bits are the size.
//
//go:wasmexport hornet-v1-command
func command(ptr uintptr, methodSize, bufferSize uint32) uint64 {
	input := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), bufferSize)

	method := (input)[:methodSize]
	req := (input)[methodSize:]

	output := handler.Handle(string(method), req)
	return (*buffer)(&output).PointerAndSize()
}

// PluginHandler is the bridge between the WebAssembly exported functions and
// the Wasm plugin implementation.
type PluginHandler interface {
	// Handle gets called for every host call to the Wasm plugin.
	// It receives the method name and the request payload as byte slice and
	// returns the response payload as byte slice.
	Handle(method string, req []byte) (resp []byte)
}

// InitPlugin initializes the Wasm plugin with the provided handler.
// This MUST be called from an init() function in your Wasm plugin code
// before the plugin can handle any requests from the host.
//
// Example:
//
//	func init() {
//	    srv := hornet.NewServer()
//	    // Register your services...
//	    hornet.InitPlugin(srv)
//	}
func InitPlugin(h PluginHandler) {
	handler = h
}

var handler PluginHandler = pluginHandlerFunc(func(string, []byte) []byte {
	// Default handler that returns an unimplemented error.
	// This will be used if InitPlugin() was not called in the Wasm plugin.
	st := status.New(codes.Unimplemented, "no plugin handler set, call hornet.InitPlugin() in the Wasm plugin to set a handler")
	out, _ := proto.Marshal(st.Proto())
	return out
})

// pluginHandlerFunc wraps a function that handles a plugin request into an
// implementation of the PluginHandler interface.
type pluginHandlerFunc func(method string, req []byte) (resp []byte)

func (f pluginHandlerFunc) Handle(method string, req []byte) (resp []byte) { return f(method, req) }
