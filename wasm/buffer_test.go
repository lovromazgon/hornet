package wasm

import (
	"testing"

	"github.com/matryer/is"
)

func TestBuffer_NewBuffer(t *testing.T) {
	t.Run("should create a buffer with a specific size", func(t *testing.T) {
		is := is.New(t)
		size := 128
		b := newBuffer(size)

		is.Equal(len(*b), size)
		is.True(cap(*b) >= size)
	})

	t.Run("should create an empty buffer for size 0", func(t *testing.T) {
		is := is.New(t)
		b := newBuffer(0)
		is.True(b != nil)    // The buffer itself is not nil
		is.Equal(len(*b), 0) // The underlying slice has zero length
		is.Equal(cap(*b), 0) // and zero capacity
	})
}

func TestBuffer_Grow(t *testing.T) {
	t.Run("should grow by re-allocating and preserve data", func(t *testing.T) {
		is := is.New(t)
		b := newBuffer(10)
		// Put some data in the buffer
		copy(*b, "0123456789")

		originalCap := cap(*b)
		originalPtr := b.Pointer()

		newSize := originalCap + 10
		reallocated := b.Grow(newSize)

		is.True(reallocated) // Should have reallocated
		is.Equal(len(*b), newSize)
		is.True(cap(*b) >= newSize)
		is.True(b.Pointer() != originalPtr)       // Pointer should change after reallocation
		is.Equal(string((*b)[:10]), "0123456789") // Old data must be preserved
	})

	t.Run("should grow by re-slicing when capacity is sufficient", func(t *testing.T) {
		is := is.New(t)
		// Create a buffer with more capacity than length
		b := buffer(make([]byte, 5, 20))
		copy(b, "hello")

		originalCap := cap(b)
		originalPtr := b.Pointer()

		newSize := 15
		reallocated := b.Grow(newSize)

		is.True(!reallocated) // Should NOT have reallocated
		is.Equal(len(b), newSize)
		is.Equal(cap(b), originalCap)      // Capacity should not change
		is.Equal(b.Pointer(), originalPtr) // Pointer should not change
		is.Equal(string(b[:5]), "hello")   // Old data must be preserved
	})
}

func TestBuffer_PointerAndSize(t *testing.T) {
	t.Run("should correctly encode pointer and size", func(t *testing.T) {
		is := is.New(t)
		// Start with a non-zero buffer per assumptions
		b := newBuffer(256)

		packed := b.PointerAndSize()
		is.True(packed != 0)

		// Decode the values
		decodedPtrVal := uint32(packed >> 32)
		decodedSize := uint32(packed)

		// This test simulates a 32-bit wasm architecture. On a 64-bit test host,
		// we must compare the decoded 32-bit pointer value with the truncated
		// original 64-bit pointer value.
		is.Equal(decodedPtrVal, uint32(b.Pointer()))
		is.Equal(int(decodedSize), len(*b))
		is.Equal(int(decodedSize), 256)
	})

	t.Run("should return 0 for an empty buffer", func(t *testing.T) {
		is := is.New(t)
		// Even though the assumption is non-zero, the methods should be robust.
		b := newBuffer(0)
		packed := b.PointerAndSize()
		is.Equal(packed, uint64(0)) // Pointer is nil (0) and size is 0
	})
}
