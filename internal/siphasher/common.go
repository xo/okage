// Copyright 2012-2016 The Rust Project Developers
// Copyright 2016-2026 Frank Denis
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package siphasher

import (
	"encoding/binary"
	"math/bits"
)

// This file ports siphasher src/common.rs.

// compress is the macro compress! in siphasher src/common.rs.
//
// Compression round for SipHash algorithm.
func compress(s *state) {
	s.v0 += s.v1
	s.v1 = bits.RotateLeft64(s.v1, 13)
	s.v1 ^= s.v0
	s.v0 = bits.RotateLeft64(s.v0, 32)
	s.v2 += s.v3
	s.v3 = bits.RotateLeft64(s.v3, 16)
	s.v3 ^= s.v2
	s.v0 += s.v3
	s.v3 = bits.RotateLeft64(s.v3, 21)
	s.v3 ^= s.v0
	s.v2 += s.v1
	s.v1 = bits.RotateLeft64(s.v1, 17)
	s.v1 ^= s.v2
	s.v2 = bits.RotateLeft64(s.v2, 32)
}

// loadIntLEU64 is the macro load_int_le! in siphasher src/common.rs, for the
// type u64.
//
// Loads an integer of the desired type from a byte stream, in LE order.
func loadIntLEU64(buf []byte, i uint) uint64 {
	return binary.LittleEndian.Uint64(buf[i : i+8])
}

// u8to64LE is u8to64_le in siphasher src/common.rs.
//
// Loads a u64 using up to 7 bytes of a byte slice.
//
// The caller must ensure that start + len <= len(buf) and len < 8.
func u8to64LE(buf []byte, start, length uint) uint64 {
	var i uint
	var out uint64
	if i+3 < length {
		out = uint64(binary.LittleEndian.Uint32(buf[start+i:]))
		i += 4
	}
	if i+1 < length {
		out |= uint64(binary.LittleEndian.Uint16(buf[start+i:])) << (i * 8)
		i += 2
	}
	if i < length {
		out |= uint64(buf[start+i]) << (i * 8)
	}
	return out
}
