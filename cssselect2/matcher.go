// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"slices"

	"github.com/xo/okage/tinycss2"
)

// This file ports the class Matcher of cssselect2 cssselect2/__init__.py.

// Version is VERSION of cssselect2, the version that the package ports.
const Version = "0.10.1"

// matcherEntry is an entry of a Matcher: the test, the specificity, the
// order, the pseudo-element and the payload of a selector.
type matcherEntry[P any] struct {
	selector *CompiledSelector
	order    int
	payload  P
}

// Match is a tuple of the result of Matcher.match in upstream: the
// specificity, the order, the pseudo-element and the payload of a selector
// that matches.
type Match[P any] struct {
	Specificity   Specificity
	Order         int
	PseudoElement string
	Payload       P
}

// Matcher is Matcher in cssselect2 cssselect2/__init__.py, a store of CSS
// selectors that matches HTML elements. P is the type of the payloads, such
// as the declarations of a style rule.
type Matcher[P any] struct {
	idSelectors             map[string][]matcherEntry[P]
	classSelectors          map[string][]matcherEntry[P]
	lowerLocalNameSelectors map[string][]matcherEntry[P]
	namespaceSelectors      map[string][]matcherEntry[P]
	langAttrSelectors       []matcherEntry[P]
	otherSelectors          []matcherEntry[P]
	order                   int
}

// NewMatcher makes an empty Matcher.
func NewMatcher[P any]() *Matcher[P] {
	return &Matcher[P]{
		idSelectors:             map[string][]matcherEntry[P]{},
		classSelectors:          map[string][]matcherEntry[P]{},
		lowerLocalNameSelectors: map[string][]matcherEntry[P]{},
		namespaceSelectors:      map[string][]matcherEntry[P]{},
	}
}

// AddSelector is Matcher.add_selector. It adds a selector and its payload to
// the matcher. Match returns the payload as it is.
func (m *Matcher[P]) AddSelector(selector *CompiledSelector, payload P) {
	m.order++

	if selector.NeverMatches {
		return
	}

	entry := matcherEntry[P]{selector, m.order, payload}
	switch {
	case selector.ID != "":
		m.idSelectors[selector.ID] = append(m.idSelectors[selector.ID], entry)
	case selector.ClassName != "":
		m.classSelectors[selector.ClassName] = append(m.classSelectors[selector.ClassName], entry)
	case selector.LocalName != "":
		m.lowerLocalNameSelectors[selector.LowerLocalName] = append(
			m.lowerLocalNameSelectors[selector.LowerLocalName], entry)
	case selector.HasNamespace:
		m.namespaceSelectors[selector.Namespace] = append(m.namespaceSelectors[selector.Namespace], entry)
	case selector.RequiresLangAttr:
		m.langAttrSelectors = append(m.langAttrSelectors, entry)
	default:
		m.otherSelectors = append(m.otherSelectors, entry)
	}
}

// Match is Matcher.match. It matches the selectors against the given
// element.
//
// It returns the selectors that match the element, from the lowest to the
// highest specificity, and in the order of AddSelector for an equal
// specificity.
func (m *Matcher[P]) Match(element *ElementWrapper) []Match[P] {
	var relevantSelectors []Match[P]

	if id, ok := element.ID(); ok {
		if entries, ok := m.idSelectors[id]; ok {
			relevantSelectors = addRelevantSelectors(element, entries, relevantSelectors)
		}
	}

	for _, className := range element.Classes() {
		if entries, ok := m.classSelectors[className]; ok {
			relevantSelectors = addRelevantSelectors(element, entries, relevantSelectors)
		}
	}

	lowerName := tinycss2.ASCIILower(element.LocalName())
	if entries, ok := m.lowerLocalNameSelectors[lowerName]; ok {
		relevantSelectors = addRelevantSelectors(element, entries, relevantSelectors)
	}
	if entries, ok := m.namespaceSelectors[element.NamespaceURL()]; ok {
		relevantSelectors = addRelevantSelectors(element, entries, relevantSelectors)
	}

	if _, ok := element.Element.Attr("", "lang"); ok {
		relevantSelectors = addRelevantSelectors(element, m.langAttrSelectors, relevantSelectors)
	}

	relevantSelectors = addRelevantSelectors(element, m.otherSelectors, relevantSelectors)

	slices.SortFunc(relevantSelectors, func(a, b Match[P]) int {
		if c := a.Specificity.Compare(b.Specificity); c != 0 {
			return c
		}
		return a.Order - b.Order
	})
	return relevantSelectors
}

// addRelevantSelectors is Matcher.add_relevant_selectors.
func addRelevantSelectors[P any](element *ElementWrapper, selectors []matcherEntry[P], relevantSelectors []Match[P]) []Match[P] {
	for _, entry := range selectors {
		if entry.selector.Test(element) {
			relevantSelectors = append(relevantSelectors, Match[P]{
				entry.selector.Specificity, entry.order, entry.selector.PseudoElement, entry.payload})
		}
	}
	return relevantSelectors
}
