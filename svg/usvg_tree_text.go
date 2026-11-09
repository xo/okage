// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"math"

	"github.com/xo/okage/internal/svgtypes"
	"github.com/xo/okage/path"
)

// This file ports usvg src/tree/text.rs, without the items of the feature
// text.
//
// TODO(phase 9): the items of the feature text: the conversions between
// FontStretch and fontdb::Stretch, between FontStyle and fontdb::Style, the
// field layouted of Text and the method Text::layouted.

// FontFamily is the re-export of svgtypes::FontFamily in usvg
// src/tree/text.rs.
type FontFamily = svgtypes.FontFamily

// FontStretch is FontStretch in usvg src/tree/text.rs. It is a font stretch
// property.
//
// The Default of upstream is FontStretchNormal, which DefaultFontStretch
// returns. The values are ordered as in upstream, so < compares them as the
// derived Ord does.
type FontStretch uint8

// The values of FontStretch.
const (
	FontStretchUltraCondensed FontStretch = iota
	FontStretchExtraCondensed
	FontStretchCondensed
	FontStretchSemiCondensed
	FontStretchNormal
	FontStretchSemiExpanded
	FontStretchExpanded
	FontStretchExtraExpanded
	FontStretchUltraExpanded
)

// String returns the Rust name of the value.
func (s FontStretch) String() string {
	switch s {
	case FontStretchUltraCondensed:
		return "UltraCondensed"
	case FontStretchExtraCondensed:
		return "ExtraCondensed"
	case FontStretchCondensed:
		return "Condensed"
	case FontStretchSemiCondensed:
		return "SemiCondensed"
	case FontStretchNormal:
		return "Normal"
	case FontStretchSemiExpanded:
		return "SemiExpanded"
	case FontStretchExpanded:
		return "Expanded"
	case FontStretchExtraExpanded:
		return "ExtraExpanded"
	case FontStretchUltraExpanded:
		return "UltraExpanded"
	}
	return "FontStretch(?)"
}

// DefaultFontStretch is the impl Default for FontStretch in usvg
// src/tree/text.rs.
func DefaultFontStretch() FontStretch {
	return FontStretchNormal
}

// FontVariation is FontVariation in usvg src/tree/text.rs. It is a font
// variation axis setting.
//
// Used for variable fonts to specify axis values like weight, width, etc.
//
// Port: the impl PartialEq compares the bits of the value. Go compares two
// values with Eq, not with ==.
type FontVariation struct {
	// The 4-byte axis tag (e.g., b"wght" for weight).
	Tag [4]byte
	// The axis value.
	Value float32
}

// NewFontVariation is FontVariation::new in usvg src/tree/text.rs. It creates
// a new font variation.
func NewFontVariation(tag [4]byte, value float32) FontVariation {
	return FontVariation{Tag: tag, Value: value}
}

// Eq is the impl PartialEq for FontVariation in usvg src/tree/text.rs.
func (v FontVariation) Eq(other FontVariation) bool {
	return v.Tag == other.Tag && math.Float32bits(v.Value) == math.Float32bits(other.Value)
}

// FontStyle is FontStyle in usvg src/tree/text.rs. It is a font style
// property.
//
// The zero value is FontStyleNormal, the Default of upstream.
type FontStyle uint8

// The values of FontStyle.
const (
	// FontStyleNormal is a face that is neither italic not obliqued.
	FontStyleNormal FontStyle = iota
	// FontStyleItalic is a form that is generally cursive in nature.
	FontStyleItalic
	// FontStyleOblique is a typically-sloped version of the regular face.
	FontStyleOblique
)

// String returns the Rust name of the value.
func (s FontStyle) String() string {
	switch s {
	case FontStyleNormal:
		return "Normal"
	case FontStyleItalic:
		return "Italic"
	case FontStyleOblique:
		return "Oblique"
	}
	return "FontStyle(?)"
}

// Font is Font in usvg src/tree/text.rs. It holds the text font properties.
type Font struct {
	families   []FontFamily
	style      FontStyle
	stretch    FontStretch
	weight     uint16
	variations []FontVariation
}

// Families is Font::families in usvg src/tree/text.rs. It returns a list of
// family names.
//
// Never empty. Uses `usvg::Options::font_family` as fallback.
func (f *Font) Families() []FontFamily {
	return f.families
}

// Style is Font::style in usvg src/tree/text.rs. It returns a font style.
func (f *Font) Style() FontStyle {
	return f.style
}

// Stretch is Font::stretch in usvg src/tree/text.rs. It returns a font
// stretch.
func (f *Font) Stretch() FontStretch {
	return f.stretch
}

// Weight is Font::weight in usvg src/tree/text.rs. It returns a font width.
func (f *Font) Weight() uint16 {
	return f.weight
}

// Variations is Font::variations in usvg src/tree/text.rs. It returns the
// font variation settings for variable fonts.
func (f *Font) Variations() []FontVariation {
	return f.variations
}

// DominantBaseline is DominantBaseline in usvg src/tree/text.rs. It is a
// dominant baseline property.
//
// The zero value is DominantBaselineAuto, the Default of upstream.
type DominantBaseline uint8

// The values of DominantBaseline.
const (
	DominantBaselineAuto DominantBaseline = iota
	DominantBaselineUseScript
	DominantBaselineNoChange
	DominantBaselineResetSize
	DominantBaselineIdeographic
	DominantBaselineAlphabetic
	DominantBaselineHanging
	DominantBaselineMathematical
	DominantBaselineCentral
	DominantBaselineMiddle
	DominantBaselineTextAfterEdge
	DominantBaselineTextBeforeEdge
)

// String returns the Rust name of the value.
func (b DominantBaseline) String() string {
	switch b {
	case DominantBaselineAuto:
		return "Auto" //nolint:goconst // the Rust name of the value
	case DominantBaselineUseScript:
		return "UseScript"
	case DominantBaselineNoChange:
		return "NoChange"
	case DominantBaselineResetSize:
		return "ResetSize"
	case DominantBaselineIdeographic:
		return "Ideographic"
	case DominantBaselineAlphabetic:
		return "Alphabetic"
	case DominantBaselineHanging:
		return "Hanging"
	case DominantBaselineMathematical:
		return "Mathematical"
	case DominantBaselineCentral:
		return "Central"
	case DominantBaselineMiddle:
		return "Middle" //nolint:goconst // the Rust name of the value
	case DominantBaselineTextAfterEdge:
		return "TextAfterEdge"
	case DominantBaselineTextBeforeEdge:
		return "TextBeforeEdge"
	}
	return "DominantBaseline(?)"
}

// AlignmentBaseline is AlignmentBaseline in usvg src/tree/text.rs. It is an
// alignment baseline property.
//
// The zero value is AlignmentBaselineAuto, the Default of upstream.
type AlignmentBaseline uint8

// The values of AlignmentBaseline.
const (
	AlignmentBaselineAuto AlignmentBaseline = iota
	AlignmentBaselineBaseline
	AlignmentBaselineBeforeEdge
	AlignmentBaselineTextBeforeEdge
	AlignmentBaselineMiddle
	AlignmentBaselineCentral
	AlignmentBaselineAfterEdge
	AlignmentBaselineTextAfterEdge
	AlignmentBaselineIdeographic
	AlignmentBaselineAlphabetic
	AlignmentBaselineHanging
	AlignmentBaselineMathematical
)

// String returns the Rust name of the value.
func (b AlignmentBaseline) String() string {
	switch b {
	case AlignmentBaselineAuto:
		return "Auto"
	case AlignmentBaselineBaseline:
		return "Baseline"
	case AlignmentBaselineBeforeEdge:
		return "BeforeEdge"
	case AlignmentBaselineTextBeforeEdge:
		return "TextBeforeEdge"
	case AlignmentBaselineMiddle:
		return "Middle"
	case AlignmentBaselineCentral:
		return "Central"
	case AlignmentBaselineAfterEdge:
		return "AfterEdge"
	case AlignmentBaselineTextAfterEdge:
		return "TextAfterEdge"
	case AlignmentBaselineIdeographic:
		return "Ideographic"
	case AlignmentBaselineAlphabetic:
		return "Alphabetic"
	case AlignmentBaselineHanging:
		return "Hanging"
	case AlignmentBaselineMathematical:
		return "Mathematical"
	}
	return "AlignmentBaseline(?)"
}

// BaselineShiftKind is the case of a BaselineShift.
type BaselineShiftKind uint8

// The cases of the enum BaselineShift in usvg src/tree/text.rs.
const (
	BaselineShiftBaseline BaselineShiftKind = iota
	BaselineShiftSubscript
	BaselineShiftSuperscript
	BaselineShiftNumber
)

// String returns the Rust name of the case.
func (k BaselineShiftKind) String() string {
	switch k {
	case BaselineShiftBaseline:
		return "Baseline"
	case BaselineShiftSubscript:
		return "Subscript"
	case BaselineShiftSuperscript:
		return "Superscript"
	case BaselineShiftNumber:
		return "Number"
	}
	return "BaselineShiftKind(?)"
}

// BaselineShift is BaselineShift in usvg src/tree/text.rs. It is a baseline
// shift property.
//
// Port: only the case Number holds data, so the enum becomes a struct with a
// kind (D27). The == operator is the derived PartialEq of upstream, as long
// as Number is zero for the other cases. The zero value is
// BaselineShiftBaseline, the Default of upstream.
type BaselineShift struct {
	Kind BaselineShiftKind
	// Number is the value of BaselineShiftNumber.
	Number float32
}

// LengthAdjust is LengthAdjust in usvg src/tree/text.rs. It is a length
// adjust property.
//
// The zero value is LengthAdjustSpacing, the Default of upstream.
type LengthAdjust uint8

// The values of LengthAdjust.
const (
	LengthAdjustSpacing LengthAdjust = iota
	LengthAdjustSpacingAndGlyphs
)

// String returns the Rust name of the value.
func (a LengthAdjust) String() string {
	switch a {
	case LengthAdjustSpacing:
		return "Spacing"
	case LengthAdjustSpacingAndGlyphs:
		return "SpacingAndGlyphs"
	}
	return "LengthAdjust(?)"
}

// FontOpticalSizing is FontOpticalSizing in usvg src/tree/text.rs. It is a
// font optical sizing property.
//
// Controls automatic adjustment of the `opsz` axis in variable fonts
// based on font size. Matches CSS `font-optical-sizing`.
//
// The zero value is FontOpticalSizingAuto, the Default of upstream.
type FontOpticalSizing uint8

// The values of FontOpticalSizing.
const (
	// FontOpticalSizingAuto automatically sets `opsz` to match font size
	// (browser default).
	FontOpticalSizingAuto FontOpticalSizing = iota
	// FontOpticalSizingNone does not automatically adjust `opsz`.
	FontOpticalSizingNone
)

// String returns the Rust name of the value.
func (s FontOpticalSizing) String() string {
	switch s {
	case FontOpticalSizingAuto:
		return "Auto"
	case FontOpticalSizingNone:
		return "None"
	}
	return "FontOpticalSizing(?)"
}

// TextDecorationStyle is TextDecorationStyle in usvg src/tree/text.rs. It is
// a text span decoration style.
//
// In SVG, text decoration and text it's applied to can have different styles.
// So you can have black text and green underline.
//
// Also, in SVG you can specify text decoration stroking.
type TextDecorationStyle struct {
	fill   *Fill
	stroke *Stroke
}

// Fill is TextDecorationStyle::fill in usvg src/tree/text.rs. It returns a
// fill style.
func (s *TextDecorationStyle) Fill() (*Fill, bool) {
	return s.fill, s.fill != nil
}

// Stroke is TextDecorationStyle::stroke in usvg src/tree/text.rs. It returns
// a stroke style.
func (s *TextDecorationStyle) Stroke() (*Stroke, bool) {
	return s.stroke, s.stroke != nil
}

// TextDecoration is TextDecoration in usvg src/tree/text.rs. It is a text
// span decoration.
type TextDecoration struct {
	underline   *TextDecorationStyle
	overline    *TextDecorationStyle
	lineThrough *TextDecorationStyle
}

// Underline is TextDecoration::underline in usvg src/tree/text.rs. It returns
// an optional underline and its style.
func (d *TextDecoration) Underline() (*TextDecorationStyle, bool) {
	return d.underline, d.underline != nil
}

// Overline is TextDecoration::overline in usvg src/tree/text.rs. It returns
// an optional overline and its style.
func (d *TextDecoration) Overline() (*TextDecorationStyle, bool) {
	return d.overline, d.overline != nil
}

// LineThrough is TextDecoration::line_through in usvg src/tree/text.rs. It
// returns an optional line-through and its style.
func (d *TextDecoration) LineThrough() (*TextDecorationStyle, bool) {
	return d.lineThrough, d.lineThrough != nil
}

// TextSpan is TextSpan in usvg src/tree/text.rs. It is a text style span.
//
// Spans do not overlap inside a text chunk.
type TextSpan struct {
	start             int
	end               int
	fill              *Fill
	stroke            *Stroke
	paintOrder        PaintOrder
	font              Font
	fontSize          NonZeroPositiveF32
	smallCaps         bool
	applyKerning      bool
	fontOpticalSizing FontOpticalSizing
	decoration        TextDecoration
	dominantBaseline  DominantBaseline
	alignmentBaseline AlignmentBaseline
	baselineShift     []BaselineShift
	visible           bool
	letterSpacing     float32
	wordSpacing       float32
	textLength        *float32
	lengthAdjust      LengthAdjust
}

// Start is TextSpan::start in usvg src/tree/text.rs. It returns a span start
// in bytes.
//
// Offset is relative to the parent text chunk and not the parent text element.
func (s *TextSpan) Start() int {
	return s.start
}

// End is TextSpan::end in usvg src/tree/text.rs. It returns a span end in
// bytes.
//
// Offset is relative to the parent text chunk and not the parent text element.
func (s *TextSpan) End() int {
	return s.end
}

// Fill is TextSpan::fill in usvg src/tree/text.rs. It returns a fill style.
func (s *TextSpan) Fill() (*Fill, bool) {
	return s.fill, s.fill != nil
}

// Stroke is TextSpan::stroke in usvg src/tree/text.rs. It returns a stroke
// style.
func (s *TextSpan) Stroke() (*Stroke, bool) {
	return s.stroke, s.stroke != nil
}

// PaintOrder is TextSpan::paint_order in usvg src/tree/text.rs. It returns a
// paint order style.
func (s *TextSpan) PaintOrder() PaintOrder {
	return s.paintOrder
}

// Font is TextSpan::font in usvg src/tree/text.rs. It returns a font.
func (s *TextSpan) Font() *Font {
	return &s.font
}

// FontSize is TextSpan::font_size in usvg src/tree/text.rs. It returns a font
// size.
func (s *TextSpan) FontSize() NonZeroPositiveF32 {
	return s.fontSize
}

// SmallCaps is TextSpan::small_caps in usvg src/tree/text.rs. It indicates
// that small caps should be used.
//
// Set by `font-variant="small-caps"`.
func (s *TextSpan) SmallCaps() bool {
	return s.smallCaps
}

// ApplyKerning is TextSpan::apply_kerning in usvg src/tree/text.rs. It
// indicates that a kerning should be applied.
//
// Supports both `kerning` and `font-kerning` properties.
func (s *TextSpan) ApplyKerning() bool {
	return s.applyKerning
}

// FontOpticalSizing is TextSpan::font_optical_sizing in usvg
// src/tree/text.rs. It returns the font optical sizing mode.
//
// When `Auto` (default), the `opsz` axis will be automatically set
// to match the font size for variable fonts that support it.
// This matches the CSS `font-optical-sizing: auto` behavior.
func (s *TextSpan) FontOpticalSizing() FontOpticalSizing {
	return s.fontOpticalSizing
}

// Decoration is TextSpan::decoration in usvg src/tree/text.rs. It returns a
// span decorations.
func (s *TextSpan) Decoration() *TextDecoration {
	return &s.decoration
}

// DominantBaseline is TextSpan::dominant_baseline in usvg src/tree/text.rs.
// It returns a span dominant baseline.
func (s *TextSpan) DominantBaseline() DominantBaseline {
	return s.dominantBaseline
}

// AlignmentBaseline is TextSpan::alignment_baseline in usvg
// src/tree/text.rs. It returns a span alignment baseline.
func (s *TextSpan) AlignmentBaseline() AlignmentBaseline {
	return s.alignmentBaseline
}

// BaselineShift is TextSpan::baseline_shift in usvg src/tree/text.rs. It
// returns a list of all baseline shift that should be applied to this span.
//
// Ordered from `text` element down to the actual `span` element.
func (s *TextSpan) BaselineShift() []BaselineShift {
	return s.baselineShift
}

// IsVisible is TextSpan::is_visible in usvg src/tree/text.rs. It returns a
// visibility property.
func (s *TextSpan) IsVisible() bool {
	return s.visible
}

// LetterSpacing is TextSpan::letter_spacing in usvg src/tree/text.rs. It
// returns a letter spacing property.
func (s *TextSpan) LetterSpacing() float32 {
	return s.letterSpacing
}

// WordSpacing is TextSpan::word_spacing in usvg src/tree/text.rs. It returns
// a word spacing property.
func (s *TextSpan) WordSpacing() float32 {
	return s.wordSpacing
}

// TextLength is TextSpan::text_length in usvg src/tree/text.rs. It returns a
// text length property.
func (s *TextSpan) TextLength() (float32, bool) {
	if s.textLength == nil {
		return 0, false
	}
	return *s.textLength, true
}

// LengthAdjust is TextSpan::length_adjust in usvg src/tree/text.rs. It
// returns a length adjust property.
func (s *TextSpan) LengthAdjust() LengthAdjust {
	return s.lengthAdjust
}

// TextAnchor is TextAnchor in usvg src/tree/text.rs. It is a text chunk
// anchor property.
//
// The zero value is TextAnchorStart, the Default of upstream.
type TextAnchor uint8

// The values of TextAnchor.
const (
	TextAnchorStart TextAnchor = iota
	TextAnchorMiddle
	TextAnchorEnd
)

// String returns the Rust name of the value.
func (a TextAnchor) String() string {
	switch a {
	case TextAnchorStart:
		return "Start"
	case TextAnchorMiddle:
		return "Middle"
	case TextAnchorEnd:
		return "End"
	}
	return "TextAnchor(?)"
}

// TextPath is TextPath in usvg src/tree/text.rs. It is a path used by
// text-on-path.
type TextPath struct {
	id          nonEmptyString
	startOffset float32
	path        *path.Path
}

// ID is TextPath::id in usvg src/tree/text.rs. It returns the element's ID.
//
// Taken from the SVG itself.
func (p *TextPath) ID() string {
	return p.id.get()
}

// StartOffset is TextPath::start_offset in usvg src/tree/text.rs. It returns
// a text offset in SVG coordinates.
//
// Percentage values already resolved.
func (p *TextPath) StartOffset() float32 {
	return p.startOffset
}

// Path is TextPath::path in usvg src/tree/text.rs. It returns a path.
func (p *TextPath) Path() *path.Path {
	return p.path
}

// TextFlowKind is the case of a TextFlow.
type TextFlowKind uint8

// The cases of the enum TextFlow in usvg src/tree/text.rs.
const (
	// TextFlowLinear is a linear layout.
	//
	// Includes left-to-right, right-to-left and top-to-bottom.
	TextFlowLinear TextFlowKind = iota
	// TextFlowPath is a text-on-path layout.
	TextFlowPath
)

// String returns the Rust name of the case.
func (k TextFlowKind) String() string {
	switch k {
	case TextFlowLinear:
		return "Linear"
	case TextFlowPath:
		return "Path"
	}
	return "TextFlowKind(?)"
}

// TextFlow is TextFlow in usvg src/tree/text.rs. It is a text chunk flow
// property.
//
// Port: only the case Path holds data, so the enum becomes a struct with a
// kind (D27). The zero value is TextFlowLinear.
type TextFlow struct {
	Kind TextFlowKind
	// Path is the text path of TextFlowPath. The text chunks share it, as
	// they share the Arc in upstream.
	Path *TextPath
}

// TextChunk is TextChunk in usvg src/tree/text.rs. It is a text chunk.
//
// Text alignment and BIDI reordering can only be done inside a text chunk.
type TextChunk struct {
	x        *float32
	y        *float32
	anchor   TextAnchor
	spans    []TextSpan
	textFlow TextFlow
	text     string
}

// X is TextChunk::x in usvg src/tree/text.rs. It returns an absolute X axis
// offset.
func (c *TextChunk) X() (float32, bool) {
	if c.x == nil {
		return 0, false
	}
	return *c.x, true
}

// Y is TextChunk::y in usvg src/tree/text.rs. It returns an absolute Y axis
// offset.
func (c *TextChunk) Y() (float32, bool) {
	if c.y == nil {
		return 0, false
	}
	return *c.y, true
}

// Anchor is TextChunk::anchor in usvg src/tree/text.rs. It returns a text
// anchor.
func (c *TextChunk) Anchor() TextAnchor {
	return c.anchor
}

// Spans is TextChunk::spans in usvg src/tree/text.rs. It returns a list of
// text chunk style spans.
func (c *TextChunk) Spans() []TextSpan {
	return c.spans
}

// TextFlow is TextChunk::text_flow in usvg src/tree/text.rs. It returns a
// text chunk flow.
func (c *TextChunk) TextFlow() TextFlow {
	return c.textFlow
}

// Text is TextChunk::text in usvg src/tree/text.rs. It returns a text chunk
// actual text.
func (c *TextChunk) Text() string {
	return c.text
}

// WritingMode is WritingMode in usvg src/tree/text.rs. It is a writing mode.
type WritingMode uint8

// The values of WritingMode.
const (
	WritingModeLeftToRight WritingMode = iota
	WritingModeTopToBottom
)

// String returns the Rust name of the value.
func (m WritingMode) String() string {
	switch m {
	case WritingModeLeftToRight:
		return "LeftToRight"
	case WritingModeTopToBottom:
		return "TopToBottom"
	}
	return "WritingMode(?)"
}

// Text is Text in usvg src/tree/text.rs. It is a text element.
//
// `text` element in SVG.
type Text struct {
	id                   string
	renderingMode        TextRendering
	dx                   []float32
	dy                   []float32
	rotate               []float32
	writingMode          WritingMode
	chunks               []TextChunk
	absTransform         Transform
	boundingBox          Rect
	absBoundingBox       Rect
	strokeBoundingBox    Rect
	absStrokeBoundingBox Rect
	flattened            *Group
	// TODO(phase 9): the field layouted of the feature text.
}

// ID is Text::id in usvg src/tree/text.rs. It returns the element's ID.
//
// Taken from the SVG itself.
// Isn't automatically generated.
// Can be empty.
func (t *Text) ID() string {
	return t.id
}

// RenderingMode is Text::rendering_mode in usvg src/tree/text.rs. It returns
// the rendering mode.
//
// `text-rendering` in SVG.
func (t *Text) RenderingMode() TextRendering {
	return t.renderingMode
}

// Dx is Text::dx in usvg src/tree/text.rs. It returns a relative X axis
// offsets.
//
// One offset for each Unicode codepoint. Aka `char` in Rust.
func (t *Text) Dx() []float32 {
	return t.dx
}

// Dy is Text::dy in usvg src/tree/text.rs. It returns a relative Y axis
// offsets.
//
// One offset for each Unicode codepoint. Aka `char` in Rust.
func (t *Text) Dy() []float32 {
	return t.dy
}

// Rotate is Text::rotate in usvg src/tree/text.rs. It returns a list of
// rotation angles.
//
// One angle for each Unicode codepoint. Aka `char` in Rust.
func (t *Text) Rotate() []float32 {
	return t.rotate
}

// WritingMode is Text::writing_mode in usvg src/tree/text.rs. It returns a
// writing mode.
func (t *Text) WritingMode() WritingMode {
	return t.writingMode
}

// Chunks is Text::chunks in usvg src/tree/text.rs. It returns a list of text
// chunks.
func (t *Text) Chunks() []TextChunk {
	return t.chunks
}

// AbsTransform is Text::abs_transform in usvg src/tree/text.rs. It returns
// the element's absolute transform.
//
// Contains all ancestors transforms including elements's transform.
//
// Note that this is not the relative transform present in SVG.
// The SVG one would be set only on groups.
func (t *Text) AbsTransform() Transform {
	return t.absTransform
}

// BoundingBox is Text::bounding_box in usvg src/tree/text.rs. It returns the
// element's text bounding box.
//
// Text bounding box is special in SVG and doesn't represent
// tight bounds of the element's content.
// You can find more about it
// [here](https://razrfalcon.github.io/notes-on-svg-parsing/text/bbox.html).
//
// `objectBoundingBox` in SVG terms. Meaning it isn't affected by parent transforms.
//
// Returns `None` when the `text` build feature was disabled.
// This is because we have to perform a text layout before calculating a bounding box.
func (t *Text) BoundingBox() Rect {
	return t.boundingBox
}

// AbsBoundingBox is Text::abs_bounding_box in usvg src/tree/text.rs. It
// returns the element's text bounding box in canvas coordinates.
//
// `userSpaceOnUse` in SVG terms.
func (t *Text) AbsBoundingBox() Rect {
	return t.absBoundingBox
}

// StrokeBoundingBox is Text::stroke_bounding_box in usvg src/tree/text.rs. It
// returns the element's object bounding box including stroke.
//
// Similar to `bounding_box`, but includes stroke.
//
// Will have the same value as `bounding_box` when path has no stroke.
func (t *Text) StrokeBoundingBox() Rect {
	return t.strokeBoundingBox
}

// AbsStrokeBoundingBox is Text::abs_stroke_bounding_box in usvg
// src/tree/text.rs. It returns the element's bounding box including stroke
// in canvas coordinates.
func (t *Text) AbsStrokeBoundingBox() Rect {
	return t.absStrokeBoundingBox
}

// Flattened is Text::flattened in usvg src/tree/text.rs. It returns the text
// converted into paths, ready to render.
//
// Note that this is only a
// "best-effort" attempt: The text will be converted into group/paths/image
// primitives, so that they can be rendered with the existing infrastructure.
// This process is in general lossless and should lead to correct output, with
// two notable exceptions:
//  1. For glyphs based on the `SVG` table, only glyphs that are pure SVG 1.1/2.0
//     are supported. Glyphs that make use of features in the OpenType specification
//     that are not part of the original SVG specification are not supported.
//  2. For glyphs based on the `COLR` table, there are a certain number of features
//     that are not (correctly) supported, such as conical
//     gradients, certain gradient transforms and some blend modes. But this shouldn't
//     cause any issues in 95% of the cases, as most of those are edge cases.
//     If the two above are not acceptable, then you will need to implement your own
//     glyph rendering logic based on the layouted glyphs (see the `layouted` method).
func (t *Text) Flattened() *Group {
	return t.flattened
}

// Subroots is Text::subroots in usvg src/tree/text.rs, which Node::subroots
// calls for a text.
func (t *Text) Subroots(f func(*Group)) {
	f(t.flattened)
}
