// Copyright 2012-2015 The Rust Project Developers
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package siphasher

import (
	"encoding/binary"
	"math/bits"
)

// This file ports siphasher src/sip128.rs.
//
// An implementation of SipHash with a 128-bit output.

// Hash128 is Hash128 in siphasher src/sip128.rs.
//
// A 128-bit (2x64) hash output.
type Hash128 struct {
	H1 uint64
	H2 uint64
}

// Equal is the impl PartialEq for Hash128 in siphasher src/sip128.rs.
//
// Constant-time equality comparison to prevent timing attacks.
func (h Hash128) Equal(other Hash128) bool {
	x := (h.H1 ^ other.H1) | (h.H2 ^ other.H2)
	return x == 0
}

// Hash128FromU128 is the impl From<u128> for Hash128 in siphasher
// src/sip128.rs. Go has no 128-bit integer, so the value comes in two
// halves: hi holds the bits 64 to 127, and lo the bits 0 to 63.
func Hash128FromU128(hi, lo uint64) Hash128 {
	return Hash128{H1: lo, H2: hi}
}

// U128 is the impl From<Hash128> for u128 in siphasher src/sip128.rs. It
// returns the 128-bit value in two halves, as Hash128FromU128 takes them.
func (h Hash128) U128() (hi, lo uint64) {
	return h.H2, h.H1
}

// SipHasher13 is SipHasher13 in siphasher src/sip128.rs.
//
// An implementation of SipHash128 1-3.
type SipHasher13 struct {
	hasher hasher[sip13Rounds]
}

// SipHasher24 is SipHasher24 in siphasher src/sip128.rs.
//
// An implementation of SipHash128 2-4.
type SipHasher24 struct {
	hasher hasher[sip24Rounds]
}

// SipHasher is SipHasher in siphasher src/sip128.rs.
//
// An implementation of SipHash128 2-4.
//
// SipHash is a general-purpose hashing function: it runs at a good speed
// (competitive with Spooky and City) and permits strong keyed hashing. This
// lets you key your hashtables from a strong RNG.
//
// Although the SipHash algorithm is considered to be generally strong, it is
// not intended for cryptographic purposes. As such, all cryptographic uses of
// this implementation are strongly discouraged.
type SipHasher struct {
	s SipHasher24
}

// hasher is Hasher in siphasher src/sip128.rs. The type parameter S gives
// the number of rounds.
type hasher[S sip] struct {
	k0     uint64
	k1     uint64
	length uint   // how many bytes we've processed
	state  state  // hash State
	tail   uint64 // unprocessed bytes le
	ntail  uint   // how many bytes in tail are valid
}

// state is State in siphasher src/sip128.rs.
type state struct {
	// v0, v2 and v1, v3 show up in pairs in the algorithm,
	// and simd implementations of SipHash will use vectors
	// of v02 and v13. By placing them in this order in the struct,
	// the compiler can pick up on just a few simd optimizations by itself.
	v0 uint64
	v2 uint64
	v1 uint64
	v3 uint64
}

// NewSipHasher is SipHasher::new in siphasher src/sip128.rs.
//
// Creates a new SipHasher with the two initial keys set to 0.
func NewSipHasher() SipHasher {
	return NewSipHasherWithKeys(0, 0)
}

// NewSipHasherWithKeys is SipHasher::new_with_keys in siphasher
// src/sip128.rs.
//
// Creates a SipHasher that is keyed off the provided keys.
func NewSipHasherWithKeys(key0, key1 uint64) SipHasher {
	return SipHasher{NewSipHasher24WithKeys(key0, key1)}
}

// NewSipHasherWithKey is SipHasher::new_with_key in siphasher
// src/sip128.rs.
//
// Creates a SipHasher from a 16 byte key.
func NewSipHasherWithKey(key [16]byte) SipHasher {
	key0 := binary.LittleEndian.Uint64(key[0:8])
	key1 := binary.LittleEndian.Uint64(key[8:16])
	return NewSipHasherWithKeys(key0, key1)
}

// Keys is SipHasher::keys in siphasher src/sip128.rs.
//
// Get the keys used by this hasher.
func (s SipHasher) Keys() (uint64, uint64) {
	return s.s.hasher.k0, s.s.hasher.k1
}

// Key is SipHasher::key in siphasher src/sip128.rs.
//
// Get the key used by this hasher as a 16 byte vector.
func (s SipHasher) Key() [16]byte {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[0:8], s.s.hasher.k0)
	binary.LittleEndian.PutUint64(b[8:16], s.s.hasher.k1)
	return b
}

// Hash is SipHasher::hash in siphasher src/sip128.rs.
//
// Hash a byte array. This is the easiest and safest way to use SipHash.
func (s SipHasher) Hash(b []byte) Hash128 {
	h := s.s.hasher
	h.write(b)
	return h.finish128()
}

// Finish128 is the impl Hasher128 for SipHasher in siphasher src/sip128.rs.
//
// Return a 128-bit hash.
func (s SipHasher) Finish128() Hash128 {
	return s.s.Finish128()
}

// NewSipHasher13 is SipHasher13::new in siphasher src/sip128.rs.
//
// Creates a new SipHasher13 with the two initial keys set to 0.
func NewSipHasher13() SipHasher13 {
	return NewSipHasher13WithKeys(0, 0)
}

// NewSipHasher13WithKeys is SipHasher13::new_with_keys in siphasher
// src/sip128.rs.
//
// Creates a SipHasher13 that is keyed off the provided keys.
func NewSipHasher13WithKeys(key0, key1 uint64) SipHasher13 {
	return SipHasher13{hasher: newHasherWithKeys[sip13Rounds](key0, key1)}
}

// NewSipHasher13WithKey is SipHasher13::new_with_key in siphasher
// src/sip128.rs.
//
// Creates a SipHasher13 from a 16 byte key.
func NewSipHasher13WithKey(key [16]byte) SipHasher13 {
	key0 := binary.LittleEndian.Uint64(key[0:8])
	key1 := binary.LittleEndian.Uint64(key[8:16])
	return NewSipHasher13WithKeys(key0, key1)
}

// Keys is SipHasher13::keys in siphasher src/sip128.rs.
//
// Get the keys used by this hasher.
func (s SipHasher13) Keys() (uint64, uint64) {
	return s.hasher.k0, s.hasher.k1
}

// Key is SipHasher13::key in siphasher src/sip128.rs.
//
// Get the key used by this hasher as a 16 byte vector.
func (s SipHasher13) Key() [16]byte {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[0:8], s.hasher.k0)
	binary.LittleEndian.PutUint64(b[8:16], s.hasher.k1)
	return b
}

// Hash is SipHasher13::hash in siphasher src/sip128.rs.
//
// Hash a byte array. This is the easiest and safest way to use SipHash.
func (s SipHasher13) Hash(b []byte) Hash128 {
	h := s.hasher
	h.write(b)
	return h.finish128()
}

// Finish128 is the impl Hasher128 for SipHasher13 in siphasher
// src/sip128.rs.
//
// Return a 128-bit hash.
func (s SipHasher13) Finish128() Hash128 {
	return s.hasher.finish128()
}

// NewSipHasher24 is SipHasher24::new in siphasher src/sip128.rs.
//
// Creates a new SipHasher24 with the two initial keys set to 0.
func NewSipHasher24() SipHasher24 {
	return NewSipHasher24WithKeys(0, 0)
}

// NewSipHasher24WithKeys is SipHasher24::new_with_keys in siphasher
// src/sip128.rs.
//
// Creates a SipHasher24 that is keyed off the provided keys.
func NewSipHasher24WithKeys(key0, key1 uint64) SipHasher24 {
	return SipHasher24{hasher: newHasherWithKeys[sip24Rounds](key0, key1)}
}

// NewSipHasher24WithKey is SipHasher24::new_with_key in siphasher
// src/sip128.rs.
//
// Creates a SipHasher24 from a 16 byte key.
func NewSipHasher24WithKey(key [16]byte) SipHasher24 {
	key0 := binary.LittleEndian.Uint64(key[0:8])
	key1 := binary.LittleEndian.Uint64(key[8:16])
	return NewSipHasher24WithKeys(key0, key1)
}

// Keys is SipHasher24::keys in siphasher src/sip128.rs.
//
// Get the keys used by this hasher.
func (s SipHasher24) Keys() (uint64, uint64) {
	return s.hasher.k0, s.hasher.k1
}

// Key is SipHasher24::key in siphasher src/sip128.rs.
//
// Get the key used by this hasher as a 16 byte vector.
func (s SipHasher24) Key() [16]byte {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[0:8], s.hasher.k0)
	binary.LittleEndian.PutUint64(b[8:16], s.hasher.k1)
	return b
}

// Hash is SipHasher24::hash in siphasher src/sip128.rs.
//
// Hash a byte array. This is the easiest and safest way to use SipHash.
func (s SipHasher24) Hash(b []byte) Hash128 {
	h := s.hasher
	h.write(b)
	return h.finish128()
}

// Finish128 is the impl Hasher128 for SipHasher24 in siphasher
// src/sip128.rs.
//
// Return a 128-bit hash.
func (s SipHasher24) Finish128() Hash128 {
	return s.hasher.finish128()
}

// newHasherWithKeys is Hasher::new_with_keys in siphasher src/sip128.rs.
func newHasherWithKeys[S sip](key0, key1 uint64) hasher[S] {
	h := hasher[S]{
		k0: key0,
		k1: key1,
		state: state{
			v0: 0,
			v1: 0xee,
			v2: 0,
			v3: 0,
		},
	}
	h.reset()
	return h
}

// reset is Hasher::reset in siphasher src/sip128.rs.
func (h *hasher[S]) reset() {
	h.length = 0
	h.state.v0 = h.k0 ^ 0x736f6d6570736575
	h.state.v1 = h.k1 ^ 0x646f72616e646f83
	h.state.v2 = h.k0 ^ 0x6c7967656e657261
	h.state.v3 = h.k1 ^ 0x7465646279746573
	h.ntail = 0
}

// shortWrite is Hasher::short_write in siphasher src/sip128.rs. size is the
// size in bytes of the Rust type of x.
//
// A specialized write function for values with size <= 8.
//
// The hashing of multi-byte integers depends on endianness. E.g.:
//   - little-endian: write_u32(0xDDCCBBAA) == write([0xAA, 0xBB, 0xCC, 0xDD])
//   - big-endian:    write_u32(0xDDCCBBAA) == write([0xDD, 0xCC, 0xBB, 0xAA])
//
// This function does the right thing for little-endian hardware. The port
// takes the little-endian order on every machine (D16). The input must be
// zero-extended to 64-bits. The caller is responsible for the
// zero-extension.
func (h *hasher[S]) shortWrite(size uint, x uint64) {
	h.length += size

	// The number of bytes needed to fill `self.tail`.
	needed := 8 - h.ntail

	h.tail |= x << (8 * h.ntail)
	if size < needed {
		h.ntail += size
		return
	}

	// `self.tail` is full, process it.
	var s S
	h.state.v3 ^= h.tail
	s.cRounds(&h.state)
	h.state.v0 ^= h.tail

	h.ntail = size - needed
	if needed < 8 {
		h.tail = x >> (8 * needed)
	} else {
		h.tail = 0
	}
}

// finish128 is Hasher::finish128 in siphasher src/sip128.rs.
func (h hasher[S]) finish128() Hash128 {
	var s S
	st := h.state

	b := ((uint64(h.length) & 0xff) << 56) | h.tail

	st.v3 ^= b
	s.cRounds(&st)
	st.v0 ^= b

	st.v2 ^= 0xee
	s.dRounds(&st)
	h1 := st.v0 ^ st.v1 ^ st.v2 ^ st.v3

	st.v1 ^= 0xdd
	s.dRounds(&st)
	h2 := st.v0 ^ st.v1 ^ st.v2 ^ st.v3

	return Hash128{H1: h1, H2: h2}
}

// Write is the method write of the impl hash::Hasher for SipHasher in
// siphasher src/sip128.rs.
func (s *SipHasher) Write(msg []byte) {
	s.s.Write(msg)
}

// Finish is the method finish of the impl hash::Hasher for SipHasher in
// siphasher src/sip128.rs.
func (s SipHasher) Finish() uint64 {
	return s.s.Finish()
}

// WriteUsize is the method write_usize of the impl hash::Hasher for
// SipHasher in siphasher src/sip128.rs.
func (s *SipHasher) WriteUsize(i uint) {
	s.s.WriteUsize(i)
}

// WriteU8 is the method write_u8 of the impl hash::Hasher for SipHasher in
// siphasher src/sip128.rs.
func (s *SipHasher) WriteU8(i uint8) {
	s.s.WriteU8(i)
}

// WriteU16 is the method write_u16 of the impl hash::Hasher for SipHasher in
// siphasher src/sip128.rs.
func (s *SipHasher) WriteU16(i uint16) {
	s.s.WriteU16(i)
}

// WriteU32 is the method write_u32 of the impl hash::Hasher for SipHasher in
// siphasher src/sip128.rs.
func (s *SipHasher) WriteU32(i uint32) {
	s.s.WriteU32(i)
}

// WriteU64 is the method write_u64 of the impl hash::Hasher for SipHasher in
// siphasher src/sip128.rs.
func (s *SipHasher) WriteU64(i uint64) {
	s.s.WriteU64(i)
}

// Write is the method write of the impl hash::Hasher for SipHasher13 in
// siphasher src/sip128.rs.
func (s *SipHasher13) Write(msg []byte) {
	s.hasher.write(msg)
}

// Finish is the method finish of the impl hash::Hasher for SipHasher13 in
// siphasher src/sip128.rs.
func (s SipHasher13) Finish() uint64 {
	return s.hasher.finish()
}

// WriteUsize is the method write_usize of the impl hash::Hasher for
// SipHasher13 in siphasher src/sip128.rs.
func (s *SipHasher13) WriteUsize(i uint) {
	s.hasher.writeUsize(i)
}

// WriteU8 is the method write_u8 of the impl hash::Hasher for SipHasher13 in
// siphasher src/sip128.rs.
func (s *SipHasher13) WriteU8(i uint8) {
	s.hasher.writeU8(i)
}

// WriteU16 is the method write_u16 of the impl hash::Hasher for SipHasher13
// in siphasher src/sip128.rs.
func (s *SipHasher13) WriteU16(i uint16) {
	s.hasher.writeU16(i)
}

// WriteU32 is the method write_u32 of the impl hash::Hasher for SipHasher13
// in siphasher src/sip128.rs.
func (s *SipHasher13) WriteU32(i uint32) {
	s.hasher.writeU32(i)
}

// WriteU64 is the method write_u64 of the impl hash::Hasher for SipHasher13
// in siphasher src/sip128.rs.
func (s *SipHasher13) WriteU64(i uint64) {
	s.hasher.writeU64(i)
}

// Write is the method write of the impl hash::Hasher for SipHasher24 in
// siphasher src/sip128.rs.
func (s *SipHasher24) Write(msg []byte) {
	s.hasher.write(msg)
}

// Finish is the method finish of the impl hash::Hasher for SipHasher24 in
// siphasher src/sip128.rs.
func (s SipHasher24) Finish() uint64 {
	return s.hasher.finish()
}

// WriteUsize is the method write_usize of the impl hash::Hasher for
// SipHasher24 in siphasher src/sip128.rs.
func (s *SipHasher24) WriteUsize(i uint) {
	s.hasher.writeUsize(i)
}

// WriteU8 is the method write_u8 of the impl hash::Hasher for SipHasher24 in
// siphasher src/sip128.rs.
func (s *SipHasher24) WriteU8(i uint8) {
	s.hasher.writeU8(i)
}

// WriteU16 is the method write_u16 of the impl hash::Hasher for SipHasher24
// in siphasher src/sip128.rs.
func (s *SipHasher24) WriteU16(i uint16) {
	s.hasher.writeU16(i)
}

// WriteU32 is the method write_u32 of the impl hash::Hasher for SipHasher24
// in siphasher src/sip128.rs.
func (s *SipHasher24) WriteU32(i uint32) {
	s.hasher.writeU32(i)
}

// WriteU64 is the method write_u64 of the impl hash::Hasher for SipHasher24
// in siphasher src/sip128.rs.
func (s *SipHasher24) WriteU64(i uint64) {
	s.hasher.writeU64(i)
}

// writeUsize is the method write_usize of the impl hash::Hasher for Hasher
// in siphasher src/sip128.rs. A usize has the size of a Go uint: 8 bytes on
// a 64-bit machine and 4 bytes on a 32-bit machine.
func (h *hasher[S]) writeUsize(i uint) {
	h.shortWrite(bits.UintSize/8, uint64(i))
}

// writeU8 is the method write_u8 of the impl hash::Hasher for Hasher in
// siphasher src/sip128.rs.
func (h *hasher[S]) writeU8(i uint8) {
	h.shortWrite(1, uint64(i))
}

// writeU16 is the method write_u16 of the trait core::hash::Hasher, which
// Hasher in siphasher src/sip128.rs does not override. It writes the two
// bytes of i, as the trait does.
func (h *hasher[S]) writeU16(i uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], i)
	h.write(b[:])
}

// writeU32 is the method write_u32 of the impl hash::Hasher for Hasher in
// siphasher src/sip128.rs.
func (h *hasher[S]) writeU32(i uint32) {
	h.shortWrite(4, uint64(i))
}

// writeU64 is the method write_u64 of the impl hash::Hasher for Hasher in
// siphasher src/sip128.rs.
func (h *hasher[S]) writeU64(i uint64) {
	h.shortWrite(8, i)
}

// write is the method write of the impl hash::Hasher for Hasher in siphasher
// src/sip128.rs.
func (h *hasher[S]) write(msg []byte) {
	var s S
	length := uint(len(msg))
	h.length += length

	var needed uint

	if h.ntail != 0 {
		needed = 8 - h.ntail
		h.tail |= u8to64LE(msg, 0, min(length, needed)) << (8 * h.ntail)
		if length < needed {
			h.ntail += length
			return
		}
		h.state.v3 ^= h.tail
		s.cRounds(&h.state)
		h.state.v0 ^= h.tail
		h.ntail = 0
	}

	// Buffered tail is now flushed, process new input.
	n := length - needed
	left := n & 0x7

	i := needed
	for i < n-left {
		mi := loadIntLEU64(msg, i)

		h.state.v3 ^= mi
		s.cRounds(&h.state)
		h.state.v0 ^= mi

		i += 8
	}

	h.tail = u8to64LE(msg, i, left)
	h.ntail = left
}

// finish is the method finish of the impl hash::Hasher for Hasher in
// siphasher src/sip128.rs.
func (h hasher[S]) finish() uint64 {
	return h.finish128().H2
}

// sip is the trait Sip in siphasher src/sip128.rs.
type sip interface {
	cRounds(s *state)
	dRounds(s *state)
}

// sip13Rounds is Sip13Rounds in siphasher src/sip128.rs.
type sip13Rounds struct{}

// cRounds is Sip13Rounds::c_rounds in siphasher src/sip128.rs.
func (sip13Rounds) cRounds(s *state) {
	compress(s)
}

// dRounds is Sip13Rounds::d_rounds in siphasher src/sip128.rs.
func (sip13Rounds) dRounds(s *state) {
	compress(s)
	compress(s)
	compress(s)
}

// sip24Rounds is Sip24Rounds in siphasher src/sip128.rs.
type sip24Rounds struct{}

// cRounds is Sip24Rounds::c_rounds in siphasher src/sip128.rs.
func (sip24Rounds) cRounds(s *state) {
	compress(s)
	compress(s)
}

// dRounds is Sip24Rounds::d_rounds in siphasher src/sip128.rs.
func (sip24Rounds) dRounds(s *state) {
	compress(s)
	compress(s)
	compress(s)
	compress(s)
}

// AsBytes is Hash128::as_bytes in siphasher src/sip128.rs.
//
// Convert into a 16-bytes vector.
func (h Hash128) AsBytes() [16]byte {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[0:8], h.H1)
	binary.LittleEndian.PutUint64(b[8:16], h.H2)
	return b
}

// AsU128 is Hash128::as_u128 in siphasher src/sip128.rs. Go has no 128-bit
// integer, so the value comes in two halves: hi holds the bits 64 to 127,
// and lo the bits 0 to 63.
//
// Convert into a u128.
func (h Hash128) AsU128() (hi, lo uint64) {
	return h.H2, h.H1
}

// AsU64 is Hash128::as_u64 in siphasher src/sip128.rs.
//
// Convert into (u64, u64).
func (h Hash128) AsU64() (uint64, uint64) {
	return h.H1, h.H2
}
