// Copyright 2026 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/model/position.rs.

import (
	"math"
	"sort"

	"github.com/xo/okage/internal/kurbo"
	"github.com/xo/okage/internal/lottiecompat"
)

// SpatialKeyframe is SpatialKeyframe in velato
// src/runtime/model/position.rs.
type SpatialKeyframe struct {
	Time  Time
	Value kurbo.Point
	// Offset from this keyframe's position, or nil.
	OutTangent *kurbo.Vec2
	// Offset from the next position, stored on this keyframe, or nil.
	InTangent *kurbo.Vec2
}

// motionPath is MotionPath in velato src/runtime/model/position.rs.
type motionPath struct {
	curve     kurbo.CubicBez
	arcLength float64
	accuracy  float64
}

// SpatialPosition is SpatialPosition in velato
// src/runtime/model/position.rs: a position that moves along the curves of
// its spatial tangents.
type SpatialPosition struct {
	keyframes []SpatialKeyframe
	paths     []*motionPath
}

// The accuracies of SpatialPosition.
const (
	spatialAccuracy         = 0.01
	spatialRelativeAccuracy = 1e-5
)

// isLinearCurve is SpatialPosition::is_linear in velato
// src/runtime/model/position.rs.
func isLinearCurve(curve kurbo.CubicBez) bool {
	chord := curve.P3.Sub(curve.P0)
	out := curve.P1.Sub(curve.P0)
	incoming := curve.P3.Sub(curve.P2)
	zero := kurbo.Vec2{}
	if chord == zero {
		return out == zero && incoming == zero
	}
	if !curve.IsFinite() || !chord.IsFinite() || chord.Cross(out) != 0.0 || chord.Cross(incoming) != 0.0 {
		return false
	}
	derivative := curve.Deriv()
	if !derivative.IsFinite() {
		return false
	}
	bounds := derivative.BoundingBox()
	direction, lo, hi := chord.Y, bounds.Y0, bounds.Y1
	if math.Abs(chord.X) >= math.Abs(chord.Y) {
		direction, lo, hi = chord.X, bounds.X0, bounds.X1
	}
	if direction > 0.0 {
		return lo >= 0.0
	}
	return hi <= 0.0
}

// NewSpatialPosition is SpatialPosition::new in velato
// src/runtime/model/position.rs.
func NewSpatialPosition(keyframes []SpatialKeyframe) *SpatialPosition {
	var paths []*motionPath
	for i := 0; i+1 < len(keyframes); i++ {
		start, end := keyframes[i], keyframes[i+1]
		outTangent, inTangent := kurbo.Vec2{}, kurbo.Vec2{}
		if start.OutTangent != nil {
			outTangent = *start.OutTangent
		}
		if start.InTangent != nil {
			inTangent = *start.InTangent
		}
		if start.Time.Hold {
			paths = append(paths, nil)
			continue
		}
		curve := kurbo.NewCubicBez(start.Value, start.Value.Add(outTangent), end.Value.Add(inTangent), end.Value)
		if isLinearCurve(curve) {
			paths = append(paths, nil)
			continue
		}
		controlLength := curve.P1.Sub(curve.P0).Hypot() + curve.P2.Sub(curve.P1).Hypot() + curve.P3.Sub(curve.P2).Hypot()
		accuracy := math.Max(math.Min(spatialAccuracy, controlLength*spatialRelativeAccuracy), 2.2250738585072014e-308)
		arcLength := math.NaN()
		if curve.IsFinite() {
			arcLength = curve.Arclen(accuracy)
		}
		paths = append(paths, &motionPath{curve: curve, arcLength: arcLength, accuracy: accuracy})
	}
	return &SpatialPosition{keyframes: keyframes, paths: paths}
}

// Evaluate is SpatialPosition::evaluate in velato
// src/runtime/model/position.rs.
func (s *SpatialPosition) Evaluate(frame float64) kurbo.Point {
	index := sort.Search(len(s.keyframes), func(i int) bool { return !(s.keyframes[i].Time.Frame <= frame) })
	if index > 0 {
		index--
	}
	if index >= len(s.keyframes) {
		return kurbo.Point{}
	}
	start := s.keyframes[index]
	if index+1 >= len(s.keyframes) {
		return start.Value
	}
	end := s.keyframes[index+1]
	if frame <= start.Time.Frame || start.Time.Hold || end.Time.Frame <= start.Time.Frame {
		return start.Value
	}
	t := clampUpTo((frame-start.Time.Frame)/(end.Time.Frame-start.Time.Frame), 1.0)
	easing := easingLerp
	if start.Time.OutTangent != nil {
		easing.O = *start.Time.OutTangent
	}
	if start.Time.InTangent != nil {
		easing.I = *start.Time.InTangent
	}
	progress := tweenF64(0.0, 1.0, t, easing)
	path := s.paths[index]
	if path == nil {
		return start.Value.Lerp(end.Value, progress)
	}
	if !lottiecompat.Velato {
		// Port: velato finds the point at the exact arc length. okage finds
		// it with the approximate arc length of ThorVG (D43).
		return thorvgSpatial(path.curve, progress)
	}
	if math.IsInf(path.arcLength, 0) || path.arcLength != path.arcLength || math.IsInf(progress, 0) || progress != progress {
		return kurbo.NewPoint(math.NaN(), math.NaN())
	}
	if path.arcLength == 0.0 || progress <= 0.0 {
		return start.Value
	}
	if progress >= 1.0 {
		return end.Value
	}
	parameter := path.curve.InvArclen(progress*path.arcLength, path.accuracy)
	return path.curve.Eval(parameter)
}

// bezierEpsilon is BEZIER_EPSILON of ThorVG.
const bezierEpsilon = 1e-2

// lineLengthApprox is _lineLengthApprox of ThorVG: the distance of two
// points by the alpha max plus beta min algorithm, with beta 3/8.
func lineLengthApprox(a, b kurbo.Point) float64 {
	dx, dy := math.Abs(b.X-a.X), math.Abs(b.Y-a.Y)
	if dx > dy {
		return dx + dy*0.375
	}
	return dy + dx*0.375
}

// bezLengthApprox is _bezLength of ThorVG with lineLengthApprox: the length
// of the polygon of c, split in half until it is within bezierEpsilon of the
// chord.
func bezLengthApprox(c kurbo.CubicBez, depth int) float64 {
	length := lineLengthApprox(c.P0, c.P1) + lineLengthApprox(c.P1, c.P2) + lineLengthApprox(c.P2, c.P3)
	chord := lineLengthApprox(c.P0, c.P3)
	if math.Abs(length-chord) > bezierEpsilon && depth < 32 {
		left, right := c.Subdivide()
		return bezLengthApprox(left, depth+1) + bezLengthApprox(right, depth+1)
	}
	return length
}

// thorvgSpatial is the interpolation of a spatial position of ThorVG: the
// point at the approximate arc length progress times the length of curve,
// with the parameter that _bezAt finds by bisection.
func thorvgSpatial(curve kurbo.CubicBez, progress float64) kurbo.Point {
	length := bezLengthApprox(curve, 0)
	at := progress * length
	if at <= 0 {
		return curve.Eval(0)
	}
	if at >= length {
		return curve.Eval(1)
	}
	smallest, biggest, t := 0.0, 1.0, 0.5
	for range 64 {
		left := curve.Subsegment(0, t)
		l := bezLengthApprox(left, 0)
		if math.Abs(l-at) < bezierEpsilon || math.Abs(smallest-biggest) < 1e-3 {
			break
		}
		if l < at {
			smallest = t
			t = (t + biggest) * 0.5
		} else {
			biggest = t
			t = (smallest + t) * 0.5
		}
	}
	return curve.Eval(t)
}
