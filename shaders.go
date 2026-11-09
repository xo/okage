// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import "github.com/xo/okage/path"

// SpreadMode is SpreadMode in tiny-skia src/shaders/mod.rs. It is a shader
// spreading mode.
type SpreadMode uint8

// The values of SpreadMode, in the order of the Rust enum.
const (
	// SpreadModePad is SpreadMode::Pad, the default. It replicates the edge
	// color if the shader draws outside of its original bounds.
	SpreadModePad SpreadMode = iota

	// SpreadModeReflect is SpreadMode::Reflect. It repeats the shader's image
	// horizontally and vertically, alternating mirror images so that adjacent
	// images always seam.
	SpreadModeReflect

	// SpreadModeRepeat is SpreadMode::Repeat. It repeats the shader's image
	// horizontally and vertically.
	SpreadModeRepeat
)

// String satisfies the fmt.Stringer interface. It returns the name of the
// Rust variant.
func (m SpreadMode) String() string {
	switch m {
	case SpreadModePad:
		return "Pad"
	case SpreadModeReflect:
		return "Reflect"
	case SpreadModeRepeat:
		return "Repeat"
	}
	return "SpreadMode(?)"
}

// Shader is Shader in tiny-skia src/shaders/mod.rs. A shader specifies the
// source color(s) for what is being drawn.
//
// If a paint has no shader, then the paint's color is used. If the paint has
// a shader, then the shader's color(s) are used instead, but they are
// modulated by the paint's alpha. This makes it easy to create a shader once
// (e.g. bitmap tiling or gradient) and then change its transparency without
// having to modify the original shader. Only the paint's alpha needs to be
// modified.
//
// Port: Shader is a Rust enum with data. In Go it is an interface with one
// type for each variant: SolidColor, LinearGradient, RadialGradient,
// SweepGradient and Pattern. Each type is a value type. A type switch on a
// Shader takes the place of a match on the enum. The unexported method keeps
// other types out. Transform and ApplyOpacity change a Rust shader in place.
// In Go they return the changed shader and leave the receiver as it was, so
// that a copy of a Paint does not share its shader with the original, as the
// clone of a Rust Paint does.
type Shader interface {
	// IsOpaque is Shader::is_opaque. It checks if the shader is guaranteed
	// to produce only opaque colors.
	IsOpaque() bool

	// Transform is Shader::transform. It returns the shader transformed by
	// ts.
	Transform(ts Transform) Shader

	// ApplyOpacity is Shader::apply_opacity. It returns the shader with its
	// opacity shifted by opacity.
	//
	// opacity is clamped to the 0..=1 range.
	//
	// This is roughly the same as Skia's SkPaint::setAlpha.
	//
	// Unlike Skia, we do not support global alpha/opacity, which in Skia is
	// set via the alpha channel of the SkPaint::fColor4f. Instead, you can
	// shift the opacity of the shader to whatever value you need.
	//
	//   - For SolidColor this function multiplies color.alpha by opacity.
	//   - For gradients this function multiplies all colors by opacity.
	//   - For Pattern this function multiplies Pattern::opacity by opacity.
	ApplyOpacity(opacity float32) Shader

	// pushStages is Shader::push_stages. If it returns false, then we draw
	// nothing (do not fall back to shader context).
	pushStages(cs ColorSpace, p *rasterPipelineBuilder) bool
}

// The types that implement Shader.
var (
	_ Shader = SolidColor{}
	_ Shader = LinearGradient{}
	_ Shader = RadialGradient{}
	_ Shader = SweepGradient{}
	_ Shader = Pattern{}
)

// SolidColor is Shader::SolidColor in tiny-skia src/shaders/mod.rs. It is a
// solid color shader.
//
// Unlike Skia, we do not have is_constant, because we do not have Color
// shaders.
type SolidColor Color

// IsOpaque is the arm Shader::SolidColor of Shader::is_opaque in tiny-skia
// src/shaders/mod.rs.
func (c SolidColor) IsOpaque() bool {
	return Color(c).IsOpaque()
}

// pushStages is the arm Shader::SolidColor of Shader::push_stages in
// tiny-skia src/shaders/mod.rs.
func (c SolidColor) pushStages(cs ColorSpace, p *rasterPipelineBuilder) bool {
	color := cs.expandColor(Color(c)).Premultiply()
	p.pushUniformColor(color)
	return true
}

// Transform is the arm Shader::SolidColor of Shader::transform in tiny-skia
// src/shaders/mod.rs. It returns c.
func (c SolidColor) Transform(_ Transform) Shader {
	return c
}

// ApplyOpacity is the arm Shader::SolidColor of Shader::apply_opacity in
// tiny-skia src/shaders/mod.rs.
func (c SolidColor) ApplyOpacity(opacity float32) Shader {
	color := Color(c)
	color.ApplyOpacity(opacity)
	return SolidColor(color)
}

// Transform is the arm Shader::LinearGradient of Shader::transform in
// tiny-skia src/shaders/mod.rs.
func (g LinearGradient) Transform(ts Transform) Shader {
	g.base.transform = g.base.transform.PostConcat(ts)
	return g
}

// ApplyOpacity is the arm Shader::LinearGradient of Shader::apply_opacity in
// tiny-skia src/shaders/mod.rs.
func (g LinearGradient) ApplyOpacity(opacity float32) Shader {
	g.base.applyOpacity(opacity)
	return g
}

// Transform is the arm Shader::RadialGradient of Shader::transform in
// tiny-skia src/shaders/mod.rs.
func (g RadialGradient) Transform(ts Transform) Shader {
	g.base.transform = g.base.transform.PostConcat(ts)
	return g
}

// ApplyOpacity is the arm Shader::RadialGradient of Shader::apply_opacity in
// tiny-skia src/shaders/mod.rs.
func (g RadialGradient) ApplyOpacity(opacity float32) Shader {
	g.base.applyOpacity(opacity)
	return g
}

// Transform is the arm Shader::SweepGradient of Shader::transform in
// tiny-skia src/shaders/mod.rs.
func (g SweepGradient) Transform(ts Transform) Shader {
	g.base.transform = g.base.transform.PostConcat(ts)
	return g
}

// ApplyOpacity is the arm Shader::SweepGradient of Shader::apply_opacity in
// tiny-skia src/shaders/mod.rs.
func (g SweepGradient) ApplyOpacity(opacity float32) Shader {
	g.base.applyOpacity(opacity)
	return g
}

// Transform is the arm Shader::Pattern of Shader::transform in tiny-skia
// src/shaders/mod.rs.
func (p Pattern) Transform(ts Transform) Shader {
	p.transform = p.transform.PostConcat(ts)
	return p
}

// ApplyOpacity is the arm Shader::Pattern of Shader::apply_opacity in
// tiny-skia src/shaders/mod.rs.
func (p Pattern) ApplyOpacity(opacity float32) Shader {
	o, ok := path.NewNormalizedF32(p.opacity.Get() * path.Bound(opacity, 0.0, 1.0))
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	p.opacity = o
	return p
}
