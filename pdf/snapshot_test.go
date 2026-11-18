// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage/path"
)

// settings1 is settings_1 of krilla tests/src/main.rs, the settings of the
// snapshots: pretty, ASCII and no compression. The base port has no XMP
// metadata, which settings_1 turns off.
func settings1() []Option {
	return []Option{WithPretty(true), WithASCIICompatible(true), WithCompression(false)}
}

// checkSnapshot is check_snapshot of krilla tests/src/main.rs. It compares
// the file with the snapshot of krilla in testdata/snapshots, byte for byte.
// The settings of the snapshots compress no stream, so the bytes are the
// bytes of krilla, with the ID of the document.
func checkSnapshot(t *testing.T, name string, d *Document) {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshots", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		gotLines := strings.Split(string(got), "\n")
		wantLines := strings.Split(string(want), "\n")
		for i := range max(len(gotLines), len(wantLines)) {
			var g, w string
			if i < len(gotLines) {
				g = gotLines[i]
			}
			if i < len(wantLines) {
				w = wantLines[i]
			}
			if g != w {
				t.Fatalf("%s differs from the snapshot at the line %d:\ngot  %q\nwant %q", name, i+1, g, w)
			}
		}
	}
}

// TestPDFEmpty is the snapshot test pdf_empty of krilla tests/src/pdf.rs.
func TestPDFEmpty(t *testing.T) {
	d := NewDocument(settings1()...)
	checkSnapshot(t, "pdf_empty", d)
}

// TestPageLabel is the snapshot test page_label of krilla tests/src/page.rs.
func TestPageLabel(t *testing.T) {
	d := NewDocument(settings1()...)
	ps, _ := PageSettingsFromWH(200, 200)
	d.StartPage(ps)
	ps, _ = PageSettingsFromWH(250, 200)
	d.StartPage(ps)

	style := NumberingStyleLowerRoman
	ps, _ = PageSettingsFromWH(250, 200)
	d.StartPage(ps.WithPageLabel(NewPageLabel(&style, nil, 2)))
	checkSnapshot(t, "page_label", d)
}

// TestPageWithCropBleedingTrimArtBoxes is the snapshot test
// page_with_crop_bleeding_trim_art_boxes of krilla tests/src/page.rs.
func TestPageWithCropBleedingTrimArtBoxes(t *testing.T) {
	d := NewDocument(settings1()...)
	rect := func(x, y, w, h float32) *Rect {
		r, ok := path.RectFromXYWH(x, y, w, h)
		if !ok {
			t.Fatal("invalid rectangle")
		}
		return &r
	}
	ps, _ := PageSettingsFromWH(200, 200)
	ps = ps.
		WithCropBox(rect(10, 10, 180, 180)).
		WithBleedBox(rect(20, 20, 160, 160)).
		WithTrimBox(rect(30, 30, 140, 140)).
		WithArtBox(rect(40, 40, 120, 120))
	d.StartPage(ps)
	checkSnapshot(t, "page_with_crop_bleeding_trim_art_boxes", d)
}
