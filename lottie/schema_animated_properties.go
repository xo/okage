// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/schema/animated_properties: the animated
// properties of the JSON of a Lottie file.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// AnimatedProperty is AnimatedProperty in velato
// src/schema/animated_properties/animated_property.rs: a property that is a
// static value of type T, or a list of keyframes.
type AnimatedProperty[T any] struct {
	PropertyIndex *float64 `json:"ix,omitempty"`
	Animated      *BoolInt `json:"a,omitempty"`
	Expression    *string  `json:"x,omitempty"`
	SlotID        *string  `json:"sid,omitempty"`
	Value         AnimatedPropertyK[T]
}

// UnmarshalJSON reads the property. The key "k" is required.
func (p *AnimatedProperty[T]) UnmarshalJSON(data []byte) error {
	var v animatedPropertyFields
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.K == nil {
		return errMissingField("k")
	}
	p.PropertyIndex, p.Animated, p.Expression, p.SlotID = v.PropertyIndex, v.Animated, v.Expression, v.SlotID
	return p.Value.UnmarshalJSON(v.K)
}

// animatedPropertyFields holds the keys of an AnimatedProperty, with "k"
// raw.
//
// Port: it is not a type in the generic method, because TinyGo 0.44 cannot
// compile a local type in a generic function.
type animatedPropertyFields struct {
	PropertyIndex *float64        `json:"ix"`
	Animated      *BoolInt        `json:"a"`
	Expression    *string         `json:"x"`
	SlotID        *string         `json:"sid"`
	K             json.RawMessage `json:"k"`
}

// AnimatedPropertyK is AnimatedPropertyK in velato
// src/schema/animated_properties/animated_property.rs. It is an untagged
// enum: a list of keyframes, or a static value (D27). Keyframes is nil for a
// static value.
type AnimatedPropertyK[T any] struct {
	Keyframes []Keyframe
	Static    T
	// IsAnimated is true for the variant AnimatedValue.
	IsAnimated bool
}

// UnmarshalJSON tries the keyframes first, then the static value, as serde
// tries the variants of an untagged enum in order.
func (k *AnimatedPropertyK[T]) UnmarshalJSON(data []byte) error {
	if isKeyframeList(data) {
		var frames []Keyframe
		if err := json.Unmarshal(data, &frames); err == nil {
			k.Keyframes, k.IsAnimated = frames, true
			return nil
		}
	}
	var static T
	if err := json.Unmarshal(data, &static); err != nil {
		return fmt.Errorf("data did not match any variant of untagged enum AnimatedPropertyK: %w", err)
	}
	k.Static, k.IsAnimated = static, false
	return nil
}

// isKeyframeList reports if data is a JSON array that is empty or holds
// objects. serde reads such an array as a list of keyframes.
func isKeyframeList(data []byte) bool {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '[' {
		return false
	}
	rest := bytes.TrimSpace(data[1:])
	return len(rest) > 0 && (rest[0] == ']' || rest[0] == '{')
}

// FloatValue is FloatValue in velato
// src/schema/animated_properties/value.rs.
type FloatValue struct {
	AnimatedProperty[float64]
}

// MultiDimensional is MultiDimensional in velato
// src/schema/animated_properties/multi_dimensional.rs.
type MultiDimensional struct {
	AnimatedProperty[[]float64]

	Length *float64
}

// UnmarshalJSON reads the property and its key "l".
func (m *MultiDimensional) UnmarshalJSON(data []byte) error {
	if err := m.AnimatedProperty.UnmarshalJSON(data); err != nil {
		return err
	}
	var v struct {
		Length *float64 `json:"l"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	m.Length = v.Length
	return nil
}

// ColorValue is ColorValue in velato
// src/schema/animated_properties/color_value.rs.
type ColorValue struct {
	AnimatedProperty[[]float64]
}

// GradientColors is GradientColors in velato
// src/schema/animated_properties/gradient_colors.rs.
type GradientColors struct {
	Colors MultiDimensional `json:"k"`
	Count  uint             `json:"p"`
}

// UnmarshalJSON requires the keys "k" and "p".
func (g *GradientColors) UnmarshalJSON(data []byte) error {
	var v struct {
		Colors *MultiDimensional `json:"k"`
		Count  *uint             `json:"p"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.Colors == nil {
		return errMissingField("k")
	}
	if v.Count == nil {
		return errMissingField("p")
	}
	g.Colors, g.Count = *v.Colors, *v.Count
	return nil
}

// KeyframeBase is KeyframeBase in velato
// src/schema/animated_properties/keyframe_base.rs.
type KeyframeBase struct {
	Time       float64               `json:"t"`
	Hold       *BoolInt              `json:"h,omitempty"`
	InTangent  *KeyframeBezierHandle `json:"i,omitempty"`
	OutTangent *KeyframeBezierHandle `json:"o,omitempty"`
}

// readKeyframeBase reads a KeyframeBase from data, with the key "t"
// required.
func readKeyframeBase(data []byte) (KeyframeBase, error) {
	var v struct {
		Time       *float64              `json:"t"`
		Hold       *BoolInt              `json:"h"`
		InTangent  *KeyframeBezierHandle `json:"i"`
		OutTangent *KeyframeBezierHandle `json:"o"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return KeyframeBase{}, err
	}
	if v.Time == nil {
		return KeyframeBase{}, errMissingField("t")
	}
	return KeyframeBase{Time: *v.Time, Hold: v.Hold, InTangent: v.InTangent, OutTangent: v.OutTangent}, nil
}

// KeyframeBezierHandle is KeyframeBezierHandle in velato
// src/schema/animated_properties/keyframe_bezier_handle.rs.
type KeyframeBezierHandle struct {
	XCoordinate KeyframeComponent `json:"x"`
	YCoordinate KeyframeComponent `json:"y"`
}

// UnmarshalJSON requires the keys "x" and "y".
func (h *KeyframeBezierHandle) UnmarshalJSON(data []byte) error {
	var v struct {
		X *KeyframeComponent `json:"x"`
		Y *KeyframeComponent `json:"y"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.X == nil {
		return errMissingField("x")
	}
	if v.Y == nil {
		return errMissingField("y")
	}
	h.XCoordinate, h.YCoordinate = *v.X, *v.Y
	return nil
}

// KeyframeComponent is KeyframeComponent in velato
// src/schema/animated_properties/keyframe_bezier_handle.rs: an array of
// values, or a single value (D27).
type KeyframeComponent struct {
	Values  []float64
	Single  float64
	IsArray bool
}

// UnmarshalJSON reads an array or a number.
func (c *KeyframeComponent) UnmarshalJSON(data []byte) error {
	var values []float64
	if err := json.Unmarshal(data, &values); err == nil {
		*c = KeyframeComponent{Values: values, IsArray: true}
		return nil
	}
	var single float64
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("data did not match any variant of untagged enum KeyframeComponent: %w", err)
	}
	*c = KeyframeComponent{Single: single}
	return nil
}

// Keyframe is Keyframe in velato src/schema/animated_properties/keyframe.rs.
type Keyframe struct {
	Base               KeyframeBase
	Value              []float64
	EndValueDeprecated []float64
}

// UnmarshalJSON reads the keyframe. The keys "t" and "s" are required.
func (k *Keyframe) UnmarshalJSON(data []byte) error {
	base, err := readKeyframeBase(data)
	if err != nil {
		return err
	}
	var v struct {
		Value *[]float64 `json:"s"`
		End   []float64  `json:"e"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.Value == nil {
		return errMissingField("s")
	}
	*k = Keyframe{Base: base, Value: *v.Value, EndValueDeprecated: v.End}
	return nil
}

// PositionKeyframe is PositionKeyframe in velato
// src/schema/animated_properties/position_keyframe.rs.
type PositionKeyframe struct {
	Keyframe

	ValueInTangent  []float64
	ValueOutTangent []float64
}

// UnmarshalJSON reads the keyframe and its spatial tangents.
func (k *PositionKeyframe) UnmarshalJSON(data []byte) error {
	if err := k.Keyframe.UnmarshalJSON(data); err != nil {
		return err
	}
	var v struct {
		In  []float64 `json:"ti"`
		Out []float64 `json:"to"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	k.ValueInTangent, k.ValueOutTangent = v.In, v.Out
	return nil
}

// PositionSchema is Position in velato
// src/schema/animated_properties/position.rs.
type PositionSchema struct {
	PropertyIndex *float64
	Animated      *BoolInt
	Expression    *string
	Length        *float64
	Value         PositionValueK
}

// UnmarshalJSON reads the position. The key "k" is required.
func (p *PositionSchema) UnmarshalJSON(data []byte) error {
	var v struct {
		PropertyIndex *float64        `json:"ix"`
		Animated      *BoolInt        `json:"a"`
		Expression    *string         `json:"x"`
		Length        *float64        `json:"l"`
		K             json.RawMessage `json:"k"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.K == nil {
		return errMissingField("k")
	}
	*p = PositionSchema{PropertyIndex: v.PropertyIndex, Animated: v.Animated, Expression: v.Expression, Length: v.Length}
	return p.Value.UnmarshalJSON(v.K)
}

// PositionValueK is PositionValueK in velato
// src/schema/animated_properties/position.rs: a list of keyframes, or a
// static value (D27).
type PositionValueK struct {
	Keyframes  []PositionKeyframe
	Static     []float64
	IsAnimated bool
}

// UnmarshalJSON tries the keyframes first, then the static value.
func (k *PositionValueK) UnmarshalJSON(data []byte) error {
	if isKeyframeList(data) {
		var frames []PositionKeyframe
		if err := json.Unmarshal(data, &frames); err == nil {
			*k = PositionValueK{Keyframes: frames, IsAnimated: true}
			return nil
		}
	}
	var static []float64
	if err := json.Unmarshal(data, &static); err != nil {
		return fmt.Errorf("data did not match any variant of untagged enum PositionValueK: %w", err)
	}
	*k = PositionValueK{Static: static}
	return nil
}

// ShapeKeyframe is ShapeKeyframe in velato
// src/schema/animated_properties/shape_keyframe.rs.
type ShapeKeyframe struct {
	Base  KeyframeBase
	Start []Bezier
	End   []Bezier
	// HasStart is true when the key "s" is present.
	HasStart bool
	HasEnd   bool
}

// UnmarshalJSON reads the keyframe. The key "t" is required.
func (k *ShapeKeyframe) UnmarshalJSON(data []byte) error {
	base, err := readKeyframeBase(data)
	if err != nil {
		return err
	}
	var v struct {
		Start *[]Bezier `json:"s"`
		End   *[]Bezier `json:"e"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*k = ShapeKeyframe{Base: base}
	if v.Start != nil {
		k.Start, k.HasStart = *v.Start, true
	}
	if v.End != nil {
		k.End, k.HasEnd = *v.End, true
	}
	return nil
}

// ShapeProperty is ShapeProperty in velato
// src/schema/animated_properties/shape_property.rs.
type ShapeProperty struct {
	PropertyIndex *float64
	Animated      *BoolInt
	Expression    *string
	Value         ShapePropertyK
}

// UnmarshalJSON reads the property. The key "k" is required.
func (p *ShapeProperty) UnmarshalJSON(data []byte) error {
	var v struct {
		PropertyIndex *float64        `json:"ix"`
		Animated      *BoolInt        `json:"a"`
		Expression    *string         `json:"x"`
		K             json.RawMessage `json:"k"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if v.K == nil {
		return errMissingField("k")
	}
	*p = ShapeProperty{PropertyIndex: v.PropertyIndex, Animated: v.Animated, Expression: v.Expression}
	return p.Value.UnmarshalJSON(v.K)
}

// ShapePropertyK is ShapePropertyK in velato
// src/schema/animated_properties/shape_property.rs: a list of keyframes, or
// a static Bezier (D27).
type ShapePropertyK struct {
	Keyframes  []ShapeKeyframe
	Static     Bezier
	IsAnimated bool
}

// UnmarshalJSON tries the keyframes first, with the checks of
// deserialize_keyframes, then the static value.
func (k *ShapePropertyK) UnmarshalJSON(data []byte) error {
	var keyframesErr error
	if isKeyframeList(data) {
		var frames []ShapeKeyframe
		if keyframesErr = json.Unmarshal(data, &frames); keyframesErr == nil {
			if keyframesErr = checkShapeKeyframes(frames); keyframesErr == nil {
				*k = ShapePropertyK{Keyframes: frames, IsAnimated: true}
				return nil
			}
		}
	}
	var static Bezier
	if err := json.Unmarshal(data, &static); err != nil {
		return fmt.Errorf("data did not match any variant of untagged enum ShapePropertyK: %w", err)
	}
	*k = ShapePropertyK{Static: static}
	return nil
}

// checkShapeKeyframes is the check of deserialize_keyframes in velato
// src/schema/animated_properties/shape_property.rs: only the last keyframe
// can omit "s", and only when another keyframe is before it.
func checkShapeKeyframes(frames []ShapeKeyframe) error {
	if len(frames) == 0 {
		return nil
	}
	for _, f := range frames[:len(frames)-1] {
		if !f.HasStart {
			return errors.New("animated shape keyframe found with missing 's' start property: only the last keyframe may omit it")
		}
	}
	if frames[len(frames)-1].HasStart {
		return nil
	}
	if len(frames) < 2 {
		return errors.New("last animated shape keyframe 's' was omitted where only one keyframe is present")
	}
	return nil
}

// SplitVector is SplitVector in velato
// src/schema/animated_properties/split_vector.rs.
type SplitVector struct {
	Split bool
	X     FloatValue
	Y     FloatValue
	Z     *FloatValue
}

// UnmarshalJSON requires the keys "s", "x" and "y".
func (s *SplitVector) UnmarshalJSON(data []byte) error {
	var v struct {
		Split *bool       `json:"s"`
		X     *FloatValue `json:"x"`
		Y     *FloatValue `json:"y"`
		Z     *FloatValue `json:"z"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch {
	case v.Split == nil:
		return errMissingField("s")
	case v.X == nil:
		return errMissingField("x")
	case v.Y == nil:
		return errMissingField("y")
	}
	*s = SplitVector{Split: *v.Split, X: *v.X, Y: *v.Y, Z: v.Z}
	return nil
}

// errMissingField is the error of serde for a missing field.
func errMissingField(name string) error {
	return fmt.Errorf("missing field `%s`", name)
}
