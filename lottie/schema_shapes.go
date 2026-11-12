// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/schema/shapes.

import (
	"encoding/json"
	"fmt"
)

// AnyShape is AnyShape in velato src/schema/shapes/mod.rs: a shape item,
// tagged by its key "ty" (D27). The field of the type holds the item.
type AnyShape struct {
	Type           string
	Group          *GroupShape
	Rectangle      *RectangleShape
	Ellipse        *EllipseShape
	Transform      *TransformShape
	Stroke         *StrokeShape
	PuckerBloat    *PuckerBloatShape
	Merge          *MergeShape
	Repeater       *RepeaterShape
	OffsetPath     *OffsetPathShape
	Fill           *FillShape
	Trim           *TrimPathShape
	Path           *PathShape
	GradientFill   *GradientFillShape
	GradientStroke *GradientStrokeShape
	Twist          *TwistShape
	RoundedCorners *RoundedCornersShape
	PolyStar       *PolyStarShape
}

// shapeRequired lists the keys that serde requires for each type of shape.
var shapeRequired = map[string][]string{
	"gr": {"it"},
	"rc": {"p", "s", "r"},
	"el": {"p", "s"},
	"tr": {"p"},
	"st": {"o", "w", "c"},
	"rp": {"c", "tr"},
	"fl": {"o", "c"},
	"tm": {"s", "e", "o"},
	"sh": {"ks"},
	"gf": {"o", "g", "s", "e"},
	"gs": {"o", "w", "g", "s", "e"},
	"tw": {"a", "p"},
	"rd": {"r"},
	"sr": {"p", "or", "os", "r", "pt", "sy"},
}

// UnmarshalJSON reads the item of the type that "ty" names. An unknown type
// is an error, as in serde.
func (s *AnyShape) UnmarshalJSON(data []byte) error {
	var tag struct {
		Type *string `json:"ty"`
	}
	if err := json.Unmarshal(data, &tag); err != nil {
		return err
	}
	if tag.Type == nil {
		return errMissingField("ty")
	}
	*s = AnyShape{Type: *tag.Type}
	if err := requireKeys(data, shapeRequired[s.Type]...); err != nil {
		return err
	}
	var target any
	switch s.Type {
	case "gr":
		s.Group = new(GroupShape)
		target = s.Group
	case "rc":
		s.Rectangle = new(RectangleShape)
		target = s.Rectangle
	case "el":
		s.Ellipse = new(EllipseShape)
		target = s.Ellipse
	case "tr":
		s.Transform = new(TransformShape)
		target = s.Transform
	case "st":
		s.Stroke = new(StrokeShape)
		target = s.Stroke
	case "pb":
		s.PuckerBloat = new(PuckerBloatShape)
		target = s.PuckerBloat
	case "mm":
		s.Merge = new(MergeShape)
		target = s.Merge
	case "rp":
		s.Repeater = new(RepeaterShape)
		target = s.Repeater
	case "op":
		s.OffsetPath = new(OffsetPathShape)
		target = s.OffsetPath
	case "fl":
		s.Fill = new(FillShape)
		target = s.Fill
	case "tm":
		s.Trim = new(TrimPathShape)
		target = s.Trim
	case "sh":
		s.Path = new(PathShape)
		target = s.Path
	case "gf":
		s.GradientFill = new(GradientFillShape)
		target = s.GradientFill
	case "gs":
		s.GradientStroke = new(GradientStrokeShape)
		target = s.GradientStroke
	case "tw":
		s.Twist = new(TwistShape)
		target = s.Twist
	case "rd":
		s.RoundedCorners = new(RoundedCornersShape)
		target = s.RoundedCorners
	case "sr":
		s.PolyStar = new(PolyStarShape)
		target = s.PolyStar
	default:
		return fmt.Errorf("unknown variant `%s`", s.Type)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	if s.Transform != nil && s.Transform.Position == nil {
		return errMissingField("p")
	}
	if s.Repeater != nil && s.Repeater.Transform.Position == nil {
		return errMissingField("p")
	}
	return nil
}

// GraphicElementShape is GraphicElementShape in velato
// src/schema/shapes/graphic_element.rs.
type GraphicElementShape struct {
	VisualObject

	Hidden        *bool            `json:"hd,omitempty"`
	BlendMode     *BlendModeSchema `json:"bm,omitempty"`
	PropertyIndex *float64         `json:"ix,omitempty"`
	CSSClass      *string          `json:"cl,omitempty"`
	XMLID         *string          `json:"ln,omitempty"`
	Index         *float64         `json:"ind,omitempty"`
}

// ShapeSchema is Shape in velato src/schema/shapes/shape.rs.
type ShapeSchema struct {
	GraphicElementShape

	Direction *ShapeDirection `json:"d,omitempty"`
}

// ShapeStyleShape is ShapeStyleShape in velato
// src/schema/shapes/shape_style.rs.
type ShapeStyleShape struct {
	GraphicElementShape

	Opacity FloatValue `json:"o"`
}

// ModifierShape is ModifierShape in velato src/schema/shapes/modifier.rs.
type ModifierShape struct {
	GraphicElementShape
}

// BaseGradientShape is BaseGradientShape in velato
// src/schema/shapes/base_gradient.rs.
type BaseGradientShape struct {
	Colors          GradientColors   `json:"g"`
	StartPoint      MultiDimensional `json:"s"`
	EndPoint        MultiDimensional `json:"e"`
	GradientType    *GradientType    `json:"t,omitempty"`
	HighlightLength *FloatValue      `json:"h,omitempty"`
	HighlightAngle  *FloatValue      `json:"a,omitempty"`
}

// BaseStrokeShape is BaseStrokeShape in velato
// src/schema/shapes/base_stroke.rs.
type BaseStrokeShape struct {
	LineCap        *LineCap          `json:"lc,omitempty"`
	LineJoin       *LineJoin         `json:"lj,omitempty"`
	MiterLimit     *float64          `json:"ml,omitempty"`
	MiterLimitAnim *FloatValue       `json:"ml2,omitempty"`
	Width          FloatValue        `json:"w"`
	Dashes         []StrokeDashShape `json:"d,omitempty"`
}

// EllipseShape is EllipseShape in velato src/schema/shapes/ellipse.rs.
type EllipseShape struct {
	ShapeSchema

	Position PositionSchema   `json:"p"`
	Size     MultiDimensional `json:"s"`
}

// FillShape is FillShape in velato src/schema/shapes/fill.rs.
type FillShape struct {
	ShapeStyleShape

	Color    ColorValue `json:"c"`
	FillRule *FillRule  `json:"r,omitempty"`
}

// GradientFillShape is GradientFillShape in velato
// src/schema/shapes/gradient_fill.rs.
type GradientFillShape struct {
	ShapeStyleShape
	BaseGradientShape

	FillRule *FillRule `json:"r,omitempty"`
}

// GradientStrokeShape is GradientStrokeShape in velato
// src/schema/shapes/gradient_stroke.rs.
type GradientStrokeShape struct {
	ShapeStyleShape
	BaseStrokeShape
	BaseGradientShape
}

// GroupShape is GroupShape in velato src/schema/shapes/group.rs.
type GroupShape struct {
	GraphicElementShape

	NumProperties *float64   `json:"np,omitempty"`
	Shapes        []AnyShape `json:"it"`
	PropertyIdx   *uint8     `json:"cix,omitempty"`
}

// MergeShape is MergeShape in velato src/schema/shapes/merge.rs.
type MergeShape struct {
	GraphicElementShape

	MergeMode *MergeMode `json:"mm,omitempty"`
}

// OffsetPathShape is OffsetPathShape in velato
// src/schema/shapes/offset_path.rs.
type OffsetPathShape struct {
	ModifierShape

	Amount     *FloatValue `json:"a,omitempty"`
	LineJoin   *LineJoin   `json:"lj,omitempty"`
	MiterLimit *FloatValue `json:"ml,omitempty"`
}

// PathShape is PathShape in velato src/schema/shapes/path.rs.
type PathShape struct {
	ShapeSchema

	ShapeProperty ShapeProperty `json:"ks"`
}

// PolyStarShape is PolyStarShape in velato src/schema/shapes/polystar.rs.
type PolyStarShape struct {
	ShapeSchema

	Position       PositionSchema `json:"p"`
	OuterRadius    FloatValue     `json:"or"`
	OuterRoundness FloatValue     `json:"os"`
	Rotation       FloatValue     `json:"r"`
	Points         FloatValue     `json:"pt"`
	StarType       StarType       `json:"sy"`
	InnerRadius    *FloatValue    `json:"ir,omitempty"`
	InnerRoundness *FloatValue    `json:"is,omitempty"`
}

// PuckerBloatShape is PuckerBloatShape in velato
// src/schema/shapes/pucker_bloat.rs.
type PuckerBloatShape struct {
	ModifierShape

	Amount *FloatValue `json:"a,omitempty"`
}

// RectangleShape is RectangleShape in velato src/schema/shapes/rectangle.rs.
type RectangleShape struct {
	ShapeSchema

	Position            PositionSchema   `json:"p"`
	Size                MultiDimensional `json:"s"`
	RoundedCornerRadius FloatValue       `json:"r"`
}

// RepeaterShape is RepeaterShape in velato src/schema/shapes/repeater.rs.
type RepeaterShape struct {
	ModifierShape

	Copies    FloatValue             `json:"c"`
	Offset    *FloatValue            `json:"o,omitempty"`
	Composite *Composite             `json:"m,omitempty"`
	Transform RepeaterTransformShape `json:"tr"`
}

// RepeaterTransformShape is RepeaterTransformShape in velato
// src/schema/shapes/repeater_transform.rs.
type RepeaterTransformShape struct {
	TransformShape

	StartOpacity *FloatValue `json:"so,omitempty"`
	EndOpacity   *FloatValue `json:"eo,omitempty"`
}

// RoundedCornersShape is RoundedCornersShape in velato
// src/schema/shapes/rounded_corners.rs.
type RoundedCornersShape struct {
	ModifierShape

	Radius FloatValue `json:"r"`
}

// StrokeDashShape is StrokeDashShape in velato
// src/schema/shapes/stroke_dash.rs.
type StrokeDashShape struct {
	VisualObject

	DashType *StrokeDashType `json:"n,omitempty"`
	Length   *FloatValue     `json:"v,omitempty"`
}

// StrokeShape is StrokeShape in velato src/schema/shapes/stroke.rs.
type StrokeShape struct {
	ShapeStyleShape
	BaseStrokeShape

	StrokeColor ColorValue `json:"c"`
}

// TransformShape is TransformShape in velato src/schema/shapes/transform.rs.
type TransformShape struct {
	GraphicElementShape
	TransformSchema
}

// TrimPathShape is TrimPathShape in velato src/schema/shapes/trim_path.rs.
type TrimPathShape struct {
	ModifierShape

	Start    FloatValue          `json:"s"`
	End      FloatValue          `json:"e"`
	Offset   FloatValue          `json:"o"`
	Multiple *TrimMultipleShapes `json:"m,omitempty"`
}

// TwistShape is TwistShape in velato src/schema/shapes/twist.rs.
type TwistShape struct {
	ModifierShape

	Angle    FloatValue     `json:"a"`
	Position PositionSchema `json:"p"`
}
