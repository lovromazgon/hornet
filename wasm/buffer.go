package wasm

import (
	"unsafe"
)

// buffer is a utility struct that holds a byte slice and an unsafe pointer to
// the start of the slice's data. It is used to efficiently pass data between
// the host and the WebAssembly module without unnecessary copying.
type buffer []byte

func newBuffer(size int) *buffer {
	b := make(buffer, size)
	return &b
}

func (b *buffer) Grow(size int) {
	if cap(*b) < size {
		// This append logic preserves existing data when growing.
		// We append to the end of the slice after expanding it to the full capacity.
		*b = append((*b)[:cap(*b)], make([]byte, size-cap(*b))...)
	}
	// Reslice to the new size if we had enough capacity.
	// This does not shrink the buffer if size < len(*b).
	if len(*b) < size {
		*b = (*b)[:size]
	}
}

// Pointer returns a pointer to the buffer's data.
// Includes a safety check for zero-length slices to prevent panics.
func (b *buffer) Pointer() unsafe.Pointer {
	if len(*b) == 0 {
		return nil
	}
	return unsafe.Pointer(&(*b)[0])
}

// PointerAndSize returns the pointer and size in a single uint64.
// The higher 32 bits are the pointer, and the lower 32 bits are the size.
func (b *buffer) PointerAndSize() uint64 {
	return (uint64(uintptr(b.Pointer())) << 32) | uint64(len(*b))
}
