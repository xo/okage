package cssselect2

import (
	"slices"
	"testing"
)

// largeTree returns a tree of n sections under a root, each with a heading
// and ten paragraphs of two inline elements.
func largeTree(n int) *testElement {
	root := &testElement{localName: "body"}
	for range n {
		s := &testElement{localName: "section"}
		s.children = append(s.children, &testElement{localName: "h2"})
		for range 10 {
			p := &testElement{localName: "p"}
			p.children = []*testElement{{localName: "em"}, {localName: "a"}}
			s.children = append(s.children, p)
		}
		root.children = append(root.children, s)
	}
	return root
}

// BenchmarkIterSubtree walks a tree of 32,001 elements with IterSubtree,
// for the allocations of the wrappers and the lists of the children.
func BenchmarkIterSubtree(b *testing.B) {
	root := largeTree(1000)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for range FromXMLRoot(root, "").IterSubtree() {
			n++
		}
		if n != 32001 {
			b.Fatalf("expected 32001 elements, got %d", n)
		}
	}
}

// appendingElement is a testElement with the optional method
// AppendChildren of ChildrenAppender.
type appendingElement struct{ *testElement }

// AppendChildren satisfies ChildrenAppender.
func (e appendingElement) AppendChildren(dst []Element) []Element {
	dst = slices.Grow(dst, len(e.children))
	for _, c := range e.children {
		dst = append(dst, appendingElement{c})
	}
	return dst
}

// BenchmarkIterSubtreeAppender walks the tree of BenchmarkIterSubtree
// through elements with AppendChildren (D105).
func BenchmarkIterSubtreeAppender(b *testing.B) {
	root := appendingElement{largeTree(1000)}
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for range FromXMLRoot(root, "").IterSubtree() {
			n++
		}
		if n != 32001 {
			b.Fatalf("expected 32001 elements, got %d", n)
		}
	}
}

// TestChildrenAppender checks that the wrappers take the children of
// AppendChildren, and that a selector matches them as it matches the
// children of Children (D105).
func TestChildrenAppender(t *testing.T) {
	root := largeTree(3)
	sel, err := CompileSelectorList("section > p:nth-child(3) a", nil)
	if err != nil {
		t.Fatal(err)
	}
	count := func(r Element) int {
		n := 0
		for range FromXMLRoot(r, "").QueryAll(sel...) {
			n++
		}
		return n
	}
	if a, b := count(appendingElement{root}), count(root); a != 3 || b != 3 {
		t.Errorf("expected 3 matches each, got %d with AppendChildren and %d with Children", a, b)
	}
}
