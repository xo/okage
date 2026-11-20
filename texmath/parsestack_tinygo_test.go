//go:build tinygo && wasm

package texmath

import (
	"testing"
	"unsafe"
)

// This file measures the stack that Parse takes on TinyGo for WebAssembly,
// where a goroutine has a stack of 64 KiB that does not grow.

// stackPointer returns the stack pointer of TinyGo on WebAssembly, which is
// in the linear memory and grows down. The runtime defines it in assembly.
//
//export tinygo_getCurrentStackPointer
func stackPointer() uintptr

// stackSpan is the part of the stack below the caller that stackUse can
// measure. The goroutine of a test has used less than 8 KiB of its 64 KiB
// when stackUse runs, so the span stays inside the stack.
const stackSpan = 56 << 10

// parseStackBudget is the most stack that Parse can take. It leaves more than
// half of the 64 KiB to the caller of Parse and to the garbage collector.
const parseStackBudget = 32 << 10

// stackUse fills the free part of the stack below its frame with a pattern,
// calls f, and returns how far down f and its callees wrote, in bytes.
//
//go:noinline
func stackUse(f func()) int {
	top := stackPointer()
	lo := top - stackSpan
	for a := lo; a < top-1024; a++ {
		*(*byte)(unsafe.Pointer(a)) = 0xa5
	}
	f()
	for a := lo; a < top-1024; a++ {
		if *(*byte)(unsafe.Pointer(a)) != 0xa5 {
			return int(top - a)
		}
	}
	return 0
}

// TestParseStackUse measures the stack of each shape of deepShapes at each
// depth up to past the limit, with and without its end. The most stack of
// all must stay within parseStackBudget.
func TestParseStackUse(t *testing.T) {
	most := 0
	for _, s := range deepShapes {
		shapeMost, at := 0, 0
		for n := 1; n <= 110; n++ {
			closed, open := s.nested(n)
			for _, tex := range []string{closed, open} {
				if u := stackUse(func() { _, _ = Parse(tex) }); u > shapeMost {
					shapeMost, at = u, n
				}
			}
		}
		t.Logf("%-22q %6d bytes at %d levels", s.open, shapeMost, at)
		most = max(most, shapeMost)
	}
	t.Logf("Parse takes at most %d bytes of the stack", most)
	if most > parseStackBudget {
		t.Errorf("Parse takes %d bytes of the stack, more than %d", most, parseStackBudget)
	}
}
