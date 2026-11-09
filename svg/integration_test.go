// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg_test

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/svg"
)

// This file ports resvg tests/integration/main.rs, render.rs and extra.rs.
//
// Port: upstream generates render.rs from the file list with gen-tests.py.
// Go makes the same table at test time: renderTests walks
// testdata/tests, leaves out the files of the list IGNORE of gen-tests.py,
// and names each test as gen-tests.py names its function. Each test is a
// subtest with that name, such as TestRender/structure_style_important.
//
// resvg requires 0 pixels that differ for each test. okage applies the gates
// of D31 instead:
//
//  1. A file without text and without a raster image must give 0 pixels that
//     differ.
//  2. A file with a raster image passes when at most 1 percent of its pixels
//     differ, because okage decodes images with other code (D7).
//  3. A file with text passes when at most 1 percent of its pixels differ,
//     because okage lays out text on go-text (D9).
//  4. A test that does not pass its gate is in allowList, with its number of
//     pixels and the reason. An entry that no longer matches fails the test,
//     so that the list stays true.
//
// A file has text if its source holds "<text", "<tspan" or "<textPath". A
// file has a raster image if its tree, with each subroot and each nested SVG
// image, holds an image node of a raster kind. That is an <image> or a
// <feImage> that loads a raster image, directly or through an SVG image.
//
// The option MAKE_REF of upstream, which writes new reference images with
// oxipng, has no port. The flag -svg.diffs writes the diff images that
// upstream writes to tests/diffs to a folder of the caller's choice.

// diffsDir is the folder of the flag -svg.diffs.
var diffsDir = flag.String("svg.diffs", "", "write the diff image of each test that differs to this folder")

// globalFontDB is GLOBAL_FONTDB in resvg tests/integration/main.rs. It loads
// the fonts of testdata/fonts in a deterministic order.
var globalFontDB = sync.OnceValue(func() *svg.FontDatabase {
	fontdb := svg.NewFontDatabase()
	// Load fonts in a deterministic order.
	entries, err := os.ReadDir(filepath.Join(testdataDir, "fonts"))
	if err != nil {
		panic(err)
	}
	var fontPaths []string
	for _, entry := range entries {
		switch filepath.Ext(entry.Name()) {
		case ".ttf", ".ttc", ".otf", ".otc", ".TTF", ".TTC", ".OTF", ".OTC":
			fontPaths = append(fontPaths, filepath.Join(testdataDir, "fonts", entry.Name()))
		}
	}
	slices.Sort(fontPaths)
	for _, p := range fontPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			panic(fmt.Sprintf("loading font %s: %v", p, err))
		}
		fontdb.LoadFontData(data)
	}
	fontdb.SetSerifFamily("Noto Serif")
	fontdb.SetSansSerifFamily("Noto Sans")
	fontdb.SetCursiveFamily("Yellowtail")
	fontdb.SetFantasyFamily("Sedgwick Ave Display")
	fontdb.SetMonospaceFamily("Noto Mono")
	return fontdb
})

// imageSize is IMAGE_SIZE in resvg tests/integration/main.rs.
const imageSize = 300

// testdataDir is the folder that holds the copies of resvg crates/resvg/tests.
const testdataDir = "testdata"

// ignoreTests is IGNORE in resvg tests/gen-tests.py.
var ignoreTests = []string{
	"tests/filters/feMorphology/huge-radius", // will timeout on CI
	"tests/structure/svg/negative-size",      // invalid size
	"tests/structure/svg/no-size",            // invalid size
	"tests/structure/svg/zero-size",          // invalid size
	"tests/structure/svg/not-UTF-8-encoding", // invalid encoding
	// Produces slightly different output on some hardware.
	// Not a bug, just a SIMD rounding difference.
	"tests/paint-servers/radialGradient/focal-point-correction",
}

// allowEntry is an entry of allowList: the number of pixels that differ, and
// why.
type allowEntry struct {
	pixels int
	reason string
}

// allowList holds each test that does not pass its gate of D31, by the name
// of its subtest (D31 rule 4).
var allowList = map[string]allowEntry{}

// testMode is TestMode in resvg tests/integration/main.rs.
type testMode struct {
	// node is the ID of TestMode::Node, which renders a node by its ID.
	node string
	// extra is the scale of TestMode::Extra, which renders an extra test
	// with a specific scale. Zero is TestMode::Normal, which renders a
	// normal SVG test.
	extra float32
}

// renderTest is a test of render.rs or extra.rs.
type renderTest struct {
	name string
	file string
	mode testMode
}

// fnName is the function name that resvg tests/gen-tests.py makes from a
// file name without its extension.
func fnName(file string) string {
	n := strings.Replace(file, "tests/", "", 1)
	n = strings.ReplaceAll(n, "/", "_")
	n = strings.ReplaceAll(n, "-", "_")
	n = strings.ReplaceAll(n, "=", "_eq_")
	n = strings.ReplaceAll(n, ".", "_")
	n = strings.ReplaceAll(n, "#", "")
	return n
}

// renderTests is the table of resvg tests/integration/render.rs, which
// gen-tests.py makes from the SVG files of tests/tests.
func renderTests(t *testing.T) []renderTest {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join(testdataDir, "tests"), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(p) != ".svg" {
			return nil
		}
		rel, err := filepath.Rel(testdataDir, p)
		if err != nil {
			return err
		}
		files = append(files, strings.TrimSuffix(filepath.ToSlash(rel), ".svg"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(files)
	tests := make([]renderTest, 0, len(files))
	for _, file := range files {
		if slices.Contains(ignoreTests, file) {
			continue
		}
		tests = append(tests, renderTest{name: fnName(file), file: file})
	}
	return tests
}

// extraTests is the table of resvg tests/integration/extra.rs.
var extraTests = []renderTest{
	{"group_with_only_transform", "extra/group-with-only-transform", testMode{extra: 1}},
	{"subpixel_rect_position", "extra/subpixel-rect-position", testMode{extra: 1}},
	{"transformed_rect", "extra/transformed-rect", testMode{extra: 1}},
	{"hidden_element", "extra/hidden-element", testMode{extra: 1}},
	{"simple_stroke", "extra/simple-stroke", testMode{extra: 1}},
	{"fill_and_stroke", "extra/fill-and-stroke", testMode{extra: 1}},
	{"paint_order_stroke", "extra/paint-order=stroke", testMode{extra: 1}},
	{"stroke_linecap_square", "extra/stroke-linecap=square", testMode{extra: 1}},
	{"miter_join_with_acute_angle", "extra/miter-join-with-acute-angle", testMode{extra: 1}},
	{"horizontal_line", "extra/horizontal-line", testMode{extra: 1}},
	{"horizontal_line_no_stroke", "extra/horizontal-line-no-stroke", testMode{extra: 1}},
	{"filter_region_precision", "extra/filter-region-precision", testMode{extra: 10}},
	{"translate_outside_viewbox", "extra/translate-outside-viewbox", testMode{extra: 1}},
	{"render_node_filter_on_empty_group", "extra/filter-on-empty-group", testMode{node: "g1"}},
	{"render_node_filter_with_transform_on_shape", "extra/filter-with-transform-on-shape", testMode{node: "g1"}},
}

// gate is the gate of D31 that a test file takes.
type gate uint8

// The gates of D31.
const (
	gateExact  gate = iota // no text and no raster image: 0 pixels
	gateRaster             // a raster image: at most 1 percent
	gateText               // text: at most 1 percent
)

// String names the gate.
func (g gate) String() string {
	switch g {
	case gateExact:
		return "exact"
	case gateRaster:
		return "raster"
	case gateText:
		return "text"
	}
	return "gate(?)"
}

// result is the outcome of one test, for the summary.
type result struct {
	name   string
	gate   gate
	pixels int
	total  int
}

// TestRender runs the tests of resvg tests/integration/render.rs.
func TestRender(t *testing.T) {
	runTests(t, renderTests(t))
}

// TestExtra runs the tests of resvg tests/integration/extra.rs.
func TestExtra(t *testing.T) {
	runTests(t, extraTests)
}

// runTests runs each test in parallel, then logs a summary.
func runTests(t *testing.T, tests []renderTest) {
	t.Helper()
	if testing.Short() {
		t.Skip("the integration tests render each test file of resvg")
	}
	var mu sync.Mutex
	var results []result
	t.Run("group", func(t *testing.T) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				runTest(t, tt, func(r result) {
					mu.Lock()
					results = append(results, r)
					mu.Unlock()
				})
			})
		}
	})
	summarize(t, results)
}

// summarize logs the counts of each gate.
func summarize(t *testing.T, results []result) {
	t.Helper()
	var exact, raster, rasterExact, text, textExact, textPixels, allowed int
	for _, r := range results {
		if _, ok := allowList[r.name]; ok {
			allowed++
		}
		switch r.gate {
		case gateExact:
			if r.pixels == 0 {
				exact++
			}
		case gateRaster:
			raster++
			if r.pixels == 0 {
				rasterExact++
			}
		case gateText:
			text++
			textPixels += r.pixels
			if r.pixels == 0 {
				textExact++
			}
		}
	}
	t.Logf("tests: %d, exact with 0 pixels: %d, raster: %d (%d with 0 pixels), text: %d (%d with 0 pixels, %d pixels in all), allow-list: %d",
		len(results), exact, raster, rasterExact, text, textExact, textPixels, allowed)
}

// runTest renders one test, passes its result to record, and applies its
// gate.
func runTest(t *testing.T, tt renderTest, record func(result)) {
	t.Helper()
	svgPath := filepath.Join(testdataDir, filepath.FromSlash(tt.file)+".svg")
	data, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatal(err)
	}
	pixels, total, tree := renderInner(t, tt, data)

	g := gateExact
	switch {
	case hasText(data):
		g = gateText
	case hasRasterImage(tree.Root()):
		g = gateRaster
	}
	record(result{name: tt.name, gate: g, pixels: pixels, total: total})

	var fail string
	switch g {
	case gateExact:
		if pixels != 0 {
			fail = fmt.Sprintf("%d pixels differ, want 0", pixels)
		}
	case gateRaster:
		if pixels*100 > total {
			fail = fmt.Sprintf("%d of %d pixels differ, want at most 1 percent", pixels, total)
		}
	case gateText:
		if pixels*100 > total {
			fail = fmt.Sprintf("%d of %d pixels differ, want at most 1 percent", pixels, total)
		}
	}

	entry, listed := allowList[tt.name]
	switch {
	case listed && fail == "":
		t.Errorf("passes its %s gate with %d pixels, remove it from allowList", g, pixels)
	case listed && entry.pixels != pixels:
		t.Errorf("%s, allowList has %d pixels: update allowList", fail, entry.pixels)
	case listed:
		t.Logf("allowList: %d pixels differ: %s", pixels, entry.reason)
	case fail != "":
		t.Errorf("%s gate: %s", g, fail)
	case pixels != 0:
		t.Logf("%s gate: %d of %d pixels differ", g, pixels, total)
	}
}

// hasText reports whether the source of an SVG file holds text.
func hasText(data []byte) bool {
	return bytes.Contains(data, []byte("<text")) ||
		bytes.Contains(data, []byte("<tspan")) ||
		bytes.Contains(data, []byte("<textPath"))
}

// hasRasterImage reports whether parent, its subroots or a nested SVG image
// holds an image node of a raster kind.
func hasRasterImage(parent *svg.Group) bool {
	found := false
	var walk func(g *svg.Group)
	walk = func(g *svg.Group) {
		for _, node := range g.Children() {
			switch n := node.(type) {
			case *svg.Group:
				walk(n)
			case *svg.Image:
				if k, ok := n.Kind().(svg.ImageKindSVG); ok {
					walk(k.Tree.Root())
				} else {
					found = true
				}
			case *svg.Text:
				walk(n.Flattened())
			}
			node.Subroots(walk)
		}
	}
	walk(parent)
	return found
}

// testLogWriter writes each log line of the renderer to the test log.
type testLogWriter struct {
	t *testing.T
}

// Write logs p to the test log.
func (w testLogWriter) Write(p []byte) (int, error) {
	w.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

// renderInner is render_inner in resvg tests/integration/main.rs. It returns
// the number of pixels that differ, the number of pixels that get_diff
// compares, and the tree.
func renderInner(t *testing.T, tt renderTest, svgData []byte) (int, int, *svg.Tree) {
	t.Helper()
	svgPath := filepath.Join(testdataDir, filepath.FromSlash(tt.file)+".svg")
	pngPath := filepath.Join(testdataDir, filepath.FromSlash(tt.file)+".png")

	logger := slog.New(slog.NewTextHandler(testLogWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	tree, err := svg.FromData(svgData,
		svg.WithResourcesDir(filepath.Dir(svgPath)),
		svg.WithFontDatabase(globalFontDB()),
		svg.WithLogger(logger))
	if err != nil {
		t.Fatal(err)
	}

	var size path.IntSize
	var pixmap *okage.Pixmap
	var ok bool

	switch {
	case tt.mode.node == "" && tt.mode.extra == 0:
		size, ok = tree.Size().ToIntSize().ScaleToWidth(imageSize)
		mustOK(t, ok)
		pixmap, ok = okage.NewPixmap(size.Width(), size.Height())
		mustOK(t, ok)
		renderTs := path.TransformFromScale(
			float32(size.Width())/tree.Size().Width(),
			float32(size.Height())/tree.Size().Height(),
		)
		m := pixmap.AsMut()
		svg.Render(tree, renderTs, &m, svg.WithLogger(logger))
	case tt.mode.node != "":
		node, ok := tree.NodeByID(tt.mode.node)
		mustOK(t, ok)
		bbox, ok := svg.NodeAbsLayerBoundingBox(node)
		mustOK(t, ok)
		size = bbox.Size().ToIntSize()
		pixmap, ok = okage.NewPixmap(size.Width(), size.Height())
		mustOK(t, ok)
		m := pixmap.AsMut()
		svg.RenderNode(node, path.IdentityTransform(), &m, svg.WithLogger(logger))
	default:
		size, ok = tree.Size().ToIntSize().ScaleBy(tt.mode.extra)
		mustOK(t, ok)
		pixmap, ok = okage.NewPixmap(size.Width(), size.Height())
		mustOK(t, ok)
		renderTs := path.TransformFromScale(tt.mode.extra, tt.mode.extra)
		m := pixmap.AsMut()
		svg.Render(tree, renderTs, &m, svg.WithLogger(logger))
	}

	var actualImage testImage
	{
		width, height := pixmap.Width(), pixmap.Height()
		data := pixmap.Clone().Take()
		demultiplyAlpha(data)

		actualImage = testImage{data: data, width: width, height: height}
	}

	imageData, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatalf("missing reference image: %v", err)
	}
	referenceImage := loadPNG(t, imageData)

	diffImage, pixelDiff, total := getDiff(&referenceImage, &actualImage)
	if pixelDiff > 0 && *diffsDir != "" {
		if err := os.MkdirAll(*diffsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		diffImage.savePNG(t, filepath.Join(*diffsDir, strings.ReplaceAll(tt.file, "/", "_")+".png"))
	}
	return pixelDiff, total, tree
}

// mustOK is the unwrap of upstream: it stops the test when ok is false.
func mustOK(t *testing.T, ok bool) {
	t.Helper()
	if !ok {
		t.Fatal("called Option::unwrap() on a None value")
	}
}

// getDiff is get_diff in resvg tests/integration/main.rs. It returns the
// diff image and the number of pixels that differ. Upstream returns None
// when no pixel differs. The third result is the number of pixels that it
// compares, which the gate of D31 for a raster image reads.
func getDiff(expectedImage, actualImage *testImage) (testImage, int, int) {
	const diffThreshold uint8 = 1

	width := max(expectedImage.width, actualImage.width)
	height := max(expectedImage.height, actualImage.height)

	diffImage := newTestImage(3*width, height)

	pixelDiff := 0

	for x := range width {
		for y := range height {
			actualPixel, aok := actualImage.getPixel(x, y)
			expectedPixel, eok := expectedImage.getPixel(x, y)

			switch {
			case aok && eok:
				diffImage.setPixel(x, y, expectedPixel)
				diffImage.setPixel(x+2*width, y, actualPixel)
				if isPixDiff(expectedPixel, actualPixel, diffThreshold) {
					pixelDiff++
					diffImage.setPixel(x+width, y, color.NRGBA{255, 0, 0, 255})
				} else {
					diffImage.setPixel(x+width, y, color.NRGBA{0, 0, 0, 255})
				}
			case aok:
				pixelDiff++
				diffImage.setPixel(x+2*width, y, actualPixel)
				diffImage.setPixel(x+width, y, color.NRGBA{255, 0, 0, 255})
			case eok:
				pixelDiff++
				diffImage.setPixel(x, y, expectedPixel)
				diffImage.setPixel(x+width, y, color.NRGBA{255, 0, 0, 255})
			default:
				pixelDiff++
				diffImage.setPixel(x, y, color.NRGBA{255, 0, 0, 255})
				diffImage.setPixel(x+width, y, color.NRGBA{255, 0, 0, 255})
			}
		}
	}

	return diffImage, pixelDiff, int(width) * int(height)
}

// demultiplyAlpha is demultiply_alpha in resvg tests/integration/main.rs. It
// demultiplies provided pixels alpha.
func demultiplyAlpha(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		p := data[i : i+4 : i+4]
		a := float64(p[3]) / 255.0
		p[2] = asU8(float64(p[2])/a + 0.5)
		p[1] = asU8(float64(p[1])/a + 0.5)
		p[0] = asU8(float64(p[0])/a + 0.5)
	}
}

// asU8 is the Rust cast "f as u8" for an f64: it truncates and saturates,
// and NaN gives 0.
func asU8(f float64) uint8 {
	switch {
	case f != f || f <= 0:
		return 0
	case f >= 255:
		return 255
	}
	return uint8(f)
}

// isPixDiff is is_pix_diff in resvg tests/integration/main.rs.
func isPixDiff(pixel1, pixel2 color.NRGBA, threshold uint8) bool {
	if pixel1.A == 0 && pixel2.A == 0 {
		return false
	}

	different := false

	different = different || absDiff(pixel1.R, pixel2.R) > threshold
	different = different || absDiff(pixel1.G, pixel2.G) > threshold
	different = different || absDiff(pixel1.B, pixel2.B) > threshold
	different = different || absDiff(pixel1.A, pixel2.A) > threshold

	return different
}

// absDiff is u8::abs_diff of Rust.
func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// loadPNG is load_png in resvg tests/integration/main.rs.
//
// Port: upstream asks the crate png to expand a palette, a gray image with
// fewer than 8 bits and the tRNS chunk, and to strip 16 bits to 8 bits. It
// then reads RGBA, gray and gray with alpha, and panics on the other color
// types. image/png decodes here, and each pixel converts to RGBA with
// straight alpha: a 16-bit value keeps its high byte, as the strip does.
func loadPNG(t *testing.T, data []byte) testImage {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			switch im := img.(type) {
			case *image.NRGBA:
				c := im.NRGBAAt(x, y)
				out = append(out, c.R, c.G, c.B, c.A)
			case *image.NRGBA64:
				c := im.NRGBA64At(x, y)
				out = append(out, uint8(c.R>>8), uint8(c.G>>8), uint8(c.B>>8), uint8(c.A>>8))
			case *image.Gray:
				g := im.GrayAt(x, y).Y
				out = append(out, g, g, g, 255)
			case *image.Gray16:
				g := uint8(im.Gray16At(x, y).Y >> 8)
				out = append(out, g, g, g, 255)
			default:
				c, _ := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
				out = append(out, c.R, c.G, c.B, c.A)
			}
		}
	}
	return testImage{data: out, width: uint32(b.Dx()), height: uint32(b.Dy())}
}

// testImage is TestImage in resvg tests/integration/main.rs.
type testImage struct {
	data   []byte
	width  uint32
	height uint32
}

// newTestImage is TestImage::new in resvg tests/integration/main.rs.
func newTestImage(width, height uint32) testImage {
	return testImage{
		data:   make([]byte, int(width)*int(height)*4),
		width:  width,
		height: height,
	}
}

// getPixel is TestImage::get_pixel in resvg tests/integration/main.rs.
func (ti *testImage) getPixel(x, y uint32) (color.NRGBA, bool) {
	if x >= ti.width || y >= ti.height {
		return color.NRGBA{}, false
	}

	pos := (int(ti.width)*int(y) + int(x)) * 4

	return color.NRGBA{ti.data[pos], ti.data[pos+1], ti.data[pos+2], ti.data[pos+3]}, true
}

// setPixel is TestImage::set_pixel in resvg tests/integration/main.rs.
func (ti *testImage) setPixel(x, y uint32, val color.NRGBA) {
	pos := (int(ti.width)*int(y) + int(x)) * 4

	ti.data[pos], ti.data[pos+1], ti.data[pos+2], ti.data[pos+3] = val.R, val.G, val.B, val.A
}

// savePNG is TestImage::save_png in resvg tests/integration/main.rs.
func (ti *testImage) savePNG(t *testing.T, name string) {
	t.Helper()
	img := &image.NRGBA{
		Pix:    ti.data,
		Stride: int(ti.width) * 4,
		Rect:   image.Rect(0, 0, int(ti.width), int(ti.height)),
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
