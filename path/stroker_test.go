// Copyright 2008 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path

import (
	"math"
	"testing"
)

// newMoveTo is PathSegment::new_move_to of the tests in tiny-skia-path
// path/src/stroker.rs.
func newMoveTo(x, y float32) PathSegment {
	return PathSegment{Kind: PathSegmentMoveTo, Points: [3]Point{PointFromXY(x, y)}}
}

// newLineTo is PathSegment::new_line_to of the tests in tiny-skia-path
// path/src/stroker.rs.
func newLineTo(x, y float32) PathSegment {
	return PathSegment{Kind: PathSegmentLineTo, Points: [3]Point{PointFromXY(x, y)}}
}

// newClose is PathSegment::new_close of the tests in tiny-skia-path
// path/src/stroker.rs.
func newClose() PathSegment {
	return PathSegment{Kind: PathSegmentClose}
}

func bits(b uint32) float32 {
	return math.Float32frombits(b)
}

// TestAutoClose is the test auto_close in tiny-skia-path path/src/stroker.rs.
//
// Make sure that subpath auto-closing is enabled.
func TestAutoClose(t *testing.T) {
	// A triangle.
	pb := NewPathBuilder()
	pb.MoveTo(10.0, 10.0)
	pb.LineTo(20.0, 50.0)
	pb.LineTo(30.0, 10.0)
	pb.Close()
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	stroke := DefaultStroke()
	strokePath, ok := NewPathStroker().Stroke(path, &stroke, 1.0)
	if !ok {
		t.Fatalf("Stroke is not ok")
	}

	iter := strokePath.Segments()
	iter.SetAutoClose(true)

	want := []PathSegment{
		newMoveTo(10.485071, 9.878732),
		newLineTo(20.485071, 49.878731),
		newLineTo(20.0, 50.0),
		newLineTo(19.514929, 49.878731),
		newLineTo(29.514929, 9.878732),
		newLineTo(30.0, 10.0),
		newLineTo(30.0, 10.5),
		newLineTo(10.0, 10.5),
		newLineTo(10.0, 10.0),
		newLineTo(10.485071, 9.878732),
		newClose(),
		newMoveTo(9.3596115, 9.5),
		newLineTo(30.640388, 9.5),
		newLineTo(20.485071, 50.121269),
		newLineTo(19.514929, 50.121269),
		newLineTo(9.514929, 10.121268),
		newLineTo(9.3596115, 9.5),
		newClose(),
	}
	for i, w := range want {
		got, ok := iter.Next()
		if !ok {
			t.Fatalf("segment %d: Next is not ok", i)
		}
		if got != w {
			t.Errorf("segment %d = %+v, want %+v", i, got, w)
		}
	}
}

// TestCubic1 is the test cubic_1 in tiny-skia-path path/src/stroker.rs.
//
// From skia/tests/StrokeTest.cpp.
func TestCubic1(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(51.0161362, 1511.52478)
	pb.CubicTo(
		51.0161362, 1511.52478,
		51.0161362, 1511.52478,
		51.0161362, 1511.52478,
	)
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	stroke := DefaultStroke()
	stroke.Width = 0.394537568

	if _, ok := NewPathStroker().Stroke(path, &stroke, 1.0); ok {
		t.Errorf("Stroke is ok, want false")
	}
}

// TestCubic2 is the test cubic_2 in tiny-skia-path path/src/stroker.rs.
//
// From skia/tests/StrokeTest.cpp.
func TestCubic2(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(bits(0x424c1086), bits(0x44bcf0cb)) // 51.0161362, 1511.52478
	pb.CubicTo(
		bits(0x424c107c), bits(0x44bcf0cb), // 51.0160980, 1511.52478
		bits(0x424c10c2), bits(0x44bcf0cb), // 51.0163651, 1511.52478
		bits(0x424c1119), bits(0x44bcf0ca), // 51.0166969, 1511.52466
	)
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	stroke := DefaultStroke()
	stroke.Width = 0.394537568

	if _, ok := NewPathStroker().Stroke(path, &stroke, 1.0); !ok {
		t.Errorf("Stroke is not ok, want ok")
	}
}

// TestBig is the test big in tiny-skia-path path/src/stroker.rs.
//
// From skia/tests/StrokeTest.cpp
// From skbug.com/6491. The large stroke width can cause numerical instabilities.
func TestBig(t *testing.T) {
	// Skia uses `kStrokeAndFill_Style` here, but we do not support it.

	pb := NewPathBuilder()
	pb.MoveTo(bits(0x46380000), bits(0xc6380000)) // 11776, -11776
	pb.LineTo(bits(0x46a00000), bits(0xc6a00000)) // 20480, -20480
	pb.LineTo(bits(0x468c0000), bits(0xc68c0000)) // 17920, -17920
	pb.LineTo(bits(0x46100000), bits(0xc6100000)) // 9216, -9216
	pb.LineTo(bits(0x46380000), bits(0xc6380000)) // 11776, -11776
	pb.Close()
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	stroke := DefaultStroke()
	stroke.Width = 1.49679073e+10

	if _, ok := NewPathStroker().Stroke(path, &stroke, 1.0); !ok {
		t.Errorf("Stroke is not ok, want ok")
	}
}

// TestQuadStrokerOneOff is the test quad_stroker_one_off in tiny-skia-path
// path/src/stroker.rs.
//
// From skia/tests/StrokerTest.cpp.
func TestQuadStrokerOneOff(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(bits(0x43c99223), bits(0x42b7417e))
	pb.QuadTo(
		bits(0x4285d839), bits(0x43ed6645),
		bits(0x43c941c8), bits(0x42b3ace3),
	)
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	stroke := DefaultStroke()
	stroke.Width = 164.683548

	if _, ok := NewPathStroker().Stroke(path, &stroke, 1.0); !ok {
		t.Errorf("Stroke is not ok, want ok")
	}
}

// TestCubicStrokerOneOff is the test cubic_stroker_one_off in tiny-skia-path
// path/src/stroker.rs.
//
// From skia/tests/StrokerTest.cpp.
func TestCubicStrokerOneOff(t *testing.T) {
	pb := NewPathBuilder()
	pb.MoveTo(bits(0x433f5370), bits(0x43d1f4b3))
	pb.CubicTo(
		bits(0x4331cb76), bits(0x43ea3340),
		bits(0x4388f498), bits(0x42f7f08d),
		bits(0x43f1cd32), bits(0x42802ec1),
	)
	path, ok := pb.Finish()
	if !ok {
		t.Fatalf("Finish is not ok")
	}

	stroke := DefaultStroke()
	stroke.Width = 42.835968

	if _, ok := NewPathStroker().Stroke(path, &stroke, 1.0); !ok {
		t.Errorf("Stroke is not ok, want ok")
	}
}
