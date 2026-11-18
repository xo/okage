// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/xo/okage/path"
)

// The tests of this file write the snapshots of krilla that draw graphics
// other than the solid paths of surface_test.go: the snapshot tests of
// tests/src/pattern.rs, stream.rs and pdf.rs.

// TestStreamResourceCache is the snapshot test stream_resource_cache of
// krilla tests/src/stream.rs, with settings_2: the colors take the embedded
// sRGB profile.
func TestStreamResourceCache(t *testing.T) {
	d := NewDocument(append(settings1(), WithNoDeviceCS(true))...)
	ps, _ := PageSettingsFromWH(200, 200)
	surface := d.StartPage(ps).Surface()
	surface.SetFill(greenFill(t, 1))
	surface.DrawPath(rectToPath(t, 0, 0, 100, 100))
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(rectToPath(t, 50, 50, 150, 150))
	surface.SetFill(blueFill(t, 1))
	surface.DrawPath(rectToPath(t, 100, 100, 200, 200))
	checkSnapshotDecoded(t, "stream_resource_cache", d)
}

// TestPDF14ICCSRGB is the snapshot test pdf_14_icc_srgb of krilla
// tests/src/pdf.rs, with settings_18: PDF 1.4 and the ICC v2 profiles.
func TestPDF14ICCSRGB(t *testing.T) {
	d := NewDocument(append(settings1(), WithPDFVersion(PDF14), WithNoDeviceCS(true))...)
	ps, _ := PageSettingsFromWH(200, 200)
	surface := d.StartPage(ps).Surface()
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(rectToPath(t, 50, 50, 100, 100))
	checkSnapshotDecoded(t, "pdf_14_icc_srgb", d)
}

// TestPDF14ICCSGray is the snapshot test pdf_14_icc_sgray of krilla
// tests/src/pdf.rs. The surface draws with the default fill, which is black
// luma.
func TestPDF14ICCSGray(t *testing.T) {
	d := NewDocument(append(settings1(), WithPDFVersion(PDF14), WithNoDeviceCS(true))...)
	ps, _ := PageSettingsFromWH(200, 200)
	surface := d.StartPage(ps).Surface()
	surface.DrawPath(rectToPath(t, 50, 50, 100, 100))
	checkSnapshotDecoded(t, "pdf_14_icc_sgray", d)
}

// TestPDFPrettyFalse is the snapshot test pdf_pretty_false of krilla
// tests/src/pdf.rs, with settings_31.
func TestPDFPrettyFalse(t *testing.T) {
	d := NewDocument(append(settings1(), WithPretty(false))...)
	ps, _ := PageSettingsFromWH(200, 200)
	surface := d.StartPage(ps).Surface()
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "pdf_pretty_false", d)
}

// radialSnapshotGradient is the radial gradient of the snapshot tests of
// krilla tests/src/pattern.rs.
func radialSnapshotGradient(stops []Stop) RadialGradient {
	return RadialGradient{
		Cx: 100, Cy: 100, Cr: 30, Fx: 120, Fy: 120, Fr: 60,
		Transform:    path.IdentityTransform(),
		SpreadMethod: SpreadMethodPad,
		Stops:        stops,
	}
}

// TestPatternGradientSingleStop is the snapshot test
// pattern_gradient_single_stop of krilla tests/src/pattern.rs. A gradient
// with one stop is a solid color.
func TestPatternGradientSingleStop(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	// stops_with_1_solid of krilla tests/src/main.rs.
	gradient := radialSnapshotGradient([]Stop{{Offset: 0.5, Color: NewColorRGB(255, 0, 0), Opacity: 1}})
	surface.SetFill(&Fill{Paint: gradient, Opacity: 1})
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "pattern_gradient_single_stop", d)
}

// TestPatternLumaStops is the snapshot test pattern_luma_stops of krilla
// tests/src/pattern.rs.
func TestPatternLumaStops(t *testing.T) {
	d, page := snapshotPage()
	surface := page.Surface()
	// stops_with_3_luma of krilla tests/src/main.rs.
	gradient := radialSnapshotGradient([]Stop{
		{Offset: 0, Color: NewColorLuma(0), Opacity: 1},
		{Offset: 0.3, Color: NewColorLuma(128), Opacity: 1},
		{Offset: 1, Color: NewColorLuma(255), Opacity: 1},
	})
	surface.SetFill(&Fill{Paint: gradient, Opacity: 1})
	surface.DrawPath(rectToPath(t, 20, 20, 180, 180))
	checkSnapshot(t, "pattern_luma_stops", d)
}

// snapshotStream matches a stream object of a snapshot: its dictionary and
// its data.
var snapshotStream = regexp.MustCompile(`(?s)<<\n([^\n]*(?:\n  [^\n]*)*)\n>>\nstream\n(.*?)\nendstream`)

// snapshotLength matches the line of the key /Length.
var snapshotLength = regexp.MustCompile(`(?m)^  /Length \d+\n`)

// normalizeSnapshot decodes each stream with the filters ASCIIHexDecode and
// FlateDecode, and removes the keys /Length, the table of cross references
// and the trailer. The compressed bytes of okage differ from the bytes of
// flate2, and the offsets and the ID of the document differ with them (D62).
func normalizeSnapshot(t *testing.T, data []byte) string {
	t.Helper()
	s := string(data)
	if i := strings.Index(s, "\nxref\n"); i >= 0 {
		s = s[:i]
	}
	s = snapshotStream.ReplaceAllStringFunc(s, func(m string) string {
		sub := snapshotStream.FindStringSubmatch(m)
		dict, body := sub[1], sub[2]
		if !strings.Contains(dict, "/Filter [/ASCIIHexDecode /FlateDecode]") {
			return m
		}
		raw, err := hex.DecodeString(strings.ReplaceAll(body, "\n", ""))
		if err != nil {
			t.Fatal(err)
		}
		r, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return "<<\n" + dict + "\n>>\nstream\n" + hex.EncodeToString(decoded) + "\nendstream"
	})
	return snapshotLength.ReplaceAllString(s, "")
}

// checkSnapshotDecoded compares the file with the snapshot of krilla after
// normalizeSnapshot, for a snapshot with a compressed stream, such as an
// ICC profile.
func checkSnapshotDecoded(t *testing.T, name string, d *Document) {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshots", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, wantS := normalizeSnapshot(t, buf.Bytes()), normalizeSnapshot(t, want)
	if got == wantS {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(wantS, "\n")
	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("%s differs from the decoded snapshot at the line %d:\ngot  %.200q\nwant %.200q", name, i+1, g, w)
		}
	}
}
