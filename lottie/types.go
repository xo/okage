// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import "github.com/xo/okage/internal/kurbo"

// The geometry types of kurbo that the API of velato uses (D41). They are
// aliases, so a program outside okage can name them, for example to
// implement a Sink.
type (
	// Affine is kurbo::Affine.
	Affine = kurbo.Affine
	// PathEl is kurbo::PathEl.
	PathEl = kurbo.PathEl
	// PathElKind is the variant of kurbo::PathEl.
	PathElKind = kurbo.PathElKind
	// Point is kurbo::Point.
	Point = kurbo.Point
	// Vec2 is kurbo::Vec2.
	Vec2 = kurbo.Vec2
	// Size is kurbo::Size.
	Size = kurbo.Size
	// Rect is kurbo::Rect.
	Rect = kurbo.Rect
)

// The variants of PathEl.
const (
	PathElMoveTo    = kurbo.PathElMoveTo
	PathElLineTo    = kurbo.PathElLineTo
	PathElQuadTo    = kurbo.PathElQuadTo
	PathElCurveTo   = kurbo.PathElCurveTo
	PathElClosePath = kurbo.PathElClosePath
)

// IdentityAffine is Affine::IDENTITY.
var IdentityAffine = kurbo.IdentityAffine

// AffineScale is Affine::scale.
func AffineScale(s float64) Affine {
	return kurbo.AffineScale(s)
}
