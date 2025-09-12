//go:build wasm

package hornet

import (
	"unsafe"
)

var mallocBuffer = newBuffer(1024) // 1kB buffer for malloc

//go:wasmexport hornet-v1-malloc
func malloc(size uint32) uintptr {
	// Allocate a buffer of the specified size.
	mallocBuffer.Grow(int(size))
	return mallocBuffer.Pointer()
}

//go:wasmexport hornet-v1-command
func command(ptr uintptr, methodSize, bufferSize uint32) uint64 {
	input := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), bufferSize)

	method := (input)[:methodSize]
	req := (input)[methodSize:]

	output := handler.Handle(string(method), req)
	return (*buffer)(&output).PointerAndSize()
}

var handler Handler = HandlerFunc(func(string, []byte) []byte {
	return []byte("no handler set, call hornet.Init() in the plugin code to set a handler")
})

// Handler is the bridge between the WebAssembly exports and the Wasm plugin.
type Handler interface {
	// Handle gets called for every host call to the Wasm plugin.
	Handle(method string, req []byte) (resp []byte)
}

// HandlerFunc is a function type that implements the Handler interface.
// It defines a function that takes a byte slice as input and returns a
// byte slice as output.
type HandlerFunc func(method string, req []byte) (resp []byte)

func (f HandlerFunc) Handle(method string, req []byte) (resp []byte) { return f(method, req) }

// Init needs to be called in an init function in the wasm plugin to initialize
// the wasm call handler.
func Init(h Handler) {
	// TODO check magic cookie env var
	// TODO check that the handler wasn't yet initialized
	handler = h
}
