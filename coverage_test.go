package okage_test

// This file ports no upstream test. It checks Paint.Coverage, the grid of 8
// by 8 samples that okage adds to tiny-skia (D54).

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/xo/okage"
)

// TestCoverageDefault checks that the default and the zero value of
// Paint.Coverage are the grid of tiny-skia, and that the grid of tiny-skia
// gives the reference image of tiny-skia.
func TestCoverageDefault(t *testing.T) {
	if c := okage.DefaultPaint().Coverage; c != okage.Coverage4x4 {
		t.Errorf("expected Coverage4x4 as the default, got %v", c)
	}
	var paint okage.Paint
	if paint.Coverage != okage.Coverage4x4 {
		t.Errorf("expected Coverage4x4 as the zero value, got %v", paint.Coverage)
	}

	// The star of TestFillFillAa of fill_integration_test.go.
	paint = okage.DefaultPaint()
	paint.SetColorRGBA8(50, 127, 150, 200)
	paint.Coverage = okage.Coverage4x4
	pb := okage.NewPathBuilder()
	pb.MoveTo(50, 7.5)
	pb.LineTo(75, 87.5)
	pb.LineTo(10, 37.5)
	pb.LineTo(90, 37.5)
	pb.LineTo(25, 87.5)
	path := must(pb.Finish())
	pixmap := must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)
	expected := check(okage.LoadPNG("testdata/images/fill/star-aa.png"))
	assertPixmapEqual(t, pixmap, expected)

	// The same star with Coverage8x8 differs.
	paint.Coverage = okage.Coverage8x8
	pixmap = must(okage.NewPixmap(100, 100))
	pixmap.FillPath(path, &paint, okage.FillRuleEvenOdd, okage.IdentityTransform(), nil)
	if string(pixmap.Data()) == string(expected.Data()) {
		t.Error("expected Coverage8x8 to change the bytes of the star")
	}
}

// TestCoverage8x8Edge checks that the coverage of a pixel that a vertical
// edge crosses moves in steps of 1/8 with Coverage8x8, and that it is closer
// to the area than the coverage of Coverage4x4.
func TestCoverage8x8Edge(t *testing.T) {
	maxErr := map[okage.Coverage]float32{}
	for i := range 32 {
		f := float32(i) / 32
		for _, c := range []okage.Coverage{okage.Coverage4x4, okage.Coverage8x8} {
			paint := okage.DefaultPaint()
			paint.Coverage = c
			path := okage.PathFromRect(must(okage.RectFromLTRB(10+f, 10, 20, 20)))
			pixmap := must(okage.NewPixmap(30, 30))
			pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
			got := float32(pixmap.Data()[(15*30+10)*4+3])
			want := (1 - f) * 255
			maxErr[c] = max(maxErr[c], max(got-want, want-got))
		}
	}
	// A grid of n samples in a row gives an error of at most half a step.
	if e := maxErr[okage.Coverage8x8]; e > 255.0/16+1 {
		t.Errorf("expected an error of at most 1/16 with Coverage8x8, got %v", e/255)
	}
	if e4, e8 := maxErr[okage.Coverage4x4], maxErr[okage.Coverage8x8]; e8 >= e4 {
		t.Errorf("expected Coverage8x8 to be closer than Coverage4x4, got %v and %v", e8/255, e4/255)
	}
}

// TestCoverage8x8Bytes checks the bytes of a fill with Coverage8x8 of curves
// and lines, with the two fill rules and a partial clip. The scalar build and
// the SIMD build give the same bytes.
func TestCoverage8x8Bytes(t *testing.T) {
	tests := []struct {
		rule okage.FillRule
		want string
	}{
		{okage.FillRuleWinding, "5a2c2af5f02cfc72e49a526256574324201b15efc2c3e5d9b69db69d03358e86"},
		{okage.FillRuleEvenOdd, "06755af40a9d9bacf147985146930e123b705a24dc8e8b09000f1fb2b1709582"},
	}
	for _, tt := range tests {
		paint := okage.DefaultPaint()
		paint.SetColorRGBA8(50, 127, 150, 200)
		paint.Coverage = okage.Coverage8x8
		pb := okage.NewPathBuilder()
		pb.MoveTo(50, 2)
		pb.CubicTo(65, 32, 77, 65, 80, 98)
		pb.LineTo(2, 38)
		pb.LineTo(20, 98)
		pb.CubicTo(23, 65, 35, 32, 50, 2)
		pb.Close()
		pb.PushCircle(60, 60, 30.3)
		path := must(pb.Finish())
		// The circle reaches past the right edge of the pixmap.
		pixmap := must(okage.NewPixmap(85, 100))
		pixmap.FillPath(path, &paint, tt.rule, okage.IdentityTransform(), nil)
		sum := sha256.Sum256(pixmap.Data())
		if got := hex.EncodeToString(sum[:]); got != tt.want {
			t.Errorf("%v: expected the bytes %s, got %s", tt.rule, tt.want, got)
		}
	}
}

// TestCoverage8x8Large checks that a fill wider than 4095 pixels takes the
// grid of tiny-skia with Coverage8x8.
func TestCoverage8x8Large(t *testing.T) {
	path := okage.PathFromRect(must(okage.RectFromLTRB(0.3, 0.3, 4999.6, 3.7)))
	var data [2][]byte
	for i, c := range []okage.Coverage{okage.Coverage4x4, okage.Coverage8x8} {
		paint := okage.DefaultPaint()
		paint.Coverage = c
		pixmap := must(okage.NewPixmap(5000, 4))
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
		data[i] = pixmap.Data()
	}
	if string(data[0]) != string(data[1]) {
		t.Error("expected the bytes of Coverage4x4")
	}
}

// TestCoverage8x8Stroke checks that a stroke that StrokePath fills takes the
// coverage of the paint, and that a hairline does not.
func TestCoverage8x8Stroke(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.MoveTo(10.3, 10.3)
	pb.LineTo(40.7, 30.1)
	path := must(pb.Finish())
	draw := func(c okage.Coverage, width float32) []byte {
		paint := okage.DefaultPaint()
		paint.Coverage = c
		stroke := okage.DefaultStroke()
		stroke.Width = width
		pixmap := must(okage.NewPixmap(50, 50))
		pixmap.StrokePath(path, &paint, &stroke, okage.IdentityTransform(), nil)
		return pixmap.Data()
	}
	if string(draw(okage.Coverage4x4, 3)) == string(draw(okage.Coverage8x8, 3)) {
		t.Error("expected a stroke of 3 pixels to change with Coverage8x8")
	}
	if string(draw(okage.Coverage4x4, 0.5)) != string(draw(okage.Coverage8x8, 0.5)) {
		t.Error("expected a hairline to keep its bytes with Coverage8x8")
	}
}

// TestCoverage8x8Closer checks that a fill of curves with Coverage8x8 is
// closer to the area that the path covers than a fill with Coverage4x4. The
// area comes from a fill at 8 times the size, with 4 by 4 samples, which
// gives 32 by 32 samples in each pixel.
func TestCoverage8x8Closer(t *testing.T) {
	pb := okage.NewPathBuilder()
	pb.PushCircle(20.3, 20.6, 7.3)
	pb.PushCircle(40.1, 22.2, 3.2)
	pb.MoveTo(5.2, 45.1)
	pb.CubicTo(20, 30, 30, 60, 45.7, 33.4)
	pb.LineTo(46.2, 47.9)
	pb.Close()
	path := must(pb.Finish())

	const size, k = 50, 8
	paint := okage.DefaultPaint()
	big := must(okage.NewPixmap(size*k, size*k))
	big.FillPath(path, &paint, okage.FillRuleWinding, okage.TransformFromScale(k, k), nil)
	area := make([]float64, size*size)
	for y := range size * k {
		for x := range size * k {
			area[(y/k)*size+x/k] += float64(big.Data()[(y*size*k+x)*4+3]) / (k * k)
		}
	}

	diff := map[okage.Coverage]float64{}
	for _, c := range []okage.Coverage{okage.Coverage4x4, okage.Coverage8x8} {
		paint.Coverage = c
		pixmap := must(okage.NewPixmap(size, size))
		pixmap.FillPath(path, &paint, okage.FillRuleWinding, okage.IdentityTransform(), nil)
		for i, a := range area {
			d := float64(pixmap.Data()[i*4+3]) - a
			diff[c] += max(d, -d) / (size * size)
		}
	}
	if d4, d8 := diff[okage.Coverage4x4], diff[okage.Coverage8x8]; !(d8 < d4) {
		t.Errorf("expected Coverage8x8 to be closer to the area than Coverage4x4, got %v and %v", d8, d4)
	} else {
		t.Logf("mean difference from the area: %v with Coverage4x4, %v with Coverage8x8", d4, d8)
	}
}
