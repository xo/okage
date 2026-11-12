// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/schema/helpers and src/schema/constants.

import (
	"encoding/json"
	"fmt"
)

// Bezier is Bezier in velato src/schema/helpers/bezier.rs.
type Bezier struct {
	Closed      *bool        `json:"c,omitempty"`
	Vertices    [][2]float64 `json:"v"`
	InTangents  [][2]float64 `json:"i"`
	OutTangents [][2]float64 `json:"o"`
}

// UnmarshalJSON requires the keys "v", "i" and "o".
func (b *Bezier) UnmarshalJSON(data []byte) error {
	type plain Bezier
	if err := requireKeys(data, "v", "i", "o"); err != nil {
		return err
	}
	return json.Unmarshal(data, (*plain)(b))
}

// BoolInt is BoolInt in velato src/schema/helpers/int_boolean.rs: 0 is
// false and 1 is true.
type BoolInt uint8

// The values of BoolInt.
const (
	BoolIntFalse BoolInt = 0
	BoolIntTrue  BoolInt = 1
)

// UnmarshalJSON reads 0 or 1.
func (b *BoolInt) UnmarshalJSON(data []byte) error {
	var v uint8
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v > 1 {
		return fmt.Errorf("invalid value: %d, expected one of: 0, 1", v)
	}
	*b = BoolInt(v)
	return nil
}

// isTrue reports if b is set to BoolIntTrue.
func (b *BoolInt) isTrue() bool {
	return b != nil && *b == BoolIntTrue
}

// Marker is Marker in velato src/schema/helpers/marker.rs.
type Marker struct {
	Comment  *string  `json:"cm,omitempty"`
	Time     *float64 `json:"tm,omitempty"`
	Duration *float64 `json:"dr,omitempty"`
}

// MaskSchema is Mask in velato src/schema/helpers/mask.rs.
type MaskSchema struct {
	Name      *string        `json:"nm,omitempty"`
	MatchName *string        `json:"mn,omitempty"`
	Inverted  *bool          `json:"inv,omitempty"`
	Shape     *ShapeProperty `json:"pt,omitempty"`
	Opacity   *FloatValue    `json:"o,omitempty"`
	Mode      *MaskMode      `json:"mode,omitempty"`
	Expand    *FloatValue    `json:"x,omitempty"`
}

// TransformSchema is Transform in velato src/schema/helpers/transform.rs.
//
// Port: Position is a pointer, because a struct that embeds TransformSchema cannot
// require the key "p" with a method of its own. The conversion reports a
// missing position as serde does.
type TransformSchema struct {
	AnchorPoint *PositionSchema   `json:"a,omitempty"`
	Position    *AnyTransformP    `json:"p,omitempty"`
	Scale       *MultiDimensional `json:"s,omitempty"`
	Rotation    *AnyTransformR    `json:"r,omitempty"`
	Skew        *FloatValue       `json:"sk,omitempty"`
	SkewAxis    *FloatValue       `json:"sa,omitempty"`
	Opacity     *FloatValue       `json:"o,omitempty"`
}

// AnyTransformP is AnyTransformP in velato src/schema/helpers/transform.rs:
// a position, or a split position (D27).
type AnyTransformP struct {
	Position      PositionSchema
	SplitPosition SplitVector
	IsSplit       bool
}

// UnmarshalJSON tries a position first, then a split position.
func (p *AnyTransformP) UnmarshalJSON(data []byte) error {
	var pos PositionSchema
	if err := json.Unmarshal(data, &pos); err == nil {
		*p = AnyTransformP{Position: pos}
		return nil
	}
	var split SplitVector
	if err := json.Unmarshal(data, &split); err != nil {
		return fmt.Errorf("data did not match any variant of untagged enum AnyTransformP: %w", err)
	}
	*p = AnyTransformP{SplitPosition: split, IsSplit: true}
	return nil
}

// AnyTransformR is AnyTransformR in velato src/schema/helpers/transform.rs:
// a rotation, or a split rotation (D27).
type AnyTransformR struct {
	Rotation    FloatValue
	XRotation   FloatValue
	YRotation   FloatValue
	ZRotation   FloatValue
	Orientation MultiDimensional
	IsSplit     bool
}

// UnmarshalJSON tries a rotation first, then a split rotation.
func (r *AnyTransformR) UnmarshalJSON(data []byte) error {
	var rot FloatValue
	if err := json.Unmarshal(data, &rot); err == nil {
		*r = AnyTransformR{Rotation: rot}
		return nil
	}
	var v struct {
		X  FloatValue       `json:"x"`
		Y  FloatValue       `json:"y"`
		Z  FloatValue       `json:"z"`
		Or MultiDimensional `json:"or"`
	}
	err := requireKeys(data, "x", "y", "z", "or")
	if err == nil {
		err = json.Unmarshal(data, &v)
	}
	if err != nil {
		return fmt.Errorf("data did not match any variant of untagged enum AnyTransformR: %w", err)
	}
	*r = AnyTransformR{XRotation: v.X, YRotation: v.Y, ZRotation: v.Z, Orientation: v.Or, IsSplit: true}
	return nil
}

// VisualObject is VisualObject in velato
// src/schema/helpers/visual_object.rs.
type VisualObject struct {
	Name      *string `json:"nm,omitempty"`
	MatchName *string `json:"mn,omitempty"`
}

// requireKeys returns the error of serde for the first key of keys that the
// JSON object data does not hold.
func requireKeys(data []byte, keys ...string) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	for _, k := range keys {
		if _, ok := m[k]; !ok {
			return errMissingField(k)
		}
	}
	return nil
}

// BlendModeSchema is BlendMode in velato src/schema/constants/blend_mode.rs.
type BlendModeSchema uint8

// The values of BlendMode.
const (
	BlendModeNormal BlendModeSchema = iota
	BlendModeMultiply
	BlendModeScreen
	BlendModeOverlay
	BlendModeDarken
	BlendModeLighten
	BlendModeColorDodge
	BlendModeColorBurn
	BlendModeHardLight
	BlendModeSoftLight
	BlendModeDifference
	BlendModeExclusion
	BlendModeHue
	BlendModeSaturation
	BlendModeColor
	BlendModeLuminosity
	BlendModeAdd
	BlendModeHardMix
)

// UnmarshalJSON reads a value from 0 to 17.
func (m *BlendModeSchema) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(m), 0, 17)
}

// Composite is Composite in velato src/schema/constants/composite.rs.
type Composite uint8

// The values of Composite.
const (
	CompositeBelow Composite = 1
	CompositeAbove Composite = 2
)

// UnmarshalJSON reads 1 or 2.
func (c *Composite) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(c), 1, 2)
}

// FillRule is FillRule in velato src/schema/constants/fill_rule.rs.
type FillRule uint8

// The values of FillRule.
const (
	FillRuleNonZero FillRule = 1
	FillRuleEvenOdd FillRule = 2
)

// UnmarshalJSON reads 1 or 2.
func (r *FillRule) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(r), 1, 2)
}

// GradientType is GradientType in velato
// src/schema/constants/gradient_type.rs.
type GradientType uint8

// The values of GradientType.
const (
	GradientTypeLinear GradientType = 1
	GradientTypeRadial GradientType = 2
)

// UnmarshalJSON reads 1 or 2.
func (g *GradientType) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(g), 1, 2)
}

// LineCap is LineCap in velato src/schema/constants/line_cap.rs.
type LineCap uint8

// The values of LineCap.
const (
	LineCapButt   LineCap = 1
	LineCapRound  LineCap = 2
	LineCapSquare LineCap = 3
)

// UnmarshalJSON reads a value from 1 to 3.
func (c *LineCap) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(c), 1, 3)
}

// LineJoin is LineJoin in velato src/schema/constants/line_join.rs.
type LineJoin uint8

// The values of LineJoin.
const (
	LineJoinMiter LineJoin = 1
	LineJoinRound LineJoin = 2
	LineJoinBevel LineJoin = 3
)

// UnmarshalJSON reads a value from 1 to 3.
func (j *LineJoin) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(j), 1, 3)
}

// MaskMode is MaskMode in velato src/schema/constants/mask_mode.rs.
type MaskMode string

// The values of MaskMode.
const (
	MaskModeNone       MaskMode = "n"
	MaskModeAdd        MaskMode = "a"
	MaskModeSubtract   MaskMode = "s"
	MaskModeIntersect  MaskMode = "i"
	MaskModeLighten    MaskMode = "l"
	MaskModeDarken     MaskMode = "d"
	MaskModeDifference MaskMode = "f"
)

// UnmarshalJSON reads one of the letters of the modes.
func (m *MaskMode) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch MaskMode(s) {
	case MaskModeNone, MaskModeAdd, MaskModeSubtract, MaskModeIntersect, MaskModeLighten, MaskModeDarken, MaskModeDifference:
		*m = MaskMode(s)
		return nil
	}
	return fmt.Errorf("unknown variant `%s`", s)
}

// MatteMode is MatteMode in velato src/schema/constants/matte_mode.rs.
type MatteMode uint8

// The values of MatteMode.
const (
	MatteModeNormal MatteMode = iota
	MatteModeAlpha
	MatteModeInvertedAlpha
	MatteModeLuma
	MatteModeInvertedLuma
)

// UnmarshalJSON reads a value from 0 to 4.
func (m *MatteMode) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(m), 0, 4)
}

// MergeMode is MergeMode in velato src/schema/constants/merge_mode.rs.
type MergeMode uint8

// The values of MergeMode.
const (
	MergeModeNormal               MergeMode = 1
	MergeModeAdd                  MergeMode = 2
	MergeModeSubtract             MergeMode = 3
	MergeModeIntersect            MergeMode = 4
	MergeModeExcludeIntersections MergeMode = 5
)

// UnmarshalJSON reads a value from 1 to 5.
func (m *MergeMode) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(m), 1, 5)
}

// ShapeDirection is ShapeDirection in velato
// src/schema/constants/shape_direction.rs.
type ShapeDirection uint8

// The values of ShapeDirection.
const (
	ShapeDirectionNormal   ShapeDirection = 1
	ShapeDirectionReversed ShapeDirection = 3
)

// UnmarshalJSON reads 1 or 3.
func (d *ShapeDirection) UnmarshalJSON(data []byte) error {
	var v uint8
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v != 1 && v != 3 {
		return fmt.Errorf("invalid value: %d, expected one of: 1, 3", v)
	}
	*d = ShapeDirection(v)
	return nil
}

// StarType is StarType in velato src/schema/constants/star_type.rs.
type StarType uint8

// The values of StarType.
const (
	StarTypeStar    StarType = 1
	StarTypePolygon StarType = 2
)

// UnmarshalJSON reads 1 or 2.
func (s *StarType) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(s), 1, 2)
}

// StrokeDashType is StrokeDashType in velato
// src/schema/constants/stroke_dash_type.rs.
type StrokeDashType string

// UnmarshalJSON reads "d", "g" or "o".
func (s *StrokeDashType) UnmarshalJSON(data []byte) error {
	var v string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v != "d" && v != "g" && v != "o" {
		return fmt.Errorf("unknown variant `%s`", v)
	}
	*s = StrokeDashType(v)
	return nil
}

// TrimMultipleShapes is TrimMultipleShapes in velato
// src/schema/constants/trim_multiple_shapes.rs.
type TrimMultipleShapes uint8

// The values of TrimMultipleShapes.
const (
	TrimMultipleShapesParallel   TrimMultipleShapes = 1
	TrimMultipleShapesSequential TrimMultipleShapes = 2
)

// UnmarshalJSON reads 1 or 2.
func (t *TrimMultipleShapes) UnmarshalJSON(data []byte) error {
	return readEnum(data, (*uint8)(t), 1, 2)
}

// readEnum reads an integer from lo to hi into v, as serde_repr reads an
// enum.
func readEnum(data []byte, v *uint8, lo, hi uint8) error {
	var n uint8
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	if n < lo || n > hi {
		return fmt.Errorf("invalid value: %d, expected a value from %d to %d", n, lo, hi)
	}
	*v = n
	return nil
}
