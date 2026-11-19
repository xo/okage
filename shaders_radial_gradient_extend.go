package okage

import "github.com/xo/okage/path"

// This file holds the radial gradient of the shadings of type 3 of PDF and
// PostScript (D64). tiny-skia does not have it, and it ports no upstream
// file.

// NewRadialGradientExtend makes a radial gradient that follows the rule of
// the radial shadings of PDF and PostScript, the shadings of type 3 (D64).
// tiny-skia does not have it.
//
// The gradient has a circle for each t. The center of the circle goes from
// startPoint at t = 0 to endPoint at t = 1, and its radius goes from
// startRadius to endRadius. A point takes the color of the largest t whose
// circle passes through the point and has a radius of at least 0. The t must
// be in 0..=1, with two exceptions:
//   - If extendStart is true, t can be less than 0. The point then takes the
//     color at t = 0.
//   - If extendEnd is true, t can be more than 1. The point then takes the
//     color at t = 1.
//
// A point without such a t is transparent black. extendStart and extendEnd
// are the two values of the key Extend of a shading of PDF, and of the
// shadings of PostScript.
//
// A gradient of NewRadialGradient takes the largest t of all, and its spread
// mode then gives the color of that t. The two rules give the same colors
// when the circles nest and both extendStart and extendEnd are true. They
// can differ in the other cases. For example, a point whose largest t is
// more than 1 takes the color of a smaller t in 0..=1 here, when one exists.
//
// In one case, the radius changes almost as fast as the center moves. The
// start circle then almost touches the end circle on the inside. The
// gradient then has one circle at most through each point. tiny-skia does
// the same for a focal point on the end circle.
//
// The gradient is a RadialGradient. Its colors come from the highp pipeline,
// and it is never opaque. A gradient with one stop has the color of that stop
// where a t exists.
//
// It returns false when:
//   - stops is empty
//   - startRadius < 0 or endRadius < 0, or a point or a radius is not finite
//   - transform is not invertible
//   - the centers are nearly equal and the radii are nearly equal, so that
//     each circle is the same circle
func NewRadialGradientExtend(
	startPoint Point,
	startRadius float32,
	endPoint Point,
	endRadius float32,
	stops []GradientStop,
	extendStart, extendEnd bool,
	transform Transform,
) (Shader, bool) {
	// The comparisons are false for NaN.
	if !(startRadius >= 0) || !(endRadius >= 0) || !isFiniteF32(startRadius) || !isFiniteF32(endRadius) {
		return nil, false
	}
	if !startPoint.IsFinite() || !endPoint.IsFinite() || len(stops) == 0 {
		return nil, false
	}
	if _, ok := transform.Invert(); !ok {
		return nil, false
	}

	length := startPoint.Sub(endPoint).Length()
	if !isFiniteF32(length) {
		return nil, false
	}
	if path.IsNearlyZeroWithinTolerance(length, degenerateThreshold) &&
		path.IsNearlyEqualWithinTolerance(startRadius, endRadius, degenerateThreshold) {
		return nil, false
	}

	// The space of the gradient has the start point at the origin, and the
	// largest of the distance and the radii is 1. The values of the stage
	// thus stay near 1.
	scale := 1 / max(length, startRadius, endRadius)
	pointsToUnit := path.TransformFromTranslate(-startPoint.X, -startPoint.Y).PostScale(scale, scale)

	cx := (endPoint.X - startPoint.X) * scale
	cy := (endPoint.Y - startPoint.Y) * scale
	r0 := startRadius * scale
	dr := float32(endRadius*scale) - r0
	a := float32(cx*cx) + float32(cy*cy) - float32(dr*dr)
	if path.IsNearlyZero(a) {
		a = 0
	}

	return RadialGradient{
		base: newGradient(stops, SpreadModePad, transform, pointsToUnit),
		gradientType: gradientType{
			kind: gradientTypeExtend,
			extend: radialExtendCtx{
				cx:          cx,
				cy:          cy,
				r0:          r0,
				dr:          dr,
				r0dr:        float32(r0 * dr),
				r0r0:        float32(r0 * r0),
				a:           a,
				extendStart: extendStart,
				extendEnd:   extendEnd,
			},
			circles: radialCircles{start: startPoint, startRadius: startRadius, end: endPoint, endRadius: endRadius},
		},
	}, true
}

// radialExtendCtx is the context of stageXYTo2PtConicalExtend (D64). In the
// space of the gradient, the start circle has its center at the origin and
// the radius r0. The circle of t has its center at t * (cx, cy) and the
// radius r0 + t * dr. A point (x, y) is on that circle when
// a * t * t - 2 * b * t + c = 0, with:
//
//	a = cx * cx + cy * cy - dr * dr
//	b = x * cx + y * cy + r0 * dr
//	c = x * x + y * y - r0 * r0
type radialExtendCtx struct {
	cx, cy      float32
	r0, dr      float32
	r0dr, r0r0  float32
	a           float32
	extendStart bool
	extendEnd   bool
}

// t returns the t of the point (x, y) in the space of the gradient, and
// false when the point has none.
//
// The roots of the equation are q / a and c / q, with
// q = b + sign(b) * sqrt(b * b - a * c). This form loses no precision when
// a is near 0. When a is 0, q / a is not finite, and c / q is the one root.
func (c *radialExtendCtx) t(x, y float32) (float32, bool) {
	b := float32(x*c.cx) + float32(y*c.cy) + c.r0dr
	cc := float32(x*x) + float32(y*y) - c.r0r0
	disc := float32(b*b) - float32(c.a*cc)
	// The comparison is false for NaN.
	if !(disc >= 0) {
		return 0, false
	}
	sq := sqrtF32(disc)
	q := b + sq
	if b < 0 {
		q = b - sq
	}
	hi, lo := q/c.a, cc/q
	if hi < lo {
		hi, lo = lo, hi
	}
	switch {
	case c.accepts(hi):
		return hi, true
	case c.accepts(lo):
		return lo, true
	}
	return 0, false
}

// accepts reports whether t is finite, is in the range of t that the extend
// flags allow, and gives a circle with a radius of at least 0.
func (c *radialExtendCtx) accepts(t float32) bool {
	// t - t is NaN for an infinite t and for NaN.
	return t-t == 0 &&
		(c.extendStart || t >= 0) &&
		(c.extendEnd || t <= 1) &&
		float32(t*c.dr)+c.r0 >= 0
}
