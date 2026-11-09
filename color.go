// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// alphaU8 is AlphaU8 in tiny-skia src/color.rs. It is an 8-bit type for an
// alpha value. 255 is 100% opaque, zero is 100% transparent.
type alphaU8 = uint8

// AlphaU8Transparent is ALPHA_U8_TRANSPARENT in tiny-skia src/color.rs. It
// represents a fully transparent AlphaU8 value.
const AlphaU8Transparent uint8 = 0x00

// AlphaU8Opaque is ALPHA_U8_OPAQUE in tiny-skia src/color.rs. It represents a
// fully opaque AlphaU8 value.
const AlphaU8Opaque uint8 = 0xFF

// AlphaTransparent is ALPHA_TRANSPARENT in tiny-skia src/color.rs. It
// represents a fully transparent alpha value.
const AlphaTransparent = path.NormalizedF32Zero

// AlphaOpaque is ALPHA_OPAQUE in tiny-skia src/color.rs. It represents a fully
// opaque alpha value.
const AlphaOpaque = path.NormalizedF32One

// ColorU8 is ColorU8 in tiny-skia src/color.rs. It is a 32-bit RGBA color
// value.
//
// Byte order: RGBA.
type ColorU8 struct {
	rgba [4]uint8
}

// ColorU8FromRGBA is ColorU8::from_rgba in tiny-skia src/color.rs. It creates
// a new color.
func ColorU8FromRGBA(r, g, b, a uint8) ColorU8 {
	return ColorU8{rgba: [4]uint8{r, g, b, a}}
}

// Red is ColorU8::red in tiny-skia src/color.rs. It returns the red component
// of the color.
func (c ColorU8) Red() uint8 {
	return c.rgba[0]
}

// Green is ColorU8::green in tiny-skia src/color.rs. It returns the green
// component of the color.
func (c ColorU8) Green() uint8 {
	return c.rgba[1]
}

// Blue is ColorU8::blue in tiny-skia src/color.rs. It returns the blue
// component of the color.
func (c ColorU8) Blue() uint8 {
	return c.rgba[2]
}

// Alpha is ColorU8::alpha in tiny-skia src/color.rs. It returns the alpha
// component of the color.
func (c ColorU8) Alpha() uint8 {
	return c.rgba[3]
}

// IsOpaque is ColorU8::is_opaque in tiny-skia src/color.rs. It returns true
// if the alpha is 255.
func (c ColorU8) IsOpaque() bool {
	return c.Alpha() == AlphaU8Opaque
}

// Premultiply is ColorU8::premultiply in tiny-skia src/color.rs. It converts
// the color into a premultiplied color.
func (c ColorU8) Premultiply() PremultipliedColorU8 {
	a := c.Alpha()
	if a != AlphaU8Opaque {
		return premultipliedColorU8FromRGBAUnchecked(
			premultiplyU8(c.Red(), a),
			premultiplyU8(c.Green(), a),
			premultiplyU8(c.Blue(), a),
			a,
		)
	}
	return premultipliedColorU8FromRGBAUnchecked(c.Red(), c.Green(), c.Blue(), a)
}

// PremultipliedColorU8 is PremultipliedColorU8 in tiny-skia src/color.rs. It
// is a 32-bit premultiplied RGBA color value.
//
// Byte order: RGBA.
type PremultipliedColorU8 struct {
	rgba [4]uint8
}

// PremultipliedColorU8Transparent is PremultipliedColorU8::TRANSPARENT in
// tiny-skia src/color.rs. It is a transparent color.
var PremultipliedColorU8Transparent = premultipliedColorU8FromRGBAUnchecked(0, 0, 0, 0)

// PremultipliedColorU8FromRGBA is PremultipliedColorU8::from_rgba in
// tiny-skia src/color.rs. It creates a new premultiplied color.
//
// The RGB components must be <= alpha. If not, it returns false.
func PremultipliedColorU8FromRGBA(r, g, b, a uint8) (PremultipliedColorU8, bool) {
	if r <= a && g <= a && b <= a {
		return PremultipliedColorU8{rgba: [4]uint8{r, g, b, a}}, true
	}
	return PremultipliedColorU8{}, false
}

// premultipliedColorU8FromRGBAUnchecked is
// PremultipliedColorU8::from_rgba_unchecked in tiny-skia src/color.rs. It
// creates a new color and does not check the components.
func premultipliedColorU8FromRGBAUnchecked(r, g, b, a uint8) PremultipliedColorU8 {
	return PremultipliedColorU8{rgba: [4]uint8{r, g, b, a}}
}

// Red is PremultipliedColorU8::red in tiny-skia src/color.rs. It returns the
// red component of the color.
//
// The value is <= alpha.
func (c PremultipliedColorU8) Red() uint8 {
	return c.rgba[0]
}

// Green is PremultipliedColorU8::green in tiny-skia src/color.rs. It returns
// the green component of the color.
//
// The value is <= alpha.
func (c PremultipliedColorU8) Green() uint8 {
	return c.rgba[1]
}

// Blue is PremultipliedColorU8::blue in tiny-skia src/color.rs. It returns the
// blue component of the color.
//
// The value is <= alpha.
func (c PremultipliedColorU8) Blue() uint8 {
	return c.rgba[2]
}

// Alpha is PremultipliedColorU8::alpha in tiny-skia src/color.rs. It returns
// the alpha component of the color.
func (c PremultipliedColorU8) Alpha() uint8 {
	return c.rgba[3]
}

// IsOpaque is PremultipliedColorU8::is_opaque in tiny-skia src/color.rs. It
// returns true if the alpha is 255.
func (c PremultipliedColorU8) IsOpaque() bool {
	return c.Alpha() == AlphaU8Opaque
}

// Demultiply is PremultipliedColorU8::demultiply in tiny-skia src/color.rs.
// It returns a demultiplied color.
func (c PremultipliedColorU8) Demultiply() ColorU8 {
	alpha := c.Alpha()
	if alpha == AlphaU8Opaque {
		return ColorU8(c)
	}
	a := float64(alpha) / 255.0
	return ColorU8FromRGBA(
		trunc64U8(float64(c.Red())/a+0.5),
		trunc64U8(float64(c.Green())/a+0.5),
		trunc64U8(float64(c.Blue())/a+0.5),
		alpha,
	)
}

// trunc64U8 is the Rust cast "f as u8" for an f64. It truncates toward zero
// and saturates. It ports no upstream function.
func trunc64U8(f float64) uint8 {
	n := wide.Trunc64I32(f)
	switch {
	case n < 0:
		return 0
	case n > 255:
		return 255
	}
	return uint8(n)
}

// Color is Color in tiny-skia src/color.rs. It is an RGBA color value that
// holds four floating point components.
//
// All values are in the range 0..=1.
type Color struct {
	r path.NormalizedF32
	g path.NormalizedF32
	b path.NormalizedF32
	a path.NormalizedF32
}

// nvZero is NV_ZERO in tiny-skia src/color.rs.
const nvZero = path.NormalizedF32Zero

// nvOne is NV_ONE in tiny-skia src/color.rs.
const nvOne = path.NormalizedF32One

// ColorTransparent is Color::TRANSPARENT in tiny-skia src/color.rs. It is a
// transparent color.
var ColorTransparent = Color{r: nvZero, g: nvZero, b: nvZero, a: nvZero}

// ColorBlack is Color::BLACK in tiny-skia src/color.rs. It is a black color.
var ColorBlack = Color{r: nvZero, g: nvZero, b: nvZero, a: nvOne}

// ColorWhite is Color::WHITE in tiny-skia src/color.rs. It is a white color.
var ColorWhite = Color{r: nvOne, g: nvOne, b: nvOne, a: nvOne}

// ColorFromRGBAUnchecked is Color::from_rgba_unchecked in tiny-skia
// src/color.rs. It creates a new color from 4 components.
//
// All values must be in the range 0..=1. It does not check them.
func ColorFromRGBAUnchecked(r, g, b, a float32) Color {
	return Color{
		r: path.NormalizedF32(r),
		g: path.NormalizedF32(g),
		b: path.NormalizedF32(b),
		a: path.NormalizedF32(a),
	}
}

// ColorFromRGBA is Color::from_rgba in tiny-skia src/color.rs. It creates a
// new color from 4 components.
//
// All values must be in the range 0..=1. If not, it returns false.
func ColorFromRGBA(r, g, b, a float32) (Color, bool) {
	nr, ok := path.NewNormalizedF32(r)
	if !ok {
		return Color{}, false
	}
	ng, ok := path.NewNormalizedF32(g)
	if !ok {
		return Color{}, false
	}
	nb, ok := path.NewNormalizedF32(b)
	if !ok {
		return Color{}, false
	}
	na, ok := path.NewNormalizedF32(a)
	if !ok {
		return Color{}, false
	}
	return Color{r: nr, g: ng, b: nb, a: na}, true
}

// ColorFromRGBA8 is Color::from_rgba8 in tiny-skia src/color.rs. It creates a
// new color from 4 components.
//
// Each uint8 is divided by 255 to get the float component.
func ColorFromRGBA8(r, g, b, a uint8) Color {
	return Color{
		r: path.NormalizedF32FromU8(r),
		g: path.NormalizedF32FromU8(g),
		b: path.NormalizedF32FromU8(b),
		a: path.NormalizedF32FromU8(a),
	}
}

// Red is Color::red in tiny-skia src/color.rs. It returns the red component
// of the color.
//
// The value is guaranteed to be in the range 0..=1.
func (c Color) Red() float32 {
	return c.r.Get()
}

// Green is Color::green in tiny-skia src/color.rs. It returns the green
// component of the color.
//
// The value is guaranteed to be in the range 0..=1.
func (c Color) Green() float32 {
	return c.g.Get()
}

// Blue is Color::blue in tiny-skia src/color.rs. It returns the blue component
// of the color.
//
// The value is guaranteed to be in the range 0..=1.
func (c Color) Blue() float32 {
	return c.b.Get()
}

// Alpha is Color::alpha in tiny-skia src/color.rs. It returns the alpha
// component of the color.
//
// The value is guaranteed to be in the range 0..=1.
func (c Color) Alpha() float32 {
	return c.a.Get()
}

// SetRed is Color::set_red in tiny-skia src/color.rs. It sets the red
// component.
//
// The new value is clipped to the range 0..=1.
func (c *Color) SetRed(v float32) {
	c.r = path.NormalizedF32Clamped(v)
}

// SetGreen is Color::set_green in tiny-skia src/color.rs. It sets the green
// component.
//
// The new value is clipped to the range 0..=1.
func (c *Color) SetGreen(v float32) {
	c.g = path.NormalizedF32Clamped(v)
}

// SetBlue is Color::set_blue in tiny-skia src/color.rs. It sets the blue
// component.
//
// The new value is clipped to the range 0..=1.
func (c *Color) SetBlue(v float32) {
	c.b = path.NormalizedF32Clamped(v)
}

// SetAlpha is Color::set_alpha in tiny-skia src/color.rs. It sets the alpha
// component.
//
// The new value is clipped to the range 0..=1.
func (c *Color) SetAlpha(v float32) {
	c.a = path.NormalizedF32Clamped(v)
}

// ApplyOpacity is Color::apply_opacity in tiny-skia src/color.rs. It shifts
// the opacity of the color.
//
// It multiplies the alpha of the color by opacity. opacity is clamped to the
// range 0..=1 first. The final alpha is also clamped.
func (c *Color) ApplyOpacity(opacity float32) {
	c.a = path.NormalizedF32Clamped(float32(c.a.Get() * path.Bound(opacity, 0.0, 1.0)))
}

// IsOpaque is Color::is_opaque in tiny-skia src/color.rs. It returns true if
// the alpha is 1.0.
func (c Color) IsOpaque() bool {
	return c.a == AlphaOpaque
}

// Premultiply is Color::premultiply in tiny-skia src/color.rs. It converts the
// color into a premultiplied color.
func (c Color) Premultiply() PremultipliedColor {
	if c.IsOpaque() {
		return PremultipliedColor(c)
	}
	return PremultipliedColor{
		r: path.NormalizedF32Clamped(float32(c.r.Get() * c.a.Get())),
		g: path.NormalizedF32Clamped(float32(c.g.Get() * c.a.Get())),
		b: path.NormalizedF32Clamped(float32(c.b.Get() * c.a.Get())),
		a: c.a,
	}
}

// ToColorU8 is Color::to_color_u8 in tiny-skia src/color.rs. It converts the
// color into a ColorU8.
func (c Color) ToColorU8() ColorU8 {
	v := colorF32ToU8(c.r, c.g, c.b, c.a)
	return ColorU8FromRGBA(v[0], v[1], v[2], v[3])
}

// PremultipliedColor is PremultipliedColor in tiny-skia src/color.rs. It is a
// premultiplied RGBA color value that holds four floating point components.
//
// All values are in the range 0..=1, and the RGB components are <= A.
type PremultipliedColor struct {
	r path.NormalizedF32
	g path.NormalizedF32
	b path.NormalizedF32
	a path.NormalizedF32
}

// Red is PremultipliedColor::red in tiny-skia src/color.rs. It returns the red
// component of the color.
//
// The value is guaranteed to be in the range 0..=1, and it is <= alpha.
func (c PremultipliedColor) Red() float32 {
	return c.r.Get()
}

// Green is PremultipliedColor::green in tiny-skia src/color.rs. It returns the
// green component of the color.
//
// The value is guaranteed to be in the range 0..=1, and it is <= alpha.
func (c PremultipliedColor) Green() float32 {
	return c.g.Get()
}

// Blue is PremultipliedColor::blue in tiny-skia src/color.rs. It returns the
// blue component of the color.
//
// The value is guaranteed to be in the range 0..=1, and it is <= alpha.
func (c PremultipliedColor) Blue() float32 {
	return c.b.Get()
}

// Alpha is PremultipliedColor::alpha in tiny-skia src/color.rs. It returns the
// alpha component of the color.
//
// The value is guaranteed to be in the range 0..=1.
func (c PremultipliedColor) Alpha() float32 {
	return c.a.Get()
}

// Demultiply is PremultipliedColor::demultiply in tiny-skia src/color.rs. It
// returns a demultiplied color.
func (c PremultipliedColor) Demultiply() Color {
	a := c.a.Get()
	if a == 0.0 {
		return ColorTransparent
	}
	return Color{
		r: path.NormalizedF32Clamped(c.r.Get() / a),
		g: path.NormalizedF32Clamped(c.g.Get() / a),
		b: path.NormalizedF32Clamped(c.b.Get() / a),
		a: c.a,
	}
}

// ToColorU8 is PremultipliedColor::to_color_u8 in tiny-skia src/color.rs. It
// converts the color into a PremultipliedColorU8.
func (c PremultipliedColor) ToColorU8() PremultipliedColorU8 {
	v := colorF32ToU8(c.r, c.g, c.b, c.a)
	return premultipliedColorU8FromRGBAUnchecked(v[0], v[1], v[2], v[3])
}

// premultiplyU8 is premultiply_u8 in tiny-skia src/color.rs. It returns
// a*b/255, and rounds any fractional bits.
func premultiplyU8(c, a uint8) uint8 {
	prod := uint32(c)*uint32(a) + 128
	return uint8((prod + (prod >> 8)) >> 8)
}

// colorF32ToU8 is color_f32_to_u8 in tiny-skia src/color.rs.
func colorF32ToU8(r, g, b, a path.NormalizedF32) [4]uint8 {
	return [4]uint8{
		wide.TruncU8(float32(r.Get()*255.0) + 0.5),
		wide.TruncU8(float32(g.Get()*255.0) + 0.5),
		wide.TruncU8(float32(b.Get()*255.0) + 0.5),
		wide.TruncU8(float32(a.Get()*255.0) + 0.5),
	}
}

// ColorSpace is ColorSpace in tiny-skia src/color.rs. It is the color space
// that is used to interpret the values of pixels.
//
// This is a very limited subset of SkColorSpace.
type ColorSpace uint8

const (
	// ColorSpaceLinear is linear RGB, the default. It assumes that #7f7f7f
	// is half as bright as #ffffff.
	ColorSpaceLinear ColorSpace = iota

	// ColorSpaceGamma2 applies a gamma factor of 2.
	//
	// This is the fastest gamma correction to apply, and it produces
	// reasonable quality blending.
	ColorSpaceGamma2

	// ColorSpaceSimpleSRGB applies a gamma factor of 2.2.
	//
	// This is also known as "simple sRGB". It is very close to the actual
	// gamma function that the sRGB color space specifies, but much faster to
	// compute.
	ColorSpaceSimpleSRGB

	// ColorSpaceFullSRGBGamma applies the full sRGB gamma function.
	//
	// This does not convert the RGB colors to CIE XYZ for blending. It only
	// applies the (full) gamma function.
	ColorSpaceFullSRGBGamma
)

// String satisfies the fmt.Stringer interface. It returns the name of the
// Rust variant.
func (cs ColorSpace) String() string {
	switch cs {
	case ColorSpaceLinear:
		return "Linear"
	case ColorSpaceGamma2:
		return "Gamma2"
	case ColorSpaceSimpleSRGB:
		return "SimpleSRGB"
	case ColorSpaceFullSRGBGamma:
		return "FullSRGBGamma"
	}
	return "ColorSpace(?)"
}

// expandChannel is ColorSpace::expand_channel in tiny-skia src/color.rs.
func (cs ColorSpace) expandChannel(x path.NormalizedF32) path.NormalizedF32 {
	switch cs {
	case ColorSpaceGamma2:
		return x.Mul(x)
	case ColorSpaceSimpleSRGB:
		return path.NormalizedF32Clamped(approxPowf(x.Get(), 2.2))
	case ColorSpaceFullSRGBGamma:
		v := x.Get()
		if v <= 0.04045 {
			v /= 12.92
		} else {
			v = approxPowf((v+0.055)/1.055, 2.4)
		}
		return path.NormalizedF32Clamped(v)
	}
	// ColorSpaceLinear
	return x
}

// expandColor is ColorSpace::expand_color in tiny-skia src/color.rs.
func (cs ColorSpace) expandColor(color Color) Color {
	color.r = cs.expandChannel(color.r)
	color.g = cs.expandChannel(color.g)
	color.b = cs.expandChannel(color.b)
	return color
}

// compressChannel is ColorSpace::compress_channel in tiny-skia src/color.rs.
//
//nolint:unused // upstream calls it only in a debug_assert
func (cs ColorSpace) compressChannel(x path.NormalizedF32) path.NormalizedF32 {
	switch cs {
	case ColorSpaceGamma2:
		return path.NormalizedF32Clamped(sqrtF32(x.Get()))
	case ColorSpaceSimpleSRGB:
		return path.NormalizedF32Clamped(approxPowf(x.Get(), 0.45454545))
	case ColorSpaceFullSRGBGamma:
		v := x.Get()
		if v <= 0.0031308 {
			v = float32(v * 12.92)
		} else {
			v = float32(approxPowf(v, 0.416666666)*1.055) - 0.055
		}
		return path.NormalizedF32Clamped(v)
	}
	// ColorSpaceLinear
	return x
}

// expandStage is ColorSpace::expand_stage in tiny-skia src/color.rs.
func (cs ColorSpace) expandStage() (stage, bool) {
	switch cs {
	case ColorSpaceGamma2:
		return stageGammaExpand2, true
	case ColorSpaceSimpleSRGB:
		return stageGammaExpand22, true
	case ColorSpaceFullSRGBGamma:
		return stageGammaExpandSrgb, true
	}
	// ColorSpaceLinear
	return 0, false
}

// expandDestStage is ColorSpace::expand_dest_stage in tiny-skia src/color.rs.
func (cs ColorSpace) expandDestStage() (stage, bool) {
	switch cs {
	case ColorSpaceGamma2:
		return stageGammaExpandDestination2, true
	case ColorSpaceSimpleSRGB:
		return stageGammaExpandDestination22, true
	case ColorSpaceFullSRGBGamma:
		return stageGammaExpandDestinationSrgb, true
	}
	// ColorSpaceLinear
	return 0, false
}

// compressStage is ColorSpace::compress_stage in tiny-skia src/color.rs.
func (cs ColorSpace) compressStage() (stage, bool) {
	switch cs {
	case ColorSpaceGamma2:
		return stageGammaCompress2, true
	case ColorSpaceSimpleSRGB:
		return stageGammaCompress22, true
	case ColorSpaceFullSRGBGamma:
		return stageGammaCompressSrgb, true
	}
	// ColorSpaceLinear
	return 0, false
}
