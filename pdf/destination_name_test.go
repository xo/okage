package pdf

import (
	"bytes"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/okage/path"
)

// TestDestinationNameResolvesLater checks that a link names a destination
// that the document registers after the page of the link, and that the
// file is the file of a link to the NamedDestination itself (D104).
func TestDestinationNameResolvesLater(t *testing.T) {
	write := func(byName bool) []byte {
		d := NewDocument(settings1()...)
		dest := NewNamedDestination("later", NewXYZDestination(1, path.PointFromXY(10, 20)))
		page := d.StartPage(DefaultPageSettings())
		var target Target = dest
		if byName {
			target = DestinationName("later")
		}
		page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), target))
		page.Finish()
		d.StartPage(DefaultPageSettings()).Finish()
		if byName && !d.RegisterNamedDestination(dest) {
			t.Fatal("the destination is rejected")
		}
		var buf bytes.Buffer
		if err := d.Finish(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	// The ID of the document differs with the order of the calls.
	id := regexp.MustCompile(`/ID \[[^\]]*\]`)
	got, want := id.ReplaceAll(write(true), nil), id.ReplaceAll(write(false), nil)
	if !bytes.Equal(got, want) {
		t.Errorf("expected the file of a link to the NamedDestination, got:\n%s\nwant:\n%s", got, want)
	}
}

// TestDestinationNameMissing checks that Document.Finish leaves out a link
// to a name without a destination, and logs it (D104).
func TestDestinationNameMissing(t *testing.T) {
	var log bytes.Buffer
	d := NewDocument(append(settings1(), WithLogger(slog.New(slog.NewTextHandler(&log, nil))))...)
	page := d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), DestinationName("nowhere")))
	page.AddAnnotation(linkTo(xywhRect(t, 0, 100, 100, 100), NewXYZDestination(0, path.PointFromXY(0, 0))))
	page.Finish()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("nowhere")) {
		t.Error("expected no link to the name nowhere")
	}
	if n := bytes.Count(buf.Bytes(), []byte("/Subtype /Link")); n != 1 {
		t.Errorf("expected one link, got %d", n)
	}
	if !strings.Contains(log.String(), "name=nowhere") {
		t.Errorf("expected a log of the name, got %q", log.String())
	}
}

// TestDestinationNameByLink checks that a link to a NamedDestination on a
// later page also registers the name (D104).
func TestDestinationNameByLink(t *testing.T) {
	d := NewDocument()
	page := d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), DestinationName("x")))
	page.Finish()
	page = d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), NewNamedDestination("x", NewXYZDestination(1, path.PointFromXY(0, 0)))))
	page.Finish()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
}
