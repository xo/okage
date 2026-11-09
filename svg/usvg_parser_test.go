// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"compress/gzip"
	"errors"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// This file holds two kinds of tests. The tests from TestFromStr to
// TestLogNone check the port against the text that usvg 0.49.0 writes, as the
// Rust oracle of the test module gave it on 2026-11-09. The tests after them
// port the tests of usvg tests/parser.rs, in its order.

const parserTestSVG = `<svg viewBox="0 0 200 100" width="400" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
  <style>.c { fill: blue; stroke: red !important }</style>
  <g id="g1" opacity="0.5" transform="rotate(30) translate(0.1 -0.0000001)">
    <rect class="c" x="10" y="10" width="50" height="20" rx="5" stroke="green" stroke-width="2.5" stroke-dasharray="1 2 3"/>
    <circle cx="100" cy="50" r="1e-3" fill="currentColor" color="#80808080"/>
    <polyline points="1,2 3,4 5,6" fill="none" stroke="black" paint-order="stroke"/>
  </g>
  <use xlink:href="#g1" x="5" y="5"/>
  <switch><rect systemLanguage="de" width="5" height="5"/><ellipse rx="3" ry="4" style="fill:#123;mix-blend-mode:multiply"/></switch>
  <text>Hello</text>
  <path d="M 10 10 L 20 20 Q 30 30 40 10 C 1.5 2.5 3.5 4.5 5 6 Z" fill="inherit" font-size="larger" stroke-width="2em" stroke="black"/>
</svg>
`

const parserTestWant = `<svg width="400" height="200" xmlns="http://www.w3.org/2000/svg">
    <g transform="matrix(2 0 0 2 0 0)">
        <g id="g1" opacity="0.5" transform="matrix(0.8660254 0.5 -0.5 0.8660254 0.08660259 0.04999992)">
            <path fill="#0000ff" stroke="#ff0000" stroke-width="2.5" stroke-dasharray="1 2 3 1 2 3" d="M 15 10 L 55 10 C 57.761425 10 60 12.238575 60 15 L 60 25 C 60 27.761423 57.761425 30 55 30 L 15 30 C 12.238575 30 10 27.761423 10 25 L 10 15 C 10 12.238575 12.238575 10 15 10 Z"/>
            <path fill="#808080" fill-opacity="0.5019608" stroke="none" d="M 100.00101 50 C 100.00101 50.000553 100.00055 50.001 100 50.001 C 99.99945 50.001 99.99899 50.000553 99.99899 50 C 99.99899 49.999447 99.99945 49.999 100 49.999 C 100.00055 49.999 100.00101 49.999447 100.00101 50 Z"/>
            <path fill="none" stroke="#000000" paint-order="stroke" d="M 1 2 L 3 4 L 5 6"/>
        </g>
        <g transform="matrix(1 0 0 1 5 5)">
            <g opacity="0.5" transform="matrix(0.8660254 0.5 -0.5 0.8660254 0.08660259 0.04999992)">
                <path fill="#0000ff" stroke="#ff0000" stroke-width="2.5" stroke-dasharray="1 2 3 1 2 3" d="M 15 10 L 55 10 C 57.761425 10 60 12.238575 60 15 L 60 25 C 60 27.761423 57.761425 30 55 30 L 15 30 C 12.238575 30 10 27.761423 10 25 L 10 15 C 10 12.238575 12.238575 10 15 10 Z"/>
                <path fill="#808080" fill-opacity="0.5019608" stroke="none" d="M 100.00101 50 C 100.00101 50.000553 100.00055 50.001 100 50.001 C 99.99945 50.001 99.99899 50.000553 99.99899 50 C 99.99899 49.999447 99.99945 49.999 100 49.999 C 100.00055 49.999 100.00101 49.999447 100.00101 50 Z"/>
                <path fill="none" stroke="#000000" paint-order="stroke" d="M 1 2 L 3 4 L 5 6"/>
            </g>
        </g>
        <g style="mix-blend-mode:multiply;isolation:auto">
            <path fill="#112233" stroke="none" d="M 3 0 C 3 2.209139 1.6568543 4 0 4 C -1.6568543 4 -3 2.209139 -3 0 C -3 -2.209139 -1.6568543 -4 -0 -4 C 1.6568543 -4 3 -2.209139 3 -0 Z"/>
        </g>
        <path fill="#000000" stroke="#000000" stroke-width="28.800001" d="M 10 10 L 20 20 Q 30 30 40 10 C 1.5 2.5 3.5 4.5 5 6 Z"/>
    </g>
</svg>
`

const parserTestStyleSVG = `<svg width="10" height="10" xmlns="http://www.w3.org/2000/svg"><rect id="r" width="5" height="5" fill="red" fill-rule="evenodd" stroke="blue" stroke-linejoin="round" stroke-linecap="square" stroke-miterlimit="2" stroke-opacity="50%" shape-rendering="crispEdges" visibility="hidden"/></svg>`

const parserTestStyleWant = `<svg width="10" height="10" xmlns="http://www.w3.org/2000/svg">
    <path id="r" fill="#ff0000" fill-rule="evenodd" stroke="#0000ff" stroke-opacity="0.5" stroke-miterlimit="2" stroke-linecap="square" stroke-linejoin="round" visibility="hidden" shape-rendering="crispEdges" d="M 0 0 L 5 0 L 5 5 L 0 5 Z"/>
</svg>
`

func TestFromStr(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{parserTestSVG, parserTestWant},
		{parserTestStyleSVG, parserTestStyleWant},
	} {
		tree, err := FromStr(c.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := tree.ToString(); got != c.want {
			t.Errorf("got\n%s\nwant\n%s", got, c.want)
		}
	}
}

func TestFromDataSVGZ(t *testing.T) {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write([]byte(parserTestSVG)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	tree, err := FromData(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.ToString(); got != parserTestWant {
		t.Errorf("got\n%s", got)
	}
	tree, err = FromDataNested([]byte(parserTestSVG))
	if err != nil {
		t.Fatal(err)
	}
	if got := tree.ToString(); got != parserTestWant {
		t.Errorf("nested: got\n%s", got)
	}
}

func TestFromDataErrors(t *testing.T) {
	for _, c := range []struct {
		data string
		want error
	}{
		{`<svg width="0" height="10" xmlns="http://www.w3.org/2000/svg"/>`, okage.ErrSVGInvalidSize},
		{`<html/>`, okage.ErrSVGParsingFailed},
		{`<html/>`, okage.ErrXMLNoRootNode},
		{`<svg`, okage.ErrSVGParsingFailed},
		{"\x1f\x8bnot gzip", okage.ErrSVGMalformedGZip},
		{"<svg>\xff</svg>", okage.ErrSVGNotAnUTF8Str},
	} {
		_, err := FromData([]byte(c.data))
		if !errors.Is(err, c.want) {
			t.Errorf("FromData(%q) = %v, want %v", c.data, err, c.want)
		}
	}
}

func TestF32Bound(t *testing.T) {
	for _, c := range []struct{ v, want float32 }{
		{-1, 0}, {0.5, 0.5}, {2, 1}, {nan32(), 0},
	} {
		if got := f32Bound(0, c.v, 1); got != c.want {
			t.Errorf("f32Bound(0, %v, 1) = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestLogNone(t *testing.T) {
	called := false
	if v, ok := logNone(3, true, func() { called = true }); v != 3 || !ok || called {
		t.Error("logNone called f for Some")
	}
	if _, ok := logNone(0, false, func() { called = true }); ok || !called {
		t.Error("logNone did not call f for None")
	}
}

// mustFromStr parses svg with opts and fails the test on an error.
func mustFromStr(t *testing.T, svg string, opts ...Option) *Tree {
	t.Helper()
	tree, err := FromStr(svg, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// mustSizeWH is usvg::Size::from_wh(w, h).unwrap().
func mustSizeWH(w, h float32) Size {
	return mustSize(path.SizeFromWH(w, h))
}

// TestGradientStopOffsetOverflowingF32 is gradient_stop_offset_overflowing_f32
// in usvg tests/parser.rs.
func TestGradientStopOffsetOverflowingF32(t *testing.T) {
	// `4e38` overflows f32 to infinity; parsing must not panic.
	svg := "<svg xmlns='http://www.w3.org/2000/svg'>" +
		"<defs><linearGradient id='g'><stop offset='4e38'/></linearGradient></defs>" +
		"<rect width='1' height='1' fill='url(#g)'/>" +
		"</svg>"
	if _, err := FromStr(svg); err != nil {
		t.Fatal(err)
	}
}

// TestClippathWithInvalidChild is clippath_with_invalid_child in usvg
// tests/parser.rs.
func TestClippathWithInvalidChild(t *testing.T) {
	svg := `
    <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1 1'>
        <clipPath id='clip1'>
            <rect/>
        </clipPath>
        <rect clip-path='url(#clip1)' width='10' height='10'/>
    </svg>
    `

	tree := mustFromStr(t, svg)
	// clipPath is invalid and should be removed together with rect.
	if tree.Root().HasChildren() {
		t.Error("the root has children")
	}
}

// stylesheetInjectionSVG is the SVG of the stylesheet tests in usvg
// tests/parser.rs.
const stylesheetInjectionSVG = `<svg id='svg1' viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'>
    <style>
        #rect4 {
            fill: green
        }
    </style>
    <rect id='rect1' x='20' y='20' width='60' height='60'/>
    <rect id='rect2' x='120' y='20' width='60' height='60' fill='green'/>
    <rect id='rect3' x='20' y='120' width='60' height='60' style='fill: green'/>
    <rect id='rect4' x='120' y='120' width='60' height='60'/>
    <rect id='rect5' x='70' y='70' width='60' height='60' style='fill: green !important'/>
</svg>
`

// checkFillColors checks that the children of the root of tree are paths
// with the fill colors of want.
func checkFillColors(t *testing.T, tree *Tree, want []Color) {
	t.Helper()
	children := tree.Root().Children()
	for i, w := range want {
		p, ok := children[i].(*Path)
		if !ok {
			t.Fatalf("child %d is %T, want *Path", i, children[i])
		}
		fill, ok := p.Fill()
		if !ok {
			t.Fatalf("child %d has no fill", i)
		}
		if got := fill.Paint(); got != Paint(w) {
			t.Errorf("child %d: paint = %v, want %v", i, got, w)
		}
	}
}

// TestStylesheetInjection is stylesheet_injection in usvg tests/parser.rs.
func TestStylesheetInjection(t *testing.T) {
	tree := mustFromStr(t, stylesheetInjectionSVG, WithStyleSheet("rect { fill: red }"))

	red, green := NewColorRGB(255, 0, 0), NewColorRGB(0, 128, 0)
	// Only the rects with no CSS attributes should be overridden.
	checkFillColors(t, tree, []Color{red, red, green, green, green})
}

// TestStylesheetInjectionWithImportant is
// stylesheet_injection_with_important in usvg tests/parser.rs.
func TestStylesheetInjectionWithImportant(t *testing.T) {
	tree := mustFromStr(t, stylesheetInjectionSVG, WithStyleSheet("rect { fill: red !important }"))

	red := NewColorRGB(255, 0, 0)
	// All rects should be overridden, since we use `important`.
	checkFillColors(t, tree, []Color{red, red, red, red, red})
}

// TestSimplifyPaths is simplify_paths in usvg tests/parser.rs.
func TestSimplifyPaths(t *testing.T) {
	svg := `
    <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 1 1'>
        <path d='M 10 20 L 10 30 Z Z Z'/>
    </svg>
    `

	tree := mustFromStr(t, svg)
	p, ok := tree.Root().Children()[0].(*Path)
	if !ok {
		t.Fatal("the first child is not a path")
	}
	// Make sure we have MLZ and not MLZZZ
	if got := len(p.Data().Verbs()); got != 3 {
		t.Errorf("len(verbs) = %d, want 3", got)
	}
}

// TestSizeDetection is size_detection_1 to size_detection_9 in usvg
// tests/parser.rs.
func TestSizeDetection(t *testing.T) {
	for i, c := range []struct {
		svg  string
		w, h float32
	}{
		{"<svg viewBox='0 0 10 20' xmlns='http://www.w3.org/2000/svg'/>", 10, 20},
		{"<svg width='30' height='40' viewBox='0 0 10 20' xmlns='http://www.w3.org/2000/svg'/>", 30, 40},
		{"<svg width='50%' height='100%' viewBox='0 0 10 20' xmlns='http://www.w3.org/2000/svg'/>", 5, 20},
		{`
    <svg xmlns='http://www.w3.org/2000/svg'>
        <circle cx='18' cy='18' r='18'/>
    </svg>
    `, 36, 36},
		{"<svg xmlns='http://www.w3.org/2000/svg'/>", 100, 100},
		{"<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 10 10' width='30'/>", 30, 30},
		{"<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 10 20' width='30'/>", 30, 60},
		{"<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 10 10' height='30'/>", 30, 30},
		{"<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 10 20' height='30'/>", 15, 30},
	} {
		tree := mustFromStr(t, c.svg)
		if got, want := tree.Size(), mustSizeWH(c.w, c.h); got != want {
			t.Errorf("size_detection_%d: size = %v, want %v", i+1, got, want)
		}
	}
}

// TestInvalidSize1 is invalid_size_1 in usvg tests/parser.rs.
func TestInvalidSize1(t *testing.T) {
	svg := "<svg width='0' height='0' viewBox='0 0 10 20' xmlns='http://www.w3.org/2000/svg'/>"
	if _, err := FromStr(svg); err == nil {
		t.Error("FromStr returned no error")
	}
}

// The test tree_is_send_and_sync of usvg tests/parser.rs checks the traits
// Send and Sync of Rust, which Go does not have. It has no port.

// checkGroup checks that node is a group with the id and the absolute
// transform, and returns it.
func checkGroup(t *testing.T, node Node, id string, abs Transform) *Group {
	t.Helper()
	g, ok := node.(*Group)
	if !ok {
		t.Fatalf("node is %T, want *Group", node)
	}
	if g.ID() != id {
		t.Errorf("id = %q, want %q", g.ID(), id)
	}
	if g.AbsTransform() != abs {
		t.Errorf("group %q: abs transform = %v, want %v", id, g.AbsTransform(), abs)
	}
	return g
}

// checkPath checks that node is a path with the absolute transform.
func checkPath(t *testing.T, node Node, abs Transform) *Path {
	t.Helper()
	p, ok := node.(*Path)
	if !ok {
		t.Fatalf("node is %T, want *Path", node)
	}
	if p.AbsTransform() != abs {
		t.Errorf("path: abs transform = %v, want %v", p.AbsTransform(), abs)
	}
	return p
}

// TestPathTransform is path_transform in usvg tests/parser.rs.
func TestPathTransform(t *testing.T) {
	svg := `
    <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'>
        <path transform='translate(10)' d='M 0 0 L 10 10'/>
    </svg>
    `

	tree := mustFromStr(t, svg)
	if got := len(tree.Root().Children()); got != 1 {
		t.Fatalf("len(children) = %d, want 1", got)
	}

	group := checkGroup(t, tree.Root().Children()[0], "", path.TransformFromTranslate(10.0, 0.0))
	checkPath(t, group.Children()[0], path.TransformFromTranslate(10.0, 0.0))
}

// TestPathTransformNested is path_transform_nested in usvg tests/parser.rs.
func TestPathTransformNested(t *testing.T) {
	svg := `
    <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'>
        <g transform='translate(20)'>
            <path transform='translate(10)' d='M 0 0 L 10 10'/>
        </g>
    </svg>
    `

	tree := mustFromStr(t, svg)
	if got := len(tree.Root().Children()); got != 1 {
		t.Fatalf("len(children) = %d, want 1", got)
	}

	group1 := checkGroup(t, tree.Root().Children()[0], "", path.TransformFromTranslate(20.0, 0.0))
	group2 := checkGroup(t, group1.Children()[0], "", path.TransformFromTranslate(30.0, 0.0))
	checkPath(t, group2.Children()[0], path.TransformFromTranslate(30.0, 0.0))
}

// TestPathTransformInSymbolNoClip is path_transform_in_symbol_no_clip in
// usvg tests/parser.rs.
func TestPathTransformInSymbolNoClip(t *testing.T) {
	svg := `
    <svg viewBox='0 0 100 100' xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink'>
        <defs>
            <symbol id='symbol1' overflow='visible'>
                <rect id='rect1' x='0' y='0' width='10' height='10'/>
            </symbol>
        </defs>
        <use id='use1' xlink:href='#symbol1' x='20'/>
    </svg>
    `

	// Will be parsed as:
	// <svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
	//     <g id="use1">
	//         <g transform="matrix(1 0 0 1 20 0)">
	//             <path fill="#000000" stroke="none" d="M 0 0 L 10 0 L 10 10 L 0 10 Z"/>
	//         </g>
	//     </g>
	// </svg>

	tree := mustFromStr(t, svg)

	group1 := checkGroup(t, tree.Root().Children()[0], "use1", path.DefaultTransform())
	group2 := checkGroup(t, group1.Children()[0], "", path.TransformFromTranslate(20.0, 0.0))
	checkPath(t, group2.Children()[0], path.TransformFromTranslate(20.0, 0.0))
}

// TestPathTransformInSymbolWithClip is path_transform_in_symbol_with_clip in
// usvg tests/parser.rs.
func TestPathTransformInSymbolWithClip(t *testing.T) {
	svg := `
    <svg viewBox='0 0 100 100' xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink'>
        <defs>
            <symbol id='symbol1' overflow='hidden'>
                <rect id='rect1' x='0' y='0' width='10' height='10'/>
            </symbol>
        </defs>
        <use id='use1' xlink:href='#symbol1' x='20'/>
    </svg>
    `

	// Will be parsed as:
	// <svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
	//     <defs>
	//         <clipPath id="clipPath1">
	//             <path fill="#000000" stroke="none" d="M 20 0 L 120 0 L 120 100 L 20 100 Z"/>
	//         </clipPath>
	//     </defs>
	//     <g id="use1" clip-path="url(#clipPath1)">
	//         <g>
	//             <g transform="matrix(1 0 0 1 20 0)">
	//                 <path fill="#000000" stroke="none" d="M 0 0 L 10 0 L 10 10 L 0 10 Z"/>
	//             </g>
	//         </g>
	//     </g>
	// </svg>

	tree := mustFromStr(t, svg)

	group1 := checkGroup(t, tree.Root().Children()[0], "use1", path.DefaultTransform())
	group2 := checkGroup(t, group1.Children()[0], "", path.DefaultTransform())
	group3 := checkGroup(t, group2.Children()[0], "", path.TransformFromTranslate(20.0, 0.0))
	checkPath(t, group3.Children()[0], path.TransformFromTranslate(20.0, 0.0))
}

// TestPathTransformInSvg is path_transform_in_svg in usvg tests/parser.rs.
func TestPathTransformInSvg(t *testing.T) {
	svg := `
    <svg viewBox='0 0 100 100' xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink'>
        <g id='g1' transform='translate(100 150)'>
            <svg id='svg1' width='100' height='50'>
                <rect id='rect1' width='10' height='10'/>
            </svg>
        </g>
    </svg>
    `

	// Will be parsed as:
	// <svg width="100" height="100" viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
	//     <defs>
	//         <clipPath id="clipPath1">
	//             <path fill="#000000" stroke="none" d="M 0 0 L 100 0 L 100 50 L 0 50 Z"/>
	//         </clipPath>
	//     </defs>
	//     <g id="g1" transform="matrix(1 0 0 1 100 150)">
	//         <g id="svg1" clip-path="url(#clipPath1)">
	//             <path id="rect1" fill="#000000" stroke="none" d="M 0 0 L 10 0 L 10 10 L 0 10 Z"/>
	//         </g>
	//     </g>
	// </svg>

	tree := mustFromStr(t, svg)

	group1 := checkGroup(t, tree.Root().Children()[0], "g1", path.TransformFromTranslate(100.0, 150.0))
	group2 := checkGroup(t, group1.Children()[0], "svg1", path.TransformFromTranslate(100.0, 150.0))
	checkPath(t, group2.Children()[0], path.TransformFromTranslate(100.0, 150.0))
}

// TestSvgWithoutXmlns is svg_without_xmlns in usvg tests/parser.rs.
func TestSvgWithoutXmlns(t *testing.T) {
	svg := `
    <svg viewBox='0 0 100 100'>
        <rect x='0' y='0' width='10' height='10'/>
    </svg>
    `

	tree := mustFromStr(t, svg)
	if got, want := tree.Size(), mustSizeWH(100.0, 100.0); got != want {
		t.Errorf("size = %v, want %v", got, want)
	}
}

// TestImageBBoxWithParentTransform is image_bbox_with_parent_transform in
// usvg tests/parser.rs.
func TestImageBBoxWithParentTransform(t *testing.T) {
	svg := `
    <svg viewBox='0 0 200 200' 
         xmlns='http://www.w3.org/2000/svg'
         xmlns:xlink='http://www.w3.org/1999/xlink'>
        <g transform='translate(25 25)'>
            <image id='image1' x='10' y='10' width='50' height='50' xlink:href='data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAABb0lEQVR4Xu3VUQ0AIAzEUOZfA87wAgkq+vGmoGlz2exz73IZAyNIpsUHEaTVQ5BYD0EEqRmI8fghgsQMxHAsRJCYgRiOhQgSMxDDsRBBYgZiOBYiSMxADMdCBIkZiOFYiCAxAzEcCxEkZiCGYyGCxAzEcCxEkJiBGI6FCBIzEMOxEEFiBmI4FiJIzEAMx0IEiRmI4ViIIDEDMRwLESRmIIZjIYLEDMRwLESQmIEYjoUIEjMQw7EQQWIGYjgWIkjMQAzHQgSJGYjhWIggMQMxHAsRJGYghmMhgsQMxHAsRJCYgRiOhQgSMxDDsRBBYgZiOBYiSMxADMdCBIkZiOFYiCAxAzEcCxEkZiCGYyGCxAzEcCxEkJiBGI6FCBIzEMOxEEFiBmI4FiJIzEAMx0IEiRmI4ViIIDEDMRwLESRmIIZjIYLEDMRwLESQmIEYjoUIEjMQw7EQQWIGYjgWIkjMQAzHQgSJGYjhWIggMQMxnAdKSlrwlejIDgAAAABJRU5ErkJggg=='/>
        </g>
    </svg>
    `

	tree := mustFromStr(t, svg)

	groupNode1, ok := tree.Root().Children()[0].(*Group)
	if !ok {
		t.Fatal("the first child is not a group")
	}
	groupNode2, ok := groupNode1.Children()[0].(*Group)
	if !ok {
		t.Fatal("the child of the group is not a group")
	}
	imageNode, ok := groupNode2.Children()[0].(*Image)
	if !ok {
		t.Fatal("the child of the group is not an image")
	}

	want := mustRect(path.RectFromXYWH(35.0, 35.0, 50.0, 50.0))
	if got := imageNode.AbsBoundingBox(); got != want {
		t.Errorf("abs bounding box = %v, want %v", got, want)
	}
}

// TestNoTextNodes is no_text_nodes in usvg tests/parser.rs.
func TestNoTextNodes(t *testing.T) {
	svg := `
    <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'>
        <g transform='translate(20)'>
            <path transform='translate(10)' d='M 0 0 L 10 10'/>
        </g>
    </svg>
    `

	tree := mustFromStr(t, svg)
	if tree.HasTextNodes() {
		t.Error("the tree has text nodes")
	}
}

// TestFlattenedTextShouldInheritAbsoluteTransform is
// flattened_text_should_inherit_absolute_transform in usvg tests/parser.rs.
func TestFlattenedTextShouldInheritAbsoluteTransform(t *testing.T) {
	t.Skip("phase 9: text")
}

// TestUseNodeAbsTransform is use_node_abs_transform in usvg tests/parser.rs.
func TestUseNodeAbsTransform(t *testing.T) {
	svg := `
    <svg viewBox='0 0 200 200'
         xmlns='http://www.w3.org/2000/svg'
         xmlns:xlink='http://www.w3.org/1999/xlink'>
        <defs>
            <rect id='rect1' x='0' y='0' width='100' height='100'/>
        </defs>
        <use xlink:href='#rect1' transform='matrix(0.5, 0, 0, 0.5, 20, 30)' />
    </svg>
    `

	tree := mustFromStr(t, svg)

	groupNode, ok := tree.Root().Children()[0].(*Group)
	if !ok {
		t.Fatal("the first child is not a group")
	}
	if sx, sy := groupNode.AbsTransform().GetScale(); sx != 0.5 || sy != 0.5 {
		t.Errorf("group scale = (%v, %v), want (0.5, 0.5)", sx, sy)
	}

	pathNode, ok := groupNode.Children()[0].(*Path)
	if !ok {
		t.Fatal("the child of the group is not a path")
	}
	if sx, sy := pathNode.AbsTransform().GetScale(); sx != 0.5 || sy != 0.5 {
		t.Errorf("path scale = (%v, %v), want (0.5, 0.5)", sx, sy)
	}
	want := mustRect(path.RectFromXYWH(20.0, 30.0, 50.0, 50.0))
	if got := pathNode.AbsBoundingBox(); got != want {
		t.Errorf("abs bounding box = %v, want %v", got, want)
	}
}

// TestNestedSvgAbsBoundingBox is nested_svg_abs_bounding_box in usvg
// tests/parser.rs.
func TestNestedSvgAbsBoundingBox(t *testing.T) {
	svg := `
    <svg xmlns='http://www.w3.org/2000/svg' width='100' height='100'>
        <svg transform='translate(10, 20)' width='50' height='50'>
            <path id='path' d='M 0 0 L 10 10'/>
        </svg>
    </svg>
    `
	tree := mustFromStr(t, svg)
	node, ok := tree.NodeByID("path")
	if !ok {
		t.Fatal("no node with the id path")
	}
	want := mustRect(path.RectFromXYWH(10.0, 20.0, 10.0, 10.0))
	if got := node.AbsBoundingBox(); got != want {
		t.Errorf("abs bounding box = %v, want %v", got, want)
	}
}

// TestResolveFrFromReferencedRadialGradient is
// resolve_fr_from_referenced_radial_gradient in usvg tests/parser.rs.
func TestResolveFrFromReferencedRadialGradient(t *testing.T) {
	svg := `
    <svg viewBox='0 0 100 100' xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink'>
        <defs>
            <radialGradient id='base' gradientUnits='userSpaceOnUse' cx='50' cy='50' r='50' fx='50' fy='50' fr='25'>
                <stop offset='0%' stop-color='red'/>
                <stop offset='100%' stop-color='blue'/>
            </radialGradient>
            <radialGradient id='ref' href='#base' xlink:href='#base'/>
        </defs>
        <rect width='100' height='100' fill='url(#ref)'/>
    </svg>
    `

	tree := mustFromStr(t, svg)

	p, ok := tree.Root().Children()[0].(*Path)
	if !ok {
		t.Fatal("the first child is not a path")
	}
	fill, ok := p.Fill()
	if !ok {
		t.Fatal("the path has no fill")
	}
	rg, ok := fill.Paint().(*RadialGradient)
	if !ok {
		t.Fatalf("paint is %T, want *RadialGradient", fill.Paint())
	}

	if got := rg.Fr().Get(); got != 25.0 {
		t.Errorf("fr = %v, want 25", got)
	}
}

// TestFromStrHugeArc makes sure that a path with an arc to a huge point ends,
// with at most kurbo.MaxArcSubdivisions curves for the arc, where usvg does
// not end (D30).
func TestFromStrHugeArc(t *testing.T) {
	const text = `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">` +
		`<path d="M 30 40 A 40 30 20 1 1 1e50 100" fill="none" stroke="black"/></svg>`
	if _, err := FromStr(text); err != nil {
		t.Fatal(err)
	}
}
