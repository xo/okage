// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"encoding/binary"
	"iter"
	"maps"
	"slices"
)

// This file ports subsetter src/remapper.rs. The trait CheckedAdd of
// upstream has no port, because the constraint remapperInt holds only the
// unsigned integer types, whose overflow Go can test.

// remapperInt is the set of integer types that a remapper counts and maps.
type remapperInt interface {
	~uint8 | ~uint16 | ~uint32
}

// remapper is Remapper in subsetter src/remapper.rs. A structure that allows
// to remap numeric types to new numbers so that they form a contiguous
// sequence of numbers.
//
// The zero value is not ready for use. Make one with newRemapper.
type remapper[C, T remapperInt] struct {
	// counter keeps track of the next number to be assigned. It always starts
	// with 0.
	counter C
	// forward maps numbers from their old value to their new value.
	forward map[T]T
	// backward stores the "reverse" mapping, i.e. given a new number, it
	// allows to map back to the old one.
	backward []T
}

// newRemapper is Remapper::new in subsetter src/remapper.rs. It creates a
// new instance of a remapper.
func newRemapper[C, T remapperInt]() remapper[C, T] {
	return remapper[C, T]{forward: map[T]T{}}
}

// equal is the impl PartialEq of Remapper in subsetter src/remapper.rs. Two
// remappers are equal when they map the same old values in the same order.
func (r *remapper[C, T]) equal(other *remapper[C, T]) bool {
	return slices.Equal(r.backward, other.backward)
}

// clone is the derived Clone of Remapper in subsetter src/remapper.rs.
func (r *remapper[C, T]) clone() remapper[C, T] {
	forward := make(map[T]T, len(r.forward))
	maps.Copy(forward, r.forward)
	return remapper[C, T]{counter: r.counter, forward: forward, backward: slices.Clone(r.backward)}
}

// get is Remapper::get in subsetter src/remapper.rs. It returns the new
// mapping of a value that has been remapped before, and false if it has not
// been remapped.
func (r *remapper[C, T]) get(old T) (T, bool) {
	v, ok := r.forward[old]
	return v, ok
}

// remap is Remapper::remap in subsetter src/remapper.rs. It remaps a new
// value, either returning the previously assigned number if it already has
// been remapped, and assigning a new number if it has not been remapped.
//
// As upstream, it panics when the counter overflows.
func (r *remapper[C, T]) remap(old T) T {
	v, ok := r.tryRemap(old)
	if !ok {
		panic("remapper was overflowed")
	}
	return v
}

// tryRemap is remap, but it returns false where remap panics, and it then
// does not change the remapper. Upstream has no such function. The closure
// of glyf.go calls it, so that a font cannot make the subsetter panic.
func (r *remapper[C, T]) tryRemap(old T) (T, bool) {
	if v, ok := r.forward[old]; ok {
		return v, true
	}
	value := r.counter
	next := r.counter + 1
	if next < r.counter {
		return 0, false
	}
	r.backward = append(r.backward, old)
	r.counter = next
	r.forward[old] = T(value)
	return T(value), true
}

// len is Remapper::len in subsetter src/remapper.rs. It returns the number
// of elements that have been remapped.
func (r *remapper[C, T]) len() C {
	return r.counter
}

// sortedIter is Remapper::sorted_iter in subsetter src/remapper.rs. It
// returns an iterator over the old values, in ascending order that is
// defined by the remapping.
func (r *remapper[C, T]) sortedIter() iter.Seq[T] {
	return slices.Values(r.backward)
}

// GlyphRemapper is GlyphRemapper in subsetter src/remapper.rs.
//
// A remapper that allows to assign a new ordering to a subset of glyphs.
//
// For example, let's say that we want to subset a font that only contains
// the glyphs 4, 9 and 16. In this case, the remapper could yield a remapping
// that assigns the following glyph IDs:
//
//   - 0 -> 0 (The .notdef glyph will always be included)
//   - 4 -> 1
//   - 9 -> 2
//   - 16 -> 3
//
// This is necessary because a font needs to have a contiguous sequence of
// glyph IDs that start from 0, so we cannot just reuse the old ones, but we
// need to define a mapping.
//
// The zero value is not ready for use. Make one with NewGlyphRemapper, which
// is GlyphRemapper::default and GlyphRemapper::new of upstream.
type GlyphRemapper struct {
	r remapper[uint16, uint16]
}

// NewGlyphRemapper is GlyphRemapper::new in subsetter src/remapper.rs. It
// creates a new instance of a glyph remapper. .notdef will always be a member
// of the subset.
func NewGlyphRemapper() *GlyphRemapper {
	g := &GlyphRemapper{r: newRemapper[uint16, uint16]()}
	// .notdef is always a part of a subset.
	g.r.remap(0)
	return g
}

// NewGlyphRemapperFromGlyphs is GlyphRemapper::new_from_glyphs in subsetter
// src/remapper.rs. It creates a remapper from an existing set of glyphs.
func NewGlyphRemapperFromGlyphs(glyphs []uint16) *GlyphRemapper {
	m := NewGlyphRemapper()
	for _, glyph := range glyphs {
		m.Remap(glyph)
	}
	return m
}

// NewGlyphRemapperFromGlyphsSorted is GlyphRemapper::new_from_glyphs_sorted
// in subsetter src/remapper.rs. It creates a remapper from an existing set of
// glyphs. The method will ensure that the mapping is monotonically
// increasing.
func NewGlyphRemapperFromGlyphsSorted(glyphs []uint16) *GlyphRemapper {
	sorted := slices.Clone(glyphs)
	slices.Sort(sorted)
	return NewGlyphRemapperFromGlyphs(slices.Compact(sorted))
}

// NumGids is GlyphRemapper::num_gids in subsetter src/remapper.rs. It
// returns the number of gids that have been remapped.
func (g *GlyphRemapper) NumGids() uint16 {
	return g.r.len()
}

// Remap is GlyphRemapper::remap in subsetter src/remapper.rs. It remaps a
// glyph ID, or returns the existing mapping if the glyph ID has already been
// remapped before.
//
// As upstream, it panics when the remapper already holds 65535 glyphs and
// the glyph is new.
func (g *GlyphRemapper) Remap(old uint16) uint16 {
	return g.r.remap(old)
}

// Get is GlyphRemapper::get in subsetter src/remapper.rs. It returns the
// mapping of a glyph ID, if it has been remapped before.
func (g *GlyphRemapper) Get(old uint16) (uint16, bool) {
	return g.r.get(old)
}

// RemappedGids is GlyphRemapper::remapped_gids in subsetter
// src/remapper.rs. It returns an iterator that yields the old glyphs, in
// ascending order that is defined by the remapping. For example, if we
// perform the following remappings:
//
//	3, 39, 8, 3, 10, 2
//
// Then the iterator will yield the following items in the order below. The
// order also implicitly defines the glyph IDs in the new mapping:
//
//	0 (0), 3 (1), 39 (2), 8 (3), 10 (4), 2 (5)
func (g *GlyphRemapper) RemappedGids() iter.Seq[uint16] {
	return g.r.sortedIter()
}

// Clone is the derived Clone of GlyphRemapper in subsetter src/remapper.rs.
// It returns a copy that does not share memory with g.
func (g *GlyphRemapper) Clone() *GlyphRemapper {
	return &GlyphRemapper{r: g.r.clone()}
}

// Equal is the derived PartialEq of GlyphRemapper in subsetter
// src/remapper.rs. Two remappers are equal when they map the same glyphs in
// the same order.
func (g *GlyphRemapper) Equal(other *GlyphRemapper) bool {
	return g.r.equal(&other.r)
}

// Hasher is the part of the trait Hasher of Rust that Hash calls. The
// hashers of internal/siphasher have these methods.
type Hasher interface {
	// WriteUsize is Hasher::write_usize of Rust. krilla gives a hasher whose
	// WriteUsize writes a u64, so that the hash is the same on each machine.
	WriteUsize(i uint)
	// Write is Hasher::write of Rust.
	Write(b []byte)
}

// Hash is the derived Hash of GlyphRemapper, and the impl Hash of Remapper,
// in subsetter src/remapper.rs. It hashes the old glyphs in the order of the
// remapping, as Rust hashes a Vec<u16>: the length with WriteUsize, then the
// bytes of the values with one call of Write. krilla hashes a remapper into
// the tag of the name of a font subset.
//
// Rust writes the bytes of a u16 in the byte order of the machine. The port
// writes them in little-endian order on every machine, as internal/siphasher
// reads them, so that each machine gives the same tag.
func (g *GlyphRemapper) Hash(h Hasher) {
	h.WriteUsize(uint(len(g.r.backward)))
	b := make([]byte, 0, 2*len(g.r.backward))
	for _, v := range g.r.backward {
		b = binary.LittleEndian.AppendUint16(b, v)
	}
	h.Write(b)
}
