// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"testing"

	"github.com/xo/okage/path"
)

// The tests of this file port the snapshot tests of krilla
// tests/src/outline.rs.

// TestOutlineSimple is the snapshot test outline_simple of krilla
// tests/src/outline.rs.
func TestOutlineSimple(t *testing.T) {
	d := NewDocument(settings1()...)
	fills := []*Fill{redFill(t, 1), greenFill(t, 1), blueFill(t, 1)}
	for index, fill := range fills {
		factor := float32(index) * 50
		p := rectToPath(t, factor, factor, 100+factor, 100+factor)
		ps, _ := PageSettingsFromWH(200, 200)
		page := d.StartPage(ps)
		surface := page.Surface()
		surface.SetFill(fill)
		surface.DrawPath(p)
		surface.Finish()
		page.Finish()
	}
	outline := NewOutline()

	child1 := NewOutlineNode("Heading 1", NewXYZDestination(0, path.PointFromXY(0, 0)))
	child1.PushChild(NewOutlineNode("Heading 1.1", NewXYZDestination(1, path.PointFromXY(50, 50))))

	child2 := NewOutlineNode("Heading 2", NewXYZDestination(2, path.PointFromXY(100, 100)))

	outline.PushChild(child1)
	outline.PushChild(child2)

	d.SetOutline(outline)
	checkSnapshot(t, "outline_simple", d)
}

// threePages starts three empty pages of 200 by 200, as the snapshot tests
// outline_open_state and outline_mixed_state do.
func threePages(d *Document) {
	for range 3 {
		ps, _ := PageSettingsFromWH(200, 200)
		d.StartPage(ps)
	}
}

// TestOutlineOpenState is the snapshot test outline_open_state of krilla
// tests/src/outline.rs.
func TestOutlineOpenState(t *testing.T) {
	d := NewDocument(settings1()...)
	threePages(d)

	outline := NewOutline()

	child1 := NewOutlineNode("Heading 1", NewXYZDestination(0, path.PointFromXY(0, 0))).WithOpen(true)
	child1.PushChild(NewOutlineNode("Heading 1.1", NewXYZDestination(1, path.PointFromXY(50, 50))))

	child2 := NewOutlineNode("Heading 2", NewXYZDestination(2, path.PointFromXY(100, 100)))

	outline.PushChild(child1)
	outline.PushChild(child2)

	d.SetOutline(outline)
	checkSnapshot(t, "outline_open_state", d)
}

// TestOutlineMixedState is the snapshot test outline_mixed_state of krilla
// tests/src/outline.rs.
func TestOutlineMixedState(t *testing.T) {
	d := NewDocument(settings1()...)
	threePages(d)

	outline := NewOutline()

	child1 := NewOutlineNode("Heading 1", NewXYZDestination(0, path.PointFromXY(0, 0))).WithOpen(true)
	nested := NewOutlineNode("Heading 1.1", NewXYZDestination(1, path.PointFromXY(50, 50))).WithOpen(true)
	nested.PushChild(NewOutlineNode("Heading 1.1.1", NewXYZDestination(1, path.PointFromXY(60, 60))))
	child1.PushChild(nested)

	child2 := NewOutlineNode("Heading 2", NewXYZDestination(2, path.PointFromXY(100, 100)))
	child2.PushChild(NewOutlineNode("Heading 2.1", NewXYZDestination(2, path.PointFromXY(120, 120))))

	outline.PushChild(child1)
	outline.PushChild(child2)

	d.SetOutline(outline)
	checkSnapshot(t, "outline_mixed_state", d)
}

// TestOutlineWithEmptyTitle is the snapshot test outline_with_empty_title of
// krilla tests/src/outline.rs.
func TestOutlineWithEmptyTitle(t *testing.T) {
	d := NewDocument(settings1()...)
	outline := NewOutline()
	outline.PushChild(NewOutlineNode("", NewXYZDestination(0, path.PointFromXY(0, 0))))
	d.SetOutline(outline)
	checkSnapshot(t, "outline_with_empty_title", d)
}

// TestOutlineEmptyAndCopied checks that an empty outline writes no outline,
// as upstream does, and that SetOutline copies the tree, so that a later
// change of a node of the caller does not change the document.
func TestOutlineEmptyAndCopied(t *testing.T) {
	d := NewDocument(settings1()...)
	d.SetOutline(NewOutline())
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("/Outlines")) {
		t.Errorf("an empty outline is written:\n%s", buf.Bytes())
	}

	d = NewDocument(settings1()...)
	node := NewOutlineNode("a", NewXYZDestination(0, path.PointFromXY(0, 0)))
	node.PushChild(NewOutlineNode("b", NewXYZDestination(0, path.PointFromXY(0, 0))))
	outline := NewOutline()
	outline.PushChild(node)
	d.SetOutline(outline)
	outline.children[0].children[0].text = "changed"
	buf.Reset()
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(buf.Bytes(), []byte("changed")) {
		t.Errorf("a change of the outline after SetOutline changes the file:\n%s", buf.Bytes())
	}
}
