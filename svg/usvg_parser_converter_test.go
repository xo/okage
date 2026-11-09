// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"testing"

	"github.com/xo/okage/path"
)

// usvg src/parser/converter.rs has no unit tests. These tests check the
// generation of ids and the conversion of lengths.

func TestGenID(t *testing.T) {
	c := newConverterCache()
	c.allIDs[stringHash("linearGradient1")] = struct{}{}
	c.allIDs[stringHash("mask2")] = struct{}{}
	for _, tc := range []struct {
		gen  func() nonEmptyString
		want string
	}{
		{c.genLinearGradientID, "linearGradient2"},
		{c.genRadialGradientID, "radialGradient1"},
		{c.genPatternID, "pattern1"},
		{c.genClipPathID, "clipPath1"},
		{c.genMaskID, "mask1"},
		{c.genMaskID, "mask3"},
		{c.genFilterID, "filter1"},
		{c.genImageID, "image1"},
	} {
		if got := tc.gen().get(); got != tc.want {
			t.Errorf("got id %q, want %q", got, tc.want)
		}
	}
}

func TestTryConvertLength(t *testing.T) {
	doc := mustParseTree(t, `<svg xmlns="http://www.w3.org/2000/svg"><rect id="r" x="50%" y="2in" font-size="10" width="2em"/></svg>`)
	r, _ := doc.elementByID("r")
	state := &converterState{
		viewBox: mustNonZeroRect(path.NonZeroRectFromXYWH(0, 0, 200, 100)),
		opt:     newOptions(),
	}
	for _, c := range []struct {
		aid  AId
		u    units
		want float32
	}{
		{AIdX, unitsUserSpaceOnUse, 100},
		{AIdX, unitsObjectBoundingBox, 0.5},
		{AIdY, unitsUserSpaceOnUse, 192},
		{AIdWidth, unitsUserSpaceOnUse, 20},
	} {
		if got, ok := r.tryConvertLength(c.aid, c.u, state); !ok || got != c.want {
			t.Errorf("tryConvertLength(%v, %v) = %v, %v, want %v", c.aid, c.u, got, ok, c.want)
		}
	}
	if _, ok := r.tryConvertLength(AIdHeight, unitsUserSpaceOnUse, state); ok {
		t.Error("tryConvertLength converted a missing attribute")
	}
}

func TestPaintUnits(t *testing.T) {
	pattern := &Pattern{units: unitsObjectBoundingBox, contentUnits: unitsObjectBoundingBox}
	if paintUnits(pattern) != unitsObjectBoundingBox || paintContentUnits(pattern) != unitsObjectBoundingBox {
		t.Error("the units of a pattern are wrong")
	}
	if paintUnits(ColorBlack()) != unitsUserSpaceOnUse || paintContentUnits(ColorBlack()) != unitsUserSpaceOnUse {
		t.Error("the units of a color are wrong")
	}
}
