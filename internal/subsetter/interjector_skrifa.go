// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"cmp"
	"encoding/binary"
	"math"
	"slices"

	"github.com/xo/okage/kurbo"
	tsfont "github.com/xo/okage/typesetting/font"
	tscff "github.com/xo/okage/typesetting/font/cff"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file ports the rest of the module skrifa of subsetter
// src/interjector.rs: SkrifaInterjector::interject and OutlinePath. It sets
// the seam interjectGlyph of lib.go. doc.go describes the seams.
//
// Upstream draws each glyph with the outline pen of skrifa, at the location
// of the interjector, and takes the advance from the glyph metrics of
// skrifa. The port reads the glyph with typesetting, which applies the
// variations (D52, D62):
//
//   - A TrueType glyph comes from Face.GlyphDataOutline, with the deltas of
//     gvar. The port finds the point of the glyph behind each point of the
//     outline, rounds the points to integers as skrifa does, and draws them
//     as skrifa draws a TrueType outline in the style of FreeType.
//   - A CFF2 glyph comes from CFF2.LoadGlyph, with the blends at the
//     location. The port gives its commands to the pen as skrifa does for an
//     outline without a scale: it truncates each point to an integer, and it
//     drops a move without a line and a line of length 0.
//   - The advance comes from Face.HorizontalAdvance, with the delta of HVAR,
//     or the deltas of gvar of the phantom points. The port rounds the delta
//     to an integer, as skrifa does.
//
// typesetting computes the deltas in float32, where skrifa computes them in
// the fixed-point numbers of OpenType. A point whose delta is very near half
// a unit can round to the other integer, so a point of an instance can differ
// by 1 unit from the point of upstream. At the default location, the outlines
// have no delta, and the subsets are the same, byte for byte. Item 5 of
// "Differences from upstream" in doc.go says more.

func init() {
	interjectGlyph = interject
}

// interject is SkrifaInterjector::interject in subsetter
// src/interjector.rs. It returns the glyph description in the glyf outline
// format, with the advance and the left side bearing of the glyph, and false
// where upstream returns None.
func interject(s *skrifaInterjector, maxp *maxpData, glyph uint16) (uint16, int16, []byte, bool) {
	inst := s.instance()

	var outlineBuilder outlinePath
	if !inst.draw(glyph, &outlineBuilder) {
		return 0, 0, nil, false
	}

	path := outlineBuilder.path

	sg, malformed := simpleGlyphFromBezPath(path)
	if malformed != malformedPathNone {
		return 0, 0, nil, false
	}
	advanceWidth, ok := inst.advanceWidth(glyph)
	if !ok {
		return 0, 0, nil, false
	}
	advance := f32RoundToU16(advanceWidth)

	// We derive the LSB from the resulting bounding box rather than from the
	// font's metrics, because the latter does not always agree with the
	// `xMin` of the fresh outline we've generated.
	//
	// The OpenType spec heavily advises xMin and LSB to match (it actually
	// requires it for variable fonts or when `head.flags` bit 1 is set).
	//
	// If `LSB != xMin`, glyphs get repositioned by PDF readers and the
	// kerning gets very wonky.
	lsb := sg.bbox.xMin

	if path.isEmpty() {
		return advance, lsb, []byte{}, true
	}

	var points uint16
	for _, c := range sg.contours {
		// The sum wraps, as it does in a release build of Rust.
		points += uint16(len(c))
	}
	maxp.maxPoints = max(maxp.maxPoints, points)
	maxp.maxContours = max(maxp.maxContours, uint16(len(sg.contours)))

	data, ok := sg.dump()
	if !ok {
		return 0, 0, nil, false
	}
	return advance, lsb, data, true
}

// f32RoundToU16 is f32::round followed by the conversion as u16 of Rust: it
// rounds half away from zero, and saturates. NaN gives 0.
func f32RoundToU16(v float32) uint16 {
	r := math.Round(float64(v))
	switch {
	case r != r || r <= 0:
		return 0
	case r >= math.MaxUint16:
		return math.MaxUint16
	}
	return uint16(r)
}

// outlinePath is OutlinePath in subsetter src/interjector.rs: the outline
// pen of skrifa that builds the path of a glyph.
type outlinePath struct {
	lastMoveTo [2]float32
	lastPoint  [2]float32
	path       bezPath
}

// pointF32 converts a point of the pen to a point of kurbo, as From<(f32,
// f32)> for Point of kurbo does.
func pointF32(x, y float32) kurbo.Point {
	return kurbo.NewPoint(float64(x), float64(y))
}

// moveTo is OutlinePen::move_to for OutlinePath in subsetter
// src/interjector.rs.
func (o *outlinePath) moveTo(x, y float32) {
	o.path.moveTo(pointF32(x, y))
	o.lastMoveTo = [2]float32{x, y}
	o.lastPoint = [2]float32{x, y}
}

// lineTo is OutlinePen::line_to for OutlinePath in subsetter
// src/interjector.rs.
func (o *outlinePath) lineTo(x, y float32) {
	o.path.lineTo(pointF32(x, y))
	o.lastPoint = [2]float32{x, y}
}

// quadTo is OutlinePen::quad_to for OutlinePath in subsetter
// src/interjector.rs.
func (o *outlinePath) quadTo(cx, cy, x, y float32) {
	// Only called by TrueType fonts.
	o.path.quadTo(pointF32(cx, cy), pointF32(x, y))
	o.lastPoint = [2]float32{x, y}
}

// curveTo is OutlinePen::curve_to for OutlinePath in subsetter
// src/interjector.rs.
func (o *outlinePath) curveTo(cx0, cy0, cx1, cy1, x, y float32) {
	// Only called by CFF2 fonts.
	cubic := kurbo.NewCubicBez(
		pointF32(o.lastPoint[0], o.lastPoint[1]),
		pointF32(cx0, cy0),
		pointF32(cx1, cy1),
		pointF32(x, y),
	)

	// It is not entirely clear how small the `accuracy` parameter needs to
	// be to produce sensible results. In `vello_cpu`, a value of around 0.025
	// is used (0.25 for the flattening accuracy and * 0.1 in the flattening
	// code, so we choose the same value here.
	quads := cubic.ToQuads(0.025)
	for {
		_, _, quad, ok := quads.Next()
		if !ok {
			break
		}
		// Note that `quad.p2` is the same as `quad.p0` of the next point in
		// the iterator.
		o.quadTo(float32(quad.P1.X), float32(quad.P1.Y), float32(quad.P2.X), float32(quad.P2.Y))
	}
}

// close is OutlinePen::close for OutlinePath in subsetter
// src/interjector.rs.
func (o *outlinePath) close() {
	o.path.closePath()
	o.lastPoint = o.lastMoveTo
}

// glyphInstance holds what interject reads of the font, which upstream reads
// with skrifa: the outlines and the metrics at the location of the
// interjector. skrifaInterjector.instance makes it once for each
// interjector.
type glyphInstance struct {
	// glyf tells whether skrifa draws the outlines of the glyf table. When
	// it is false and cff2 is not nil, skrifa draws the outlines of the CFF2
	// table. When both are unset, the font has no outline that skrifa
	// draws, and each glyph is empty.
	glyf bool
	// glyphs holds the glyphs of the glyf table, as typesetting reads them.
	glyphs tables.Glyf
	cff2   *tscff.CFF2

	// face draws the glyphs and gives the advances at the location, and
	// base gives the advances at the default location. They are nil when
	// typesetting cannot read the font.
	face, base *tsfont.Face
	// coords holds the normalized coordinates, or nil at the default
	// location, as LocationRef::effective_coords of skrifa gives them.
	coords []tables.Coord

	// The metrics of GlyphMetrics of skrifa src/metrics.rs.
	glyphCount          uint32
	hMetrics            []uint16
	defaultAdvanceWidth uint16
	hasHVAR, hasGvar    bool

	// floats and rounded hold the results of floatPoints and glyfPoints
	// for each glyph.
	floats, rounded map[tables.GlyphID]glyfPointsResult
}

// instance returns the glyph instance of the interjector, and makes it on
// the first call.
func (s *skrifaInterjector) instance() *glyphInstance {
	if s.inst == nil {
		s.inst = newGlyphInstance(s)
	}
	return s.inst
}

// The tags of the tables that the glyph instance reads.
var (
	tagFVARInst = Tag{'f', 'v', 'a', 'r'}
	tagAVARInst = Tag{'a', 'v', 'a', 'r'}
	tagGVARInst = Tag{'g', 'v', 'a', 'r'}
	tagHVARInst = Tag{'H', 'V', 'A', 'R'}
)

// newGlyphInstance reads the font of the interjector for interject.
func newGlyphInstance(s *skrifaInterjector) *glyphInstance {
	ref := s.fontRef
	inst := &glyphInstance{}
	if !s.isDefaultLocation() {
		inst.coords = s.location
	}

	// GlyphMetrics::new of skrifa src/metrics.rs.
	if maxp, ok := ref.table(tagMAXP); ok && len(maxp) >= 6 {
		inst.glyphCount = uint32(binary.BigEndian.Uint16(maxp[4:]))
	}
	hhea, okHhea := ref.table(tagHHEA)
	hmtx, okHmtx := ref.table(tagHMTX)
	if okHhea && okHmtx && len(hhea) >= 36 {
		n := int(binary.BigEndian.Uint16(hhea[34:]))
		if 4*n <= len(hmtx) {
			inst.hMetrics = make([]uint16, n)
			for i := range inst.hMetrics {
				inst.hMetrics[i] = binary.BigEndian.Uint16(hmtx[4*i:])
			}
			if n > 0 {
				inst.defaultAdvanceWidth = inst.hMetrics[n-1]
			}
		}
	}
	if raw, ok := ref.table(tagHVARInst); ok {
		if _, _, err := tables.ParseHVAR(raw); err == nil {
			inst.hasHVAR = true
		}
	}
	if raw, ok := ref.table(tagGVARInst); ok {
		if _, _, err := tables.ParseGvar(raw); err == nil {
			inst.hasGvar = true
		}
	}

	// OutlineGlyphCollection::new of skrifa src/outline/mod.rs takes the
	// glyf table before the CFF2 table.
	head, okHead := ref.table(tagHEAD)
	glyf, okGlyf := ref.table(tagGLYF)
	loca, okLoca := ref.table(tagLOCA)
	if okHead && okGlyf && okLoca && okHmtx {
		if h, _, err := tables.ParseHead(head); err == nil {
			inst.glyf = true
			offsets, err := tables.ParseLoca(loca, int(inst.glyphCount), h.IndexToLocFormat == 1)
			if err == nil {
				inst.glyphs, _ = tables.ParseGlyf(glyf, offsets)
			}
		}
	}
	if !inst.glyf {
		if raw, ok := ref.table(tagCFF2); ok {
			inst.cff2, _ = tscff.ParseCFF2(raw)
		}
	}

	// typesetting reads a font from a font file. The port gives it a font
	// file with the tables that it needs, and with an empty cmap table, which
	// it requires. So the glyph instance reads the same tables as the rest of
	// the subsetter, also in a font collection.
	tags := []Tag{tagHEAD, tagMAXP, tagHHEA, tagHMTX, tagFVARInst, tagAVARInst, tagHVARInst}
	if inst.glyf {
		tags = append(tags, tagLOCA, tagGLYF, tagGVARInst)
	}
	tbls := []ot.Table{{Tag: ot.NewTag('c', 'm', 'a', 'p'), Content: emptyCmap}}
	for _, tag := range tags {
		if data, ok := ref.table(tag); ok {
			tbls = append(tbls, ot.Table{Tag: ot.NewTag(tag[0], tag[1], tag[2], tag[3]), Content: data})
		}
	}
	slices.SortFunc(tbls, func(a, b ot.Table) int { return cmp.Compare(a.Tag, b.Tag) })
	ld, err := ot.NewLoader(ot.NewSlicedResource(ot.WriteTTF(tbls)))
	if err != nil {
		return inst
	}
	fnt, err := tsfont.NewFont(ld)
	if err != nil {
		return inst
	}
	inst.face = tsfont.NewFace(fnt)
	inst.face.SetCoords(inst.coords)
	inst.base = tsfont.NewFace(fnt)
	return inst
}

// emptyCmap is a cmap table of version 0 with a format 4 subtable that maps
// no character, for the font file that the glyph instance gives to
// typesetting.
var emptyCmap = []byte{
	0, 0, 0, 1, // version, numTables
	0, 3, 0, 1, 0, 0, 0, 12, // platformID, encodingID, offset
	0, 4, 0, 24, 0, 0, // format, length, language
	0, 2, 0, 2, 0, 0, 0, 0, // segCountX2, searchRange, entrySelector, rangeShift
	0xFF, 0xFF, // endCode
	0, 0, // reservedPad
	0xFF, 0xFF, // startCode
	0, 1, // idDelta
	0, 0, // idRangeOffset
}

// draw is OutlineGlyph::draw of skrifa src/outline/mod.rs for the glyph, as
// interject calls it with the outline pen and the unhinted settings without
// a scale. A glyph without an outline draws nothing. It returns false where
// skrifa returns an error.
func (inst *glyphInstance) draw(glyph uint16, pen *outlinePath) bool {
	switch {
	case inst.glyf:
		return inst.drawGlyf(glyph, pen)
	case inst.cff2 != nil:
		return inst.drawCFF2(glyph, pen)
	}
	return true
}

// advanceWidth is GlyphMetrics::advance_width of skrifa src/metrics.rs,
// without a scale. It returns false where skrifa returns None, for a glyph
// that is not in the font.
func (inst *glyphInstance) advanceWidth(glyph uint16) (float32, bool) {
	if uint32(glyph) >= inst.glyphCount {
		return 0, false
	}
	advance := int32(inst.defaultAdvanceWidth)
	if int(glyph) < len(inst.hMetrics) {
		advance = int32(inst.hMetrics[glyph])
	}
	if inst.coords != nil && inst.face != nil && (inst.hasHVAR || inst.hasGvar) {
		// skrifa rounds the delta of HVAR to an integer in
		// ItemVariationStore::compute_delta of read-fonts, and then
		// truncates it, as FreeType truncates metric deltas. It rounds the
		// delta of the phantom points of gvar to an integer.
		delta := float64(inst.face.HorizontalAdvance(tsfont.GID(glyph)) - inst.base.HorizontalAdvance(tsfont.GID(glyph)))
		advance += int32(math.Floor(delta + 0.5))
	}
	return float32(advance), true
}

// flatPoint is a point of the outline of a TrueType glyph, after its
// components are put in its place: whether it is on the curve, whether it
// ends a contour, its coordinates as typesetting gives them, and its
// coordinates in the 26.6 format, as skrifa computes them.
type flatPoint struct {
	onCurve, endPoint bool
	set               bool
	fx, fy            float32
	x, y              int32
}

// glyfPointsResult is an entry of the cache of glyphInstance.floatPoints.
type glyfPointsResult struct {
	points []flatPoint
	ok     bool
}

// maxFlattenVisits is the largest number of glyphs that flattenGlyf visits
// for one glyph.
const maxFlattenVisits = 1 << 16

// maxCompositeNesting is maxCompositeNesting of typesetting
// font/glyphs.go: the depth of the components that typesetting reads.
const maxCompositeNesting = 20

// drawGlyf draws a TrueType glyph. It is FreeTypeScaler::unhinted and
// ScaledOutline::to_path of skrifa src/outline/glyf, on the points of the
// outline that typesetting gives. glyfPoints computes the points as skrifa
// does, and drawGlyf draws the contours in the style of FreeType.
func (inst *glyphInstance) drawGlyf(glyph uint16, pen *outlinePath) bool {
	if inst.face == nil {
		// typesetting cannot read the font.
		return false
	}
	if int(glyph) >= len(inst.glyphs) {
		// skrifa has no outline for the glyph, and draws nothing.
		return true
	}
	points, ok := inst.glyfPoints(tables.GlyphID(glyph), 0)
	// Outline::ensure_point_count_limit of skrifa src/outline/glyf/outline.rs,
	// with the 4 phantom points.
	if !ok || len(points)+4 > math.MaxUint16 {
		return false
	}
	start := 0
	for i, p := range points {
		if !p.endPoint {
			continue
		}
		contourToPathFreeType(points[start:i+1], pen)
		start = i + 1
	}
	return true
}

// floatPoints returns the points of the outline of a glyph that
// typesetting gives, with the deltas at the location.
//
// typesetting gives the outline as segments, in the style of HarfBuzz.
// flattenGlyf lists the points behind the segments, as typesetting lists
// them, and matchSegments finds the coordinates of each point in the
// segments. floatPoints keeps the result of each glyph, because the
// components of the composite glyphs repeat.
func (inst *glyphInstance) floatPoints(gid tables.GlyphID) ([]flatPoint, bool) {
	if r, ok := inst.floats[gid]; ok {
		return r.points, r.ok
	}
	var points []flatPoint
	budget := maxFlattenVisits
	ok := inst.flattenGlyf(gid, 0, map[tables.GlyphID]struct{}{}, &points, &budget)
	if ok {
		var outline tsfont.GlyphOutline
		outline, ok = inst.face.GlyphDataOutline(tsfont.GID(gid))
		ok = ok && matchSegments(points, outline.Segments)
	}
	if inst.floats == nil {
		inst.floats = map[tables.GlyphID]glyfPointsResult{}
	}
	inst.floats[gid] = glyfPointsResult{points, ok}
	return points, ok
}

// round26Dot6 rounds a coordinate to an integer, half up, as Fixed::to_i32
// of font-types rounds a delta, and gives it in the 26.6 format.
func round26Dot6(v float64) int32 {
	return int32(math.Floor(v+0.5)) << 6
}

// The flags of a component of a composite glyph, CompositeGlyphFlags in
// read-fonts src/tables/glyf.rs.
const (
	flagWeHaveAScale            = 0x0008
	flagWeHaveAnXAndYScale      = 0x0040
	flagWeHaveATwoByTwo         = 0x0080
	flagScaledComponentOffset   = 0x0800
	flagUnscaledComponentOffset = 0x1000
)

// mulFixed is impl Mul of font-types src/fixed.rs on the bits of two
// numbers: the product, shifted by 16 bits, rounded to the nearest, with
// ties away from zero.
func mulFixed(a, b int32) int32 {
	ab := int64(a) * int64(b)
	if ab < 0 {
		ab--
	}
	return int32((ab + 0x8000) >> 16)
}

// glyfPoints returns the points of the outline of a glyph, with their
// coordinates as FreeTypeScaler of skrifa src/outline/glyf/mod.rs computes
// them for an outline without a scale.
//
// skrifa adds the deltas to the points of each simple glyph, and rounds each
// sum to an integer. A composite glyph puts the points of each component at
// the offset of the component, which skrifa rounds to an integer with its
// delta. typesetting adds the deltas in float32 and rounds nothing, so a
// coordinate of a composite glyph is not always the rounded coordinate that
// typesetting gives. glyfPoints rounds the points of each component as
// skrifa does, and finds the offset of the component with its delta from the
// points that typesetting gives for the composite glyph and for the
// component. It then transforms the component and adds the rounded offset
// as skrifa does in FreeTypeScaler::load_composite.
//
// When the components that typesetting gives for the composite glyph are not
// the components alone, as in a font whose components hold each other, it
// rounds the points that typesetting gives.
func (inst *glyphInstance) glyfPoints(gid tables.GlyphID, depth int) ([]flatPoint, bool) {
	if int(gid) >= len(inst.glyphs) {
		return nil, true
	}
	if r, ok := inst.rounded[gid]; ok {
		return r.points, r.ok
	}
	points, ok := inst.glyfPointsOnce(gid, depth)
	if inst.rounded == nil {
		inst.rounded = map[tables.GlyphID]glyfPointsResult{}
	}
	inst.rounded[gid] = glyfPointsResult{points, ok}
	return points, ok
}

// glyfPointsOnce is glyfPoints for a glyph that it has not computed yet.
func (inst *glyphInstance) glyfPointsOnce(gid tables.GlyphID, depth int) ([]flatPoint, bool) {
	pts, ok := inst.floatPoints(gid)
	if !ok {
		return nil, false
	}
	out := slices.Clone(pts)
	rounded := func() ([]flatPoint, bool) {
		for i := range out {
			out[i].x, out[i].y = round26Dot6(float64(out[i].fx)), round26Dot6(float64(out[i].fy))
		}
		return out, true
	}
	composite, isComposite := inst.glyphs[gid].Data.(tables.CompositeGlyph)
	if !isComposite || depth >= maxCompositeNesting {
		return rounded()
	}
	at := 0
	for _, item := range composite.Glyphs {
		child, ok := inst.glyfPoints(item.GlyphIndex, depth+1)
		if !ok {
			return nil, false
		}
		n := len(child)
		if at+n > len(out) {
			return rounded()
		}
		// Prepares the transform components for our conversion math below.
		scale := func(i int) int32 { return int32(item.Scale[i]*16384) * 4 }
		xx, yx, xy, yy := scale(0), scale(1), scale(2), scale(3)
		haveXform := item.Flags&(flagWeHaveAScale|flagWeHaveAnXAndYScale|flagWeHaveATwoByTwo) != 0
		for i, p := range child {
			q := &out[at+i]
			q.x, q.y = p.x, p.y
			if haveXform {
				// This juggling is necessary because, unlike FreeType, skrifa
				// also returns unscaled outlines in 26.6 format.
				ux, uy := (p.x+32)>>6, (p.y+32)>>6
				q.x = (mulFixed(ux, xx) + mulFixed(uy, xy)) << 6
				q.y = (mulFixed(ux, yx) + mulFixed(uy, yy)) << 6
			}
		}
		var offX, offY int32
		switch {
		case item.IsAnchored():
			base, component := item.ArgsAsIndices()
			if base >= at || component >= n {
				return nil, false
			}
			offX, offY = out[base].x-out[at+component].x, out[base].y-out[at+component].y
		case n > 0:
			argX, argY := item.ArgsAsTranslation()
			x, y := int32(argX), int32(argY)
			// The translation that typesetting applies.
			tx, ty := float64(argX), float64(argY)
			m := item.Scale
			if item.IsScaledOffsets() {
				tx, ty = tx*float64(m[0])+ty*float64(m[2]), tx*float64(m[1])+ty*float64(m[3])
			}
			if haveXform && item.Flags&(flagScaledComponentOffset|flagUnscaledComponentOffset) == flagScaledComponentOffset {
				// According to FreeType, this algorithm is a "guess" and
				// works better than the one documented by Apple.
				hypot := func(a, b int32) int32 {
					a, b = max(a, -a), max(b, -b)
					if a > b {
						return a + ((3 * b) >> 3)
					}
					return b + ((3 * a) >> 3)
				}
				// FreeType uses a fixed point multiplication here.
				x = mulFixed(x, hypot(xx, xy))
				y = mulFixed(y, hypot(yy, yx))
			}
			// The offset of typesetting holds the translation, the delta of
			// the component, and the shift of the phantom points of the
			// composite glyph and of the component. skrifa rounds off the
			// fractional parts of the delta, as FreeType does.
			fx, fy := float64(child[0].fx), float64(child[0].fy)
			ox := float64(out[at].fx) - (fx*float64(m[0]) + fy*float64(m[2]))
			oy := float64(out[at].fy) - (fx*float64(m[1]) + fy*float64(m[3]))
			offX = x<<6 + round26Dot6(ox-tx)
			offY = y<<6 + round26Dot6(oy-ty)
		}
		for i := range n {
			out[at+i].x += offX
			out[at+i].y += offY
		}
		at += n
	}
	if at != len(out) {
		return rounded()
	}
	return out, true
}

// flattenGlyf lists the points of the outline of a glyph, as
// getPointsForGlyphRec of typesetting font/glyphs.go lists them, without the
// phantom points. A composite glyph gives the points of its components in
// order. A component that is too deep, that is missing, or that is in the
// chain of components that holds it gives no point.
//
// It returns false when a simple glyph has an end point of a contour that is
// less than the end point before it. skrifa returns an error for such a
// glyph in FreeTypeScaler::load_simple, and typesetting panics on it.
//
// It also returns false when the glyph has more than u16::MAX points, as
// skrifa does, or when it visits more than budget glyphs. Components that
// repeat other components can make a number of visits that grows
// exponentially with the depth, and typesetting also visits them.
func (inst *glyphInstance) flattenGlyf(gid tables.GlyphID, depth int, current map[tables.GlyphID]struct{}, out *[]flatPoint, budget *int) bool {
	if depth > maxCompositeNesting || int(gid) >= len(inst.glyphs) {
		return true
	}
	*budget--
	if *budget < 0 || len(*out) > math.MaxUint16 {
		return false
	}
	switch data := inst.glyphs[gid].Data.(type) {
	case tables.SimpleGlyph:
		start := len(*out)
		for _, p := range data.Points {
			*out = append(*out, flatPoint{onCurve: p.Flag&flagOnCurvePoint != 0})
		}
		last := 0
		for _, end := range data.EndPtsOfContours {
			if int(end) < last || int(end) >= len(data.Points) {
				return false
			}
			last = int(end)
			(*out)[start+last].endPoint = true
		}
	case tables.CompositeGlyph:
		for _, item := range data.Glyphs {
			if _, has := current[item.GlyphIndex]; has {
				continue
			}
			current[item.GlyphIndex] = struct{}{}
			ok := inst.flattenGlyf(item.GlyphIndex, depth+1, current, out, budget)
			delete(current, item.GlyphIndex)
			if !ok {
				return false
			}
		}
	}
	return true
}

// matchSegments finds the coordinates of each point of a glyph in the
// segments that buildSegments of typesetting font/renderer.go makes of them.
// It follows buildSegments, and takes each point from the argument of the
// segment that holds it. It returns false when the segments do not come from
// the points.
func matchSegments(points []flatPoint, segs []ot.Segment) bool {
	const midpoint = -1
	k := 0
	emit := func(op ot.SegmentOp, args ...int) bool {
		if k >= len(segs) || segs[k].Op != op {
			return false
		}
		for j, a := range args {
			if a == midpoint {
				continue
			}
			p := segs[k].Args[j]
			points[a].fx, points[a].fy = p.X, p.Y
			points[a].set = true
		}
		k++
		return true
	}

	var firstOnCurveValid, firstOffCurveValid, lastOffCurveValid bool
	var firstOffCurve, lastOffCurve int
	for i, point := range points {
		switch {
		case !firstOnCurveValid:
			switch {
			case point.onCurve:
				firstOnCurveValid = true
				if !emit(ot.SegmentOpMoveTo, i) {
					return false
				}
			case !firstOffCurveValid:
				firstOffCurve = i
				firstOffCurveValid = true
				if !point.endPoint {
					continue
				}
			default:
				firstOnCurveValid = true
				lastOffCurve = i
				lastOffCurveValid = true
				if !emit(ot.SegmentOpMoveTo, midpoint) {
					return false
				}
			}
		case !lastOffCurveValid:
			if !point.onCurve {
				lastOffCurve = i
				lastOffCurveValid = true
				if !point.endPoint {
					continue
				}
			} else if !emit(ot.SegmentOpLineTo, i) {
				return false
			}
		default:
			if !point.onCurve {
				if !emit(ot.SegmentOpQuadTo, lastOffCurve, midpoint) {
					return false
				}
				lastOffCurve = i
				lastOffCurveValid = true
			} else {
				if !emit(ot.SegmentOpQuadTo, lastOffCurve, i) {
					return false
				}
				lastOffCurveValid = false
			}
		}

		if point.endPoint {
			// closing the contour
			var ok bool
			switch {
			case !firstOffCurveValid && !lastOffCurveValid:
				ok = emit(ot.SegmentOpLineTo, midpoint)
			case !firstOffCurveValid && lastOffCurveValid:
				ok = emit(ot.SegmentOpQuadTo, lastOffCurve, midpoint)
			case firstOffCurveValid && !lastOffCurveValid:
				ok = emit(ot.SegmentOpQuadTo, firstOffCurve, midpoint)
			default:
				ok = emit(ot.SegmentOpQuadTo, lastOffCurve, midpoint) &&
					emit(ot.SegmentOpQuadTo, firstOffCurve, midpoint)
			}
			if !ok {
				return false
			}
			firstOnCurveValid = false
			firstOffCurveValid = false
			lastOffCurveValid = false
		}
	}
	if k != len(segs) {
		return false
	}
	// Each point of a contour is in a segment. The points after the last end
	// point make no segment, and skrifa does not draw them either.
	last := -1
	for i, p := range points {
		if p.endPoint {
			last = i
		}
	}
	for _, p := range points[:last+1] {
		if !p.set {
			return false
		}
	}
	return true
}

// midpoint26Dot6 is PointCoord::midpoint for F26Dot6 in read-fonts
// src/tables/glyf.rs: the midpoint in the 26.6 format, with the integer
// division of FreeType. The addition wraps.
func midpoint26Dot6(a, b int32) int32 {
	return (a + b) / 2
}

// f32From26Dot6 is F26Dot6::to_f32 in font-types.
func f32From26Dot6(v int32) float32 {
	return float32(v) * (1.0 / 64.0)
}

// contourToPathFreeType is contour_to_path_freetype of skrifa
// src/outline/path.rs for a contour of quadratic curves.
func contourToPathFreeType(points []flatPoint, pen *outlinePath) {
	if len(points) == 0 {
		return
	}
	firstPoint, lastPoint := points[0], points[len(points)-1]
	// For FreeType style, we may need to omit the last point if we find the
	// first on curve there
	rest := points
	// Find our starting point
	var startPoint flatPoint
	switch {
	case firstPoint.onCurve:
		// We're starting with an on curve, so consume the point
		rest = points[1:]
		startPoint = firstPoint
	case lastPoint.onCurve:
		// The last point is an on curve, so let's start there
		rest = points[:len(points)-1]
		startPoint = lastPoint
	default:
		// It's also an off curve, so take implicit midpoint
		startPoint = flatPoint{x: midpoint26Dot6(lastPoint.x, firstPoint.x), y: midpoint26Dot6(lastPoint.y, firstPoint.y)}
	}
	pen.moveTo(f32From26Dot6(startPoint.x), f32From26Dot6(startPoint.y))

	// PendingState of skrifa src/outline/path.rs, without the cubic curves.
	var pending flatPoint
	havePending := false
	emit := func(point flatPoint) {
		switch {
		case !havePending && !point.onCurve:
			pending, havePending = point, true
		case !havePending:
			pen.lineTo(f32From26Dot6(point.x), f32From26Dot6(point.y))
		case !point.onCurve:
			mid := flatPoint{x: midpoint26Dot6(pending.x, point.x), y: midpoint26Dot6(pending.y, point.y)}
			pen.quadTo(f32From26Dot6(pending.x), f32From26Dot6(pending.y), f32From26Dot6(mid.x), f32From26Dot6(mid.y))
			pending = point
		default:
			pen.quadTo(f32From26Dot6(pending.x), f32From26Dot6(pending.y), f32From26Dot6(point.x), f32From26Dot6(point.y))
			havePending = false
		}
	}
	for _, p := range rest {
		emit(p)
	}
	// PendingState::finish: we always want to end with an explicit on-curve.
	if havePending {
		startPoint.onCurve = true
		emit(startPoint)
	}
	pen.close()
}

// drawCFF2 draws a CFF2 glyph. It is Outlines::draw of skrifa
// src/outline/cff/mod.rs, without hinting and without a scale, on the
// segments that CFF2.LoadGlyph of typesetting gives at the location.
//
// skrifa evaluates the char string with read-fonts, and gives its commands
// to a TransformSink, then to a NopFilterSink, and then to the pen. The
// segments of typesetting hold a move, a line or a curve for each command of
// read-fonts. They also hold a line back to the start of a contour before
// each move after the first, and no command to close a contour. drawCFF2
// closes each contour before the next move and at the end, as read-fonts
// does, and the line back to the start then disappears in the NopFilterSink.
//
// skrifa applies the FontMatrix of the dictionaries of the CFF2 table, and
// typesetting does not read it. The CFF2 fonts that subsetter tests have no
// FontMatrix.
func (inst *glyphInstance) drawCFF2(glyph uint16, pen *outlinePath) bool {
	segs, _, err := inst.cff2.LoadGlyph(tables.GlyphID(glyph), inst.coords)
	if err != nil {
		return false
	}
	sink := transformSink{inner: &nopFilterSink{inner: pen}}
	open := false
	// ensureOpen is Evaluator::ensure_open of read-fonts src/ps/cs.rs: a
	// line or a curve without a move starts at the origin.
	ensureOpen := func() {
		if !open {
			sink.moveTo(0, 0)
			open = true
		}
	}
	for _, seg := range segs {
		switch seg.Op {
		case ot.SegmentOpMoveTo:
			if open {
				sink.close()
			}
			open = true
			sink.moveTo(fixedFromF32(seg.Args[0].X), fixedFromF32(seg.Args[0].Y))
		case ot.SegmentOpLineTo:
			ensureOpen()
			sink.lineTo(fixedFromF32(seg.Args[0].X), fixedFromF32(seg.Args[0].Y))
		case ot.SegmentOpCubeTo:
			ensureOpen()
			sink.curveTo(
				fixedFromF32(seg.Args[0].X), fixedFromF32(seg.Args[0].Y),
				fixedFromF32(seg.Args[1].X), fixedFromF32(seg.Args[1].Y),
				fixedFromF32(seg.Args[2].X), fixedFromF32(seg.Args[2].Y))
		default:
			return false
		}
	}
	if open {
		sink.close()
	}
	// evaluate of read-fonts src/ps/cs.rs calls finish after the last close.
	sink.inner.finish()
	return true
}

// fixed is Fixed of font-types: a number in the 16.16 format.
type fixed int32

// fixedFromF32 converts a coordinate of typesetting to the nearest number in
// the 16.16 format. A coordinate that is not finite, or that is out of the
// range of the format, saturates.
func fixedFromF32(v float32) fixed {
	f := math.Round(float64(v) * 65536)
	switch {
	case f != f:
		return 0
	case f <= math.MinInt32:
		return math.MinInt32
	case f >= math.MaxInt32:
		return math.MaxInt32
	}
	return fixed(f)
}

// mul is impl Mul for Fixed in font-types src/fixed.rs: the product,
// rounded to the nearest number, with ties away from zero.
func (a fixed) mul(b fixed) fixed {
	ab := int64(a) * int64(b)
	if ab < 0 {
		return fixed((ab + 0x8000 - 1) >> 16)
	}
	return fixed((ab + 0x8000) >> 16)
}

// toF32 is Fixed::to_f32 in font-types src/fixed.rs.
func (a fixed) toF32() float32 {
	return float32(a) * (1.0 / 65536.0)
}

// oneOver64 is ONE_OVER_64 in read-fonts src/ps/cs.rs.
const oneOver64 fixed = 0x400

// transformSink is TransformSink in read-fonts src/ps/cs.rs, without a
// matrix and without a scale, as skrifa makes it for an outline without a
// scale.
type transformSink struct {
	inner *nopFilterSink
}

// transform is TransformSink::transform in read-fonts src/ps/cs.rs, for the
// unscaled case. It converts a point to font units as FreeType does, and so
// truncates it to an integer.
func (t transformSink) transform(x, y fixed) (fixed, fixed) {
	// 1. Multiply by 1/64
	ax := x.mul(oneOver64)
	ay := y.mul(oneOver64)
	// 2. Truncate the bottom 10 bits. Combined with the division by 64,
	// converts to font units.
	bx := ax >> 10
	by := ay >> 10
	// 4. Convert from integer to 16.16. The shift wraps, as it does in Rust.
	return fixed(int32(uint32(bx) << 16)), fixed(int32(uint32(by) << 16))
}

func (t transformSink) moveTo(x, y fixed) {
	x, y = t.transform(x, y)
	t.inner.moveTo(x, y)
}

func (t transformSink) lineTo(x, y fixed) {
	x, y = t.transform(x, y)
	t.inner.lineTo(x, y)
}

func (t transformSink) curveTo(cx1, cy1, cx2, cy2, x, y fixed) {
	cx1, cy1 = t.transform(cx1, cy1)
	cx2, cy2 = t.transform(cx2, cy2)
	x, y = t.transform(x, y)
	t.inner.curveTo(cx1, cy1, cx2, cy2, x, y)
}

func (t transformSink) close() {
	t.inner.close()
}

// pendingKind is the variant of PendingElement in read-fonts src/ps/cs.rs.
type pendingKind uint8

const (
	pendingNone pendingKind = iota
	pendingMove
	pendingLine
	pendingCurve
)

// pendingElement is Option<PendingElement> in read-fonts src/ps/cs.rs. A
// move and a line use the last two values.
type pendingElement struct {
	kind pendingKind
	v    [6]fixed
}

// targetPoint is PendingElement::target_point in read-fonts src/ps/cs.rs.
func (p pendingElement) targetPoint() [2]fixed {
	return [2]fixed{p.v[4], p.v[5]}
}

// nopFilterSink is NopFilterSink in read-fonts src/ps/cs.rs. It suppresses
// a move without a line or a curve, and a line of length 0, as FreeType
// does, before it gives the commands to the pen. Its inner sink is
// PenSink of skrifa src/outline/cff/mod.rs, which converts each value to an
// f32.
type nopFilterSink struct {
	isOpen    bool
	haveStart bool
	start     [2]fixed
	pending   pendingElement
	inner     *outlinePath
}

// flushPending is NopFilterSink::flush_pending in read-fonts src/ps/cs.rs.
func (n *nopFilterSink) flushPending(forClose bool) {
	pending := n.pending
	n.pending = pendingElement{}
	switch pending.kind {
	case pendingMove:
		if !forClose {
			n.isOpen = true
			n.inner.moveTo(pending.v[4].toF32(), pending.v[5].toF32())
			n.start, n.haveStart = pending.targetPoint(), true
		}
	case pendingLine:
		if !forClose || !n.haveStart || n.start != pending.targetPoint() {
			n.inner.lineTo(pending.v[4].toF32(), pending.v[5].toF32())
		}
	case pendingCurve:
		v := pending.v
		n.inner.curveTo(v[0].toF32(), v[1].toF32(), v[2].toF32(), v[3].toF32(), v[4].toF32(), v[5].toF32())
	}
}

func (n *nopFilterSink) moveTo(x, y fixed) {
	n.pending = pendingElement{kind: pendingMove, v: [6]fixed{4: x, 5: y}}
}

func (n *nopFilterSink) lineTo(x, y fixed) {
	// Omit the line if we're already at the given position
	if n.pending.kind != pendingNone && n.pending.targetPoint() == [2]fixed{x, y} {
		return
	}
	n.flushPending(false)
	n.pending = pendingElement{kind: pendingLine, v: [6]fixed{4: x, 5: y}}
}

func (n *nopFilterSink) curveTo(cx1, cy1, cx2, cy2, x, y fixed) {
	n.flushPending(false)
	n.pending = pendingElement{kind: pendingCurve, v: [6]fixed{cx1, cy1, cx2, cy2, x, y}}
}

func (n *nopFilterSink) close() {
	n.flushPending(true)
	if n.isOpen {
		n.inner.close()
		n.isOpen = false
	}
}

func (n *nopFilterSink) finish() {
	n.close()
}
