// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"bufio"
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/roxmltree"
)

// This file ports cssselect2 tests/test_cssselect2.py. testdata/make.py runs
// upstream on the cases of testdata/*.txt and writes its results, and the
// tests compare the port with them. Where upstream raises an exception other
// than SelectorError, the port returns a SelectorError (D50).

// testElement is an element of an in-memory tree, as an Element of
// ElementTree is.
type testElement struct {
	localName    string
	namespaceURL string
	attrs        map[[2]string]string
	text         string
	children     []*testElement
}

func (e *testElement) Children() iter.Seq[Element] {
	return func(yield func(Element) bool) {
		for _, c := range e.children {
			if !yield(c) {
				return
			}
		}
	}
}

func (e *testElement) LocalName() string    { return e.localName }
func (e *testElement) NamespaceURL() string { return e.namespaceURL }
func (e *testElement) Text() string         { return e.text }

func (e *testElement) Attr(namespaceURL, name string) (string, bool) {
	v, ok := e.attrs[[2]string{namespaceURL, name}]
	return v, ok
}

// newTestElement makes an element.
func newTestElement(namespaceURL, localName string, attrs ...string) *testElement {
	e := &testElement{localName: localName, namespaceURL: namespaceURL, attrs: map[[2]string]string{}}
	for i := 0; i+1 < len(attrs); i += 2 {
		e.attrs[[2]string{"", attrs[i]}] = attrs[i+1]
	}
	return e
}

// fromRoxmltree copies an element of roxmltree into the in-memory tree, as
// the parser of ElementTree builds it: the comments and the processing
// instructions are dropped.
func fromRoxmltree(n roxmltree.Node) *testElement {
	name := n.TagName()
	ns, _ := name.Namespace()
	e := &testElement{localName: name.Name(), namespaceURL: ns, attrs: map[[2]string]string{}}
	for a := range n.Attributes() {
		ans, _ := a.Namespace()
		e.attrs[[2]string{ans, a.Name()}] = a.Value()
	}
	var text strings.Builder
	seenElement := false
	for c := range n.Children() {
		switch {
		case c.IsElement():
			seenElement = true
			e.children = append(e.children, fromRoxmltree(c))
		case c.IsText() && !seenElement:
			t, _ := c.Text()
			text.WriteString(t)
		}
	}
	e.text = text.String()
	return e
}

// parseDocument parses an XML document of testdata into the in-memory tree.
func parseDocument(t *testing.T, name string) *testElement {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return parseString(t, string(data))
}

// parseString parses an XML document into the in-memory tree.
func parseString(t *testing.T, s string) *testElement {
	t.Helper()
	opt := roxmltree.DefaultParsingOptions()
	opt.AllowDTD = true
	doc, err := roxmltree.ParseWithOptions(s, opt)
	if err != nil {
		t.Fatal(err)
	}
	return fromRoxmltree(doc.RootElement())
}

// find returns the first element of the subtree with the given ID.
func find(e *testElement, id string) *testElement {
	if e.attrs[[2]string{"", "id"}] == id {
		return e
	}
	for _, c := range e.children {
		if f := find(c, id); f != nil {
			return f
		}
	}
	return nil
}

// testDocument is get_test_document of the tests of upstream.
func testDocument(t *testing.T) *testElement {
	t.Helper()
	document := parseDocument(t, "content.xhtml")
	parent := find(document, "root")
	const xhtml = "http://www.w3.org/1999/xhtml"
	// Setup namespace tests
	for _, id := range []string{"any-namespace", "no-namespace"} {
		div := newTestElement(xhtml, "div", "id", id)
		div.children = []*testElement{
			newTestElement(xhtml, "div", "id", id+"-div1"),
			newTestElement(xhtml, "div", "id", id+"-div2"),
			newTestElement("", "div", "id", id+"-div3"),
			newTestElement("http://www.example.org/ns", "div", "id", id+"-div4"),
		}
		parent.children = append(parent.children, div)
	}
	return document
}

// testCase is a line of a file of testdata.
type testCase struct {
	line     int
	selector string
	want     string
}

// readCases reads testdata/<name>.txt.
func readCases(t *testing.T, name string) []testCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var cases []testCase
	s := bufio.NewScanner(f)
	s.Buffer(nil, 1<<20)
	for n := 1; s.Scan(); n++ {
		input, want, ok := strings.Cut(s.Text(), "\t")
		if !ok {
			t.Fatalf("%s.txt:%d: no tab", name, n)
		}
		selector, err := strconv.Unquote(input)
		if err != nil {
			t.Fatalf("%s.txt:%d: %v", name, n, err)
		}
		cases = append(cases, testCase{n, selector, want})
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return cases
}

// isError reports whether upstream raised an exception for the case.
func (c testCase) isError() bool {
	return c.want == `"SelectorError"` || strings.HasPrefix(c.want, `["exception", `)
}

// runCases compiles the selector of each case. It checks that the port
// returns a SelectorError where upstream raises an exception, and otherwise
// compares the JSON of f with the result of upstream.
func runCases(t *testing.T, name string, f func([]*CompiledSelector) any) {
	t.Helper()
	for _, c := range readCases(t, name) {
		selectors, err := CompileSelectorList(c.selector, nil)
		if c.isError() {
			var selectorError *SelectorError
			if !errors.Is(err, okage.ErrCSSSelector) || !errors.As(err, &selectorError) {
				t.Errorf("%s.txt:%d: %q: got %v, want a SelectorError", name, c.line, c.selector, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s.txt:%d: %q: %v", name, c.line, c.selector, err)
			continue
		}
		if got := toJSON(f(selectors)); got != c.want {
			t.Errorf("%s.txt:%d: %q\n got: %s\nwant: %s", name, c.line, c.selector, got, c.want)
		}
	}
}

// toJSON is json.dumps of Python for the values of the tests.
func toJSON(v any) string {
	switch v := v.(type) {
	case nil:
		return "null"
	case string:
		return strconv.Quote(v)
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = toJSON(x)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	panic(fmt.Sprintf("toJSON: %T", v))
}

// ids returns the ID of each element, or "nil" for an element with none.
func ids(elements iter.Seq[*ElementWrapper]) []any {
	result := []any{}
	for e := range elements {
		id, ok := e.Element.Attr("", "id")
		if !ok {
			id = "nil"
		}
		result = append(result, id)
	}
	return result
}

func TestSelect(t *testing.T) {
	root := parseDocument(t, "ids.html")
	runCases(t, "select", func(selectors []*CompiledSelector) any {
		return []any{
			ids(FromXMLRoot(root, "").QueryAll(selectors...)),
			ids(FromHTMLRoot(root, "").QueryAll(selectors...)),
		}
	})
}

func TestSpecificity(t *testing.T) {
	runCases(t, "specificity", func(selectors []*CompiledSelector) any {
		result := []any{}
		for _, s := range selectors {
			var pseudo any
			if s.PseudoElement != "" {
				pseudo = s.PseudoElement
			}
			result = append(result, []any{
				[]any{s.Specificity[0], s.Specificity[1], s.Specificity[2]}, pseudo, s.NeverMatches})
		}
		return result
	})
}

func TestValidSelectors(t *testing.T) {
	document := testDocument(t)
	runCases(t, "w3_valid", func(selectors []*CompiledSelector) any {
		result := []any{}
		for e := range FromXMLRoot(document, "").QueryAll(selectors...) {
			if id, ok := e.ID(); ok {
				result = append(result, id)
			} else {
				result = append(result, nil)
			}
		}
		return result
	})
}

func TestInvalidSelectors(t *testing.T) {
	runCases(t, "w3_invalid", func(selectors []*CompiledSelector) any {
		return len(selectors)
	})
	for _, c := range readCases(t, "w3_invalid") {
		if !c.isError() {
			t.Errorf("w3_invalid.txt:%d: %q is valid in upstream", c.line, c.selector)
		}
	}
}

func TestSelectShakespeare(t *testing.T) {
	// find(.//{xhtml}body) of upstream.
	var findBody func(e *testElement) *testElement
	findBody = func(e *testElement) *testElement {
		if e.localName == "body" && e.namespaceURL == xhtmlNamespace {
			return e
		}
		for _, c := range e.children {
			if b := findBody(c); b != nil {
				return b
			}
		}
		return nil
	}
	body := findBody(parseDocument(t, "shakespeare.html"))
	root := FromXMLRoot(body, "")
	runCases(t, "shakespeare", func(selectors []*CompiledSelector) any {
		n := 0
		for range root.QueryAll(selectors...) {
			n++
		}
		return n
	})
}

func TestMatcher(t *testing.T) {
	selectors := []string{
		"*", "div", "DIV", "li", ".c", ".a.b", "#first-li", "li#first-li",
		"ol > li", "ol li.c", "[lang]", `[lang|="en"]`, ":lang(en)", "a[href]",
		"|li", ":first-child", "li:nth-child(2n)", "div::before", "li::marker",
		":hover", ":is(li, div)", ":where(li)", "p:has(input)", "#outer-div *",
		"html|*",
	}
	root := parseDocument(t, "ids.html")
	for _, c := range readCases(t, "matcher") {
		matcher := NewMatcher[int]()
		for i, selector := range selectors {
			compiled, err := CompileSelectorList(selector, Namespaces{"html": xhtmlNamespace})
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range compiled {
				matcher.AddSelector(s, i)
			}
		}
		wrap := FromXMLRoot
		if c.selector == "html" {
			wrap = FromHTMLRoot
		}
		result := []any{}
		for e := range wrap(root, "").IterSubtree() {
			id, ok := e.Element.Attr("", "id")
			if !ok {
				id = "nil"
			}
			matches := []any{}
			for _, m := range matcher.Match(e) {
				var pseudo any
				if m.PseudoElement != "" {
					pseudo = m.PseudoElement
				}
				matches = append(matches, []any{
					[]any{m.Specificity[0], m.Specificity[1], m.Specificity[2]}, m.Order, pseudo, m.Payload})
			}
			result = append(result, []any{id, matches})
		}
		if got := toJSON(result); got != c.want {
			t.Errorf("matcher.txt:%d: %s\n got: %s\nwant: %s", c.line, c.selector, got, c.want)
		}
	}
}

// matches compiles selector and reports whether root matches it.
func matches(t *testing.T, root *ElementWrapper, selector string) bool {
	t.Helper()
	compiled, err := CompileSelectorList(selector, nil)
	if err != nil {
		t.Fatal(err)
	}
	return root.Matches(compiled...)
}

func TestLang(t *testing.T) {
	doc := parseString(t, `
        <html xmlns="http://www.w3.org/1999/xhtml"></html>
    `)
	if matches(t, FromXMLRoot(doc, ""), ":lang(fr)") {
		t.Error(":lang(fr) matches")
	}

	doc = parseString(t, "\n        <html xmlns=\"http://www.w3.org/1999/xhtml\">\n"+
		"            <meta http-equiv=\"Content-Language\" content=\" fr \t\"/>\n        </html>\n    ")
	if !matches(t, FromXMLRoot(doc, "en"), ":lang(fr)") {
		t.Error(":lang(fr) does not match")
	}

	doc = parseString(t, "\n        <html>\n"+
		"            <meta http-equiv=\"Content-Language\" content=\" fr \t\"/>\n        </html>\n    ")
	if !matches(t, FromXMLRoot(doc, "en"), ":lang(en)") {
		t.Error(":lang(en) does not match")
	}

	doc = parseString(t, "<html></html>")
	if !matches(t, FromXMLRoot(doc, "en"), ":lang(en)") {
		t.Error(":lang(en) does not match")
	}
	if matches(t, FromXMLRoot(doc, "en, es"), ":lang(en)") {
		t.Error(":lang(en) matches")
	}
	if matches(t, FromXMLRoot(doc, ""), ":lang(en)") {
		t.Error(":lang(en) matches")
	}

	doc = parseString(t, `<html lang="eN"></html>`)
	if !matches(t, FromHTMLRoot(doc, ""), ":lang(en)") {
		t.Error(":lang(en) does not match")
	}
	if matches(t, FromXMLRoot(doc, ""), ":lang(en)") {
		t.Error(":lang(en) matches")
	}
}

func TestQuery(t *testing.T) {
	root := FromHTMLRoot(parseDocument(t, "ids.html"), "")
	compiled, err := CompileSelectorList("li.c", nil)
	if err != nil {
		t.Fatal(err)
	}
	e := root.Query(compiled...)
	if id, _ := e.ID(); id != "third-li" {
		t.Errorf("Query = %q", id)
	}
	if root.Query() != nil {
		t.Error("Query() is not nil")
	}
	var nested strings.Builder
	for range MaxDepth + 1 {
		nested.WriteString(":is(")
	}
	nested.WriteString("a")
	if _, err := CompileSelectorList(nested.String(), nil); !errors.Is(err, okage.ErrCSSSelector) {
		t.Errorf("nested :is() = %v", err)
	}
}
