package okage_test

// This file ports no upstream test. It checks Paint.StrokeAdjust, the
// automatic stroke adjustment of PDF that okage adds to tiny-skia (D54).

import (
	"fmt"
	"testing"

	"github.com/xo/okage"
)

// alphaColumn returns the alpha of each pixel of the column x of p.
func alphaColumn(p *okage.Pixmap, x uint32) []uint8 {
	col := make([]uint8, p.Height())
	for y := range p.Height() {
		col[y] = p.Data()[(y*p.Width()+x)*4+3]
	}
	return col
}

// fullRows returns the column col as a string, with "#" for a pixel of full
// coverage, "." for an empty pixel and "?" for any other.
func fullRows(col []uint8) string {
	s := make([]byte, len(col))
	for i, a := range col {
		switch a {
		case 255:
			s[i] = '#'
		case 0:
			s[i] = '.'
		default:
			s[i] = '?'
		}
	}
	return string(s)
}

// TestStrokeAdjustHorizontal checks that a horizontal line with stroke
// adjustment covers whole rows: as many as the width on the device, rounded,
// and at least 1.
func TestStrokeAdjustHorizontal(t *testing.T) {
	tests := []struct {
		y, width, scale float32
		want            string
	}{
		{10, 0, 1, "..........#........."},
		{10, 0.3, 1, "..........#........."},
		{10.3, 0.5, 1, "..........#........."},
		{10.5, 0.5, 1, "..........#........."},
		{10.9, 1, 1, "..........#........."},
		{10, 1.6, 1, ".........##........."},
		{10.3, 1.6, 1, ".........##........."},
		{10.6, 1.6, 1, "..........##........"},
		{10.3, 2.5, 1, ".........###........"},
		// 0.4 at the scale 2 is 0.8 pixels, and 0.8 at the scale 2 is 1.6.
		{5.2, 0.4, 2, "..........#........."},
		{5.2, 0.8, 2, ".........##........."},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("y=%v width=%v scale=%v", tt.y, tt.width, tt.scale), func(t *testing.T) {
			for _, aa := range []bool{true, false} {
				paint := okage.DefaultPaint()
				paint.AntiAlias = aa
				paint.StrokeAdjust = true
				pb := okage.NewPathBuilder()
				pb.MoveTo(1/tt.scale, tt.y)
				pb.LineTo(19/tt.scale, tt.y)
				stroke := okage.DefaultStroke()
				stroke.Width = tt.width
				pixmap := must(okage.NewPixmap(20, 20))
				pixmap.StrokePath(must(pb.Finish()), &paint, &stroke, okage.TransformFromScale(tt.scale, tt.scale), nil)
				if got := fullRows(alphaColumn(pixmap, 10)); got != tt.want {
					t.Errorf("aa=%v: expected the rows %s, got %s", aa, tt.want, got)
				}
			}
		})
	}
}

// TestStrokeAdjustVertical checks that a vertical line with stroke
// adjustment covers whole columns.
func TestStrokeAdjustVertical(t *testing.T) {
	paint := okage.DefaultPaint()
	paint.StrokeAdjust = true
	pb := okage.NewPathBuilder()
	pb.MoveTo(7.3, 2)
	pb.LineTo(7.3, 18)
	stroke := okage.DefaultStroke()
	stroke.Width = 0.5
	pixmap := must(okage.NewPixmap(20, 20))
	pixmap.StrokePath(must(pb.Finish()), &paint, &stroke, okage.IdentityTransform(), nil)
	row := make([]uint8, 20)
	for x := range uint32(20) {
		row[x] = pixmap.Data()[(10*20+x)*4+3]
	}
	if got, want := fullRows(row), ".......#............"; got != want {
		t.Errorf("expected the columns %s, got %s", want, got)
	}
}

// TestStrokeAdjustRect checks that each pixel of a thin rectangle with
// stroke adjustment has full coverage or none.
func TestStrokeAdjustRect(t *testing.T) {
	path := okage.PathFromRect(must(okage.RectFromLTRB(3.3, 4.6, 15.8, 12.2)))
	paint := okage.DefaultPaint()
	paint.StrokeAdjust = true
	stroke := okage.DefaultStroke()
	stroke.Width = 0.3
	pixmap := must(okage.NewPixmap(20, 20))
	pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)
	full := 0
	for i := 3; i < len(pixmap.Data()); i += 4 {
		switch pixmap.Data()[i] {
		case 255:
			full++
		case 0:
		default:
			t.Fatalf("expected full coverage or none, got %d at pixel %d", pixmap.Data()[i], i/4)
		}
	}
	// The sides are 13 and 9 pixels long, from the columns 3 to 15 and the
	// rows 4 to 12, and the corners are shared.
	if want := 2*13 + 2*9 - 4; full != want {
		t.Errorf("expected %d pixels, got %d", want, full)
	}
}

// TestStrokeAdjustDash checks that a dashed line with stroke adjustment
// keeps its dashes, in the units of the path.
func TestStrokeAdjustDash(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(0, 5.3)
	pb.LineTo(20, 5.3)
	paint := okage.DefaultPaint()
	paint.StrokeAdjust = true
	stroke := okage.DefaultStroke()
	stroke.Width = 0.25
	stroke.Dash = must(okage.NewStrokeDash([]float32{2, 2}, 0))
	pixmap := must(okage.NewPixmap(40, 20))
	pixmap.StrokePath(must(pb.Finish()), &paint, &stroke, okage.TransformFromScale(2, 2), nil)
	row := make([]uint8, 40)
	for x := range uint32(40) {
		row[x] = pixmap.Data()[(10*40+x)*4+3]
	}
	if got, want := fullRows(row), "####....####....####....####....####...."; got != want {
		t.Errorf("expected the dashes %s, got %s", want, got)
	}
}

// TestStrokeAdjustOff checks that a paint without stroke adjustment draws
// a thin line over two light rows, as tiny-skia does.
func TestStrokeAdjustOff(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(1, 10)
	pb.LineTo(19, 10)
	paint := okage.DefaultPaint()
	stroke := okage.DefaultStroke()
	stroke.Width = 0.5
	pixmap := must(okage.NewPixmap(20, 20))
	pixmap.StrokePath(must(pb.Finish()), &paint, &stroke, okage.IdentityTransform(), nil)
	col := alphaColumn(pixmap, 10)
	if col[9] != 64 || col[10] != 64 {
		t.Errorf("expected the alphas 64 and 64, got %d and %d", col[9], col[10])
	}
}
