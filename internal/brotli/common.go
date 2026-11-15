// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package brotli implements the Brotli compressed data format,
// described in RFC 7932.
package brotli

import (
	"fmt"

	"github.com/xo/okage"
)

// This package is a copy of the decoder of the package brotli of
// github.com/dsnet/compress at the commit 39efe44ab707, with the changes of
// D45: it has no encoder, it holds the helpers that it took from the packages
// internal and internal/errors of that module, it reads the dictionary from
// dictionary.bin, and its errors wrap okage.ErrFontData.

// errorf returns an error that wraps okage.ErrFontData, with the text of f
// and a.
func errorf(f string, a ...any) error {
	return fmt.Errorf("%w: brotli: "+f, append([]any{okage.ErrFontData}, a...)...)
}

var (
	errCorrupted = errorf("corrupted input")
	errInvalid   = errorf("invalid argument")
	errUnaligned = errorf("non-aligned bit buffer")
)

// thrown holds an error that throw raises, so that recoverError recovers
// only the errors of this package. It replaces errors.Panic and
// errors.Recover of github.com/dsnet/compress/internal/errors, which this
// package used to report an error from a tight loop without a check of
// "err != nil" after each call.
type thrown struct{ err error }

// throw raises err, which the deferred recoverError of Reader.Read returns.
func throw(err error) {
	panic(thrown{err})
}

// recoverError recovers an error that throw raised, and stores it in err. It
// raises any other panic again.
func recoverError(err *error) {
	switch ex := recover().(type) {
	case nil:
	case thrown:
		*err = ex.err
	default:
		panic(ex)
	}
}

// moveToFront is the decoder of MoveToFront of
// github.com/dsnet/compress/internal. It assumes that the alphabet is densely
// packed within 0..255.
type moveToFront struct {
	dict [256]uint8 // Mapping from indexes to values
	tail int        // Number of tail bytes that are already ordered
}

// Decode replaces each index of idxs with its value, and moves the value to
// the front.
func (m *moveToFront) Decode(idxs []uint8) {
	for i := range m.dict[:256-m.tail] {
		m.dict[i] = uint8(i) // Reset dict to be identity
	}

	var maxIdx int
	for i, idx := range idxs {
		val := m.dict[idx] // Forward lookup val in dict
		idxs[i] = val

		maxIdx |= int(idx)
		copy(m.dict[1:], m.dict[:idx])
		m.dict[0] = val
	}
	m.tail = 256 - maxIdx - 1
}

var (
	reverseLUT [256]uint8
)

func init() {
	initLUTs()
}

func initLUTs() {
	initCommonLUTs()
	initPrefixLUTs()
	initContextLUTs()
	initDictLUTs()
}

func initCommonLUTs() {
	for i := range reverseLUT {
		b := uint8(i)
		b = (b&0xaa)>>1 | (b&0x55)<<1
		b = (b&0xcc)>>2 | (b&0x33)<<2
		b = (b&0xf0)>>4 | (b&0x0f)<<4
		reverseLUT[i] = b
	}
}

// neededBits computes the minimum number of bits needed to encode n elements.
func neededBits(n uint32) (nb uint) {
	for n--; n > 0; n >>= 1 {
		nb++
	}
	return
}

// reverseUint32 reverses all bits of v.
func reverseUint32(v uint32) (x uint32) {
	x |= uint32(reverseLUT[byte(v>>0)]) << 24
	x |= uint32(reverseLUT[byte(v>>8)]) << 16
	x |= uint32(reverseLUT[byte(v>>16)]) << 8
	x |= uint32(reverseLUT[byte(v>>24)]) << 0
	return x
}

// reverseBits reverses the lower n bits of v.
func reverseBits(v uint32, n uint) uint32 {
	return reverseUint32(v << (32 - n))
}

func allocUint8s(s []uint8, n int) []uint8 {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]uint8, n, n*3/2)
}

func allocUint32s(s []uint32, n int) []uint32 {
	if cap(s) >= n {
		return s[:n]
	}
	return make([]uint32, n, n*3/2)
}

func extendSliceUints32s(s [][]uint32, n int) [][]uint32 {
	if cap(s) >= n {
		return s[:n]
	}
	ss := make([][]uint32, n, n*3/2)
	copy(ss, s[:cap(s)])
	return ss
}

func extendDecoders(s []prefixDecoder, n int) []prefixDecoder {
	if cap(s) >= n {
		return s[:n]
	}
	ss := make([]prefixDecoder, n, n*3/2)
	copy(ss, s[:cap(s)])
	return ss
}
