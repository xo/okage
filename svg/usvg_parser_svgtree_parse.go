// Copyright 2021 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/roxmltree"
	"github.com/xo/okage/internal/simplecss"
	"github.com/xo/okage/internal/svgtypes"
)

// This file ports usvg src/parser/svgtree/parse.rs.
//
// Port: a roxmltree::Error that this file makes is the sentinel error of its
// case, okage.ErrXMLNoRootNode or okage.ErrXMLNodesLimitReached, because the
// constructors of internal/roxmltree are private. An error of
// internal/roxmltree passes through as it is.

const (
	svgNS          = "http://www.w3.org/2000/svg"
	xlinkNS        = "http://www.w3.org/1999/xlink"
	xmlNamespaceNS = "http://www.w3.org/XML/1998/namespace"
)

// parseTree is Document::parse_tree in usvg src/parser/svgtree/parse.rs. It
// parses a [`Document`] from a [`roxmltree::Document`].
//
// Port: the empty string is an injected style sheet of None. opt is the
// options of the parse, for the logger (D20).
func parseTree(xml *roxmltree.Document, injectedStylesheet string, maxDepth uint32, opt *options) (*document, error) {
	return parse(xml, injectedStylesheet, maxDepth, opt)
}

// append is Document::append in usvg src/parser/svgtree/parse.rs.
func (d *document) append(parentID nodeID, kind nodeKind) nodeID {
	newChildID := nodeIDFromUsize(len(d.nodes))
	d.nodes = append(d.nodes, nodeData{
		parent:      someNodeID(parentID),
		nextSibling: 0,
		firstChild:  0,
		lastChild:   0,
		kind:        kind,
	})

	parent := &d.nodes[parentID.getUsize()]
	if lastChildID, ok := parent.lastChild.get(); ok {
		d.nodes[lastChildID.getUsize()].nextSibling = someNodeID(newChildID)
	}

	if parent.firstChild != 0 {
		parent.lastChild = someNodeID(newChildID)
	} else {
		parent.firstChild = someNodeID(newChildID)
		parent.lastChild = someNodeID(newChildID)
	}

	return newChildID
}

// appendAttribute is Document::append_attribute in usvg
// src/parser/svgtree/parse.rs.
func (d *document) appendAttribute(name AId, value string, important bool) {
	d.attrs = append(d.attrs, attribute{
		name:      name,
		value:     value,
		important: important,
	})
}

// parse is parse in usvg src/parser/svgtree/parse.rs.
func parse(xml *roxmltree.Document, injectedStylesheet string, maxDepth uint32, opt *options) (*document, error) {
	doc := &document{
		nodes: nil,
		attrs: nil,
		links: map[string]nodeID{},
		opt:   opt,
	}

	// build a map of id -> node for resolve_href
	idMap := map[string]roxmltree.Node{}
	for node := range xml.Descendants() {
		if id, ok := node.Attribute(roxmltree.ExpandedNameFromStr("id")); ok {
			if _, ok := idMap[id]; !ok {
				idMap[id] = node
			}
		}
	}

	// Add a root node.
	doc.nodes = append(doc.nodes, nodeData{kind: nodeKind{kind: nodeKindRoot}})

	styleTexts := styleTexts(xml)
	styleSheet := resolveCSS(styleTexts, injectedStylesheet, opt)

	err := parseXMLNodeChildren(
		xml.Root(),
		xml.Root(),
		doc.root().id,
		&styleSheet,
		false,
		0,
		maxDepth,
		doc,
		idMap,
	)
	if err != nil {
		return nil, err
	}

	// Check that the root element is `svg`.
	child, ok := doc.root().firstElementChild()
	if !ok || !child.hasTagName(EIdSvg) {
		return nil, okage.ErrXMLNoRootNode
	}

	links := map[string]nodeID{}
	for node := range doc.descendants() {
		if id, ok := node.attributeStr(AIdId); ok {
			links[id] = node.id
		}
	}
	doc.links = links

	doc.circularReferences = findCircularReferences(doc)

	return doc, nil
}

// parseTagName is parse_tag_name in usvg src/parser/svgtree/parse.rs.
func parseTagName(node roxmltree.Node) (EId, bool) {
	if !node.IsElement() {
		return 0, false
	}

	if ns, ok := node.TagName().Namespace(); ok && ns != svgNS {
		return 0, false
	}

	return eidFromStr(node.TagName().Name())
}

// parseXMLNodeChildren is parse_xml_node_children in usvg
// src/parser/svgtree/parse.rs.
func parseXMLNodeChildren(
	parent roxmltree.Node,
	origin roxmltree.Node,
	parentID nodeID,
	styleSheet *simplecss.StyleSheet,
	ignoreIDs bool,
	depth uint32,
	maxDepth uint32,
	doc *document,
	idMap map[string]roxmltree.Node,
) error {
	for node := range parent.Children() {
		err := parseXMLNode(
			node,
			origin,
			parentID,
			styleSheet,
			ignoreIDs,
			depth,
			maxDepth,
			doc,
			idMap,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// parseXMLNode is parse_xml_node in usvg src/parser/svgtree/parse.rs.
func parseXMLNode(
	node roxmltree.Node,
	origin roxmltree.Node,
	parentID nodeID,
	styleSheet *simplecss.StyleSheet,
	ignoreIDs bool,
	depth uint32,
	maxDepth uint32,
	doc *document,
	idMap map[string]roxmltree.Node,
) error {
	if depth > maxDepth {
		return okage.ErrXMLNodesLimitReached
	}

	tagName, ok := parseTagName(node)
	if !ok {
		return nil
	}

	if tagName == EIdStyle {
		return nil
	}

	// TODO: remove?
	// Treat links as groups.
	if tagName == EIdA {
		tagName = EIdG
	}

	nodeID, err := parseSvgElement(node, parentID, tagName, styleSheet, ignoreIDs, doc)
	if err != nil {
		return err
	}
	switch tagName {
	case EIdText:
		return parseSvgTextElement(node, nodeID, styleSheet, doc)
	case EIdUse:
		return parseSvgUseElement(
			node,
			origin,
			nodeID,
			styleSheet,
			depth+1,
			maxDepth,
			doc,
			idMap,
		)
	}
	return parseXMLNodeChildren(
		node,
		origin,
		nodeID,
		styleSheet,
		ignoreIDs,
		depth+1,
		maxDepth,
		doc,
		idMap,
	)
}

// parseSvgElement is parse_svg_element in usvg src/parser/svgtree/parse.rs.
func parseSvgElement(
	xmlNode roxmltree.Node,
	parentID nodeID,
	tagName EId,
	styleSheet *simplecss.StyleSheet,
	ignoreIDs bool,
	doc *document,
) (nodeID, error) {
	attrsStartIdx := len(doc.attrs)
	hrefIdx, hasHrefIdx := 0, false

	// Copy presentational attributes first.
	for attr := range xmlNode.Attributes() {
		ns, hasNS := attr.Namespace()
		if hasNS && ns != svgNS && ns != xlinkNS && ns != xmlNamespaceNS {
			continue
		}

		aid, ok := aidFromStr(attr.Name())
		if !ok {
			continue
		}

		// During a `use` resolving, all `id` attributes must be ignored.
		// Otherwise we will get elements with duplicated id's.
		if ignoreIDs && aid == AIdId {
			continue
		}

		// For some reason those properties are allowed only inside a `style` attribute and CSS.
		if aid == AIdMixBlendMode || aid == AIdIsolation || aid == AIdFontKerning {
			continue
		} else if aid == AIdImageRendering {
			switch attr.Value() {
			case "smooth", "high-quality", "crisp-edges", "pixelated": //nolint:goconst // an SVG keyword, as upstream writes it
				continue
			}
		}

		// SVG 2: when both `href` and `xlink:href` are present, the unprefixed
		// `href` takes precedence and the XLink one must be ignored, regardless
		// of their order in the source document.
		// https://www.w3.org/TR/SVG2/linking.html#XLinkRefAttrs
		if aid == AIdHref {
			isUnprefixed := !hasNS
			isXlink := hasNS && ns == xlinkNS
			if !isUnprefixed && !isXlink {
				continue
			}

			if hasHrefIdx {
				// An `href` was already stored. Only an unprefixed `href` is
				// allowed to override it; an `xlink:href` is ignored.
				if isUnprefixed {
					doc.attrs[hrefIdx].value = attr.ValueStorage()
				}
				continue
			}
		}

		added := appendAttribute(
			parentID,
			tagName,
			aid,
			attr.ValueStorage(),
			false,
			doc,
		)
		if added && aid == AIdHref {
			hrefIdx, hasHrefIdx = len(doc.attrs)-1, true
		}
	}

	insertAttribute := func(aid AId, value string, important bool) {
		// Check that attribute already exists.
		idx, hasIdx := 0, false
		for i, a := range doc.attrs[attrsStartIdx:] {
			if a.name == aid {
				idx, hasIdx = i, true
				break
			}
		}

		// Append an attribute as usual.
		added := appendAttribute(
			parentID,
			tagName,
			aid,
			value,
			important,
			doc,
		)

		// Check that attribute was actually added, because it could be skipped.
		if added && hasIdx {
			lastIdx := len(doc.attrs) - 1
			existingIdx := attrsStartIdx + idx

			// See https://developer.mozilla.org/en-US/docs/Web/CSS/important
			// When a declaration is important, the order of precedence is reversed.
			// Declarations marked as important in the user-agent style sheets override
			// all important declarations in the user style sheets. Similarly, all important
			// declarations in the user style sheets override all important declarations in the
			// author's style sheets. Finally, all important declarations take precedence over
			// all animations.
			//
			// Which means:
			// 1) Existing is not important, new is not important -> swap
			// 2) Existing is important, new is not important -> don't swap
			// 3) Existing is not important, new is important -> swap
			// 4) Existing is important, new is important -> don't swap (since the order
			// is reversed, so existing important attributes take precedence over new
			// important attributes)
			hasPrecedence := !doc.attrs[existingIdx].important

			if hasPrecedence {
				doc.attrs[existingIdx], doc.attrs[lastIdx] = doc.attrs[lastIdx], doc.attrs[existingIdx]
			}

			// Remove last.
			doc.attrs = doc.attrs[:len(doc.attrs)-1]
		}
	}

	// Apply CSS.
	for _, rule := range styleSheet.Rules {
		if rule.Selector.Matches(xmlNode2{xmlNode}) {
			for _, declaration := range rule.Declarations {
				writeDeclaration(declaration, insertAttribute, doc.opt)
			}
		}
	}

	// Split a `style` attribute.
	if value, ok := xmlNode.Attribute(roxmltree.ExpandedNameFromStr("style")); ok {
		writeDeclarations(value, insertAttribute, doc.opt)
	}

	if len(doc.nodes) > 1_000_000 {
		return 0, okage.ErrXMLNodesLimitReached
	}

	nodeID := doc.append(
		parentID,
		nodeKind{
			kind:       nodeKindElement,
			tagName:    tagName,
			attributes: newShortRange(uint32(attrsStartIdx), uint32(len(doc.attrs))),
		},
	)

	return nodeID, nil
}

// writeDeclarations is write_declarations in usvg
// src/parser/svgtree/parse.rs.
//
// Port: upstream passes a function that borrows or copies each value, so
// that a value borrows the input where it can. A Go string always shares
// its bytes, so that parameter has no Go form. opt holds the logger (D20).
func writeDeclarations(text string, insert func(AId, string, bool), opt *options) {
	for declaration := range simplecss.NewDeclarationTokenizer(text).All() {
		writeDeclaration(declaration, insert, opt)
	}
}

// writeDeclaration is write_declaration in usvg
// src/parser/svgtree/parse.rs.
//
// Port: as in writeDeclarations, the function that borrows or copies a
// value has no Go form. opt holds the logger (D20).
func writeDeclaration(declaration simplecss.Declaration, insert func(AId, string, bool), opt *options) {
	// TODO: perform XML attribute normalization
	imp := declaration.Important
	val := declaration.Value

	if declaration.Name == "marker" {
		insert(AIdMarkerStart, val, imp)
		insert(AIdMarkerMid, val, imp)
		insert(AIdMarkerEnd, val, imp)
	} else if declaration.Name == "font" {
		if shorthand, err := svgtypes.ParseFontShorthand(val); err == nil {
			// First we need to reset all values to their default.
			insert(AIdFontStyle, "normal", imp)
			insert(AIdFontVariant, "normal", imp)
			insert(AIdFontWeight, "normal", imp)
			insert(AIdFontStretch, "normal", imp)
			insert(AIdLineHeight, "normal", imp)
			insert(AIdFontSizeAdjust, "none", imp)
			insert(AIdFontKerning, "auto", imp)
			insert(AIdFontVariantCaps, "normal", imp)
			insert(AIdFontVariantLigatures, "normal", imp)
			insert(AIdFontVariantNumeric, "normal", imp)
			insert(AIdFontVariantEastAsian, "normal", imp)
			insert(AIdFontVariantPosition, "normal", imp)

			// Then, we set the properties that have been declared.
			if shorthand.FontStretch != "" {
				insert(AIdFontStretch, shorthand.FontStretch, imp)
			}
			if shorthand.FontWeight != "" {
				insert(AIdFontWeight, shorthand.FontWeight, imp)
			}
			if shorthand.FontVariant != "" {
				insert(AIdFontVariant, shorthand.FontVariant, imp)
			}
			if shorthand.FontStyle != "" {
				insert(AIdFontStyle, shorthand.FontStyle, imp)
			}
			insert(AIdFontSize, shorthand.FontSize, imp)
			insert(AIdFontFamily, shorthand.FontFamily, imp)
		} else {
			opt.warnf(
				"Failed to parse %s value: '%s'",
				AIdFont,
				declaration.Value,
			)
		}
	} else if aid, ok := aidFromStr(declaration.Name); ok {
		// Parse only the presentation attributes.
		if aid.isPresentation() {
			insert(aid, val, imp)
		}
	}
}

// appendAttribute is append_attribute in usvg src/parser/svgtree/parse.rs.
//
// Port: the Go name has no suffix, because the method
// document.appendAttribute does not collide with a function.
func appendAttribute(
	parentID nodeID,
	tagName EId,
	aid AId,
	value string,
	important bool,
	doc *document,
) bool {
	switch aid {
	// The `style` attribute will be split into attributes, so we don't need it.
	case AIdStyle,
		// No need to copy a `class` attribute since CSS were already resolved.
		AIdClass:
		return false
	}

	// Ignore `xlink:href` on `tspan` (which was originally `tref` or `a`),
	// because we will convert `tref` into `tspan` anyway.
	if tagName == EIdTspan && aid == AIdHref {
		return false
	}

	if aid.allowsInheritValue() && value == "inherit" {
		return resolveInherit(parentID, aid, doc)
	}

	doc.appendAttribute(aid, value, important)
	return true
}

// resolveInherit is resolve_inherit in usvg src/parser/svgtree/parse.rs.
func resolveInherit(parentID nodeID, aid AId, doc *document) bool {
	if aid.isInheritable() {
		// Inheritable attributes can inherit a value from an any ancestor.
		for n := range doc.get(parentID).ancestors() {
			if !n.hasAttribute(aid) {
				continue
			}
			for _, attr := range n.attributes() {
				if attr.name == aid {
					doc.attrs = append(doc.attrs, attribute{
						name:      aid,
						value:     attr.value,
						important: attr.important,
					})

					return true
				}
			}
			break
		}
	} else {
		// Non-inheritable attributes can inherit a value only from a direct parent.
		for _, attr := range doc.get(parentID).attributes() {
			if attr.name == aid {
				doc.attrs = append(doc.attrs, attribute{
					name:      aid,
					value:     attr.value,
					important: attr.important,
				})

				return true
			}
		}
	}

	// Fallback to a default value if possible.
	var value string
	switch aid {
	case AIdImageRendering, AIdShapeRendering, AIdTextRendering:
		value = "auto" //nolint:goconst // an SVG keyword, as upstream writes it

	case AIdClipPath,
		AIdFilter,
		AIdMarkerEnd,
		AIdMarkerMid,
		AIdMarkerStart,
		AIdMask,
		AIdStroke,
		AIdStrokeDasharray,
		AIdTextDecoration:
		value = "none" //nolint:goconst // an SVG keyword, as upstream writes it

	case AIdFontStretch,
		AIdFontStyle,
		AIdFontVariant,
		AIdFontWeight,
		AIdLetterSpacing,
		AIdWordSpacing:
		value = "normal" //nolint:goconst // an SVG keyword, as upstream writes it

	case AIdFill, AIdFloodColor, AIdStopColor:
		value = "black"

	case AIdFillOpacity,
		AIdFloodOpacity,
		AIdOpacity,
		AIdStopOpacity,
		AIdStrokeOpacity:
		value = "1"

	case AIdClipRule, AIdFillRule:
		value = "nonzero"

	case AIdBaselineShift:
		value = "baseline"
	case AIdColorInterpolationFilters:
		value = "linearRGB" //nolint:goconst // an SVG keyword, as upstream writes it
	case AIdDirection:
		value = "ltr"
	case AIdDisplay:
		value = "inline"
	case AIdFontSize:
		value = "medium"
	case AIdOverflow:
		value = "visible" //nolint:goconst // an SVG keyword, as upstream writes it
	case AIdStrokeDashoffset:
		value = "0"
	case AIdStrokeLinecap:
		value = "butt"
	case AIdStrokeLinejoin:
		value = "miter"
	case AIdStrokeMiterlimit:
		value = "4"
	case AIdStrokeWidth:
		value = "1"
	case AIdTextAnchor:
		value = "start"
	case AIdVisibility:
		value = "visible"
	case AIdWritingMode:
		value = "lr-tb"
	default:
		return false
	}

	doc.appendAttribute(aid, value, false)
	return true
}

// resolveHref is resolve_href in usvg src/parser/svgtree/parse.rs.
func resolveHref(node roxmltree.Node, idMap map[string]roxmltree.Node) (roxmltree.Node, bool) {
	// See the comment in `parse_svg_element` about `href` precedence.
	//
	// Note: `roxmltree::Node::attribute("href")` matches by local name only and
	// would return whichever `href`/`xlink:href` comes first, so we have to
	// filter by namespace explicitly.
	linkValue, ok := "", false
	for a := range node.Attributes() {
		if _, hasNS := a.Namespace(); a.Name() == "href" && !hasNS { //nolint:goconst // an SVG keyword, as upstream writes it
			linkValue, ok = a.Value(), true
			break
		}
	}
	if !ok {
		for a := range node.Attributes() {
			if ns, hasNS := a.Namespace(); a.Name() == "href" && hasNS && ns == xlinkNS {
				linkValue, ok = a.Value(), true
				break
			}
		}
	}
	if !ok {
		return roxmltree.Node{}, false
	}

	linkID, err := svgtypes.ParseIRI(linkValue)
	if err != nil {
		return roxmltree.Node{}, false
	}

	n, ok := idMap[string(linkID)]
	return n, ok
}

// parseSvgUseElement is parse_svg_use_element in usvg
// src/parser/svgtree/parse.rs.
func parseSvgUseElement(
	node roxmltree.Node,
	origin roxmltree.Node,
	parentID nodeID,
	styleSheet *simplecss.StyleSheet,
	depth uint32,
	maxDepth uint32,
	doc *document,
	idMap map[string]roxmltree.Node,
) error {
	link, ok := resolveHref(node, idMap)
	if !ok {
		return nil
	}

	if link == node || link == origin {
		id, _ := node.Attribute(roxmltree.ExpandedNameFromTuple(svgNS, "id"))
		doc.opt.warnf("Recursive 'use' detected. '%s' will be skipped.", id)
		return nil
	}

	// Make sure we're linked to an SVG element.
	if _, ok := parseTagName(link); !ok {
		return nil
	}

	// Check that none of the linked node's children reference current `use` node
	// via other `use` node.
	//
	// Example:
	// <g id="g1">
	//     <use xlink:href="#use1" id="use2"/>
	// </g>
	// <use xlink:href="#g1" id="use1"/>
	//
	// `use2` should be removed.
	//
	// Also, child should not reference its parent:
	// <g id="g1">
	//     <use xlink:href="#g1" id="use1"/>
	// </g>
	//
	// `use1` should be removed.
	isRecursive := false
	first := true
	for linkChild := range link.Descendants() {
		if first {
			first = false
			continue
		}
		if !linkChild.HasTagName(roxmltree.ExpandedNameFromTuple(svgNS, "use")) {
			continue
		}
		if link2, ok := resolveHref(linkChild, idMap); ok {
			if link2 == node || link2 == link {
				isRecursive = true
				break
			}
		}
	}

	if isRecursive {
		id, _ := node.Attribute(roxmltree.ExpandedNameFromTuple(svgNS, "id"))
		doc.opt.warnf("Recursive 'use' detected. '%s' will be skipped.", id)
		return nil
	}

	return parseXMLNode(
		link,
		node,
		parentID,
		styleSheet,
		true,
		depth+1,
		maxDepth,
		doc,
		idMap,
	)
}

// styleTexts is style_texts in usvg src/parser/svgtree/parse.rs. It returns
// the child text content of each CSS `style` element.
func styleTexts(xml *roxmltree.Document) []string {
	var texts []string
	for n := range xml.Descendants() {
		if !n.HasTagName(roxmltree.ExpandedNameFromStr("style")) {
			continue
		}
		if t, ok := n.Attribute(roxmltree.ExpandedNameFromStr("type")); ok && t != "" && !eqIgnoreASCIICase(t, "text/css") {
			continue
		}
		var b strings.Builder
		for c := range n.Children() {
			if !c.IsText() {
				continue
			}
			if text, ok := c.Text(); ok {
				b.WriteString(text)
			}
		}
		texts = append(texts, b.String())
	}
	return texts
}

// eqIgnoreASCIICase is str::eq_ignore_ascii_case of Rust. Unlike
// strings.EqualFold, it folds only ASCII letters.
func eqIgnoreASCIICase(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// resolveCSS is resolve_css in usvg src/parser/svgtree/parse.rs.
//
// Port: the empty string is a style sheet of None. opt gives the logger of
// simplecss (D20).
func resolveCSS(styleTexts []string, styleSheet string, opt *options) simplecss.StyleSheet {
	sheet := simplecss.NewStyleSheet()
	var cssOpts []simplecss.Option
	if opt != nil && opt.logger != nil {
		cssOpts = append(cssOpts, simplecss.WithLogger(opt.logger))
	}

	// Injected style sheets do not override internal ones (we mimic the logic of rsvg-convert),
	// so we need to parse it first.
	if styleSheet != "" {
		sheet.ParseMore(styleSheet, cssOpts...)
	}

	for _, text := range styleTexts {
		sheet.ParseMore(text, cssOpts...)
	}

	return sheet
}

// xmlNode2 is XmlNode in usvg src/parser/svgtree/parse.rs. It implements
// simplecss.Element for a node of roxmltree.
//
// Port: the Go name takes a suffix, because parseSvgElement names its
// parameter xmlNode after the Rust code.
type xmlNode2 struct {
	n roxmltree.Node
}

// ParentElement is simplecss::Element::parent_element for XmlNode in usvg
// src/parser/svgtree/parse.rs.
func (x xmlNode2) ParentElement() (simplecss.Element, bool) {
	p, ok := x.n.ParentElement()
	if !ok {
		return nil, false
	}
	return xmlNode2{p}, true
}

// PrevSiblingElement is simplecss::Element::prev_sibling_element for XmlNode
// in usvg src/parser/svgtree/parse.rs.
func (x xmlNode2) PrevSiblingElement() (simplecss.Element, bool) {
	p, ok := x.n.PrevSiblingElement()
	if !ok {
		return nil, false
	}
	return xmlNode2{p}, true
}

// HasLocalName is simplecss::Element::has_local_name for XmlNode in usvg
// src/parser/svgtree/parse.rs.
func (x xmlNode2) HasLocalName(localName string) bool {
	return x.n.TagName().Name() == localName
}

// AttributeMatches is simplecss::Element::attribute_matches for XmlNode in
// usvg src/parser/svgtree/parse.rs.
func (x xmlNode2) AttributeMatches(localName string, operator simplecss.AttributeOperator) bool {
	value, ok := x.n.Attribute(roxmltree.ExpandedNameFromStr(localName))
	if !ok {
		return false
	}
	return operator.Matches(value)
}

// PseudoClassMatches is simplecss::Element::pseudo_class_matches for XmlNode
// in usvg src/parser/svgtree/parse.rs.
func (x xmlNode2) PseudoClassMatches(class simplecss.PseudoClass) bool {
	if class.Kind == simplecss.PseudoClassFirstChild {
		_, ok := x.PrevSiblingElement()
		return !ok
	}
	// TODO: lang
	return false // Since we are querying a static SVG we can ignore other pseudo-classes.
}

// findCircularReferences is find_circular_references in usvg
// src/parser/svgtree/parse.rs.
func findCircularReferences(doc *document) map[circularReference]struct{} {
	search := newCycleSearch()
	for node := range doc.descendants() {
		if isResource(node) {
			search.visit(node)
		}
	}

	return search.circularReferences
}

// visitState is Visit in usvg src/parser/svgtree/parse.rs.
type visitState uint8

// The values of visitState.
const (
	visitInProgress visitState = iota
	visitDone
)

// cycleSearch is CycleSearch in usvg src/parser/svgtree/parse.rs.
type cycleSearch struct {
	visits             map[nodeID]visitState
	templates          map[nodeID]nodeID
	chain              []nodeID
	circularReferences map[circularReference]struct{}
}

// newCycleSearch is the derived Default for CycleSearch in usvg
// src/parser/svgtree/parse.rs.
func newCycleSearch() *cycleSearch {
	return &cycleSearch{
		visits:             map[nodeID]visitState{},
		templates:          map[nodeID]nodeID{},
		circularReferences: map[circularReference]struct{}{},
	}
}

// referenceFrame is an entry of the stack of CycleSearch::visit: a node and
// the references of it that are left.
type referenceFrame struct {
	id   nodeID
	refs []reference
}

// visit is CycleSearch::visit in usvg src/parser/svgtree/parse.rs.
func (c *cycleSearch) visit(node svgNode) {
	node = c.withContent(node)
	if _, ok := c.visits[node.id]; ok {
		return
	}

	c.visits[node.id] = visitInProgress
	stack := []referenceFrame{{node.id, references(node)}}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if len(top.refs) == 0 {
			c.visits[top.id] = visitDone
			stack = stack[:len(stack)-1]
			continue
		}
		ref := top.refs[0]
		top.refs = top.refs[1:]

		content := c.withContent(ref.target)
		switch v, ok := c.visits[content.id]; {
		case ok && v == visitInProgress:
			node.doc.opt.warnf(
				"Circular reference to '%s' via '%s' ignored.",
				ref.target.elementID(),
				ref.aid,
			)
			c.circularReferences[circularReference{ref.user.id, ref.aid}] = struct{}{}
		case ok:
		default:
			c.visits[content.id] = visitInProgress
			stack = append(stack, referenceFrame{content.id, references(content)})
		}
	}
}

// withContent is CycleSearch::with_content in usvg
// src/parser/svgtree/parse.rs.
//
// Returns the first element in the `xlink:href` chain of a pattern or filter that has
// children, since its children are used instead. Elements sharing a template are resolved to
// the same element, so its content is only walked once, and results are cached along the
// chain.
func (c *cycleSearch) withContent(node svgNode) svgNode {
	if tag, ok := node.tagName(); !ok || (tag != EIdPattern && tag != EIdFilter) {
		return node
	}

	current := node
	var content svgNode
	for {
		if id, ok := c.templates[current.id]; ok {
			content = node.document().get(id)
			break
		}

		if current.hasChildren() {
			content = current
			break
		}

		// Resolves to `node` until the chain is done, which also ends an `xlink:href` cycle.
		c.templates[current.id] = node.id
		c.chain = append(c.chain, current.id)
		link, ok := current.nodeAttribute(AIdHref)
		if !ok || !sameTagName(link, node) {
			content = node
			break
		}
		current = link
	}

	for _, id := range c.chain {
		c.templates[id] = content.id
	}
	c.chain = c.chain[:0]

	return content
}

// sameTagName reports whether a and b have the same tag name, as the
// comparison of two Option<EId> of upstream does.
func sameTagName(a, b svgNode) bool {
	ta, oka := a.tagName()
	tb, okb := b.tagName()
	return oka == okb && ta == tb
}

// reference is Reference in usvg src/parser/svgtree/parse.rs: the element
// that uses the reference, the attribute, and the referenced element.
type reference struct {
	user   svgNode
	aid    AId
	target svgNode
}

// references is references in usvg src/parser/svgtree/parse.rs.
//
// Returns the references that are followed when rendering `node`,
// as `(element that uses the reference, attribute, referenced element)`.
func references(node svgNode) []reference {
	var refs []reference
	tag, _ := node.tagName()
	switch tag {
	// Masks, filters, paint servers and markers are ignored inside clip paths.
	case EIdClipPath:
		pushLink(node, AIdClipPath, EIdClipPath, &refs)
		forEachContentDescendant(node, func(child svgNode) {
			pushLink(child, AIdClipPath, EIdClipPath, &refs)
		})
	case EIdMask:
		pushLink(node, AIdMask, EIdMask, &refs)
		forEachContentDescendant(node, func(child svgNode) { pushReferences(child, &refs) })
	case EIdPattern:
		forEachContentDescendant(node, func(child svgNode) { pushReferences(child, &refs) })
	case EIdFilter:
		for child := range node.descendants() {
			if t, ok := child.tagName(); ok && t == EIdFeImage {
				if link, ok := child.nodeAttribute(AIdHref); ok {
					if lt, ok := link.tagName(); ok && (lt.IsGraphic() || lt == EIdG || lt == EIdSwitch || lt == EIdSvg) {
						refs = append(refs, reference{child, AIdHref, link})
					}
				}
			}
		}
	case EIdMarker:
		forEachContentDescendant(node, func(child svgNode) { pushReferences(child, &refs) })
	// An element referenced by `feImage`.
	default:
		pushReferences(node, &refs)
		forEachContentDescendant(node, func(child svgNode) { pushReferences(child, &refs) })
	}

	return refs
}

// forEachContentDescendant is for_each_content_descendant in usvg
// src/parser/svgtree/parse.rs.
//
// Calls `f` for each descendant of `node`, skipping nested resources and other definitions.
// Those are only rendered where they are referenced, and resources are visited on their own.
func forEachContentDescendant(node svgNode, f func(svgNode)) {
	nodeTag, nodeOK := node.tagName()
	for child := range node.children() {
		// A `use` stores a copy of the `symbol` it references as its child.
		tag, ok := child.tagName()
		isDefinition := isResource(child) ||
			(ok && tag == EIdDefs) ||
			(ok && tag == EIdSymbol && (!nodeOK || nodeTag != EIdUse))
		if !isDefinition {
			f(child)
			forEachContentDescendant(child, f)
		}
	}
}

// isResource is is_resource in usvg src/parser/svgtree/parse.rs.
func isResource(node svgNode) bool {
	tag, ok := node.tagName()
	if !ok {
		return false
	}
	switch tag {
	case EIdClipPath, EIdMask, EIdPattern, EIdFilter, EIdMarker:
		return true
	}
	return false
}

// pushReferences is push_references in usvg src/parser/svgtree/parse.rs.
func pushReferences(node svgNode, refs *[]reference) {
	if !node.isElement() {
		return
	}

	pushLink(node, AIdClipPath, EIdClipPath, refs)
	pushLink(node, AIdMask, EIdMask, refs)

	if value, ok := node.attributeStr(AIdFilter); ok {
		for filter, err := range svgtypes.NewFilterValueListParser(value).All() {
			if err != nil || filter.Kind != svgtypes.FilterValueURL {
				continue
			}
			if link, ok := node.document().elementByID(filter.URL); ok {
				if t, ok := link.tagName(); ok && t == EIdFilter {
					*refs = append(*refs, reference{node, AIdFilter, link})
				}
			}
		}
	}

	// Paint and markers are inherited, so they have to be resolved like during conversion.
	for _, aid := range []AId{AIdFill, AIdStroke} {
		if value, ok := nodeFindAttribute(node, aid, fromValueStr); ok {
			if paint, err := svgtypes.ParsePaint(value); err == nil && paint.Kind == svgtypes.PaintFuncIRI {
				if link, ok := node.document().elementByID(paint.FuncIRI); ok {
					if t, ok := link.tagName(); ok && t == EIdPattern {
						*refs = append(*refs, reference{node, aid, link})
					}
				}
			}
		}
	}

	tag, _ := node.tagName()
	isShape := false
	switch tag {
	case EIdRect, EIdCircle, EIdEllipse, EIdLine, EIdPolyline, EIdPolygon, EIdPath:
		isShape = true
	}
	// Markers are never drawn inside a clip path.
	if isShape && !insideClipPath(node) {
		for _, aid := range []AId{AIdMarkerStart, AIdMarkerMid, AIdMarkerEnd} {
			if link, ok := nodeFindAttribute(node, aid, fromValueSvgNode); ok {
				if t, ok := link.tagName(); ok && t == EIdMarker {
					*refs = append(*refs, reference{node, aid, link})
				}
			}
		}
	}
}

// insideClipPath reports whether node or one of its ancestors is a
// clipPath element, as `node.ancestors().any(...)` of push_references does.
func insideClipPath(node svgNode) bool {
	for n := range node.ancestors() {
		if t, ok := n.tagName(); ok && t == EIdClipPath {
			return true
		}
	}
	return false
}

// pushLink is push_link in usvg src/parser/svgtree/parse.rs.
func pushLink(node svgNode, aid AId, tagName EId, refs *[]reference) {
	if link, ok := node.nodeAttribute(aid); ok {
		if t, ok := link.tagName(); ok && t == tagName {
			*refs = append(*refs, reference{node, aid, link})
		}
	}
}
