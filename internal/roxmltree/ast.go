// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

// This file ports to_yaml of roxmltree tests/ast.rs. It is not a test file,
// so that the Rust oracle of the test module can compare the tree of each
// SVG file of resvg with the same text.

import (
	"cmp"
	"slices"
	"strings"
)

// ToYAML is to_yaml in roxmltree tests/ast.rs. It writes the tree of doc as
// the YAML text that the AST tests of upstream compare.
func ToYAML(doc *Document) string {
	var s strings.Builder
	toYAMLInner(doc, &s)
	return s.String()
}

// toYAMLInner is _to_yaml in roxmltree tests/ast.rs.
func toYAMLInner(doc *Document, s *strings.Builder) {
	if !doc.Root().HasChildren() {
		s.WriteString("Document:")
		return
	}

	s.WriteString("Document:\n")
	printChildrenYAML(doc.Root(), 1, s)
}

// writelnIndentedYAML is the macro writeln_indented in roxmltree
// tests/ast.rs.
func writelnIndentedYAML(depth int, s *strings.Builder, text string) {
	for range depth {
		s.WriteString("  ")
	}
	s.WriteString(text)
	s.WriteByte('\n')
}

// printChildrenYAML is the function print_children inside _to_yaml in
// roxmltree tests/ast.rs.
func printChildrenYAML(parent Node, depth int, s *strings.Builder) {
	for child := range parent.Children() {
		switch child.NodeType() {
		case NodeTypeElement:
			writelnIndentedYAML(depth, s, "- Element:")

			if ns, ok := child.TagName().Namespace(); ok && ns != "" {
				writelnIndentedYAML(depth+2, s, "tag_name: "+child.TagName().Name()+"@"+ns)
			} else {
				writelnIndentedYAML(depth+2, s, "tag_name: "+child.TagName().Name())
			}

			type pair struct{ name, value string }

			var attrs []pair
			for attr := range child.Attributes() {
				if ns, ok := attr.Namespace(); ok {
					attrs = append(attrs, pair{attr.Name() + "@" + ns, attr.Value()})
				} else {
					attrs = append(attrs, pair{attr.Name(), attr.Value()})
				}
			}
			if len(attrs) != 0 {
				slices.SortStableFunc(attrs, func(a, b pair) int { return cmp.Compare(a.name, b.name) })

				writelnIndentedYAML(depth+2, s, "attributes:")
				for _, a := range attrs {
					writelnIndentedYAML(depth+3, s, a.name+": "+rustDebugStr(a.value))
				}
			}

			var nsList []pair
			for ns := range child.Namespaces() {
				name, ok := ns.Name()
				if !ok {
					name = rustNone
				}
				uri := ns.URI()
				if uri == "" {
					uri = `""`
				}
				nsList = append(nsList, pair{name, uri})
			}
			if len(nsList) != 0 {
				slices.SortStableFunc(nsList, func(a, b pair) int { return cmp.Compare(a.name, b.name) })

				writelnIndentedYAML(depth+2, s, "namespaces:")
				for _, ns := range nsList {
					writelnIndentedYAML(depth+3, s, ns.name+": "+ns.value)
				}
			}

			if child.HasChildren() {
				writelnIndentedYAML(depth+2, s, "children:")
				printChildrenYAML(child, depth+3, s)
			}
		case NodeTypeText:
			text, _ := child.Text()
			writelnIndentedYAML(depth, s, "- Text: "+rustDebugStr(text))
		case NodeTypeComment:
			text, _ := child.Text()
			writelnIndentedYAML(depth, s, "- Comment: "+rustDebugStr(text))
		case NodeTypePI:
			if p, _ := child.Parent(); p.IsRoot() {
				continue
			}

			writelnIndentedYAML(depth, s, "- PI:")

			pi, _ := child.PI()
			writelnIndentedYAML(depth+2, s, "target: "+rustDebugStr(pi.Target))
			if pi.Value != "" {
				writelnIndentedYAML(depth+2, s, "value: "+rustDebugStr(pi.Value))
			}
		case NodeTypeRoot:
		}
	}
}
