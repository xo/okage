// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file port the tests of krilla tests/src/annotation.rs.
// The snapshot annotation_to_embedded_pdf_page needs an embedded PDF file,
// which the base port leaves out (D62).

// startSnapshotPage starts the page of 200 by 200 of a snapshot test of
// krilla that takes a page.
func startSnapshotPage(d *Document) *Page {
	ps, _ := PageSettingsFromWH(200, 200)
	return d.StartPage(ps)
}

// TestAnnotationToLink is the snapshot test annotation_to_link of krilla
// tests/src/annotation.rs.
func TestAnnotationToLink(t *testing.T) {
	d := NewDocument(settings1()...)
	page := startSnapshotPage(d)
	page.AddAnnotation(linkTo(xywhRect(t, 50, 50, 100, 100), NewLinkAction("https://www.youtube.com")))
	checkSnapshot(t, "annotation_to_link", d)
}

// TestAnnotationWithBorder is the snapshot test annotation_with_border of
// krilla tests/src/annotation.rs.
func TestAnnotationWithBorder(t *testing.T) {
	d := NewDocument(settings1()...)
	page := startSnapshotPage(d)
	page.AddAnnotation(AnnotationFromLink(
		NewLinkAnnotation(xywhRect(t, 50, 50, 100, 100), NewLinkAction("https://www.youtube.com")).
			WithBorder(NewLinkBorder(1, NewColorRGB(255, 0, 0))),
	))
	checkSnapshot(t, "annotation_with_border", d)
}

// TestAnnotationWithQuadPoints is the snapshot test
// annotation_with_quad_points of krilla tests/src/annotation.rs.
func TestAnnotationWithQuadPoints(t *testing.T) {
	d := NewDocument(settings1()...)
	page := startSnapshotPage(d)
	surface := page.Surface()
	path1 := rectToPath(t, 0, 0, 50, 50)
	path2 := rectToPath(t, 50, 50, 100, 100)
	surface.SetFill(greenFill(t, 1))
	surface.DrawPath(path1)
	surface.DrawPath(path2)
	surface.Finish()

	page.AddAnnotation(AnnotationFromLink(NewLinkAnnotationWithQuadPoints(
		[]Quadrilateral{
			{
				path.PointFromXY(0, 50),
				path.PointFromXY(50, 50),
				path.PointFromXY(50, 0),
				path.PointFromXY(0, 0),
			},
			QuadrilateralFromRect(xywhRect(t, 50, 50, 50, 50)),
		},
		NewLinkAction("https://www.youtube.com"),
	)))
	checkSnapshot(t, "annotation_with_quad_points", d)
}

// TestAnnotationToInvalidDestination is the test
// annotation_to_invalid_destination of krilla tests/src/annotation.rs.
// krilla panics, and Finish returns ErrPDFInvalidDestination.
func TestAnnotationToInvalidDestination(t *testing.T) {
	d := NewDocument(settings1()...)
	page := startSnapshotPage(d)
	page.AddAnnotation(linkTo(xywhRect(t, 50, 50, 100, 100), NewXYZDestination(1, path.PointFromXY(100, 100))))
	page.Finish()

	var buf bytes.Buffer
	err := d.Finish(&buf)
	if !errors.Is(err, okage.ErrPDFInvalidDestination) {
		t.Fatalf("Finish gives %v, want ErrPDFInvalidDestination", err)
	}
}

// TestAnnotationToDestination is the snapshot test annotation_to_destination
// of krilla tests/src/annotation.rs.
func TestAnnotationToDestination(t *testing.T) {
	d := NewDocument(settings1()...)
	page := startSnapshotPage(d)
	page.AddAnnotation(linkTo(xywhRect(t, 50, 0, 100, 100), NewXYZDestination(1, path.PointFromXY(100, 100))))
	surface := page.Surface()
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(rectToPath(t, 50, 0, 150, 100))
	surface.Finish()
	page.Finish()

	page = startSnapshotPage(d)
	page.AddAnnotation(linkTo(xywhRect(t, 50, 100, 100, 100), NewXYZDestination(0, path.PointFromXY(0, 0))))
	surface = page.Surface()
	surface.SetFill(greenFill(t, 1))
	surface.DrawPath(rectToPath(t, 50, 100, 150, 200))
	surface.Finish()
	page.Finish()

	checkSnapshot(t, "annotation_to_destination", d)
}

// TestAnnotationErrors checks the errors that the port returns where krilla
// panics, and that no annotation is written when Finish fails.
func TestAnnotationErrors(t *testing.T) {
	nan := float32(math.NaN())
	const huge = 3e38
	cases := []struct {
		name string
		a    func(t *testing.T) Annotation
		want error
	}{
		{"no quadrilateral", func(*testing.T) Annotation {
			return AnnotationFromLink(NewLinkAnnotationWithQuadPoints(nil, NewLinkAction("x")))
		}, okage.ErrPDFInvalidAnnotation},
		{"NaN quadrilateral", func(*testing.T) Annotation {
			q := Quadrilateral{path.PointFromXY(nan, nan), path.PointFromXY(nan, nan), path.PointFromXY(nan, nan), path.PointFromXY(nan, nan)}
			return AnnotationFromLink(NewLinkAnnotationWithQuadPoints([]Quadrilateral{q}, NewLinkAction("x")))
		}, okage.ErrPDFInvalidAnnotation},
		{"nil target", func(t *testing.T) Annotation {
			t.Helper()
			return linkTo(xywhRect(t, 0, 0, 10, 10), nil)
		}, okage.ErrPDFInvalidAnnotation},
		{"nil go-to destination", func(t *testing.T) Annotation {
			t.Helper()
			return linkTo(xywhRect(t, 0, 0, 10, 10), NewGotoAction(nil))
		}, okage.ErrPDFInvalidAnnotation},
		// The page of this case is huge high, so the flip of the rectangle
		// overflows.
		{"rectangle that does not flip", func(t *testing.T) Annotation {
			t.Helper()
			return linkTo(xywhRect(t, 0, -huge, 10, 1), NewLinkAction("x"))
		}, okage.ErrPDFInvalidAnnotation},
		{"go-to a page that does not exist", func(t *testing.T) Annotation {
			t.Helper()
			return linkTo(xywhRect(t, 0, 0, 10, 10), NewGotoAction(NewXYZDestination(5, path.PointFromXY(0, 0))))
		}, okage.ErrPDFInvalidDestination},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := NewDocument(settings1()...)
			ps, _ := PageSettingsFromWH(10, huge)
			page := d.StartPage(ps)
			page.AddAnnotation(c.a(t))
			var buf bytes.Buffer
			if err := d.Finish(&buf); !errors.Is(err, c.want) {
				t.Fatalf("Finish gives %v, want %v", err, c.want)
			}
		})
	}
}

// TestAnnotationActions checks the other actions and options of a link: a
// go-to action to a named and to an XYZ destination, which share one object
// of the destination, the alt text, a CMYK
// and a luma border, and the quadrilaterals, which PDF 1.5 leaves out.
func TestAnnotationActions(t *testing.T) {
	alt := "the alt text"
	d := NewDocument(append(settings1(), WithPDFVersion(PDF15))...)
	page := startSnapshotPage(d)
	page.AddAnnotation(NewAnnotationLink(
		NewLinkAnnotation(xywhRect(t, 0, 0, 10, 10),
			NewGotoAction(NewNamedDestination("n", NewXYZDestination(0, path.PointFromXY(1, 2))))).
			WithBorder(NewLinkBorder(2, NewColorCMYK(0, 0, 0, 255))),
		&alt,
	).WithLocation(3))
	page.AddAnnotation(AnnotationFromLink(NewLinkAnnotation(xywhRect(t, 0, 0, 10, 10),
		NewGotoAction(NewXYZDestination(0, path.PointFromXY(1, 2)))).
		WithBorder(NewLinkBorder(0.5, NewColorLuma(128)))))
	page.AddAnnotation(AnnotationFromLink(NewLinkAnnotationWithQuadPoints(
		[]Quadrilateral{QuadrilateralFromRect(xywhRect(t, 0, 0, 10, 10))}, NewLinkAction("x"))))
	page.Finish()
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 10, 10), NewLinkAction("after finish")))

	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{
		"/S /GoTo\n    /D (n)", "/S /GoTo\n    /D 5 0 R", "/Contents (the alt text)",
		"/C [0 0 0 1]", "/C [0.5019608]", "/Border [0 0 2]", "/Dests <<\n      /Names [(n) 5 0 R]",
	} {
		if !bytes.Contains(buf.Bytes(), []byte(w)) {
			t.Errorf("the file has no %q:\n%s", w, buf.Bytes())
		}
	}
	for _, w := range []string{"/QuadPoints", "after finish"} {
		if bytes.Contains(buf.Bytes(), []byte(w)) {
			t.Errorf("the file has %q:\n%s", w, buf.Bytes())
		}
	}
}
