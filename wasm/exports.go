//go:build wasm

package wasm

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
