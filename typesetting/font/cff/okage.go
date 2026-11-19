// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package cff

// okage: this file is not in go-text. It holds the additions of D59: the
// predefined charsets, the seac of endchar and the FontMatrix of each font
// dictionary.

import (
	"math"

	ps "github.com/xo/okage/typesetting/font/cff/interpreter"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// The predefined charsets of CFF, in Appendix C of Adobe Technical Note
// 5176, The Compact Font Format Specification. Each one holds the SID of
// each glyph, indexed by glyph ID. A Top DICT selects one with the charset
// offset 0, 1 or 2. Each variable is a copy, so a change to it does not
// change how the package reads a font.
//
// okage: go-text keeps these tables unexported (D59).
var (
	// CharsetISOAdobe is the charset with the offset 0. The SID of each glyph
	// is its glyph ID.
	CharsetISOAdobe = charsetISOAdobe
	// CharsetExpert is the charset with the offset 1.
	CharsetExpert = charsetExpert
	// CharsetExpertSubset is the charset with the offset 2.
	CharsetExpertSubset = charsetExpertSubset
)

// standardEncoding holds the SID of each code of StandardEncoding, in
// Appendix B of Adobe Technical Note 5176. The SID 0 marks a code without a
// glyph. The seac of endchar names its two glyphs with these codes.
var standardEncoding = [256]uint16{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
	17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32,
	33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48,
	49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60, 61, 62, 63, 64,
	65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
	81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110,
	0, 111, 112, 113, 114, 0, 115, 116, 117, 118, 119, 120, 121, 122, 0, 123,
	0, 124, 125, 126, 127, 128, 129, 130, 131, 0, 132, 133, 0, 134, 135, 136,
	137, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 138, 0, 139, 0, 0, 0, 0, 140, 141, 142, 143, 0, 0, 0, 0,
	0, 144, 0, 0, 0, 145, 0, 0, 146, 147, 148, 149, 0, 0, 0, 0,
}

// UnitsPerEm returns the number of units in one em of the outlines that
// LoadGlyph returns. It is 1 divided by the y scale of the FontMatrix of the
// Top DICT, as FreeType computes it, and 1000 for a font without a
// FontMatrix in its Top DICT. In an OpenType font, it is usually the
// unitsPerEm of the head table.
//
// okage: go-text has no such method (D59).
func (f *CFF) UnitsPerEm() float64 {
	if f.unitsPerEm == 0 {
		return 1000
	}
	return f.unitsPerEm
}

// fontMatrix is a FontMatrix of a DICT: xx, yx, xy, yy, dx and dy, as the
// operands of the operator.
type fontMatrix [6]float64

// defaultFontMatrix is the FontMatrix of a Top DICT that gives none.
var defaultFontMatrix = fontMatrix{0.001, 0, 0, 0.001, 0, 0}

// valid tells whether the matrix is finite and can be inverted.
func (m fontMatrix) valid() bool {
	for _, v := range m {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return float64(m[0]*m[3])-float64(m[1]*m[2]) != 0
}

// scale returns the scale that FreeType uses to normalize the matrix: the
// absolute value of yy, or of yx if yy is 0.
func (m fontMatrix) scale() float64 {
	if m[3] != 0 {
		return math.Abs(m[3])
	}
	return math.Abs(m[1])
}

// concat returns the matrix of a font dictionary, sub, followed by the
// linear part of the matrix of the Top DICT, m. The translation of m does
// not apply, as in FreeType and skrifa.
//
// okage: the methods of fontMatrix convert each product, so that the Go
// compiler cannot fuse it with an add, and every platform gives the same
// result (D16).
func (m fontMatrix) concat(sub fontMatrix) fontMatrix {
	return fontMatrix{
		float64(m[0]*sub[0]) + float64(m[2]*sub[1]),
		float64(m[1]*sub[0]) + float64(m[3]*sub[1]),
		float64(m[0]*sub[2]) + float64(m[2]*sub[3]),
		float64(m[1]*sub[2]) + float64(m[3]*sub[3]),
		float64(m[0]*sub[4]) + float64(m[2]*sub[5]),
		float64(m[1]*sub[4]) + float64(m[3]*sub[5]),
	}
}

// isIdentity tells whether the matrix leaves each point unchanged.
func (m fontMatrix) isIdentity() bool {
	return m == fontMatrix{1, 0, 0, 1, 0, 0}
}

// apply returns the point p moved by the matrix.
func (m fontMatrix) apply(p ot.SegmentPoint) ot.SegmentPoint {
	x, y := float64(p.X), float64(p.Y)
	return ot.SegmentPoint{
		X: float32(float64(m[0]*x) + float64(m[2]*y) + m[4]),
		Y: float32(float64(m[1]*x) + float64(m[3]*y) + m[5]),
	}
}

// transform applies the matrix to the segments and to their bounds. A
// matrix with a rotation or a skew gives the bounds of the four corners of
// the bounds, not of the points of the segments.
func (m fontMatrix) transform(segments []ot.Segment, bounds *ps.PathBounds) {
	if m.isIdentity() {
		return
	}
	for i := range segments {
		for j := range segments[i].ArgsSlice() {
			segments[i].Args[j] = m.apply(segments[i].Args[j])
		}
	}
	if !hasOutline(segments) {
		return
	}
	corners := [4]ps.Point{
		bounds.Min, {X: bounds.Max.X, Y: bounds.Min.Y},
		bounds.Max, {X: bounds.Min.X, Y: bounds.Max.Y},
	}
	var out ps.PathBounds
	for i, c := range corners {
		q := m.apply(ot.SegmentPoint{X: float32(c.X), Y: float32(c.Y)})
		pt := ps.Point{X: float64(q.X), Y: float64(q.Y)}
		if i == 0 {
			out.Min, out.Max = pt, pt
			continue
		}
		out.Enlarge(pt)
	}
	*bounds = out
}

// fontTransforms returns the transform of each font dictionary of a font,
// and the units per em of its outlines. top is the Top DICT. fds holds the
// font dictionaries of a CIDFont, and is nil for another font.
//
// The transform maps a point of a charstring to the units of the font, as
// FreeType does for a CFF font. The FontMatrix of a font dictionary applies
// first, then the linear part of the FontMatrix of the Top DICT. The result
// is divided by the scale of the Top DICT, so that the transform of a font
// with only the FontMatrix 0.001 0 0 0.001 0 0 is the identity. A
// FontMatrix that is not finite, or that cannot be inverted, does not apply.
//
// FreeType divides by the unitsPerEm of the head table of an OpenType font
// whose Top DICT has no FontMatrix, and this function divides by 0.001.
// The two differ only for a font dictionary with a FontMatrix in such a
// font.
func fontTransforms(top topDict, fds []topDict) ([]fontMatrix, float64) {
	topMatrix, hasTop := defaultFontMatrix, false
	if top.hasFontMatrix && top.fontMatrix.valid() && top.fontMatrix.scale() != 0 {
		topMatrix, hasTop = top.fontMatrix, true
	}
	unitsPerEm := 1 / topMatrix.scale()
	// A transform that is not finite leaves the outlines of go-text.
	scale := func(m fontMatrix) fontMatrix {
		for i := range m {
			m[i] *= unitsPerEm
		}
		if !m.valid() {
			return fontMatrix{1, 0, 0, 1, 0, 0}
		}
		return m
	}
	if fds == nil {
		return []fontMatrix{scale(topMatrix)}, unitsPerEm
	}
	out := make([]fontMatrix, len(fds))
	for i, fd := range fds {
		m := topMatrix
		if fd.hasFontMatrix && fd.fontMatrix.valid() {
			m = fd.fontMatrix
			if hasTop {
				m = topMatrix.concat(fd.fontMatrix)
			}
		}
		out[i] = scale(m)
	}
	return out, unitsPerEm
}

// hasOutline tells whether the segments draw a line or a curve.
func hasOutline(segments []ot.Segment) bool {
	for _, s := range segments {
		if s.Op != ot.SegmentOpMoveTo {
			return true
		}
	}
	return false
}

// standardGlyph returns the glyph of a code of StandardEncoding: the first
// glyph whose SID in the charset is the SID of the code.
func (f *CFF) standardGlyph(code float64) (tables.GlyphID, bool) {
	if code < 0 || code > 255 {
		return 0, false
	}
	sid := standardEncoding[int(code)]
	if sid == 0 {
		return 0, false
	}
	for g, s := range f.charset {
		if s == sid && g < len(f.Charstrings) {
			return tables.GlyphID(g), true
		}
	}
	return 0, false
}

// loadSeac adds the base glyph and the accent of a seac to the outline of
// the loader, in the order of the Type 2 specification and of FreeType: the
// base glyph at the origin, then the accent at adx, ady. A code without a
// glyph, or a component that does not load, adds nothing. A component that
// is itself a seac adds only its own outline.
func (f *CFF) loadSeac(loader *type2CharstringHandler) {
	adx, ady, bchar, achar := loader.seacArgs[0], loader.seacArgs[1], loader.seacArgs[2], loader.seacArgs[3]
	components := [2]struct {
		code   float64
		dx, dy float64
	}{{bchar, 0, 0}, {achar, adx, ady}}
	for _, c := range components {
		glyph, ok := f.standardGlyph(c.code)
		if !ok {
			continue
		}
		var (
			psi  ps.Machine
			part type2CharstringHandler
		)
		if err := psi.Run(f.Charstrings[glyph], f.localSubrs[0], f.globalSubrs, &part); err != nil {
			continue
		}
		move := fontMatrix{1, 0, 0, 1, c.dx, c.dy}
		move.transform(part.cs.Segments, &part.cs.Bounds)
		if hasOutline(part.cs.Segments) {
			if hasOutline(loader.cs.Segments) {
				loader.cs.Bounds.Enlarge(part.cs.Bounds.Min)
				loader.cs.Bounds.Enlarge(part.cs.Bounds.Max)
			} else {
				loader.cs.Bounds = part.cs.Bounds
			}
		}
		loader.cs.Segments = append(loader.cs.Segments, part.cs.Segments...)
	}
}
