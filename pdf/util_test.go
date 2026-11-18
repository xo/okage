// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"encoding/base64"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/internal/siphasher"
	"github.com/xo/okage/path"
)

// hashedInt and hashedOther are two types with the same fields, which must
// get two hashes from sipHash.
type (
	hashedInt   struct{ v uint32 }
	hashedOther struct{ v uint32 }
)

func (h hashedInt) hash(s *hasher)   { s.u32(h.v) }
func (hashedInt) typeID() string     { return "hashedInt" }
func (h hashedOther) hash(s *hasher) { s.u32(h.v) }
func (hashedOther) typeID() string   { return "hashedOther" }

// TestStableHash checks that the hasher writes a value as the impl Hash of
// Rust does: a slice of bytes with its length as a u64, a string with the
// byte 0xff after it. The snapshots compare the ID of the document, which is
// stable_hash_base64 of the bytes of the file, with krilla.
func TestStableHash(t *testing.T) {
	want := func(b []byte) siphasher.Hash128 {
		s := siphasher.NewSipHasher13()
		s.Write(b)
		return s.Finish128()
	}
	data := []byte("%PDF-1.7\n")
	prefixed := binary.LittleEndian.AppendUint64(nil, uint64(len(data)))
	if got := stableHash128(func(h *hasher) { h.bytes(data) }); got != want(append(prefixed, data...)) {
		t.Errorf("the hash of a slice is %x", got.AsBytes())
	}
	if got := stableHash128(func(h *hasher) { h.str("PDF 1.7") }); got != want(append([]byte("PDF 1.7"), 0xff)) {
		t.Errorf("the hash of a string is %x", got.AsBytes())
	}
	all := stableHash128(func(h *hasher) {
		h.u8(1)
		h.u16(2)
		h.u32(3)
		h.i32(-4)
		h.u64(5)
		h.usize(6)
		h.bool(true)
		h.bool(false)
		h.u128(siphasher.Hash128FromU128(7, 8))
	})
	var b []byte
	b = append(b, 1)
	b = binary.LittleEndian.AppendUint16(b, 2)
	b = binary.LittleEndian.AppendUint32(b, 3)
	b = binary.LittleEndian.AppendUint32(b, 0xfffffffc)
	b = binary.LittleEndian.AppendUint64(b, 5)
	b = binary.LittleEndian.AppendUint64(b, 6)
	b = append(b, 1, 0)
	b = binary.LittleEndian.AppendUint64(b, 8)
	b = binary.LittleEndian.AppendUint64(b, 7)
	if all != want(b) {
		t.Errorf("the hash of the integers is %x", all.AsBytes())
	}

	// stable_hash_base64 writes the u128 in big-endian order.
	h := stableHash128(func(h *hasher) { h.bytes(data) })
	hi, lo := h.AsU128()
	be := binary.BigEndian.AppendUint64(binary.BigEndian.AppendUint64(nil, hi), lo)
	if got := stableHashBase64(func(h *hasher) { h.bytes(data) }); got != base64.StdEncoding.EncodeToString(be) {
		t.Errorf("the base64 of the hash is %s", got)
	}
}

// TestSipHash checks that two types with the same fields get two hashes, as
// the type_id of upstream gives them, and that prehashed compares hashes.
func TestSipHash(t *testing.T) {
	a, b := hashedInt{1}, hashedOther{1}
	if sipHash(a) == sipHash(b) {
		t.Error("two types get the same hash")
	}
	if sipHash(a) != sipHash(hashedInt{1}) || sipHash(b) != sipHash(hashedOther{1}) {
		t.Error("a value gets two hashes")
	}
	c := hashedInt{2}
	if sipHash(a) == sipHash(c) {
		t.Error("two values get the same hash")
	}

	p, q, r := newPrehashed(a), newPrehashed(hashedInt{1}), newPrehashed(c)
	if !p.equal(q) || p.equal(r) {
		t.Error("prehashed compares the values in another way than the hashes")
	}
	h1, h2 := newHasher(), newHasher()
	p.write(h1)
	h2.u128(sipHash(a))
	if h1.finish128() != h2.finish128() {
		t.Error("a prehashed value does not write its hash")
	}
}

// TestGroupByKey checks SliceExt::group_by_key of krilla src/util.rs, as the
// widths of a CID font use it.
func TestGroupByKey(t *testing.T) {
	type group struct {
		key   int
		items []int
	}
	var got []group
	for k, items := range groupByKey([]int{1, 1, 2, 3, 3, 3, 1}, func(i int) int { return i }) {
		got = append(got, group{k, slices.Clone(items)})
	}
	want := []group{{1, []int{1, 1}}, {2, []int{2}}, {3, []int{3, 3, 3}}, {1, []int{1}}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].key != want[i].key || !slices.Equal(got[i].items, want[i].items) {
			t.Errorf("group %d is %v, want %v", i, got[i], want[i])
		}
	}
	for range groupByKey([]int(nil), func(i int) int { return i }) {
		t.Error("an empty slice gives a group")
	}
	n := 0
	for range groupByKey([]int{1, 2, 3}, func(i int) int { return i }) {
		n++
		break
	}
	if n != 1 {
		t.Errorf("a break gives %d groups", n)
	}
}

// TestCalculateStrokeBBox checks calculate_stroke_bbox of krilla
// src/util.rs on a line with butt caps.
func TestCalculateStrokeBBox(t *testing.T) {
	var pb path.PathBuilder
	pb.MoveTo(0, 0)
	pb.LineTo(10, 0)
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("no path")
	}
	stroke := path.DefaultStroke()
	stroke.Width = 2
	bbox, ok := calculateStrokeBBox(&stroke, p)
	if !ok {
		t.Fatal("no bounds")
	}
	if bbox.Left() != 0 || bbox.Top() != -1 || bbox.Right() != 10 || bbox.Bottom() != 1 {
		t.Errorf("the bounds are %s", bbox)
	}
	stroke.Width = -1
	if _, ok := calculateStrokeBBox(&stroke, p); ok {
		t.Error("a negative width gives bounds")
	}
}

// TestGeom checks the functions of krilla src/geom.rs.
func TestGeom(t *testing.T) {
	r, _ := path.RectFromLTRB(1, 2, 3, 4)
	o, _ := path.RectFromLTRB(-1, 3, 2, 10)
	rectExpand(&r, o)
	if r.Left() != -1 || r.Top() != 2 || r.Right() != 3 || r.Bottom() != 10 {
		t.Errorf("the union is %s", r)
	}
	if got := rectToPDFRect(r); got != pdfwriter.NewRect(-1, 2, 3, 10) {
		t.Errorf("the PDF rectangle is %v", got)
	}
	q := QuadrilateralFromRect(r)
	want := Quadrilateral{{X: -1, Y: 10}, {X: 3, Y: 10}, {X: 3, Y: 2}, {X: -1, Y: 2}}
	if q != want {
		t.Errorf("the quadrilateral is %v", q)
	}
	ts := path.TransformFromRow(1, 2, 3, 4, 5, 6)
	if got := transformToPDF(ts); got != [6]float32{1, 2, 3, 4, 5, 6} {
		t.Errorf("the transform is %v", got)
	}
	h1, h2 := newHasher(), newHasher()
	h1.transform(ts)
	for _, v := range []uint32{0x40a00000, 0x40c00000, 0x3f800000, 0x40800000, 0x40400000, 0x40000000} {
		h2.u32(v)
	}
	if h1.finish128() != h2.finish128() {
		t.Error("the hash of a transform is not tx, ty, sx, sy, kx, ky")
	}
	if n, ok := NewNormalizedF32(0.5); !ok || n != 0.5 {
		t.Errorf("NewNormalizedF32(0.5) gives %v, %t", n, ok)
	}
	if _, ok := NewNormalizedF32(1.5); ok {
		t.Error("NewNormalizedF32(1.5) is valid")
	}
	if NormalizedF32Zero != 0 || NormalizedF32One != 1 {
		t.Error("the constants of NormalizedF32 are wrong")
	}
}
