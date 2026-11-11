// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

// This file ports svgtypes src/color.rs.

import (
	"math"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/kurbo"
)

// Color is Color in svgtypes src/color.rs.
//
// Representation of the [`<color>`] type.
//
// [`<color>`]: https://www.w3.org/TR/css-color-3/
type Color struct {
	Red   uint8
	Green uint8
	Blue  uint8
	Alpha uint8
}

// NewColorRGB is Color::new_rgb in svgtypes src/color.rs.
//
// Constructs a new `Color` from RGB values.
func NewColorRGB(red, green, blue uint8) Color {
	return Color{
		Red:   red,
		Green: green,
		Blue:  blue,
		Alpha: 255,
	}
}

// newColorRGBF32 is Color::new_rgb_f32 in svgtypes src/color.rs. It
// constructs a new `Color` from RGB values in the range 0..1.
func newColorRGBF32(red, green, blue float32) Color {
	return Color{
		Red:   f32ToU8(roundF32(red * 255.0)),
		Green: f32ToU8(roundF32(green * 255.0)),
		Blue:  f32ToU8(roundF32(blue * 255.0)),
		Alpha: 255,
	}
}

// NewColorRGBA is Color::new_rgba in svgtypes src/color.rs.
//
// Constructs a new `Color` from RGBA values.
func NewColorRGBA(red, green, blue, alpha uint8) Color {
	return Color{
		Red:   red,
		Green: green,
		Blue:  blue,
		Alpha: alpha,
	}
}

// ColorBlack is Color::black in svgtypes src/color.rs.
//
// Constructs a new `Color` set to black.
func ColorBlack() Color {
	return NewColorRGB(0, 0, 0)
}

// ColorWhite is Color::white in svgtypes src/color.rs.
//
// Constructs a new `Color` set to white.
func ColorWhite() Color {
	return NewColorRGB(255, 255, 255)
}

// ColorGray is Color::gray in svgtypes src/color.rs.
//
// Constructs a new `Color` set to gray.
func ColorGray() Color {
	return NewColorRGB(128, 128, 128)
}

// ColorRed is Color::red in svgtypes src/color.rs.
//
// Constructs a new `Color` set to red.
func ColorRed() Color {
	return NewColorRGB(255, 0, 0)
}

// ColorGreen is Color::green in svgtypes src/color.rs.
//
// Constructs a new `Color` set to green.
func ColorGreen() Color {
	return NewColorRGB(0, 128, 0)
}

// ColorBlue is Color::blue in svgtypes src/color.rs.
//
// Constructs a new `Color` set to blue.
func ColorBlue() Color {
	return NewColorRGB(0, 0, 255)
}

// ParseColor is the impl FromStr for Color in svgtypes src/color.rs.
//
// Parses [CSS3](https://www.w3.org/TR/css-color-3/) `Color` from a string.
//
// # Errors
//
//   - Returns error if a color has an invalid format.
//   - Returns error if `<color>` is followed by `<icccolor>`. It's not supported.
//
// # Notes
//
//   - Any non-`hexdigit` bytes will be treated as `0`.
//   - The [SVG 1.1 spec] has an error.
//     There should be a `number`, not an `integer` for percent values ([details]).
//   - It also supports 4 digits and 8 digits hex notation from the
//     [CSS Color Module Level 4][css-color-4-hex].
//
// [SVG 1.1 spec]: https://www.w3.org/TR/SVG11/types.html#DataTypeColor
// [details]: https://lists.w3.org/Archives/Public/www-svg/2014Jan/0109.html
// [css-color-4-hex]: https://www.w3.org/TR/css-color-4/#hex-notation
func ParseColor(text string) (Color, error) {
	s := NewStream(text)
	color, err := s.ParseColor()
	if err != nil {
		return Color{}, err
	}

	// Check that we are at the end of the stream. Otherwise color can be followed by icccolor,
	// which is not supported.
	s.SkipSpaces()
	if !s.AtEnd() {
		return Color{}, errUnexpectedData(s.CalcCharPos())
	}

	return color, nil
}

// TryParseColor is Stream::try_parse_color in svgtypes src/color.rs.
//
// Tries to parse a color, but doesn't advance on error.
func (s *Stream) TryParseColor() (Color, bool) {
	s2 := *s
	if color, err := s2.ParseColor(); err == nil {
		*s = s2
		return color, true
	}
	return Color{}, false
}

// ParseColor is Stream::parse_color in svgtypes src/color.rs.
//
// Parses a color.
func (s *Stream) ParseColor() (Color, error) {
	s.SkipSpaces()

	color := ColorBlack()

	c, err := s.CurrByte()
	if err != nil {
		return Color{}, err
	}
	if c == '#' {
		// See https://www.w3.org/TR/css-color-4/#hex-notation
		s.Advance(1)
		colorStr := s.ConsumeBytes(func(_ *Stream, c byte) bool { return isHexDigit(c) })
		// get color data len until first space or stream end
		switch len(colorStr) {
		case 6:
			// #rrggbb
			color.Red = hexPair(colorStr[0], colorStr[1])
			color.Green = hexPair(colorStr[2], colorStr[3])
			color.Blue = hexPair(colorStr[4], colorStr[5])
		case 8:
			// #rrggbbaa
			color.Red = hexPair(colorStr[0], colorStr[1])
			color.Green = hexPair(colorStr[2], colorStr[3])
			color.Blue = hexPair(colorStr[4], colorStr[5])
			color.Alpha = hexPair(colorStr[6], colorStr[7])
		case 3:
			// #rgb
			color.Red = shortHex(colorStr[0])
			color.Green = shortHex(colorStr[1])
			color.Blue = shortHex(colorStr[2])
		case 4:
			// #rgba
			color.Red = shortHex(colorStr[0])
			color.Green = shortHex(colorStr[1])
			color.Blue = shortHex(colorStr[2])
			color.Alpha = shortHex(colorStr[3])
		default:
			return Color{}, okage.ErrSVGTypesInvalidValue
		}
	} else {
		// TODO: remove allocation
		// The ident holds only ASCII bytes, so strings.ToLower is
		// str::to_ascii_lowercase here.
		name := strings.ToLower(s.ConsumeASCIIIdent())
		switch name {
		case "rgb", "rgba":
			if err := s.ConsumeByte('('); err != nil {
				return Color{}, err
			}

			isPercent := false
			value, err := s.ParseNumber()
			if err != nil {
				return Color{}, err
			}
			if s.StartsWith("%") {
				s.Advance(1)
				isPercent = true
			}
			s.SkipSpaces()
			s.parseListSeparator()

			if isPercent {
				// The division and multiply are explicitly not collapsed, to ensure the red
				// component has the same rounding behavior as the green and blue components.
				color.Red = f64ToU8(kurbo.Round((value / 100.0) * 255.0))
				g, err := s.ParseListNumberOrPercent()
				if err != nil {
					return Color{}, err
				}
				color.Green = f64ToU8(kurbo.Round(g * 255.0))
				b, err := s.ParseListNumberOrPercent()
				if err != nil {
					return Color{}, err
				}
				color.Blue = f64ToU8(kurbo.Round(b * 255.0))
			} else {
				color.Red = f64ToU8(kurbo.Round(value))
				g, err := s.ParseListNumber()
				if err != nil {
					return Color{}, err
				}
				color.Green = f64ToU8(kurbo.Round(g))
				b, err := s.ParseListNumber()
				if err != nil {
					return Color{}, err
				}
				color.Blue = f64ToU8(kurbo.Round(b))
			}

			s.SkipSpaces()
			if !s.StartsWith(")") {
				if s.IsCurrByteEq('/') {
					s.Advance(1)
					s.SkipSpaces()
				}
				a, err := s.ParseListNumber()
				if err != nil {
					return Color{}, err
				}
				color.Alpha = f64ToU8(kurbo.Round(a * 255.0))
			}

			s.SkipSpaces()
			if err := s.ConsumeByte(')'); err != nil {
				return Color{}, err
			}
		case "hsl", "hsla":
			if err := s.ConsumeByte('('); err != nil {
				return Color{}, err
			}

			angle, err := s.ParseListAngle()
			if err != nil {
				return Color{}, err
			}
			hue := angle.ToDegrees()
			hue = math.Mod(math.Mod(hue, 360.0)+360.0, 360.0)

			sat, err := s.ParseListNumberOrPercent()
			if err != nil {
				return Color{}, err
			}
			saturation := f64Bound(0.0, sat, 1.0)
			light, err := s.ParseListNumberOrPercent()
			if err != nil {
				return Color{}, err
			}
			lightness := f64Bound(0.0, light, 1.0)

			color = hslToRGB(float32(hue)/60.0, float32(saturation), float32(lightness))

			s.SkipSpaces()
			if !s.StartsWith(")") {
				if s.IsCurrByteEq('/') {
					s.Advance(1)
					s.SkipSpaces()
				}
				a, err := s.ParseListNumberOrPercent()
				if err != nil {
					return Color{}, err
				}
				color.Alpha = f64ToU8(kurbo.Round(a * 255.0))
			}

			s.SkipSpaces()
			if err := s.ConsumeByte(')'); err != nil {
				return Color{}, err
			}
		case "hwb":
			if err := s.ConsumeByte('('); err != nil {
				return Color{}, err
			}

			angle, err := s.ParseAngle()
			if err != nil {
				return Color{}, err
			}
			hue := angle.ToDegrees()
			hue = math.Mod(math.Mod(hue, 360.0)+360.0, 360.0)

			w, err := s.parseHwbComponent()
			if err != nil {
				return Color{}, err
			}
			white := f64Bound(0.0, w, 1.0)
			b, err := s.parseHwbComponent()
			if err != nil {
				return Color{}, err
			}
			black := f64Bound(0.0, b, 1.0)

			color = hwbToRGB(float32(hue)/60.0, float32(white), float32(black))

			s.SkipSpaces()
			if s.IsCurrByteEq('/') {
				s.Advance(1)
				a, err := s.ParseNumberOrPercent()
				if err != nil {
					return Color{}, err
				}
				color.Alpha = f64ToU8(kurbo.Round(a * 255.0))
			}

			s.SkipSpaces()
			if err := s.ConsumeByte(')'); err != nil {
				return Color{}, err
			}
		default:
			c, ok := fromStr(name)
			if !ok {
				return Color{}, okage.ErrSVGTypesInvalidValue
			}
			color = c
		}
	}

	return color, nil
}

// fromHex is from_hex in svgtypes src/color.rs.
func fromHex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return '0'
	}
}

// shortHex is short_hex in svgtypes src/color.rs.
func shortHex(c byte) byte {
	h := fromHex(c)
	return (h << 4) | h
}

// hexPair is hex_pair in svgtypes src/color.rs.
func hexPair(c1, c2 byte) byte {
	h1 := fromHex(c1)
	h2 := fromHex(c2)
	return (h1 << 4) | h2
}

// hslToRGB is hsl_to_rgb in svgtypes src/color.rs.
//
// `hue` is in a 0..6 range, while `saturation` and `lightness` are in a 0..=1 range.
// Based on https://www.w3.org/TR/css-color-3/#hsl-color
func hslToRGB(hue, saturation, lightness float32) Color {
	red, green, blue := hslToRGBF32(hue, saturation, lightness)
	return newColorRGBF32(red, green, blue)
}

// hslToRGBF32 is hsl_to_rgb_f32 in svgtypes src/color.rs.
//
// `hue` is in a 0..6 range, while `saturation` and `lightness` are in a 0..1 range.
// Based on https://www.w3.org/TR/css-color-3/#hsl-color
func hslToRGBF32(hue, saturation, lightness float32) (float32, float32, float32) {
	var t2 float32
	if lightness <= 0.5 {
		t2 = lightness * (saturation + 1.0)
	} else {
		t2 = lightness + saturation - float32(lightness*saturation)
	}

	t1 := float32(lightness*2.0) - t2
	red := hueToRGB(t1, t2, hue+2.0)
	green := hueToRGB(t1, t2, hue)
	blue := hueToRGB(t1, t2, hue-2.0)
	return red, green, blue
}

// hueToRGB is hue_to_rgb in svgtypes src/color.rs.
func hueToRGB(t1, t2, hue float32) float32 {
	if hue < 0.0 {
		hue += 6.0
	}
	if hue >= 6.0 {
		hue -= 6.0
	}

	switch {
	case hue < 1.0:
		return float32((t2-t1)*hue) + t1
	case hue < 3.0:
		return t2
	case hue < 4.0:
		return float32((t2-t1)*(4.0-hue)) + t1
	default:
		return t1
	}
}

// hwbToRGB is hwb_to_rgb in svgtypes src/color.rs.
//
// `hue` is in a 0..6 range, while `white` and `black` are in a 0..1 range.
// Based on https://www.w3.org/TR/css-color-4/#hwb-to-rgb
func hwbToRGB(hue, white, black float32) Color {
	if white+black >= 1.0 {
		gray := white / (white + black)
		return newColorRGBF32(gray, gray, gray)
	}
	factor := 1.0 - white - black
	red, green, blue := hslToRGBF32(hue, 1.0, 0.5)
	red = float32(red*factor) + white
	green = float32(green*factor) + white
	blue = float32(blue*factor) + white
	return newColorRGBF32(red, green, blue)
}

// parseHwbComponent is Stream::parse_hwb_component in svgtypes
// src/color.rs.
func (s *Stream) parseHwbComponent() (float64, error) {
	n, err := s.ParseNumber()
	if err != nil {
		return 0, err
	}
	if s.StartsWith("%") {
		s.Advance(1)
	}
	return n / 100.0, nil
}

// f64Bound is f64_bound in svgtypes src/color.rs. It is f64::clamp, which
// keeps NaN.
func f64Bound(min, val, max float64) float64 {
	x := val
	if x < min {
		x = min
	}
	if x > max {
		x = max
	}
	return x
}

// The functions below have no Rust source. They follow the rules of Rust for
// a conversion with `as` and for f32::round.

// f64ToU8 is the conversion `x as u8` of Rust, which saturates: NaN and a
// value below 0 give 0, and a value above 255 gives 255. Other values are
// cut toward zero.
func f64ToU8(x float64) uint8 {
	switch {
	case x != x || x <= 0:
		return 0
	case x >= 255:
		return 255
	}
	return uint8(x)
}

// f32ToU8 is the conversion `x as u8` of Rust for an f32.
func f32ToU8(x float32) uint8 {
	return f64ToU8(float64(x))
}

// roundF32 is f32::round, which rounds half away from zero. The f64 result of
// math.Round of an f32 is exact, so the conversion back is exact.
func roundF32(x float32) float32 {
	return float32(math.Round(float64(x)))
}
