//go:build wasm

package wasm

import (
	"unsafe"
)

var mallocBuffer = newBuffer(1024) // 1kB buffer for malloc

//go:wasmexport hornet-malloc
func malloc(size uint32) unsafe.Pointer {
	// Allocate a buffer of the specified size.
	mallocBuffer.Grow(int(size))
	return mallocBuffer.Pointer()
}

//go:wasmexport hornet-command
func command(method string, ptr unsafe.Pointer, size uint32) uint64 {
	in := unsafe.Slice((*byte)(ptr), size)
	out := handler.Handle(method, in)
	return (*buffer)(&out).PointerAndSize()
}
