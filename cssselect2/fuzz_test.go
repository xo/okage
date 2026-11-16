// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"errors"
	"testing"

	"github.com/xo/okage"
)

// This file ports no upstream file. It holds the fuzz test of the selector
// parser and of the compiler (D50).

// FuzzCompileSelectorList checks that no selector makes the parser or the
// compiler panic or hang, that each error is a SelectorError, and that the
// compiled selectors match a small document without a panic.
func FuzzCompileSelectorList(f *testing.F) {
	for _, seed := range []string{
		"", "div", "a[href^=http i], ol > li:nth-child(2n+1 of .c)",
		":is(div, :::wrong) ~ p:has(> fieldset, + ol)", "ns|div *|*",
		":not(:lang(en, fr)) :nth-last-of-type(-n+3)", "[*|a]", ":lang()",
		"#x.y::before", ":local-link:enabled:checked:empty:root",
	} {
		f.Add(seed)
	}
	root := FromHTMLRoot(fuzzDocument(), "en")
	f.Fuzz(func(t *testing.T, selector string) {
		if len(selector) > 1024 {
			return
		}
		compiled, err := CompileSelectorList(selector, Namespaces{"": xhtmlNamespace, "ns": "urn:x"})
		if err != nil {
			if !errors.Is(err, okage.ErrCSSSelector) {
				t.Fatalf("error %v does not wrap ErrCSSSelector", err)
			}
			return
		}
		matcher := NewMatcher[int]()
		for i, s := range compiled {
			matcher.AddSelector(s, i)
			_ = s.Specificity
		}
		for e := range root.IterSubtree() {
			_ = matcher.Match(e)
		}
	})
}

// fuzzDocument builds a small HTML document in memory.
func fuzzDocument() *testElement {
	html := newTestElement(xhtmlNamespace, "html", "lang", "en")
	body := newTestElement(xhtmlNamespace, "body")
	p := newTestElement(xhtmlNamespace, "p", "id", "x", "class", "y c")
	p.text = "text"
	a := newTestElement(xhtmlNamespace, "a", "href", "//[x", "rel", "tag")
	fieldset := newTestElement(xhtmlNamespace, "fieldset", "disabled", "")
	legend := newTestElement(xhtmlNamespace, "legend")
	input := newTestElement(xhtmlNamespace, "input", "type", "checkbox", "checked", "")
	fieldset.children = []*testElement{legend, input}
	ol := newTestElement(xhtmlNamespace, "ol", "class", "c")
	for range 4 {
		ol.children = append(ol.children, newTestElement(xhtmlNamespace, "li"))
	}
	other := newTestElement("urn:x", "div")
	other.attrs[[2]string{xmlNamespace, "lang"}] = "fr"
	body.children = []*testElement{p, a, fieldset, ol, other}
	html.children = []*testElement{body}
	return html
}
