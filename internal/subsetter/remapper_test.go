// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"slices"
	"testing"
)

// subsetter src/remapper.rs has no tests. These tests check the examples of
// its comments, and the parts of the port that upstream does not have.

func TestGlyphRemapper(t *testing.T) {
	m := NewGlyphRemapper()
	for _, g := range []uint16{3, 39, 8, 3, 10, 2} {
		m.Remap(g)
	}
	if got, want := slices.Collect(m.RemappedGids()), []uint16{0, 3, 39, 8, 10, 2}; !slices.Equal(got, want) {
		t.Errorf("the remapped glyphs are %v, want %v", got, want)
	}
	if m.NumGids() != 6 {
		t.Errorf("NumGids is %d", m.NumGids())
	}
	if g, ok := m.Get(39); !ok || g != 2 {
		t.Errorf("Get(39) = %d, %v", g, ok)
	}
	if _, ok := m.Get(40); ok {
		t.Error("Get(40) finds a glyph")
	}

	c := m.Clone()
	c.Remap(40)
	if m.Equal(c) || m.NumGids() != 6 {
		t.Error("a change of the clone changes the remapper")
	}
	if !m.Equal(NewGlyphRemapperFromGlyphs([]uint16{3, 39, 8, 10, 2})) {
		t.Error("the remapper is not equal to the remapper of the same glyphs")
	}
	sorted := NewGlyphRemapperFromGlyphsSorted([]uint16{10, 3, 39, 3, 0, 8})
	if got, want := slices.Collect(sorted.RemappedGids()), []uint16{0, 3, 8, 10, 39}; !slices.Equal(got, want) {
		t.Errorf("the sorted remapper holds %v, want %v", got, want)
	}
}

// TestGlyphRemapperOverflow checks that Remap panics as upstream when a new
// glyph does not fit, and that tryRemap returns false.
func TestGlyphRemapperOverflow(t *testing.T) {
	m := NewGlyphRemapper()
	for g := 1; g < 0xFFFF; g++ {
		m.Remap(uint16(g))
	}
	if m.NumGids() != 0xFFFF {
		t.Fatalf("NumGids is %d", m.NumGids())
	}
	if g := m.Remap(5); g != 5 {
		t.Errorf("Remap(5) = %d", g)
	}
	if _, ok := m.r.tryRemap(0xFFFF); ok {
		t.Error("tryRemap adds a glyph to a full remapper")
	}
	defer func() {
		if recover() == nil {
			t.Error("Remap does not panic on a full remapper")
		}
	}()
	m.Remap(0xFFFF)
}

// TestRemapperU8 checks a remapper of u8, as the font dicts of CFF use it.
func TestRemapperU8(t *testing.T) {
	r := newRemapper[uint8, uint8]()
	for v := range 255 {
		if got := r.remap(uint8(254 - v)); got != uint8(v) {
			t.Fatalf("remap(%d) = %d", 254-v, got)
		}
	}
	if r.len() != 255 || r.remap(254) != 0 {
		t.Errorf("len is %d", r.len())
	}
	if _, ok := r.tryRemap(255); ok {
		t.Error("tryRemap adds a value to a full remapper")
	}
	if got := slices.Collect(r.sortedIter()); len(got) != 255 || got[0] != 254 {
		t.Errorf("sortedIter gives %v", got[:3])
	}
}

// recordingHasher records the calls of Hash.
type recordingHasher struct {
	calls []any
}

func (h *recordingHasher) WriteUsize(i uint) { h.calls = append(h.calls, i) }
func (h *recordingHasher) Write(b []byte)    { h.calls = append(h.calls, slices.Clone(b)) }

// TestGlyphRemapperHash checks the calls of Hash: the length, then the
// glyphs as little-endian bytes, as Rust hashes a Vec<u16>.
func TestGlyphRemapperHash(t *testing.T) {
	var h recordingHasher
	NewGlyphRemapperFromGlyphs([]uint16{0x0102, 3}).Hash(&h)
	if len(h.calls) != 2 {
		t.Fatalf("Hash makes the calls %v", h.calls)
	}
	b, ok := h.calls[1].([]byte)
	if h.calls[0] != uint(3) || !ok || !slices.Equal(b, []byte{0, 0, 2, 1, 3, 0}) {
		t.Errorf("Hash makes the calls %v", h.calls)
	}
}
