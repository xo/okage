// Copyright 2026 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/runtime/trim.rs and src/runtime/effects.rs.

import (
	"math"

	"github.com/xo/okage/internal/kurbo"
)

// trimAccuracy is ACCURACY in velato src/runtime/trim.rs.
const trimAccuracy = 0.1

// pathLength is length in velato src/runtime/trim.rs.
func pathLength(elements []kurbo.PathEl) float64 {
	total := 0.0
	for _, seg := range kurbo.Segments(elements) {
		total += seg.Arclen(trimAccuracy)
	}
	return total
}

// trimInterval is an interval of a contour, in units of length.
type trimInterval struct {
	start, end float64
}

// trimPath is trim in velato src/runtime/trim.rs. sequential is nil for a
// parallel trim, and else the offset of the path and the total length.
func trimPath(elements []kurbo.PathEl, ranges []trimRange, sequential *[2]float64, output []kurbo.PathEl) []kurbo.PathEl {
	offset := 0.0
	if sequential != nil {
		offset = sequential[0]
	}
	start := 0
	for end := 1; end <= len(elements); end++ {
		if end != len(elements) && elements[end].Kind != kurbo.PathElMoveTo && elements[end-1].Kind != kurbo.PathElClosePath {
			continue
		}
		contour := elements[start:end]
		contourLength := pathLength(contour)
		base, total := 0.0, contourLength
		if sequential != nil {
			base, total = offset, sequential[1]
		}
		var intervals []trimInterval
		for _, r := range ranges {
			iv := trimInterval{rustMax(r.Start*total-base, 0.0), rustMin(r.End*total-base, contourLength)}
			if iv.end > iv.start {
				intervals = append(intervals, iv)
			}
		}
		whole := false
		for _, iv := range intervals {
			if iv.start == 0.0 && iv.end == contourLength {
				whole = true
				break
			}
		}
		if whole {
			output = append(output, contour...)
		} else if len(intervals) > 0 {
			output = trimContour(contour, intervals, contourLength, output)
		}
		offset += contourLength
		start = end
	}
	return output
}

// trimContour is trim_contour in velato src/runtime/trim.rs.
func trimContour(elements []kurbo.PathEl, intervals []trimInterval, length float64, output []kurbo.PathEl) []kurbo.PathEl {
	type segLen struct {
		seg kurbo.PathSeg
		len float64
	}
	var segments []segLen
	for _, seg := range kurbo.Segments(elements) {
		segments = append(segments, segLen{seg, seg.Arclen(trimAccuracy)})
	}
	closed := len(elements) > 0 && elements[len(elements)-1].Kind == kurbo.PathElClosePath
	previousEnd, hasPrevious := 0.0, false
	for _, iv := range intervals {
		needMove := !(closed && hasPrevious && previousEnd == length && iv.start == 0.0)
		position := 0.0
		for _, s := range segments {
			if position >= iv.end {
				break
			}
			next := position + s.len
			if s.len > 0.0 && next > iv.start && position < iv.end {
				t0 := 0.0
				if iv.start > position {
					t0 = clampUpTo(s.seg.InvArclen(iv.start-position, trimAccuracy), 1.0)
				}
				t1 := 1.0
				if iv.end < next {
					t1 = clampUpTo(s.seg.InvArclen(iv.end-position, trimAccuracy), 1.0)
				}
				if t1 > t0 {
					sub := s.seg.Subsegment(t0, t1)
					if needMove {
						output = append(output, kurbo.MoveTo(sub.Start()))
						needMove = false
					}
					output = append(output, sub.AsPathEl())
				}
			}
			position = next
		}
		previousEnd, hasPrevious = iv.end, true
	}
	return output
}

// FilterLayerResult is FilterLayerResult in velato src/runtime/effects.rs.
type FilterLayerResult uint8

// The values of FilterLayerResult.
const (
	FilterPushed FilterLayerResult = iota
	FilterUnsupported
)

// BlurEdgeMode is BlurEdgeMode in velato src/runtime/effects.rs: how
// Gaussian blur samples outside the source bounds.
type BlurEdgeMode uint8

// The values of BlurEdgeMode.
const (
	// BlurDuplicate extends the edge samples beyond the source bounds.
	BlurDuplicate BlurEdgeMode = iota
	// BlurWrap tiles the source outside its bounds.
	BlurWrap
)

// FilterEffect is FilterEffect in velato src/runtime/effects.rs: a layer
// effect evaluated for one frame (D27). Each kind uses the fields of its
// Rust variant.
type FilterEffect struct {
	Kind LayerEffectKind
	// GaussianBlur.
	StdDeviation kurbo.Vec2
	EdgeMode     BlurEdgeMode
	// DropShadow, and Color for Fill.
	Color              Color
	Offset             kurbo.Vec2
	ShadowStdDeviation float64
	// Tint.
	Black, White Color
	Amount       float64
}

// Evaluate is LayerEffect::evaluate in velato src/runtime/effects.rs. It
// returns false where the effect does nothing at frame.
func (e *LayerEffect) Evaluate(frame float64) (FilterEffect, bool) {
	nonnegative := func(v float64) float64 {
		if isFiniteF64(v) {
			return math.Max(v, 0.0)
		}
		return 0.0
	}
	switch e.Kind {
	case EffectGaussianBlur:
		// Match ThorVG's Lottie blurriness-to-sigma conversion.
		sigma := nonnegative(e.Blurriness.Evaluate(frame)) * 0.3
		if sigma == 0.0 {
			return FilterEffect{}, false
		}
		dimensions := castU32(e.Dimensions.Evaluate(frame))
		sx, sy := sigma, sigma
		if dimensions == 3 {
			sx = 0.0
		}
		if dimensions == 2 {
			sy = 0.0
		}
		edge := BlurDuplicate
		if e.Wrap.Evaluate(frame) == 1.0 {
			edge = BlurWrap
		}
		return FilterEffect{Kind: EffectGaussianBlur, StdDeviation: kurbo.NewVec2(sx, sy), EdgeMode: edge}, true
	case EffectDropShadow:
		opacity := math.Min(nonnegative(e.Opacity.Evaluate(frame))/255.0, 1.0)
		if opacity == 0.0 {
			return FilterEffect{}, false
		}
		angle := toRadians(e.Angle.Evaluate(frame))
		distance := e.Distance.Evaluate(frame)
		if !isFiniteF64(angle) || !isFiniteF64(distance) {
			return FilterEffect{}, false
		}
		return FilterEffect{
			Kind:               EffectDropShadow,
			Color:              e.Color.EvaluateOr(frame, ColorTransparent).WithAlpha(float32(opacity)),
			Offset:             kurbo.NewVec2(kurbo.Sin(angle)*distance, -kurbo.Cos(angle)*distance),
			ShadowStdDeviation: nonnegative(e.Softness.Evaluate(frame)) * 0.3,
		}, true
	case EffectFill:
		return FilterEffect{
			Kind:  EffectFill,
			Color: e.Color.EvaluateOr(frame, ColorTransparent).WithAlpha(float32(math.Min(nonnegative(e.Opacity.Evaluate(frame)), 1.0))),
		}, true
	}
	amount := math.Min(nonnegative(e.Amount.Evaluate(frame))/100.0, 1.0)
	if amount == 0.0 {
		return FilterEffect{}, false
	}
	return FilterEffect{
		Kind:   EffectTint,
		Black:  e.Black.EvaluateOr(frame, ColorBlack).WithAlpha(1.0),
		White:  e.White.EvaluateOr(frame, ColorWhite).WithAlpha(1.0),
		Amount: amount,
	}, true
}

// castU32 is the cast "as u32" of an f64.
func castU32(x float64) uint32 {
	switch {
	case !(x > 0):
		return 0
	case x >= math.MaxUint32:
		return math.MaxUint32
	}
	return uint32(x)
}
