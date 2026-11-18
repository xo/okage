// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"slices"
	"strconv"
	"testing"
)

// This file is not a port. It tests that the groups of selectors of the
// Matcher of D61 give the matches of a test of each selector on each
// element, and it measures the cost of Match.

// matcherSelectors are the selectors of TestMatchesEachSelector: the
// selectors of TestMatcher and of select.txt, and selectors with :is() and
// :where() that the Matcher keeps with the names of their subject (D61).
func matcherSelectors(t *testing.T) []string {
	t.Helper()
	selectors := []string{
		"*", "div", "DIV", "li", ".c", ".a.b", "#first-li", "li#first-li",
		"ol > li", "ol li.c", "[lang]", `[lang|="en"]`, ":lang(en)", "a[href]",
		"|li", ":first-child", "li:nth-child(2n)", "div::before", "li::marker",
		":hover", ":is(li, div)", ":where(li)", "p:has(input)", "#outer-div *",
		"html|*",
		":is(li, LI)", ":is(Li)", ":where(div, p) > :is(li, ol)", ":is(li):is(ol, li)",
		":is(li):is(div)", ":is(*, li)", ":is(li, [id])", ":is(li, div *)",
		":is(:is(li, p), :where(a, Div))", "ol :is(li, a)::marker", ":is(li, x|y)",
		"html|li:is(li)", ":is(li, :not(div))", ":is(li, :has(p))", "div :where(:is(p, li) a, li)",
	}
	for _, c := range readCases(t, "select") {
		if !c.isError() {
			selectors = append(selectors, c.selector)
		}
	}
	return selectors
}

// mixedCaseDocument returns a document whose names are not all in lower
// case, in the XHTML namespace and in no namespace.
func mixedCaseDocument() *testElement {
	root := newTestElement(xhtmlNamespace, "html")
	for i, name := range []string{"li", "LI", "Li", "div", "DIV", "Div", "ol", "OL", "p", "a"} {
		ns := xhtmlNamespace
		if i%2 == 1 {
			ns = ""
		}
		child := newTestElement(ns, name, "id", "e"+strconv.Itoa(i))
		child.children = []*testElement{newTestElement(xhtmlNamespace, "li"), newTestElement("", "LI")}
		root.children = append(root.children, child)
	}
	return root
}

func TestMatchesEachSelector(t *testing.T) {
	namespaces := Namespaces{"html": xhtmlNamespace, "x": "http://www.example.org/ns"}
	matcher := NewMatcher[int]()
	var all []*CompiledSelector
	for i, selector := range matcherSelectors(t) {
		compiled, err := CompileSelectorList(selector, namespaces)
		if err != nil {
			continue
		}
		for _, s := range compiled {
			matcher.AddSelector(s, i)
			all = append(all, s)
		}
	}
	documents := map[string]*testElement{
		"ids":     parseDocument(t, "ids.html"),
		"content": testDocument(t),
		"mixed":   mixedCaseDocument(),
	}
	for name, document := range documents {
		for _, html := range []bool{false, true} {
			root := FromXMLRoot(document, "")
			if html {
				root = FromHTMLRoot(document, "")
			}
			var buffer []Match[int]
			for element := range root.IterSubtree() {
				// The selectors that match, in the order of AddSelector.
				var want []orderedMatch
				for i, s := range all {
					if !s.NeverMatches && s.Test(element) {
						want = append(want, orderedMatch{i + 1, s.PseudoElement})
					}
				}
				var got []orderedMatch
				matches := matcher.Match(element)
				for _, m := range matches {
					got = append(got, orderedMatch{m.Order, m.PseudoElement})
				}
				if !slices.IsSortedFunc(matches, compareMatches) {
					t.Errorf("%s html=%v %s: the matches are not sorted", name, html, element.LocalName())
				}
				buffer = matcher.AppendMatches(buffer[:0], element)
				if !slices.Equal(buffer, matches) {
					t.Errorf("%s html=%v %s: AppendMatches differs from Match", name, html, element.LocalName())
				}
				slices.SortFunc(got, func(a, b orderedMatch) int { return a.order - b.order })
				if !slices.Equal(got, want) {
					t.Errorf("%s html=%v %s: matches %v, want %v", name, html, element.LocalName(), got, want)
				}
			}
		}
	}
}

// orderedMatch is the order and the pseudo-element of a match.
type orderedMatch struct {
	order         int
	pseudoElement string
}

// benchmarkSelectors are selectors in the style of the user-agent style sheet
// of a renderer of HTML. Most of them name an element. Some have :is() or
// only an attribute, and some have a pseudo-element.
var benchmarkSelectors = []string{
	"[hidden]", "area, base, head, link, meta, script, style, template, title",
	"input[type=hidden i]", "menu[type=context i]",
	"address, article, aside, blockquote, body, center, dd, details, div, dl, dt, fieldset",
	"figure, footer, form, h1, h2, h3, h4, h5, h6, header, hr, html, legend, main, nav, ol, p, pre, section, ul",
	"button, input, select, textarea", "li", "table", "caption", "colgroup, colgroup[hidden]",
	"col, col[hidden]", "thead, thead[hidden]", "tbody, tbody[hidden]", "tfoot, tfoot[hidden]",
	"tr, tr[hidden]", "td, th, td[hidden], th[hidden]",
	":is(colgroup, col, thead, tbody, tfoot, tr, td, th)[hidden]",
	":is(dir, dl, menu, ol, ul) :is(dir, dl, menu, ol, ul)",
	"[dir=ltr i] dd", "[dir=rtl i] dd", "[dir] [dir=ltr i] dd",
	"[dir=ltr i] :is(dir, menu, ol, ul)", "[dir=rtl i] :is(dir, menu, ol, ul)",
	"[dir] [dir=ltr i] :is(dir, menu, ol, ul)", "[dir] [dir=rtl i] :is(dir, menu, ol, ul)",
	"table:is([rules], [frame])", "table[frame=box i]", "table[rules=all i] > tr > td",
	"table[rules=all i] > :is(thead, tbody, tfoot) > tr > :is(td, th)",
	":is(td, th)[nowrap]", "td[align=left i]", "tr[valign=top i] > :is(td, th)",
	":link", ":link, :visited, ins, u", "a:visited", "b, strong", "i, em, cite, var, dfn",
	"code, kbd, samp, tt, pre", "q::before", "q::after", "::marker", "li::marker",
	"[dir=ltr i]", "[dir=rtl i]", ":lang(en) q", "::footnote-call", "::footnote-marker",
	":is(input, input[value=''], input[type=checkbox i], input[type=radio i])",
	"tbody > tr:nth-child(2n)", "tr > td:first-child", "td:last-child", "html:root",
}

// benchmarkTable returns an HTML document with a table of rows rows and cols
// columns.
func benchmarkTable(rows, cols int) *testElement {
	tbody := newTestElement(xhtmlNamespace, "tbody")
	for r := range rows {
		tr := newTestElement(xhtmlNamespace, "tr")
		for c := range cols {
			td := newTestElement(xhtmlNamespace, "td")
			td.text = strconv.Itoa(r*cols + c)
			tr.children = append(tr.children, td)
		}
		tbody.children = append(tbody.children, tr)
	}
	table := newTestElement(xhtmlNamespace, "table")
	table.children = []*testElement{tbody}
	body := newTestElement(xhtmlNamespace, "body")
	body.children = []*testElement{table}
	html := newTestElement(xhtmlNamespace, "html", "lang", "en")
	html.children = []*testElement{newTestElement(xhtmlNamespace, "head"), body}
	return html
}

// benchmarkMatcher returns a Matcher of benchmarkSelectors and the elements
// of a table of 1,000 rows and 10 columns. A table has 11,006 elements, so
// the time of an operation is the time of 11,006 matches.
func benchmarkMatcher(b *testing.B) (*Matcher[int], []*ElementWrapper) {
	b.Helper()
	matcher := NewMatcher[int]()
	for i, selector := range benchmarkSelectors {
		compiled, err := CompileSelectorList(selector, nil)
		if err != nil {
			b.Fatal(err)
		}
		for _, s := range compiled {
			matcher.AddSelector(s, i)
		}
	}
	var elements []*ElementWrapper
	for e := range FromHTMLRoot(benchmarkTable(1000, 10), "").IterSubtree() {
		elements = append(elements, e)
	}
	return matcher, elements
}

func BenchmarkMatchTable(b *testing.B) {
	matcher, elements := benchmarkMatcher(b)
	for b.Loop() {
		for _, e := range elements {
			_ = matcher.Match(e)
		}
	}
}

func BenchmarkAppendMatchesTable(b *testing.B) {
	matcher, elements := benchmarkMatcher(b)
	var buffer []Match[int]
	for b.Loop() {
		for _, e := range elements {
			buffer = matcher.AppendMatches(buffer[:0], e)
		}
	}
}
