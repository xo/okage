// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// finish finishes a document into bytes, and fails the test on an error.
func finish(t *testing.T, d *Document) string {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestPageSettings checks the constructors and the methods of PageSettings
// in krilla src/page.rs.
func TestPageSettings(t *testing.T) {
	for _, wh := range [][2]float32{{0, 1}, {1, 0}, {-1, 1}, {float32(math.NaN()), 1}, {1, float32(math.Inf(1))}} {
		if _, ok := PageSettingsFromWH(wh[0], wh[1]); ok {
			t.Errorf("the size %v gives page settings", wh)
		}
	}
	d := DefaultPageSettings()
	if d.mediaBox.Width() != 595 || d.mediaBox.Height() != 842 || d.surfaceHeight() != 842 {
		t.Errorf("the default media box is %s", d.mediaBox)
	}
	r, _ := path.RectFromXYWH(1, 2, 3, 4)
	s := d.WithCropBox(&r)
	r, _ = path.RectFromXYWH(5, 6, 7, 8)
	if s.cropBox == nil || s.cropBox.Left() != 1 {
		t.Errorf("the crop box follows the variable: %v", s.cropBox)
	}
	if s.WithCropBox(nil).cropBox != nil {
		t.Error("nil does not remove the crop box")
	}
	if d.cropBox != nil {
		t.Error("WithCropBox changes the settings that it copies")
	}
	m := NewPageSettings(r)
	if m.mediaBox != r || m.pageLabel != (PageLabel{}) {
		t.Errorf("NewPageSettings gives %+v", m)
	}
	if got := (&Page{pageSettings: m}).rootTransform(); got != path.TransformFromRow(1, 0, 0, -1, 0, 8) {
		t.Errorf("the root transform is %v", got)
	}
}

// TestPageLabels checks the filter of PageLabelContainer::serialize in krilla
// src/page.rs, which writes a label only where the label of the page before
// does not continue to it. Upstream compares the offset of the previous
// label with the offset of the label plus 1. test/krilla compares the files
// with krilla.
func TestPageLabels(t *testing.T) {
	arabic, roman := NumberingStyleArabic, NumberingStyleLowerRoman
	empty, prefix := "", "A-"
	cases := []struct {
		prev, label PageLabel
		follow      bool
	}{
		{PageLabel{}, PageLabel{}, true},
		{NewPageLabel(&arabic, nil, 3), NewPageLabel(&arabic, nil, 2), true},
		{NewPageLabel(&arabic, nil, 2), NewPageLabel(&arabic, nil, 3), false},
		{NewPageLabel(&arabic, nil, 0), NewPageLabel(&arabic, nil, 1), false},
		{NewPageLabel(&arabic, nil, 1), NewPageLabel(&arabic, nil, 0), false},
		{NewPageLabel(&arabic, nil, 5), NewPageLabel(&arabic, nil, math.MaxUint32), false},
	}
	for _, c := range cases {
		if got := offsetsFollow(c.prev, c.label); got != c.follow {
			t.Errorf("the offsets %d and %d follow: %t", c.prev.offset, c.label.offset, got)
		}
	}
	if !NewPageLabel(nil, nil, 0).isEmpty() || NewPageLabel(nil, &empty, 0).isEmpty() {
		t.Error("a label with the prefix \"\" is empty")
	}
	if _, ok := newPageLabelContainer([]PageLabel{{}, {}}); ok {
		t.Error("pages without labels get a label tree")
	}

	d := NewDocument(WithPretty(false))
	for _, l := range []PageLabel{
		NewPageLabel(&roman, nil, 0),
		NewPageLabel(&roman, nil, 0),
		NewPageLabel(nil, &prefix, 0),
		NewPageLabel(&arabic, nil, math.MaxInt32+1),
		{},
	} {
		d.StartPage(DefaultPageSettings().WithPageLabel(l))
	}
	out := finish(t, d)
	for _, want := range []string{
		"<</Nums[0 3 0 R 2 4 0 R 3 5 0 R 4 6 0 R]>>",
		"<</Type/PageLabel/S/r>>",
		"<</Type/PageLabel/P(A-)>>",
		"<</Type/PageLabel/S/D>>",
		"<</Type/PageLabel>>",
		"/PageLabels 2 0 R",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the file has no %s:\n%s", want, out)
		}
	}
	for _, s := range []NumberingStyle{NumberingStyleArabic, NumberingStyleLowerRoman, NumberingStyleUpperRoman, NumberingStyleLowerAlpha, NumberingStyleUpperAlpha} {
		if s.toPDF() != pdfwriter.NumberingStyle(s) {
			t.Errorf("the style %d gives %s", s, s.toPDF())
		}
	}
}

// TestDocumentPages checks that StartPage and Finish finish the page that is
// open, and that a page finishes its surface first.
func TestDocumentPages(t *testing.T) {
	d := NewDocument(WithPretty(false))
	p1 := d.StartPage(DefaultPageSettings())
	calls := 0
	p1.finishSurface = func() { calls++ }
	ps, _ := PageSettingsFromWH(100, 50)
	p2 := d.StartPage(ps)
	if !p1.finished || calls != 1 || d.page != p2 {
		t.Errorf("StartPage does not finish the open page: %t, %d", p1.finished, calls)
	}
	p1.Finish()
	if calls != 1 || len(d.sc.pageInfos) != 1 {
		t.Errorf("a second Finish of a page does something: %d, %d", calls, len(d.sc.pageInfos))
	}
	out := finish(t, d)
	if !strings.Contains(out, "/Count 2") || !strings.Contains(out, "/MediaBox[0 0 100 50]") {
		t.Errorf("the file does not have the two pages:\n%s", out)
	}
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, okage.ErrPDFFinished) {
		t.Errorf("a second Finish gives %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("StartPage after Finish does not panic")
			}
		}()
		d.StartPage(DefaultPageSettings())
	}()
	d.SetLocation(7)
	if d.sc.location != 7 {
		t.Error("SetLocation does not set the location")
	}
	d.ResetLocation()
	if d.sc.location != 0 {
		t.Error("ResetLocation does not reset the location")
	}
}

// errWriter is a writer that fails.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

// TestDocumentErrors checks the errors of Document.Finish.
func TestDocumentErrors(t *testing.T) {
	if err := NewDocument().Finish(errWriter{}); err == nil || !strings.Contains(err.Error(), "writing pdf: disk full") {
		t.Errorf("a writer that fails gives %v", err)
	}

	// The media box flips to a box that is not finite, where krilla panics.
	r, ok := path.RectFromXYWH(0, -3e38, 10, 3e38)
	if !ok {
		t.Fatal("invalid rectangle")
	}
	d := NewDocument()
	d.StartPage(NewPageSettings(r))
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, okage.ErrPDFInvalidBox) {
		t.Errorf("a media box that does not flip gives %v", err)
	}
	ps, _ := PageSettingsFromWH(10, 3e38)
	d = NewDocument()
	d.StartPage(ps.WithArtBox(&r))
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, okage.ErrPDFInvalidBox) {
		t.Errorf("an art box that does not flip gives %v", err)
	}

	// A page tree with more kids than an array of PDF 1.4 can hold. The
	// streams of the pages are compressed. A new zlib writer for each stream
	// allocated 5.8 GB, and the reset writers of D74 allocate about 50 MB.
	d = NewDocument(WithPDFVersion(PDF14))
	for range arrayLen + 1 {
		d.StartPage(DefaultPageSettings())
	}
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, okage.ErrPDFTooLongArray) {
		t.Errorf("8192 pages of PDF 1.4 give %v", err)
	}
}

// testAnnotation is an annotation for the seam of the annotations of a page.
type testAnnotation struct {
	err error
}

func (a testAnnotation) serialize(sc *serializeContext, cc *chunkContainer, ref pdfwriter.Ref, pageHeight float32) error {
	if a.err != nil {
		return a.err
	}
	cc.nonStream.annotations.Indirect(ref).Dict().
		Pair(pdfwriter.Name("Height"), pdfwriter.Real(pageHeight)).
		Finish()
	return nil
}

// TestAnnotations checks the annotations of InternalPage::serialize in
// krilla src/page.rs: the key /Annots, and the key /Tabs from PDF 1.5.
func TestAnnotations(t *testing.T) {
	for _, c := range []struct {
		version PDFVersion
		tabs    bool
	}{
		{PDF14, false},
		{PDF15, true},
		{PDF20, true},
	} {
		d := NewDocument(WithPDFVersion(c.version), WithPretty(false))
		p := d.StartPage(DefaultPageSettings())
		p.annotations = append(p.annotations, testAnnotation{}, testAnnotation{})
		out := finish(t, d)
		if !strings.Contains(out, "/Annots[2 0 R 3 0 R]") || !strings.Contains(out, "<</Height 842>>") {
			t.Errorf("%s: the page has no annotations:\n%s", c.version, out)
		}
		if strings.Contains(out, "/Tabs/S") != c.tabs {
			t.Errorf("%s: the key /Tabs is there: %t", c.version, !c.tabs)
		}
		if len(d.sc.pageInfos[0].annotations) != 2 {
			t.Errorf("%s: the page infos have %d annotations", c.version, len(d.sc.pageInfos[0].annotations))
		}
	}
	d := NewDocument()
	p := d.StartPage(DefaultPageSettings())
	fault := errors.New("fault")
	p.annotations = append(p.annotations, testAnnotation{err: fault})
	if err := d.Finish(&bytes.Buffer{}); !errors.Is(err, fault) {
		t.Errorf("the error of an annotation gives %v", err)
	}
}

// TestPageStream checks that a page writes the stream that its surface gives,
// with its resources and its validation errors.
func TestPageStream(t *testing.T) {
	d := NewDocument(WithPretty(false), WithCompression(false))
	p := d.StartPage(DefaultPageSettings())
	rd := newResourceDictionaryBuilder()
	rd.registerResource(extGStateResource{pdfwriter.NewRef(1)})
	p.pageStream = newStream([]byte("0 0 m"), Rect{}, []validationError{{kind: validationTransparency}}, rd.finish(), false)
	out := finish(t, d)
	for _, want := range []string{"stream\n0 0 m\nendstream", "/ExtGState<</g0"} {
		if !strings.Contains(out, want) {
			t.Errorf("the file has no %q:\n%s", want, out)
		}
	}
}

// TestSerializeContext checks that the chunks and the content streams of the
// serializer take the setting pretty, and that the references follow each
// other after the reference of the page tree.
func TestSerializeContext(t *testing.T) {
	for _, pretty := range []bool{false, true} {
		sc := NewDocument(WithPretty(pretty)).sc
		settings := pdfwriter.Settings{Pretty: pretty}
		draw := func(c *pdfwriter.Content) string {
			c.MoveTo(1, 2).LineTo(3, 4)
			return string(c.Finish().AsSlice())
		}
		if got, want := draw(sc.newContent()), draw(pdfwriter.NewContentWithSettings(settings)); got != want {
			t.Errorf("pretty %t gives the content %q, want %q", pretty, got, want)
		}
		write := func(c *pdfwriter.Chunk) string {
			c.Indirect(pdfwriter.NewRef(1)).Dict().Pair(pdfwriter.Name("A"), pdfwriter.Int(1)).Finish()
			return string(c.AsBytes())
		}
		if got, want := write(sc.newChunk()), write(pdfwriter.NewChunkWithSettings(settings)); got != want {
			t.Errorf("pretty %t gives the chunk %q, want %q", pretty, got, want)
		}
		if sc.pageTreeRef.Get() != 1 || sc.newRef().Get() != 2 || sc.newRef().Get() != 3 {
			t.Error("the references do not follow the page tree")
		}
	}
}
