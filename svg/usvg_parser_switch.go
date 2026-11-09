// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"slices"
	"strings"
)

// This file ports usvg src/parser/switch.rs.

// features is FEATURES in usvg src/parser/switch.rs.
//
// Full list can be found here: https://www.w3.org/TR/SVG11/feature.html
var features = []string{
	"http://www.w3.org/TR/SVG11/feature#SVGDOM-static",
	"http://www.w3.org/TR/SVG11/feature#SVG-static",
	"http://www.w3.org/TR/SVG11/feature#CoreAttribute", // no xml:base and xml:lang
	"http://www.w3.org/TR/SVG11/feature#Structure",
	"http://www.w3.org/TR/SVG11/feature#BasicStructure",
	"http://www.w3.org/TR/SVG11/feature#ContainerAttribute", // `enable-background`
	"http://www.w3.org/TR/SVG11/feature#ConditionalProcessing",
	"http://www.w3.org/TR/SVG11/feature#Image",
	"http://www.w3.org/TR/SVG11/feature#Style",
	// "http://www.w3.org/TR/SVG11/feature#ViewportAttribute", // `clip` and `overflow`, not yet
	"http://www.w3.org/TR/SVG11/feature#Shape",
	"http://www.w3.org/TR/SVG11/feature#Text",
	"http://www.w3.org/TR/SVG11/feature#BasicText",
	"http://www.w3.org/TR/SVG11/feature#PaintAttribute",      // no color-interpolation and color-rendering
	"http://www.w3.org/TR/SVG11/feature#BasicPaintAttribute", // no color-interpolation
	"http://www.w3.org/TR/SVG11/feature#OpacityAttribute",
	"http://www.w3.org/TR/SVG11/feature#GraphicsAttribute",
	"http://www.w3.org/TR/SVG11/feature#BasicGraphicsAttribute",
	"http://www.w3.org/TR/SVG11/feature#Marker",
	// "http://www.w3.org/TR/SVG11/feature#ColorProfile", // not yet
	"http://www.w3.org/TR/SVG11/feature#Gradient",
	"http://www.w3.org/TR/SVG11/feature#Pattern",
	"http://www.w3.org/TR/SVG11/feature#Clip",
	"http://www.w3.org/TR/SVG11/feature#BasicClip",
	"http://www.w3.org/TR/SVG11/feature#Mask",
	"http://www.w3.org/TR/SVG11/feature#Filter",
	"http://www.w3.org/TR/SVG11/feature#BasicFilter",
	// only xlink:href
	"http://www.w3.org/TR/SVG11/feature#XlinkAttribute",
	// "http://www.w3.org/TR/SVG11/feature#Font",
	// "http://www.w3.org/TR/SVG11/feature#BasicFont",
}

// convertSwitch is convert in usvg src/parser/switch.rs.
func convertSwitch(
	node svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) bool {
	var child svgNode
	found := false
	for n := range node.children() {
		if isConditionPassed(n, state.opt) {
			child, found = n, true
			break
		}
	}
	if !found {
		return false
	}
	if g, ok := convertGroup(node, state, false, cache, parent, func(cache *converterCache, g *Group) {
		convertElement(child, state, cache, g)
	}); ok {
		parent.children = append(parent.children, g)
	}

	return true
}

// isConditionPassed is is_condition_passed in usvg src/parser/switch.rs.
func isConditionPassed(node svgNode, opt *options) bool {
	if !node.isElement() {
		return false
	}

	if node.hasAttribute(AIdRequiredExtensions) {
		return false
	}

	// 'The value is a list of feature strings, with the individual values separated by white space.
	// Determines whether all of the named features are supported by the user agent.
	// Only feature strings defined in the Feature String appendix are allowed.
	// If all of the given features are supported, then the attribute evaluates to true;
	// otherwise, the current element and its children are skipped and thus will not be rendered.'
	if feats, ok := node.attributeStr(AIdRequiredFeatures); ok {
		for feature := range strings.SplitSeq(feats, " ") {
			if !slices.Contains(features, feature) {
				return false
			}
		}
	}

	if !isValidSysLang(node, opt) {
		return false
	}

	return true
}

// isValidSysLang is is_valid_sys_lang in usvg src/parser/switch.rs. SVG spec
// 5.8.5.
func isValidSysLang(node svgNode, opt *options) bool {
	// 'The attribute value is a comma-separated list of language names
	// as defined in BCP 47.'
	//
	// But we support only simple cases like `en` or `en-US`.
	// No one really uses this, especially with complex BCP 47 values.
	langs, ok := node.attributeStr(AIdSystemLanguage)
	if !ok {
		return true
	}
	hasMatch := false
	for lang := range strings.SplitSeq(langs, ",") {
		lang = strings.TrimFunc(lang, isRustWhitespace)

		// 'Evaluates to `true` if one of the languages indicated by user preferences exactly
		// equals one of the languages given in the value of this parameter.'
		if slices.Contains(opt.languages, lang) {
			hasMatch = true
			break
		}

		// 'If one of the languages indicated by user preferences exactly equals a prefix
		// of one of the languages given in the value of this parameter such that
		// the first tag character following the prefix is `-`.'
		if idx := strings.IndexByte(lang, '-'); idx >= 0 {
			langPrefix := lang[:idx]
			if slices.Contains(opt.languages, langPrefix) {
				hasMatch = true
				break
			}
		}
	}

	return hasMatch
}
