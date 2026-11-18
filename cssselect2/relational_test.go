package cssselect2

import (
	"iter"
	"testing"
)

// This file ports no upstream test. It tests the cache of the results of
// :has() of D58 on large trees.

// countingElement is an element that counts the calls that read the tree.
type countingElement struct {
	localName string
	children  []*countingElement
	calls     *int
}

func (e *countingElement) Children() iter.Seq[Element] {
	*e.calls++
	return func(yield func(Element) bool) {
		for _, c := range e.children {
			if !yield(c) {
				return
			}
		}
	}
}

func (e *countingElement) LocalName() string {
	*e.calls++
	return e.localName
}

func (e *countingElement) NamespaceURL() string            { return "" }
func (e *countingElement) Attr(_, _ string) (string, bool) { return "", false }
func (e *countingElement) Text() string                    { return "" }

// chainTree makes a chain of n div elements, each the only child of the
// one before, with an a element as the child of the last one.
func chainTree(n int, calls *int) *countingElement {
	root := &countingElement{localName: "div", calls: calls}
	e := root
	for range n - 1 {
		c := &countingElement{localName: "div", calls: calls}
		e.children = []*countingElement{c}
		e = c
	}
	e.children = []*countingElement{{localName: "a", calls: calls}}
	return root
}

// flatTree makes a root with n children. Each child is a p element, except
// the last one, which is an a element.
func flatTree(n int, calls *int) *countingElement {
	root := &countingElement{localName: "body", calls: calls}
	for i := range n {
		name := "p"
		if i == n-1 {
			name = "a"
		}
		root.children = append(root.children, &countingElement{localName: name, calls: calls})
	}
	return root
}

// balancedTree makes a binary tree of div elements of the given depth.
func balancedTree(depth int, calls *int) *countingElement {
	e := &countingElement{localName: "div", calls: calls}
	if depth > 1 {
		e.children = []*countingElement{balancedTree(depth-1, calls), balancedTree(depth-1, calls)}
	}
	return e
}

// TestHasIsLinear matches selectors with :has() against every element of
// large trees. Upstream reads the tree a number of times in the square of
// the number of elements, or in a power of it for the nested :has(). The
// port reads it a number of times that is linear in the number of elements.
func TestHasIsLinear(t *testing.T) {
	const n = 4000
	tests := []struct {
		name     string
		tree     func(calls *int) *countingElement
		elements int
		selector string
		matches  int
	}{
		{"descendant in a chain", func(c *int) *countingElement { return chainTree(n, c) }, n + 1, "div:has(a)", n},
		{"descendant list in a chain", func(c *int) *countingElement { return chainTree(n, c) }, n + 1, ":has(p, a)", n},
		{"nested descendant in a chain", func(c *int) *countingElement { return chainTree(n, c) }, n + 1, ":has(:has(:has(a)))", n - 2},
		{"child in a chain", func(c *int) *countingElement { return chainTree(n, c) }, n + 1, ":has(> :has(> :has(> a)))", 1},
		{"subsequent sibling", func(c *int) *countingElement { return flatTree(n, c) }, n + 1, "p:has(~ a)", n - 1},
		{"nested subsequent sibling", func(c *int) *countingElement { return flatTree(n, c) }, n + 1, ":has(~ :has(~ :has(~ a)))", n - 3},
		{"next sibling", func(c *int) *countingElement { return flatTree(n, c) }, n + 1, "p:has(+ p)", n - 2},
		{"nested in a balanced tree", func(c *int) *countingElement { return balancedTree(12, c) }, 1<<12 - 1, ":has(:has(:has(*)))", 1<<9 - 1},
		{"descendant of a sibling", func(c *int) *countingElement { return flatTree(n, c) }, n + 1, ":has(~ p:has(a)), body:has(a)", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			root := tt.tree(&calls)
			selectors, err := CompileSelectorList(tt.selector, nil)
			if err != nil {
				t.Fatal(err)
			}
			matcher := NewMatcher[int]()
			for i, s := range selectors {
				matcher.AddSelector(s, i)
			}
			matches := 0
			for el := range FromXMLRoot(root, "").IterSubtree() {
				if len(matcher.Match(el)) > 0 {
					matches++
				}
			}
			if matches != tt.matches {
				t.Errorf("%d elements match, want %d", matches, tt.matches)
			}
			// Each element is read a few times for each relative selector:
			// once by the walk of the matcher, once by the walk of each
			// relative selector, and once as the subject of each one.
			if limit := 20 * tt.elements; calls > limit {
				t.Errorf("the matching reads the tree %d times, want at most %d", calls, limit)
			}
		})
	}
}
