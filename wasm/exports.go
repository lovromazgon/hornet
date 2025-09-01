//go:build wasm

package wasm

import (
	"bytes"
	"unsafe"
)

var mallocBuffer = newBuffer(1024) // 1kB buffer for malloc

//go:wasmexport hornet-v1-malloc
func malloc(size uint32) unsafe.Pointer {
	// Allocate a buffer of the specified size.
	mallocBuffer.Grow(int(size))
	return mallocBuffer.Pointer()
}

//go:wasmexport hornet-v1-command
func command(ptr unsafe.Pointer, size uint32) uint64 {
	raw := unsafe.Slice((*byte)(ptr), size)
	i := bytes.IndexByte(raw, '\u0000')
	if i == -1 {
		panic("invalid input to hornet-v1-command - expected NUL character separating method and data")
	}
	out := handler.Handle(string(raw[:i]), raw[i+1:])
	return (*buffer)(&out).PointerAndSize()
}
