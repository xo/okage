package krilla

import (
	"bytes"
	"image"
	"image/color"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// abs returns the absolute path of a file of the repository, for a question
// of the oracle.
func abs(t *testing.T, parts ...string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// checkFile reads a PDF file with the reader, runs `qpdf --check` on it, and
// renders its first page with MuPDF and Poppler at 72 dots for each inch. It
// fails when a stream does not decode, when qpdf finds a fault, or when a
// page is not w by h pixels, within one pixel, or has no pixel that is not
// white. A missing program skips its part only.
func checkFile(t *testing.T, name string, data []byte, w, h int) *pdfFile {
	t.Helper()
	f, err := readPDF(data)
	if err != nil {
		t.Fatalf("reading the PDF file: %v", err)
	}
	for _, o := range f.Objects {
		if o.DecodeErr != nil {
			t.Errorf("object %d: %v", o.Ref.Num, o.DecodeErr)
		}
	}
	file := writeTemp(t, name, data)
	t.Run("qpdf", func(t *testing.T) {
		qpdfCheck(t, file)
	})
	check := func(t *testing.T, img *image.NRGBA) {
		t.Helper()
		b := img.Bounds()
		if b.Dx() < w-1 || b.Dx() > w+1 || b.Dy() < h-1 || b.Dy() > h+1 {
			t.Errorf("the page is %s, want %dx%d", describeImage(img), w, h)
		}
		if strings.HasSuffix(describeImage(img), " 0 pixels that are not white") {
			t.Errorf("the page is blank")
		}
		t.Logf("the page is %s", describeImage(img))
	}
	var mu, pp *image.NRGBA
	t.Run("mutool", func(t *testing.T) {
		mu = renderMuPDF(t, file, 1, 72)
		check(t, mu)
	})
	t.Run("pdftoppm", func(t *testing.T) {
		pp = renderPoppler(t, file, 1, 72)
		check(t, pp)
	})
	if mu != nil && pp != nil {
		t.Logf("%d pixels differ between MuPDF and Poppler by more than 32", diffPixels(mu, pp, 32))
	}
	return f
}

// TestLogo is the gate of phase 11 of the PDF writer: the oracle writes the
// PDF file of docs/okage.svg, the reader reads it and decodes its streams,
// qpdf accepts it, and MuPDF and Poppler render it.
func TestLogo(t *testing.T) {
	data := askPDF(t, svgQuestion{Svg: svgSpec{File: abs(t, "docs", "okage.svg")}})
	f := checkFile(t, "okage.pdf", data, 1142, 508)
	pages := 0
	for _, o := range f.Objects {
		d, ok := o.Value.(*pdfDict)
		if !ok || d.get("Type") != pdfName("Page") {
			continue
		}
		pages++
		if got := canon(f.resolve(d.get("MediaBox"))); got != "[0 0 1142 508]" {
			t.Errorf("the media box is %s", got)
		}
	}
	if pages != 1 {
		t.Errorf("the file has %d pages, want 1", pages)
	}
}

// TestCompareIgnoresTheEncoding writes the logo with the settings of krilla
// that change only the encoding of the streams, and compares each file with
// the file of the first settings of its group. The objects must compare
// equal (D62). The setting pretty also changes the white space of the content
// streams, which the comparison does not ignore, so each group has one value
// of pretty. The files have no XMP metadata, which the base port leaves out
// (D62). Its InstanceID is a hash of the compressed bytes, as /ID is.
func TestCompareIgnoresTheEncoding(t *testing.T) {
	logo := abs(t, "docs", "okage.svg")
	groups := [][]map[string]any{
		{
			{"xmp_metadata": false},
			{"xmp_metadata": false, "compress_content_streams": false},
			{"xmp_metadata": false, "ascii_compatible": true},
		},
		{
			{"xmp_metadata": false, "pretty": true},
			{"xmp_metadata": false, "pretty": true, "ascii_compatible": true, "compress_content_streams": false},
		},
	}
	for _, settings := range groups {
		var files []*pdfFile
		for _, s := range settings {
			data := askPDF(t, svgQuestion{Svg: svgSpec{File: logo, Settings: s}})
			f, err := readPDF(data)
			if err != nil {
				t.Fatalf("reading the file of the settings %v: %v", s, err)
			}
			files = append(files, f)
		}
		for i, f := range files[1:] {
			if diffs := comparePDF(f, files[0]); len(diffs) != 0 {
				t.Errorf("the settings %v give %d differences from %v:\n%s",
					settings[i+1], len(diffs), settings[0], strings.Join(diffs[:min(len(diffs), 10)], "\n"))
			}
		}
	}
}

// TestCompareFindsADifference makes sure that comparePDF reports a scene
// that differs in one color.
func TestCompareFindsADifference(t *testing.T) {
	page := func(c map[string]any) pageSpec {
		return pageSpec{Width: 100, Height: 100, Ops: []op{setFill(c, 1, "nonzero"), drawPath(rect(10, 10, 80, 80))}}
	}
	settings := map[string]any{"xmp_metadata": false}
	answers := ask(t,
		sceneQuestion{Krilla: scene{Settings: settings, Pages: []pageSpec{page(rgb(255, 0, 0))}}},
		sceneQuestion{Krilla: scene{Settings: settings, Pages: []pageSpec{page(rgb(255, 0, 1))}}},
	)
	var files []*pdfFile
	for _, a := range answers {
		if a.Err != "" {
			t.Fatalf("the oracle gives an error: %s", a.Err)
		}
		f, err := readPDF(a.Data)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	if diffs := comparePDF(files[0], files[0]); len(diffs) != 0 {
		t.Errorf("a file differs from itself:\n%s", strings.Join(diffs, "\n"))
	}
	diffs := comparePDF(files[1], files[0])
	if len(diffs) != 1 || !strings.Contains(diffs[0], "the data of the stream differs") {
		t.Errorf("want one difference in a content stream, got %d:\n%s", len(diffs), strings.Join(diffs, "\n"))
	}
}

// TestScene writes a scene that makes each kind of call of the surface. qpdf
// must accept the file, and MuPDF and Poppler must draw it.
func TestScene(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{uint8(60 * x), uint8(60 * y), 200, uint8(255 - 40*x)})
		}
	}
	linear := map[string]any{"linear": map[string]any{
		"x1": 0, "y1": 0, "x2": 100, "y2": 0, "spread": "reflect",
		"stops": []any{stop(0, rgb(255, 0, 0), 1), stop(1, rgb(0, 0, 255), 0.5)},
	}}
	radial := map[string]any{"radial": map[string]any{
		"fx": 50, "fy": 50, "fr": 0, "cx": 50, "cy": 50, "cr": 40,
		"stops": []any{stop(0, luma(255), 1), stop(1, luma(0), 1)},
	}}
	sweep := map[string]any{"sweep": map[string]any{
		"cx": 50, "cy": 50, "start_angle": 0, "end_angle": 360,
		"stops": []any{stop(0, rgb(0, 160, 0), 1), stop(1, rgb(255, 255, 0), 1)},
	}}
	pattern := map[string]any{"pattern": map[string]any{
		"width": 10, "height": 10,
		"ops": []op{setFill(rgb(0, 128, 0), 1, "nonzero"), drawPath(rect(0, 0, 5, 5))},
	}}
	text := "Hi fi"
	q := sceneQuestion{Krilla: scene{
		Fonts:  []fontSpec{{File: abs(t, "svg", "testdata", "fonts", "NotoSans-Regular.ttf")}},
		Images: []imageSpec{rgbaImage(img)},
		Pages: []pageSpec{
			{Width: 200, Height: 200, Ops: []op{
				setFill(linear, 1, "nonzero"),
				drawPath(rect(0, 0, 100, 100)),
				pushTransform(1, 0, 0, 1, 100, 0),
				setFill(radial, 0.8, "evenodd"),
				drawPath(rect(0, 0, 100, 100)),
				pop(),
				pushTransform(1, 0, 0, 1, 0, 100),
				pushClipPath([]seg{{0, 50, 0}, {3, 100, 0, 100, 100, 50, 100}, {2, 0, 100, 0, 50}, {4}}, "nonzero"),
				setFill(sweep, 1, "nonzero"),
				drawPath(rect(0, 0, 100, 100)),
				pop(),
				pop(),
				pushTransform(1, 0, 0, 1, 100, 100),
				pushOpacity(0.5),
				pushBlendMode("multiply"),
				pushIsolated(),
				pushMask("luminosity", setFill(luma(200), 1, "nonzero"), drawPath(rect(0, 0, 50, 100))),
				setFill(pattern, 1, "nonzero"),
				setStroke(map[string]any{
					"paint": cmyk(0, 0, 0, 200), "width": 3, "line_cap": "round", "line_join": "bevel",
					"dash": map[string]any{"array": []float32{6, 3}, "offset": 1},
				}),
				drawPath(rect(10, 10, 80, 80)),
				pop(),
				pop(),
				pop(),
				pop(),
				noStroke(),
				drawImage(0, 40, 40),
				pop(),
			}},
			{Width: 200, Height: 100, Ops: []op{
				setFill(rgb(0, 0, 0), 1, "nonzero"),
				drawText(10, 40, 0, 24, text),
				// The glyphs of NotoSans for "Hi fi", with "fi" as one
				// cluster of two glyphs, which krilla writes with
				// ActualText.
				drawGlyphs(10, 80, 0, 24, text,
					glyph(43, 0, 1, 0.741), glyph(76, 1, 2, 0.258), glyph(3, 2, 3, 0.26),
					glyph(73, 3, 5, 0.344), glyph(76, 3, 5, 0.258)),
				noFill(),
				setStroke(map[string]any{"paint": rgb(255, 0, 0), "width": 0.5}),
				drawText(120, 40, 0, 24, "Hi"),
				pushTransform(1, 0, 0, 1, 150, 50),
				// A filter, which krilla-svg draws with resvg into an image.
				drawSVG(abs(t, "svg", "testdata", "tests", "filters", "filter-functions", "blur-function-mm-value.svg"), 50, 50),
				pop(),
			}},
		},
	}}
	data := askPDF(t, q)
	f := checkFile(t, "scene.pdf", data, 200, 200)
	for _, want := range []string{"/ActualText", "/CIDFontType2", "/ToUnicode", "/SMask", "/BM /Multiply", "/PatternType 1", "/Subtype /Image"} {
		found := false
		for _, o := range f.Objects {
			if strings.Contains(canon(o.Value), want) || (o.IsStream && bytes.Contains(o.Data, []byte(want))) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no object of the file holds %s", want)
		}
	}
}

// TestSceneErrors makes sure that a fault of a question is an answer with
// an error, and that the oracle answers the next question.
func TestSceneErrors(t *testing.T) {
	answers := ask(t,
		sceneQuestion{Krilla: scene{Pages: []pageSpec{{Width: 10, Height: 10, Ops: []op{pushOpacity(0.5)}}}}},
		sceneQuestion{Krilla: scene{Pages: []pageSpec{{Width: 10, Height: 10, Ops: []op{pop()}}}}},
		sceneQuestion{Krilla: scene{Pages: []pageSpec{{Width: 10, Height: 10, Ops: []op{drawImage(3, 1, 1)}}}}},
		sceneQuestion{Krilla: scene{Pages: []pageSpec{{Width: 10, Height: 10, Ops: []op{drawPath(rect(1, 1, 5, 5))}}}}},
	)
	for i, want := range []string{"1 pushes have no pop", "a pop has no push", "the image 3 does not exist"} {
		if !strings.Contains(answers[i].Err, want) {
			t.Errorf("question %d: got the error %q, want %q", i, answers[i].Err, want)
		}
	}
	if answers[3].Err != "" || !bytes.HasPrefix(answers[3].Data, []byte("%PDF-")) {
		t.Errorf("the last question gives no PDF file: %q", answers[3].Err)
	}
}

// TestSubset asks the oracle for subsets of fonts.
func TestSubset(t *testing.T) {
	noto := abs(t, "svg", "testdata", "fonts", "NotoSans-Regular.ttf")
	flex := abs(t, "svg", "testdata", "fonts", "RobotoFlex.subset.ttf")
	answers := ask(t,
		subsetQuestion{Subset: subsetSpec{Font: noto, Glyphs: []uint16{36, 37, 38}}},
		subsetQuestion{Subset: subsetSpec{Font: noto, Glyphs: []uint16{36, 37, 38}}},
		subsetQuestion{Subset: subsetSpec{Font: noto, Glyphs: []uint16{38, 37, 36}}},
		subsetQuestion{Subset: subsetSpec{Font: noto, Glyphs: []uint16{36, 37, 38}, Coords: &[]coord{}}},
		subsetQuestion{Subset: subsetSpec{Font: flex, Glyphs: []uint16{1, 2, 3}, Coords: &[]coord{{"wght", 700}, {"wdth", 50}}}},
		subsetQuestion{Subset: subsetSpec{Font: abs(t, "missing.ttf"), Glyphs: []uint16{1}}},
	)
	tag := regexp.MustCompile(`^[A-Z]{6}$`)
	for i, a := range answers[:5] {
		if a.Err != "" {
			t.Fatalf("question %d: %s", i, a.Err)
		}
		if len(a.Data) < 12 || !tag.Match(a.Data[:6]) {
			t.Fatalf("question %d: the answer does not start with a tag: %q", i, a.Data[:min(12, len(a.Data))])
		}
		if !bytes.HasPrefix(a.Data[6:], []byte{0, 1, 0, 0}) {
			t.Errorf("question %d: the subset is not a TrueType font: % x", i, a.Data[6:10])
		}
	}
	if !bytes.Equal(answers[0].Data, answers[1].Data) {
		t.Error("the same question gives two answers")
	}
	if bytes.Equal(answers[0].Data[:6], answers[2].Data[:6]) {
		t.Error("another order of the glyphs gives the same tag")
	}
	if !bytes.Equal(answers[0].Data, answers[3].Data) {
		t.Error("subset_with_variations without coordinates differs from subset for a TrueType font")
	}
	if !strings.Contains(answers[5].Err, "missing.ttf") {
		t.Errorf("a missing font gives the error %q", answers[5].Err)
	}
}

// TestPDFWriter asks the oracle for the bytes of calls of pdf-writer.
func TestPDFWriter(t *testing.T) {
	no := false
	content := []writeOp{
		{Op: "q"},
		{Op: "rg", Operands: []any{pw.Real(1), pw.Real(0.5), pw.Int(0)}},
		{Op: "re", Operands: []any{pw.Int(10), pw.Int(10), pw.Real(80.5), pw.Int(80)}},
		{Op: "f"},
		{Op: "Q"},
	}
	answers := ask(t,
		// The test test_xref_free_list_short of lib.rs of pdf-writer.
		writerQuestion{PdfWriter: writerSpec{Objects: []writerObject{
			{ID: 1, Value: pw.Int(1)},
			{ID: 2, Value: pw.Int(2)},
		}}},
		writerQuestion{PdfWriter: writerSpec{Pretty: &no, Objects: []writerObject{
			{ID: 1, Start: "catalog", Dict: [][2]any{pw.Pair("Pages", pw.Ref(2))}},
			{ID: 2, Start: "pages", Dict: [][2]any{pw.Pair("Kids", pw.Array(pw.Ref(3))), pw.Pair("Count", pw.Int(1))}},
			{ID: 3, Start: "page", Dict: [][2]any{
				pw.Pair("Parent", pw.Ref(2)),
				pw.Pair("MediaBox", pw.Array(pw.Int(0), pw.Int(0), pw.Int(100), pw.Int(100))),
				pw.Pair("Contents", pw.Ref(4)),
				pw.Pair("Resources", pw.Dict()),
			}},
			{ID: 4, Content: content},
			{ID: 5, Value: pw.Dict(
				pw.Pair("Null", pw.Null), pw.Pair("Bool", pw.Bool(true)), pw.Pair("Str", pw.Str([]byte("a(b)\n"))),
				pw.Pair("Text", pw.Text("é")), pw.Pair("Name", pw.Name("A B")),
			)},
		}}},
		writerQuestion{PdfWriter: writerSpec{Objects: []writerObject{{ID: 1, Start: "nothing"}}}},
	)
	want := strings.Join([]string{
		"%PDF-1.7\n%\x80\x80\x80\x80\n",
		"1 0 obj\n1\nendobj\n",
		"2 0 obj\n2\nendobj\n",
		"xref",
		"0 3",
		"0000000000 65535 f\r",
		"0000000016 00000 n\r",
		"0000000034 00000 n\r",
		"trailer",
		"<<\n  /Size 3\n>>",
		"startxref\n52\n%%EOF",
	}, "\n")
	if string(answers[0].Data) != want {
		t.Errorf("got %q, want %q", answers[0].Data, want)
	}
	if answers[1].Err != "" {
		t.Fatal(answers[1].Err)
	}
	f := checkFile(t, "writer.pdf", answers[1].Data, 100, 100)
	if o := f.object(4); o == nil || string(o.Data) != "q 1 0.5 0 rg 10 10 80.5 80 re\nf\nQ" {
		t.Errorf("the content stream is not as expected: %+v", o)
	}
	if got := canon(f.object(5).Value); got != `<< /Null null /Bool true /Str "a(b)\n" /Text <feff00e9> /Name /A B >>` {
		t.Errorf("object 5 is %s", got)
	}
	if !strings.Contains(answers[2].Err, "nothing") {
		t.Errorf("an unknown start gives the error %q", answers[2].Err)
	}
}
