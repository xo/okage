// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/transform.rs.

import (
	"iter"

	"github.com/xo/okage/internal/kurbo"
)

// Transform is Transform in svgtypes src/transform.rs.
//
// Representation of the [`<transform>`] type.
//
// [`<transform>`]: https://www.w3.org/TR/SVG2/coords.html#InterfaceSVGTransform
type Transform struct {
	A float64
	B float64
	C float64
	D float64
	E float64
	F float64
}

// NewTransform is Transform::new in svgtypes src/transform.rs.
//
// Constructs a new transform.
func NewTransform(a, b, c, d, e, f float64) Transform {
	return Transform{A: a, B: b, C: c, D: d, E: e, F: f}
}

// DefaultTransform is the impl Default for Transform in svgtypes
// src/transform.rs.
func DefaultTransform() Transform {
	return NewTransform(1.0, 0.0, 0.0, 1.0, 0.0, 0.0)
}

// TransformListTokenKind is the case of the enum TransformListToken in
// svgtypes src/transform.rs (D27).
type TransformListTokenKind uint8

// The cases of TransformListToken.
const (
	TransformListTokenMatrix TransformListTokenKind = iota
	TransformListTokenTranslate
	TransformListTokenScale
	TransformListTokenRotate
	TransformListTokenSkewX
	TransformListTokenSkewY
)

// transformListTokenKindNames holds the Rust name of each case of TransformListTokenKind.
var transformListTokenKindNames = [...]string{
	TransformListTokenMatrix:    "Matrix",
	TransformListTokenTranslate: "Translate",
	TransformListTokenScale:     "Scale",
	TransformListTokenRotate:    "Rotate",
	TransformListTokenSkewX:     "SkewX",
	TransformListTokenSkewY:     "SkewY",
}

// String returns the Rust name of the case.
func (k TransformListTokenKind) String() string {
	if int(k) < len(transformListTokenKindNames) {
		return transformListTokenKindNames[k]
	}
	return "TransformListTokenKind(?)"
}

// TransformListToken is TransformListToken in svgtypes src/transform.rs.
// Each case holds f64 values, so it is a struct with a kind and the fields
// of every case (D27). A field that a case does not have is zero.
//
//   - Matrix: A, B, C, D, E, F.
//   - Translate: Tx, Ty.
//   - Scale: Sx, Sy.
//   - Rotate, SkewX and SkewY: Angle.
type TransformListToken struct {
	Kind TransformListTokenKind

	A float64
	B float64
	C float64
	D float64
	E float64
	F float64

	Tx float64
	Ty float64

	Sx float64
	Sy float64

	Angle float64
}

// TransformListParser is TransformListParser in svgtypes src/transform.rs.
//
// A pull-based [`<transform-list>`] parser.
//
// # Errors
//
//   - Most of the `Error` types can occur.
//
// # Notes
//
//   - There are no separate `rotate(<rotate-angle> <cx> <cy>)` type.
//     It will be automatically split into three `Transform` tokens:
//     `translate(<cx> <cy>) rotate(<rotate-angle>) translate(-<cx> -<cy>)`.
//     Just like the spec is stated.
//
// [`<transform-list>`]: https://www.w3.org/TR/SVG11/shapes.html#PointsBNF
type TransformListParser struct {
	stream Stream

	rotateTsX, rotateTsY float64
	hasRotateTs          bool

	lastAngle    float64
	hasLastAngle bool
}

// NewTransformListParser is the impl From<&str> for TransformListParser in
// svgtypes src/transform.rs.
func NewTransformListParser(text string) *TransformListParser {
	return &TransformListParser{
		stream: NewStream(text),
	}
}

// Next is Iterator::next for TransformListParser in svgtypes
// src/transform.rs. ok is false at the end, where Rust returns None. When ok
// is true, the token and err are the Result of Rust.
func (p *TransformListParser) Next() (TransformListToken, bool, error) {
	if p.hasLastAngle {
		a := p.lastAngle
		p.hasLastAngle = false
		return TransformListToken{Kind: TransformListTokenRotate, Angle: a}, true, nil
	}

	if p.hasRotateTs {
		x, y := p.rotateTsX, p.rotateTsY
		p.hasRotateTs = false
		return TransformListToken{Kind: TransformListTokenTranslate, Tx: -x, Ty: -y}, true, nil
	}

	p.stream.SkipSpaces()

	if p.stream.AtEnd() {
		// empty attribute is still a valid value
		return TransformListToken{}, false, nil
	}

	res, err := p.parseNext()
	if err != nil {
		p.stream.JumpToEnd()
	}

	return res, true, err
}

// All returns an iterator over the results of Next, as the Iterator impl of
// TransformListParser in svgtypes src/transform.rs gives them.
func (p *TransformListParser) All() iter.Seq2[TransformListToken, error] {
	return func(yield func(TransformListToken, error) bool) {
		for {
			v, ok, err := p.Next()
			if !ok || !yield(v, err) {
				return
			}
		}
	}
}

// parseNext is TransformListParser::parse_next in svgtypes src/transform.rs.
func (p *TransformListParser) parseNext() (TransformListToken, error) {
	s := &p.stream

	start := s.Pos()
	name := s.ConsumeASCIIIdent()
	s.SkipSpaces()
	if err := s.ConsumeByte('('); err != nil {
		return TransformListToken{}, err
	}

	var t TransformListToken
	switch name {
	case "matrix":
		var v [6]float64
		for i := range v {
			n, err := s.ParseListNumber()
			if err != nil {
				return TransformListToken{}, err
			}
			v[i] = n
		}
		t = TransformListToken{
			Kind: TransformListTokenMatrix,
			A:    v[0],
			B:    v[1],
			C:    v[2],
			D:    v[3],
			E:    v[4],
			F:    v[5],
		}
	case "translate":
		x, err := s.ParseListNumber()
		if err != nil {
			return TransformListToken{}, err
		}
		s.SkipSpaces()

		var y float64
		if s.IsCurrByteEq(')') {
			// 'If <ty> is not provided, it is assumed to be zero.'
			y = 0.0
		} else {
			y, err = s.ParseListNumber()
			if err != nil {
				return TransformListToken{}, err
			}
		}

		t = TransformListToken{Kind: TransformListTokenTranslate, Tx: x, Ty: y}
	case "scale":
		x, err := s.ParseListNumber()
		if err != nil {
			return TransformListToken{}, err
		}
		s.SkipSpaces()

		var y float64
		if s.IsCurrByteEq(')') {
			// 'If <sy> is not provided, it is assumed to be equal to <sx>.'
			y = x
		} else {
			y, err = s.ParseListNumber()
			if err != nil {
				return TransformListToken{}, err
			}
		}

		t = TransformListToken{Kind: TransformListTokenScale, Sx: x, Sy: y}
	case "rotate":
		a, err := s.ParseListNumber()
		if err != nil {
			return TransformListToken{}, err
		}
		s.SkipSpaces()

		if !s.IsCurrByteEq(')') {
			// 'If optional parameters <cx> and <cy> are supplied, the rotate is about the
			// point (cx, cy). The operation represents the equivalent of the following
			// specification:
			// translate(<cx>, <cy>) rotate(<rotate-angle>) translate(-<cx>, -<cy>).'
			cx, err := s.ParseListNumber()
			if err != nil {
				return TransformListToken{}, err
			}
			cy, err := s.ParseListNumber()
			if err != nil {
				return TransformListToken{}, err
			}
			p.rotateTsX, p.rotateTsY, p.hasRotateTs = cx, cy, true
			p.lastAngle, p.hasLastAngle = a, true

			t = TransformListToken{Kind: TransformListTokenTranslate, Tx: cx, Ty: cy}
		} else {
			t = TransformListToken{Kind: TransformListTokenRotate, Angle: a}
		}
	case "skewX":
		a, err := s.ParseListNumber()
		if err != nil {
			return TransformListToken{}, err
		}
		t = TransformListToken{Kind: TransformListTokenSkewX, Angle: a}
	case "skewY":
		a, err := s.ParseListNumber()
		if err != nil {
			return TransformListToken{}, err
		}
		t = TransformListToken{Kind: TransformListTokenSkewY, Angle: a}
	default:
		return TransformListToken{}, errUnexpectedData(s.CalcCharPosAt(start))
	}

	s.SkipSpaces()
	if err := s.ConsumeByte(')'); err != nil {
		return TransformListToken{}, err
	}
	s.SkipSpaces()

	if s.IsCurrByteEq(',') {
		s.Advance(1)
	}

	return t, nil
}

// ParseTransform is the impl FromStr for Transform in svgtypes
// src/transform.rs.
func ParseTransform(text string) (Transform, error) {
	tokens := NewTransformListParser(text)
	ts := DefaultTransform()

	for token, err := range tokens.All() {
		if err != nil {
			return Transform{}, err
		}
		switch token.Kind {
		case TransformListTokenMatrix:
			ts = multiply(&ts, new(NewTransform(token.A, token.B, token.C, token.D, token.E, token.F)))
		case TransformListTokenTranslate:
			ts = multiply(&ts, new(NewTransform(1.0, 0.0, 0.0, 1.0, token.Tx, token.Ty)))
		case TransformListTokenScale:
			ts = multiply(&ts, new(NewTransform(token.Sx, 0.0, 0.0, token.Sy, 0.0, 0.0)))
		case TransformListTokenRotate:
			v := toRadians(token.Angle)
			a := kurbo.Cos(v)
			b := kurbo.Sin(v)
			c := -b
			d := a
			ts = multiply(&ts, new(NewTransform(a, b, c, d, 0.0, 0.0)))
		case TransformListTokenSkewX:
			c := kurbo.Tan(toRadians(token.Angle))
			ts = multiply(&ts, new(NewTransform(1.0, 0.0, c, 1.0, 0.0, 0.0)))
		case TransformListTokenSkewY:
			b := kurbo.Tan(toRadians(token.Angle))
			ts = multiply(&ts, new(NewTransform(1.0, b, 0.0, 1.0, 0.0, 0.0)))
		}
	}

	return ts, nil
}

// multiply is multiply in svgtypes src/transform.rs.
func multiply(ts1, ts2 *Transform) Transform {
	return Transform{
		A: float64(ts1.A*ts2.A) + float64(ts1.C*ts2.B),
		B: float64(ts1.B*ts2.A) + float64(ts1.D*ts2.B),
		C: float64(ts1.A*ts2.C) + float64(ts1.C*ts2.D),
		D: float64(ts1.B*ts2.C) + float64(ts1.D*ts2.D),
		E: float64(ts1.A*ts2.E) + float64(ts1.C*ts2.F) + ts1.E,
		F: float64(ts1.B*ts2.E) + float64(ts1.D*ts2.F) + ts1.F,
	}
}
