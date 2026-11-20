package okage_test

// This file ports no upstream file. It holds benchmarks of the work that ivy
// gives okage: the clips of a page of a spreadsheet, an image that a raster
// page scales up, the composite of a group with each blend mode, and fills
// through a mask whose rows are mostly 0 (D35).

import (
	"testing"

	"github.com/xo/okage"
)

// The grid of cells of the spreadsheet benchmarks. A cell is 75 by 21 points,
// and the page maps a point to 4/3 pixels, as ivy does at 96 dpi. So the
// edges of most cells are not on the pixel grid.
const (
	benchCellColumns = 16
	benchCellRows    = 31
	benchCellWidth   = 75
	benchCellHeight  = 21
)

// benchPageTransform maps the points of a page to the pixels at 96 dpi.
func benchPageTransform() okage.Transform {
	return okage.TransformFromScale(4.0/3.0, 4.0/3.0)
}

// benchCellPath returns the clip of the cell at column i and row j. When
// round is true, the clip is a rectangle with round corners, and else it is
// a rectangle.
func benchCellPath(b *testing.B, i, j int, round bool) *okage.Path {
	b.Helper()
	x := float32(4 + i*benchCellWidth)
	y := float32(4 + j*benchCellHeight)
	if !round {
		return okage.PathFromRect(must(okage.RectFromXYWH(x, y, benchCellWidth, benchCellHeight)))
	}
	const r = 5
	pb := okage.NewPathBuilder()
	pb.MoveTo(x+r, y)
	pb.LineTo(x+benchCellWidth-r, y)
	pb.QuadTo(x+benchCellWidth, y, x+benchCellWidth, y+r)
	pb.LineTo(x+benchCellWidth, y+benchCellHeight-r)
	pb.QuadTo(x+benchCellWidth, y+benchCellHeight, x+benchCellWidth-r, y+benchCellHeight)
	pb.LineTo(x+r, y+benchCellHeight)
	pb.QuadTo(x, y+benchCellHeight, x, y+benchCellHeight-r)
	pb.LineTo(x, y+r)
	pb.QuadTo(x, y, x+r, y)
	pb.Close()
	return must(pb.Finish())
}

// benchTextPath returns a path that stands for the text of the cell at
// column i and row j: a row of small shapes, as glyphs are.
func benchTextPath(b *testing.B, i, j int) *okage.Path {
	b.Helper()
	x := float32(6 + i*benchCellWidth)
	y := float32(8 + j*benchCellHeight)
	pb := okage.NewPathBuilder()
	for k := range 12 {
		gx := x + float32(k*7)
		pb.MoveTo(gx, y+12)
		pb.LineTo(gx+2.5, y)
		pb.LineTo(gx+5.5, y+12)
		pb.LineTo(gx+4, y+12)
		pb.LineTo(gx+2.5, y+5)
		pb.LineTo(gx+1.5, y+12)
		pb.Close()
	}
	return must(pb.Finish())
}

// clipSpreadsheet draws a page of a spreadsheet as ivy does. The page clips
// to its area with a new mask. Each cell clones that mask, intersects the
// clone with the clip of the cell, and fills its background and its text
// through the clone.
func clipSpreadsheet(b *testing.B, round bool) {
	b.Helper()
	ts := benchPageTransform()
	type cell struct {
		clip, text *okage.Path
	}
	var cells []cell
	for j := range benchCellRows {
		for i := range benchCellColumns {
			cells = append(cells, cell{benchCellPath(b, i, j, round), benchTextPath(b, i, j)})
		}
	}
	area := okage.PathFromRect(must(okage.RectFromXYWH(2, 2, 1196, 665)))
	background := okage.DefaultPaint()
	background.AntiAlias = true
	text := okage.DefaultPaint()
	text.AntiAlias = true
	text.SetColorRGBA8(20, 20, 20, 255)
	pixmap := must(okage.NewPixmap(benchPageWidth, benchPageHeight))
	b.ReportAllocs()
	for b.Loop() {
		pixmap.Fill(okage.ColorWhite)
		page := must(okage.NewMask(benchPageWidth, benchPageHeight))
		page.FillPath(area, okage.FillRuleWinding, true, ts)
		for k, c := range cells {
			clip := page.Clone()
			clip.IntersectPath(c.clip, okage.FillRuleWinding, true, ts)
			background.SetColorRGBA8(uint8(200+k%50), 230, uint8(240-k%40), 255)
			pixmap.FillPath(c.clip, &background, okage.FillRuleWinding, ts, clip)
			pixmap.FillPath(c.text, &text, okage.FillRuleWinding, ts, clip)
		}
	}
}

// BenchmarkClipSpreadsheet draws a page of a spreadsheet whose cells clip
// to rectangles.
func BenchmarkClipSpreadsheet(b *testing.B) {
	clipSpreadsheet(b, false)
}

// BenchmarkClipSpreadsheetPath draws a page of a spreadsheet whose cells
// clip to rectangles with round corners.
func BenchmarkClipSpreadsheetPath(b *testing.B) {
	clipSpreadsheet(b, true)
}

// BenchmarkDrawPixmapUpscale draws a small image onto a page, scaled up by
// 8, as ivy draws a raster page, with each filter that samples more than one
// pixel.
func BenchmarkDrawPixmapUpscale(b *testing.B) {
	src := must(okage.NewPixmap(benchPageWidth/8, benchPageHeight/8+1))
	data := src.DataMut()
	for i := 0; i < len(data); i += okage.BytesPerPixel {
		v := uint8(i / okage.BytesPerPixel * 13)
		data[i], data[i+1], data[i+2], data[i+3] = v, 255-v, v/3, 255
	}
	for _, q := range []okage.FilterQuality{okage.FilterQualityBilinear, okage.FilterQualityBicubic} {
		b.Run(q.String(), func(b *testing.B) {
			paint := okage.DefaultPixmapPaint()
			paint.Quality = q
			drawPixmap(b, src, paint, okage.TransformFromScale(8, 8), nil)
		})
	}
}

// BenchmarkCompositeBlend draws a transparency group onto a page with each
// blend mode, as ivy composites a group of a PDF file. A pattern runs in the
// high precision pipeline (highp).
func BenchmarkCompositeBlend(b *testing.B) {
	layer := benchLayer(b)
	for mode := okage.BlendModeClear; mode <= okage.BlendModeLuminosity; mode++ {
		b.Run(mode.String(), func(b *testing.B) {
			paint := okage.DefaultPixmapPaint()
			paint.BlendMode = mode
			drawPixmap(b, layer, paint, okage.IdentityTransform(), nil)
		})
	}
}

// BenchmarkFillMaskZeroRows fills and strokes large paths through a mask
// that is 0 on all rows but a band of 30 rows, as a fill through the clip of
// a cell is.
func BenchmarkFillMaskZeroRows(b *testing.B) {
	fillMaskZeroRows(b, false)
}

// BenchmarkFillMaskZeroRowsDense is BenchmarkFillMaskZeroRows with a mask
// whose data the caller holds, as a soft mask from MaskFromPixmap is. Such
// a mask holds every row (D35).
func BenchmarkFillMaskZeroRowsDense(b *testing.B) {
	fillMaskZeroRows(b, true)
}

// fillMaskZeroRows fills and strokes large paths through a mask that is 0
// on all rows but a band of 30 rows. When dense is true, the mask holds
// every row.
func fillMaskZeroRows(b *testing.B, dense bool) {
	b.Helper()
	mask := must(okage.NewMask(benchPageWidth, benchPageHeight))
	if dense {
		mask.DataMut()
	}
	band := okage.PathFromRect(must(okage.RectFromXYWH(100.5, 430.25, 1400, 30)))
	mask.FillPath(band, okage.FillRuleWinding, true, okage.IdentityTransform())
	circle := must(okage.PathFromCircle(800, 446, 430))
	pb := okage.NewPathBuilder()
	for i := range 40 {
		pb.MoveTo(float32(i*40), 0)
		pb.LineTo(float32(i*40+300), benchPageHeight)
	}
	lines := must(pb.Finish())
	paint := okage.DefaultPaint()
	paint.AntiAlias = true
	paint.SetColorRGBA8(50, 127, 150, 200)
	hairline := okage.DefaultStroke()
	hairline.Width = 0.5
	wide := okage.DefaultStroke()
	wide.Width = 3
	pixmap := must(okage.NewPixmap(benchPageWidth, benchPageHeight))
	b.ReportAllocs()
	for b.Loop() {
		pixmap.FillPath(circle, &paint, okage.FillRuleWinding, okage.IdentityTransform(), mask)
		pixmap.StrokePath(lines, &paint, &hairline, okage.IdentityTransform(), mask)
		pixmap.StrokePath(lines, &paint, &wide, okage.IdentityTransform(), mask)
	}
}
