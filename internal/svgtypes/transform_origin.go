// Copyright 2023 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/transform_origin.rs. The enum
// TransformOriginError becomes the sentinel errors
// ErrSVGTypesTransformOrigin* of the root package (D21).

import "github.com/xo/okage"

// position is the private enum Position in svgtypes src/transform_origin.rs.
// Each case holds one value, so it is a struct with a kind (D27): isLength
// chooses between the case Length, with length, and the case
// DirectionalPosition, with dp.
type position struct {
	isLength bool
	length   Length
	dp       DirectionalPosition
}

// isVertical is Position::is_vertical in svgtypes src/transform_origin.rs.
func (p position) isVertical() bool {
	if p.isLength {
		return true
	}
	return p.dp.IsVertical()
}

// isHorizontal is Position::is_horizontal in svgtypes
// src/transform_origin.rs.
func (p position) isHorizontal() bool {
	if p.isLength {
		return true
	}
	return p.dp.IsHorizontal()
}

// lengthFromPosition is the impl From<Position> for Length in svgtypes
// src/transform_origin.rs.
func lengthFromPosition(value position) Length {
	if value.isLength {
		return value.length
	}
	return LengthFromDirectionalPosition(value.dp)
}

// TransformOrigin is TransformOrigin in svgtypes src/transform_origin.rs.
//
// Representation of the [`<transform-origin>`] type.
//
// [`<transform-origin>`]: https://drafts.csswg.org/css-transforms/#transform-origin-property
type TransformOrigin struct {
	// The x offset of the transform origin.
	XOffset Length
	// The y offset of the transform origin.
	YOffset Length
	// The z offset of the transform origin.
	ZOffset Length
}

// NewTransformOrigin is TransformOrigin::new in svgtypes
// src/transform_origin.rs.
//
// Constructs a new transform origin.
func NewTransformOrigin(xOffset, yOffset, zOffset Length) TransformOrigin {
	return TransformOrigin{
		XOffset: xOffset,
		YOffset: yOffset,
		ZOffset: zOffset,
	}
}

// ParseTransformOrigin is the impl FromStr for TransformOrigin in svgtypes
// src/transform_origin.rs.
//
// It panics where upstream reaches unreachable!(): when the first part does
// not parse but the text does not fail before the match, such as " " or
// ",5". The port keeps this fault of upstream.
func ParseTransformOrigin(text string) (TransformOrigin, error) {
	stream := NewStream(text)

	if stream.AtEnd() {
		return TransformOrigin{}, okage.ErrSVGTypesTransformOriginMissingParameters
	}

	parsePart := func(stream *Stream) (position, bool) {
		if dp, err := stream.ParseDirectionalPosition(); err == nil {
			return position{dp: dp}, true
		} else if l, err := stream.ParseLength(); err == nil {
			return position{isLength: true, length: l}, true
		}
		return position{}, false
	}

	firstArg, hasFirst := parsePart(&stream)
	var secondArg position
	hasSecond := false
	var thirdArg Length
	hasThird := false

	if !stream.AtEnd() {
		stream.SkipSpaces()
		stream.parseListSeparator()
		var ok bool
		secondArg, ok = parsePart(&stream)
		if !ok {
			return TransformOrigin{}, okage.ErrSVGTypesTransformOriginInvalidParameters
		}
		hasSecond = true
	}

	if !stream.AtEnd() {
		stream.SkipSpaces()
		stream.parseListSeparator()
		l, err := stream.ParseLength()
		if err != nil {
			return TransformOrigin{}, okage.ErrSVGTypesTransformOriginInvalidParameters
		}
		thirdArg = l
		hasThird = true
	}

	stream.SkipSpaces()

	if !stream.AtEnd() {
		return TransformOrigin{}, okage.ErrSVGTypesTransformOriginInvalidParameters
	}

	var result TransformOrigin
	switch {
	case hasFirst && !hasSecond && !hasThird:
		p := firstArg
		var xOffset, yOffset Length
		if p.isHorizontal() {
			xOffset, yOffset = lengthFromPosition(p), LengthFromDirectionalPosition(DirectionalPositionCenter)
		} else {
			xOffset, yOffset = LengthFromDirectionalPosition(DirectionalPositionCenter), lengthFromPosition(p)
		}

		result = NewTransformOrigin(xOffset, yOffset, NewLength(0.0, LengthUnitPx))
	case hasFirst && hasSecond:
		p1, p2 := firstArg, secondArg
		if hasThird {
			if thirdArg.Unit == LengthUnitPercent {
				return TransformOrigin{}, okage.ErrSVGTypesTransformOriginZIndexIsPercentage
			}
		}

		length := NewLength(0.0, LengthUnitPx)
		if hasThird {
			length = thirdArg
		}

		check := func(pos position) bool {
			if pos.isLength {
				return true
			}
			return pos.dp == DirectionalPositionCenter
		}

		onlyKeywordIsCenter := check(p1) && check(p2)

		if onlyKeywordIsCenter {
			result = NewTransformOrigin(lengthFromPosition(p1), lengthFromPosition(p2), length)
		} else {
			// There is at least one of `left`, `right`, `top`, or `bottom`
			switch {
			case p1.isHorizontal() && p2.isVertical():
				result = NewTransformOrigin(lengthFromPosition(p1), lengthFromPosition(p2), length)
			case p1.isVertical() && p2.isHorizontal():
				result = NewTransformOrigin(lengthFromPosition(p2), lengthFromPosition(p1), length)
			default:
				return TransformOrigin{}, okage.ErrSVGTypesTransformOriginInvalidParameters
			}
		}
	default:
		panic("svgtypes: internal error: entered unreachable code")
	}

	return result, nil
}
