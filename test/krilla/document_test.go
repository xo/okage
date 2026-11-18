package krilla

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
)

// The tests of this file compare the documents and the pages of the package
// pdf with krilla, through the question "krilla" of the oracle (the gate of
// units 1 to 4 of phase 14 of the PDF writer in docs/PLAN.md). A scene of
// these tests has pages without a surface, because the surface comes with
// unit 5. The Go side builds the same scene with pdf. The objects of the two
// files must be the same after the reader decodes the streams. When no
// stream is compressed, the bytes must be the same, with the ID of the
// document. qpdf must accept each file of pdf.

// goScene builds the PDF file of a scene with the package pdf. It reads the
// settings and the pages that the oracle reads, and it fails for a page with
// calls of the surface.
func goScene(s scene) ([]byte, error) {
	var opts []pdf.Option
	for k, v := range s.Settings {
		flag, _ := v.(bool)
		switch k {
		case "pretty":
			opts = append(opts, pdf.WithPretty(flag))
		case "ascii_compatible":
			opts = append(opts, pdf.WithASCIICompatible(flag))
		case "compress_content_streams":
			opts = append(opts, pdf.WithCompression(flag))
		case "no_device_cs":
			opts = append(opts, pdf.WithNoDeviceCS(flag))
		case "xmp_metadata":
			if flag {
				return nil, errors.New("pdf has no XMP metadata")
			}
		case "version":
			versions := map[any]pdf.PDFVersion{
				"1.4": pdf.PDF14, "1.5": pdf.PDF15, "1.6": pdf.PDF16, "1.7": pdf.PDF17, "2.0": pdf.PDF20,
			}
			version, ok := versions[v]
			if !ok {
				return nil, fmt.Errorf("the version %v is not known", v)
			}
			opts = append(opts, pdf.WithPDFVersion(version))
		default:
			return nil, fmt.Errorf("the setting %q is not ported", k)
		}
	}
	d := pdf.NewDocument(opts...)
	for i, p := range s.Pages {
		if len(p.Ops) != 0 || p.Surface == nil || *p.Surface {
			return nil, fmt.Errorf("page %d has a surface, which comes with unit 5", i)
		}
		ps, err := goPageSettings(p)
		if err != nil {
			return nil, err
		}
		d.StartPage(ps).Finish()
	}
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// goPageSettings makes the page settings of a page of a scene, as
// page_settings of the oracle does.
func goPageSettings(p pageSpec) (pdf.PageSettings, error) {
	rect := func(r *[4]float32) (*pdf.Rect, error) {
		if r == nil {
			return nil, nil
		}
		rr, ok := path.RectFromXYWH(r[0], r[1], r[2], r[3])
		if !ok {
			return nil, fmt.Errorf("the rectangle %v is not valid", *r)
		}
		return &rr, nil
	}
	var ps pdf.PageSettings
	if p.MediaBox != nil {
		r, err := rect(p.MediaBox)
		if err != nil {
			return ps, err
		}
		ps = pdf.NewPageSettings(*r)
	} else {
		var ok bool
		if ps, ok = pdf.PageSettingsFromWH(p.Width, p.Height); !ok {
			return ps, errors.New("the size of a page is not valid")
		}
	}
	for _, b := range []struct {
		r    *[4]float32
		with func(pdf.PageSettings, *pdf.Rect) pdf.PageSettings
	}{
		{p.CropBox, pdf.PageSettings.WithCropBox},
		{p.BleedBox, pdf.PageSettings.WithBleedBox},
		{p.TrimBox, pdf.PageSettings.WithTrimBox},
		{p.ArtBox, pdf.PageSettings.WithArtBox},
	} {
		r, err := rect(b.r)
		if err != nil {
			return ps, err
		}
		ps = b.with(ps, r)
	}
	if l := p.Label; l != nil {
		var style *pdf.NumberingStyle
		if l.Style != nil {
			styles := map[string]pdf.NumberingStyle{
				"D": pdf.NumberingStyleArabic, "r": pdf.NumberingStyleLowerRoman, "R": pdf.NumberingStyleUpperRoman,
				"a": pdf.NumberingStyleLowerAlpha, "A": pdf.NumberingStyleUpperAlpha,
			}
			s, ok := styles[*l.Style]
			if !ok {
				return ps, fmt.Errorf("the numbering style %q is not known", *l.Style)
			}
			style = &s
		}
		var offset uint32
		if l.Offset != nil {
			offset = *l.Offset
		}
		ps = ps.WithPageLabel(pdf.NewPageLabel(style, l.Prefix, offset))
	}
	return ps, nil
}

// noSurface is the value of the field surface of a page without a surface.
var noSurface = new(bool)

// page returns a page of a scene of w by h without a surface.
func page(w, h float32) pageSpec {
	return pageSpec{Width: w, Height: h, Surface: noSurface}
}

// xywh returns a rectangle of a page of a scene.
func xywh(x, y, w, h float32) *[4]float32 {
	return &[4]float32{x, y, w, h}
}

// label returns a page label of a scene. An empty style or prefix and an
// offset of 0 are None.
func label(style, prefix string, offset uint32) *labelSpec {
	l := &labelSpec{}
	if style != "" {
		l.Style = &style
	}
	if prefix != "" {
		l.Prefix = &prefix
	}
	if offset != 0 {
		l.Offset = &offset
	}
	return l
}

// documentCase is a scene of the tests of this file.
type documentCase struct {
	name  string
	scene scene
}

// documentCases returns the scenes that units 1 to 4 of phase 14 can
// express: the settings of a document, the sizes and the boxes of its pages,
// and their labels.
func documentCases() []documentCase {
	var cases []documentCase
	add := func(name string, settings map[string]any, pages ...pageSpec) {
		s := map[string]any{"xmp_metadata": false}
		maps.Copy(s, settings)
		if pages == nil {
			// The field pages of a scene is not optional.
			pages = []pageSpec{}
		}
		cases = append(cases, documentCase{name, scene{Settings: s, Pages: pages}})
	}

	// An empty document gets the default page of Document::finish, with
	// each combination of the settings that change the bytes of the file.
	for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "2.0"} {
		for _, pretty := range []bool{false, true} {
			for _, ascii := range []bool{false, true} {
				for _, compress := range []bool{false, true} {
					name := fmt.Sprintf("empty/%s/pretty=%t/ascii=%t/compress=%t", version, pretty, ascii, compress)
					add(name, map[string]any{
						"version": version, "pretty": pretty, "ascii_compatible": ascii,
						"compress_content_streams": compress,
					})
				}
			}
		}
	}
	add("default settings", nil)
	add("no device cs", map[string]any{"no_device_cs": true}, page(100, 100))

	pretty := map[string]any{"pretty": true, "ascii_compatible": true, "compress_content_streams": false}
	add("sizes", pretty, page(200, 200), page(595.28, 841.89), page(0.5, 14400), page(1e-3, 3e4), page(612, 792))
	add("media boxes", pretty,
		pageSpec{MediaBox: xywh(-100, -100, 200, 200), Surface: noSurface},
		pageSpec{MediaBox: xywh(100, 100, 200, 200), Surface: noSurface},
		pageSpec{MediaBox: xywh(-150, -200, 500, 500), Surface: noSurface},
		pageSpec{MediaBox: xywh(0.25, 1.75, 33.3, 44.4), Surface: noSurface},
	)
	add("boxes", pretty,
		pageSpec{
			Width: 200, Height: 200, Surface: noSurface,
			CropBox: xywh(10, 10, 180, 180), BleedBox: xywh(20, 20, 160, 160),
			TrimBox: xywh(30, 30, 140, 140), ArtBox: xywh(40, 40, 120, 120),
		},
		pageSpec{Width: 300, Height: 100, Surface: noSurface, CropBox: xywh(-10, -10, 400, 400)},
		pageSpec{MediaBox: xywh(50, 50, 100, 100), Surface: noSurface, TrimBox: xywh(60, 70, 0, 0), ArtBox: xywh(55.5, 55.5, 9.1, 9.2)},
	)

	labelled := func(l *labelSpec) pageSpec {
		p := page(100, 100)
		p.Label = l
		return p
	}
	add("page label", pretty, page(200, 200), page(250, 200), labelled(label("r", "", 2)))
	add("labels of each style", pretty,
		labelled(label("D", "", 0)), labelled(label("r", "", 0)), labelled(label("R", "", 0)),
		labelled(label("a", "", 0)), labelled(label("A", "", 0)), labelled(label("", "Cover", 0)),
	)
	add("labels that repeat", pretty,
		labelled(label("D", "", 0)), labelled(label("D", "", 0)), labelled(label("D", "", 0)),
		page(100, 100), page(100, 100), labelled(label("R", "", 0)), labelled(label("R", "", 0)),
	)
	// Upstream compares the offset of the previous label with the offset of
	// the label plus 1.
	add("labels with offsets", pretty,
		labelled(label("D", "", 1)), labelled(label("D", "", 2)), labelled(label("D", "", 1)),
		labelled(label("D", "", 5)), labelled(label("D", "", 4)), labelled(label("D", "", 3)),
		labelled(label("D", "", 0)), labelled(label("D", "", 7)),
	)
	add("labels with prefixes", pretty,
		labelled(label("D", "A-", 1)), labelled(label("D", "A-", 1)), labelled(label("D", "B-", 1)),
		labelled(label("", "é (x) \\ ü", 0)), labelled(label("a", "\U0001F600", 3)),
	)
	add("label offsets that do not fit an i32", pretty,
		labelled(label("D", "", math.MaxInt32)), labelled(label("D", "", math.MaxInt32+1)),
		labelled(label("D", "", math.MaxUint32)), labelled(label("D", "", math.MaxUint32)),
		labelled(label("D", "", 1)),
	)
	add("labels compressed", map[string]any{"version": "1.4"},
		labelled(label("r", "", 0)), labelled(label("r", "x", 0)), page(10, 10))
	return cases
}

// emptyPrefix makes a label whose prefix is the empty string, which is
// Some("") and not None.
func emptyPrefix() pageSpec {
	p := page(100, 100)
	s, e := "D", ""
	p.Label = &labelSpec{Style: &s, Prefix: &e}
	return p
}

// TestDocumentGo compares the files of pdf with the files of krilla for the
// scenes of documentCases.
func TestDocumentGo(t *testing.T) {
	cases := documentCases()
	cases = append(cases, documentCase{"label with an empty prefix", scene{
		Settings: map[string]any{"xmp_metadata": false, "pretty": true, "compress_content_streams": false},
		Pages:    []pageSpec{emptyPrefix(), emptyPrefix(), page(100, 100)},
	}})
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if answers[i].Err != "" {
				t.Fatalf("the oracle gives an error: %s", answers[i].Err)
			}
			got, err := goScene(c.scene)
			if err != nil {
				t.Fatal(err)
			}
			want := answers[i].Data
			if compress, ok := c.scene.Settings["compress_content_streams"].(bool); ok && !compress {
				if !bytes.Equal(got, want) {
					t.Errorf("the bytes differ: %s", firstDiff(got, want))
				}
			}
			gotFile, err := readPDF(got)
			if err != nil {
				t.Fatalf("reading the file of pdf: %v", err)
			}
			wantFile, err := readPDF(want)
			if err != nil {
				t.Fatalf("reading the file of krilla: %v", err)
			}
			if diffs := comparePDF(gotFile, wantFile); len(diffs) != 0 {
				t.Errorf("%d differences:\n%s", len(diffs), strings.Join(diffs[:min(len(diffs), 10)], "\n"))
			}
			qpdfCheck(t, writeTemp(t, "go.pdf", got))
		})
	}
}

// TestDocumentGoLimits compares the error of PDF 1.4 for a page tree with
// more kids than an array of PDF 1.4 can hold.
func TestDocumentGoLimits(t *testing.T) {
	pages := make([]pageSpec, 8192)
	for i := range pages {
		pages[i] = page(10, 10)
	}
	s := scene{Settings: map[string]any{"xmp_metadata": false, "version": "1.4"}, Pages: pages}
	a := ask(t, sceneQuestion{Krilla: s})[0]
	if !strings.Contains(a.Err, "TooLongArray") {
		t.Fatalf("the oracle gives %q, want the error TooLongArray", a.Err)
	}
	if _, err := goScene(s); !errors.Is(err, okage.ErrPDFTooLongArray) {
		t.Fatalf("pdf gives the error %v, want ErrPDFTooLongArray", err)
	}

	// PDF 1.5 has no such limit.
	s.Settings["version"] = "1.5"
	a = ask(t, sceneQuestion{Krilla: s})[0]
	if a.Err != "" {
		t.Fatalf("the oracle gives an error: %s", a.Err)
	}
	got, err := goScene(s)
	if err != nil {
		t.Fatal(err)
	}
	gotFile, err := readPDF(got)
	if err != nil {
		t.Fatal(err)
	}
	wantFile, err := readPDF(a.Data)
	if err != nil {
		t.Fatal(err)
	}
	if diffs := comparePDF(gotFile, wantFile); len(diffs) != 0 {
		t.Errorf("%d differences:\n%s", len(diffs), strings.Join(diffs[:min(len(diffs), 10)], "\n"))
	}
}
