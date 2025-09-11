//go:build wasm

package wasm

import (
	"unsafe"

	"github.com/lovromazgon/hornet/buffer"
)

var allocations = make(map[uintptr][]byte)

//go:wasmexport hornet-v1-malloc
func malloc(ptr uintptr, size uint32) uintptr {
	b, ok := allocations[ptr]
	if !ok {
		// New allocation
		b = make([]byte, size)
		ptr = uintptr(unsafe.Pointer(&b[0]))
		allocations[ptr] = b
	}

	buf := (*buffer.Buffer)(&b)
	ptrChanged := buf.Grow(int(size))
	if ptrChanged {
		delete(allocations, ptr)
		ptr = buf.Pointer()
		allocations[ptr] = *buf
	}

	return buf.Pointer()
}

//go:wasmexport hornet-v1-command
func command(ptr uintptr, methodSize, bufferSize uint32) uint64 {
	input := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), bufferSize)

	method := (input)[:methodSize]
	req := (input)[methodSize:]

	output := handler.Handle(string(method), req)
	return (*buffer.Buffer)(&output).PointerAndSize()
}
