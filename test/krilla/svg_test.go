package krilla

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/pdfsvg"
	"github.com/xo/okage/svg"
)

// The tests of this file compare pdfsvg with krilla-svg, through the
// question "svg" of the oracle (the gate of phase 17 of the PDF writer in
// docs/PLAN.md, item 4 of "Testing"). Each SVG file of the tests of resvg in
// svg/testdata/tests is drawn as the test svg_impl of krilla draws it: one
// page with the size of the tree, the fonts of svg/testdata/fonts, and the
// filter scale 2. When the objects of the two files are the same after the
// reader decodes the streams, the file passes. Otherwise MuPDF and Poppler
// render both files, and the metric of D31 compares the pixels of each
// renderer. A file without text and without a raster image must give 0
// pixels that differ. A file with text or a raster image can differ in at
// most 1 percent of its pixels. A file that differs because svg follows a
// later usvg than krilla-svg is in svgDifferences, with the reason.

// svgFontsDir is the folder of the fonts of the tests of resvg.
var svgFontsDir = filepath.Join(root, "svg", "testdata", "fonts")

// svgFonts gives the fonts of the tests of resvg, with the generic families
// of globalFontDB of the tests of svg.
func svgFonts() *fontsSpec {
	dir, err := filepath.Abs(svgFontsDir)
	if err != nil {
		panic(err)
	}
	return &fontsSpec{
		Dirs:      []string{dir},
		Serif:     "Noto Serif",
		SansSerif: "Noto Sans",
		Cursive:   "Yellowtail",
		Fantasy:   "Sedgwick Ave Display",
		Monospace: "Noto Mono",
	}
}

// svgFontDB is the font database of svgFonts, which the files of the
// tests share.
var svgFontDB = sync.OnceValue(func() *svg.FontDatabase {
	db, err := goFontDB(svgFonts())
	if err != nil {
		panic(err)
	}
	return db
})

// svgOracleTimeout is the time that the oracle has for one SVG file.
const svgOracleTimeout = time.Minute

// svgFilterScale is the filter scale of the test svg_impl of krilla.
const svgFilterScale = 2

// svgQuestionFor gives the question of the oracle for an SVG file, with the
// fonts and the settings of svgFile.
func svgQuestionFor(file string) svgQuestion {
	scale := float32(svgFilterScale)
	return svgQuestion{Svg: svgSpec{
		File:        file,
		FilterScale: &scale,
		Fonts:       svgFonts(),
		Settings:    map[string]any{"xmp_metadata": false},
	}}
}

// goSVG writes the PDF file of an SVG file with svg and pdfsvg, as the
// oracle writes it with usvg and krilla-svg. The document draws the SVG
// glyphs of fonts with pdfsvg.RenderGlyph.
func goSVG(file string) ([]byte, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	tree, err := svg.FromData(data,
		svg.WithResourcesDir(filepath.Dir(file)),
		svg.WithFontDatabase(svgFontDB()))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", file, err)
	}
	d := pdf.NewDocument(pdf.WithSVGGlyphs(pdfsvg.RenderGlyph))
	ps, ok := pdf.PageSettingsFromWH(tree.Size().Width(), tree.Size().Height())
	if !ok {
		return nil, fmt.Errorf("the size of %s is not valid", file)
	}
	page := d.StartPage(ps)
	s := page.Surface()
	if !pdfsvg.Draw(s, tree, tree.Size(), pdfsvg.WithFilterScale(svgFilterScale)) {
		return nil, fmt.Errorf("pdfsvg cannot draw %s", file)
	}
	s.Finish()
	page.Finish()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// svgDifferences holds the files of the tests of resvg, by their path in
// svg/testdata/tests without the suffix, that differ because svg follows a
// later usvg than krilla-svg, with the reason (item 4 of "Testing"). The
// reason names the commit of resvg after 0.47 that changed the reference
// image of the test, which svg ports. A file that the oracle does not
// answer is listed too: usvg 0.47 overflows its stack on some recursive
// references, and does not end on a loop of links.
var svgDifferences = map[string]string{
	"filters/feComposite/operator=lighter":                                    "resvg b12646b2 (#1142): Add support for plus-lighter and lighter",
	"filters/feDiffuseLighting/complex-transform":                             "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feDisplacementMap/semi-transparent-map":                          "resvg 0c40ddc7 (#1157): Fix feDisplacementMap applying scale twice",
	"filters/feDisplacementMap/uniform-map":                                   "resvg 0c40ddc7 (#1157): Fix feDisplacementMap applying scale twice",
	"filters/feDropShadow/flood-color=currentColor":                           "resvg 4bd3bebf (#1217): Resolve currentColor in flood-color",
	"filters/feFlood/complex-transform":                                       "resvg 0a5a15b4 (#1188): Fix filter primitive subregions with primitiveUnits=objectBoundingBox",
	"filters/feFlood/partial-subregion-with-primitiveUnits=objectBoundingBox": "resvg 0a5a15b4 (#1188): Fix filter primitive subregions with primitiveUnits=objectBoundingBox",
	"filters/feGaussianBlur/complex-transform":                                "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feImage/link-on-an-element-with-complex-transform":               "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feImage/recursive-links-3":                                       "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"filters/feImage/shared-with-objectBoundingBox-gradient":                  "resvg 48d7286e (#1164): Resolve paint servers in masks, patterns and feImage before caching",
	"filters/feImage/with-subregion-5":                                        "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feMerge/complex-transform":                                       "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feMorphology/empty-radius":                                       "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/huge-radius":                                        "the oracle does not answer in a minute, because feMorphology of resvg 0.47 is slow with a huge radius. The tests of resvg skip the file",
	"filters/feMorphology/negative-radius":                                    "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/no-radius":                                          "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/operator=dilate":                                    "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius=0.5":                                         "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius=0.5-with-objectBoundingBox":                  "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius=10-0":                                        "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius=10-1":                                        "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius=1-10":                                        "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius-larger-than-region":                          "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/radius-with-too-many-values":                        "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/simple-case":                                        "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/source-with-opacity":                                "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feMorphology/zero-radius":                                        "resvg fa9ff329 (#1120): Fix feMorphology kernel size and radius rounding",
	"filters/feOffset/complex-transform":                                      "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/fePointLight/complex-transform":                                  "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feSpotLight/complex-transform":                                   "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feTile/complex-transform":                                        "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/feTurbulence/complex-transform":                                  "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/filter-functions/two-urls-with-different-regions":                "resvg ff43ecf1 (#1144): Fix bug in filter chains with different filter regions",
	"filters/filter/on-group-with-an-empty-filtered-child":                    "resvg 88a0c592 (#1167): Make group bounding boxes optional. The empty group has no layer bounding box, so pdfsvg draws no filter for it",
	"filters/filter/subregion-and-primitiveUnits=objectBoundingBox-1":         "resvg 0a5a15b4 (#1188): Fix filter primitive subregions with primitiveUnits=objectBoundingBox",
	"filters/filter/subregion-and-primitiveUnits=objectBoundingBox-2":         "resvg 0a5a15b4 (#1188): Fix filter primitive subregions with primitiveUnits=objectBoundingBox",
	"filters/filter/subregion-and-primitiveUnits=objectBoundingBox-3":         "resvg 0a5a15b4 (#1188): Fix filter primitive subregions with primitiveUnits=objectBoundingBox",
	"filters/filter/transform-on-shape":                                       "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/filter/transform-on-shape-with-filter-region":                    "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/filter/with-multiple-transforms-1":                               "resvg 978775b5 (#1155): Fix rendering of filters with skews and rotations",
	"filters/flood-color/currentColor":                                        "resvg 4bd3bebf (#1217): Resolve currentColor in flood-color",
	"masking/clipPath/recursive-indirect-on-child":                            "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"masking/clipPath/recursive-indirect-on-self":                             "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"masking/mask/nested-mask-with-reference-to-parent":                       "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"masking/mask/recursive-indirect-on-child":                                "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"masking/mask/recursive-on-inherited-marker":                              "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"masking/mask/shared-with-objectBoundingBox-gradient":                     "resvg 48d7286e (#1164): Resolve paint servers in masks, patterns and feImage before caching",
	"painting/color/currentColor":                                             "resvg 4bd3bebf (#1217): Resolve currentColor in flood-color",
	"painting/context/with-pattern-objectBoundingBox-in-use":                  "resvg 91455ee0 (#1171): Fix spacing and seams in fractional-size SVG patterns",
	"painting/marker/recursive-5":                                             "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"painting/marker/recursive-via-mask":                                      "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"painting/marker/recursive-via-pattern":                                   "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"painting/marker/with-viewBox-1":                                          "resvg 8fef71a6 (#1163): Fix marker viewport clipping with preserveAspectRatio",
	"painting/marker/with-viewBox-and-preserveAspectRatio=slice":              "resvg 8fef71a6 (#1163): Fix marker viewport clipping with preserveAspectRatio",
	"painting/mix-blend-mode/plus-lighter":                                    "resvg b12646b2 (#1142): Add support for plus-lighter and lighter. pdfsvg draws the group with the raster fallback, and krilla-svg draws it as normal",
	"paint-servers/linearGradient/gradientTransform-via-href":                 "resvg 1fe3cfe8 (#1135): fix(usvg): inherit gradientTransform and patternTransform via href",
	"paint-servers/linearGradient/href-precedence-over-xlink":                 "resvg 376fafb2 (#1080): Make href take precedence over xlink:href",
	"paint-servers/linearGradient/recursive-xlink-href-into-loop":             "resvg 107c3128 (#1218): fix: check for loops in HrefIter",
	"paint-servers/linearGradient/recursive-xlink-href-into-loop-long-chain":  "resvg 107c3128 (#1218): fix: check for loops in HrefIter",
	"paint-servers/pattern/nested-pattern-inheriting-recursive-fill":          "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"paint-servers/pattern/patternTransform-via-href":                         "resvg 1fe3cfe8 (#1135): fix(usvg): inherit gradientTransform and patternTransform via href",
	"paint-servers/pattern/recursive-indirect-on-child":                       "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"paint-servers/pattern/recursive-on-inherited-fill":                       "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"paint-servers/pattern/recursive-on-inherited-stroke":                     "resvg 500d8605 (#1225): Properly recognize and reject recursive references",
	"paint-servers/pattern/shared-with-objectBoundingBox-gradient":            "resvg 48d7286e (#1164): Resolve paint servers in masks, patterns and feImage before caching",
	"paint-servers/radialGradient/gradientTransform-via-href":                 "resvg 1fe3cfe8 (#1135): fix(usvg): inherit gradientTransform and patternTransform via href",
	"structure/image/objectBoundingBox-clip-on-sliced-image":                  "resvg 021d44b7 (#1077): fix(usvg): use the viewport as the bbox of a sliced <image>",
	"structure/style/comment-between-rules":                                   "resvg 88635659 (#1161): Read all text of a style element, also after an XML comment",
	"structure/style/empty-type":                                              "resvg 88635659 (#1161): Read all text of a style element, also after an XML comment",
	"structure/style/uppercase-type":                                          "resvg 88635659 (#1161): Read all text of a style element, also after an XML comment",
	"structure/svg/nested-svg-with-transform-and-clip":                        "resvg 0b58fd6d (#1084): Fix properties of nested SVGs being applied twice",
	"structure/svg/nested-svg-with-transform-and-opacity":                     "resvg 0b58fd6d (#1084): Fix properties of nested SVGs being applied twice",
	"structure/svg/root-transform-with-viewbox":                               "resvg f6d4f7ab (#1085): Apply root SVG transform after the viewBox mapping",
	"structure/use/href-precedence-over-xlink":                                "resvg 376fafb2 (#1080): Make href take precedence over xlink:href",
	"text/dominant-baseline/inherited-through-nested-tspan":                   "resvg adb61c73 (#1086): Inherit dominant-baseline into nested <tspan> elements",
	"text/font-family/noto-sans-malayalam-1":                                  "resvg 961b7597 (#1043): Correctly calculate glyph advances",
	"text/text-anchor/arabic-with-dx":                                         "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
	"text/text-anchor/negative-dx":                                            "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
	"text/text-decoration/underline-with-dy-on-arabic":                        "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
	"text/textLength/zero":                                                    "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
	"text/textPath/arabic-with-dy":                                            "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
	"text/textPath/with-error-path":                                           "resvg 3c42a96f (#1020): Support path attribute on <textPath> element",
	"text/textPath/with-path":                                                 "resvg 3c42a96f (#1020): Support path attribute on <textPath> element",
	"text/textPath/with-path-and-xlink-href":                                  "resvg 3c42a96f (#1020): Support path attribute on <textPath> element",
	"text/tspan/arabic-with-dy":                                               "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
	"text/writing-mode/tb-with-negative-dy-and-text-anchor":                   "resvg 101e3f10 (#1145): Bug fixes for RTL text and dx/dy",
}

// The outcomes of a file of TestSVGFiles.
const (
	outcomeSameObjects  = "the same objects"
	outcomeSamePixels   = "objects differ, 0 pixels differ"
	outcomeWithinRaster = "text or a raster image, at most 1 percent of the pixels differ"
	outcomeBothFail     = "neither usvg nor svg parses the file"
	outcomeListed       = "differs, in the list of later usvg"
	outcomeFail         = "fails"
)

// TestSVGFiles compares the PDF file of each SVG file of the tests of resvg
// with the file of krilla-svg, and logs the number of files of each
// outcome.
func TestSVGFiles(t *testing.T) {
	needOracle(t)
	needProgram(t, "mutool")
	needProgram(t, "pdftoppm")
	dir := filepath.Join(root, "svg", "testdata", "tests")
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".svg") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if testing.Short() {
		// One file in 20 in short mode.
		var short []string
		for i, f := range files {
			if i%20 == 0 {
				short = append(short, f)
			}
		}
		files = short
	}

	var mu sync.Mutex
	counts := map[string]int{}
	seen := map[string]bool{}
	t.Run("files", func(t *testing.T) {
		for _, file := range files {
			name, err := filepath.Rel(dir, file)
			if err != nil {
				t.Fatal(err)
			}
			name = filepath.ToSlash(strings.TrimSuffix(name, ".svg"))
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				outcome := compareSVGFile(t, name, abs(t, "svg", "testdata", "tests", filepath.FromSlash(name)+".svg"))
				mu.Lock()
				counts[outcome]++
				seen[name] = true
				mu.Unlock()
			})
		}
	})
	var lines []string
	for k, v := range counts {
		lines = append(lines, fmt.Sprintf("%5d %s", v, k))
	}
	sort.Strings(lines)
	t.Logf("%d SVG files:\n%s", len(files), strings.Join(lines, "\n"))
	if !testing.Short() {
		for name := range svgDifferences {
			if !seen[name] {
				t.Errorf("the list of later usvg holds %s, which is not a test", name)
			}
		}
	}
}

// compareSVGFile compares the PDF file of pdfsvg for an SVG file with the
// file of krilla-svg, and returns the outcome.
func compareSVGFile(t *testing.T, name, file string) string {
	t.Helper()
	reason, listed := svgDifferences[name]
	fail := func(format string, args ...any) string {
		if listed {
			t.Logf("listed (%s): "+format, append([]any{reason}, args...)...)
			return outcomeListed
		}
		t.Errorf(format, args...)
		return outcomeFail
	}
	got, goErr := goSVG(file)
	// usvg 0.47 does not end for some files, so the oracle has a limit.
	ctx, cancel := context.WithTimeout(t.Context(), svgOracleTimeout)
	defer cancel()
	answers, err := tryAsk(ctx, t, svgQuestionFor(file))
	if err != nil {
		if goErr != nil {
			t.Errorf("pdfsvg gives an error, and the oracle does not answer: %v", goErr)
			return outcomeFail
		}
		return fail("the oracle does not answer: %v", err)
	}
	a := answers[0]
	switch {
	case a.Err != "" && goErr != nil:
		return outcomeBothFail
	case a.Err != "":
		return fail("krilla-svg gives an error and pdfsvg does not: %s", a.Err)
	case goErr != nil:
		return fail("pdfsvg gives an error and krilla-svg does not: %v", goErr)
	}
	gotFile, err := readPDF(got)
	if err != nil {
		t.Fatalf("reading the file of pdfsvg: %v", err)
	}
	for _, o := range gotFile.Objects {
		if o.DecodeErr != nil {
			t.Errorf("object %d of the file of pdfsvg: %v", o.Ref.Num, o.DecodeErr)
		}
	}
	wantFile, err := readPDF(a.Data)
	if err != nil {
		t.Fatalf("reading the file of krilla-svg: %v", err)
	}
	diffs := comparePDF(gotFile, wantFile)
	if len(diffs) == 0 {
		if listed {
			t.Errorf("the file is in the list of later usvg (%s), but the objects are the same", reason)
		}
		return outcomeSameObjects
	}
	rasterOrText := hasRasterOrText(gotFile) || hasRasterOrText(wantFile)
	gotPath := writeTemp(t, "pdfsvg.pdf", got)
	wantPath := writeTemp(t, "krilla.pdf", a.Data)
	worst := 0.0
	var notes []string
	for _, r := range []struct {
		name   string
		render func(*testing.T, string, int, float64) *image.NRGBA
	}{{"MuPDF", renderMuPDF}, {"Poppler", renderPoppler}} {
		g, w := r.render(t, gotPath, 1, 72), r.render(t, wantPath, 1, 72)
		n := diffPixels(g, w, 1)
		b := w.Bounds()
		share := float64(n) / float64(max(1, b.Dx()*b.Dy()))
		worst = max(worst, share)
		notes = append(notes, fmt.Sprintf("%s: %d of %d pixels differ", r.name, n, b.Dx()*b.Dy()))
	}
	summary := fmt.Sprintf("%s. %d objects differ, the first: %s", strings.Join(notes, ", "), len(diffs), diffs[0])
	switch {
	case worst == 0:
		if listed {
			t.Errorf("the file is in the list of later usvg (%s), but 0 pixels differ", reason)
		}
		return outcomeSamePixels
	case rasterOrText && worst <= 0.01:
		if listed {
			t.Errorf("the file is in the list of later usvg (%s), but it is within the gate: %s", reason, summary)
		}
		return outcomeWithinRaster
	}
	return fail("%s", summary)
}

// hasRasterOrText reports whether a PDF file holds an image or a font.
func hasRasterOrText(f *pdfFile) bool {
	for _, o := range f.Objects {
		c := canon(o.Value)
		if strings.Contains(c, "/Subtype /Image") || strings.Contains(c, "/Type /Font") {
			return true
		}
	}
	return false
}

// goFontDB fills a font database as load_svg of the oracle fills the
// database of usvg: the fonts of each folder in the order of their names,
// then each file, then the generic families. A nil spec gives an empty
// database, the default of usvg.
func goFontDB(f *fontsSpec) (*svg.FontDatabase, error) {
	db := svg.NewFontDatabase()
	if f == nil {
		return db, nil
	}
	var files []string
	for _, d := range f.Dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			return nil, err
		}
		var names []string
		for _, e := range entries {
			switch strings.ToLower(filepath.Ext(e.Name())) {
			case ".ttf", ".ttc", ".otf", ".otc":
				names = append(names, filepath.Join(d, e.Name()))
			}
		}
		slices.Sort(names)
		files = append(files, names...)
	}
	files = append(files, f.Files...)
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		db.LoadFontData(data)
	}
	for _, s := range []struct {
		family string
		set    func(string)
	}{
		{f.Serif, db.SetSerifFamily},
		{f.SansSerif, db.SetSansSerifFamily},
		{f.Cursive, db.SetCursiveFamily},
		{f.Fantasy, db.SetFantasyFamily},
		{f.Monospace, db.SetMonospaceFamily},
	} {
		if s.family != "" {
			s.set(s.family)
		}
	}
	return db, nil
}

// goDrawSVG makes the call draw_svg of a scene with pdfsvg, as the oracle
// makes it with krilla-svg. A width or a height of 0 takes the size of the
// tree.
func goDrawSVG(s *pdf.Surface, o opSpec) error {
	data, err := os.ReadFile(o.File)
	if err != nil {
		return err
	}
	db, err := goFontDB(o.Fonts)
	if err != nil {
		return err
	}
	tree, err := svg.FromData(data, svg.WithResourcesDir(filepath.Dir(o.File)), svg.WithFontDatabase(db))
	if err != nil {
		return fmt.Errorf("parsing %s: %w", o.File, err)
	}
	w, h := o.Width, o.Height
	if w == 0 {
		w = tree.Size().Width()
	}
	if h == 0 {
		h = tree.Size().Height()
	}
	size, ok := path.SizeFromWH(w, h)
	if !ok {
		return errors.New("the size of the SVG file is not valid")
	}
	var opts []pdfsvg.Option
	if o.EmbedText != nil {
		opts = append(opts, pdfsvg.WithEmbedText(*o.EmbedText))
	}
	if o.FilterScale != nil {
		opts = append(opts, pdfsvg.WithFilterScale(*o.FilterScale))
	}
	if !pdfsvg.Draw(s, tree, size, opts...) {
		return errors.New("pdfsvg cannot draw the tree")
	}
	return nil
}

// TestSceneDrawSVG compares the call draw_svg of a scene, which goDrawSVG
// makes with pdfsvg, with krilla-svg: an SVG file at another size than its
// own, after a transform, with a filter, which both draw into an image, and
// with the settings of SvgSettings.
func TestSceneDrawSVG(t *testing.T) {
	logo := abs(t, "docs", "okage.svg")
	blur := abs(t, "svg", "testdata", "tests", "filters", "filter-functions", "blur-function-mm-value.svg")
	text := abs(t, "svg", "testdata", "tests", "text", "text", "simple-case.svg")
	withText := drawSVG(text, 100, 100)
	withText["fonts"] = svgFonts()
	outlinedText := drawSVG(text, 100, 100)
	outlinedText["fonts"] = svgFonts()
	outlinedText["embed_text"] = false
	scaled := drawSVG(blur, 50, 50)
	scaled["filter_scale"] = 1
	q := sceneQuestion{Krilla: scene{
		Settings: map[string]any{"xmp_metadata": false},
		Pages: []pageSpec{{Width: 200, Height: 200, Ops: []op{
			drawSVG(logo, 200, 89),
			pushTransform(1, 0, 0, 1, 100, 100),
			drawSVG(blur, 50, 50),
			pop(),
			pushTransform(0.5, 0, 0, 0.5, 150, 150),
			scaled,
			pop(),
		}}, {Width: 200, Height: 200, Ops: []op{
			withText,
			pushTransform(1, 0, 0, 1, 100, 100),
			outlinedText,
			pop(),
		}}},
	}}
	compareScene(t, documentCase{"draw_svg", q.Krilla}, ask(t, q)[0])
}
