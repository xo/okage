// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package path_test

// This file ports tiny-skia tests/integration/path.rs. The test is of a path
// only, so it is in the package path.

import (
	"math"
	"slices"
	"testing"

	"github.com/xo/okage/path"
)

func TestPathEmpty(t *testing.T) {
	pb := path.NewPathBuilder()
	if _, ok := pb.Finish(); ok {
		t.Error("Finish returned Some, want None")
	}
}

func TestPathLine(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 20, 30, 40)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 20)),
		lineTo(path.PointFromXY(30, 40)),
	})

	if got, want := p.String(), "Path { segments: \"M 10 20 L 30 40\", bounds: Rect { left: 10.0, top: 20.0, right: 30.0, bottom: 40.0 } }"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPathNoMoveBeforeLine(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(0, 0, 30, 40)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(0, 0)),
		lineTo(path.PointFromXY(30, 40)),
	})
}

func TestPathNoMoveBeforeQuad(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.QuadTo(40, 30, 60, 75)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(0, 0, 60, 75)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(0, 0)),
		quadTo(path.PointFromXY(40, 30), path.PointFromXY(60, 75)),
	})
}

func TestPathNoMoveBeforeCubic(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.CubicTo(40, 30, 60, 75, 33, 66)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(0, 0, 60, 75)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(0, 0)),
		cubicTo(path.PointFromXY(40, 30), path.PointFromXY(60, 75), path.PointFromXY(33, 66)),
	})
}

func TestPathNoMoveBeforeClose(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.Close()
	if _, ok := pb.Finish(); ok {
		t.Error("Finish returned Some, want None")
	}
}

func TestPathDoubleClose(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 10)
	pb.LineTo(20, 10)
	pb.LineTo(20, 20)
	pb.Close()
	pb.Close()
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 10, 20, 20)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 10)),
		lineTo(path.PointFromXY(20, 10)),
		lineTo(path.PointFromXY(20, 20)),
		closeSegment(),
	})
}

func TestPathDoubleMoveTo1(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.MoveTo(30, 40)
	if _, ok := pb.Finish(); ok {
		t.Error("Finish returned Some, want None")
	}
}

func TestPathDoubleMoveTo2(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.MoveTo(20, 10)
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(20, 10, 30, 40)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(20, 10)),
		lineTo(path.PointFromXY(30, 40)),
	})
}

func TestPathTwoContours(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	pb.MoveTo(100, 200)
	pb.LineTo(300, 400)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 20, 300, 400)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 20)),
		lineTo(path.PointFromXY(30, 40)),
		moveTo(path.PointFromXY(100, 200)),
		lineTo(path.PointFromXY(300, 400)),
	})
}

func TestPathTwoClosedContours(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	pb.Close()
	pb.MoveTo(100, 200)
	pb.LineTo(300, 400)
	pb.Close()
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 20, 300, 400)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 20)),
		lineTo(path.PointFromXY(30, 40)),
		closeSegment(),
		moveTo(path.PointFromXY(100, 200)),
		lineTo(path.PointFromXY(300, 400)),
		closeSegment(),
	})
}

func TestPathLineAfterClose(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	pb.Close()
	pb.LineTo(20, 20)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 20, 30, 40)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 20)),
		lineTo(path.PointFromXY(30, 40)),
		closeSegment(),
		moveTo(path.PointFromXY(10, 20)),
		lineTo(path.PointFromXY(20, 20)),
	})
}

func TestPathHorLine(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 10)
	pb.LineTo(20, 10)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 10, 20, 10)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 10)),
		lineTo(path.PointFromXY(20, 10)),
	})
}

func TestPathVerLine(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 10)
	pb.LineTo(10, 20)
	p := must(pb.Finish())

	assertRect(t, p.Bounds(), must(path.RectFromLTRB(10, 10, 10, 20)))
	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(10, 10)),
		lineTo(path.PointFromXY(10, 20)),
	})
}

func TestPathTranslate(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	p = must(p.Transform(path.TransformFromTranslate(10, 20)))

	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(20, 40)),
		lineTo(path.PointFromXY(40, 60)),
	})
}

func TestPathScale(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	p = must(p.Transform(path.TransformFromScale(2, 0.5)))

	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(20, 10)),
		lineTo(path.PointFromXY(60, 20)),
	})
}

func TestPathTransform(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	p = must(p.Transform(path.TransformFromRow(2, 0.7, -0.3, 0.5, 10, 20)))

	assertSegments(t, p, []path.PathSegment{
		moveTo(path.PointFromXY(24, 37)),
		lineTo(path.PointFromXY(58, 61)),
	})
}

func TestPathInvalidTransform(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(10, 20)
	pb.LineTo(30, 40)
	p := must(pb.Finish())

	// will produce infinity
	if _, ok := p.Transform(path.TransformFromScale(math.MaxFloat32, math.MaxFloat32)); ok {
		t.Error("Transform returned Some, want None")
	}
}

func TestPathCircle(t *testing.T) {
	if _, ok := path.PathFromCircle(250, 250, 300); !ok { // Must not panic.
		t.Error("PathFromCircle returned None, want Some")
	}
}

func TestPathLargeCircle(t *testing.T) {
	if _, ok := path.PathFromCircle(250, 250, 2000); !ok { // Must not panic.
		t.Error("PathFromCircle returned None, want Some")
	}
}

func TestPathTightBounds1(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(50, 85)
	pb.LineTo(65, 135)
	pb.LineTo(150, 135)
	pb.LineTo(85, 135)
	pb.QuadTo(100, 45, 50, 85)
	p := must(pb.Finish())
	tightBounds := must(p.ComputeTightBounds())
	assertRect(t, p.Bounds(), must(path.RectFromXYWH(50, 45, 100, 90)))
	assertRect(t, tightBounds, must(path.RectFromXYWH(50, 72.692307, 100, 62.307693)))
}

func TestPathTightBounds2(t *testing.T) {
	pb := path.NewPathBuilder()
	pb.MoveTo(-19.309214, 72.11173)
	pb.CubicTo(-24.832062, 67.477516, -20.490944, 62.16584, -9.61306, 60.247776)
	pb.CubicTo(1.2648277, 58.329712, 14.560249, 60.53159, 20.083096, 65.16581)
	pb.CubicTo(14.560249, 60.53159, 18.901363, 55.219913, 29.779247, 53.30185)
	pb.CubicTo(40.65713, 51.383785, 53.952557, 53.585663, 59.475407, 58.21988)
	pb.QuadTo(74.4754, 70.80637, 50.083096, 90.3388)
	pb.QuadTo(-4.3092155, 84.69823, -19.309214, 72.11173)
	pb.Close()
	p := must(pb.Finish())
	tightBounds := must(p.ComputeTightBounds())
	assertRect(t, tightBounds, must(path.RectFromXYWH(-21.707121, 52.609154, 86.894302, 37.729645)))
}

// must is the unwrap of an Option in Rust. It returns v, and it panics if ok
// is false, as unwrap does.
func must[T any](v T, ok bool) T {
	if !ok {
		panic("called unwrap on a None value")
	}
	return v
}

// moveTo is PathSegment::MoveTo.
func moveTo(p path.Point) path.PathSegment {
	return path.PathSegment{Kind: path.PathSegmentMoveTo, Points: [3]path.Point{p}}
}

// lineTo is PathSegment::LineTo.
func lineTo(p path.Point) path.PathSegment {
	return path.PathSegment{Kind: path.PathSegmentLineTo, Points: [3]path.Point{p}}
}

// quadTo is PathSegment::QuadTo.
func quadTo(p0, p1 path.Point) path.PathSegment {
	return path.PathSegment{Kind: path.PathSegmentQuadTo, Points: [3]path.Point{p0, p1}}
}

// cubicTo is PathSegment::CubicTo.
func cubicTo(p0, p1, p2 path.Point) path.PathSegment {
	return path.PathSegment{Kind: path.PathSegmentCubicTo, Points: [3]path.Point{p0, p1, p2}}
}

// closeSegment is PathSegment::Close.
func closeSegment() path.PathSegment {
	return path.PathSegment{Kind: path.PathSegmentClose}
}

// assertSegments is assert_eq! for the segments of a path and a list of
// segments.
func assertSegments(t *testing.T, p *path.Path, want []path.PathSegment) {
	t.Helper()
	var got []path.PathSegment
	for s := range p.Segments().All() {
		got = append(got, s)
	}
	if !slices.Equal(got, want) {
		t.Errorf("got segments %v, want %v", got, want)
	}
}

// assertRect is assert_eq! for two rectangles.
func assertRect(t *testing.T, got, want path.Rect) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
