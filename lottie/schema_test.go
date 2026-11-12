// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports the tests of velato src/schema: the tests that read JSON.
// The tests that write JSON have no port, because okage only reads it.

import (
	"encoding/json"
	"testing"
)

// TestAnimationDeserialize is test_deserialize in velato src/schema/mod.rs.
func TestAnimationDeserialize(t *testing.T) {
	a, err := ParseAnimation([]byte(`{"v": "5.5.2", "nm": "Example", "fr": 60, "ip": 0, "op": 60, "w": 512, "h": 512, "ddd": 0, "layers": []}`))
	if err != nil {
		t.Fatal(err)
	}
	switch {
	case a.Name == nil || *a.Name != "Example":
		t.Errorf("name is %v", a.Name)
	case a.Version == nil || *a.Version != "5.5.2":
		t.Errorf("version is %v", a.Version)
	case a.FrameRate != 60 || a.InPoint != 0 || a.OutPoint != 60 || a.Width != 512 || a.Height != 512:
		t.Errorf("got %+v", a)
	case a.ThreeDimensional == nil || *a.ThreeDimensional != BoolIntFalse:
		t.Errorf("ddd is %v", a.ThreeDimensional)
	case len(a.Layers) != 0 || a.Assets != nil:
		t.Errorf("got layers %v and assets %v", a.Layers, a.Assets)
	}
}

// TestLayerDeserialize is test_deserialize in velato
// src/schema/layers/mod.rs.
func TestLayerDeserialize(t *testing.T) {
	var l AnyLayer
	err := json.Unmarshal([]byte(`{
		"ddd": 0, "ty": 4, "ind": 1, "st": 0, "ip": 0, "op": 180, "nm": "Ellipse",
		"mn": "{0a36d01c-18e1-48d3-8e8f-cc093b3f24ba}",
		"ks": {
			"a": {"a": 0, "k": [256, 256]},
			"p": {"a": 0, "k": [256, 256]},
			"s": {"a": 0, "k": [100, 100]},
			"r": {"a": 0, "k": 0},
			"o": {"a": 0, "k": 100}
		},
		"shapes": [{
			"ty": "gr", "nm": "Group", "mn": "{f1becc2a-49f0-4f0c-918f-bdffe4c6870f}",
			"it": [{
				"ty": "el", "nm": "Ellipse", "mn": "{2aabac6e-1dd8-41b0-b60b-baf75ccb6318}",
				"p": {"a": 0, "k": [303.9044776119403, 324.9671641791045]},
				"s": {"a": 0, "k": [205.46865671641788, 204.6089552238806]}
			}]
		}]
	}`), &l)
	if err != nil {
		t.Fatal(err)
	}
	s := l.Shape
	if s == nil {
		t.Fatal("not a shape layer")
	}
	if *s.Name != "Ellipse" || s.LayerType != 4 || *s.Index != 1 || *s.StartTime != 0 || s.OutPoint != 180 {
		t.Errorf("got %+v", s.LayerSchema)
	}
	if p := s.Transform.Position; p == nil || p.IsSplit || p.Position.Value.Static[0] != 256 {
		t.Errorf("position is %+v", p)
	}
	if r := s.Transform.Rotation; r == nil || r.IsSplit || r.Rotation.Value.Static != 0 {
		t.Errorf("rotation is %+v", r)
	}
	if len(s.Shapes) != 1 || s.Shapes[0].Group == nil || len(s.Shapes[0].Group.Shapes) != 1 {
		t.Fatalf("shapes are %+v", s.Shapes)
	}
	e := s.Shapes[0].Group.Shapes[0].Ellipse
	if e == nil || *e.Name != "Ellipse" || e.Size.Value.Static[1] != 204.6089552238806 {
		t.Errorf("ellipse is %+v", e)
	}
}

// TestPrecompositionDeserialize is test_serde_deserialize in velato
// src/schema/layers/precomposition.rs.
func TestPrecompositionDeserialize(t *testing.T) {
	var a AnyAsset
	if err := json.Unmarshal([]byte(`{"id": "precomp_0", "fr": 60, "nm": "Example", "xt": 0, "layers": []}`), &a); err != nil {
		t.Fatal(err)
	}
	p := a.Precomposition
	if p == nil || p.ID != "precomp_0" || *p.Name != "Example" || *p.FrameRate != 60 || *p.Extra != BoolIntFalse || len(p.Layers) != 0 {
		t.Errorf("got %+v", p)
	}
}

// TestShapeDeserialize is test_deserialize in velato
// src/schema/shapes/mod.rs.
func TestShapeDeserialize(t *testing.T) {
	var s AnyShape
	err := json.Unmarshal([]byte(`{
		"ty": "gr", "nm": "Group", "mn": "{f1becc2a-49f0-4f0c-918f-bdffe4c6870f}",
		"it": [{
			"ty": "el", "nm": "Ellipse", "mn": "{2aabac6e-1dd8-41b0-b60b-baf75ccb6318}",
			"p": {"a": 0, "k": [303.9044776119403, 324.9671641791045]},
			"s": {"a": 0, "k": [205.46865671641788, 204.6089552238806]}
		}]
	}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	if s.Group == nil || *s.Group.Name != "Group" || len(s.Group.Shapes) != 1 {
		t.Fatalf("got %+v", s)
	}
	e := s.Group.Shapes[0].Ellipse
	if e == nil || e.Position.Value.Static[0] != 303.9044776119403 || *e.Position.Animated != BoolIntFalse {
		t.Errorf("ellipse is %+v", e)
	}
}

// TestImageDeserialize is test_deserialize in velato
// src/schema/assets/image.rs.
func TestImageDeserialize(t *testing.T) {
	var a AnyAsset
	if err := json.Unmarshal([]byte(`{"id": "my image", "h": 512, "w": 512, "e": 1, "p": "data:image/png;base64,..."}`), &a); err != nil {
		t.Fatal(err)
	}
	img := a.Image
	if img == nil || img.ID != "my image" || *img.Width != 512 || *img.Height != 512 ||
		*img.Embedded != BoolIntTrue || img.FileName != "data:image/png;base64,..." || img.Dir != nil || img.Sequence != nil {
		t.Errorf("got %+v", img)
	}
}

// TestUnknownLayerType checks that a layer of another type is an error, as
// in velato.
func TestUnknownLayerType(t *testing.T) {
	var l AnyLayer
	if err := json.Unmarshal([]byte(`{"ty": 5, "ip": 0, "op": 1, "ks": {"p": {"k": [0, 0]}}}`), &l); err == nil {
		t.Error("a text layer gave no error")
	}
}
