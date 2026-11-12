// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/model/value.rs.

import (
	"math"
	"sort"

	"github.com/xo/okage/internal/kurbo"
)

// Value is Value in velato src/runtime/model/value.rs: a fixed or animated
// value (D27).
type Value[T any] struct {
	Fixed    T
	Animated *Animated[T]
}

// fixedValue returns a fixed value.
func fixedValue[T any](v T) Value[T] {
	return Value[T]{Fixed: v}
}

// Evaluate is Value::evaluate in velato src/runtime/model/value.rs: the
// value at frame, or the zero value of T where an animated value has no
// keyframe.
func (v Value[T]) Evaluate(frame float64) T {
	var zero T
	return v.EvaluateOr(frame, zero)
}

// IsFixed is Value::is_fixed in velato src/runtime/model/value.rs.
func (v Value[T]) IsFixed() bool {
	return v.Animated == nil
}

// EvaluateOr is Value::evaluate_or in velato src/runtime/model/value.rs.
func (v Value[T]) EvaluateOr(frame float64, def T) T {
	if v.Animated == nil {
		return v.Fixed
	}
	return v.Animated.EvaluateOr(frame, def)
}

// Easing is Easing in velato src/runtime/model/value.rs.
type Easing struct {
	O EasingHandle
	I EasingHandle
}

// easingLerp is Easing::LERP in velato src/runtime/model/value.rs.
var easingLerp = Easing{
	O: EasingHandle{X: 0.0, Y: 0.0},
	I: EasingHandle{X: 1.0, Y: 1.0},
}

// EasingHandle is EasingHandle in velato src/runtime/model/value.rs.
type EasingHandle struct {
	X, Y float64
}

// Time is Time in velato src/runtime/model/value.rs: the time of a keyframe,
// as a frame number.
type Time struct {
	// Frame number.
	Frame float64
	// Easing tangent going into the next keyframe.
	InTangent *EasingHandle
	// Easing tangent leaving the current keyframe.
	OutTangent *EasingHandle
	// Whether it's a hold frame.
	Hold bool
}

// framesAndWeight is Time::frames_and_weight in velato
// src/runtime/model/value.rs. It returns the indices of the two keyframes
// around frame, the weight between them, the easing and the hold flag, and
// false when times is empty.
func framesAndWeight(times []Time, frame float64) ([2]int, float64, Easing, bool, bool) {
	if len(times) == 0 {
		return [2]int{}, 0, Easing{}, false, false
	}
	ix := binarySearchTimes(times, frame)
	ix0 := min(ix, len(times)-1)
	ix1 := min(ix0+1, len(times)-1)

	t0 := times[ix0]
	t1 := times[ix1]
	t0ox, t0oy := 0.0, 0.0
	if t0.OutTangent != nil {
		t0ox, t0oy = t0.OutTangent.X, t0.OutTangent.Y
	}
	t0ix, t0iy := 1.0, 1.0
	if t0.InTangent != nil {
		t0ix, t0iy = t0.InTangent.X, t0.InTangent.Y
	}
	easing := Easing{
		O: EasingHandle{X: t0ox, Y: t0oy},
		I: EasingHandle{X: t0ix, Y: t0iy},
	}
	hold := t0.Hold
	t := 0.0
	if ix0 != ix1 {
		t = (frame - t0.Frame) / (t1.Frame - t0.Frame)
	}
	return [2]int{ix0, ix1}, clampUpTo(t, 1.0), easing, hold, true
}

// binarySearchTimes is the binary_search_by of Time::frames_and_weight: the
// index of a keyframe at frame, or the index before the place where frame
// goes, saturated at 0.
//
// Port: Rust's binary_search_by can return any of several equal elements.
// For the keyframes of a file, which have distinct frames, it returns the
// one match. The Go search returns the first match.
func binarySearchTimes(times []Time, frame float64) int {
	i := sort.Search(len(times), func(i int) bool { return !(times[i].Frame < frame) })
	if i < len(times) && !(times[i].Frame > frame) {
		return i
	}
	if i == 0 {
		return 0
	}
	return i - 1
}

// clampUpTo is f64::clamp from 0 to hi. It returns NaN for a NaN x.
func clampUpTo(x, hi float64) float64 {
	if x < 0 {
		x = 0
	}
	if x > hi {
		x = hi
	}
	return x
}

// Animated is Animated in velato src/runtime/model/value.rs: the keyframes
// of a value and the function that tweens between two values.
type Animated[T any] struct {
	Times  []Time
	Values []T
	tween  func(a, b T, t float64, easing Easing) T
}

// newAnimated returns an animated value that tweens with tween.
func newAnimated[T any](times []Time, values []T, tween func(a, b T, t float64, easing Easing) T) Value[T] {
	return Value[T]{Animated: &Animated[T]{Times: times, Values: values, tween: tween}}
}

// EvaluateOr is Animated::evaluate_or in velato
// src/runtime/model/value.rs.
func (a *Animated[T]) EvaluateOr(frame float64, def T) T {
	if v, ok := a.evaluateInner(frame); ok {
		return v
	}
	return def
}

// evaluateInner is Animated::evaluate_inner in velato
// src/runtime/model/value.rs.
func (a *Animated[T]) evaluateInner(frame float64) (T, bool) {
	var zero T
	ix, t, easing, hold, ok := framesAndWeight(a.Times, frame)
	if !ok {
		return zero, false
	}
	if hold {
		t = 0.0
	}
	if ix[0] >= len(a.Values) || ix[1] >= len(a.Values) {
		return zero, false
	}
	return a.tween(a.Values[ix[0]], a.Values[ix[1]], t, easing), true
}

// tweenF64 is impl Tween for f64 in velato src/runtime/model/value.rs: the
// value between a and b at the normalized time t, through the cubic Bezier
// curve of easing.
func tweenF64(a, b, t float64, easing Easing) float64 {
	// avoid unnecessary calculations
	if t <= 0.0 {
		return a
	}
	if t >= 1.0 {
		return b
	}

	curve := kurbo.NewCubicBez(
		kurbo.NewPoint(0.0, 0.0),
		kurbo.NewPoint(easing.O.X, easing.O.Y),
		kurbo.NewPoint(easing.I.X, easing.I.Y),
		kurbo.NewPoint(1.0, 1.0),
	)

	// Manual copy-paste of kurbo::bezpath::cubic_bez_coefs in preparation
	// for kurbo::common::solve_cubic.
	x0, x1, x2, x3 := curve.P0.X, curve.P1.X, curve.P2.X, curve.P3.X
	c0 := x0
	c1 := 3.0 * (x1 - x0)
	c2 := 3.0 * (x2 - 2.0*x1 + x0)
	c3 := x3 - 3.0*x2 + 3.0*x1 - x0

	// Find where along the curve we are at normalized time t.
	maxCoeff := math.Max(math.Max(math.Abs(c0), math.Abs(c1)), math.Max(math.Abs(c2), math.Abs(c3)))
	eps := 1e-12 * math.Max(maxCoeff, 1.0)

	u := 0.0
	found := false
	for _, r := range kurbo.SolveCubic(c0-t, c1, c2, c3) {
		if r >= -eps && r <= 1.0+eps {
			u, found = r, true
			break
		}
	}
	if !found {
		// Fallback when no valid root found due to precision issues.
		valAt0 := math.Abs(c0 - t)
		valAt1 := math.Abs(c0 + c1 + c2 + c3 - t)
		if valAt0 > valAt1 {
			u = 1.0
		}
	}

	// Evaluate the curve at that point to get the y value.
	easedY := curve.Eval(u).Y
	return a + (b-a)*easedY
}

// tweenPoint is impl Tween for kurbo::Point in velato
// src/runtime/model/value.rs.
func tweenPoint(a, b kurbo.Point, t float64, easing Easing) kurbo.Point {
	return kurbo.NewPoint(tweenF64(a.X, b.X, t, easing), tweenF64(a.Y, b.Y, t, easing))
}

// tweenVec2 is impl Tween for kurbo::Vec2 in velato
// src/runtime/model/value.rs.
func tweenVec2(a, b kurbo.Vec2, t float64, easing Easing) kurbo.Vec2 {
	return kurbo.NewVec2(tweenF64(a.X, b.X, t, easing), tweenF64(a.Y, b.Y, t, easing))
}

// tweenSize is impl Tween for kurbo::Size in velato
// src/runtime/model/value.rs.
func tweenSize(a, b kurbo.Size, t float64, easing Easing) kurbo.Size {
	return kurbo.NewSize(tweenF64(a.Width, b.Width, t, easing), tweenF64(a.Height, b.Height, t, easing))
}

// tweenColor is impl Tween for peniko::Color in velato
// src/runtime/model/value.rs.
func tweenColor(a, b Color, t float64, easing Easing) Color {
	var c Color
	for i := range c {
		c[i] = float32(tweenF64(float64(a[i]), float64(b[i]), t, easing))
	}
	return c
}
