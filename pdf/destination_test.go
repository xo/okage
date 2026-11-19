// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file port the tests of krilla
// src/interactive/destination.rs and the snapshot tests of
// tests/src/destination.rs.

// xywhRect returns a rectangle, as Rect::from_xywh(...).unwrap().
func xywhRect(t *testing.T, x, y, w, h float32) Rect {
	t.Helper()
	r, ok := path.RectFromXYWH(x, y, w, h)
	if !ok {
		t.Fatal("invalid rectangle")
	}
	return r
}

// linkTo returns a link annotation of rect to the target, as
// LinkAnnotation::new(rect, target).into().
func linkTo(rect Rect, target Target) Annotation {
	return AnnotationFromLink(NewLinkAnnotation(rect, target))
}

// TestNamedDuplicateRejected is the test named_duplicate_rejected of krilla
// src/interactive/destination.rs.
func TestNamedDuplicateRejected(t *testing.T) {
	d := NewDocument()
	if !d.RegisterNamedDestination(NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(0, 0)))) {
		t.Error("the first destination is rejected")
	}
	if d.RegisterNamedDestination(NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(100, 100)))) {
		t.Error("the second destination of the name is accepted")
	}
}

// TestNamedDuplicateSameLocationAllowed is the test
// named_duplicate_same_location_allowed of krilla
// src/interactive/destination.rs.
func TestNamedDuplicateSameLocationAllowed(t *testing.T) {
	d := NewDocument()
	for range 2 {
		if !d.RegisterNamedDestination(NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(0, 0)))) {
			t.Error("the destination is rejected")
		}
	}
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
}

// TestNamedDuplicateAnnotationRejected is the test
// named_duplicate_annotation_rejected of krilla
// src/interactive/destination.rs.
func TestNamedDuplicateAnnotationRejected(t *testing.T) {
	d := NewDocument()
	page := d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100),
		NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(0, 0)))))
	page.AddAnnotation(linkTo(xywhRect(t, 0, 100, 100, 100),
		NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(100, 100)))))
	page.Finish()

	var buf bytes.Buffer
	err := d.Finish(&buf)
	if !errors.Is(err, okage.ErrPDFDuplicateNamedDestination) {
		t.Fatalf("Finish gives %v, want ErrPDFDuplicateNamedDestination", err)
	}
	if want := "writing pdf: duplicate named destination: same"; err.Error() != want {
		t.Errorf("the error is %q, want %q", err, want)
	}
}

// TestNamedDuplicateManualThenAnnotationRejected is the test
// named_duplicate_manual_then_annotation_rejected of krilla
// src/interactive/destination.rs.
func TestNamedDuplicateManualThenAnnotationRejected(t *testing.T) {
	d := NewDocument()
	if !d.RegisterNamedDestination(NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(0, 0)))) {
		t.Error("the destination is rejected")
	}
	page := d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100),
		NewNamedDestination("same", NewXYZDestination(0, path.PointFromXY(100, 100)))))
	page.Finish()

	var buf bytes.Buffer
	if err := d.Finish(&buf); !errors.Is(err, okage.ErrPDFDuplicateNamedDestination) {
		t.Fatalf("Finish gives %v, want ErrPDFDuplicateNamedDestination", err)
	}
}

// TestDestinationNamed is the snapshot test destination_named of krilla
// tests/src/destination.rs.
func TestDestinationNamed(t *testing.T) {
	d := NewDocument(settings1()...)
	dest1 := NewNamedDestination("hi", NewXYZDestination(0, path.PointFromXY(100, 100)))
	dest2 := NewNamedDestination("by", NewXYZDestination(1, path.PointFromXY(0, 0)))

	page := d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), dest1))
	page.AddAnnotation(linkTo(xywhRect(t, 100, 100, 100, 100), dest2))

	surface := page.Surface()
	surface.SetFill(redFill(t, 1))
	surface.DrawPath(rectToPath(t, 0, 0, 100, 100))
	surface.SetFill(greenFill(t, 1))
	surface.DrawPath(rectToPath(t, 100, 100, 200, 200))
	surface.Finish()
	page.Finish()

	page = d.StartPage(DefaultPageSettings())
	page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), dest1))
	surface = page.Surface()
	surface.SetFill(blueFill(t, 1))
	surface.DrawPath(rectToPath(t, 0, 0, 100, 100))
	surface.Finish()
	page.Finish()

	checkSnapshot(t, "destination_named", d)
}

// TestDestinationNamedSorting is the snapshot test destination_named_sorting
// of krilla tests/src/destination.rs. The entries of the name tree must be
// sorted by name.
func TestDestinationNamedSorting(t *testing.T) {
	d := NewDocument(settings1()...)
	page := d.StartPage(DefaultPageSettings())
	for _, name := range []string{"aaa", "bb", "a", "zzz", "y", "aa", "x", "ab"} {
		dest := NewNamedDestination(name, NewXYZDestination(0, path.PointFromXY(0, 0)))
		page.AddAnnotation(linkTo(xywhRect(t, 0, 0, 100, 100), dest))
	}
	checkSnapshot(t, "destination_named_sorting", d)
}

// TestDestinationInvalidPage checks the error of a destination to a page
// that does not exist, where krilla panics: an index past the last page, and
// a negative index.
func TestDestinationInvalidPage(t *testing.T) {
	for _, index := range []int{1, -1} {
		d := NewDocument(settings1()...)
		d.RegisterNamedDestination(NewNamedDestination("x", NewXYZDestination(index, path.PointFromXY(0, 0))))
		var buf bytes.Buffer
		if err := d.Finish(&buf); !errors.Is(err, okage.ErrPDFInvalidDestination) {
			t.Errorf("the page %d gives %v, want ErrPDFInvalidDestination", index, err)
		}
	}
}
