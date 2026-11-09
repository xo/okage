// Copyright 2018 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/angle.rs.

// AngleUnit is AngleUnit in svgtypes src/angle.rs.
//
// List of all SVG angle units.
type AngleUnit uint8

// The values of AngleUnit.
const (
	AngleUnitDegrees AngleUnit = iota
	AngleUnitGradians
	AngleUnitRadians
	AngleUnitTurns
)

// angleUnitNames holds the Rust name of each unit of AngleUnit.
var angleUnitNames = [...]string{
	AngleUnitDegrees:  "Degrees",
	AngleUnitGradians: "Gradians",
	AngleUnitRadians:  "Radians",
	AngleUnitTurns:    "Turns",
}

// String returns the Rust name of the unit.
func (u AngleUnit) String() string {
	if int(u) < len(angleUnitNames) {
		return angleUnitNames[u]
	}
	return "AngleUnit(?)"
}

// Angle is Angle in svgtypes src/angle.rs.
//
// Representation of the [`<angle>`] type.
//
// [`<angle>`]: https://www.w3.org/TR/css-values-3/#angles
type Angle struct {
	Number float64
	Unit   AngleUnit
}

// NewAngle is Angle::new in svgtypes src/angle.rs.
//
// Constructs a new angle.
func NewAngle(number float64, unit AngleUnit) Angle {
	return Angle{Number: number, Unit: unit}
}

// ToDegrees is Angle::to_degrees in svgtypes src/angle.rs.
//
// Converts angle to degrees.
func (a Angle) ToDegrees() float64 {
	switch a.Unit {
	case AngleUnitGradians:
		return a.Number * 180.0 / 200.0
	case AngleUnitRadians:
		return toDegrees(a.Number)
	case AngleUnitTurns:
		return a.Number * 360.0
	default:
		return a.Number
	}
}

// ParseAngle is the impl FromStr for Angle in svgtypes src/angle.rs.
func ParseAngle(text string) (Angle, error) {
	s := NewStream(text)
	l, err := s.ParseAngle()
	if err != nil {
		return Angle{}, err
	}

	if !s.AtEnd() {
		return Angle{}, errUnexpectedData(s.CalcCharPos())
	}

	return NewAngle(l.Number, l.Unit), nil
}

// ParseAngle is Stream::parse_angle in svgtypes src/angle.rs.
//
// Parses angle from the stream.
//
// <https://www.w3.org/TR/SVG2/types.html#InterfaceSVGAngle>
//
// # Notes
//
//   - Suffix must be lowercase, otherwise it will be an error.
func (s *Stream) ParseAngle() (Angle, error) {
	s.SkipSpaces()

	n, err := s.ParseNumber()
	if err != nil {
		return Angle{}, err
	}

	if s.AtEnd() {
		return NewAngle(n, AngleUnitDegrees), nil
	}

	var u AngleUnit
	switch {
	case s.StartsWith("deg"):
		s.Advance(3)
		u = AngleUnitDegrees
	case s.StartsWith("grad"):
		s.Advance(4)
		u = AngleUnitGradians
	case s.StartsWith("rad"):
		s.Advance(3)
		u = AngleUnitRadians
	case s.StartsWith("turn"):
		s.Advance(4)
		u = AngleUnitTurns
	default:
		u = AngleUnitDegrees
	}

	return NewAngle(n, u), nil
}

// The functions below have no Rust source. They are methods of f64 in the
// Rust core library.

// toDegrees is f64::to_degrees: x * (180.0f64 / consts::PI). The constant is
// the f64 division of 180 by the f64 value of pi, which is correctly rounded
// with respect to the true value of 180/pi.
func toDegrees(x float64) float64 {
	return x * (180.0 / pi)
}

// toRadians is f64::to_radians: x * RADS_PER_DEG, where RADS_PER_DEG is
// consts::PI / 180.0, the f64 division of the f64 value of pi by 180.
func toRadians(x float64) float64 {
	return x * (pi / 180.0)
}

// pi is core::f64::consts::PI. A typed constant holds the f64 value, so that
// a constant expression with it rounds as the Rust expression does.
const pi float64 = 3.14159265358979323846264338327950288
