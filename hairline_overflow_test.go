package okage_test

// This file ports no upstream test. It checks a fault of tiny-skia that
// okage keeps (D16): a hairline without anti-aliasing can give a rect one
// column past the right side of the pixmap, and tiny-skia draws that pixel
// on the first column of the next row. On the last row, okage skips the
// pixel, where tiny-skia panics (D107).

import (
	"testing"

	"github.com/xo/okage"
)

// TestHairlinePastTheRightSide draws the hairline of the random scene 16312
// of the Rust oracle. Its pixel (13, 5) is past the right side of the
// pixmap, so tiny-skia draws the pixel (0, 6).
func TestHairlinePastTheRightSide(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(1.5595906972885132, 38.03263473510742)
	pb.QuadTo(12.012727737426758, 24, 11, 49.25)
	pb.LineTo(1.1404210329055786, 52.39041519165039)
	pb.LineTo(16.17230987548828, -1.75)
	pb.LineTo(12.25, 11.963037490844727)
	pb.LineTo(5.610728740692139, 50.93592834472656)
	pb.Close()
	pb.MoveTo(3.7186620235443115, -3.0876543521881104)
	pb.LineTo(13.5, 24.37908935546875)
	pb.LineTo(-0.10380573570728302, 18.5)
	pb.Close()
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("no path")
	}
	pixmap, _ := okage.NewPixmap(13, 49)
	paint := okage.DefaultPaint()
	paint.SetColorRGBA8(255, 0, 0, 255)
	paint.AntiAlias = false
	stroke := okage.DefaultStroke()
	stroke.Width = 0
	ts := okage.TransformFromRow(1.0756762027740479, 0, 0, 1.03444504737854, -2.5, -0.25591886043548584)
	pixmap.StrokePath(p, &paint, &stroke, ts, nil)
	if c, _ := pixmap.Pixel(0, 6); c.Alpha() != 255 || c.Red() != 255 {
		t.Errorf("pixel (0, 6): got %d %d %d %d, want 255 0 0 255", c.Red(), c.Green(), c.Blue(), c.Alpha())
	}
}

// TestHairlinePastTheLastRow draws a hairline whose pixel past the right side
// is on the last row of the pixmap, which FuzzFillPathHuge found. The next
// row is past the end of the pixmap, and tiny-skia panics. okage skips the
// pixel (D107).
func TestHairlinePastTheLastRow(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(-6e12, -7.0857142858321e13)
	pb.CubicTo(1.150000000038e12, 2.16e12, -3303, 64.2, 1154, 1019)
	pb.Close()
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("no path")
	}
	pixmap, _ := okage.NewPixmap(27, 27)
	paint := okage.DefaultPaint()
	paint.AntiAlias = false
	stroke := okage.DefaultStroke()
	stroke.Width = 0
	ts := okage.TransformFromScale(0.0036734692, 0.0036734692)
	pixmap.StrokePath(p, &paint, &stroke, ts, nil)
}
