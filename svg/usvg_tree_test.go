// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"testing"

	"github.com/xo/okage/path"
)

// usvg src/tree/mod.rs has no tests. These tests check the port of its
// functions that the parser calls, on trees that the tests build by hand.

func testRect(t *testing.T, x, y, w, h float32) Rect {
	t.Helper()
	r, ok := path.RectFromXYWH(x, y, w, h)
	if !ok {
		t.Fatalf("RectFromXYWH(%v, %v, %v, %v) failed", x, y, w, h)
	}
	return r
}

func testNonZeroRect(t *testing.T, x, y, w, h float32) NonZeroRect {
	t.Helper()
	r, ok := path.NonZeroRectFromXYWH(x, y, w, h)
	if !ok {
		t.Fatalf("NonZeroRectFromXYWH(%v, %v, %v, %v) failed", x, y, w, h)
	}
	return r
}

func testPath(t *testing.T, id string, rect Rect, fill *Fill, stroke *Stroke, ts Transform) *Path {
	t.Helper()
	return newPath(id, true, fill, stroke, PaintOrderFillAndStroke, DefaultShapeRendering(), path.PathFromRect(rect), ts)
}

func testStroke(paint Paint, width float32) *Stroke {
	w, _ := path.NewNonZeroPositiveF32(width)
	return &Stroke{
		paint:      paint,
		miterlimit: DefaultStrokeMiterlimit(),
		opacity:    path.NormalizedF32One,
		width:      w,
		linecap:    LineCapButt,
		linejoin:   LineJoinMiter,
		contextElement: &contextElement{
			kind: contextElementPathNode,
		},
	}
}

func TestNonEmptyString(t *testing.T) {
	if _, ok := newNonEmptyString(" \t\n"); ok {
		t.Error("newNonEmptyString accepted a blank string")
	}
	s, ok := newNonEmptyString(" a ")
	if !ok || s.get() != " a " || s.take() != " a " {
		t.Errorf("newNonEmptyString(\" a \") = %q, %v", s, ok)
	}
}

func TestNonZeroF32(t *testing.T) {
	if _, ok := NewNonZeroF32(0.0); ok {
		t.Error("NewNonZeroF32(0) succeeded")
	}
	if _, ok := NewNonZeroF32(1e-45); ok {
		t.Error("NewNonZeroF32(1e-45) succeeded, but it is within 4 ULPs of zero")
	}
	n, ok := NewNonZeroF32(-2.5)
	if !ok || n.Get() != -2.5 {
		t.Errorf("NewNonZeroF32(-2.5) = %v, %v", n, ok)
	}
}

func TestEnumStrings(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{unitsUserSpaceOnUse.String(), "userSpaceOnUse"},
		{unitsObjectBoundingBox.String(), "objectBoundingBox"},
		{visibilityVisible.String(), "Visible"},
		{visibilityHidden.String(), "Hidden"},
		{visibilityCollapse.String(), "Collapse"},
		{BlendModeColorDodge.String(), "color-dodge"},
		{BlendModePlusLighter.String(), "plus-lighter"},
		{ImageRenderingHighQuality.String(), "high-quality"},
		{MaskTypeAlpha.String(), "Alpha"},
		{BlendMode(200).String(), "BlendMode(?)"},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Errorf("got %q, want %q", test.got, test.want)
		}
	}
}

func TestParseRendering(t *testing.T) {
	for r := ShapeRenderingOptimizeSpeed; r <= ShapeRenderingGeometricPrecision; r++ {
		if got, ok := ParseShapeRendering(r.String()); !ok || got != r {
			t.Errorf("ParseShapeRendering(%q) = %v, %v", r.String(), got, ok)
		}
	}
	for r := TextRenderingOptimizeSpeed; r <= TextRenderingGeometricPrecision; r++ {
		if got, ok := ParseTextRendering(r.String()); !ok || got != r {
			t.Errorf("ParseTextRendering(%q) = %v, %v", r.String(), got, ok)
		}
	}
	for r := ImageRenderingOptimizeQuality; r <= ImageRenderingPixelated; r++ {
		if got, ok := ParseImageRendering(r.String()); !ok || got != r {
			t.Errorf("ParseImageRendering(%q) = %v, %v", r.String(), got, ok)
		}
	}
	if _, ok := ParseShapeRendering("auto"); ok {
		t.Error("ParseShapeRendering(auto) succeeded")
	}
	if _, ok := ParseTextRendering("auto"); ok {
		t.Error("ParseTextRendering(auto) succeeded")
	}
	if _, ok := ParseImageRendering("auto"); ok {
		t.Error("ParseImageRendering(auto) succeeded")
	}
	if ShapeRenderingCrispEdges.UseShapeAntialiasing() || !DefaultShapeRendering().UseShapeAntialiasing() {
		t.Error("UseShapeAntialiasing is wrong")
	}
	if DefaultTextRendering() != TextRenderingOptimizeLegibility {
		t.Error("DefaultTextRendering is wrong")
	}
}

func TestStrokeMiterlimit(t *testing.T) {
	if got := NewStrokeMiterlimit(0.5).Get(); got != 1.0 {
		t.Errorf("NewStrokeMiterlimit(0.5) = %v, want 1", got)
	}
	if got := DefaultStrokeMiterlimit().Get(); got != 4.0 {
		t.Errorf("DefaultStrokeMiterlimit() = %v, want 4", got)
	}
	a := NewStrokeMiterlimit(4.0)
	b := StrokeMiterlimit(math.Float32frombits(math.Float32bits(4.0) + 3))
	if !a.Eq(b) || a.Eq(NewStrokeMiterlimit(5.0)) {
		t.Error("StrokeMiterlimit.Eq is wrong")
	}
}

func TestStrokeToTinySkia(t *testing.T) {
	s := testStroke(ColorBlack(), 2.0)
	s.linecap = LineCapSquare
	s.linejoin = LineJoinMiterClip
	ts := s.ToTinySkia()
	if ts.Width != 2.0 || ts.MiterLimit != 4.0 || ts.LineCap != path.LineCapSquare ||
		ts.LineJoin != path.LineJoinMiterClip || ts.Dash != nil {
		t.Errorf("ToTinySkia() = %+v", ts)
	}
	if _, ok := s.Dasharray(); ok {
		t.Error("Dasharray() reported a dash array")
	}
	s.dasharray = []float32{1, 2}
	s.dashoffset = 0.5
	if ts := s.ToTinySkia(); ts.Dash == nil {
		t.Error("ToTinySkia() lost the dash array")
	}
	s.dasharray = []float32{1}
	if ts := s.ToTinySkia(); ts.Dash != nil {
		t.Error("ToTinySkia() made a dash of an odd array")
	}
}

func TestPaintEquality(t *testing.T) {
	lg1 := &LinearGradient{}
	lg2 := &LinearGradient{}
	tests := []struct {
		a, b Paint
		want bool
	}{
		{ColorBlack(), NewColorRGB(0, 0, 0), true},
		{ColorBlack(), ColorWhite(), false},
		{lg1, lg1, true},
		{lg1, lg2, false},
		{lg1, ColorBlack(), false},
	}
	for i, test := range tests {
		if got := test.a == test.b; got != test.want {
			t.Errorf("%d: got %v, want %v", i, got, test.want)
		}
	}
	if f := DefaultFill(); f.Paint() != Paint(ColorBlack()) || f.Opacity() != path.NormalizedF32One || f.Rule() != FillRuleNonZero {
		t.Errorf("DefaultFill() = %+v", f)
	}
}

func TestNewPath(t *testing.T) {
	r := testRect(t, 10, 20, 30, 40)
	p := testPath(t, "p", r, nil, nil, path.TransformFromTranslate(5, 5))
	if bbox, sbbox := mustRect(p.BoundingBox()), mustRect(p.StrokeBoundingBox()); bbox != r || sbbox != r {
		t.Errorf("bounding boxes = %v, %v, want %v", bbox, sbbox, r)
	}
	if want, abs, sabs := testRect(t, 15, 25, 30, 40), mustRect(p.AbsBoundingBox()), mustRect(p.AbsStrokeBoundingBox()); abs != want || sabs != want {
		t.Errorf("abs bounding boxes = %v, %v, want %v", abs, sabs, want)
	}

	// A stroke grows the stroke bounding box by half of its width.
	p = testPath(t, "p", r, nil, testStroke(ColorBlack(), 2.0), path.DefaultTransform())
	if want, got := testRect(t, 9, 19, 32, 42), mustRect(p.StrokeBoundingBox()); got != want {
		t.Errorf("StrokeBoundingBox() = %v, want %v", got, want)
	}

	// A skew transforms the path, not its bounding box.
	p = testPath(t, "p", r, nil, nil, path.TransformFromSkew(1, 0))
	if want, got := testRect(t, 30, 20, 70, 40), mustRect(p.AbsBoundingBox()); got != want {
		t.Errorf("AbsBoundingBox() = %v, want %v", got, want)
	}

	if p := newSimplePath(path.PathFromRect(r)); p.data == nil {
		t.Error("newSimplePath has no data")
	}
}

func testTree(t *testing.T) (*Tree, *Path, *Group) {
	t.Helper()
	lg := &LinearGradient{}
	lg.id = "lg"
	lg.units = unitsObjectBoundingBox
	pattern := &Pattern{
		id:           "patt",
		units:        unitsUserSpaceOnUse,
		contentUnits: unitsUserSpaceOnUse,
		rect:         testNonZeroRect(t, 0, 0, 10, 10),
		viewBox:      &viewBox{rect: testNonZeroRect(t, 0, 0, 10, 10)},
		root:         emptyGroupValue(),
	}
	pattern.root.children = []Node{testPath(t, "inpatt", testRect(t, 0, 0, 5, 5), &Fill{paint: lg}, nil, path.DefaultTransform())}

	clip := emptyClipPath("clip")
	clip.root.children = []Node{testPath(t, "inclip", testRect(t, 0, 0, 50, 50), nil, nil, path.DefaultTransform())}

	p1 := testPath(t, "p1", testRect(t, 10, 10, 100, 100), &Fill{paint: lg}, testStroke(pattern, 2), path.DefaultTransform())
	g := emptyGroup()
	g.id = "g"
	g.clipPath = clip
	g.isContextElement = true
	p1.fill.contextElement = &contextElement{kind: contextElementUseNode}
	g.children = []Node{p1}

	tree := &Tree{root: emptyGroupValue()}
	tree.root.children = []Node{g}
	tree.size, _ = path.SizeFromWH(200, 200)
	return tree, p1, g
}

func TestGroupBoundingBoxes(t *testing.T) {
	_, _, g := testTree(t)
	if want, got := testRect(t, 10, 10, 100, 100), mustRect(g.BoundingBox()); got != want {
		t.Errorf("BoundingBox() = %v, want %v", got, want)
	}
	if want, got := testRect(t, 9, 9, 102, 102), mustRect(g.StrokeBoundingBox()); got != want {
		t.Errorf("StrokeBoundingBox() = %v, want %v", got, want)
	}
	// The clip path cuts the layer bounding box.
	layer, absLayer := mustNonZeroRect(g.LayerBoundingBox()), mustNonZeroRect(g.AbsLayerBoundingBox())
	if want := testNonZeroRect(t, 9, 9, 41, 41); layer != want || absLayer != want {
		t.Errorf("LayerBoundingBox() = %v, %v, want %v", layer, absLayer, want)
	}
	if r, ok := NodeAbsLayerBoundingBox(g); !ok || r != absLayer {
		t.Errorf("NodeAbsLayerBoundingBox() = %v, %v", r, ok)
	}
	if !g.ShouldIsolate() || !g.HasChildren() {
		t.Error("ShouldIsolate or HasChildren is wrong")
	}

	// A filter region takes the place of the layer bounding box.
	_, _, g = testTree(t)
	g.filters = []*Filter{{id: "f", rect: testNonZeroRect(t, 0, 0, 300, 300)}}
	if want, got := testNonZeroRect(t, 0, 0, 300, 300), mustNonZeroRect(g.LayerBoundingBox()); got != want {
		t.Errorf("LayerBoundingBox() = %v, want %v", got, want)
	}

	// An empty group has no bounding boxes.
	empty := emptyGroup()
	if _, ok := empty.BoundingBox(); ok {
		t.Error("BoundingBox of an empty group is set")
	}
	if _, ok := empty.LayerBoundingBox(); ok {
		t.Error("LayerBoundingBox of an empty group is set")
	}
	if _, ok := empty.FiltersBoundingBox(); ok {
		t.Error("FiltersBoundingBox of a group with no filters succeeded")
	}
}

func TestTreeCollect(t *testing.T) {
	tree, p1, g := testTree(t)
	tree.collectPaintServers()
	if len(tree.LinearGradients()) != 1 || len(tree.Patterns()) != 1 || len(tree.RadialGradients()) != 0 {
		t.Errorf("collectPaintServers: %d linear, %d radial, %d patterns",
			len(tree.LinearGradients()), len(tree.RadialGradients()), len(tree.Patterns()))
	}
	var clipPaths []*ClipPath
	tree.root.collectClipPaths(&clipPaths)
	if len(clipPaths) != 1 || clipPaths[0] != g.clipPath {
		t.Errorf("collectClipPaths = %v", clipPaths)
	}
	var masks []*Mask
	tree.root.collectMasks(&masks)
	if len(masks) != 0 {
		t.Errorf("collectMasks = %v", masks)
	}
	g.filters = []*Filter{{id: "f"}}
	var filters []*Filter
	tree.root.collectFilters(&filters)
	if len(filters) != 1 {
		t.Errorf("collectFilters = %v", filters)
	}
	tree.filters = filters
	if !tree.HasDefsNodes() {
		t.Error("HasDefsNodes() = false")
	}

	if n, ok := tree.NodeByID("p1"); !ok || n != Node(p1) {
		t.Errorf("NodeByID(p1) = %v, %v", n, ok)
	}
	if _, ok := tree.NodeByID(""); ok {
		t.Error("NodeByID of an empty ID succeeded")
	}
	if _, ok := tree.NodeByID("inclip"); ok {
		t.Error("NodeByID found a node of a clip path")
	}
	if tree.HasTextNodes() {
		t.Error("HasTextNodes() = true")
	}
	text := &Text{id: "t", flattened: emptyGroup()}
	g.clipPath.root.children = append(g.clipPath.root.children, text)
	if !tree.HasTextNodes() {
		t.Error("HasTextNodes() = false for a text in a clip path")
	}

	var subroots []*Group
	p1.Subroots(func(sub *Group) { subroots = append(subroots, sub) })
	if len(subroots) != 1 || subroots[0].children[0].ID() != "inpatt" {
		t.Errorf("Subroots of the path = %v", subroots)
	}
}

func TestImage(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	opt := newOptions()
	size, ok := imageKindActualSize(ImageKindPNG(buf.Bytes()), opt)
	if !ok || size.Width() != 3 || size.Height() != 2 {
		t.Errorf("imageKindActualSize(PNG) = %v, %v", size, ok)
	}
	if _, ok := imageKindActualSize(ImageKindJPEG("not a jpeg"), opt); ok {
		t.Error("imageKindActualSize of bad data succeeded")
	}

	tree, _, _ := testTree(t)
	if size, ok := imageKindActualSize(ImageKindSVG{Tree: tree}, opt); !ok || size != tree.size {
		t.Errorf("imageKindActualSize(SVG) = %v, %v", size, ok)
	}

	img := &Image{
		id:             "img",
		size:           size,
		kind:           ImageKindSVG{Tree: tree},
		absBoundingBox: testNonZeroRect(t, 1, 2, 3, 4),
	}
	if r, ok := NodeStrokeBoundingBox(img); img.BoundingBox() != testRect(t, 0, 0, 3, 2) || !ok || r != img.BoundingBox() {
		t.Errorf("BoundingBox() = %v", img.BoundingBox())
	}
	if r, ok := NodeAbsStrokeBoundingBox(img); !ok || r != testRect(t, 1, 2, 3, 4) {
		t.Errorf("NodeAbsStrokeBoundingBox() = %v", r)
	}
	n := 0
	img.Subroots(func(*Group) { n++ })
	if n != 1 {
		t.Errorf("Subroots called f %d times, want 1", n)
	}
	if got := ImageKindPNG(nil).String(); got != "ImageKind::PNG(..)" {
		t.Errorf("String() = %q", got)
	}
}

// assertNoBoundingBoxes is assert_no_bounding_boxes in the tests of usvg
// src/tree/mod.rs.
func assertNoBoundingBoxes(t *testing.T, group *Group) {
	t.Helper()
	for _, set := range []bool{
		group.boundingBox.done.Load(),
		group.absBoundingBox.done.Load(),
		group.strokeBoundingBox.done.Load(),
		group.absStrokeBoundingBox.done.Load(),
		group.layerBoundingBox.done.Load(),
		group.absLayerBoundingBox.done.Load(),
	} {
		if set {
			t.Errorf("group %q has a bounding box", group.id)
		}
	}

	for _, child := range group.Children() {
		switch n := child.(type) {
		case *Group:
			assertNoBoundingBoxes(t, n)
		case *Path:
			if n.boundingBox.done.Load() ||
				n.absBoundingBox.done.Load() ||
				n.strokeBoundingBox.done.Load() ||
				n.absStrokeBoundingBox.done.Load() {
				t.Errorf("path %q has a bounding box", n.id)
			}
		}
	}
}

// TestPathsInGroupsDontCalculateBoundingBoxes is
// paths_in_groups_dont_calculate_bounding_boxes in the tests of usvg
// src/tree/mod.rs.
func TestPathsInGroupsDontCalculateBoundingBoxes(t *testing.T) {
	tree := mustFromStr(t, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200">
                <g transform="translate(10 20)" opacity="0.5">
                    <path d="M 20 20 C 80 0 120 200 180 180" fill="green" stroke="black" stroke-width="10"/>
                    <g transform="rotate(30)">
                        <rect x="20" y="20" width="100" height="100" stroke="blue"/>
                    </g>
                </g>
            </svg>`)
	assertNoBoundingBoxes(t, tree.Root())
}

// TestUserSpaceClipPathsDontCalculateBoundingBoxes is
// user_space_clip_paths_dont_calculate_bounding_boxes in the tests of usvg
// src/tree/mod.rs.
func TestUserSpaceClipPathsDontCalculateBoundingBoxes(t *testing.T) {
	tree := mustFromStr(t, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200">
                <clipPath id="clip">
                    <rect x="20" y="20" width="160" height="160"/>
                </clipPath>
                <g clip-path="url(#clip)">
                    <path d="M 20 20 C 80 0 120 200 180 180" fill="green" stroke="black" stroke-width="10"/>
                </g>
            </svg>`)
	assertNoBoundingBoxes(t, tree.Root())
}

// TestUseElementsDontCalculateBoundingBoxes is
// use_elements_dont_calculate_bounding_boxes in the tests of usvg
// src/tree/mod.rs.
func TestUseElementsDontCalculateBoundingBoxes(t *testing.T) {
	tree := mustFromStr(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="200" height="200">
                <defs>
                    <path id="shape" d="M 20 20 C 80 0 120 200 180 180" stroke="black" stroke-width="10"/>
                </defs>
                <use xlink:href="#shape" fill="green"/>
                <use xlink:href="#shape" x="10" y="10" fill="blue"/>
            </svg>`)
	assertNoBoundingBoxes(t, tree.Root())
}

// TestUserSpacePaintServersDontCalculateBoundingBoxes is
// user_space_paint_servers_dont_calculate_bounding_boxes in the tests of
// usvg src/tree/mod.rs.
func TestUserSpacePaintServersDontCalculateBoundingBoxes(t *testing.T) {
	tree := mustFromStr(t, `<svg xmlns="http://www.w3.org/2000/svg" width="200" height="200">
                <linearGradient id="lg" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="200" y2="0">
                    <stop offset="0" stop-color="white"/>
                    <stop offset="1" stop-color="black"/>
                </linearGradient>
                <path d="M 20 20 C 80 0 120 200 180 180" fill="url(#lg)" stroke="url(#lg)" stroke-width="10"/>
            </svg>`)
	assertNoBoundingBoxes(t, tree.Root())
}
