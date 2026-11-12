// Copyright 2024 the Velato Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports the tests of velato src/import/converters.rs.

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/kurbo"
)

// makeShapeLayer is make_shape_layer in the tests of velato
// src/import/converters.rs.
func makeShapeLayer(t *testing.T, hidden, matteTarget bool) *AnyLayer {
	t.Helper()
	extra := ""
	if hidden {
		extra += `, "hd": true`
	}
	if matteTarget {
		extra += `, "td": 1`
	}
	var l AnyLayer
	err := json.Unmarshal([]byte(fmt.Sprintf(`{
		"nm": "test", "ty": 4, "ind": 0, "st": 0, "ip": 0, "op": 60,
		"ks": {"p": {"a": 0, "k": [0, 0]}, "r": {"a": 0, "k": 0}},
		"shapes": []%s
	}`, extra)), &l)
	if err != nil {
		t.Fatal(err)
	}
	return &l
}

// TestHiddenLayerRetainsContent is hidden_layer_retains_content.
func TestHiddenLayerRetainsContent(t *testing.T) {
	var c converter
	e := c.convLayer(makeShapeLayer(t, true, false))
	if !e.layer.Hidden || e.layer.Content.Kind != ContentShape {
		t.Errorf("got %+v", e.layer)
	}
}

// TestHiddenMatteLayerRetainsIsMatteSource is
// hidden_matte_layer_retains_is_matte_source.
func TestHiddenMatteLayerRetainsIsMatteSource(t *testing.T) {
	var c converter
	e := c.convLayer(makeShapeLayer(t, true, true))
	if !e.layer.Hidden || !e.layer.IsMatteSource {
		t.Errorf("got %+v", e.layer)
	}
}

// TestVisibleMatteLayerHasIsMatteSourceTrue is
// visible_matte_layer_has_is_matte_source_true.
func TestVisibleMatteLayerHasIsMatteSourceTrue(t *testing.T) {
	var c converter
	if e := c.convLayer(makeShapeLayer(t, false, true)); !e.layer.IsMatteSource {
		t.Errorf("got %+v", e.layer)
	}
}

// TestVisibleLayerHasContent is visible_layer_has_content.
func TestVisibleLayerHasContent(t *testing.T) {
	var c converter
	if e := c.convLayer(makeShapeLayer(t, false, false)); e.layer.Content.Kind != ContentShape {
		t.Errorf("got %+v", e.layer)
	}
}

// TestImageAssetsAndLayerReferencesAreRetained is
// image_assets_and_layer_references_are_retained.
func TestImageAssetsAndLayerReferencesAreRetained(t *testing.T) {
	c, err := Parse([]byte(`{
		"v": "5.5.2", "fr": 60, "ip": 0, "op": 60, "w": 512, "h": 512,
		"assets": [{"id": "texture", "h": 32, "w": 64, "u": "i/", "p": "texture.png", "e": 0}],
		"layers": [{
			"ty": 2, "ind": 1, "ip": 0, "op": 60, "st": 0, "refId": "texture",
			"ks": {
				"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]},
				"s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0},
				"o": {"a": 0, "k": 100}, "sk": {"a": 0, "k": 0}, "sa": {"a": 0, "k": 0}
			}
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	image, ok := c.Images["texture"]
	if !ok {
		t.Fatal("no image")
	}
	if *image.Width != 64 || *image.Height != 32 || image.Location() != "i/texture.png" || image.Embedded {
		t.Errorf("got %+v", image)
	}
	if l := c.Layers[0].Content; l.Kind != ContentImage || l.Name != "texture" {
		t.Errorf("content is %+v", l)
	}
}

// TestRepeaterRetainsSplitPositionIdentityScaleAndComposite is
// repeater_retains_split_position_identity_scale_and_composite.
func TestRepeaterRetainsSplitPositionIdentityScaleAndComposite(t *testing.T) {
	var s AnyShape
	err := json.Unmarshal([]byte(`{
		"ty": "rp", "c": {"a": 0, "k": 3}, "m": 2,
		"tr": {"p": {"s": true, "x": {"a": 0, "k": 12}, "y": {"a": 0, "k": 34}}}
	}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	var c converter
	shape, ok := c.convShape(&s)
	if !ok || shape.Kind != ShapeRepeater || shape.Repeater.Animated != nil {
		t.Fatalf("expected a fixed repeater, got %+v", shape)
	}
	r := shape.Repeater.Fixed
	if r.Position != kurbo.NewPoint(12, 34) || r.Scale != kurbo.NewVec2(100, 100) || r.Composite != RepeaterAbove {
		t.Errorf("got %+v", r)
	}
}

// TestUnsupportedIsAnError checks that a feature where velato panics is an
// error that wraps okage.ErrLottieUnsupported (D41).
func TestUnsupportedIsAnError(t *testing.T) {
	_, err := Parse([]byte(`{
		"fr": 60, "ip": 0, "op": 60, "w": 10, "h": 10,
		"layers": [{"ty": 3, "ip": 0, "op": 60, "bm": 16, "ks": {"p": {"k": [0, 0]}}}]
	}`))
	if !errors.Is(err, okage.ErrLottieUnsupported) {
		t.Errorf("got %v", err)
	}
	_, err = Parse([]byte(`{"fr": 0, "ip": 0, "op": 60, "w": 10, "h": 10, "layers": []}`))
	if !errors.Is(err, okage.ErrLottieInvalid) {
		t.Errorf("got %v", err)
	}
	_, err = Parse([]byte(`{"fr": 60}`))
	if !errors.Is(err, okage.ErrLottieParse) {
		t.Errorf("got %v", err)
	}
}
