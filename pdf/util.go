// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"encoding/base64"
	"encoding/binary"
	"iter"

	"github.com/xo/okage/internal/siphasher"
	"github.com/xo/okage/path"
)

// The trait NameExt of krilla src/util.rs makes a name of pdf-writer from a
// string. The port converts the string with pdfwriter.Name, so it has no
// port.
//
// Deferred of krilla src/util.rs runs a function on another thread when the
// feature rayon is on, and at once when it is off. The port computes each
// value at once, as krilla without rayon does, so Deferred has no port. The
// module test_utils holds the paths of the tests, which the tests of the port
// give in their own way.

// calculateStrokeBBox is calculate_stroke_bbox of krilla src/util.rs. It
// gives the tight bounds of the stroke of a path. krilla takes the Stroke of
// krilla and converts it into the stroke of tiny-skia. The caller gives the
// stroke of path, which is that conversion.
func calculateStrokeBBox(stroke *path.Stroke, p *path.Path) (Rect, bool) {
	if stroked, ok := p.Stroke(stroke, 1.0); ok {
		return stroked.ComputeTightBounds()
	}
	return Rect{}, false
}

// hasher is the trait Hasher of Rust, on the SipHasher13 of siphasher with
// the keys 0, as krilla hashes a value. A method of hasher writes a value as
// the impl Hash of Rust writes it, so that stableHash128 gives the hash of
// upstream. A usize is a u64, as StableHasher of stable_hash128 writes it.
// sip_hash writes a usize with the width of the machine, which is the same
// on a machine of 64 bits.
type hasher struct {
	s siphasher.SipHasher13
}

// newHasher returns a hasher with the keys 0, as SipHasher13::new.
func newHasher() *hasher {
	return &hasher{s: siphasher.NewSipHasher13()}
}

// write is Hasher::write. It writes the bytes as they are.
func (h *hasher) write(b []byte) {
	h.s.Write(b)
}

// u8 is Hasher::write_u8, and the Hash of a u8.
func (h *hasher) u8(v uint8) {
	h.s.WriteU8(v)
}

// u16 is Hasher::write_u16, and the Hash of a u16.
func (h *hasher) u16(v uint16) {
	h.s.WriteU16(v)
}

// u32 is Hasher::write_u32, and the Hash of a u32. The bits of an f32 go
// through it, as krilla hashes them with to_bits.
func (h *hasher) u32(v uint32) {
	h.s.WriteU32(v)
}

// i32 is Hasher::write_i32, and the Hash of an i32, such as the number of a
// Ref.
func (h *hasher) i32(v int32) {
	h.s.WriteU32(uint32(v))
}

// u64 is Hasher::write_u64, and the Hash of a u64.
func (h *hasher) u64(v uint64) {
	h.s.WriteU64(v)
}

// u128 is Hasher::write_u128, and the Hash of a u128. It writes the bits 0 to
// 63 first, as a little-endian machine does.
func (h *hasher) u128(v siphasher.Hash128) {
	hi, lo := v.AsU128()
	h.s.WriteU64(lo)
	h.s.WriteU64(hi)
}

// usize is Hasher::write_usize of StableHasher, which writes a u64. The
// length of a slice, the discriminant of an enum and the Hash of a usize go
// through it.
func (h *hasher) usize(v int) {
	h.s.WriteU64(uint64(v))
}

// bool is the Hash of a bool, which writes a u8.
func (h *hasher) bool(v bool) {
	if v {
		h.u8(1)
	} else {
		h.u8(0)
	}
}

// bytes is the Hash of a [u8] or a Vec<u8>: the length, then the bytes in one
// write.
func (h *hasher) bytes(b []byte) {
	h.usize(len(b))
	h.write(b)
}

// str is the Hash of a str or a String: the bytes, then the byte 0xff.
func (h *hasher) str(s string) {
	h.write([]byte(s))
	h.u8(0xff)
}

// finish128 is Hasher128::finish128.
func (h *hasher) finish128() siphasher.Hash128 {
	return h.s.Finish128()
}

// hashable is the trait Hash of Rust, for the values that krilla hashes. The
// method hash writes the value into h as the impl Hash of upstream does: the
// fields in their order, the discriminant of an enum with usize, a slice with
// its length first.
type hashable interface {
	hash(h *hasher)
}

// sipHashable is the trait SipHashable of krilla src/util.rs. Rust hashes the
// type_id of a value before the value, so that two values of other types do
// not get the same hash. Go has no type_id without the package reflect, which
// okage does not use (D25), so each type gives a name of its own. Each name
// must differ from the names of the other types of the package. The name of
// the Go type is the convention.
type sipHashable interface {
	hashable
	typeID() string
}

// sipHash is SipHashable::sip_hash of krilla src/util.rs. It hashes the name
// of the type, then the value. The hash is a key of a cache, and it differs
// from the hash of upstream, which hashes a type_id.
func sipHash(v sipHashable) siphasher.Hash128 {
	h := newHasher()
	h.str(v.typeID())
	v.hash(h)
	return h.finish128()
}

// prehashed is Prehashed of krilla src/util.rs. It holds a value with its
// hash, so that a large value, such as a font, is hashed once. Two prehashed
// values are equal when their hashes are equal.
type prehashed[T sipHashable] struct {
	hash  siphasher.Hash128
	value T
}

// newPrehashed is Prehashed::new of krilla src/util.rs.
func newPrehashed[T sipHashable](value T) prehashed[T] {
	return prehashed[T]{hash: sipHash(value), value: value}
}

// equal is the impl PartialEq for Prehashed of krilla src/util.rs. It
// compares the hashes only.
func (p prehashed[T]) equal(other prehashed[T]) bool {
	return p.hash == other.hash
}

// write is the impl Hash for Prehashed of krilla src/util.rs. It writes the
// hash as a u128. A type that holds a prehashed value calls it from its
// method hash.
func (p prehashed[T]) write(h *hasher) {
	h.u128(p.hash)
}

// groupByKey is SliceExt::group_by_key of krilla src/util.rs, with the
// iterator GroupByKey.
//
// Split a slice into consecutive runs with the same key and yield for each
// such run the key and the slice of elements with that key.
func groupByKey[T any, K comparable](s []T, f func(T) K) iter.Seq2[K, []T] {
	return func(yield func(K, []T) bool) {
		for len(s) > 0 {
			key := f(s[0])
			count := 1
			for count < len(s) && f(s[count]) == key {
				count++
			}
			head, tail := s[:count], s[count:]
			s = tail
			if !yield(key, head) {
				return
			}
		}
	}
}

// stableHashBase64 is stable_hash_base64 of krilla src/util.rs. It gives the
// hash of stableHash128 in base64, with the bits of the u128 in big-endian
// order.
//
// Create a base64-encoded hash of the value.
func stableHashBase64(write func(h *hasher)) string {
	hi, lo := stableHash128(write).AsU128()
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], hi)
	binary.BigEndian.PutUint64(b[8:], lo)
	return base64.StdEncoding.EncodeToString(b[:])
}

// stableHash128 is stable_hash128 of krilla src/util.rs. The function write
// writes the value into the hasher, as its impl Hash does. The hash is the
// same as the hash of upstream on each machine.
//
// Calculate a 128-bit siphash of a value.
func stableHash128(write func(h *hasher)) siphasher.Hash128 {
	h := newHasher()
	write(h)
	return h.finish128()
}
