package path

// This file ports no upstream file. It holds the tests of the bounds of
// points that are not finite (D107).

import (
	"math"
	"testing"
)

// TestRectFromPointsNotFinite checks that RectFromPoints returns false for
// a NaN or an infinity in the first point, the second point and a later
// point, for each count of points from 1 to 6.
func TestRectFromPointsNotFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for n := 1; n <= 6; n++ {
		points := make([]Point, n)
		for i := range points {
			points[i] = PointFromXY(float32(i), float32(2*i))
		}
		if _, ok := RectFromPoints(points); !ok {
			t.Errorf("%d finite points: expected a rect", n)
		}
		for i := range n {
			for _, v := range []float32{nan, inf, -inf} {
				for axis := range 2 {
					bad := append([]Point(nil), points...)
					if axis == 0 {
						bad[i].X = v
					} else {
						bad[i].Y = v
					}
					if r, ok := RectFromPoints(bad); ok {
						t.Errorf("%d points with %v in the axis %d of the point %d: expected no rect, got %v", n, v, axis, i, r)
					}
				}
			}
		}
	}
}

// TestPathBuilderNotFinite checks that Finish returns false for the path of
// the fuzz test FuzzEncode of ivy, whose second point holds a NaN.
func TestPathBuilderNotFinite(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(109, 19)
	pb.QuadTo(-15, float32(math.NaN()), -6, 125)
	pb.CubicTo(18, 3, -9, 36, -5, 228)
	pb.Close()
	if p, ok := pb.Finish(); ok {
		t.Errorf("expected no path, got the bounds %v", p.Bounds())
	}
}

// TestPathTransformNotFinite checks that a transform that makes the first
// point NaN gives no path. Each product overflows to an infinity, and their
// sum is NaN.
func TestPathTransformNotFinite(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(1e30, 1e30)
	pb.LineTo(0, 0)
	pb.LineTo(10, 0)
	pb.Close()
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("expected a path")
	}
	ts := TransformFromRow(1e10, 0, -1e10, 1, 0, 0)
	if p, ok := p.Transform(ts); ok {
		t.Errorf("expected no path, got the bounds %v", p.Bounds())
	}
}
