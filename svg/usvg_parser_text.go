// Copyright 2019 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/parser/text.rs.

// Port: each impl FromValue of this file matches the value with a map, the
// same table as the match of upstream.

// textAnchorValues holds the values of the impl FromValue for TextAnchor in
// usvg src/parser/text.rs.
var textAnchorValues = map[string]TextAnchor{
	"start":   TextAnchorStart,
	cssMiddle: TextAnchorMiddle,
	"end":     TextAnchorEnd,
}

// fromValueTextAnchor is the impl FromValue for TextAnchor in usvg
// src/parser/text.rs.
func fromValueTextAnchor(_ svgNode, _ AId, value string) (TextAnchor, bool) {
	v, ok := textAnchorValues[value]
	return v, ok
}

// alignmentBaselineValues holds the values of the impl FromValue for
// AlignmentBaseline in usvg src/parser/text.rs.
var alignmentBaselineValues = map[string]AlignmentBaseline{
	cssAuto:           AlignmentBaselineAuto,
	cssBaseline:       AlignmentBaselineBaseline,
	"before-edge":     AlignmentBaselineBeforeEdge,
	cssTextBeforeEdge: AlignmentBaselineTextBeforeEdge,
	cssMiddle:         AlignmentBaselineMiddle,
	cssCentral:        AlignmentBaselineCentral,
	"after-edge":      AlignmentBaselineAfterEdge,
	cssTextAfterEdge:  AlignmentBaselineTextAfterEdge,
	cssIdeographic:    AlignmentBaselineIdeographic,
	cssAlphabetic:     AlignmentBaselineAlphabetic,
	cssHanging:        AlignmentBaselineHanging,
	cssMathematical:   AlignmentBaselineMathematical,
}

// fromValueAlignmentBaseline is the impl FromValue for AlignmentBaseline in
// usvg src/parser/text.rs.
func fromValueAlignmentBaseline(_ svgNode, _ AId, value string) (AlignmentBaseline, bool) {
	v, ok := alignmentBaselineValues[value]
	return v, ok
}

// dominantBaselineValues holds the values of the impl FromValue for
// DominantBaseline in usvg src/parser/text.rs.
var dominantBaselineValues = map[string]DominantBaseline{
	cssAuto:           DominantBaselineAuto,
	"use-script":      DominantBaselineUseScript,
	"no-change":       DominantBaselineNoChange,
	"reset-size":      DominantBaselineResetSize,
	cssIdeographic:    DominantBaselineIdeographic,
	cssAlphabetic:     DominantBaselineAlphabetic,
	cssHanging:        DominantBaselineHanging,
	cssMathematical:   DominantBaselineMathematical,
	cssCentral:        DominantBaselineCentral,
	cssMiddle:         DominantBaselineMiddle,
	cssTextAfterEdge:  DominantBaselineTextAfterEdge,
	cssTextBeforeEdge: DominantBaselineTextBeforeEdge,
}

// fromValueDominantBaseline is the impl FromValue for DominantBaseline in
// usvg src/parser/text.rs.
func fromValueDominantBaseline(_ svgNode, _ AId, value string) (DominantBaseline, bool) {
	v, ok := dominantBaselineValues[value]
	return v, ok
}

// lengthAdjustValues holds the values of the impl FromValue for
// LengthAdjust in usvg src/parser/text.rs.
var lengthAdjustValues = map[string]LengthAdjust{
	"spacing":          LengthAdjustSpacing,
	"spacingAndGlyphs": LengthAdjustSpacingAndGlyphs,
}

// fromValueLengthAdjust is the impl FromValue for LengthAdjust in usvg
// src/parser/text.rs.
func fromValueLengthAdjust(_ svgNode, _ AId, value string) (LengthAdjust, bool) {
	v, ok := lengthAdjustValues[value]
	return v, ok
}

// fontStyleValues holds the values of the impl FromValue for FontStyle in
// usvg src/parser/text.rs.
var fontStyleValues = map[string]FontStyle{
	cssNormal: FontStyleNormal,
	"italic":  FontStyleItalic,
	"oblique": FontStyleOblique,
}

// fromValueFontStyle is the impl FromValue for FontStyle in usvg
// src/parser/text.rs.
func fromValueFontStyle(_ svgNode, _ AId, value string) (FontStyle, bool) {
	v, ok := fontStyleValues[value]
	return v, ok
}

// The keywords of CSS that more than one property of this file takes.
const (
	cssNone           = "none"
	cssAuto           = "auto"
	cssNormal         = "normal"
	cssBaseline       = "baseline"
	cssMiddle         = "middle"
	cssCentral        = "central"
	cssTextBeforeEdge = "text-before-edge"
	cssTextAfterEdge  = "text-after-edge"
	cssIdeographic    = "ideographic"
	cssAlphabetic     = "alphabetic"
	cssHanging        = "hanging"
	cssMathematical   = "mathematical"
)

// characterPosition is CharacterPosition in usvg src/parser/text.rs. It is a
// text character position.
//
// _Character_ is a Unicode codepoint.
//
// Port: a nil field is None.
type characterPosition struct {
	// An absolute X axis position.
	x *float32
	// An absolute Y axis position.
	y *float32
	// A relative X axis offset.
	dx *float32
	// A relative Y axis offset.
	dy *float32
}

// convertTextNode is convert in usvg src/parser/text.rs. It takes another
// name, because convert of src/text/mod.rs is convertText (D10).
func convertTextNode(
	textNode svgNode,
	state *converterState,
	cache *converterCache,
	parent *Group,
) {
	posList := resolvePositionsList(textNode, state)
	rotateList := resolveRotateList(textNode)
	writingMode := convertWritingMode(textNode)

	chunks := collectTextChunks(textNode, posList, state, cache)

	renderingMode, ok := nodeFindAttribute(textNode, AIdTextRendering, fromValueTextRendering)
	if !ok {
		renderingMode = state.opt.textRendering
	}

	// Nodes generated by markers must not have an ID. Otherwise we would have duplicates.
	id := ""
	if len(state.parentMarkers) == 0 {
		id = textNode.elementID()
	}

	dummy := mustRect(path.RectFromXYWH(0.0, 0.0, 0.0, 0.0))

	dx := make([]float32, len(posList))
	dy := make([]float32, len(posList))
	for i, v := range posList {
		if v.dx != nil {
			dx[i] = *v.dx
		}
		if v.dy != nil {
			dy[i] = *v.dy
		}
	}

	text := &Text{
		id:            id,
		renderingMode: renderingMode,
		dx:            dx,
		dy:            dy,
		rotate:        rotateList,
		writingMode:   writingMode,
		chunks:        chunks,
		absTransform:  parent.absTransform,
		// All fields below will be reset by `text_to_paths`.
		boundingBox:          dummy,
		absBoundingBox:       dummy,
		strokeBoundingBox:    dummy,
		absStrokeBoundingBox: dummy,
		flattened:            emptyGroup(),
		layouted:             nil,
	}

	if !convertText(text, &state.opt.fontResolver, cache) {
		return
	}

	parent.children = append(parent.children, text)
}

// iterState is IterState in usvg src/parser/text.rs.
type iterState struct {
	charsCount      int
	chunkBytesCount int
	splitChunk      bool
	textFlow        TextFlow
	chunks          []TextChunk
}

// collectTextChunks is collect_text_chunks in usvg src/parser/text.rs.
func collectTextChunks(
	textNode svgNode,
	posList []characterPosition,
	state *converterState,
	cache *converterCache,
) []TextChunk {
	iterState := &iterState{
		charsCount:      0,
		chunkBytesCount: 0,
		splitChunk:      false,
		textFlow:        TextFlow{Kind: TextFlowLinear},
		chunks:          nil,
	}

	collectTextChunksImpl(textNode, posList, state, cache, iterState)

	return iterState.chunks
}

// collectTextChunksImpl is collect_text_chunks_impl in usvg
// src/parser/text.rs.
func collectTextChunksImpl(
	parent svgNode,
	posList []characterPosition,
	state *converterState,
	cache *converterCache,
	iterState *iterState,
) {
	for child := range parent.children() {
		if child.isElement() {
			if child.hasTagName(EIdTextPath) {
				if !parent.hasTagName(EIdText) {
					// `textPath` can be set only as a direct `text` element child.
					iterState.charsCount += countChars(child)
					continue
				}

				v, ok := resolveTextFlow(child, state)
				if !ok {
					// Skip an invalid text path and all it's children.
					// We have to update the chars count,
					// because `pos_list` was calculated including this text path.
					iterState.charsCount += countChars(child)
					continue
				}
				iterState.textFlow = v

				iterState.splitChunk = true
			}

			collectTextChunksImpl(child, posList, state, cache, iterState)

			iterState.textFlow = TextFlow{Kind: TextFlowLinear}

			// Next char after `textPath` should be split too.
			if child.hasTagName(EIdTextPath) {
				iterState.splitChunk = true
			}

			continue
		}

		if !parent.isVisibleElement(state.opt) {
			iterState.charsCount += utf8.RuneCountInString(child.text())
			continue
		}

		anchor, _ := nodeFindAttribute(parent, AIdTextAnchor, fromValueTextAnchor)

		// TODO: what to do when <= 0? UB?
		fontSizeValue := resolveFontSize(parent, state)
		fontSize, ok := path.NewNonZeroPositiveF32(fontSizeValue)
		if !ok {
			// Skip this span.
			iterState.charsCount += utf8.RuneCountInString(child.text())
			continue
		}

		font := convertFont(parent, state)

		rawPaintOrder, _ := nodeFindAttribute(parent, AIdPaintOrder, fromValuePaintOrder)
		paintOrder := svgPaintOrderToUsvg(rawPaintOrder)

		dominantBaseline, _ := nodeFindAttribute(parent, AIdDominantBaseline, fromValueDominantBaseline)

		// `no-change` means "use parent".
		if dominantBaseline == DominantBaselineNoChange {
			p, ok := parent.parentElement()
			if !ok {
				panic("called `Option::unwrap()` on a `None` value")
			}
			dominantBaseline, _ = nodeFindAttribute(p, AIdDominantBaseline, fromValueDominantBaseline)
		}

		applyKerning := true
		if parent.resolveLength(AIdKerning, state, -1.0) == 0.0 {
			applyKerning = false
		} else if v, ok := nodeFindAttribute(parent, AIdFontKerning, fromValueStr); ok && v == cssNone {
			applyKerning = false
		}

		fontOpticalSizing := FontOpticalSizingAuto // "auto" or missing (browser default)
		if v, ok := nodeFindAttribute(parent, AIdFontOpticalSizing, fromValueStr); ok && v == cssNone {
			fontOpticalSizing = FontOpticalSizingNone
		}

		var textLength *float32
		if n, ok := parent.tryConvertLength(AIdTextLength, unitsUserSpaceOnUse, state); ok {
			// Negative values should be ignored.
			if n >= 0.0 {
				textLength = &n
			}
		}

		vis, _ := nodeFindAttribute(parent, AIdVisibility, fromValueVisibility)

		fill, _ := resolveFill(parent, true, state, cache)
		stroke, _ := resolveStroke(parent, true, state, cache)
		smallCaps, _ := nodeFindAttribute(parent, AIdFontVariant, fromValueStr)
		alignmentBaseline, _ := nodeFindAttribute(parent, AIdAlignmentBaseline, fromValueAlignmentBaseline)
		lengthAdjust, _ := nodeFindAttribute(parent, AIdLengthAdjust, fromValueLengthAdjust)

		span := TextSpan{
			start:             0,
			end:               0,
			fill:              fill,
			stroke:            stroke,
			paintOrder:        paintOrder,
			font:              font,
			fontSize:          fontSize,
			smallCaps:         smallCaps == "small-caps",
			applyKerning:      applyKerning,
			fontOpticalSizing: fontOpticalSizing,
			decoration:        resolveDecoration(parent, state, cache),
			visible:           vis == visibilityVisible,
			dominantBaseline:  dominantBaseline,
			alignmentBaseline: alignmentBaseline,
			baselineShift:     convertBaselineShift(parent, state),
			letterSpacing:     parent.resolveLength(AIdLetterSpacing, state, 0.0),
			wordSpacing:       parent.resolveLength(AIdWordSpacing, state, 0.0),
			textLength:        textLength,
			lengthAdjust:      lengthAdjust,
		}

		isNewSpan := true
		for _, c := range child.text() {
			charLen := utf8.RuneLen(c)
			if charLen < 0 {
				charLen = 3 // U+FFFD, as Rust holds an invalid char
			}

			// Create a new chunk if:
			// - this is the first span (yes, position can be None)
			// - text character has an absolute coordinate assigned to it (via x/y attribute)
			// - `c` is the first char of the `textPath`
			// - `c` is the first char after `textPath`
			pos := posList[iterState.charsCount]
			isNewChunk := pos.x != nil ||
				pos.y != nil ||
				iterState.splitChunk ||
				len(iterState.chunks) == 0

			iterState.splitChunk = false

			switch {
			case isNewChunk:
				iterState.chunkBytesCount = 0

				span2 := span.clone()
				span2.start = 0
				span2.end = charLen

				iterState.chunks = append(iterState.chunks, TextChunk{
					x:        cloneF32Ptr(pos.x),
					y:        cloneF32Ptr(pos.y),
					anchor:   anchor,
					spans:    []TextSpan{span2},
					textFlow: iterState.textFlow,
					text:     string(c),
				})
			case isNewSpan:
				// Add this span to the last text chunk.
				span2 := span.clone()
				span2.start = iterState.chunkBytesCount
				span2.end = iterState.chunkBytesCount + charLen

				if n := len(iterState.chunks); n > 0 {
					chunk := &iterState.chunks[n-1]
					chunk.text += string(c)
					chunk.spans = append(chunk.spans, span2)
				}
			default:
				// Extend the last span.
				if n := len(iterState.chunks); n > 0 {
					chunk := &iterState.chunks[n-1]
					chunk.text += string(c)
					if m := len(chunk.spans); m > 0 {
						chunk.spans[m-1].end += charLen
					}
				}
			}

			isNewSpan = false
			iterState.charsCount++
			iterState.chunkBytesCount += charLen
		}
	}
}

// cloneF32Ptr returns a copy of the value of p, or nil for nil. It is the
// copy of an Option<f32> of upstream.
func cloneF32Ptr(p *float32) *float32 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// clone is the derived Clone for TextSpan in usvg src/tree/text.rs. It
// copies the fill, the stroke and the styles of the decorations, which the
// conversion of the paint servers changes in place.
func (s *TextSpan) clone() TextSpan {
	c := *s
	c.fill = cloneFill(s.fill)
	c.stroke = cloneStroke(s.stroke)
	c.decoration.underline = cloneDecorationStyle(s.decoration.underline)
	c.decoration.overline = cloneDecorationStyle(s.decoration.overline)
	c.decoration.lineThrough = cloneDecorationStyle(s.decoration.lineThrough)
	c.baselineShift = slices.Clone(s.baselineShift)
	return c
}

// cloneDecorationStyle is the Clone of an Option<TextDecorationStyle> of
// upstream. A nil style is None.
func cloneDecorationStyle(s *TextDecorationStyle) *TextDecorationStyle {
	if s == nil {
		return nil
	}
	return &TextDecorationStyle{fill: cloneFill(s.fill), stroke: cloneStroke(s.stroke)}
}

// resolveTextFlow is resolve_text_flow in usvg src/parser/text.rs.
func resolveTextFlow(node svgNode, state *converterState) (TextFlow, bool) {
	linkedNode, ok := node.attributeNode(AIdHref)
	if !ok {
		return TextFlow{}, false
	}
	p, ok := convertShapes(linkedNode, state)
	if !ok {
		return TextFlow{}, false
	}

	// The reference path's transform needs to be applied
	transform := linkedNode.resolveTransform(AIdTransform, state)
	if !transform.IsIdentity() {
		p, ok = p.Clone().Transform(transform)
		if !ok {
			return TextFlow{}, false
		}
	}

	startOffsetLength, _ := node.attributeLength(AIdStartOffset)
	var startOffset float32
	if startOffsetLength.Unit == svgtypes.LengthUnitPercent {
		// 'If a percentage is given, then the `startOffset` represents
		// a percentage distance along the entire path.'
		pathLen := pathLength(p)
		startOffset = float32(pathLen * (startOffsetLength.Number / 100.0))
	} else {
		startOffset = node.resolveLength(AIdStartOffset, state, 0.0)
	}

	id, ok := newNonEmptyString(linkedNode.elementID())
	if !ok {
		return TextFlow{}, false
	}
	return TextFlow{Kind: TextFlowPath, Path: &TextPath{
		id:          id,
		startOffset: startOffset,
		path:        p,
	}}, true
}

// convertFont is convert_font in usvg src/parser/text.rs.
func convertFont(node svgNode, state *converterState) Font {
	style, _ := nodeFindAttribute(node, AIdFontStyle, fromValueFontStyle)
	stretch := convFontStretch(node)
	weight := resolveFontWeight(node)
	variations := parseFontVariationSettings(node, state.opt)

	// Auto-map standard font properties to variation axes if not explicitly set.
	// This allows variable fonts to work with regular font-weight/font-stretch properties.
	hasTag := func(tag string) bool {
		return slices.ContainsFunc(variations, func(v FontVariation) bool { return string(v.Tag[:]) == tag })
	}
	hasWght := hasTag("wght")
	hasWdth := hasTag("wdth")
	hasItal := hasTag("ital")
	hasSlnt := hasTag("slnt")

	// Map font-weight to wght axis (if not already set)
	if !hasWght {
		variations = append(variations, NewFontVariation([4]byte{'w', 'g', 'h', 't'}, float32(weight)))
	}

	// Map font-stretch to wdth axis (if not already set)
	// CSS font-stretch percentages: ultra-condensed=50%, condensed=75%, normal=100%, expanded=125%, ultra-expanded=200%
	if !hasWdth {
		var wdth float32
		switch stretch {
		case FontStretchUltraCondensed:
			wdth = 50.0
		case FontStretchExtraCondensed:
			wdth = 62.5
		case FontStretchCondensed:
			wdth = 75.0
		case FontStretchSemiCondensed:
			wdth = 87.5
		case FontStretchNormal:
			wdth = 100.0
		case FontStretchSemiExpanded:
			wdth = 112.5
		case FontStretchExpanded:
			wdth = 125.0
		case FontStretchExtraExpanded:
			wdth = 150.0
		case FontStretchUltraExpanded:
			wdth = 200.0
		}
		if wdth != 100.0 {
			variations = append(variations, NewFontVariation([4]byte{'w', 'd', 't', 'h'}, wdth))
		}
	}

	// Map font-style: italic to ital axis (if not already set)
	if !hasItal && style == FontStyleItalic {
		variations = append(variations, NewFontVariation([4]byte{'i', 't', 'a', 'l'}, 1.0))
	}

	// Map font-style: oblique to slnt axis (if not already set)
	// Default oblique angle is typically 12-14 degrees
	if !hasSlnt && style == FontStyleOblique {
		variations = append(variations, NewFontVariation([4]byte{'s', 'l', 'n', 't'}, -12.0))
	}

	fontFamilies := ""
	for n := range node.ancestors() {
		if n.hasAttribute(AIdFontFamily) {
			fontFamilies, _ = n.attributeStr(AIdFontFamily)
			break
		}
	}

	parsed, err := svgtypes.ParseFontFamilies(fontFamilies)
	families, _ := logNone(parsed, err == nil, func() {
		state.opt.warnf(
			"Failed to parse %s value: '%s'. Falling back to %s.",
			AIdFontFamily,
			fontFamilies,
			state.opt.fontFamily,
		)
	})
	if err != nil {
		families = nil
	}

	if len(families) == 0 {
		families = append(families, FontFamily{Kind: FontFamilyNamed, Name: state.opt.fontFamily})
	}

	return Font{
		families:   families,
		style:      style,
		stretch:    stretch,
		weight:     weight,
		variations: variations,
	}
}

// parseFontVariationSettings is parse_font_variation_settings in usvg
// src/parser/text.rs. It parses the `font-variation-settings` CSS property.
//
// Syntax: `normal | [ <string> <number> ]#`
// Example: `"wght" 700, "wdth" 50`
//
// Port: opt holds the logger (D20).
func parseFontVariationSettings(node svgNode, opt *options) []FontVariation {
	var value string
	found := false
	for n := range node.ancestors() {
		if n.hasAttribute(AIdFontVariationSettings) {
			value, _ = n.attributeStr(AIdFontVariationSettings)
			opt.debugf("Found font-variation-settings: '%s'", value)
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	// "normal" means no variations
	if strings.EqualFold(value, cssNormal) || value == "" {
		return nil
	}

	var variations []FontVariation

	// Parse comma-separated list of "tag" value pairs
	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimFunc(part, unicode.IsSpace)
		if part == "" {
			continue
		}

		// Find the tag (quoted string) and value
		// Format: "wght" 700 or 'wght' 700
		chars := []rune(part)
		i := 0

		// Skip whitespace
		for i < len(chars) && unicode.IsSpace(chars[i]) {
			i++
		}

		// Parse quoted tag
		if i >= len(chars) || (chars[i] != '"' && chars[i] != '\'') {
			continue // Invalid format
		}
		quote := chars[i]
		i++

		var tagStr strings.Builder
		for i < len(chars) {
			c := chars[i]
			i++
			if c == quote {
				break
			}
			tagStr.WriteRune(c)
		}

		// Tag must be exactly 4 characters
		if tagStr.Len() != 4 {
			opt.warnf(
				"Invalid font-variation-settings tag: '%s' (must be 4 characters)",
				tagStr.String(),
			)
			continue
		}

		// Skip whitespace before value
		for i < len(chars) && unicode.IsSpace(chars[i]) {
			i++
		}

		// Parse the numeric value
		valueStr := strings.TrimFunc(string(chars[i:]), unicode.IsSpace)

		v, ok := parseRustF32(valueStr)
		if !ok {
			opt.warnf("Invalid font-variation-settings value: '%s'", valueStr)
			continue
		}

		tagBytes := tagStr.String()
		tag := [4]byte{tagBytes[0], tagBytes[1], tagBytes[2], tagBytes[3]}

		variations = append(variations, NewFontVariation(tag, v))
	}

	return variations
}

// parseRustF32 is str::parse::<f32> of Rust. It accepts a decimal number
// with an optional sign and exponent, "inf", "infinity" and "NaN" in any
// case, and nothing else.
func parseRustF32(s string) (float32, bool) {
	if s == "" {
		return 0, false
	}
	t := strings.TrimLeft(s, "+-")
	if len(s)-len(t) > 1 {
		return 0, false
	}
	switch strings.ToLower(t) {
	case "inf", "infinity", "nan":
	default:
		for _, c := range t {
			if !(c >= '0' && c <= '9' || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-') {
				return 0, false
			}
		}
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		var numErr *strconv.NumError
		if !errors.As(err, &numErr) || !errors.Is(numErr.Err, strconv.ErrRange) {
			return 0, false
		}
	}
	return float32(v), true
}

// convFontStretch is conv_font_stretch in usvg src/parser/text.rs.
//
// TODO: properly resolve narrower/wider.
func convFontStretch(node svgNode) FontStretch {
	for n := range node.ancestors() {
		if !n.hasAttribute(AIdFontStretch) {
			continue
		}
		v, _ := n.attributeStr(AIdFontStretch)
		switch v {
		case "narrower", "condensed":
			return FontStretchCondensed
		case "ultra-condensed":
			return FontStretchUltraCondensed
		case "extra-condensed":
			return FontStretchExtraCondensed
		case "semi-condensed":
			return FontStretchSemiCondensed
		case "semi-expanded":
			return FontStretchSemiExpanded
		case "wider", "expanded":
			return FontStretchExpanded
		case "extra-expanded":
			return FontStretchExtraExpanded
		case "ultra-expanded":
			return FontStretchUltraExpanded
		}
		return FontStretchNormal
	}
	return FontStretchNormal
}

// fontWeightLighter is the value lighter of font-weight.
const fontWeightLighter = "lighter"

// fontWeightValues holds the numeric values of font-weight that
// resolve_font_weight in usvg src/parser/text.rs matches.
var fontWeightValues = map[string]int{
	cssNormal: 400,
	"bold":    700,
	"100":     100,
	"200":     200,
	"300":     300,
	"400":     400,
	"500":     500,
	"600":     600,
	"700":     700,
	"800":     800,
	"900":     900,
}

// resolveFontWeight is resolve_font_weight in usvg src/parser/text.rs.
func resolveFontWeight(node svgNode) uint16 {
	bound := func(minimum, val, maximum int) int {
		return max(minimum, min(maximum, val))
	}

	var nodes []svgNode
	for n := range node.ancestors() {
		nodes = append(nodes, n)
	}
	weight := 400
	// skip Root
	for i := len(nodes) - 2; i >= 0; i-- {
		n := nodes[i]
		v, _ := n.attributeStr(AIdFontWeight)
		switch v {
		case "bolder":
			// By the CSS2 spec the default value should be 400
			// so `bolder` will result in 500.
			// But Chrome and Inkscape will give us 700.
			// Have no idea is it a bug or something, but
			// we will follow such behavior for now.
			step := 100
			if weight == 400 {
				step = 300
			}

			weight = bound(100, weight+step, 900)
		case fontWeightLighter:
			// By the CSS2 spec the default value should be 400
			// so `lighter` will result in 300.
			// But Chrome and Inkscape will give us 200.
			// Have no idea is it a bug or something, but
			// we will follow such behavior for now.
			step := 100
			if weight == 400 {
				step = 200
			}

			weight = bound(100, weight-step, 900)
		default:
			if w, ok := fontWeightValues[v]; ok {
				weight = w
			}
		}
	}

	return uint16(weight)
}

// resolvePositionsList is resolve_positions_list in usvg src/parser/text.rs.
// It resolves text's character positions.
//
// This includes: x, y, dx, dy.
//
// # The character
//
// The first problem with this task is that the *character* itself
// is basically undefined in the SVG spec. Sometimes it's an *XML character*,
// sometimes a *glyph*, and sometimes just a *character*.
//
// There is an ongoing [discussion](https://github.com/w3c/svgwg/issues/537)
// on the SVG working group that addresses this by stating that a character
// is a Unicode code point. But it's not final.
//
// Also, according to the SVG 2 spec, *character* is *a Unicode code point*.
//
// Anyway, we treat a character as a Unicode code point.
//
// # Algorithm
//
// To resolve positions, we have to iterate over descendant nodes and
// if the current node is a `tspan` and has x/y/dx/dy attribute,
// than the positions from this attribute should be assigned to the characters
// of this `tspan` and it's descendants.
//
// Positions list can have more values than characters in the `tspan`,
// so we have to clamp it, because values should not overlap, e.g.:
//
// (we ignore whitespaces for example purposes,
// so the `text` content is `Text` and not `T ex t`)
//
//	<text>
//	  a
//	  <tspan x="10 20 30">
//	    bc
//	  </tspan>
//	  d
//	</text>
//
// In this example, the `d` position should not be set to `30`.
// And the result should be: `[None, 10, 20, None]`
//
// Another example:
//
//	<text>
//	  <tspan x="100 110 120 130">
//	    a
//	    <tspan x="50">
//	      bc
//	    </tspan>
//	  </tspan>
//	  d
//	</text>
//
// The result should be: `[100, 50, 120, None]`.
func resolvePositionsList(textNode svgNode, state *converterState) []characterPosition {
	// Allocate a list that has all characters positions set to `None`.
	totalChars := countChars(textNode)
	list := make([]characterPosition, totalChars)

	offset := 0
	for child := range textNode.descendants() {
		if child.isElement() {
			// We must ignore text positions on `textPath`.
			if !child.hasTagName(EIdText) && !child.hasTagName(EIdTspan) {
				continue
			}

			childChars := countChars(child)
			pushList := func(aid AId, field func(*characterPosition) **float32) {
				if numList, ok := convertList(child, aid, state); ok {
					// Note that we are using not the total count,
					// but the amount of characters in the current `tspan` (with children).
					n := min(len(numList), childChars)
					for i := range n {
						v := numList[i]
						*field(&list[offset+i]) = &v
					}
				}
			}

			pushList(AIdX, func(c *characterPosition) **float32 { return &c.x })
			pushList(AIdY, func(c *characterPosition) **float32 { return &c.y })
			pushList(AIdDx, func(c *characterPosition) **float32 { return &c.dx })
			pushList(AIdDy, func(c *characterPosition) **float32 { return &c.dy })
		} else if child.isText() {
			// Advance the offset.
			offset += utf8.RuneCountInString(child.text())
		}
	}

	return list
}

// resolveRotateList is resolve_rotate_list in usvg src/parser/text.rs. It
// resolves characters rotation.
//
// The algorithm is well explained
// [in the SVG spec](https://www.w3.org/TR/SVG11/text.html#TSpanElement) (scroll down a bit).
//
// ![](https://www.w3.org/TR/SVG11/images/text/tspan05-diagram.png)
//
// Note: this algorithm differs from the position resolving one.
func resolveRotateList(textNode svgNode) []float32 {
	// Allocate a list that has all characters angles set to `0.0`.
	list := make([]float32, countChars(textNode))
	var last float32
	offset := 0
	for child := range textNode.descendants() {
		if child.isElement() {
			if rotate, ok := nodeAttribute(child, AIdRotate, fromValueNumberList); ok {
				for i := range countChars(child) {
					if i < len(rotate) {
						a := rotate[i]
						list[offset+i] = a
						last = a
					} else {
						// If the rotate list doesn't specify the rotation for
						// this character - use the last one.
						list[offset+i] = last
					}
				}
			}
		} else if child.isText() {
			// Advance the offset.
			offset += utf8.RuneCountInString(child.text())
		}
	}

	return list
}

// findDecoration is the inner function find_decoration of
// resolve_decoration in usvg src/parser/text.rs. It checks if a decoration
// is present in a single node.
func findDecoration(node svgNode, value string) bool {
	if strValue, ok := nodeAttribute(node, AIdTextDecoration, fromValueStr); ok {
		return slices.Contains(strings.Split(strValue, " "), value)
	}
	return false
}

// resolveDecoration is resolve_decoration in usvg src/parser/text.rs. It
// resolves node's `text-decoration` property.
func resolveDecoration(
	tspan svgNode,
	state *converterState,
	cache *converterCache,
) TextDecoration {
	// The algorithm is as follows: First, we check whether the given text decoration appears in ANY
	// ancestor, i.e. it can also appear in ancestors outside of the <text> element. If the text
	// decoration is declared somewhere, it means that this tspan will have it. However, we still
	// need to find the corresponding fill/stroke for it. To do this, we iterate through all
	// ancestors (i.e. tspans) until we find the text decoration declared. If not, we will
	// stop at latest at the text node, and use its fill/stroke.
	genStyle := func(textDecoration string) *TextDecorationStyle {
		found := false
		for n := range tspan.ancestors() {
			if findDecoration(n, textDecoration) {
				found = true
				break
			}
		}
		if !found {
			return nil
		}

		var fillNode *svgNode
		var strokeNode *svgNode

		for node := range tspan.ancestors() {
			if findDecoration(node, textDecoration) || node.hasTagName(EIdText) {
				if fillNode == nil {
					n := node
					fillNode = &n
				}
				if strokeNode == nil {
					n := node
					strokeNode = &n
				}
				break
			}
		}

		style := &TextDecorationStyle{}
		if fillNode != nil {
			style.fill, _ = resolveFill(*fillNode, true, state, cache)
		}
		if strokeNode != nil {
			style.stroke, _ = resolveStroke(*strokeNode, true, state, cache)
		}
		return style
	}

	return TextDecoration{
		underline:   genStyle("underline"),
		overline:    genStyle("overline"),
		lineThrough: genStyle("line-through"),
	}
}

// convertBaselineShift is convert_baseline_shift in usvg
// src/parser/text.rs.
func convertBaselineShift(node svgNode, state *converterState) []BaselineShift {
	var shift []BaselineShift
	var nodes []svgNode
	for n := range node.ancestors() {
		if n.hasTagName(EIdText) {
			break
		}
		nodes = append(nodes, n)
	}
	for _, n := range nodes {
		if length, ok := nodeTryAttribute(n, AIdBaselineShift, fromValueLength); ok {
			if length.Unit == svgtypes.LengthUnitPercent {
				v := resolveFontSize(n, state) * (float32(length.Number) / 100.0)
				shift = append(shift, BaselineShift{Kind: BaselineShiftNumber, Number: v})
			} else {
				v := convertLength(
					length,
					n,
					AIdBaselineShift,
					unitsObjectBoundingBox,
					state,
				)
				shift = append(shift, BaselineShift{Kind: BaselineShiftNumber, Number: v})
			}
		} else if s, ok := nodeAttribute(n, AIdBaselineShift, fromValueStr); ok {
			switch s {
			case "sub":
				shift = append(shift, BaselineShift{Kind: BaselineShiftSubscript})
			case "super":
				shift = append(shift, BaselineShift{Kind: BaselineShiftSuperscript})
			default:
				shift = append(shift, BaselineShift{Kind: BaselineShiftBaseline})
			}
		}
	}

	if !slices.ContainsFunc(shift, func(base BaselineShift) bool { return base.Kind != BaselineShiftBaseline }) {
		shift = nil
	}

	return shift
}

// countChars is count_chars in usvg src/parser/text.rs.
func countChars(node svgNode) int {
	n := 0
	for d := range node.descendants() {
		if d.isText() {
			n += utf8.RuneCountInString(d.text())
		}
	}
	return n
}

// convertWritingMode is convert_writing_mode in usvg src/parser/text.rs. It
// converts the writing mode.
//
// [SVG 2] references [CSS Writing Modes Level 3] for the definition of the
// 'writing-mode' property, there are only two writing modes:
// horizontal left-to-right and vertical right-to-left.
//
// That specification introduces new values for the property. The SVG 1.1
// values are obsolete but must still be supported by converting the specified
// values to computed values as follows:
//
//   - `lr`, `lr-tb`, `rl`, `rl-tb` => `horizontal-tb`
//   - `tb`, `tb-rl` => `vertical-rl`
//
// The current `vertical-lr` behaves exactly the same as `vertical-rl`.
//
// Also, looks like no one really supports the `rl` and `rl-tb`, except `Batik`.
// And I'm not sure if its behaviour is correct.
//
// So we will ignore it as well, mainly because I have no idea how exactly
// it should affect the rendering.
//
// [SVG 2]: https://www.w3.org/TR/SVG2/text.html#WritingModeProperty
// [CSS Writing Modes Level 3]: https://www.w3.org/TR/css-writing-modes-3/#svg-writing-mode-css
func convertWritingMode(textNode svgNode) WritingMode {
	for n := range textNode.ancestors() {
		if !n.hasAttribute(AIdWritingMode) {
			continue
		}
		v, ok := n.attributeStr(AIdWritingMode)
		if !ok {
			v = "lr-tb"
		}
		switch v {
		case "tb", "tb-rl", "vertical-rl", "vertical-lr":
			return WritingModeTopToBottom
		}
		return WritingModeLeftToRight
	}
	return WritingModeLeftToRight
}

// pathLength is path_length in usvg src/parser/text.rs.
func pathLength(p *path.Path) float64 {
	prevMX := p.Points()[0].X
	prevMY := p.Points()[0].Y
	prevX := prevMX
	prevY := prevMY

	var length float64
	for seg := range p.Segments().All() {
		curve, ok := segmentCurve(seg, &prevMX, &prevMY, &prevX, &prevY)
		if !ok {
			continue
		}

		length += curve.Arclen(0.5)
		prevX = float32(curve.P3.X)
		prevY = float32(curve.P3.Y)
	}

	return length
}
