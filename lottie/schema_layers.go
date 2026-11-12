// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports velato src/schema/layers, src/schema/assets and
// src/schema/animation.

import (
	"encoding/json"
	"fmt"
)

// LayerSchema is Layer in velato src/schema/layers/layer.rs.
type LayerSchema struct {
	VisualObject

	ThreeDimensional *BoolInt `json:"ddd,omitempty"`
	Hidden           *bool    `json:"hd,omitempty"`
	LayerType        uint8    `json:"ty"`
	Index            *uint    `json:"ind,omitempty"`
	ParentIndex      *uint    `json:"parent,omitempty"`
	TimeStretch      *float64 `json:"sr,omitempty"`
	InPoint          float64  `json:"ip"`
	OutPoint         float64  `json:"op"`
	StartTime        *float64 `json:"st,omitempty"`
}

// VisualLayer is VisualLayer in velato src/schema/layers/visual.rs.
type VisualLayer struct {
	LayerSchema

	MatteMode                *MatteMode        `json:"tt,omitempty"`
	MatteTarget              *BoolInt          `json:"td,omitempty"`
	MasksProperties          []MaskSchema      `json:"masksProperties,omitempty"`
	Effects                  []json.RawMessage `json:"ef,omitempty"`
	Transform                TransformSchema   `json:"ks"`
	RotateToMatchAnimPosPath *BoolInt          `json:"ao,omitempty"`
	MatteLayerIndex          *float64          `json:"tp,omitempty"`
	HasMask                  *bool             `json:"hasMask,omitempty"`
	MotionBlur               *bool             `json:"mb,omitempty"`
	BlendMode                *BlendModeSchema  `json:"bm,omitempty"`
	CSSClass                 *string           `json:"cl,omitempty"`
	ID                       *string           `json:"ln,omitempty"`
	TagName                  *string           `json:"tg,omitempty"`
	TransformBeforeMask      *BoolInt          `json:"ct,omitempty"`
}

// ImageLayer is ImageLayer in velato src/schema/layers/image.rs.
type ImageLayer struct {
	VisualLayer

	RefID string `json:"refId"`
}

// NullLayer is NullLayer in velato src/schema/layers/null.rs.
type NullLayer struct {
	VisualLayer
}

// PrecompositionLayer is PrecompositionLayer in velato
// src/schema/layers/precomposition.rs.
type PrecompositionLayer struct {
	VisualLayer

	RefID     string      `json:"refId"`
	Width     float64     `json:"w"`
	Height    float64     `json:"h"`
	StartTime float64     `json:"st"`
	TimeRemap *FloatValue `json:"tm,omitempty"`
}

// ShapeLayer is ShapeLayer in velato src/schema/layers/shape.rs.
type ShapeLayer struct {
	VisualLayer

	Shapes []AnyShape `json:"shapes"`
}

// SolidLayer is SolidLayer in velato src/schema/layers/solid.rs.
type SolidLayer struct {
	VisualLayer

	Width  float64 `json:"sw"`
	Height float64 `json:"sh"`
	Color  string  `json:"sc"`
}

// AnyLayer is AnyLayer in velato src/schema/layers/mod.rs: a layer of the
// type that "ty" names (D27).
type AnyLayer struct {
	Precomposition *PrecompositionLayer
	Solid          *SolidLayer
	Shape          *ShapeLayer
	Null           *NullLayer
	Image          *ImageLayer
}

// layerRequired lists the keys that serde requires for each type of layer.
var layerRequired = map[uint64][]string{
	0: {"ty", "ip", "op", "ks", "refId", "w", "h", "st"},
	1: {"ty", "ip", "op", "ks", "sw", "sh", "sc"},
	2: {"ty", "ip", "op", "ks", "refId"},
	3: {"ty", "ip", "op", "ks"},
	4: {"ty", "ip", "op", "ks", "shapes"},
}

// UnmarshalJSON reads the layer of the type that "ty" names. Another type is
// an error.
func (l *AnyLayer) UnmarshalJSON(data []byte) error {
	var tag struct {
		Type *uint64 `json:"ty"`
	}
	if err := json.Unmarshal(data, &tag); err != nil || tag.Type == nil {
		return errMissingField("ty")
	}
	required, ok := layerRequired[*tag.Type]
	if !ok {
		return fmt.Errorf("unknown layer type: %d", *tag.Type)
	}
	if err := requireKeys(data, required...); err != nil {
		return err
	}
	*l = AnyLayer{}
	var target any
	switch *tag.Type {
	case 0:
		l.Precomposition = new(PrecompositionLayer)
		target = l.Precomposition
	case 1:
		l.Solid = new(SolidLayer)
		target = l.Solid
	case 2:
		l.Image = new(ImageLayer)
		target = l.Image
	case 3:
		l.Null = new(NullLayer)
		target = l.Null
	case 4:
		l.Shape = new(ShapeLayer)
		target = l.Shape
	}
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	if l.visual().Transform.Position == nil {
		return errMissingField("p")
	}
	return nil
}

// visual returns the VisualLayer of the layer.
func (l *AnyLayer) visual() *VisualLayer {
	switch {
	case l.Precomposition != nil:
		return &l.Precomposition.VisualLayer
	case l.Solid != nil:
		return &l.Solid.VisualLayer
	case l.Shape != nil:
		return &l.Shape.VisualLayer
	case l.Null != nil:
		return &l.Null.VisualLayer
	}
	return &l.Image.VisualLayer
}

// Asset is Asset in velato src/schema/assets/asset.rs.
type Asset struct {
	VisualObject

	ID string `json:"id"`
}

// FileAsset is FileAsset in velato src/schema/assets/file_asset.rs.
type FileAsset struct {
	Asset

	Dir      *string  `json:"u,omitempty"`
	FileName string   `json:"p"`
	Embedded *BoolInt `json:"e,omitempty"`
}

// Image is Image in velato src/schema/assets/image.rs.
type Image struct {
	FileAsset

	SlotID   *string  `json:"sid,omitempty"`
	Width    *float64 `json:"w,omitempty"`
	Height   *float64 `json:"h,omitempty"`
	Sequence *bool    `json:"-"`
}

// Precomposition is Precomposition in velato
// src/schema/assets/precomposition.rs.
type Precomposition struct {
	Asset
	CompositionSchema

	FrameRate *float64 `json:"fr,omitempty"`
	Extra     *BoolInt `json:"xt,omitempty"`
}

// AnyAsset is AnyAsset in velato src/schema/assets/mod.rs: an image or a
// precomposition (D27).
type AnyAsset struct {
	Image          *Image
	Precomposition *Precomposition
}

// UnmarshalJSON picks the type of the asset by its keys, as velato does.
func (a *AnyAsset) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	id := "<unknown>"
	if raw, ok := m["id"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			id = s
		}
	}
	*a = AnyAsset{}
	if _, ok := m["layers"]; ok {
		p := new(Precomposition)
		err := requireKeys(data, "id", "layers")
		if err == nil {
			err = json.Unmarshal(data, p)
		}
		if err != nil {
			return fmt.Errorf("asset id=%q (Precomposition): %w", id, err)
		}
		a.Precomposition = p
		return nil
	}
	if raw, ok := m["t"]; ok {
		var t uint64
		if json.Unmarshal(raw, &t) == nil && t == 3 {
			return fmt.Errorf("asset id=%q: DataSource asset not yet supported", id)
		}
	}
	_, hasW := m["w"]
	_, hasH := m["h"]
	if hasW || hasH {
		img := new(Image)
		err := requireKeys(data, "id", "p")
		if err == nil {
			err = json.Unmarshal(data, img)
		}
		if err == nil {
			if raw, ok := m["t"]; ok {
				var s string
				if err = json.Unmarshal(raw, &s); err == nil && s == "seq" {
					seq := true
					img.Sequence = &seq
				}
			}
		}
		if err != nil {
			return fmt.Errorf("asset id=%q (Image): %w", id, err)
		}
		a.Image = img
		return nil
	}
	return fmt.Errorf("asset id=%q: Sound asset not yet supported", id)
}

// CompositionSchema is Composition in velato
// src/schema/animation/composition.rs.
type CompositionSchema struct {
	Layers []AnyLayer `json:"layers"`
}

// Meta is Meta in velato src/schema/animation/meta.rs.
type Meta struct {
	Author      *string         `json:"a,omitempty"`
	Description *string         `json:"d,omitempty"`
	ThemeColor  *string         `json:"tc,omitempty"`
	Generator   *string         `json:"g,omitempty"`
	Keywords    json.RawMessage `json:"k,omitempty"`
}

// Animation is Animation in velato src/schema/animation/animation.rs: the
// top level object of a Lottie file.
type Animation struct {
	VisualObject
	CompositionSchema

	Version          *string         `json:"v,omitempty"`
	Ver              *uint32         `json:"ver,omitempty"`
	FrameRate        float64         `json:"fr"`
	InPoint          float64         `json:"ip"`
	OutPoint         float64         `json:"op"`
	Width            uint            `json:"w"`
	Height           uint            `json:"h"`
	ThreeDimensional *BoolInt        `json:"ddd,omitempty"`
	Assets           []AnyAsset      `json:"assets,omitempty"`
	Markers          []Marker        `json:"markers,omitempty"`
	Meta             *Meta           `json:"meta,omitempty"`
	Metadata         json.RawMessage `json:"metadata,omitempty"`
}

// ParseAnimation is Animation::from_slice in velato
// src/schema/animation/animation.rs.
func ParseAnimation(data []byte) (*Animation, error) {
	if err := requireKeys(data, "layers", "fr", "ip", "op", "w", "h"); err != nil {
		return nil, err
	}
	a := new(Animation)
	if err := json.Unmarshal(data, a); err != nil {
		return nil, err
	}
	return a, nil
}
