// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import (
	"math"
	"testing"
)

// The tests of svgtypes src/path.rs.

func TestPathParser(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []PathSegment
	}{
		{"null", "", nil},
		{"not_a_path", "q", nil},
		{"not_a_move_to", "L 20 30", nil},
		{
			"stop_on_err_1", "M 10 20 L 30 40 L 50",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
			},
		},
		{
			"move_to_1", "M 10 20",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
			},
		},
		{
			"move_to_2", "m 10 20",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: false, X: 10.0, Y: 20.0},
			},
		},
		{
			"move_to_3", "M 10 20 30 40 50 60",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 50.0, Y: 60.0},
			},
		},
		{
			"move_to_4", "M 10 20 30 40 50 60 M 70 80 90 100 110 120",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 50.0, Y: 60.0},
				{Kind: PathSegmentMoveTo, Abs: true, X: 70.0, Y: 80.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 90.0, Y: 100.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 110.0, Y: 120.0},
			},
		},
		{
			"arc_to_1", "M 10 20 A 5 5 30 1 1 20 20",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentEllipticalArc, Abs: true, Rx: 5.0, Ry: 5.0, XAxisRotation: 30.0, LargeArc: true, Sweep: true, X: 20.0, Y: 20.0},
			},
		},
		{
			"arc_to_2", "M 10 20 a 5 5 30 0 0 20 20",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentEllipticalArc, Abs: false, Rx: 5.0, Ry: 5.0, XAxisRotation: 30.0, LargeArc: false, Sweep: false, X: 20.0, Y: 20.0},
			},
		},
		{
			"arc_to_10", "M10-20A5.5.3-4 010-.1",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: -20.0},
				{Kind: PathSegmentEllipticalArc, Abs: true, Rx: 5.5, Ry: 0.3, XAxisRotation: -4.0, LargeArc: false, Sweep: true, X: 0.0, Y: -0.1},
			},
		},
		{
			"separator_1", "M 10 20 L 5 15 C 10 20 30 40 50 60",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 5.0, Y: 15.0},
				{Kind: PathSegmentCurveTo, Abs: true, X1: 10.0, Y1: 20.0, X2: 30.0, Y2: 40.0, X: 50.0, Y: 60.0},
			},
		},
		{
			"separator_2", "M 10, 20 L 5, 15 C 10, 20 30, 40 50, 60",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 5.0, Y: 15.0},
				{Kind: PathSegmentCurveTo, Abs: true, X1: 10.0, Y1: 20.0, X2: 30.0, Y2: 40.0, X: 50.0, Y: 60.0},
			},
		},
		{
			"separator_3", "M 10,20 L 5,15 C 10,20 30,40 50,60",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 5.0, Y: 15.0},
				{Kind: PathSegmentCurveTo, Abs: true, X1: 10.0, Y1: 20.0, X2: 30.0, Y2: 40.0, X: 50.0, Y: 60.0},
			},
		},
		{
			"separator_4", "M10, 20 L5, 15 C10, 20 30 40 50 60",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 5.0, Y: 15.0},
				{Kind: PathSegmentCurveTo, Abs: true, X1: 10.0, Y1: 20.0, X2: 30.0, Y2: 40.0, X: 50.0, Y: 60.0},
			},
		},
		{
			"separator_5", "M10 20V30H40V50H60Z",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentVerticalLineTo, Abs: true, Y: 30.0},
				{Kind: PathSegmentHorizontalLineTo, Abs: true, X: 40.0},
				{Kind: PathSegmentVerticalLineTo, Abs: true, Y: 50.0},
				{Kind: PathSegmentHorizontalLineTo, Abs: true, X: 60.0},
				{Kind: PathSegmentClosePath, Abs: true},
			},
		},
		{
			"all_segments_1", "M 10 20 L 30 40 H 50 V 60 C 70 80 90 100 110 120 S 130 140 150 160\n        Q 170 180 190 200 T 210 220 A 50 50 30 1 1 230 240 Z",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
				{Kind: PathSegmentHorizontalLineTo, Abs: true, X: 50.0},
				{Kind: PathSegmentVerticalLineTo, Abs: true, Y: 60.0},
				{Kind: PathSegmentCurveTo, Abs: true, X1: 70.0, Y1: 80.0, X2: 90.0, Y2: 100.0, X: 110.0, Y: 120.0},
				{Kind: PathSegmentSmoothCurveTo, Abs: true, X2: 130.0, Y2: 140.0, X: 150.0, Y: 160.0},
				{Kind: PathSegmentQuadratic, Abs: true, X1: 170.0, Y1: 180.0, X: 190.0, Y: 200.0},
				{Kind: PathSegmentSmoothQuadratic, Abs: true, X: 210.0, Y: 220.0},
				{Kind: PathSegmentEllipticalArc, Abs: true, Rx: 50.0, Ry: 50.0, XAxisRotation: 30.0, LargeArc: true, Sweep: true, X: 230.0, Y: 240.0},
				{Kind: PathSegmentClosePath, Abs: true},
			},
		},
		{
			"all_segments_2", "m 10 20 l 30 40 h 50 v 60 c 70 80 90 100 110 120 s 130 140 150 160\n        q 170 180 190 200 t 210 220 a 50 50 30 1 1 230 240 z",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: false, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: false, X: 30.0, Y: 40.0},
				{Kind: PathSegmentHorizontalLineTo, Abs: false, X: 50.0},
				{Kind: PathSegmentVerticalLineTo, Abs: false, Y: 60.0},
				{Kind: PathSegmentCurveTo, Abs: false, X1: 70.0, Y1: 80.0, X2: 90.0, Y2: 100.0, X: 110.0, Y: 120.0},
				{Kind: PathSegmentSmoothCurveTo, Abs: false, X2: 130.0, Y2: 140.0, X: 150.0, Y: 160.0},
				{Kind: PathSegmentQuadratic, Abs: false, X1: 170.0, Y1: 180.0, X: 190.0, Y: 200.0},
				{Kind: PathSegmentSmoothQuadratic, Abs: false, X: 210.0, Y: 220.0},
				{Kind: PathSegmentEllipticalArc, Abs: false, Rx: 50.0, Ry: 50.0, XAxisRotation: 30.0, LargeArc: true, Sweep: true, X: 230.0, Y: 240.0},
				{Kind: PathSegmentClosePath, Abs: false},
			},
		},
		{
			"close_path_1", "M10 20 L 30 40 ZM 100 200 L 300 400",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
				{Kind: PathSegmentClosePath, Abs: true},
				{Kind: PathSegmentMoveTo, Abs: true, X: 100.0, Y: 200.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 300.0, Y: 400.0},
			},
		},
		{
			"close_path_2", "M10 20 L 30 40 zM 100 200 L 300 400",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
				{Kind: PathSegmentClosePath, Abs: false},
				{Kind: PathSegmentMoveTo, Abs: true, X: 100.0, Y: 200.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 300.0, Y: 400.0},
			},
		},
		{
			"close_path_3", "M10 20 L 30 40 Z Z Z",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 10.0, Y: 20.0},
				{Kind: PathSegmentLineTo, Abs: true, X: 30.0, Y: 40.0},
				{Kind: PathSegmentClosePath, Abs: true},
				{Kind: PathSegmentClosePath, Abs: true},
				{Kind: PathSegmentClosePath, Abs: true},
			},
		},
		{"invalid_1", "M\t.", nil},
		{
			"invalid_2", "M 0 0 Z 2",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 0.0, Y: 0.0},
				{Kind: PathSegmentClosePath, Abs: true},
			},
		},
		{
			"invalid_3", "M 0 0 Z H 10",
			[]PathSegment{
				{Kind: PathSegmentMoveTo, Abs: true, X: 0.0, Y: 0.0},
				{Kind: PathSegmentClosePath, Abs: true},
				{Kind: PathSegmentHorizontalLineTo, Abs: true, X: 10.0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewPathParser(tt.text)
			for _, want := range tt.want {
				got, ok, err := s.Next()
				if !ok || err != nil || got != want {
					t.Errorf("got %+v, %v, %v, want %+v", got, ok, err, want)
				}
			}

			if _, ok, err := s.Next(); ok && err == nil {
				t.Error("got a segment, want an error or the end")
			}
		})
	}
}

func TestPathHelperFunctions(t *testing.T) {
	if !(PathSegment{Kind: PathSegmentMoveTo, Abs: true, X: 0.0, Y: 0.0}).IsAbs() {
		t.Error("IsAbs = false")
	}
	if (PathSegment{Kind: PathSegmentMoveTo, Abs: false, X: 0.0, Y: 0.0}).IsAbs() { // asserting false
		t.Error("IsAbs = true")
	}
	if got := (PathSegment{Kind: PathSegmentMoveTo, Abs: true, X: 0.0, Y: 0.0}).Command(); got != 'M' {
		t.Errorf("Command = %c", got)
	}
}

func TestSimplifyingPathParser(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []SimplePathSegment
	}{
		{
			"ignore_duplicated_close_paths", "M 10 20 L 30 40 Z Z Z Z",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 10.0, Y: 20.0},
				{Kind: SimplePathSegmentLineTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentClosePath},
			},
		},
		{
			"relative_move_to", "m 30 40 110 120 -20 -130",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentLineTo, X: 140.0, Y: 160.0},
				{Kind: SimplePathSegmentLineTo, X: 120.0, Y: 30.0},
			},
		},
		{
			"smooth_curve_to_after_move_to", "M 30 40 S 171 45 180 155",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentCurveTo, X1: 30.0, Y1: 40.0, X2: 171.0, Y2: 45.0, X: 180.0, Y: 155.0},
			},
		},
		{
			"smooth_curve_to_after_curve_to", "M 30 40 C 16 137 171 45 100 90 S 171 45 180 155",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentCurveTo, X1: 16.0, Y1: 137.0, X2: 171.0, Y2: 45.0, X: 100.0, Y: 90.0},
				{Kind: SimplePathSegmentCurveTo, X1: 29.0, Y1: 135.0, X2: 171.0, Y2: 45.0, X: 180.0, Y: 155.0},
			},
		},
		{
			"relative_smooth_curve_to_after_arc_to", "M 1 5 A 5 5 0 0 1 3 1 s 3 2 8 2",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 1.0, Y: 5.0},
				{Kind: SimplePathSegmentCurveTo, X1: 1.0, Y1: 3.4262134833347355, X2: 1.7409707866677877, Y2: 1.9442719099991592, X: 2.9999999999999996, Y: 1.0000000000000004},
				{Kind: SimplePathSegmentCurveTo, X1: 2.9999999999999996, Y1: 1.0000000000000004, X2: 6.0, Y2: 3.0000000000000004, X: 11.0, Y: 3.0000000000000004},
			},
		},
		{
			"smooth_quadratic_after_move_to", "M 30 40 T 180 155",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentQuadratic, X1: 30.0, Y1: 40.0, X: 180.0, Y: 155.0},
			},
		},
		{
			"smooth_quadratic_after_quadratic", "M 30 40 Q 171 45 100 90 T 160 180",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentQuadratic, X1: 171.0, Y1: 45.0, X: 100.0, Y: 90.0},
				{Kind: SimplePathSegmentQuadratic, X1: 29.0, Y1: 135.0, X: 160.0, Y: 180.0},
			},
		},
		{
			"relative_smooth_quadratic_after_quadratic", "M 30 40 Q 171 45 100 90 t 60 80",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentQuadratic, X1: 171.0, Y1: 45.0, X: 100.0, Y: 90.0},
				{Kind: SimplePathSegmentQuadratic, X1: 29.0, Y1: 135.0, X: 160.0, Y: 170.0},
			},
		},
		{
			"relative_smooth_quadratic_after_relative_quadratic", "M 30 40 q 171 45 50 40 t 60 80",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentQuadratic, X1: 201.0, Y1: 85.0, X: 80.0, Y: 80.0},
				{Kind: SimplePathSegmentQuadratic, X1: -41.0, Y1: 75.0, X: 140.0, Y: 160.0},
			},
		},
		{
			"smooth_quadratic_after_smooth_quadratic", "M 30 30 T 40 140 T 170 30",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 30.0},
				{Kind: SimplePathSegmentQuadratic, X1: 30.0, Y1: 30.0, X: 40.0, Y: 140.0},
				{Kind: SimplePathSegmentQuadratic, X1: 50.0, Y1: 250.0, X: 170.0, Y: 30.0},
			},
		},
		{
			"smooth_quadratic_after_relative_smooth_quadratic", "M 30 30 T 40 140 t 100 -30",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 30.0},
				{Kind: SimplePathSegmentQuadratic, X1: 30.0, Y1: 30.0, X: 40.0, Y: 140.0},
				{Kind: SimplePathSegmentQuadratic, X1: 50.0, Y1: 250.0, X: 140.0, Y: 110.0},
			},
		},
		{
			"smooth_quadratic_after_relative_quadratic", "M 30 30 T 40 140 q 30 100 120 -30",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 30.0},
				{Kind: SimplePathSegmentQuadratic, X1: 30.0, Y1: 30.0, X: 40.0, Y: 140.0},
				{Kind: SimplePathSegmentQuadratic, X1: 70.0, Y1: 240.0, X: 160.0, Y: 110.0},
			},
		},
		{
			"smooth_quadratic_after_relative_smooth_curve_to", "M 30 30 T 40 170 s 90 -20 90 -90",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 30.0},
				{Kind: SimplePathSegmentQuadratic, X1: 30.0, Y1: 30.0, X: 40.0, Y: 170.0},
				{Kind: SimplePathSegmentCurveTo, X1: 40.0, Y1: 170.0, X2: 130.0, Y2: 150.0, X: 130.0, Y: 80.0},
			},
		},
		{
			"quadratic_after_smooth_quadratic", "M 30 30 T 40 140 Q 80 180 170 30",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 30.0},
				{Kind: SimplePathSegmentQuadratic, X1: 30.0, Y1: 30.0, X: 40.0, Y: 140.0},
				{Kind: SimplePathSegmentQuadratic, X1: 80.0, Y1: 180.0, X: 170.0, Y: 30.0},
			},
		},
		{
			"relative_smooth_quadratic_to_after_arc_to", "M 1 5 A 5 5 0 0 1 3 1 t 8 2",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 1.0, Y: 5.0},
				{Kind: SimplePathSegmentCurveTo, X1: 1.0, Y1: 3.4262134833347355, X2: 1.7409707866677877, Y2: 1.9442719099991592, X: 2.9999999999999996, Y: 1.0000000000000004},
				{Kind: SimplePathSegmentQuadratic, X1: 2.9999999999999996, Y1: 1.0000000000000004, X: 11.0, Y: 3.0000000000000004},
			},
		},
		{
			"implicit_move_to_after_close_path", "M 10 20 L 30 40 Z L 50 60",
			[]SimplePathSegment{
				{Kind: SimplePathSegmentMoveTo, X: 10.0, Y: 20.0},
				{Kind: SimplePathSegmentLineTo, X: 30.0, Y: 40.0},
				{Kind: SimplePathSegmentClosePath},
				{Kind: SimplePathSegmentMoveTo, X: 10.0, Y: 20.0},
				{Kind: SimplePathSegmentLineTo, X: 50.0, Y: 60.0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSimplifyingPathParser(tt.text)
			for _, want := range tt.want {
				got, ok, err := s.Next()
				if !ok || err != nil || got != want {
					t.Errorf("got %+v, %v, %v, want %+v", got, ok, err, want)
				}
			}

			if _, ok, err := s.Next(); ok && err == nil {
				t.Error("got a segment, want an error or the end")
			}
		})
	}
}

func TestSimplifyingPathParserArcTo(t *testing.T) {
	s := NewSimplifyingPathParser("M 30 40 A 40 30 20 1 1 150 100")
	if got, ok, err := s.Next(); !ok || err != nil || got != (SimplePathSegment{Kind: SimplePathSegmentMoveTo, X: 30.0, Y: 40.0}) {
		t.Errorf("got %+v, %v, %v", got, ok, err)
	}
	curve1, _, _ := s.Next()
	curve2, _, _ := s.Next()
	if _, ok, err := s.Next(); ok && err == nil {
		t.Error("got a segment, want an error or the end")
	}

	check := func(c SimplePathSegment, want [6]float64) {
		t.Helper()
		if c.Kind != SimplePathSegmentCurveTo {
			t.Fatalf("invalid type %v", c.Kind)
		}
		got := [6]float64{math.Round(c.X1), math.Round(c.Y1), math.Round(c.X2), math.Round(c.Y2), math.Round(c.X), math.Round(c.Y)}
		if got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
	check(curve1, [6]float64{45.0, 16.0, 84.0, 10.0, 117.0, 27.0})
	check(curve2, [6]float64{150.0, 43.0, 165.0, 76.0, 150.0, 100.0})
}
