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
func parseTree(xml *roxmltree.Document, injectedStylesheet string, opt *options) (*document, error) {
	return parse(xml, injectedStylesheet, opt)
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
func parse(xml *roxmltree.Document, injectedStylesheet string, opt *options) (*document, error) {
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

	// Collect all elements with `id` attribute, and all elements that
	// can be referenced recursively.
	links := map[string]nodeID{}
	var patterns, clipPaths, masks, filters, feImages []nodeID
	for node := range doc.descendants() {
		if id, ok := node.attributeStr(AIdId); ok {
			links[id] = node.id
		}

		if tag, ok := node.tagName(); ok {
			switch tag {
			case EIdPattern:
				patterns = append(patterns, node.id)
			case EIdClipPath:
				clipPaths = append(clipPaths, node.id)
			case EIdMask:
				masks = append(masks, node.id)
			case EIdFilter:
				filters = append(filters, node.id)
			case EIdFeImage:
				feImages = append(feImages, node.id)
			}
		}
	}
	doc.links = links

	fixRecursivePatterns(patterns, doc)
	fixRecursiveLinks(clipPaths, AIdClipPath, doc)
	fixRecursiveLinks(masks, AIdMask, doc)
	fixRecursiveLinks(filters, AIdFilter, doc)
	fixRecursiveFeImage(feImages, doc)

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
	doc *document,
	idMap map[string]roxmltree.Node,
) error {
	if depth > 1024 {
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
		return parseSvgUseElement(node, origin, nodeID, styleSheet, depth+1, doc, idMap)
	}
	return parseXMLNodeChildren(
		node,
		origin,
		nodeID,
		styleSheet,
		ignoreIDs,
		depth+1,
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

	writeDeclaration := func(declaration simplecss.Declaration) {
		// TODO: perform XML attribute normalization
		imp := declaration.Important
		val := declaration.Value

		if declaration.Name == "marker" {
			insertAttribute(AIdMarkerStart, val, imp)
			insertAttribute(AIdMarkerMid, val, imp)
			insertAttribute(AIdMarkerEnd, val, imp)
		} else if declaration.Name == "font" {
			if shorthand, err := svgtypes.ParseFontShorthand(val); err == nil {
				// First we need to reset all values to their default.
				insertAttribute(AIdFontStyle, "normal", imp)
				insertAttribute(AIdFontVariant, "normal", imp)
				insertAttribute(AIdFontWeight, "normal", imp)
				insertAttribute(AIdFontStretch, "normal", imp)
				insertAttribute(AIdLineHeight, "normal", imp)
				insertAttribute(AIdFontSizeAdjust, "none", imp)
				insertAttribute(AIdFontKerning, "auto", imp)
				insertAttribute(AIdFontVariantCaps, "normal", imp)
				insertAttribute(AIdFontVariantLigatures, "normal", imp)
				insertAttribute(AIdFontVariantNumeric, "normal", imp)
				insertAttribute(AIdFontVariantEastAsian, "normal", imp)
				insertAttribute(AIdFontVariantPosition, "normal", imp)

				// Then, we set the properties that have been declared.
				if shorthand.FontStretch != "" {
					insertAttribute(AIdFontStretch, shorthand.FontStretch, imp)
				}
				if shorthand.FontWeight != "" {
					insertAttribute(AIdFontWeight, shorthand.FontWeight, imp)
				}
				if shorthand.FontVariant != "" {
					insertAttribute(AIdFontVariant, shorthand.FontVariant, imp)
				}
				if shorthand.FontStyle != "" {
					insertAttribute(AIdFontStyle, shorthand.FontStyle, imp)
				}
				insertAttribute(AIdFontSize, shorthand.FontSize, imp)
				insertAttribute(AIdFontFamily, shorthand.FontFamily, imp)
			} else {
				doc.opt.warnf(
					"Failed to parse %s value: '%s'",
					AIdFont,
					declaration.Value,
				)
			}
		} else if aid, ok := aidFromStr(declaration.Name); ok {
			// Parse only the presentation attributes.
			if aid.isPresentation() {
				insertAttribute(aid, val, imp)
			}
		}
	}

	// Apply CSS.
	for _, rule := range styleSheet.Rules {
		if rule.Selector.Matches(xmlNode2{xmlNode}) {
			for _, declaration := range rule.Declarations {
				writeDeclaration(declaration)
			}
		}
	}

	// Split a `style` attribute.
	if value, ok := xmlNode.Attribute(roxmltree.ExpandedNameFromStr("style")); ok {
		for declaration := range simplecss.NewDeclarationTokenizer(value).All() {
			writeDeclaration(declaration)
		}
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

// fixRecursivePatterns is fix_recursive_patterns in usvg
// src/parser/svgtree/parse.rs.
func fixRecursivePatterns(patterns []nodeID, doc *document) {
	for {
		nodeID, ok := findRecursivePattern(AIdFill, patterns, doc)
		if !ok {
			break
		}
		idx := mustAttributeID(doc.get(nodeID).attributeID(AIdFill))
		doc.attrs[idx].value = "none"
	}

	for {
		nodeID, ok := findRecursivePattern(AIdStroke, patterns, doc)
		if !ok {
			break
		}
		idx := mustAttributeID(doc.get(nodeID).attributeID(AIdStroke))
		doc.attrs[idx].value = "none"
	}
}

// findRecursivePattern is find_recursive_pattern in usvg
// src/parser/svgtree/parse.rs.
func findRecursivePattern(aid AId, patterns []nodeID, doc *document) (nodeID, bool) {
	for _, id := range patterns {
		patternNode := doc.get(id)
		for node := range patternNode.descendants() {
			value, ok := node.attributeStr(aid)
			if !ok {
				continue
			}

			paint, err := svgtypes.ParsePaint(value)
			if err != nil || paint.Kind != svgtypes.PaintFuncIRI {
				continue
			}
			linkID := paint.FuncIRI
			if linkID == patternNode.elementID() {
				// If a pattern child has a link to the pattern itself
				// then we have to replace it with `none`.
				// Otherwise we will get endless loop/recursion and stack overflow.
				return node.id, true
			}
			// Check that linked node children doesn't link this pattern.
			linkedNode, ok := doc.elementByID(linkID)
			if !ok {
				continue
			}
			for node2 := range linkedNode.descendants() {
				value2, ok := node2.attributeStr(aid)
				if !ok {
					continue
				}

				paint2, err := svgtypes.ParsePaint(value2)
				if err == nil && paint2.Kind == svgtypes.PaintFuncIRI {
					if paint2.FuncIRI == patternNode.elementID() {
						return node2.id, true
					}
				}
			}
		}
	}

	return 0, false
}

// fixRecursiveLinks is fix_recursive_links in usvg
// src/parser/svgtree/parse.rs.
func fixRecursiveLinks(nodes []nodeID, aid AId, doc *document) {
	for {
		nodeID, ok := findRecursiveLink(nodes, aid, doc)
		if !ok {
			break
		}
		idx := mustAttributeID(doc.get(nodeID).attributeID(aid))
		doc.attrs[idx].value = "none"
	}
}

// findRecursiveLink is find_recursive_link in usvg
// src/parser/svgtree/parse.rs.
func findRecursiveLink(nodes []nodeID, aid AId, doc *document) (nodeID, bool) {
	for _, id := range nodes {
		node := doc.get(id)
		for child := range node.descendants() {
			link, ok := child.nodeAttribute(aid)
			if !ok {
				continue
			}
			if link == node {
				// If an element child has a link to the element itself
				// then we have to replace it with `none`.
				// Otherwise we will get endless loop/recursion and stack overflow.
				return child.id, true
			}
			// Check that linked node children doesn't link this element.
			for node2 := range link.descendants() {
				if link2, ok := node2.nodeAttribute(aid); ok && link2 == node {
					return node2.id, true
				}
			}
		}
	}

	return 0, false
}

// fixRecursiveFeImage is fix_recursive_fe_image in usvg
// src/parser/svgtree/parse.rs. It detects cases like:
//
//	<filter id="filter1">
//	  <feImage xlink:href="#rect1"/>
//	</filter>
//	<rect id="rect1" x="36" y="36" width="120" height="120" fill="green" filter="url(#filter1)"/>
func fixRecursiveFeImage(feImages []nodeID, doc *document) {
	var ids []nodeID
	for _, id := range feImages {
		feNode := doc.get(id)
		link, ok := feNode.nodeAttribute(AIdHref)
		if !ok {
			continue
		}
		filterURI, ok := link.attributeStr(AIdFilter)
		if !ok {
			continue
		}
		p, ok := feNode.parent()
		if !ok {
			panic("svg: called Option::unwrap() on a None value")
		}
		filterID := p.elementID()
		for fn, err := range svgtypes.NewFilterValueListParser(filterURI).All() {
			if err != nil {
				continue
			}
			if fn.Kind == svgtypes.FilterValueURL && fn.URL == filterID {
				ids = append(ids, link.id)
			}
		}
	}

	for _, id := range ids {
		idx := mustAttributeID(doc.get(id).attributeID(AIdFilter))
		doc.attrs[idx].value = "none"
	}
}

// mustAttributeID is the unwrap of an Option<usize> that upstream relies on.
// It panics when ok is false.
func mustAttributeID(idx int, ok bool) int {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return idx
}
