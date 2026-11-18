// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"iter"
	"slices"
	"strings"

	"github.com/xo/okage/tinycss2"
)

// Element is an element of a document tree that selectors match. Upstream
// matches an Element of xml.etree.ElementTree. A program adapts its own tree
// to this interface, such as a tree of HTML or of roxmltree. Each value of
// Element must be comparable, such as a pointer or a small struct. Two values
// for the same element must be equal, and two values for different elements
// must not be, because the results of :has() are kept for each value (D58).
type Element interface {
	// Children returns the child elements, in tree order. It skips the
	// comments, the processing instructions and the text.
	Children() iter.Seq[Element]
	// LocalName returns the local name of the element.
	LocalName() string
	// NamespaceURL returns the namespace URL of the element, or "" for no
	// namespace.
	NamespaceURL() string
	// Attr returns the value of the attribute with the given namespace URL
	// and local name, and false if the element has no such attribute. The
	// namespace URL "" is no namespace. In an HTML document, cssselect2
	// asks for the lower case name.
	Attr(namespaceURL, name string) (string, bool)
	// Text returns the text before the first child element, as the
	// attribute text of ElementTree does. Only :empty reads it, and only
	// whether it is empty.
	Text() string
}

// ElementWrapper is ElementWrapper in cssselect2 cssselect2/tree.py. It
// wraps an Element for the matching of selectors.
//
// Make one with FromXMLRoot or FromHTMLRoot for the root element of a
// document, and reach the other elements with methods such as IterChildren
// and IterSubtree.
type ElementWrapper struct {
	// Element is the wrapped element, the etree_element of upstream.
	Element Element
	// Parent is the wrapper of the parent, or nil for the root element.
	Parent *ElementWrapper
	// Previous is the wrapper of the previous sibling, or nil.
	Previous *ElementWrapper
	// Index is the position within the children of the parent, from 0.
	// EtreeSiblings()[Index] is always Element.
	Index int
	// InHTMLDocument is true for a wrapper of FromHTMLRoot. It makes the
	// names of the selectors case-insensitive.
	InHTMLDocument bool
	// TransportContentLanguage is the content language of the transport,
	// such as the header Content-Language of HTTP, or "".
	TransportContentLanguage string

	etreeSiblings []Element

	// cache holds the results of :has() for the tree. All the wrappers of
	// one root share it (D58).
	cache *matchCache

	children      []Element
	childrenDone  bool
	lang          string
	langDone      bool
	disabled      bool
	disabledDone  bool
	classes       []string
	classesDone   bool
	id            string
	hasID, idDone bool
	localName     string
	namespaceURL  string
	nameDone      bool
}

// FromXMLRoot is ElementWrapper.from_xml_root in cssselect2
// cssselect2/tree.py. It wraps the root of an XML or XHTML document.
//
// If root is not the root of its document, the matching behaves as if it
// were. The selectors are not scoped to the subtree of root.
// contentLanguage is the content language of the transport, or "".
func FromXMLRoot(root Element, contentLanguage string) *ElementWrapper {
	return fromRoot(root, contentLanguage, false)
}

// FromHTMLRoot is ElementWrapper.from_html_root in cssselect2
// cssselect2/tree.py. It is FromXMLRoot with names that are not case
// sensitive. Use it for a document of an HTML parser, with the MIME type
// text/html.
func FromHTMLRoot(root Element, contentLanguage string) *ElementWrapper {
	return fromRoot(root, contentLanguage, true)
}

// fromRoot is ElementWrapper._from_root.
func fromRoot(root Element, contentLanguage string, inHTMLDocument bool) *ElementWrapper {
	return newElementWrapper(root, nil, 0, nil, inHTMLDocument, contentLanguage)
}

// newElementWrapper is the constructor of ElementWrapper.
func newElementWrapper(etreeElement Element, parent *ElementWrapper, index int, previous *ElementWrapper,
	inHTMLDocument bool, contentLanguage string,
) *ElementWrapper {
	e := &ElementWrapper{
		Element:                  etreeElement,
		Parent:                   parent,
		Previous:                 previous,
		Index:                    index,
		InHTMLDocument:           inHTMLDocument,
		TransportContentLanguage: contentLanguage,
	}
	if parent != nil {
		e.etreeSiblings = parent.EtreeChildren()
		e.cache = parent.cache
	} else {
		e.etreeSiblings = []Element{etreeElement}
		e.cache = &matchCache{}
	}
	return e
}

// EtreeSiblings is the attribute etree_siblings of upstream: the children of
// the parent, or the element alone for the root.
func (e *ElementWrapper) EtreeSiblings() []Element {
	return e.etreeSiblings
}

// Ancestors is the property ancestors of upstream: the wrappers of the
// ancestors, from the root to the parent.
func (e *ElementWrapper) Ancestors() []*ElementWrapper {
	var ancestors []*ElementWrapper
	for a := e.Parent; a != nil; a = a.Parent {
		ancestors = append(ancestors, a)
	}
	slices.Reverse(ancestors)
	return ancestors
}

// PreviousSiblings is the property previous_siblings of upstream: the
// wrappers of the previous siblings, in tree order.
func (e *ElementWrapper) PreviousSiblings() []*ElementWrapper {
	var siblings []*ElementWrapper
	for s := e.Previous; s != nil; s = s.Previous {
		siblings = append(siblings, s)
	}
	slices.Reverse(siblings)
	return siblings
}

// IterSiblings is ElementWrapper.iter_siblings. It returns new wrappers of
// the siblings of the element, in tree order, with the element itself.
func (e *ElementWrapper) IterSiblings() iter.Seq[*ElementWrapper] {
	if e.Parent == nil {
		return func(yield func(*ElementWrapper) bool) {
			yield(e)
		}
	}
	return e.Parent.IterChildren()
}

// IterNextSiblings is ElementWrapper.iter_next_siblings. It returns new
// wrappers of the next siblings of the element, in tree order.
func (e *ElementWrapper) IterNextSiblings() iter.Seq[*ElementWrapper] {
	return e.iterSiblingsAfter()
}

// iterSiblingsAfter returns the wrappers of IterSiblings after the index of
// the element. Upstream finds the element in the siblings by equality, and
// the port uses its index.
func (e *ElementWrapper) iterSiblingsAfter() iter.Seq[*ElementWrapper] {
	return func(yield func(*ElementWrapper) bool) {
		for s := range e.IterSiblings() {
			if s.Index > e.Index {
				if !yield(s) {
					return
				}
			}
		}
	}
}

// IterChildren is ElementWrapper.iter_children. It returns new wrappers of
// the child elements, in tree order.
func (e *ElementWrapper) IterChildren() iter.Seq[*ElementWrapper] {
	return func(yield func(*ElementWrapper) bool) {
		var child *ElementWrapper
		for i, etreeChild := range e.EtreeChildren() {
			child = newElementWrapper(etreeChild, e, i, child, e.InHTMLDocument, "")
			if !yield(child) {
				return
			}
		}
	}
}

// IterSubtree is ElementWrapper.iter_subtree. It returns new wrappers of the
// subtree of the element, in tree order, with the element itself. As in
// upstream, the wrappers of the children are the wrappers of IterChildren.
func (e *ElementWrapper) IterSubtree() iter.Seq[*ElementWrapper] {
	return func(yield func(*ElementWrapper) bool) {
		// frame is an iterator of the children of parent: the index of the
		// next child, and the wrapper of the previous one.
		type frame struct {
			parent   *ElementWrapper
			index    int
			previous *ElementWrapper
		}
		if !yield(e) {
			return
		}
		stack := []*frame{{parent: e}}
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			children := top.parent.EtreeChildren()
			if top.index >= len(children) {
				stack = stack[:len(stack)-1]
				continue
			}
			child := newElementWrapper(children[top.index], top.parent, top.index, top.previous,
				top.parent.InHTMLDocument, "")
			top.index++
			top.previous = child
			if !yield(child) {
				return
			}
			stack = append(stack, &frame{parent: child})
		}
	}
}

// compileTests is ElementWrapper._compile. It keeps the selectors with no
// pseudo-element that can match.
func compileTests(selectors []*CompiledSelector) []*CompiledSelector {
	var tests []*CompiledSelector
	for _, s := range selectors {
		if s.PseudoElement == "" && !s.NeverMatches {
			tests = append(tests, s)
		}
	}
	return tests
}

// Matches is ElementWrapper.matches. It reports whether the element matches
// any of the given selectors. Upstream also takes the arguments of
// CompileSelectorList. In Go, compile them first.
func (e *ElementWrapper) Matches(selectors ...*CompiledSelector) bool {
	for _, test := range compileTests(selectors) {
		if test.Test(e) {
			return true
		}
	}
	return false
}

// QueryAll is ElementWrapper.query_all. It returns the elements of the
// subtree, in tree order, that match any of the given selectors. The
// selectors are scoped to the subtree of the element.
func (e *ElementWrapper) QueryAll(selectors ...*CompiledSelector) iter.Seq[*ElementWrapper] {
	tests := compileTests(selectors)
	return func(yield func(*ElementWrapper) bool) {
		if len(tests) == 0 {
			return
		}
		for element := range e.IterSubtree() {
			for _, test := range tests {
				if test.Test(element) {
					if !yield(element) {
						return
					}
					break
				}
			}
		}
	}
}

// Query is ElementWrapper.query. It returns the first element that matches
// any of the given selectors, or nil.
func (e *ElementWrapper) Query(selectors ...*CompiledSelector) *ElementWrapper {
	for element := range e.QueryAll(selectors...) {
		return element
	}
	return nil
}

// EtreeChildren is the property etree_children of upstream: the child
// elements, without the other nodes such as comments and processing
// instructions.
func (e *ElementWrapper) EtreeChildren() []Element {
	if !e.childrenDone {
		e.childrenDone = true
		for child := range e.Element.Children() {
			e.children = append(e.children, child)
		}
	}
	return e.children
}

// SetEtreeChildren sets the cached property etree_children, as WeasyPrint
// does for the elements of SVG that it makes. It drops the results of :has()
// that the wrappers of the tree keep (D58).
func (e *ElementWrapper) SetEtreeChildren(children []Element) {
	e.children = children
	e.childrenDone = true
	if e.cache != nil {
		e.cache.clear()
	}
}

// names caches the local name and the namespace URL.
func (e *ElementWrapper) names() {
	if !e.nameDone {
		e.nameDone = true
		e.localName = e.Element.LocalName()
		e.namespaceURL = e.Element.NamespaceURL()
	}
}

// LocalName is the property local_name of upstream.
func (e *ElementWrapper) LocalName() string {
	e.names()
	return e.localName
}

// NamespaceURL is the property namespace_url of upstream.
func (e *ElementWrapper) NamespaceURL() string {
	e.names()
	return e.namespaceURL
}

// ID is the property id of upstream: the attribute id, and false for None.
func (e *ElementWrapper) ID() (string, bool) {
	if !e.idDone {
		e.idDone = true
		e.id, e.hasID = e.Element.Attr("", "id")
	}
	return e.id, e.hasID
}

// Classes is the property classes of upstream: the classes of the element,
// with no duplicate. Upstream holds them in a set.
func (e *ElementWrapper) Classes() []string {
	if !e.classesDone {
		e.classesDone = true
		class, _ := e.Element.Attr("", "class")
		for _, c := range splitWhitespace(class) {
			if !contains(e.classes, c) {
				e.classes = append(e.classes, c)
			}
		}
	}
	return e.classes
}

// Lang is the property lang of upstream: the language of the element, in
// ASCII lower case, or "" when it is not known. It follows
// http://whatwg.org/C#language.
func (e *ElementWrapper) Lang() string {
	if !e.langDone {
		e.lang = e.computeLang()
		e.langDone = true
	}
	return e.lang
}

// computeLang is the body of the property lang.
func (e *ElementWrapper) computeLang() string {
	if xmlLang, ok := e.Element.Attr(xmlNamespace, "lang"); ok {
		return tinycss2.ASCIILower(xmlLang)
	}
	isHTML := e.InHTMLDocument || e.NamespaceURL() == xhtmlNamespace
	if isHTML {
		if lang, ok := e.Element.Attr("", "lang"); ok {
			return tinycss2.ASCIILower(lang)
		}
	}
	if e.Parent != nil {
		return e.Parent.Lang()
	}
	// Root element
	if isHTML {
		var contentLanguage string
		var hasContentLanguage bool
		for meta := range iterTag(e.Element, xhtmlNamespace, "meta") {
			httpEquiv, _ := meta.Attr("", "http-equiv")
			if tinycss2.ASCIILower(httpEquiv) == "content-language" {
				content, ok := meta.Attr("", "content")
				contentLanguage, hasContentLanguage = parseContentLanguage(content, ok)
			}
		}
		if hasContentLanguage {
			return tinycss2.ASCIILower(contentLanguage)
		}
	}
	// Empty string means unknown
	lang, _ := parseContentLanguage(e.TransportContentLanguage, e.TransportContentLanguage != "")
	return lang
}

// iterTag is Element.iter(tag) of ElementTree: the element and its
// descendants with the given tag, in tree order.
func iterTag(root Element, namespaceURL, localName string) iter.Seq[Element] {
	return func(yield func(Element) bool) {
		stack := []Element{root}
		for len(stack) > 0 {
			el := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if el.LocalName() == localName && el.NamespaceURL() == namespaceURL {
				if !yield(el) {
					return
				}
			}
			var children []Element
			for child := range el.Children() {
				children = append(children, child)
			}
			for _, child := range slices.Backward(children) {
				stack = append(stack, child)
			}
		}
	}
}

// InDisabledFieldset is the property in_disabled_fieldset of upstream.
func (e *ElementWrapper) InDisabledFieldset() bool {
	if !e.disabledDone {
		e.disabled = e.computeInDisabledFieldset()
		e.disabledDone = true
	}
	return e.disabled
}

// computeInDisabledFieldset is the body of the property
// in_disabled_fieldset.
func (e *ElementWrapper) computeInDisabledFieldset() bool {
	if e.Parent == nil {
		return false
	}
	isTag := func(el Element, localName string) bool {
		return el.NamespaceURL() == xhtmlNamespace && el.LocalName() == localName
	}
	_, parentDisabled := e.Parent.Element.Attr("", "disabled")
	disabledFieldset := isTag(e.Parent.Element, "fieldset") && parentDisabled
	if disabledFieldset && isTag(e.Element, "legend") {
		disabledFieldset = false
		for s := e.Previous; s != nil; s = s.Previous {
			if isTag(s.Element, "legend") {
				disabledFieldset = true
				break
			}
		}
	}
	return disabledFieldset || e.Parent.InDisabledFieldset()
}

// parseContentLanguage is _parse_content_language in cssselect2
// cssselect2/tree.py. It returns false for None. ok is false for a value of
// None.
func parseContentLanguage(value string, ok bool) (string, bool) {
	if ok && !strings.Contains(value, ",") {
		if parts := splitWhitespace(value); len(parts) == 1 {
			return parts[0], true
		}
	}
	return "", false
}
