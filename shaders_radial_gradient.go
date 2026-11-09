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

// focalData is FocalData in tiny-skia src/shaders/radial_gradient.rs.
type focalData struct {
	r1        float32 // r1 after mapping focal point to (0, 0)
	focalX    float32 // f
	isSwapped bool
}

// set is FocalData::set in tiny-skia src/shaders/radial_gradient.rs.
func (fd *focalData) set(r0, r1 float32, matrix *Transform) bool {
	fd.isSwapped = false
	fd.focalX = r0 / (r0 - r1)

	if path.IsNearlyZero(fd.focalX - 1.0) {
		// swap r0, r1
		//
		// Port: nothing reads r0 after the swap, so Go sets only r1.
		*matrix = matrix.PostTranslate(-1.0, 0.0).PostScale(-1.0, 1.0)
		r1 = r0

		fd.focalX = 0.0 // because r0 is now 0
		fd.isSwapped = true
	}

	// Map {focal point, (1, 0)} to {(0, 0), (1, 0)}
	from := [2]Point{path.PointFromXY(fd.focalX, 0.0), path.PointFromXY(1.0, 0.0)}
	to := [2]Point{path.PointFromXY(0.0, 0.0), path.PointFromXY(1.0, 0.0)}

	focalMatrix, ok := tsFromPolyToPoly(from[0], from[1], to[0], to[1])
	if !ok {
		return false
	}

	*matrix = matrix.PostConcat(focalMatrix)
	fd.r1 = r1 / absF32(1.0-fd.focalX) // focalMatrix has a scale of 1/(1-f).

	// The following transformations are just to accelerate the shader computation by saving
	// some arithmetic operations.
	if fd.isFocalOnCircle() {
		*matrix = matrix.PostScale(0.5, 0.5)
	} else {
		*matrix = matrix.PostScale(
			fd.r1/(float32(fd.r1*fd.r1)-1.0),
			1.0/sqrtF32(absF32(float32(fd.r1*fd.r1)-1.0)),
		)
	}

	*matrix = matrix.PostScale(absF32(1.0-fd.focalX), absF32(1.0-fd.focalX)) // scale |1 - f|

	return true
}

// isFocalOnCircle is FocalData::is_focal_on_circle in tiny-skia
// src/shaders/radial_gradient.rs.
func (fd *focalData) isFocalOnCircle() bool {
	return path.IsNearlyZero(1.0 - fd.r1)
}

// isWellBehaved is FocalData::is_well_behaved in tiny-skia
// src/shaders/radial_gradient.rs.
func (fd *focalData) isWellBehaved() bool {
	return !fd.isFocalOnCircle() && fd.r1 > 1.0
}

// isNativelyFocal is FocalData::is_natively_focal in tiny-skia
// src/shaders/radial_gradient.rs.
func (fd *focalData) isNativelyFocal() bool {
	return path.IsNearlyZero(fd.focalX)
}

// gradientTypeKind is the variant of the enum GradientType in tiny-skia
// src/shaders/radial_gradient.rs.
type gradientTypeKind uint8

// The values of gradientTypeKind.
const (
	// gradientTypeRadial is GradientType::Radial.
	gradientTypeRadial gradientTypeKind = iota
	// gradientTypeStrip is GradientType::Strip.
	gradientTypeStrip
	// gradientTypeFocal is GradientType::Focal.
	gradientTypeFocal
)

// gradientType is GradientType in tiny-skia src/shaders/radial_gradient.rs.
//
// Port: the Rust enum has data. Go holds the variant in kind and the data of
// every variant in the other fields. Only the fields of the variant of kind
// are valid.
type gradientType struct {
	kind gradientTypeKind

	// radius1 and radius2 are the data of GradientType::Radial.
	radius1 float32
	radius2 float32

	// scaledR0 is the data of GradientType::Strip. It is the radius of the
	// first circle scaled by the distance between centers (r0 / d_center).
	scaledR0 float32

	// focal is the data of GradientType::Focal.
	focal focalData
}

// RadialGradient is RadialGradient in tiny-skia
// src/shaders/radial_gradient.rs. It is a 2-point conical gradient shader.
type RadialGradient struct {
	base         gradient
	gradientType gradientType
}

// NewRadialGradient is RadialGradient::new in tiny-skia
// src/shaders/radial_gradient.rs. It creates a new two-point conical
// gradient shader.
//
// A two-point conical gradient (also known as a radial gradient)
// interpolates colors between two circles defined by their center points and
// radii.
//
// It returns a SolidColor when:
//   - len(stops) == 1
//
// It returns false when:
//   - stops is empty
//   - startRadius < 0 or endRadius < 0
//   - transform is not invertible
//   - The gradient is degenerate (both radii and centers are equal, except
//     in specific pad mode cases)
func NewRadialGradient(
	startPoint Point,
	startRadius float32,
	endPoint Point,
	endRadius float32,
	stops []GradientStop,
	mode SpreadMode,
	transform Transform,
) (Shader, bool) {
	if startRadius < 0.0 || endRadius < 0.0 {
		return nil, false
	}

	switch len(stops) {
	case 0:
		return nil, false
	case 1:
		return SolidColor(stops[0].color), true
	}

	if _, ok := transform.Invert(); !ok {
		return nil, false
	}

	length := startPoint.Sub(endPoint).Length()
	if !isFiniteF32(length) {
		return nil, false
	}
	if path.IsNearlyZeroWithinTolerance(length, degenerateThreshold) {
		if path.IsNearlyEqualWithinTolerance(startRadius, endRadius, degenerateThreshold) {
			// Degenerate case, where the interpolation region area approaches zero. The proper
			// behavior depends on the tile mode, which is consistent with the default degenerate
			// gradient behavior, except when mode = clamp and the radii > 0.
			if mode == SpreadModePad && endRadius > degenerateThreshold {
				// The interpolation region becomes an infinitely thin ring at the radius, so the
				// final gradient is the first color repeated from p=0 to 1, and then a hard
				// stop switching to the last color at p=1.
				startColor := stops[0].color
				endColor := stops[len(stops)-1].color
				newStops := []GradientStop{
					NewGradientStop(0.0, startColor),
					NewGradientStop(1.0, startColor),
					NewGradientStop(1.0, endColor),
				}
				// If the center positions are the same, then the gradient is the radial variant
				// of a 2 pt conical gradient, an actual radial gradient (startRadius == 0), or
				// it is fully degenerate (startRadius == endRadius).
				// We can treat this gradient as a simple radial, which is faster. If we got
				// here, we know that endRadius is not equal to 0, so this produces a meaningful
				// gradient
				return newRadialGradientUnchecked(
					startPoint,
					endRadius,
					newStops,
					mode,
					transform,
				)
			}
			// TODO: Consider making a degenerate gradient
			return nil, false
		}

		if path.IsNearlyZeroWithinTolerance(startRadius, degenerateThreshold) {
			// If the center positions are the same, then the gradient
			// is the radial variant of a 2 pt conical gradient,
			// an actual radial gradient (startRadius == 0),
			// or it is fully degenerate (startRadius == endRadius).
			// We can treat this gradient as a simple radial, which is faster. If we got here,
			// we know that endRadius is not equal to 0, so this produces a meaningful gradient.
			return newRadialGradientUnchecked(startPoint, endRadius, stops, mode, transform)
		}
	}

	return createRadialGradient(
		startPoint,
		startRadius,
		endPoint,
		endRadius,
		stops,
		mode,
		transform,
	)
}

// newRadialGradientUnchecked is RadialGradient::new_radial_unchecked in
// tiny-skia src/shaders/radial_gradient.rs. It creates a simple radial
// gradient shader without validation.
//
// This is an optimized path for creating radial gradients when the start
// radius is 0 and the gradient is known to be valid. The function computes
// the points-to-unit transformation internally based on the center point and
// radius.
//
// The parameters:
//   - center: The center point of the radial gradient
//   - radius: The radius of the gradient (assumed to be > 0)
//   - stops: Color stops for the gradient (assumed to have length >= 2)
//   - mode: How the gradient extends beyond its bounds
//   - transform: The gradient's transformation matrix (assumed to be invertible)
func newRadialGradientUnchecked(
	center Point,
	radius float32,
	stops []GradientStop,
	mode SpreadMode,
	transform Transform,
) (Shader, bool) {
	inv := path.Invert(radius)
	pointsToUnit := path.TransformFromTranslate(-center.X, -center.Y).PostScale(inv, inv)

	return RadialGradient{
		base: newGradient(stops, mode, transform, pointsToUnit),
		gradientType: gradientType{
			kind:    gradientTypeRadial,
			radius1: 0.0,
			radius2: radius,
		},
	}, true
}

// IsOpaque is the arm Shader::RadialGradient of Shader::is_opaque in
// tiny-skia src/shaders/mod.rs.
//
// A radial gradient can have points that are "undefined" so we just assume
// that it is not opaque.
func (g RadialGradient) IsOpaque() bool {
	return false
}

// pushStages is RadialGradient::push_stages in tiny-skia
// src/shaders/radial_gradient.rs.
func (g RadialGradient) pushStages(cs ColorSpace, p *rasterPipelineBuilder) bool {
	var p0, p1 float32
	switch g.gradientType.kind {
	case gradientTypeRadial:
		radius1 := g.gradientType.radius1
		radius2 := g.gradientType.radius2
		if radius1 == 0.0 {
			p0, p1 = 1.0, 0.0
		} else {
			dRadius := radius2 - radius1
			// For concentric gradients: t = t * scale + bias
			p0 = maxF32(radius1, radius2) / dRadius
			p1 = -radius1 / dRadius
		}
	case gradientTypeStrip:
		scaledR0 := g.gradientType.scaledR0
		p0, p1 = scaledR0*scaledR0, 0.0 /*unused*/
	case gradientTypeFocal:
		fd := g.gradientType.focal
		p0, p1 = 1.0/fd.r1, fd.focalX
	}

	p.ctx.twoPointConicalGradient = twoPointConicalGradientCtx{
		mask: wide.U32x8{},
		p0:   p0,
		p1:   p1,
	}

	return g.base.pushStages(
		p,
		cs,
		func(p *rasterPipelineBuilder) {
			switch g.gradientType.kind {
			case gradientTypeRadial:
				p.push(stageXYToRadius)
				// Apply scale/bias to map t from [0, 1] based on r_max to proper t where
				// t=0 at r0 and t=1 at r1
				if p0 != 1.0 || p1 != 0.0 {
					p.push(stageApplyConcentricScaleBias)
				}
			case gradientTypeStrip:
				p.push(stageXYTo2PtConicalStrip)
				p.push(stageMask2PtConicalNan)
			case gradientTypeFocal:
				fd := g.gradientType.focal
				switch {
				case fd.isFocalOnCircle():
					p.push(stageXYTo2PtConicalFocalOnCircle)
				case fd.isWellBehaved():
					p.push(stageXYTo2PtConicalWellBehaved)
				case fd.isSwapped || (1.0-fd.focalX) < 0.0:
					p.push(stageXYTo2PtConicalSmaller)
				default:
					p.push(stageXYTo2PtConicalGreater)
				}

				if !fd.isWellBehaved() {
					p.push(stageMask2PtConicalDegenerates)
				}

				if (1.0 - fd.focalX) < 0.0 {
					p.push(stageNegateX)
				}

				if !fd.isNativelyFocal() {
					p.push(stageAlter2PtConicalCompensateFocal)
				}

				if fd.isSwapped {
					p.push(stageAlter2PtConicalUnswap)
				}
			}
		},
		func(p *rasterPipelineBuilder) {
			switch g.gradientType.kind {
			case gradientTypeStrip:
				p.push(stageApplyVectorMask)
			case gradientTypeFocal:
				if !g.gradientType.focal.isWellBehaved() {
					p.push(stageApplyVectorMask)
				}
			}
		},
	)
}

// createRadialGradient is create in tiny-skia
// src/shaders/radial_gradient.rs.
func createRadialGradient(
	c0 Point,
	r0 float32,
	c1 Point,
	r1 float32,
	stops []GradientStop,
	mode SpreadMode,
	transform Transform,
) (Shader, bool) {
	var gt gradientType
	var gradientMatrix Transform

	if path.IsNearlyZero(c0.Sub(c1).Length()) {
		if path.IsNearlyZero(maxF32(r0, r1)) || path.IsNearlyEqual(r0, r1) {
			// Degenerate case. Avoid dividing by zero. The caller catches it,
			// but just in case, recheck here.
			return nil, false
		}

		// Concentric case: we can pretend we are radial (with a tiny twist).
		scale := 1.0 / maxF32(r0, r1)
		gradientMatrix = path.TransformFromTranslate(-c1.X, -c1.Y).PostScale(scale, scale)
		gt = gradientType{
			kind:    gradientTypeRadial,
			radius1: r0,
			radius2: r1,
		}
	} else {
		var ok bool
		gradientMatrix, ok = mapToUnitX(c0, c1)
		if !ok {
			return nil, false
		}
		dCenter := c0.Sub(c1).Length()
		if path.IsNearlyZero(r0 - r1) {
			scaledR0 := r0 / dCenter
			gt = gradientType{kind: gradientTypeStrip, scaledR0: scaledR0}
		} else {
			gt = gradientType{kind: gradientTypeFocal}
		}
	}
	if gt.kind == gradientTypeFocal {
		dCenter := c0.Sub(c1).Length()
		if !gt.focal.set(r0/dCenter, r1/dCenter, &gradientMatrix) {
			return nil, false
		}
	}

	return RadialGradient{
		base:         newGradient(stops, mode, transform, gradientMatrix),
		gradientType: gt,
	}, true
}

// mapToUnitX is map_to_unit_x in tiny-skia src/shaders/radial_gradient.rs.
func mapToUnitX(origin, xIsOne Point) (Transform, bool) {
	return tsFromPolyToPoly(
		origin,
		xIsOne,
		path.PointFromXY(0.0, 0.0),
		path.PointFromXY(1.0, 0.0),
	)
}

// tsFromPolyToPoly is ts_from_poly_to_poly in tiny-skia
// src/shaders/radial_gradient.rs.
func tsFromPolyToPoly(src1, src2, dst1, dst2 Point) (Transform, bool) {
	tmp := fromPoly2(src1, src2)
	res, ok := tmp.Invert()
	if !ok {
		return Transform{}, false
	}
	tmp = fromPoly2(dst1, dst2)
	return tmp.PreConcat(res), true
}

// fromPoly2 is from_poly2 in tiny-skia src/shaders/radial_gradient.rs.
func fromPoly2(p0, p1 Point) Transform {
	return path.TransformFromRow(
		p1.Y-p0.Y,
		p0.X-p1.X,
		p1.X-p0.X,
		p1.Y-p0.Y,
		p0.X,
		p0.Y,
	)
}
