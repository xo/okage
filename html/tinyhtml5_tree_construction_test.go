// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 tests/test_tree_construction.py (D49). It runs
// the tests of html5lib-tests in testdata/tinyhtml5/tree-construction with
// the namespaced tree, the "ns" case of upstream.

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// convert is convert in tinyhtml5 tests/test_tree_construction.py.
func convert(data []string, stripChars int) []string {
	out := make([]string, len(data))
	for i, line := range data {
		if strings.HasPrefix(line, "|") {
			line = line[min(stripChars, len(line)):]
		}
		out[i] = line
	}
	return out
}

// etreeTag is the tag of an element of ElementTree: "{namespace}name".
func etreeTag(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return "{" + namespace + "}" + name
}

// serialize is serialize in tinyhtml5 tests/test_tree_construction.py.
func serialize(out []string, item *Node, indent int, skip bool) []string {
	pad := strings.Repeat("  ", indent)
	switch item.typ {
	case CommentNode:
		return append(out, pad+"<!-- "+item.data+" -->")
	case DoctypeNode:
		text := item.name
		if public := item.publicID; public != "" {
			if strings.Contains(public, `"`) {
				public = "'" + public + "'"
			} else {
				public = `"` + public + `"`
			}
			if item.name == "html" {
				text += " PUBLIC " + public
			} else {
				text += " " + public + ` ""`
			}
		} else if system := item.systemID; system != "" {
			system = `"` + system + `"`
			if item.name == "html" {
				text += " SYSTEM"
			} else {
				text += ` ""`
			}
			text += " " + system
		}
		return append(out, "<!DOCTYPE "+text+">")
	case TextNode:
		if item.data != "" {
			out = append(out, pad+`"`+item.data+`"`)
		}
		return out
	}
	if !skip {
		out = append(out, pad+"<"+etreeTag(item.namespace, item.name)+">")
		indent++
		pad = strings.Repeat("  ", indent)
		type kv struct{ key, value string }
		attrs := make([]kv, 0, len(item.attrs))
		for _, a := range item.attrs {
			attrs = append(attrs, kv{etreeTag(a.Namespace, a.Name), a.Value})
		}
		slices.SortFunc(attrs, func(a, b kv) int { return strings.Compare(a.key, b.key) })
		for _, a := range attrs {
			out = append(out, pad+a.key+`="`+a.value+`"`)
		}
	}
	for child := range item.Children() {
		out = serialize(out, child, indent, false)
	}
	return out
}

// treeConstructionXfails is _xfails in tinyhtml5
// tests/test_tree_construction.py.
var treeConstructionXfails = func() map[string]bool {
	ids := []string{
		"adoption01-17",

		"blocks-12",

		"foreign-fragment-2", "foreign-fragment-3", "foreign-fragment-18",
		"foreign-fragment-19", "foreign-fragment-22", "foreign-fragment-23",
		"foreign-fragment-26", "foreign-fragment-27", "foreign-fragment-30",
		"foreign-fragment-31", "foreign-fragment-34", "foreign-fragment-35",
		"foreign-fragment-39", "foreign-fragment-41", "foreign-fragment-60",
		"foreign-fragment-61", "foreign-fragment-62", "foreign-fragment-63",
		"foreign-fragment-66",

		"isindex-0", "isindex-1", "isindex-2", "isindex-3",

		"namespace-sensitivity-0",

		"ruby-0", "ruby-1", "ruby-2", "ruby-3", "ruby-5", "ruby-7",
		"ruby-10", "ruby-12", "ruby-15", "ruby-17", "ruby-20",

		"tests2-6", "tests2-7",

		"tests8-5",

		"tests11-2", "tests11-4", "tests11-5", "tests11-6",

		"tests18-15",

		"tests19-7", "tests19-14", "tests19-17",

		"tests25-7",

		"tests26-16", "tests26-17", "tests26-18", "tests26-19",

		"webik02-14", "webik02-15", "webik02-16",

		// Introduced by the fork.

		// Doctype.
		"doctype01-26", "doctype01-28", "doctype01-29", "doctype01-31", "doctype01-32",
		"doctype01-33", "doctype01-34", "doctype01-35", "doctype01-36",
		"tests6-46",
		"quirks01-0", "quirks01-1", "quirks01-2", "quirks01-3",

		// <template>, <search>.
		"template-111", "search-element-0", "search-element-1",

		// Mismatched elements.
		"webkit02-11", "webkit02-15", "webkit02-16", "webkit02-17",

		// Attributes namespace.
		"webkit02-22",

		// Namespace in fragment.
		"foreign-fragment-59", "foreign-fragment-65",
	}
	for i := range 7 {
		ids = append(ids, fmt.Sprintf("template-%d", i))
	}
	for i := 8; i < 39; i++ {
		ids = append(ids, fmt.Sprintf("template-%d", i))
	}
	for i := 40; i < 111; i++ {
		ids = append(ids, fmt.Sprintf("template-%d", i))
	}
	// <hr> in <options>.
	for i := 25; i < 35; i++ {
		ids = append(ids, fmt.Sprintf("webkit02-%d", i))
	}
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}()

// parseTestInput parses the input of a test as a string, as
// HTMLParser.parse and HTMLParser.parse_fragment of tinyhtml5 parser.py do
// for a str.
func parseTestInput(input, container string) *Document {
	p := newHTMLParser(newDocumentOptions(nil))
	p.parseStream(newUnicodeInputStream(input), container)
	return p.document(container != "")
}

// TestTreeConstruction is test_tree_construction in tinyhtml5
// tests/test_tree_construction.py.
func TestTreeConstruction(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "tinyhtml5", "tree-construction", "*.dat"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("finding the tests: %v", err)
	}
	passed, xfailed, xpassed := 0, 0, 0
	for _, path := range paths {
		stem := strings.TrimSuffix(filepath.Base(path), ".dat")
		for i, test := range readTestData(t, path, "data") {
			if _, ok := test["script-on"]; ok {
				continue
			}
			id := fmt.Sprintf("%s-%d", stem, i)
			t.Run(id, func(t *testing.T) {
				input := test["data"]
				container := test["document-fragment"]
				expected := strings.Join(convert(strings.Split(test["document"], "\n"), 2), "\n")

				document := parseTestInput(input, container)

				output := strings.Join(convert(serialize(nil, document.Root(), 0, true), 3), "\n")
				for _, ns := range []string{NamespaceHTML, NamespaceMathML, NamespaceSVG, NamespaceXLink, NamespaceXML, NamespaceXMLNS} {
					value := prefixes(ns)
					repl := value + " "
					if value == "html" {
						repl = ""
					}
					output = strings.ReplaceAll(output, "{"+ns+"}", repl)
				}

				switch {
				case treeConstructionXfails[id] && expected == output:
					xpassed++
					t.Logf("%s passes but is marked as xfail", id)
				case treeConstructionXfails[id]:
					xfailed++
				case expected != output:
					t.Errorf("\nInput:\n%s\n\nExpected:\n%s\n\nReceived:\n%s\n", input, expected, output)
				default:
					passed++
				}
			})
		}
	}
	t.Logf("%d passed, %d expected failures, %d unexpected passes", passed, xfailed, xpassed)
}
