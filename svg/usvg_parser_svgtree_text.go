// Copyright 2021 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"strings"

	"github.com/xo/okage/internal/roxmltree"
	"github.com/xo/okage/internal/simplecss"
	"github.com/xo/okage/internal/svgtypes"
)

// This file ports usvg src/parser/svgtree/text.rs. The crate compiles it
// without the feature text too, because the svgtree keeps the text nodes.

// parseSvgTextElement is parse_svg_text_element in usvg
// src/parser/svgtree/text.rs.
func parseSvgTextElement(
	parent roxmltree.Node,
	parentID nodeID,
	styleSheet *simplecss.StyleSheet,
	doc *document,
) error {
	space := xmlSpaceDefault
	if doc.get(parentID).hasAttribute(AIdSpace) {
		space = getXMLSpace(doc, parentID, xmlSpaceDefault)
	} else {
		for node := range doc.get(parentID).ancestors() {
			if node.hasAttribute(AIdSpace) {
				space = getXMLSpace(doc, node.id, xmlSpaceDefault)
				break
			}
		}
	}

	if err := parseSvgTextElementImpl(parent, parentID, styleSheet, space, doc); err != nil {
		return err
	}

	trimTextNodes(parentID, space, doc)
	return nil
}

// parseSvgTextElementImpl is parse_svg_text_element_impl in usvg
// src/parser/svgtree/text.rs.
func parseSvgTextElementImpl(
	parent roxmltree.Node,
	parentID nodeID,
	styleSheet *simplecss.StyleSheet,
	space xmlSpace,
	doc *document,
) error {
	for node := range parent.Children() {
		if node.IsText() {
			t, _ := node.Text()
			text := trimText(t, space)
			doc.append(parentID, nodeKind{kind: nodeKindText, text: text})
			continue
		}

		tagName, ok := parseTagName(node)
		if !ok {
			continue
		}

		if tagName == EIdA {
			// Treat links as simple text.
			tagName = EIdTspan
		}

		if tagName != EIdTspan && tagName != EIdTref && tagName != EIdTextPath {
			continue
		}

		// `textPath` must be a direct `text` child.
		if tagName == EIdTextPath && parent.TagName().Name() != "text" { //nolint:goconst // an SVG keyword, as upstream writes it
			continue
		}

		// We are converting `tref` into `tspan` to simplify later use.
		isTref := false
		if tagName == EIdTref {
			tagName = EIdTspan
			isTref = true
		}

		nodeID, err := parseSvgElement(node, parentID, tagName, styleSheet, false, doc)
		if err != nil {
			return err
		}
		space := getXMLSpace(doc, nodeID, space)

		if isTref {
			linkValue, ok := node.Attribute(roxmltree.ExpandedNameFromTuple(xlinkNS, "href"))
			if !ok {
				linkValue, ok = node.Attribute(roxmltree.ExpandedNameFromStr("href"))
			}

			if ok {
				if text, ok := resolveTrefText(node.Document(), linkValue); ok {
					text = trimText(text, space)
					doc.append(nodeID, nodeKind{kind: nodeKindText, text: text})
				}
			}
		} else if err := parseSvgTextElementImpl(node, nodeID, styleSheet, space, doc); err != nil {
			return err
		}
	}

	return nil
}

// resolveTrefText is resolve_tref_text in usvg src/parser/svgtree/text.rs.
func resolveTrefText(xml *roxmltree.Document, href string) (string, bool) {
	iri, err := svgtypes.ParseIRI(href)
	if err != nil {
		return "", false
	}
	id := string(iri)

	// Find linked element in the original tree.
	var node roxmltree.Node
	found := false
	for n := range xml.Descendants() {
		if v, ok := n.Attribute(roxmltree.ExpandedNameFromStr("id")); ok && v == id {
			node, found = n, true
			break
		}
	}
	if !found {
		return "", false
	}

	// `tref` should be linked to an SVG element.
	if _, ok := parseTagName(node); !ok {
		return "", false
	}

	// 'All character data within the referenced element, including character data enclosed
	// within additional markup, will be rendered.'
	//
	// So we don't care about attributes and everything. Just collecting text nodes data.
	//
	// Note: we have to filter nodes by `is_text()` first since `text()` will look up
	// for text nodes in element children therefore we will get duplicates.
	var b strings.Builder
	for n := range node.Descendants() {
		if !n.IsText() {
			continue
		}
		if t, ok := n.Text(); ok {
			b.WriteString(t)
		}
	}
	text := b.String()
	if text == "" {
		return "", false
	}
	return text, true
}

// xmlSpace is XmlSpace in usvg src/parser/svgtree/text.rs.
type xmlSpace uint8

// The values of xmlSpace.
const (
	xmlSpaceDefault xmlSpace = iota
	xmlSpacePreserve
)

// getXMLSpace is get_xmlspace in usvg src/parser/svgtree/text.rs.
func getXMLSpace(doc *document, nodeID nodeID, def xmlSpace) xmlSpace {
	v, ok := doc.get(nodeID).attributeStr(AIdSpace)
	switch {
	case ok && v == "preserve":
		return xmlSpacePreserve
	case ok:
		return xmlSpaceDefault
	}
	return def
}

// removeFirstSpace is StrTrim::remove_first_space for String in usvg
// src/parser/svgtree/text.rs.
func removeFirstSpace(s *string) {
	*s = (*s)[1:]
}

// removeLastSpace is StrTrim::remove_last_space for String in usvg
// src/parser/svgtree/text.rs.
func removeLastSpace(s *string) {
	*s = (*s)[:len(*s)-1]
}

// textOf returns the text of the text node id, for a change.
func (d *document) textOf(id nodeID) *string {
	return &d.nodes[id.getUsize()].kind.text
}

// trimTextNodes is trim_text_nodes in usvg src/parser/svgtree/text.rs. It
// prepares text nodes according to the spec:
// https://www.w3.org/TR/SVG11/text.html#WhiteSpace
//
// This function handles:
// - 'xml:space' processing
// - tabs and newlines removing/replacing
// - spaces trimming.
func trimTextNodes(textElemID nodeID, xmlspace xmlSpace, doc *document) {
	var nodes []textNodeDepth // TODO: allocate only once
	collectTextNodes(doc.get(textElemID), 0, &nodes)

	// `trim` method has already collapsed all spaces into a single one,
	// so we have to check only for one leading or trailing space.

	if len(nodes) == 1 {
		// Process element with a single text node child.

		nodeID := nodes[0].id

		if xmlspace == xmlSpaceDefault {
			if doc.nodes[nodeID.getUsize()].kind.kind == nodeKindText {
				text := doc.textOf(nodeID)
				switch len(*text) {
				case 0: // An empty string. Do nothing.
				case 1:
					// If string has only one character and it's a space - clear this string.
					if (*text)[0] == ' ' {
						*text = ""
					}
				default:
					// 'text' has at least 2 bytes, so indexing is safe.
					c1 := (*text)[0]
					c2 := (*text)[len(*text)-1]

					if c1 == ' ' {
						removeFirstSpace(text)
					}

					if c2 == ' ' {
						removeLastSpace(text)
					}
				}
			}
		}
		// Do nothing when xml:space=preserve.
	} else if len(nodes) > 1 {
		// Process element with many text node children.

		// We manage all text nodes as a single text node
		// and trying to remove duplicated spaces across nodes.
		//
		// For example    '<text>Text <tspan> text </tspan> text</text>'
		// is the same is '<text>Text <tspan>text</tspan> text</text>'

		i := 0
		length := len(nodes) - 1
		var lastNonEmpty nodeID
		hasLastNonEmpty := false
		for i < length {
			// Process pairs.
			node1ID, depth1 := nodes[i].id, nodes[i].depth
			node2ID, depth2 := nodes[i+1].id, nodes[i+1].depth

			if doc.get(node1ID).text() == "" {
				if hasLastNonEmpty {
					node1ID = lastNonEmpty
				}
			}

			// Parent of the text node is always an element node and always exist,
			// so unwrap is safe.
			p1, _ := doc.get(node1ID).parent()
			p2, _ := doc.get(node2ID).parent()
			xmlspace1 := getXMLSpace(doc, p1.id, xmlspace)
			xmlspace2 := getXMLSpace(doc, p2.id, xmlspace)

			// >text<..>text<
			//  1  2    3  4
			text1 := doc.get(node1ID).text()
			text2 := doc.get(node2ID).text()
			c1, hasC1 := firstByte(text1)
			c2, hasC2 := lastByte(text1)
			c3, hasC3 := firstByte(text2)
			c4, hasC4 := lastByte(text2)

			// NOTE: xml:space processing is mostly an undefined behavior,
			// because everyone do it differently.
			// We're mimicking the Chrome behavior.

			// Remove space from the second text node if both nodes has bound spaces.
			// From: '<text>Text <tspan> text</tspan></text>'
			// To:   '<text>Text <tspan>text</tspan></text>'
			//
			// See text-tspan-02-b.svg for details.
			if depth1 < depth2 {
				if hasC3 && c3 == ' ' {
					if xmlspace2 == xmlSpaceDefault {
						if doc.nodes[node2ID.getUsize()].kind.kind == nodeKindText {
							removeFirstSpace(doc.textOf(node2ID))
						}
					}
				}
			} else if hasC2 && c2 == ' ' && hasC3 && c2 == c3 {
				if xmlspace1 == xmlSpaceDefault && xmlspace2 == xmlSpaceDefault {
					if doc.nodes[node1ID.getUsize()].kind.kind == nodeKindText {
						removeLastSpace(doc.textOf(node1ID))
					}
				} else if xmlspace1 == xmlSpacePreserve && xmlspace2 == xmlSpaceDefault {
					if doc.nodes[node2ID.getUsize()].kind.kind == nodeKindText {
						removeFirstSpace(doc.textOf(node2ID))
					}
				}
			}

			isFirst := i == 0
			isLast := i == length-1

			if isFirst &&
				hasC1 && c1 == ' ' &&
				xmlspace1 == xmlSpaceDefault &&
				doc.get(node1ID).text() != "" {
				// Remove a leading space from a first text node.
				if doc.nodes[node1ID.getUsize()].kind.kind == nodeKindText {
					removeFirstSpace(doc.textOf(node1ID))
				}
			} else if isLast &&
				hasC4 && c4 == ' ' &&
				doc.get(node2ID).text() != "" &&
				xmlspace2 == xmlSpaceDefault {
				// Remove a trailing space from a last text node.
				// Also check that 'text2' is not empty already.
				if doc.nodes[node2ID.getUsize()].kind.kind == nodeKindText {
					removeLastSpace(doc.textOf(node2ID))
				}
			}

			if isLast &&
				hasC2 && c2 == ' ' &&
				doc.get(node1ID).text() != "" &&
				doc.get(node2ID).text() == "" &&
				strings.HasSuffix(doc.get(node1ID).text(), " ") {
				if doc.nodes[node1ID.getUsize()].kind.kind == nodeKindText {
					removeLastSpace(doc.textOf(node1ID))
				}
			}

			if strings.TrimFunc(doc.get(node1ID).text(), isRustWhitespace) != "" {
				lastNonEmpty, hasLastNonEmpty = node1ID, true
			}

			i++
		}
	}

	// TODO: find a way to remove all empty text nodes
}

// firstByte returns the first byte of s, or false for an empty s.
func firstByte(s string) (byte, bool) {
	if s == "" {
		return 0, false
	}
	return s[0], true
}

// lastByte returns the last byte of s, or false for an empty s.
func lastByte(s string) (byte, bool) {
	if s == "" {
		return 0, false
	}
	return s[len(s)-1], true
}

// isRustWhitespace is char::is_whitespace of Rust, which str::trim uses: the
// characters with the Unicode property White_Space.
func isRustWhitespace(c rune) bool {
	switch c {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680,
		0x2000, 0x2001, 0x2002, 0x2003, 0x2004, 0x2005, 0x2006, 0x2007, 0x2008, 0x2009, 0x200A,
		0x2028, 0x2029, 0x202F, 0x205F, 0x3000:
		return true
	}
	return false
}

// textNodeDepth is a (NodeId, usize) of upstream: a text node and its depth.
type textNodeDepth struct {
	id    nodeID
	depth int
}

// collectTextNodes is collect_text_nodes in usvg src/parser/svgtree/text.rs.
func collectTextNodes(parent svgNode, depth int, nodes *[]textNodeDepth) {
	for child := range parent.children() {
		if child.isText() {
			*nodes = append(*nodes, textNodeDepth{id: child.id, depth: depth})
		} else if child.isElement() {
			collectTextNodes(child, depth+1, nodes)
		}
	}
}

// trimText is trim_text in usvg src/parser/svgtree/text.rs.
func trimText(text string, space xmlSpace) string {
	var s strings.Builder
	s.Grow(len(text))

	prev := '0'
	for _, c := range text {
		// \r, \n and \t should be converted into spaces.
		switch c {
		case '\r', '\n', '\t':
			c = ' '
		}

		// Skip continuous spaces.
		if space == xmlSpaceDefault && c == ' ' && c == prev {
			continue
		}

		prev = c

		s.WriteRune(c)
	}

	return s.String()
}
