// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/path.rs.

import (
	"iter"

	"github.com/xo/okage/internal/kurbo"
)

// PathSegmentKind is the case of the enum PathSegment in svgtypes
// src/path.rs (D27).
type PathSegmentKind uint8

// The cases of PathSegment.
const (
	PathSegmentMoveTo PathSegmentKind = iota
	PathSegmentLineTo
	PathSegmentHorizontalLineTo
	PathSegmentVerticalLineTo
	PathSegmentCurveTo
	PathSegmentSmoothCurveTo
	PathSegmentQuadratic
	PathSegmentSmoothQuadratic
	PathSegmentEllipticalArc
	PathSegmentClosePath
)

// pathSegmentKindNames holds the Rust name of each case of PathSegmentKind.
var pathSegmentKindNames = [...]string{
	PathSegmentMoveTo:           "MoveTo",
	PathSegmentLineTo:           "LineTo",
	PathSegmentHorizontalLineTo: "HorizontalLineTo",
	PathSegmentVerticalLineTo:   "VerticalLineTo",
	PathSegmentCurveTo:          "CurveTo",
	PathSegmentSmoothCurveTo:    "SmoothCurveTo",
	PathSegmentQuadratic:        "Quadratic",
	PathSegmentSmoothQuadratic:  "SmoothQuadratic",
	PathSegmentEllipticalArc:    "EllipticalArc",
	PathSegmentClosePath:        "ClosePath",
}

// String returns the Rust name of the case.
func (k PathSegmentKind) String() string {
	if int(k) < len(pathSegmentKindNames) {
		return pathSegmentKindNames[k]
	}
	return "PathSegmentKind(?)"
}

// PathSegment is PathSegment in svgtypes src/path.rs. Each case holds the
// flag abs and some numbers, so the enum is a struct with a kind and the
// fields of every case (D27). A field that a case does not have is zero.
//
//   - MoveTo, LineTo and SmoothQuadratic: X, Y.
//   - HorizontalLineTo: X. VerticalLineTo: Y.
//   - CurveTo: X1, Y1, X2, Y2, X, Y.
//   - SmoothCurveTo: X2, Y2, X, Y.
//   - Quadratic: X1, Y1, X, Y.
//   - EllipticalArc: Rx, Ry, XAxisRotation, LargeArc, Sweep, X, Y.
//   - ClosePath: no number.
//
// Representation of a path segment.
//
// If you want to change the segment type (for example `MoveTo` to `LineTo`)
// you should create a new segment.
// But you still can change points or make segment relative or absolute.
type PathSegment struct {
	Kind PathSegmentKind
	Abs  bool

	X1 float64
	Y1 float64
	X2 float64
	Y2 float64
	X  float64
	Y  float64

	Rx            float64
	Ry            float64
	XAxisRotation float64
	LargeArc      bool
	Sweep         bool
}

// IsAbs is PathSegment::is_abs in svgtypes src/path.rs.
//
// Returns `true` if the segment is absolute.
func (p PathSegment) IsAbs() bool {
	return p.Abs
}

// Command is PathSegment::command in svgtypes src/path.rs.
//
// Returns the segment letter.
func (p PathSegment) Command() byte {
	if p.IsAbs() {
		switch p.Kind {
		case PathSegmentMoveTo:
			return 'M'
		case PathSegmentLineTo:
			return 'L'
		case PathSegmentHorizontalLineTo:
			return 'H'
		case PathSegmentVerticalLineTo:
			return 'V'
		case PathSegmentCurveTo:
			return 'C'
		case PathSegmentSmoothCurveTo:
			return 'S'
		case PathSegmentQuadratic:
			return 'Q'
		case PathSegmentSmoothQuadratic:
			return 'T'
		case PathSegmentEllipticalArc:
			return 'A'
		default:
			return 'Z'
		}
	}
	switch p.Kind {
	case PathSegmentMoveTo:
		return 'm'
	case PathSegmentLineTo:
		return 'l'
	case PathSegmentHorizontalLineTo:
		return 'h'
	case PathSegmentVerticalLineTo:
		return 'v'
	case PathSegmentCurveTo:
		return 'c'
	case PathSegmentSmoothCurveTo:
		return 's'
	case PathSegmentQuadratic:
		return 'q'
	case PathSegmentSmoothQuadratic:
		return 't'
	case PathSegmentEllipticalArc:
		return 'a'
	default:
		return 'z'
	}
}

// PathParser is PathParser in svgtypes src/path.rs.
//
// A pull-based [path data] parser.
//
// # Errors
//
//   - Most of the `Error` types can occur.
//
// # Notes
//
// The library does not support implicit commands, so they will be converted to an explicit one.
// It mostly affects an implicit `MoveTo`, which will be converted, according to the spec,
// into an explicit `LineTo`.
//
// Example: `M 10 20 30 40 50 60` -> `M 10 20 L 30 40 L 50 60`
//
// [path data]: https://www.w3.org/TR/SVG2/paths.html#PathData
type PathParser struct {
	stream     Stream
	prevCmd    byte
	hasPrevCmd bool
}

// NewPathParser is the impl From<&str> for PathParser in svgtypes
// src/path.rs.
func NewPathParser(v string) *PathParser {
	return &PathParser{
		stream: NewStream(v),
	}
}

// Next is Iterator::next for PathParser in svgtypes src/path.rs. ok is false
// at the end, where Rust returns None. When ok is true, the segment and err
// are the Result of Rust.
func (p *PathParser) Next() (PathSegment, bool, error) {
	s := &p.stream

	s.SkipSpaces()

	if s.AtEnd() {
		return PathSegment{}, false, nil
	}

	res, err := nextImpl(s, &p.prevCmd, &p.hasPrevCmd)
	if err != nil {
		s.JumpToEnd()
	}

	return res, true, err
}

// All returns an iterator over the results of Next, as the Iterator impl of
// PathParser in svgtypes src/path.rs gives them.
func (p *PathParser) All() iter.Seq2[PathSegment, error] {
	return func(yield func(PathSegment, error) bool) {
		for {
			v, ok, err := p.Next()
			if !ok || !yield(v, err) {
				return
			}
		}
	}
}

// nextImpl is next_impl in svgtypes src/path.rs. prevCmd and hasPrevCmd are
// the Option<u8> prev_cmd of upstream.
func nextImpl(s *Stream, prevCmd *byte, hasPrevCmd *bool) (PathSegment, error) {
	start := s.Pos()

	has := *hasPrevCmd
	firstChar := s.CurrByteUnchecked()

	if !has && !isCmd(firstChar) {
		return PathSegment{}, errUnexpectedData(s.CalcCharPosAt(start))
	}

	if !has && firstChar != 'M' && firstChar != 'm' {
		// The first segment must be a MoveTo.
		return PathSegment{}, errUnexpectedData(s.CalcCharPosAt(start))
	}

	// TODO: simplify
	var isImplicitMoveTo bool
	var cmd byte
	switch {
	case isCmd(firstChar):
		isImplicitMoveTo = false
		cmd = firstChar
		s.Advance(1)
	case isNumberStart(firstChar) && has:
		pCmd := *prevCmd

		if pCmd == 'Z' || pCmd == 'z' {
			// ClosePath cannot be followed by a number.
			return PathSegment{}, errUnexpectedData(s.CalcCharPosAt(start))
		}

		if pCmd == 'M' || pCmd == 'm' {
			// 'If a moveto is followed by multiple pairs of coordinates,
			// the subsequent pairs are treated as implicit lineto commands.'
			// So we parse them as LineTo.
			isImplicitMoveTo = true
			if isAbsolute(pCmd) {
				cmd = 'L'
			} else {
				cmd = 'l'
			}
		} else {
			isImplicitMoveTo = false
			cmd = pCmd
		}
	default:
		return PathSegment{}, errUnexpectedData(s.CalcCharPosAt(start))
	}

	cmdl := toRelative(cmd)
	absolute := isAbsolute(cmd)

	// nums parses n numbers of a list in order, as the fields of the Rust
	// struct literal are parsed.
	nums := func(n int) ([6]float64, error) {
		var v [6]float64
		for i := range n {
			x, err := s.ParseListNumber()
			if err != nil {
				return v, err
			}
			v[i] = x
		}
		return v, nil
	}

	var token PathSegment
	switch cmdl {
	case 'm':
		v, err := nums(2)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{Kind: PathSegmentMoveTo, Abs: absolute, X: v[0], Y: v[1]}
	case 'l':
		v, err := nums(2)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{Kind: PathSegmentLineTo, Abs: absolute, X: v[0], Y: v[1]}
	case 'h':
		v, err := nums(1)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{Kind: PathSegmentHorizontalLineTo, Abs: absolute, X: v[0]}
	case 'v':
		v, err := nums(1)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{Kind: PathSegmentVerticalLineTo, Abs: absolute, Y: v[0]}
	case 'c':
		v, err := nums(6)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{
			Kind: PathSegmentCurveTo,
			Abs:  absolute,
			X1:   v[0],
			Y1:   v[1],
			X2:   v[2],
			Y2:   v[3],
			X:    v[4],
			Y:    v[5],
		}
	case 's':
		v, err := nums(4)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{
			Kind: PathSegmentSmoothCurveTo,
			Abs:  absolute,
			X2:   v[0],
			Y2:   v[1],
			X:    v[2],
			Y:    v[3],
		}
	case 'q':
		v, err := nums(4)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{
			Kind: PathSegmentQuadratic,
			Abs:  absolute,
			X1:   v[0],
			Y1:   v[1],
			X:    v[2],
			Y:    v[3],
		}
	case 't':
		v, err := nums(2)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{Kind: PathSegmentSmoothQuadratic, Abs: absolute, X: v[0], Y: v[1]}
	case 'a':
		// TODO: radius cannot be negative
		v, err := nums(3)
		if err != nil {
			return PathSegment{}, err
		}
		largeArc, err := parseFlag(s)
		if err != nil {
			return PathSegment{}, err
		}
		sweep, err := parseFlag(s)
		if err != nil {
			return PathSegment{}, err
		}
		xy, err := nums(2)
		if err != nil {
			return PathSegment{}, err
		}
		token = PathSegment{
			Kind:          PathSegmentEllipticalArc,
			Abs:           absolute,
			Rx:            v[0],
			Ry:            v[1],
			XAxisRotation: v[2],
			LargeArc:      largeArc,
			Sweep:         sweep,
			X:             xy[0],
			Y:             xy[1],
		}
	case 'z':
		token = PathSegment{Kind: PathSegmentClosePath, Abs: absolute}
	default:
		panic("svgtypes: internal error: entered unreachable code")
	}

	if isImplicitMoveTo {
		if absolute {
			*prevCmd = 'M'
		} else {
			*prevCmd = 'm'
		}
	} else {
		*prevCmd = cmd
	}
	*hasPrevCmd = true

	return token, nil
}

// isCmd is is_cmd in svgtypes src/path.rs.
//
// Returns `true` if the selected char is the command.
func isCmd(c byte) bool {
	switch c {
	case 'M', 'm',
		'Z', 'z',
		'L', 'l',
		'H', 'h',
		'V', 'v',
		'C', 'c',
		'S', 's',
		'Q', 'q',
		'T', 't',
		'A', 'a':
		return true
	}
	return false
}

// isAbsolute is is_absolute in svgtypes src/path.rs.
//
// Returns `true` if the selected char is the absolute command.
func isAbsolute(c byte) bool {
	switch c {
	case 'M', 'Z', 'L', 'H', 'V', 'C', 'S', 'Q', 'T', 'A':
		return true
	}
	return false
}

// toRelative is to_relative in svgtypes src/path.rs.
//
// Converts the selected command char into the relative command char.
func toRelative(c byte) byte {
	switch c {
	case 'M':
		return 'm'
	case 'Z':
		return 'z'
	case 'L':
		return 'l'
	case 'H':
		return 'h'
	case 'V':
		return 'v'
	case 'C':
		return 'c'
	case 'S':
		return 's'
	case 'Q':
		return 'q'
	case 'T':
		return 't'
	case 'A':
		return 'a'
	default:
		return c
	}
}

// isNumberStart is is_number_start in svgtypes src/path.rs.
func isNumberStart(c byte) bool {
	return (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+'
}

// parseFlag is parse_flag in svgtypes src/path.rs.
//
// By the SVG spec 'large-arc' and 'sweep' must contain only one char
// and can be written without any separators, e.g.: 10 20 30 01 10 20.
func parseFlag(s *Stream) (bool, error) {
	s.SkipSpaces()

	c, err := s.CurrByte()
	if err != nil {
		return false, err
	}
	switch c {
	case '0', '1':
		s.Advance(1)
		if s.IsCurrByteEq(',') {
			s.Advance(1)
		}
		s.SkipSpaces()

		return c == '1', nil
	default:
		return false, errUnexpectedData(s.CalcCharPosAt(s.Pos()))
	}
}

// SimplePathSegmentKind is the case of the enum SimplePathSegment in
// svgtypes src/path.rs (D27).
type SimplePathSegmentKind uint8

// The cases of SimplePathSegment.
const (
	SimplePathSegmentMoveTo SimplePathSegmentKind = iota
	SimplePathSegmentLineTo
	SimplePathSegmentCurveTo
	SimplePathSegmentQuadratic
	SimplePathSegmentClosePath
)

// simplePathSegmentKindNames holds the Rust name of each case of SimplePathSegmentKind.
var simplePathSegmentKindNames = [...]string{
	SimplePathSegmentMoveTo:    "MoveTo",
	SimplePathSegmentLineTo:    "LineTo",
	SimplePathSegmentCurveTo:   "CurveTo",
	SimplePathSegmentQuadratic: "Quadratic",
	SimplePathSegmentClosePath: "ClosePath",
}

// String returns the Rust name of the case.
func (k SimplePathSegmentKind) String() string {
	if int(k) < len(simplePathSegmentKindNames) {
		return simplePathSegmentKindNames[k]
	}
	return "SimplePathSegmentKind(?)"
}

// SimplePathSegment is SimplePathSegment in svgtypes src/path.rs. It is a
// struct with a kind and the fields of every case (D27). A field that a case
// does not have is zero.
//
//   - MoveTo and LineTo: X, Y.
//   - CurveTo: X1, Y1, X2, Y2, X, Y.
//   - Quadratic: X1, Y1, X, Y.
//   - ClosePath: no number.
//
// Representation of a simple path segment.
type SimplePathSegment struct {
	Kind SimplePathSegmentKind

	X1 float64
	Y1 float64
	X2 float64
	Y2 float64
	X  float64
	Y  float64
}

// SimplifyingPathParser is SimplifyingPathParser in svgtypes src/path.rs.
//
// A simplifying Path Data parser.
//
// A more high-level Path Data parser on top of [PathParser] that provides:
//
//   - Relative to absolute segment coordinates conversion
//   - `ArcTo` to `CurveTo`s conversion
//   - `SmoothCurveTo` and `SmoothQuadratic` conversion
//   - `HorizontalLineTo` and `VerticalLineTo` to `LineTo` conversion
//   - Implicit `MoveTo` after `ClosePath` handling
//
// In the end, only absolute `MoveTo`, `LineTo`, `CurveTo`, `Quadratic` and `ClosePath` segments
// will be produced.
type SimplifyingPathParser struct {
	parser PathParser

	// Previous MoveTo coordinates.
	prevMx float64
	prevMy float64

	// Previous SmoothQuadratic coordinates.
	prevTx float64
	prevTy float64

	// Previous coordinates.
	prevX float64
	prevY float64

	prevSeg          PathSegment
	prevSimpleSeg    SimplePathSegment
	hasPrevSimpleSeg bool
	buffer           []SimplePathSegment
}

// NewSimplifyingPathParser is the impl From<&str> for SimplifyingPathParser
// in svgtypes src/path.rs.
func NewSimplifyingPathParser(v string) *SimplifyingPathParser {
	return &SimplifyingPathParser{
		parser: *NewPathParser(v),
		prevMx: 0.0,
		prevMy: 0.0,
		prevTx: 0.0,
		prevTy: 0.0,
		prevX:  0.0,
		prevY:  0.0,
		prevSeg: PathSegment{
			Kind: PathSegmentMoveTo,
			Abs:  true,
			X:    0.0,
			Y:    0.0,
		},
		hasPrevSimpleSeg: false,
		buffer:           nil,
	}
}

// prevIsClosePath reports whether prev_simple_seg is
// Some(SimplePathSegment::ClosePath).
func (p *SimplifyingPathParser) prevIsClosePath() bool {
	return p.hasPrevSimpleSeg && p.prevSimpleSeg.Kind == SimplePathSegmentClosePath
}

// popFront is self.buffer.remove(0). It shifts the other segments down, as
// Vec::remove does, so that the buffer keeps its storage and the next append
// does not allocate.
func (p *SimplifyingPathParser) popFront() SimplePathSegment {
	seg := p.buffer[0]
	n := copy(p.buffer, p.buffer[1:])
	p.buffer = p.buffer[:n]
	return seg
}

// Next is Iterator::next for SimplifyingPathParser in svgtypes src/path.rs.
// ok is false at the end, where Rust returns None. When ok is true, the
// segment and err are the Result of Rust.
func (p *SimplifyingPathParser) Next() (SimplePathSegment, bool, error) {
	if len(p.buffer) != 0 {
		return p.popFront(), true, nil
	}

	segment, ok, err := p.parser.Next()
	if !ok {
		return SimplePathSegment{}, false, nil
	}
	if err != nil {
		return SimplePathSegment{}, true, err
	}

	// If a ClosePath segment is followed by any command other than MoveTo or ClosePath
	// then MoveTo is implicit.
	if p.prevIsClosePath() {
		switch segment.Kind {
		case PathSegmentMoveTo, PathSegmentClosePath:
		default:
			newSeg := SimplePathSegment{
				Kind: SimplePathSegmentMoveTo,
				X:    p.prevMx,
				Y:    p.prevMy,
			}
			p.buffer = append(p.buffer, newSeg)
			p.prevSimpleSeg, p.hasPrevSimpleSeg = newSeg, true
		}
	}

	switch segment.Kind {
	case PathSegmentMoveTo:
		x, y := segment.X, segment.Y
		if !segment.Abs {
			// When we get 'm'(relative) segment, which is not first segment - then it's
			// relative to a previous 'M'(absolute) segment, not to the first segment.
			if p.prevIsClosePath() {
				x += p.prevMx
				y += p.prevMy
			} else {
				x += p.prevX
				y += p.prevY
			}
		}

		p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentMoveTo, X: x, Y: y})
		p.prevSeg = segment
	case PathSegmentLineTo:
		x, y := segment.X, segment.Y
		if !segment.Abs {
			x += p.prevX
			y += p.prevY
		}

		p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentLineTo, X: x, Y: y})
		p.prevSeg = segment
	case PathSegmentHorizontalLineTo:
		x := segment.X
		if !segment.Abs {
			x += p.prevX
		}

		p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentLineTo, X: x, Y: p.prevY})
		p.prevSeg = segment
	case PathSegmentVerticalLineTo:
		y := segment.Y
		if !segment.Abs {
			y += p.prevY
		}

		p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentLineTo, X: p.prevX, Y: y})
		p.prevSeg = segment
	case PathSegmentCurveTo:
		x1, y1, x2, y2, x, y := segment.X1, segment.Y1, segment.X2, segment.Y2, segment.X, segment.Y
		if !segment.Abs {
			x1 += p.prevX
			y1 += p.prevY
			x2 += p.prevX
			y2 += p.prevY
			x += p.prevX
			y += p.prevY
		}

		p.buffer = append(p.buffer, SimplePathSegment{
			Kind: SimplePathSegmentCurveTo,
			X1:   x1,
			Y1:   y1,
			X2:   x2,
			Y2:   y2,
			X:    x,
			Y:    y,
		})

		// Remember as absolute.
		p.prevSeg = PathSegment{
			Kind: PathSegmentCurveTo,
			Abs:  true,
			X1:   x1,
			Y1:   y1,
			X2:   x2,
			Y2:   y2,
			X:    x,
			Y:    y,
		}
	case PathSegmentSmoothCurveTo:
		x2, y2, x, y := segment.X2, segment.Y2, segment.X, segment.Y
		// 'The first control point is assumed to be the reflection of the second control
		// point on the previous command relative to the current point.
		// (If there is no previous command or if the previous command
		// was not an C, c, S or s, assume the first control point is
		// coincident with the current point.)'
		var x1, y1 float64
		switch p.prevSeg.Kind {
		case PathSegmentCurveTo, PathSegmentSmoothCurveTo:
			x1, y1 = float64(p.prevSeg.X*2.0)-p.prevSeg.X2, float64(p.prevSeg.Y*2.0)-p.prevSeg.Y2
		default:
			x1, y1 = p.prevX, p.prevY
		}

		if !segment.Abs {
			x2 += p.prevX
			y2 += p.prevY
			x += p.prevX
			y += p.prevY
		}

		p.buffer = append(p.buffer, SimplePathSegment{
			Kind: SimplePathSegmentCurveTo,
			X1:   x1,
			Y1:   y1,
			X2:   x2,
			Y2:   y2,
			X:    x,
			Y:    y,
		})

		// Remember as absolute.
		p.prevSeg = PathSegment{
			Kind: PathSegmentSmoothCurveTo,
			Abs:  true,
			X2:   x2,
			Y2:   y2,
			X:    x,
			Y:    y,
		}
	case PathSegmentQuadratic:
		x1, y1, x, y := segment.X1, segment.Y1, segment.X, segment.Y
		if !segment.Abs {
			x1 += p.prevX
			y1 += p.prevY
			x += p.prevX
			y += p.prevY
		}

		p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentQuadratic, X1: x1, Y1: y1, X: x, Y: y})

		// Remember as absolute.
		p.prevSeg = PathSegment{
			Kind: PathSegmentQuadratic,
			Abs:  true,
			X1:   x1,
			Y1:   y1,
			X:    x,
			Y:    y,
		}
	case PathSegmentSmoothQuadratic:
		x, y := segment.X, segment.Y
		// 'The control point is assumed to be the reflection of
		// the control point on the previous command relative to
		// the current point. (If there is no previous command or
		// if the previous command was not a Q, q, T or t, assume
		// the control point is coincident with the current point.)'
		var x1, y1 float64
		switch p.prevSeg.Kind {
		case PathSegmentQuadratic:
			x1, y1 = float64(p.prevSeg.X*2.0)-p.prevSeg.X1, float64(p.prevSeg.Y*2.0)-p.prevSeg.Y1
		case PathSegmentSmoothQuadratic:
			x1, y1 = float64(p.prevSeg.X*2.0)-p.prevTx, float64(p.prevSeg.Y*2.0)-p.prevTy
		default:
			x1, y1 = p.prevX, p.prevY
		}

		p.prevTx = x1
		p.prevTy = y1

		if !segment.Abs {
			x += p.prevX
			y += p.prevY
		}

		p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentQuadratic, X1: x1, Y1: y1, X: x, Y: y})

		// Remember as absolute.
		p.prevSeg = PathSegment{Kind: PathSegmentSmoothQuadratic, Abs: true, X: x, Y: y}
	case PathSegmentEllipticalArc:
		x, y := segment.X, segment.Y
		if !segment.Abs {
			x += p.prevX
			y += p.prevY
		}

		svgArc := kurbo.SvgArc{
			From:      kurbo.NewPoint(p.prevX, p.prevY),
			To:        kurbo.NewPoint(x, y),
			Radii:     kurbo.NewVec2(segment.Rx, segment.Ry),
			XRotation: toRadians(segment.XAxisRotation),
			LargeArc:  segment.LargeArc,
			Sweep:     segment.Sweep,
		}

		if arc, ok := kurbo.FromSvgArc(&svgArc); ok {
			arc.ToCubicBeziers(0.1, func(p1, p2, pt kurbo.Point) {
				p.buffer = append(p.buffer, SimplePathSegment{
					Kind: SimplePathSegmentCurveTo,
					X1:   p1.X,
					Y1:   p1.Y,
					X2:   p2.X,
					Y2:   p2.Y,
					X:    pt.X,
					Y:    pt.Y,
				})
			})
		} else {
			p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentLineTo, X: x, Y: y})
		}

		p.prevSeg = segment
	case PathSegmentClosePath:
		if p.prevIsClosePath() {
			// Do not add sequential ClosePath segments.
			// Otherwise it will break markers rendering.
		} else {
			p.buffer = append(p.buffer, SimplePathSegment{Kind: SimplePathSegmentClosePath})
		}

		p.prevSeg = segment
	}

	// Remember last position.
	if len(p.buffer) != 0 {
		newSegment := p.buffer[len(p.buffer)-1]
		p.prevSimpleSeg, p.hasPrevSimpleSeg = newSegment, true

		switch newSegment.Kind {
		case SimplePathSegmentMoveTo:
			p.prevX = newSegment.X
			p.prevY = newSegment.Y
			p.prevMx = p.prevX
			p.prevMy = p.prevY
		case SimplePathSegmentLineTo:
			p.prevX = newSegment.X
			p.prevY = newSegment.Y
		case SimplePathSegmentCurveTo:
			p.prevX = newSegment.X
			p.prevY = newSegment.Y
		case SimplePathSegmentQuadratic:
			p.prevX = newSegment.X
			p.prevY = newSegment.Y
		case SimplePathSegmentClosePath:
			// ClosePath moves us to the last MoveTo coordinate.
			p.prevX = p.prevMx
			p.prevY = p.prevMy
		}
	}

	if len(p.buffer) == 0 {
		return p.Next()
	}

	return p.popFront(), true, nil
}

// All returns an iterator over the results of Next, as the Iterator impl of
// SimplifyingPathParser in svgtypes src/path.rs gives them.
func (p *SimplifyingPathParser) All() iter.Seq2[SimplePathSegment, error] {
	return func(yield func(SimplePathSegment, error) bool) {
		for {
			v, ok, err := p.Next()
			if !ok || !yield(v, err) {
				return
			}
		}
	}
}
