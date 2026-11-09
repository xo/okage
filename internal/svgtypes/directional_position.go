// Copyright 2023 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/directional_position.rs.

// DirectionalPosition is DirectionalPosition in svgtypes
// src/directional_position.rs.
//
// List of all SVG directional positions.
type DirectionalPosition uint8

// The values of DirectionalPosition.
const (
	// DirectionalPositionTop is DirectionalPosition::Top.
	//
	// The `top` position.
	DirectionalPositionTop DirectionalPosition = iota
	// DirectionalPositionCenter is DirectionalPosition::Center.
	//
	// The `center` position.
	DirectionalPositionCenter
	// DirectionalPositionBottom is DirectionalPosition::Bottom.
	//
	// The `bottom` position.
	DirectionalPositionBottom
	// DirectionalPositionRight is DirectionalPosition::Right.
	//
	// The `right` position.
	DirectionalPositionRight
	// DirectionalPositionLeft is DirectionalPosition::Left.
	//
	// The `left` position.
	DirectionalPositionLeft
)

// directionalPositionNames holds the Rust name of each value of DirectionalPosition.
var directionalPositionNames = [...]string{
	DirectionalPositionTop:    "Top",
	DirectionalPositionCenter: "Center",
	DirectionalPositionBottom: "Bottom",
	DirectionalPositionRight:  "Right",
	DirectionalPositionLeft:   "Left",
}

// String returns the Rust name of the value.
func (p DirectionalPosition) String() string {
	if int(p) < len(directionalPositionNames) {
		return directionalPositionNames[p]
	}
	return "DirectionalPosition(?)"
}

// IsHorizontal is DirectionalPosition::is_horizontal in svgtypes
// src/directional_position.rs.
//
// Checks whether the value can be a horizontal position.
func (p DirectionalPosition) IsHorizontal() bool {
	return p == DirectionalPositionCenter || p == DirectionalPositionLeft || p == DirectionalPositionRight
}

// IsVertical is DirectionalPosition::is_vertical in svgtypes
// src/directional_position.rs.
//
// Checks whether the value can be a vertical position.
func (p DirectionalPosition) IsVertical() bool {
	return p == DirectionalPositionCenter || p == DirectionalPositionTop || p == DirectionalPositionBottom
}

// LengthFromDirectionalPosition is the impl From<DirectionalPosition> for
// Length in svgtypes src/directional_position.rs.
func LengthFromDirectionalPosition(value DirectionalPosition) Length {
	switch value {
	case DirectionalPositionLeft, DirectionalPositionTop:
		return NewLength(0.0, LengthUnitPercent)
	case DirectionalPositionRight, DirectionalPositionBottom:
		return NewLength(100.0, LengthUnitPercent)
	default:
		return NewLength(50.0, LengthUnitPercent)
	}
}

// ParseDirectionalPosition is the impl FromStr for DirectionalPosition in
// svgtypes src/directional_position.rs.
func ParseDirectionalPosition(text string) (DirectionalPosition, error) {
	s := NewStream(text)
	dirPos, err := s.ParseDirectionalPosition()
	if err != nil {
		return 0, err
	}

	if !s.AtEnd() {
		return 0, errUnexpectedData(s.CalcCharPos())
	}

	return dirPos, nil
}

// ParseDirectionalPosition is Stream::parse_directional_position in svgtypes
// src/directional_position.rs.
//
// Parses a directional position [`left`, `center`, `right`, `bottom`, `top`] from the stream.
func (s *Stream) ParseDirectionalPosition() (DirectionalPosition, error) {
	s.SkipSpaces()

	switch {
	case s.StartsWith("left"):
		s.Advance(4)
		return DirectionalPositionLeft, nil
	case s.StartsWith("right"):
		s.Advance(5)
		return DirectionalPositionRight, nil
	case s.StartsWith("top"):
		s.Advance(3)
		return DirectionalPositionTop, nil
	case s.StartsWith("bottom"):
		s.Advance(6)
		return DirectionalPositionBottom, nil
	case s.StartsWith("center"):
		s.Advance(6)
		return DirectionalPositionCenter, nil
	default:
		return 0, errInvalidString(
			[]string{
				s.SliceTail(),
				"left",
				"right",
				"top",
				"bottom",
				"center",
			},
			s.CalcCharPos(),
		)
	}
}
