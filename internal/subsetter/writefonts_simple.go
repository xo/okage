// Copyright 2019 Colin Rothfels
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"math"

	"github.com/xo/okage/kurbo"
)

// This file ports the code of write-fonts 0.48.0 that
// SkrifaInterjector::interject of subsetter src/interjector.rs calls to
// write a simple glyph of the glyf table (D62):
//
//   - SimpleGlyph::from_bezpath of write-fonts/src/tables/glyf/simple.rs,
//     with simple_glyphs_from_kurbo, InterpolatableContourBuilder,
//     is_mid_point and is_implicit_on_curve, for one path,
//   - impl FontWrite for SimpleGlyph, with compute_point_deltas and
//     RepeatableFlag, which dump_table calls,
//   - impl From<Rect> for Bbox of write-fonts/src/tables/glyf.rs,
//   - OtRound of write-fonts/src/round.rs, and isclose and WrappingGet of
//     write-fonts/src/util.rs.
//
// write-fonts builds the glyphs of many paths at once, so that the glyphs
// can be interpolated. subsetter gives one path, so the port takes one path.
// The port also holds the few parts of BezPath of kurbo 0.13 that write-fonts
// and the interjector call, because the kurbo of okage does not have BezPath
// (D7).

// bezPath is BezPath in kurbo src/bezpath.rs: a list of path elements.
type bezPath []kurbo.PathEl

// moveTo is BezPath::move_to in kurbo src/bezpath.rs.
func (p *bezPath) moveTo(pt kurbo.Point) {
	*p = append(*p, kurbo.MoveTo(pt))
}

// lineTo is BezPath::line_to in kurbo src/bezpath.rs.
func (p *bezPath) lineTo(pt kurbo.Point) {
	*p = append(*p, kurbo.LineTo(pt))
}

// quadTo is BezPath::quad_to in kurbo src/bezpath.rs.
func (p *bezPath) quadTo(p1, p2 kurbo.Point) {
	*p = append(*p, kurbo.QuadTo(p1, p2))
}

// closePath is BezPath::close_path in kurbo src/bezpath.rs.
func (p *bezPath) closePath() {
	*p = append(*p, kurbo.ClosePath())
}

// isEmpty is BezPath::is_empty in kurbo src/bezpath.rs. A path is empty
// when it holds only moves and closes.
func (p bezPath) isEmpty() bool {
	for _, el := range p {
		if el.Kind != kurbo.PathElMoveTo && el.Kind != kurbo.PathElClosePath {
			return false
		}
	}
	return true
}

// controlBox is BezPath::control_box in kurbo src/bezpath.rs: the smallest
// rectangle that holds each point of the path, with the control points. An
// empty path gives the zero rectangle.
func (p bezPath) controlBox() kurbo.Rect {
	var cbox kurbo.Rect
	have := false
	add := func(pts ...kurbo.Point) {
		for _, pt := range pts {
			if have {
				cbox = cbox.UnionPt(pt)
			} else {
				cbox = kurbo.RectFromPoints(pt, pt)
				have = true
			}
		}
	}
	for _, el := range p {
		switch el.Kind {
		case kurbo.PathElMoveTo, kurbo.PathElLineTo:
			add(el.P[0])
		case kurbo.PathElQuadTo:
			add(el.P[0], el.P[1])
		case kurbo.PathElCurveTo:
			add(el.P[0], el.P[1], el.P[2])
		}
	}
	return cbox
}

// curvePoint is CurvePoint in read-fonts src/tables/glyf.rs: a point of a
// glyph, with its flag on-curve.
type curvePoint struct {
	x, y    int16
	onCurve bool
}

// bboxGlyf is Bbox in write-fonts src/tables/glyf.rs: the bounding box of a
// glyph. Its name has the suffix of its module, as simpleGlyphGlyf has.
type bboxGlyf struct {
	xMin, yMin, xMax, yMax int16
}

// bboxFromRect is impl From<Rect> for Bbox in write-fonts
// src/tables/glyf.rs.
func bboxFromRect(r kurbo.Rect) bboxGlyf {
	return bboxGlyf{
		xMin: otRound(math.Min(r.X0, r.X1)),
		yMin: otRound(math.Min(r.Y0, r.Y1)),
		xMax: otRound(math.Max(r.X0, r.X1)),
		yMax: otRound(math.Max(r.Y0, r.Y1)),
	}
}

// otRound is impl OtRound<i16> for f64 in write-fonts src/round.rs: it
// rounds half up, as the OpenType specification says. The conversion to
// i16 saturates and gives 0 for NaN, as the operator as of Rust does.
func otRound(v float64) int16 {
	f := math.Floor(v + 0.5)
	switch {
	case f != f:
		return 0
	case f <= math.MinInt16:
		return math.MinInt16
	case f >= math.MaxInt16:
		return math.MaxInt16
	}
	return int16(f)
}

// otRoundF64 is impl OtRound<f64> for f64 in write-fonts src/round.rs.
func otRoundF64(v float64) float64 {
	return math.Floor(v + 0.5)
}

// isclose is isclose in write-fonts src/util.rs, with the default
// FloatComparator: a relative tolerance of 1e-9 and no absolute tolerance,
// as math.isclose of Python.
func isclose(a, b float64) bool {
	const relTol, absTol = 1e-9, 0.0
	if a == b {
		return true
	}
	if math.IsInf(a, 0) || math.IsNaN(a) || math.IsInf(b, 0) || math.IsNaN(b) {
		return false
	}
	diff := math.Abs(a - b)
	return diff <= math.Abs(relTol*b) || diff <= math.Abs(relTol*a) || diff <= absTol
}

// simpleGlyphGlyf is SimpleGlyph in write-fonts src/tables/glyf/simple.rs: a
// glyph without components. Each contour holds only line and quadratic
// Bézier segments. Its name has the suffix of the module glyf, as D10 says,
// because a helper of the tests of the package is named simpleGlyph.
type simpleGlyphGlyf struct {
	bbox         bboxGlyf
	contours     [][]curvePoint
	instructions []byte
}

// malformedPath is MalformedPath in write-fonts src/tables/glyf/simple.rs:
// the reason that a path cannot make a glyph. Its zero value means that the
// path is not malformed.
type malformedPath uint8

// The cases of MalformedPath that a path of one glyph can give. TooSmall is
// never returned by upstream, and UnequalNumberOfElements and
// InconsistentPathElements need more than one path, so the port does not
// have them.
const (
	malformedPathNone malformedPath = iota
	malformedPathHasCubic
	malformedPathMissingMove
)

// The flags of a point of a simple glyph, SimpleGlyphFlags in read-fonts
// src/tables/glyf.rs.
const (
	flagOnCurvePoint                  uint8 = 0x01
	flagXShortVector                  uint8 = 0x02
	flagYShortVector                  uint8 = 0x04
	flagRepeatFlag                    uint8 = 0x08
	flagXIsSameOrPositiveXShortVector uint8 = 0x10
	flagYIsSameOrPositiveYShortVector uint8 = 0x20
)

// coordDeltaShortMax is SHORT_MAX of compute_point_deltas in write-fonts
// src/tables/glyf/simple.rs: the largest delta that a short delta holds.
const coordDeltaShortMax = math.MaxUint8

// simpleGlyphFromBezPath is SimpleGlyph::from_bezpath in write-fonts
// src/tables/glyf/simple.rs, and simple_glyphs_from_kurbo for one path.
//
// It makes a simple glyph from a path. The path may contain only line and
// quadratic Bézier segments. The caller is responsible for converting any
// cubic segments to quadratics before calling.
//
// It returns the reason when the path is malformed: when it contains cubic
// segments, or does not begin with a move.
//
// In the glyf table, the contours of a simple glyph close implicitly when
// they are drawn. In font sources and in SVG, open and closed paths are
// distinct. An explicit close joins the first and the last point of a
// contour, which is not the same as a last command that ends at the start
// of the contour.
func simpleGlyphFromBezPath(path bezPath) (simpleGlyphGlyf, malformedPath) {
	var contours [][]contourPoint
	var current []contourPoint
	haveCurrent := false
	for _, el := range path {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			// we have a new contour, flush the current one
			if haveCurrent {
				contours = append(contours, current)
			}
			current = []contourPoint{{point: el.P[0], onCurve: true}}
			haveCurrent = true
		case kurbo.PathElLineTo:
			if !haveCurrent {
				return simpleGlyphGlyf{}, malformedPathMissingMove
			}
			current = append(current, contourPoint{point: el.P[0], onCurve: true})
		case kurbo.PathElQuadTo:
			if !haveCurrent {
				return simpleGlyphGlyf{}, malformedPathMissingMove
			}
			current = append(current,
				contourPoint{point: el.P[0], onCurve: false},
				contourPoint{point: el.P[1], onCurve: true})
		case kurbo.PathElCurveTo:
			return simpleGlyphGlyf{}, malformedPathHasCubic
		case kurbo.PathElClosePath:
			if !haveCurrent {
				return simpleGlyphGlyf{}, malformedPathMissingMove
			}
			// remove last point in closed path if has same coords as the move
			// point. This matches the handling of the point pens of FontTools.
			if len(current) > 1 && current[len(current)-1] == current[0] {
				current = current[:len(current)-1]
			}
		}
	}
	if haveCurrent {
		contours = append(contours, current)
	}

	glyph := simpleGlyphGlyf{
		// The glyph wants the control point box, not the tight bounding
		// box, so the box is not the bounding box of the path.
		bbox:     bboxFromRect(path.controlBox()),
		contours: make([][]curvePoint, 0, len(contours)),
	}
	for _, c := range contours {
		glyph.contours = append(glyph.contours, buildContour(c))
	}
	return glyph, malformedPathNone
}

// contourPoint is ContourPoint in write-fonts src/tables/glyf/simple.rs: a
// point with its flag on-curve, before it is rounded to integers.
type contourPoint struct {
	point   kurbo.Point
	onCurve bool
}

// buildContour is InterpolatableContourBuilder::build in write-fonts
// src/tables/glyf/simple.rs, for one contour. It rounds the points to
// integers, and drops each on-curve point that can be implied.
func buildContour(points []contourPoint) []curvePoint {
	out := make([]curvePoint, 0, len(points))
	for i, p := range points {
		if isImplicitOnCurve(points, i) {
			continue
		}
		out = append(out, curvePoint{x: otRound(p.point.X), y: otRound(p.point.Y), onCurve: p.onCurve})
	}
	return out
}

// isMidPoint is is_mid_point in write-fonts src/tables/glyf/simple.rs. It
// tells whether p1 is the midpoint of p0 and p2, before or after the
// coordinates are rounded to integers, to avoid false negatives due to
// rounding.
func isMidPoint(p0, p1, p2 kurbo.Point) bool {
	mid := p0.Midpoint(p2)
	if isclose(mid.X, p1.X) && isclose(mid.Y, p1.Y) {
		return true
	}
	return otRoundF64(p0.X)+otRoundF64(p2.X) == otRoundF64(p1.X)*2.0 &&
		otRoundF64(p0.Y)+otRoundF64(p2.Y) == otRoundF64(p1.Y)*2.0
}

// isImplicitOnCurve is is_implicit_on_curve in write-fonts
// src/tables/glyf/simple.rs. The point at idx can be implied when it is
// on the curve, halfway between two points off the curve.
func isImplicitOnCurve(points []contourPoint, idx int) bool {
	p1 := points[idx]
	if !p1.onCurve {
		return false
	}
	// wrapping_prev and wrapping_next of write-fonts src/util.rs.
	prev, next := idx-1, idx+1
	if idx == 0 {
		prev = len(points) - 1
	}
	if idx == len(points)-1 {
		next = 0
	}
	p0, p2 := points[prev], points[next]
	if p0.onCurve || p0.onCurve != p2.onCurve {
		return false
	}
	// drop p1 if halfway between p0 and p2
	return isMidPoint(p0.point, p1.point, p2.point)
}

// coordDelta is CoordDelta in write-fonts src/tables/glyf/simple.rs: how a
// delta of a coordinate is written. size is 0 for a delta that the flag
// repeats, 1 for a short delta, and 2 for a long delta.
type coordDelta struct {
	size  uint8
	value int16
}

// pointDelta is an item of compute_point_deltas in write-fonts
// src/tables/glyf/simple.rs: the flag and the deltas of a point.
type pointDelta struct {
	flag uint8
	x, y coordDelta
}

// flagAndDelta is flag_and_delta of compute_point_deltas in write-fonts
// src/tables/glyf/simple.rs, which serves x and y with their flags.
func flagAndDelta(value int16, shortFlag, sameOrPos uint8) (uint8, coordDelta) {
	switch {
	case value == 0:
		return sameOrPos, coordDelta{}
	case value >= -coordDeltaShortMax && value <= -1:
		return shortFlag, coordDelta{size: 1, value: -value}
	case value >= 1 && value <= coordDeltaShortMax:
		return shortFlag | sameOrPos, coordDelta{size: 1, value: value}
	}
	return 0, coordDelta{size: 2, value: value}
}

// computePointDeltas is SimpleGlyph::compute_point_deltas in write-fonts
// src/tables/glyf/simple.rs. It computes the flags and the deltas of the
// points of the glyph, without the binary encoding and without the
// repeated flags.
func (g simpleGlyphGlyf) computePointDeltas() []pointDelta {
	var out []pointDelta
	var lastX, lastY int16
	for _, c := range g.contours {
		for _, point := range c {
			var flag uint8
			// The subtraction wraps, as it does in a release build of Rust.
			dx := point.x - lastX
			dy := point.y - lastY
			lastX = point.x
			lastY = point.y

			if point.onCurve {
				flag |= flagOnCurvePoint
			}
			xFlag, xData := flagAndDelta(dx, flagXShortVector, flagXIsSameOrPositiveXShortVector)
			yFlag, yData := flagAndDelta(dy, flagYShortVector, flagYIsSameOrPositiveYShortVector)
			flag |= xFlag | yFlag
			out = append(out, pointDelta{flag: flag, x: xData, y: yData})
		}
	}
	return out
}

// repeatableFlag is RepeatableFlag in write-fonts src/tables/glyf/simple.rs:
// a flag that can have a repeat byte.
type repeatableFlag struct {
	flag   uint8
	repeat uint8
}

// repeatableFlagsFromFlags is RepeatableFlag::iter_from_flags in write-fonts
// src/tables/glyf/simple.rs. It returns the flags with their repeat values.
//
// If a flag repeats exactly once, then there is no difference in size
// between a repeat flag followed by a value of 1, and the flag twice without
// the repeat bit. fontmake writes the flag twice, and write-fonts does what
// fontmake does: a single repeat becomes two flags without the repeat bit.
func repeatableFlagsFromFlags(flags []uint8) []repeatableFlag {
	var out []repeatableFlag
	var prev repeatableFlag
	havePrev := false
	for _, flag := range flags {
		if !havePrev {
			prev, havePrev = repeatableFlag{flag: flag}, true
			continue
		}
		last := prev
		if last.flag&^flagRepeatFlag == flag && last.repeat < math.MaxUint8 {
			last.repeat++
			last.flag |= flagRepeatFlag
			prev = last
			continue
		}
		// split a single repeat into two non-repeat flags
		if last.repeat == 1 {
			last.flag &^= flagRepeatFlag
			last.repeat = 0
			out = append(out, last)
		}
		out = append(out, last)
		prev = repeatableFlag{flag: flag}
	}
	if havePrev {
		if prev.repeat == 1 {
			prev.flag &^= flagRepeatFlag
			prev.repeat = 0
			out = append(out, prev)
		}
		out = append(out, prev)
	}
	return out
}

// dump is dump_table of write-fonts src/write.rs for a SimpleGlyph, with
// impl FontWrite for SimpleGlyph of src/tables/glyf/simple.rs. A glyph
// without a contour gives no byte.
//
// It returns false where write-fonts panics: when the glyph has i16::MAX
// contours or more, or u16::MAX bytes of instructions or more.
func (g simpleGlyphGlyf) dump() ([]byte, bool) {
	if len(g.contours) >= math.MaxInt16 || len(g.instructions) >= math.MaxUint16 {
		return nil, false
	}
	nContours := int16(len(g.contours))
	if nContours == 0 {
		// we don't bother writing empty glyphs
		return nil, true
	}
	w := newWriter()
	w.writeI16(nContours)
	w.writeI16(g.bbox.xMin)
	w.writeI16(g.bbox.yMin)
	w.writeI16(g.bbox.xMax)
	w.writeI16(g.bbox.yMax)
	// now write end points of contours:
	cur := 0
	for _, c := range g.contours {
		cur += len(c)
		// The conversion and the subtraction wrap, as they do in a release
		// build of Rust.
		w.writeU16(uint16(cur) - 1)
	}
	w.writeU16(uint16(len(g.instructions)))
	w.extend(g.instructions)

	deltas := g.computePointDeltas()
	flags := make([]uint8, len(deltas))
	for i, d := range deltas {
		flags[i] = d.flag
	}
	for _, f := range repeatableFlagsFromFlags(flags) {
		w.writeU8(f.flag)
		if f.flag&flagRepeatFlag != 0 {
			w.writeU8(f.repeat)
		}
	}
	for _, d := range deltas {
		d.x.write(w)
	}
	for _, d := range deltas {
		d.y.write(w)
	}
	if w.len()%2 != 0 {
		w.writeU8(0)
	}
	return w.finish(), true
}

// write is impl FontWrite for CoordDelta in write-fonts
// src/tables/glyf/simple.rs. A repeated delta writes nothing.
func (d coordDelta) write(w *writer) {
	switch d.size {
	case 1:
		w.writeU8(uint8(d.value))
	case 2:
		w.writeI16(d.value)
	}
}
