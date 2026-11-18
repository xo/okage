// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

// This file ports no upstream file. It holds the cache of the results of
// :has() of D58. Upstream walks the subtree or the next siblings of an
// element each time that it tests :has() on the element. That takes time in
// the square of the number of elements, and in a power of it for :has()
// inside :has(). The port gives the same results, and keeps the result of
// each relative selector for each element. All the wrappers of one tree share
// one cache, so each relative selector tests each element about once.

// relation is a relative selector of :has(): its combinator, and the
// expression of its selector.
type relation struct {
	combinator string
	expression expr
}

// relationKey is the key of a result in a matchCache.
type relationKey struct {
	relation *relation
	element  Element
}

// matchCache holds the result of each relative selector of :has() for the
// elements of one tree. The result for an element is true when an element
// that the combinator reaches from it matches the selector.
type matchCache struct {
	results map[relationKey]bool
}

// get returns the result of r for el, and false when the cache does not
// hold it.
func (c *matchCache) get(r *relation, el Element) (result, ok bool) {
	result, ok = c.results[relationKey{r, el}]
	return result, ok
}

// set keeps the result of r for el.
func (c *matchCache) set(r *relation, el Element, result bool) {
	if c.results == nil {
		c.results = map[relationKey]bool{}
	}
	c.results[relationKey{r, el}] = result
}

// clear drops all the results.
func (c *matchCache) clear() {
	clear(c.results)
}

// matches reports whether an element that the combinator of r reaches from
// el matches the selector of r. It is the expression
// any(expression for el in elements) of upstream.
func (r *relation) matches(el *ElementWrapper) bool {
	c := el.cache
	if c == nil {
		// A wrapper that a program made without FromXMLRoot or
		// FromHTMLRoot has no cache. The cache of this call still bounds
		// its time.
		c = &matchCache{}
	}
	if result, ok := c.get(r, el.Element); ok {
		return result
	}
	switch r.combinator {
	case " ":
		return r.descendant(el, c)
	case ">":
		result := false
		for child := range el.IterChildren() {
			if r.expression.eval(child) {
				result = true
				break
			}
		}
		c.set(r, el.Element, result)
		return result
	case "+":
		result := false
		if next := el.nextSibling(); next != nil {
			result = r.expression.eval(next)
		}
		c.set(r, el.Element, result)
		return result
	case "~":
		return r.subsequentSibling(el, c)
	}
	return false
}

// descendant is matches for the combinator " ". It walks the subtree of el
// with a stack, and keeps the result of each element that it leaves. The
// result of an element is true when a child matches, or when the result of
// a child is true. The walk does not enter an element whose result the cache
// holds, and it leaves an element at its first child that makes its result
// true.
func (r *relation) descendant(el *ElementWrapper, c *matchCache) bool {
	type frame struct {
		wrapper  *ElementWrapper
		index    int
		previous *ElementWrapper
		result   bool
	}
	stack := []frame{{wrapper: el}}
	for {
		top := &stack[len(stack)-1]
		children := top.wrapper.EtreeChildren()
		if !top.result && top.index < len(children) {
			child := newElementWrapper(children[top.index], top.wrapper, top.index, top.previous,
				top.wrapper.InHTMLDocument, "")
			top.index++
			top.previous = child
			if r.expression.eval(child) {
				top.result = true
				continue
			}
			if result, ok := c.get(r, child.Element); ok {
				top.result = result
				continue
			}
			stack = append(stack, frame{wrapper: child})
			continue
		}
		result := top.result
		c.set(r, top.wrapper.Element, result)
		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			return result
		}
		// The parent entered this child only while its result was false.
		stack[len(stack)-1].result = result
	}
}

// subsequentSibling is matches for the combinator "~". It walks the next
// siblings of el up to the first one that matches, or up to the first one
// whose result the cache holds. Each sibling before that one gets the same
// result as el.
func (r *relation) subsequentSibling(el *ElementWrapper, c *matchCache) bool {
	walked := []Element{el.Element}
	result := false
	for s := el.nextSibling(); s != nil; s = s.nextSibling() {
		if r.expression.eval(s) {
			result = true
			break
		}
		if cached, ok := c.get(r, s.Element); ok {
			result = cached
			break
		}
		walked = append(walked, s.Element)
	}
	for _, e := range walked {
		c.set(r, e, result)
	}
	return result
}

// nextSibling returns a new wrapper of the next sibling of the element, or
// nil for the last child and for the root. The element is the previous
// sibling of the new wrapper.
func (e *ElementWrapper) nextSibling() *ElementWrapper {
	if e.Parent == nil {
		return nil
	}
	siblings := e.Parent.EtreeChildren()
	i := e.Index + 1
	if i >= len(siblings) {
		return nil
	}
	return newElementWrapper(siblings[i], e.Parent, i, e, e.Parent.InHTMLDocument, "")
}
