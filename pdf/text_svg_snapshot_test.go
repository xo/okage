// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf_test

import (
	"testing"

	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
	"github.com/xo/okage/pdfsvg"
)

// TestFontType3 is the snapshot tests font_type3_color_glyphs,
// font_type3_pdf_14 and font_type3_with_not_def of krilla
// tests/src/font.rs. krilla draws the SVG glyphs of Twitter Color Emoji with
// render_svg_glyph of krilla-svg, as the settings of its tests do. The port
// draws them with pdfsvg.RenderGlyph, so the test is in the package
// pdf_test, which can import pdfsvg.
func TestFontType3(t *testing.T) {
	for _, c := range []struct {
		name string
		opts []pdf.Option
		y    float32
		size float32
		text string
	}{
		{"font_type3_color_glyphs", nil, 25, 25, "😀😃"},
		{"font_type3_pdf_14", []pdf.Option{pdf.WithPDFVersion(pdf.PDF14)}, 25, 25, "😀"},
		{"font_type3_with_not_def", nil, 100, 32, "H😄"},
	} {
		t.Run(c.name, func(t *testing.T) {
			opts := append([]pdf.Option{pdf.WithSVGGlyphs(pdfsvg.RenderGlyph)}, c.opts...)
			d, s := pdf.TextSnapshotPage(t, opts...)
			s.DrawText(path.PointFromXY(0, c.y), pdf.LoadTestFont(t, "TwitterColorEmoji.subset.ttf"), c.size, c.text, false, pdf.TextDirectionAuto)
			pdf.CheckSnapshotDecoded(t, c.name, d)
		})
	}
}
