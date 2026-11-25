// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pathbool

import (
	"math"
	"strings"
	"testing"

	"github.com/xo/okage/kurbo"
)

// square returns a closed square from (x0, y0) to (x1, y1), counterclockwise
// in a coordinate system where y goes up, or clockwise when cw is true.
func square(x0, y0, x1, y1 float64, cw bool) []kurbo.PathEl {
	pts := []kurbo.Point{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}
	if cw {
		pts[1], pts[3] = pts[3], pts[1]
	}
	return []kurbo.PathEl{kurbo.MoveTo(pts[0]), kurbo.LineTo(pts[1]), kurbo.LineTo(pts[2]), kurbo.LineTo(pts[3]), kurbo.ClosePath()}
}

// circle returns a circle with the center (x, y) and the radius r, made of
// cubic curves.
func circle(x, y, r float64) []kurbo.PathEl {
	return kurbo.NewEllipse(kurbo.NewPoint(x, y), kurbo.NewVec2(r, r), 0).PathElements(1e-4)
}

// measure returns the signed area of the closed contours of path, and the
// number of contours. A contour with the inside on its left adds its area.
func measure(t *testing.T, path []kurbo.PathEl) (float64, int) {
	t.Helper()
	area, contours := 0.0, 0
	var start, last kurbo.Point
	for _, el := range path {
		switch el.Kind {
		case kurbo.PathElMoveTo:
			start, last = el.P[0], el.P[0]
			contours++
		case kurbo.PathElLineTo:
			area += last.X*el.P[0].Y - el.P[0].X*last.Y
			last = el.P[0]
		case kurbo.PathElClosePath:
			area += last.X*start.Y - start.X*last.Y
		default:
			t.Fatalf("the result holds the element %v", el.Kind)
		}
	}
	return area / 2, contours
}

func TestCombine(t *testing.T) {
	a := square(0, 0, 10, 10, false)
	b := square(5, 5, 15, 15, false)
	halfPlane := square(0, -20, 20, 20, false)
	tests := []struct {
		name     string
		op       Op
		operands [][]kurbo.PathEl
		area     float64
		contours int
	}{
		{"overlap union", Union, [][]kurbo.PathEl{a, b}, 175, 1},
		{"overlap difference", Difference, [][]kurbo.PathEl{a, b}, 75, 1},
		{"overlap intersection", Intersection, [][]kurbo.PathEl{a, b}, 25, 1},
		{"overlap xor", Xor, [][]kurbo.PathEl{a, b}, 150, 2},
		{"clockwise operand", Intersection, [][]kurbo.PathEl{a, square(5, 5, 15, 15, true)}, 25, 1},
		{"same union", Union, [][]kurbo.PathEl{a, a}, 100, 1},
		{"same intersection", Intersection, [][]kurbo.PathEl{a, a}, 100, 1},
		{"same difference", Difference, [][]kurbo.PathEl{a, a}, 0, 0},
		{"same xor", Xor, [][]kurbo.PathEl{a, square(0, 0, 10, 10, true)}, 0, 0},
		{"shared edge union", Union, [][]kurbo.PathEl{a, square(10, 0, 20, 10, false)}, 200, 1},
		{"shared edge intersection", Intersection, [][]kurbo.PathEl{a, square(10, 0, 20, 10, false)}, 0, 0},
		{"shared corner union", Union, [][]kurbo.PathEl{a, square(10, 10, 20, 20, false)}, 200, 2},
		{"hole", Difference, [][]kurbo.PathEl{a, square(2, 2, 8, 8, false)}, 64, 2},
		{"three operands", Difference, [][]kurbo.PathEl{square(0, 0, 30, 10, false), square(5, -5, 10, 15, false), square(20, -5, 25, 15, false)}, 200, 3},
		{"three intersections", Intersection, [][]kurbo.PathEl{a, b, square(-5, 7, 20, 20, false)}, 15, 1},
		{"one operand", Union, [][]kurbo.PathEl{a}, 100, 1},
		{"empty first operand", Union, [][]kurbo.PathEl{nil, a}, 100, 1},
		{"empty difference", Difference, [][]kurbo.PathEl{nil, a}, 0, 0},
		{"overlapping subpaths", Union, [][]kurbo.PathEl{append(append([]kurbo.PathEl{}, a...), b...)}, 175, 1},
		{"half circle", Intersection, [][]kurbo.PathEl{circle(0, 0, 10), halfPlane}, 50 * math.Pi, 1},
		{"circle hole", Difference, [][]kurbo.PathEl{square(-20, -20, 20, 20, false), circle(0, 0, 10)}, 1600 - 100*math.Pi, 2},
	}
	for _, tc := range tests {
		path := Combine(tc.op, tc.operands, 0.01, nil)
		area, contours := measure(t, path)
		// The polylines of a curve lie inside it, so a circle loses about
		// two thirds of the tolerance times its perimeter.
		slack := 1e-9
		if strings.Contains(tc.name, "circle") {
			slack = 0.5
		}
		if math.Abs(area-tc.area) > slack || contours != tc.contours {
			t.Errorf("%s: the area is %v in %d contours, want %v in %d", tc.name, area, contours, tc.area, tc.contours)
		}
	}
}

// TestCombineCircles checks that the union and the intersection of two
// circles add up to the areas of the circles, and that each point of the
// result lies within the tolerance of a circle.
func TestCombineCircles(t *testing.T) {
	c1, c2 := circle(0, 0, 10), circle(8, 3, 7)
	union, _ := measure(t, Combine(Union, [][]kurbo.PathEl{c1, c2}, 0.01, nil))
	inter, _ := measure(t, Combine(Intersection, [][]kurbo.PathEl{c1, c2}, 0.01, nil))
	if want := 100*math.Pi + 49*math.Pi; math.Abs(union+inter-want) > 0.01*(20+14)*math.Pi {
		t.Errorf("the union %v and the intersection %v add up to %v, want %v", union, inter, union+inter, want)
	}
	for _, el := range Combine(Xor, [][]kurbo.PathEl{c1, c2}, 0.01, nil) {
		if el.Kind == kurbo.PathElClosePath {
			continue
		}
		p := el.P[0]
		d1 := math.Abs(math.Hypot(p.X, p.Y) - 10)
		d2 := math.Abs(math.Hypot(p.X-8, p.Y-3) - 7)
		if min(d1, d2) > 0.02 {
			t.Errorf("the point %v lies %v from the circles", p, min(d1, d2))
		}
	}
}

// BenchmarkCombine measures the intersection of two circles and a square
// with the tolerance of lottie, 0.1.
func BenchmarkCombine(b *testing.B) {
	operands := [][]kurbo.PathEl{circle(0, 0, 100), circle(80, 30, 70), square(-50, -50, 50, 50, false)}
	var path []kurbo.PathEl
	for b.Loop() {
		path = Combine(Intersection, operands, 0.1, path[:0])
	}
}

// TestCombineNoArea checks that operands with no area give no result.
func TestCombineNoArea(t *testing.T) {
	line := []kurbo.PathEl{kurbo.MoveTo(kurbo.NewPoint(0, 0)), kurbo.LineTo(kurbo.NewPoint(10, 10))}
	for _, operands := range [][][]kurbo.PathEl{nil, {nil}, {line}, {line, line}} {
		if path := Combine(Union, operands, 0.1, nil); len(path) != 0 {
			t.Errorf("%v gives %v", operands, path)
		}
	}
}
