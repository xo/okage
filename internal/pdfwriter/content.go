// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"fmt"
	"math"
)

// This file ports pdf-writer src/content.rs, with Chunk::ext_graphics of
// src/chunk.rs.

// Content is Content in pdf-writer src/content.rs.
//
// A builder for a content stream.
//
// Each method of Content first finishes the Operation that is open, as Rust
// drops it at the end of its statement.
type Content struct {
	buf      Buf
	settings Settings
	qDepth   int
}

// Core methods.

// NewContent is Content::new in pdf-writer src/content.rs.
//
// Create a new content stream with the default settings and buffer capacity
// (currently 1 KB).
func NewContent() *Content {
	return NewContentWithSettings(DefaultSettings())
}

// NewContentWithSettings is Content::with_settings in pdf-writer
// src/content.rs.
//
// Create a new content stream with the given settings and the default buffer
// capacity (currently 1 KB).
func NewContentWithSettings(settings Settings) *Content {
	return NewContentWithSettingsAndCapacity(settings, 1204)
}

// NewContentWithCapacity is Content::with_capacity in pdf-writer
// src/content.rs.
//
// Create a new content stream with the default settings and the specified
// initial buffer capacity.
func NewContentWithCapacity(capacity int) *Content {
	return NewContentWithSettingsAndCapacity(DefaultSettings(), capacity)
}

// NewContentWithSettingsAndCapacity is Content::with_settings_and_capacity
// in pdf-writer src/content.rs.
//
// Create a new content stream with the given settings and the specified
// initial buffer capacity.
func NewContentWithSettingsAndCapacity(settings Settings, capacity int) *Content {
	return &Content{
		buf:      newBufWithCapacity(capacity),
		settings: settings,
		qDepth:   0,
	}
}

// Len is Content::len in pdf-writer src/content.rs.
//
// The number of bytes that were written so far.
func (c *Content) Len() int {
	return c.buf.Len()
}

// Reserve is Content::reserve in pdf-writer src/content.rs.
//
// Reserve an additional number of bytes in the buffer.
func (c *Content) Reserve(additional int) {
	c.buf.reserve(additional)
}

// AsBytes is Content::as_bytes in pdf-writer src/content.rs.
//
// The bytes already written so far.
func (c *Content) AsBytes() []byte {
	return c.buf.AsSlice()
}

// Op is Content::op in pdf-writer src/content.rs.
//
// Start writing an arbitrary operation.
func (c *Content) Op(operator string) *Operation {
	c.buf.finishFrom(0)
	return startOperation(&c.buf, operator, c.settings)
}

// Finish is Content::finish in pdf-writer src/content.rs. Upstream takes the
// Content by value. Here the Content gives its own buffer, and a later call
// on the Content writes into that buffer.
//
// Return the buffer of the content stream.
//
// The buffer is essentially a thin wrapper around two objects:
//
//   - A Limits object, which can optionally be used to keep track of data
//     such as the largest used integer or the longest string used in the
//     content streams, which is useful information for some export modes.
//   - The actual underlying data of the content stream, which can be written
//     to a chunk (and optionally apply a filter before doing so).
func (c *Content) Finish() *Buf {
	c.buf.finishFrom(0)
	if n := len(c.buf.inner); n > 0 && c.buf.inner[n-1] == '\n' {
		c.buf.inner = c.buf.inner[:n-1]
	}
	return &c.buf
}

// Operation is Operation in pdf-writer src/content.rs.
//
// Writer for an operation in a content stream.
//
// This struct is created by Content.Op.
type Operation struct {
	buf      *Buf
	op       string
	first    bool
	settings Settings

	level int
	done  bool
}

// startOperation is Operation::start in pdf-writer src/content.rs.
func startOperation(buf *Buf, op string, settings Settings) *Operation {
	o := &Operation{buf: buf, op: op, first: true, settings: settings}
	o.level = buf.openWriter(o)
	return o
}

// enter finishes the writers inside the operation, and panics if the
// operation is finished.
func (o *Operation) enter() {
	if o.done {
		panic("pdfwriter: the Operation is finished")
	}
	o.buf.finishFrom(o.level + 1)
}

// Operand is Operation::operand in pdf-writer src/content.rs.
//
// Write a primitive operand.
func (o *Operation) Operand(value Primitive) *Operation {
	o.Obj().Primitive(value)
	return o
}

// Operands is Operation::operands in pdf-writer src/content.rs.
//
// Write a sequence of primitive operands.
func (o *Operation) Operands(values ...Primitive) *Operation {
	for _, value := range values {
		o.Operand(value)
	}
	return o
}

// Obj is Operation::obj in pdf-writer src/content.rs.
//
// Start writing an arbitrary object operand.
func (o *Operation) Obj() Obj {
	o.enter()
	// In case we are writing the first object, we want a newline to
	// separate it from previous operations (looks nicer). Otherwise, a
	// space is sufficient.
	padByte := byte(' ')
	if o.first {
		padByte = '\n'
	}

	// Similarly to how chunks are handled, we always add padding when
	// pretty-writing is enabled, and only lazily add padding depending on
	// whether it's really necessary if not.
	needsPadding := true
	if o.settings.Pretty {
		if len(o.buf.inner) != 0 {
			o.buf.push(padByte)
		}

		needsPadding = false
	}

	o.first = false
	return directObj(o.buf, 0, o.settings, needsPadding)
}

// Finish is the trait Finish in pdf-writer src/object.rs for an Operation.
// It writes the operator, as Rust does when it drops the operation. The
// writers inside the operation finish first. Finish does nothing when the
// operation is already finished.
func (o *Operation) Finish() {
	if !o.done {
		o.buf.finishFrom(o.level)
	}
}

// close is the impl Drop for Operation in pdf-writer src/content.rs.
func (o *Operation) close() {
	o.done = true
	padByte := byte(' ')
	if o.first {
		padByte = '\n'
	}

	// For example, in case we previously wrote a BT operator and then a
	// `[]` operand in the next operation, we don't need to pad them.
	inner := o.buf.inner
	if (o.settings.Pretty || (len(inner) > 0 && !isDelimiterCharacter(inner[len(inner)-1]))) &&
		len(inner) != 0 {
		o.buf.push(padByte)
	}

	o.buf.extendString(o.op)
}

// General graphics state.

// SetLineWidth is Content::set_line_width in pdf-writer src/content.rs.
//
// w: Set the stroke line width.
//
// Panics if width is negative.
func (c *Content) SetLineWidth(width float32) *Content {
	if !(width >= 0.0) {
		panic("line width must be positive")
	}
	c.Op("w").Operand(Real(width))
	return c
}

// SetLineCap is Content::set_line_cap in pdf-writer src/content.rs.
//
// J: Set the line cap style.
func (c *Content) SetLineCap(lineCap LineCapStyle) *Content {
	c.Op("J").Operand(Int(lineCap.toInt()))
	return c
}

// SetLineJoin is Content::set_line_join in pdf-writer src/content.rs.
//
// j: Set the line join style.
func (c *Content) SetLineJoin(join LineJoinStyle) *Content {
	c.Op("j").Operand(Int(join.toInt()))
	return c
}

// SetMiterLimit is Content::set_miter_limit in pdf-writer src/content.rs.
//
// M: Set the miter limit.
func (c *Content) SetMiterLimit(limit float32) *Content {
	c.Op("M").Operand(Real(limit))
	return c
}

// SetDashPattern is Content::set_dash_pattern in pdf-writer src/content.rs.
//
// d: Set the line dash pattern.
func (c *Content) SetDashPattern(array []float32, phase float32) *Content {
	op := c.Op("d")
	a := op.Obj().Array()
	for _, v := range array {
		a.Item(Real(v))
	}
	op.Operand(Real(phase))
	op.Finish()
	return c
}

// SetRenderingIntent is Content::set_rendering_intent in pdf-writer
// src/content.rs.
//
// ri: Set the color rendering intent to the parameter. PDF 1.1+.
func (c *Content) SetRenderingIntent(intent RenderingIntent) *Content {
	c.Op("ri").Operand(intent.toName())
	return c
}

// SetFlatness is Content::set_flatness in pdf-writer src/content.rs.
//
// i: Set the flatness tolerance in device pixels.
//
// Panics if tolerance is negative or larger than 100.
func (c *Content) SetFlatness(tolerance int32) *Content {
	if tolerance < 0 || tolerance > 100 {
		panic("flatness tolerance must be between 0 and 100")
	}
	c.Op("i").Operand(Int(tolerance))
	return c
}

// SetParameters is Content::set_parameters in pdf-writer src/content.rs.
//
// gs: Set the parameters from an ExtGState dictionary. PDF 1.2+.
func (c *Content) SetParameters(dict Name) *Content {
	c.Op("gs").Operand(dict)
	return c
}

// LineCapStyle is LineCapStyle in pdf-writer src/content.rs.
//
// How to terminate lines.
type LineCapStyle uint8

// The values of LineCapStyle.
const (
	// LineCapStyleButtCap squares the line of at the endpoints of the path.
	LineCapStyleButtCap LineCapStyle = iota
	// LineCapStyleRoundCap rounds the line off at its end with a
	// semicircular arc as wide as the stroke.
	LineCapStyleRoundCap
	// LineCapStyleProjectingSquareCap ends the line with a square cap that
	// protrudes by half the width of the stroke.
	LineCapStyleProjectingSquareCap
)

// lineCapStyleNames holds the Rust name of each value of LineCapStyle.
var lineCapStyleNames = [...]string{
	LineCapStyleButtCap:             "ButtCap",
	LineCapStyleRoundCap:            "RoundCap",
	LineCapStyleProjectingSquareCap: "ProjectingSquareCap",
}

// String returns the Rust name of the line cap style.
func (s LineCapStyle) String() string {
	if int(s) < len(lineCapStyleNames) {
		return lineCapStyleNames[s]
	}
	return "LineCapStyle(?)"
}

// toInt is LineCapStyle::to_int in pdf-writer src/content.rs.
func (s LineCapStyle) toInt() int32 {
	switch s {
	case LineCapStyleButtCap:
		return 0
	case LineCapStyleRoundCap:
		return 1
	case LineCapStyleProjectingSquareCap:
		return 2
	}
	panic(fmt.Sprintf("pdfwriter: the line cap style %d is not known", uint8(s)))
}

// LineJoinStyle is LineJoinStyle in pdf-writer src/content.rs.
//
// How to join lines at corners.
type LineJoinStyle uint8

// The values of LineJoinStyle.
const (
	// LineJoinStyleMiterJoin joins the lines with a sharp corner where the
	// outsides of the lines intersect.
	LineJoinStyleMiterJoin LineJoinStyle = iota
	// LineJoinStyleRoundJoin joins the lines with a smooth circular segment.
	LineJoinStyleRoundJoin
	// LineJoinStyleBevelJoin ends both lines with butt caps and joins them
	// with a triangle.
	LineJoinStyleBevelJoin
)

// lineJoinStyleNames holds the Rust name of each value of LineJoinStyle.
var lineJoinStyleNames = [...]string{
	LineJoinStyleMiterJoin: "MiterJoin",
	LineJoinStyleRoundJoin: "RoundJoin",
	LineJoinStyleBevelJoin: "BevelJoin",
}

// String returns the Rust name of the line join style.
func (s LineJoinStyle) String() string {
	if int(s) < len(lineJoinStyleNames) {
		return lineJoinStyleNames[s]
	}
	return "LineJoinStyle(?)"
}

// toInt is LineJoinStyle::to_int in pdf-writer src/content.rs.
func (s LineJoinStyle) toInt() int32 {
	switch s {
	case LineJoinStyleMiterJoin:
		return 0
	case LineJoinStyleRoundJoin:
		return 1
	case LineJoinStyleBevelJoin:
		return 2
	}
	panic(fmt.Sprintf("pdfwriter: the line join style %d is not known", uint8(s)))
}

// RenderingIntent is RenderingIntent in pdf-writer src/content.rs. The zero
// value is RenderingIntentAbsoluteColorimetric. The default of upstream is
// RenderingIntentRelativeColorimetric.
//
// How the output device should aim to render colors.
type RenderingIntent uint8

// The values of RenderingIntent.
const (
	// RenderingIntentAbsoluteColorimetric only considers the light source,
	// not the output's white point.
	RenderingIntentAbsoluteColorimetric RenderingIntent = iota
	// RenderingIntentRelativeColorimetric considers both the light source
	// and the output's white point.
	RenderingIntentRelativeColorimetric
	// RenderingIntentSaturation preserves saturation.
	RenderingIntentSaturation
	// RenderingIntentPerceptual preserves a pleasing visual appearance.
	RenderingIntentPerceptual
)

// String returns the PDF name of the rendering intent, without the slash.
func (i RenderingIntent) String() string {
	return string(i.toName())
}

// toName is RenderingIntent::to_name in pdf-writer src/content.rs.
func (i RenderingIntent) toName() Name {
	switch i {
	case RenderingIntentAbsoluteColorimetric:
		return Name("AbsoluteColorimetric")
	case RenderingIntentRelativeColorimetric:
		return Name("RelativeColorimetric")
	case RenderingIntentSaturation:
		return Name("Saturation")
	case RenderingIntentPerceptual:
		return Name("Perceptual")
	}
	panic(fmt.Sprintf("pdfwriter: the rendering intent %d is not known", uint8(i)))
}

// Special graphics state.

// SaveState is Content::save_state in pdf-writer src/content.rs.
//
// q: Save the graphics state on the stack.
func (c *Content) SaveState() *Content {
	c.Op("q")
	if c.qDepth < math.MaxInt {
		c.qDepth++
	}
	return c
}

// RestoreState is Content::restore_state in pdf-writer src/content.rs.
//
// Q: Restore the graphics state from the stack.
func (c *Content) RestoreState() *Content {
	c.Op("Q")
	if c.qDepth > 0 {
		c.qDepth--
	}
	return c
}

// StateNestingDepth is Content::state_nesting_depth in pdf-writer
// src/content.rs.
//
// The current q nesting depth.
func (c *Content) StateNestingDepth() int {
	return c.qDepth
}

// Transform is Content::transform in pdf-writer src/content.rs.
//
// cm: Pre-concatenate the matrix with the current transformation matrix.
func (c *Content) Transform(matrix [6]float32) *Content {
	op := c.Op("cm")
	for _, v := range matrix {
		op.Operand(Real(v))
	}
	return c
}

// Path construction.

// MoveTo is Content::move_to in pdf-writer src/content.rs.
//
// m: Begin a new subpath at (x, y).
func (c *Content) MoveTo(x, y float32) *Content {
	c.Op("m").Operands(Real(x), Real(y))
	return c
}

// LineTo is Content::line_to in pdf-writer src/content.rs.
//
// l: Append a straight line to (x, y).
func (c *Content) LineTo(x, y float32) *Content {
	c.Op("l").Operands(Real(x), Real(y))
	return c
}

// CubicTo is Content::cubic_to in pdf-writer src/content.rs.
//
// c: Append a cubic Bézier segment to (x3, y3) with (x1, y1), (x2, y2) as
// control points.
func (c *Content) CubicTo(x1, y1, x2, y2, x3, y3 float32) *Content {
	c.Op("c").Operands(Real(x1), Real(y1), Real(x2), Real(y2), Real(x3), Real(y3))
	return c
}

// CubicToInitial is Content::cubic_to_initial in pdf-writer src/content.rs.
//
// v: Append a cubic Bézier segment to (x3, y3) with (x2, y2) as control
// point.
func (c *Content) CubicToInitial(x2, y2, x3, y3 float32) *Content {
	c.Op("v").Operands(Real(x2), Real(y2), Real(x3), Real(y3))
	return c
}

// CubicToFinal is Content::cubic_to_final in pdf-writer src/content.rs.
//
// y: Append a cubic Bézier segment to (x3, y3) with (x1, y1) as control
// point.
func (c *Content) CubicToFinal(x1, y1, x3, y3 float32) *Content {
	c.Op("y").Operands(Real(x1), Real(y1), Real(x3), Real(y3))
	return c
}

// ClosePath is Content::close_path in pdf-writer src/content.rs.
//
// h: Close the current subpath with a straight line.
func (c *Content) ClosePath() *Content {
	c.Op("h")
	return c
}

// Rect is Content::rect in pdf-writer src/content.rs.
//
// re: Append a rectangle to the current path.
func (c *Content) Rect(x, y, width, height float32) *Content {
	c.Op("re").Operands(Real(x), Real(y), Real(width), Real(height))
	return c
}

// Path painting.

// Stroke is Content::stroke in pdf-writer src/content.rs.
//
// S: Stroke the current path.
func (c *Content) Stroke() *Content {
	c.Op("S")
	return c
}

// CloseAndStroke is Content::close_and_stroke in pdf-writer src/content.rs.
//
// s: Close the current path and then stroke it.
func (c *Content) CloseAndStroke() *Content {
	c.Op("s")
	return c
}

// FillNonzero is Content::fill_nonzero in pdf-writer src/content.rs.
//
// f: Fill the current path using the nonzero winding number rule.
func (c *Content) FillNonzero() *Content {
	c.Op("f")
	return c
}

// FillEvenOdd is Content::fill_even_odd in pdf-writer src/content.rs.
//
// f*: Fill the current path using the even-odd rule.
func (c *Content) FillEvenOdd() *Content {
	c.Op("f*")
	return c
}

// FillNonzeroAndStroke is Content::fill_nonzero_and_stroke in pdf-writer
// src/content.rs.
//
// B: Fill the current path using the nonzero winding number rule and then
// stroke it.
func (c *Content) FillNonzeroAndStroke() *Content {
	c.Op("B")
	return c
}

// FillEvenOddAndStroke is Content::fill_even_odd_and_stroke in pdf-writer
// src/content.rs.
//
// B*: Fill the current path using the even-odd rule and then stroke it.
func (c *Content) FillEvenOddAndStroke() *Content {
	c.Op("B*")
	return c
}

// CloseFillNonzeroAndStroke is Content::close_fill_nonzero_and_stroke in
// pdf-writer src/content.rs.
//
// b: Close the current path, fill it using the nonzero winding number rule
// and then stroke it.
func (c *Content) CloseFillNonzeroAndStroke() *Content {
	c.Op("b")
	return c
}

// CloseFillEvenOddAndStroke is Content::close_fill_even_odd_and_stroke in
// pdf-writer src/content.rs.
//
// b*: Close the current path, fill it using the even-odd rule and then
// stroke it.
func (c *Content) CloseFillEvenOddAndStroke() *Content {
	c.Op("b*")
	return c
}

// EndPath is Content::end_path in pdf-writer src/content.rs.
//
// n: End the current path without filling or stroking it.
//
// This is primarily used for clipping paths.
func (c *Content) EndPath() *Content {
	c.Op("n")
	return c
}

// Clipping paths.

// ClipNonzero is Content::clip_nonzero in pdf-writer src/content.rs.
//
// W: Intersect the current clipping path with the current path using the
// nonzero winding number rule.
func (c *Content) ClipNonzero() *Content {
	c.Op("W")
	return c
}

// ClipEvenOdd is Content::clip_even_odd in pdf-writer src/content.rs.
//
// W*: Intersect the current clipping path with the current path using the
// even-odd rule.
func (c *Content) ClipEvenOdd() *Content {
	c.Op("W*")
	return c
}

// Text objects.

// BeginText is Content::begin_text in pdf-writer src/content.rs.
//
// BT: Begin a text object.
func (c *Content) BeginText() *Content {
	c.Op("BT")
	return c
}

// EndText is Content::end_text in pdf-writer src/content.rs.
//
// ET: End a text object.
func (c *Content) EndText() *Content {
	c.Op("ET")
	return c
}

// Text state.

// SetCharSpacing is Content::set_char_spacing in pdf-writer src/content.rs.
//
// Tc: Set the character spacing.
func (c *Content) SetCharSpacing(spacing float32) *Content {
	c.Op("Tc").Operand(Real(spacing))
	return c
}

// SetWordSpacing is Content::set_word_spacing in pdf-writer src/content.rs.
//
// Tw: Set the word spacing.
func (c *Content) SetWordSpacing(spacing float32) *Content {
	c.Op("Tw").Operand(Real(spacing))
	return c
}

// SetHorizontalScaling is Content::set_horizontal_scaling in pdf-writer
// src/content.rs.
//
// Tz: Set the horizontal scaling.
func (c *Content) SetHorizontalScaling(scaling float32) *Content {
	c.Op("Tz").Operand(Real(scaling))
	return c
}

// SetLeading is Content::set_leading in pdf-writer src/content.rs.
//
// TL: Set the leading.
func (c *Content) SetLeading(leading float32) *Content {
	c.Op("TL").Operand(Real(leading))
	return c
}

// SetFont is Content::set_font in pdf-writer src/content.rs.
//
// Tf: Set font and font size.
func (c *Content) SetFont(font Name, size float32) *Content {
	c.Op("Tf").Operand(font).Operand(Real(size))
	return c
}

// SetTextRenderingMode is Content::set_text_rendering_mode in pdf-writer
// src/content.rs.
//
// Tr: Set the text rendering mode.
func (c *Content) SetTextRenderingMode(mode TextRenderingMode) *Content {
	c.Op("Tr").Operand(Int(mode.toInt()))
	return c
}

// SetRise is Content::set_rise in pdf-writer src/content.rs.
//
// Ts: Set the rise.
func (c *Content) SetRise(rise float32) *Content {
	c.Op("Ts").Operand(Real(rise))
	return c
}

// TextRenderingMode is TextRenderingMode in pdf-writer src/content.rs. The
// zero value is TextRenderingModeFill, the default of upstream.
//
// How to render text.
type TextRenderingMode uint8

// The values of TextRenderingMode.
const (
	// TextRenderingModeFill just fills the text.
	TextRenderingModeFill TextRenderingMode = iota
	// TextRenderingModeStroke just strokes the text.
	TextRenderingModeStroke
	// TextRenderingModeFillStroke first fills and then strokes the text.
	TextRenderingModeFillStroke
	// TextRenderingModeInvisible does not fill and does not stroke the text.
	TextRenderingModeInvisible
	// TextRenderingModeFillClip fills the text, then applies the text
	// outlines to the current clipping path.
	TextRenderingModeFillClip
	// TextRenderingModeStrokeClip strokes the text, then applies the text
	// outlines to the current clipping path.
	TextRenderingModeStrokeClip
	// TextRenderingModeFillStrokeClip first fills, then strokes the text and
	// finally applies the text outlines to the current clipping path.
	TextRenderingModeFillStrokeClip
	// TextRenderingModeClip applies the text outlines to the current
	// clipping path.
	TextRenderingModeClip
)

// textRenderingModeNames holds the Rust name of each value of
// TextRenderingMode.
var textRenderingModeNames = [...]string{
	TextRenderingModeFill:           "Fill",
	TextRenderingModeStroke:         "Stroke",
	TextRenderingModeFillStroke:     "FillStroke",
	TextRenderingModeInvisible:      "Invisible",
	TextRenderingModeFillClip:       "FillClip",
	TextRenderingModeStrokeClip:     "StrokeClip",
	TextRenderingModeFillStrokeClip: "FillStrokeClip",
	TextRenderingModeClip:           "Clip",
}

// String returns the Rust name of the text rendering mode.
func (m TextRenderingMode) String() string {
	if int(m) < len(textRenderingModeNames) {
		return textRenderingModeNames[m]
	}
	return "TextRenderingMode(?)"
}

// toInt is TextRenderingMode::to_int in pdf-writer src/content.rs.
func (m TextRenderingMode) toInt() int32 {
	if int(m) < len(textRenderingModeNames) {
		return int32(m)
	}
	panic(fmt.Sprintf("pdfwriter: the text rendering mode %d is not known", uint8(m)))
}

// Text positioning.

// NextLine is Content::next_line in pdf-writer src/content.rs.
//
// Td: Move to the start of the next line.
func (c *Content) NextLine(x, y float32) *Content {
	c.Op("Td").Operands(Real(x), Real(y))
	return c
}

// NextLineAndSetLeading is Content::next_line_and_set_leading in pdf-writer
// src/content.rs.
//
// TD: Move to the start of the next line and set the text state's leading
// parameter to -y.
func (c *Content) NextLineAndSetLeading(x, y float32) *Content {
	c.Op("TD").Operands(Real(x), Real(y))
	return c
}

// SetTextMatrix is Content::set_text_matrix in pdf-writer src/content.rs.
//
// Tm: Set the text matrix.
func (c *Content) SetTextMatrix(matrix [6]float32) *Content {
	op := c.Op("Tm")
	for _, v := range matrix {
		op.Operand(Real(v))
	}
	return c
}

// NextLineUsingLeading is Content::next_line_using_leading in pdf-writer
// src/content.rs.
//
// T*: Move to the start of the next line, determining the vertical offset
// through the text state's leading parameter.
func (c *Content) NextLineUsingLeading() *Content {
	c.Op("T*")
	return c
}

// Text showing.

// Show is Content::show in pdf-writer src/content.rs.
//
// Tj: Show text.
//
// The encoding of the text depends on the font.
func (c *Content) Show(text Str) *Content {
	c.Op("Tj").Operand(text)
	return c
}

// NextLineShow is Content::next_line_show in pdf-writer src/content.rs.
//
// ': Move to the next line and show text.
func (c *Content) NextLineShow(text Str) *Content {
	c.Op("'").Operand(text)
	return c
}

// NextLineShowAndSetWordAndCharSpacing is
// Content::next_line_show_and_set_word_and_char_spacing in pdf-writer
// src/content.rs.
//
// ": Move to the next line, show text and set the text state's word and
// character spacing.
func (c *Content) NextLineShowAndSetWordAndCharSpacing(wordSpacing, charSpacing float32, text Str) *Content {
	c.Op("\"").Operands(Real(wordSpacing), Real(charSpacing)).Operand(text)
	return c
}

// ShowPositioned is Content::show_positioned in pdf-writer src/content.rs.
//
// TJ: Start showing text with individual glyph positioning.
func (c *Content) ShowPositioned() *ShowPositioned {
	return startShowPositioned(c.Op("TJ"))
}

// ShowPositioned is ShowPositioned in pdf-writer src/content.rs.
//
// Writer for an individual glyph positioning operation.
//
// This struct is created by Content.ShowPositioned.
type ShowPositioned struct {
	*Operation
}

// startShowPositioned is ShowPositioned::start in pdf-writer src/content.rs.
func startShowPositioned(op *Operation) *ShowPositioned {
	return &ShowPositioned{Operation: op}
}

// Items is ShowPositioned::items in pdf-writer src/content.rs.
//
// Start writing the array of strings and adjustments. Required.
func (s *ShowPositioned) Items() *PositionedItems {
	return startPositionedItems(s.Obj())
}

// PositionedItems is PositionedItems in pdf-writer src/content.rs.
//
// Writer for a positioned items array.
//
// This struct is created by ShowPositioned.Items.
type PositionedItems struct {
	*Array
}

// startPositionedItems is PositionedItems::start in pdf-writer
// src/content.rs.
func startPositionedItems(obj Obj) *PositionedItems {
	return &PositionedItems{Array: obj.Array()}
}

// Show is PositionedItems::show in pdf-writer src/content.rs.
//
// Show a continuous string without adjustments.
//
// The encoding of the text depends on the font.
func (p *PositionedItems) Show(text Str) *PositionedItems {
	p.Item(text)
	return p
}

// Adjust is PositionedItems::adjust in pdf-writer src/content.rs.
//
// Specify an adjustment between two glyphs.
//
// The amount is specified in thousands of units of text space and is
// subtracted from the current writing-mode dependent coordinate.
func (p *PositionedItems) Adjust(amount float32) *PositionedItems {
	p.Item(Real(amount))
	return p
}

// Type 3 fonts.
//
// These operators are only allowed in Type 3 CharProcs.

// StartColorGlyph is Content::start_color_glyph in pdf-writer
// src/content.rs.
//
// d0: Starts a Type 3 glyph that contains color information.
//
//   - wx defines the glyph's width
//   - wy is set to 0.0 automatically
func (c *Content) StartColorGlyph(wx float32) *Content {
	c.Op("d0").Operands(Real(wx), Real(0.0))
	return c
}

// StartShapeGlyph is Content::start_shape_glyph in pdf-writer
// src/content.rs.
//
// d1: Starts a Type 3 glyph that contains only shape information.
//
//   - wx defines the glyph's width
//   - wy is set to 0.0 automatically
//   - llX and llY define the lower-left corner of the glyph bounding box
//   - urX and urY define the upper-right corner of the glyph bounding box
func (c *Content) StartShapeGlyph(wx, llX, llY, urX, urY float32) *Content {
	c.Op("d1").Operands(Real(wx), Real(0.0), Real(llX), Real(llY), Real(urX), Real(urY))
	return c
}

// Color.

// SetStrokeColorSpace is Content::set_stroke_color_space in pdf-writer
// src/content.rs. Upstream takes any value that converts into a
// ColorSpaceOperand. ColorSpaceOperandFromName converts a Name.
//
// CS: Set the stroke color space to the parameter. PDF 1.1+.
//
// The parameter must be the name of a parameter-less color space or of a
// color space dictionary within the current resource dictionary.
func (c *Content) SetStrokeColorSpace(space ColorSpaceOperand) *Content {
	c.Op("CS").Operand(space.toName())
	return c
}

// SetFillColorSpace is Content::set_fill_color_space in pdf-writer
// src/content.rs, as SetStrokeColorSpace is.
//
// cs: Set the fill color space to the parameter. PDF 1.1+.
//
// The parameter must be the name of a parameter-less color space or of a
// color space dictionary within the current resource dictionary.
func (c *Content) SetFillColorSpace(space ColorSpaceOperand) *Content {
	c.Op("cs").Operand(space.toName())
	return c
}

// SetStrokeColor is Content::set_stroke_color in pdf-writer src/content.rs.
//
// SCN: Set the stroke color to the parameter within the current color space.
// PDF 1.2+.
func (c *Content) SetStrokeColor(color ...float32) *Content {
	op := c.Op("SCN")
	for _, v := range color {
		op.Operand(Real(v))
	}
	return c
}

// SetStrokePattern is Content::set_stroke_pattern in pdf-writer
// src/content.rs.
//
// SCN: Set the stroke pattern. PDF 1.2+.
//
// The name parameter is the name of a pattern. If this is an uncolored
// pattern, a tint color in the current Pattern base color space must be
// given, otherwise, the color iterator shall remain empty.
func (c *Content) SetStrokePattern(tint []float32, name Name) *Content {
	op := c.Op("SCN")
	for _, v := range tint {
		op.Operand(Real(v))
	}
	op.Operand(name)
	return c
}

// SetFillColor is Content::set_fill_color in pdf-writer src/content.rs.
//
// scn: Set the fill color to the parameter within the current color space.
// PDF 1.2+.
func (c *Content) SetFillColor(color ...float32) *Content {
	op := c.Op("scn")
	for _, v := range color {
		op.Operand(Real(v))
	}
	return c
}

// SetFillPattern is Content::set_fill_pattern in pdf-writer src/content.rs.
//
// scn: Set the fill pattern. PDF 1.2+.
//
// The name parameter is the name of a pattern. If this is an uncolored
// pattern, a tint color in the current Pattern base color space must be
// given, otherwise, the color iterator shall remain empty.
func (c *Content) SetFillPattern(tint []float32, name Name) *Content {
	op := c.Op("scn")
	for _, v := range tint {
		op.Operand(Real(v))
	}
	op.Operand(name)
	return c
}

// SetStrokeGray is Content::set_stroke_gray in pdf-writer src/content.rs.
//
// G: Set the stroke color to the parameter and the color space to
// DeviceGray.
func (c *Content) SetStrokeGray(gray float32) *Content {
	c.Op("G").Operand(Real(gray))
	return c
}

// SetFillGray is Content::set_fill_gray in pdf-writer src/content.rs.
//
// g: Set the fill color to the parameter and the color space to DeviceGray.
func (c *Content) SetFillGray(gray float32) *Content {
	c.Op("g").Operand(Real(gray))
	return c
}

// SetStrokeRGB is Content::set_stroke_rgb in pdf-writer src/content.rs.
//
// RG: Set the stroke color to the parameter and the color space to
// DeviceRGB.
func (c *Content) SetStrokeRGB(r, g, b float32) *Content {
	c.Op("RG").Operands(Real(r), Real(g), Real(b))
	return c
}

// SetFillRGB is Content::set_fill_rgb in pdf-writer src/content.rs.
//
// rg: Set the fill color to the parameter and the color space to DeviceRGB.
func (c *Content) SetFillRGB(r, g, b float32) *Content {
	c.Op("rg").Operands(Real(r), Real(g), Real(b))
	return c
}

// SetStrokeCMYK is Content::set_stroke_cmyk in pdf-writer src/content.rs.
//
// K: Set the stroke color to the parameter and the color space to
// DeviceCMYK.
func (c *Content) SetStrokeCMYK(cyan, magenta, yellow, black float32) *Content {
	c.Op("K").Operands(Real(cyan), Real(magenta), Real(yellow), Real(black))
	return c
}

// SetFillCMYK is Content::set_fill_cmyk in pdf-writer src/content.rs.
//
// k: Set the fill color to the parameter and the color space to DeviceCMYK.
func (c *Content) SetFillCMYK(cyan, magenta, yellow, black float32) *Content {
	c.Op("k").Operands(Real(cyan), Real(magenta), Real(yellow), Real(black))
	return c
}

// ColorSpaceOperand is ColorSpaceOperand in pdf-writer src/content.rs. It is
// a struct with the case in Kind, and the name of the case Named in Name
// (D27). The zero value is the case DeviceGray.
//
// A color space operand to the CS or cs operator.
type ColorSpaceOperand struct {
	Kind ColorSpaceOperandKind
	Name Name
}

// ColorSpaceOperandKind is the case of a ColorSpaceOperand.
type ColorSpaceOperandKind uint8

// The cases of ColorSpaceOperand.
const (
	// ColorSpaceOperandDeviceGray is the predefined, parameterless gray
	// device-dependent color space. Needs no further entries in the resource
	// dictionary. This is not a normed color space, meaning the exact look
	// depends on the device.
	//
	// Writing cs with this is equivalent to writing g.
	ColorSpaceOperandDeviceGray ColorSpaceOperandKind = iota
	// ColorSpaceOperandDeviceRGB is the predefined, parameterless RGB
	// device-dependent color space. Needs no further entries in the resource
	// dictionary.
	//
	// Writing cs with this is equivalent to writing rg.
	ColorSpaceOperandDeviceRGB
	// ColorSpaceOperandDeviceCMYK is the predefined, parameterless CMYK
	// device-dependent color space. Needs no further entries in the resource
	// dictionary.
	//
	// Writing cs with this is equivalent to writing k.
	ColorSpaceOperandDeviceCMYK
	// ColorSpaceOperandPattern is a pattern with color defined by the
	// pattern itself.
	//
	// When writing a cs operation with this, you must also write an scn
	// operation with a name pointing to an entry in the current resource
	// dictionary's pattern dictionary.
	ColorSpaceOperandPattern
	// ColorSpaceOperandNamed is a named color space defined in the current
	// resource dictionary's color space dictionary.
	//
	// When this points to a pattern color space, You must also write an scn
	// operation with a name pointing to an entry in the current resource
	// dictionary's pattern dictionary. The color will be taken from the tint
	// passed to SCN and not the pattern itself.
	ColorSpaceOperandNamed
)

// colorSpaceOperandKindNames holds the Rust name of each case of
// ColorSpaceOperand.
var colorSpaceOperandKindNames = [...]string{
	ColorSpaceOperandDeviceGray: "DeviceGray",
	ColorSpaceOperandDeviceRGB:  "DeviceRgb",
	ColorSpaceOperandDeviceCMYK: "DeviceCmyk",
	ColorSpaceOperandPattern:    "Pattern",
	ColorSpaceOperandNamed:      "Named",
}

// String returns the Rust name of the case.
func (k ColorSpaceOperandKind) String() string {
	if int(k) < len(colorSpaceOperandKindNames) {
		return colorSpaceOperandKindNames[k]
	}
	return "ColorSpaceOperandKind(?)"
}

// ColorSpaceOperandFromName is the impl From<Name> for ColorSpaceOperand in
// pdf-writer src/content.rs. It gives the case Named.
func ColorSpaceOperandFromName(name Name) ColorSpaceOperand {
	return ColorSpaceOperand{Kind: ColorSpaceOperandNamed, Name: name}
}

// toName is ColorSpaceOperand::to_name in pdf-writer src/content.rs.
func (o ColorSpaceOperand) toName() Name {
	switch o.Kind {
	case ColorSpaceOperandDeviceGray:
		return Name("DeviceGray")
	case ColorSpaceOperandDeviceRGB:
		return Name("DeviceRGB")
	case ColorSpaceOperandDeviceCMYK:
		return Name("DeviceCMYK")
	case ColorSpaceOperandPattern:
		return Name("Pattern")
	case ColorSpaceOperandNamed:
		return o.Name
	}
	panic(fmt.Sprintf("pdfwriter: the color space operand %d is not known", uint8(o.Kind)))
}

// Shading patterns.

// Shading is Content::shading in pdf-writer src/content.rs.
//
// sh: Fill the whole drawing area with the specified shading.
func (c *Content) Shading(shading Name) *Content {
	c.Op("sh").Operand(shading)
	return c
}

// XObjects.

// XObject is Content::x_object in pdf-writer src/content.rs.
//
// Do: Write an external object.
func (c *Content) XObject(name Name) *Content {
	c.Op("Do").Operand(name)
	return c
}

// Marked Content.

// MarkedContentPoint is Content::marked_content_point in pdf-writer
// src/content.rs.
//
// MP: Write a marked-content point. PDF 1.2+.
func (c *Content) MarkedContentPoint(tag Name) *Content {
	c.Op("MP").Operand(tag)
	return c
}

// MarkedContentPointWithProperties is
// Content::marked_content_point_with_properties in pdf-writer
// src/content.rs.
//
// DP: Start writing a marked-content point operation. PDF 1.2+.
func (c *Content) MarkedContentPointWithProperties(tag Name) *MarkContent {
	op := c.Op("DP")
	op.Operand(tag)
	return startMarkContent(op)
}

// BeginMarkedContent is Content::begin_marked_content in pdf-writer
// src/content.rs.
//
// BMC: Begin a marked-content sequence. PDF 1.2+.
func (c *Content) BeginMarkedContent(tag Name) *Content {
	c.Op("BMC").Operand(tag)
	return c
}

// BeginMarkedContentWithProperties is
// Content::begin_marked_content_with_properties in pdf-writer
// src/content.rs.
//
// BDC: Start writing a "begin marked content" operation. PDF 1.2+.
func (c *Content) BeginMarkedContentWithProperties(tag Name) *MarkContent {
	op := c.Op("BDC")
	op.Operand(tag)
	return startMarkContent(op)
}

// EndMarkedContent is Content::end_marked_content in pdf-writer
// src/content.rs.
//
// EMC: End a marked-content sequence. PDF 1.2+.
func (c *Content) EndMarkedContent() *Content {
	c.Op("EMC")
	return c
}

// MarkContent is MarkContent in pdf-writer src/content.rs.
//
// Writer for a begin marked content operation. PDF 1.3+.
type MarkContent struct {
	*Operation
}

// startMarkContent is MarkContent::start in pdf-writer src/content.rs.
func startMarkContent(op *Operation) *MarkContent {
	return &MarkContent{Operation: op}
}

// Properties is MarkContent::properties in pdf-writer src/content.rs.
//
// Start writing this marked content's property list. Mutually exclusive
// with PropertiesNamed.
func (m *MarkContent) Properties() *PropertyList {
	return Start[PropertyList](m.Obj())
}

// PropertiesNamed is MarkContent::properties_named in pdf-writer
// src/content.rs. Upstream takes the MarkContent by value, so it finishes
// the operation.
//
// Reference a property list from the Resource dictionary. These property
// lists can be written using the Resources.Properties method. Mutually
// exclusive with Properties.
func (m *MarkContent) PropertiesNamed(name Name) {
	m.Operand(name)
	m.Finish()
}

// PropertyList is PropertyList in pdf-writer src/content.rs.
//
// Writer for property list dictionary. Can be used as a generic dictionary.
// PDF 1.3+.
type PropertyList struct {
	*Dict
}

// start is the macro writer! for PropertyList in pdf-writer src/content.rs.
func (p *PropertyList) start(obj Obj) {
	p.Dict = obj.Dict()
}

// Identify is PropertyList::identify in pdf-writer src/content.rs.
//
// Write the /MCID marked content identifier.
func (p *PropertyList) Identify(identifier int32) *PropertyList {
	p.Pair(Name("MCID"), Int(identifier))
	return p
}

// ActualText is PropertyList::actual_text in pdf-writer src/content.rs.
//
// Write the /ActualText attribute to indicate the text replacement of this
// marked content sequence. PDF 1.5+.
func (p *PropertyList) ActualText(text TextStrLike) *PropertyList {
	p.Pair(Name("ActualText"), text)
	return p
}

// Artifact is PropertyList::artifact in pdf-writer src/content.rs.
//
// Start writing artifact property list. The tag of the marked content
// operation must have been /Artifact. PDF 1.4+.
func (p *PropertyList) Artifact() *Artifact {
	return startArtifactWithDict(p.Dict)
}

// Artifact is Artifact in pdf-writer src/content.rs.
//
// Writer for an actifact property list dictionary. PDF 1.4+.
//
// Required for marking up pagination artifacts in some PDF/A profiles.
type Artifact struct {
	*Dict
}

// start is the macro writer! for Artifact in pdf-writer src/content.rs.
func (a *Artifact) start(obj Obj) {
	a.Dict = obj.Dict()
}

// startArtifactWithDict is Artifact::start_with_dict in pdf-writer
// src/content.rs.
func startArtifactWithDict(dict *Dict) *Artifact {
	return &Artifact{Dict: dict}
}

// Kind is Artifact::kind in pdf-writer src/content.rs.
//
// Write the /Type entry to set the type of artifact. Specific to artifacts.
// PDF 1.4+.
func (a *Artifact) Kind(kind ArtifactType) *Artifact {
	a.Pair(Name("Type"), kind.toName())
	return a
}

// Subtype is Artifact::subtype in pdf-writer src/content.rs.
//
// Write the /Subtype entry to set the subtype of pagination or inline
// artifacts. Specific to artifacts. PDF 1.7+.
func (a *Artifact) Subtype(subtype ArtifactSubtype) *Artifact {
	a.Pair(Name("Subtype"), subtype.toName())
	return a
}

// BoundingBox is Artifact::bounding_box in pdf-writer src/content.rs.
//
// Write the /BBox entry to set the bounding box of the artifact. Specific
// to artifacts. Required for background artifacts. PDF 1.4+.
func (a *Artifact) BoundingBox(bbox Rect) *Artifact {
	a.Pair(Name("BBox"), bbox)
	return a
}

// Attached is Artifact::attached in pdf-writer src/content.rs.
//
// Write the /Attached entry to set where the artifact is attached to the
// page. Only for pagination and full-page background artifacts. Specific to
// artifacts. PDF 1.4+.
func (a *Artifact) Attached(attachment ...ArtifactAttachment) *Artifact {
	array := NewTypedArray[Name](a.Insert(Name("Attached")).Array())
	for _, v := range attachment {
		array.Item(v.toName())
	}
	return a
}

// ArtifactType is ArtifactType in pdf-writer src/content.rs.
//
// The various types of layout artifacts.
type ArtifactType uint8

// The values of ArtifactType.
const (
	// ArtifactTypePagination is for artifacts of the pagination process like
	// headers, footers, page numbers.
	ArtifactTypePagination ArtifactType = iota
	// ArtifactTypeLayout is for artifacts of the layout process such as
	// footnote rules.
	ArtifactTypeLayout
	// ArtifactTypePage is for artifacts of the page, like printer's marks.
	ArtifactTypePage
	// ArtifactTypeBackground is for background image artifacts. PDF 1.7+.
	ArtifactTypeBackground
	// ArtifactTypeInline is for artefacts related to inline content, such as
	// line numbers or redactions. PDF 2.0+.
	ArtifactTypeInline
)

// String returns the PDF name of the artifact type, without the slash.
func (t ArtifactType) String() string {
	return string(t.toName())
}

// toName is ArtifactType::to_name in pdf-writer src/content.rs.
func (t ArtifactType) toName() Name {
	switch t {
	case ArtifactTypePagination:
		return Name("Pagination")
	case ArtifactTypeLayout:
		return Name("Layout")
	case ArtifactTypePage:
		return Name("Page")
	case ArtifactTypeBackground:
		return Name("Background")
	case ArtifactTypeInline:
		return Name("Inline")
	}
	panic(fmt.Sprintf("pdfwriter: the artifact type %d is not known", uint8(t)))
}

// ArtifactSubtype is ArtifactSubtype in pdf-writer src/content.rs. It is a
// struct with the case in Kind, and the name of the case Custom in Name
// (D27). The zero value is the case Header.
//
// The various subtypes of pagination artifacts.
type ArtifactSubtype struct {
	Kind ArtifactSubtypeKind
	Name Name
}

// ArtifactSubtypeKind is the case of an ArtifactSubtype.
type ArtifactSubtypeKind uint8

// The cases of ArtifactSubtype.
const (
	// ArtifactSubtypeHeader is for headers.
	ArtifactSubtypeHeader ArtifactSubtypeKind = iota
	// ArtifactSubtypeFooter is for footers.
	ArtifactSubtypeFooter
	// ArtifactSubtypeWatermark is for background watermarks.
	ArtifactSubtypeWatermark
	// ArtifactSubtypePageNumber is for page numbers. PDF 2.0+.
	ArtifactSubtypePageNumber
	// ArtifactSubtypeBates is for Bates numbering. PDF 2.0+.
	ArtifactSubtypeBates
	// ArtifactSubtypeLineNumber is for line numbers. PDF 2.0+.
	ArtifactSubtypeLineNumber
	// ArtifactSubtypeRedaction is for redactions. PDF 2.0+.
	ArtifactSubtypeRedaction
	// ArtifactSubtypeCustom is a custom subtype named according to ISO
	// 32000-1:2008 Annex E.
	ArtifactSubtypeCustom
)

// artifactSubtypeKindNames holds the Rust name of each case of
// ArtifactSubtype.
var artifactSubtypeKindNames = [...]string{
	ArtifactSubtypeHeader:     "Header",
	ArtifactSubtypeFooter:     "Footer",
	ArtifactSubtypeWatermark:  "Watermark",
	ArtifactSubtypePageNumber: "PageNumber",
	ArtifactSubtypeBates:      "Bates",
	ArtifactSubtypeLineNumber: "LineNumber",
	ArtifactSubtypeRedaction:  "Redaction",
	ArtifactSubtypeCustom:     "Custom",
}

// String returns the Rust name of the case.
func (k ArtifactSubtypeKind) String() string {
	if int(k) < len(artifactSubtypeKindNames) {
		return artifactSubtypeKindNames[k]
	}
	return "ArtifactSubtypeKind(?)"
}

// toName is ArtifactSubtype::to_name in pdf-writer src/content.rs.
func (s ArtifactSubtype) toName() Name {
	switch s.Kind {
	case ArtifactSubtypeHeader:
		return Name("Header")
	case ArtifactSubtypeFooter:
		return Name("Footer")
	case ArtifactSubtypeWatermark:
		return Name("Watermark")
	case ArtifactSubtypePageNumber:
		return Name("PageNum")
	case ArtifactSubtypeBates:
		return Name("Bates")
	case ArtifactSubtypeLineNumber:
		return Name("LineNum")
	case ArtifactSubtypeRedaction:
		return Name("Redaction")
	case ArtifactSubtypeCustom:
		return s.Name
	}
	panic(fmt.Sprintf("pdfwriter: the artifact subtype %d is not known", uint8(s.Kind)))
}

// ArtifactAttachment is ArtifactAttachment in pdf-writer src/content.rs.
//
// Where a layout Artifact is attached to the page edge.
type ArtifactAttachment uint8

// The values of ArtifactAttachment.
const (
	ArtifactAttachmentLeft ArtifactAttachment = iota
	ArtifactAttachmentTop
	ArtifactAttachmentRight
	ArtifactAttachmentBottom
)

// String returns the PDF name of the attachment, without the slash.
func (a ArtifactAttachment) String() string {
	return string(a.toName())
}

// toName is ArtifactAttachment::to_name in pdf-writer src/content.rs.
func (a ArtifactAttachment) toName() Name {
	switch a {
	case ArtifactAttachmentLeft:
		return Name("Left")
	case ArtifactAttachmentTop:
		return Name("Top")
	case ArtifactAttachmentRight:
		return Name("Right")
	case ArtifactAttachmentBottom:
		return Name("Bottom")
	}
	panic(fmt.Sprintf("pdfwriter: the artifact attachment %d is not known", uint8(a)))
}

// Compatibility.

// BeginCompat is Content::begin_compat in pdf-writer src/content.rs.
//
// BX: Begin a compatibility section.
func (c *Content) BeginCompat() *Content {
	c.Op("BX")
	return c
}

// EndCompat is Content::end_compat in pdf-writer src/content.rs.
//
// EX: End a compatibility section.
func (c *Content) EndCompat() *Content {
	c.Op("EX")
	return c
}

// Resources is Resources in pdf-writer src/content.rs.
//
// Writer for a resource dictionary.
//
// This struct is created by Pages.Resources, Page.Resources,
// FormXObject.Resources, and TilingPattern.Resources.
type Resources struct {
	*Dict
}

// start is the macro writer! for Resources in pdf-writer src/content.rs.
func (r *Resources) start(obj Obj) {
	r.Dict = obj.Dict()
}

// XObjects is Resources::x_objects in pdf-writer src/content.rs.
//
// Start writing the /XObject dictionary.
//
// Relevant types:
//   - ImageXObject
//   - FormXObject
func (r *Resources) XObjects() *Dict {
	return r.Insert(Name("XObject")).Dict()
}

// Fonts is Resources::fonts in pdf-writer src/content.rs.
//
// Start writing the /Font dictionary.
//
// Relevant types:
//   - Type1Font
//   - Type3Font
//   - Type0Font
func (r *Resources) Fonts() *Dict {
	return r.Insert(Name("Font")).Dict()
}

// ColorSpaces is Resources::color_spaces in pdf-writer src/content.rs.
//
// Start writing the /ColorSpace dictionary. PDF 1.1+.
//
// Relevant types:
//   - ColorSpace
func (r *Resources) ColorSpaces() *Dict {
	return r.Insert(Name("ColorSpace")).Dict()
}

// Patterns is Resources::patterns in pdf-writer src/content.rs.
//
// Start writing the /Pattern dictionary. PDF 1.2+.
//
// Relevant types:
//   - TilingPattern
//   - ShadingPattern
func (r *Resources) Patterns() *Dict {
	return r.Insert(Name("Pattern")).Dict()
}

// Shadings is Resources::shadings in pdf-writer src/content.rs.
//
// Start writing the /Shading dictionary. PDF 1.3+.
//
// Relevant types:
//   - FunctionShading
func (r *Resources) Shadings() *Dict {
	return r.Insert(Name("Shading")).Dict()
}

// ExtGStates is Resources::ext_g_states in pdf-writer src/content.rs.
//
// Start writing the /ExtGState dictionary. PDF 1.2+.
//
// Relevant types:
//   - ExtGraphicsState
func (r *Resources) ExtGStates() *Dict {
	return r.Insert(Name("ExtGState")).Dict()
}

// ProcSets is Resources::proc_sets in pdf-writer src/content.rs.
//
// Write the /ProcSet attribute.
//
// This defines what procedure sets are sent to an output device when
// printing the file as PostScript. The attribute is only used for PDFs with
// versions below 1.4.
func (r *Resources) ProcSets(sets ...ProcSet) *Resources {
	array := r.Insert(Name("ProcSet")).Array()
	for _, set := range sets {
		array.Item(set.toName())
	}
	return r
}

// ProcSetsAll is Resources::proc_sets_all in pdf-writer src/content.rs.
//
// Write the /ProcSet attribute with all available procedure sets.
//
// The PDF 1.7 specification recommends that modern PDFs either omit the
// attribute or specify all available procedure sets, as this function does.
func (r *Resources) ProcSetsAll() *Resources {
	return r.ProcSets(
		ProcSetPDF,
		ProcSetText,
		ProcSetImageGrayscale,
		ProcSetImageColor,
		ProcSetImageIndexed,
	)
}

// Properties is Resources::properties in pdf-writer src/content.rs.
//
// Start writing the /Properties attribute.
//
// This allows to write property lists with indirect objects for
// marked-content sequences. These properties can be used by property lists
// using the MarkContent.PropertiesNamed method. PDF 1.2+.
func (r *Resources) Properties() *WriterDict[PropertyList, *PropertyList] {
	return NewWriterDict[PropertyList](r.Insert(Name("Properties")).Dict())
}

// ProcSet is ProcSet in pdf-writer src/content.rs.
//
// What procedure sets to send to a PostScript printer or other output
// device.
//
// This enumeration provides compatibility for printing PDFs of versions 1.3
// and below.
type ProcSet uint8

// The values of ProcSet. The Go names spell the initialisms in capitals, so
// that ProcSetPDF is ProcSet::Pdf.
const (
	// ProcSetPDF is for painting and graphics state.
	ProcSetPDF ProcSet = iota
	// ProcSetText is for text.
	ProcSetText
	// ProcSetImageGrayscale is for grayscale images and masks.
	ProcSetImageGrayscale
	// ProcSetImageColor is for color images.
	ProcSetImageColor
	// ProcSetImageIndexed is for images with color tables.
	ProcSetImageIndexed
)

// String returns the PDF name of the procedure set, without the slash.
func (s ProcSet) String() string {
	return string(s.toName())
}

// toName is ProcSet::to_name in pdf-writer src/content.rs.
func (s ProcSet) toName() Name {
	switch s {
	case ProcSetPDF:
		return Name("PDF")
	case ProcSetText:
		return Name("Text")
	case ProcSetImageGrayscale:
		return Name("ImageB")
	case ProcSetImageColor:
		return Name("ImageC")
	case ProcSetImageIndexed:
		return Name("ImageI")
	}
	panic(fmt.Sprintf("pdfwriter: the procedure set %d is not known", uint8(s)))
}

// ExtGraphicsState is ExtGraphicsState in pdf-writer src/content.rs.
//
// Writer for a dictionary with additional parameters for the graphics
// state.
//
// This struct is created by Chunk.ExtGraphics and ShadingPattern.ExtGraphics.
type ExtGraphicsState struct {
	*Dict
}

// start is the macro writer! for ExtGraphicsState in pdf-writer
// src/content.rs.
func (e *ExtGraphicsState) start(obj Obj) {
	e.Dict = obj.Dict()
	e.Pair(Name("Type"), Name("ExtGState"))
}

// LineWidth is ExtGraphicsState::line_width in pdf-writer src/content.rs.
//
// Write the LW attribute to set the line width. PDF 1.3+.
func (e *ExtGraphicsState) LineWidth(width float32) *ExtGraphicsState {
	e.Pair(Name("LW"), Real(width))
	return e
}

// LineCap is ExtGraphicsState::line_cap in pdf-writer src/content.rs.
//
// Write the LC attribute to set the line cap style. PDF 1.3+.
func (e *ExtGraphicsState) LineCap(lineCap LineCapStyle) *ExtGraphicsState {
	e.Pair(Name("LC"), Int(lineCap.toInt()))
	return e
}

// LineJoin is ExtGraphicsState::line_join in pdf-writer src/content.rs.
//
// Write the LJ attribute to set the line join style. PDF 1.3+.
func (e *ExtGraphicsState) LineJoin(join LineJoinStyle) *ExtGraphicsState {
	e.Pair(Name("LJ"), Int(join.toInt()))
	return e
}

// MiterLimit is ExtGraphicsState::miter_limit in pdf-writer src/content.rs.
//
// Write the ML attribute to set the miter limit. PDF 1.3+.
func (e *ExtGraphicsState) MiterLimit(limit float32) *ExtGraphicsState {
	e.Pair(Name("ML"), Real(limit))
	return e
}

// DashPattern is ExtGraphicsState::dash_pattern in pdf-writer
// src/content.rs.
//
// Write the D attribute to set the dash pattern. PDF 1.3+.
func (e *ExtGraphicsState) DashPattern(pattern []float32, phase float32) *ExtGraphicsState {
	array := e.Insert(Name("D")).Array()
	inner := array.Push().Array()
	for _, v := range pattern {
		inner.Item(Real(v))
	}
	array.Item(Real(phase))
	array.Finish()
	return e
}

// RenderingIntent is ExtGraphicsState::rendering_intent in pdf-writer
// src/content.rs.
//
// Write the RI attribute to set the rendering intent. PDF 1.3+.
func (e *ExtGraphicsState) RenderingIntent(intent RenderingIntent) *ExtGraphicsState {
	e.Pair(Name("RI"), intent.toName())
	return e
}

// Overprint is ExtGraphicsState::overprint in pdf-writer src/content.rs.
//
// Write the OP attribute to set the overprint mode for all operations,
// except if an op entry is present. If so, only influence the stroking
// operations. PDF 1.2+.
func (e *ExtGraphicsState) Overprint(overprint bool) *ExtGraphicsState {
	e.Pair(Name("OP"), Bool(overprint))
	return e
}

// OverprintFill is ExtGraphicsState::overprint_fill in pdf-writer
// src/content.rs.
//
// Write the op attribute to set the overprint mode for fill operations.
// PDF 1.3+.
func (e *ExtGraphicsState) OverprintFill(overprint bool) *ExtGraphicsState {
	e.Pair(Name("op"), Bool(overprint))
	return e
}

// OverprintMode is ExtGraphicsState::overprint_mode in pdf-writer
// src/content.rs.
//
// Write the OPM attribute to set the overprint mode for components that have
// been zeroed out. PDF 1.3+.
//
// Note that this attribute is restricted by PDF/A.
func (e *ExtGraphicsState) OverprintMode(mode OverprintMode) *ExtGraphicsState {
	e.Pair(Name("OPM"), Int(mode.toInt()))
	return e
}

// Font is ExtGraphicsState::font in pdf-writer src/content.rs.
//
// Write the Font attribute to set the font. PDF 1.3+.
func (e *ExtGraphicsState) Font(font Name, size float32) *ExtGraphicsState {
	array := e.Insert(Name("Font")).Array()
	array.Item(font)
	array.Item(Real(size))
	array.Finish()
	return e
}

// BlackGeneration is ExtGraphicsState::black_generation in pdf-writer
// src/content.rs.
//
// Write the BG attribute to set the black generation function.
func (e *ExtGraphicsState) BlackGeneration(fn Ref) *ExtGraphicsState {
	e.Pair(Name("BG"), fn)
	return e
}

// BlackGenerationDefault is ExtGraphicsState::black_generation_default in
// pdf-writer src/content.rs.
//
// Write the BG2 attribute to set the black-generation function back to the
// function that has been in effect at the beginning of the page. PDF 1.3+.
func (e *ExtGraphicsState) BlackGenerationDefault() *ExtGraphicsState {
	e.Pair(Name("BG2"), Name("Default"))
	return e
}

// UndercolorRemoval is ExtGraphicsState::undercolor_removal in pdf-writer
// src/content.rs.
//
// Write the UCR attribute to set the undercolor removal function.
func (e *ExtGraphicsState) UndercolorRemoval(fn Ref) *ExtGraphicsState {
	e.Pair(Name("UCR"), fn)
	return e
}

// UndercolorRemovalDefault is ExtGraphicsState::undercolor_removal_default
// in pdf-writer src/content.rs.
//
// Write the UCR2 attribute to set the undercolor removal function back to
// the function that has been in effect at the beginning of the page. PDF
// 1.3+.
func (e *ExtGraphicsState) UndercolorRemovalDefault() *ExtGraphicsState {
	e.Pair(Name("UCR2"), Name("Default"))
	return e
}

// Transfer is ExtGraphicsState::transfer in pdf-writer src/content.rs.
//
// Write the TR attribute to set the transfer function.
//
// Note that this key is illegal in PDF/A.
func (e *ExtGraphicsState) Transfer(fn Ref) *ExtGraphicsState {
	e.Pair(Name("TR"), fn)
	return e
}

// TransferDefault is ExtGraphicsState::transfer_default in pdf-writer
// src/content.rs.
//
// Write the TR2 attribute to set the transfer function back to the function
// that has been in effect at the beginning of the page. PDF 1.3+.
func (e *ExtGraphicsState) TransferDefault() *ExtGraphicsState {
	e.Pair(Name("TR2"), Name("Default"))
	return e
}

// Halftone is ExtGraphicsState::halftone in pdf-writer src/content.rs.
//
// Write the HT attribute to set the halftone.
//
// Note that this value may be ignored in PDF/A.
func (e *ExtGraphicsState) Halftone(ht Ref) *ExtGraphicsState {
	e.Pair(Name("HT"), ht)
	return e
}

// HalftoneDefault is ExtGraphicsState::halftone_default in pdf-writer
// src/content.rs.
//
// Write the HT attribute to set the halftone back to the one that has been
// in effect at the beginning of the page.
func (e *ExtGraphicsState) HalftoneDefault() *ExtGraphicsState {
	e.Pair(Name("HT"), Name("Default"))
	return e
}

// Flatness is ExtGraphicsState::flatness in pdf-writer src/content.rs.
//
// Write the FL attribute to set the flatness tolerance. PDF 1.3+.
//
// Note that this key may be ignored in PDF/A.
func (e *ExtGraphicsState) Flatness(tolerance float32) *ExtGraphicsState {
	e.Pair(Name("FL"), Real(tolerance))
	return e
}

// Smoothness is ExtGraphicsState::smoothness in pdf-writer src/content.rs.
//
// Write the SM attribute to set the smoothness tolerance. PDF 1.3+.
func (e *ExtGraphicsState) Smoothness(tolerance float32) *ExtGraphicsState {
	e.Pair(Name("SM"), Real(tolerance))
	return e
}

// StrokeAdjustment is ExtGraphicsState::stroke_adjustment in pdf-writer
// src/content.rs.
//
// Write the SA attribute to set automatic stroke adjustment.
func (e *ExtGraphicsState) StrokeAdjustment(adjust bool) *ExtGraphicsState {
	e.Pair(Name("SA"), Bool(adjust))
	return e
}

// BlendMode is ExtGraphicsState::blend_mode in pdf-writer src/content.rs.
//
// Write the BM attribute to set the blend mode. PDF 1.4+.
//
// Note that this key is restricted in PDF/A-1.
func (e *ExtGraphicsState) BlendMode(mode BlendMode) *ExtGraphicsState {
	e.Pair(Name("BM"), mode.toName())
	return e
}

// SoftMask is ExtGraphicsState::soft_mask in pdf-writer src/content.rs.
//
// Start writing the SMask attribute. PDF 1.4+.
//
// Note that this key is forbidden in PDF/A-1.
func (e *ExtGraphicsState) SoftMask() *SoftMask {
	return Start[SoftMask](e.Insert(Name("SMask")))
}

// SoftMaskName is ExtGraphicsState::soft_mask_name in pdf-writer
// src/content.rs.
//
// Write the SMask attribute using a name. PDF 1.4+.
//
// Note that this key is forbidden in PDF/A-1.
func (e *ExtGraphicsState) SoftMaskName(mask Name) *ExtGraphicsState {
	e.Pair(Name("SMask"), mask)
	return e
}

// StrokingAlpha is ExtGraphicsState::stroking_alpha in pdf-writer
// src/content.rs.
//
// Write the CA attribute to set the stroking alpha constant. PDF 1.4+.
//
// Note that this key is restricted in PDF/A-1.
func (e *ExtGraphicsState) StrokingAlpha(alpha float32) *ExtGraphicsState {
	e.Pair(Name("CA"), Real(alpha))
	return e
}

// NonStrokingAlpha is ExtGraphicsState::non_stroking_alpha in pdf-writer
// src/content.rs.
//
// Write the ca attribute to set the non-stroking alpha constant. PDF 1.4+.
//
// Note that this key is restricted in PDF/A-1.
func (e *ExtGraphicsState) NonStrokingAlpha(alpha float32) *ExtGraphicsState {
	e.Pair(Name("ca"), Real(alpha))
	return e
}

// AlphaSource is ExtGraphicsState::alpha_source in pdf-writer
// src/content.rs.
//
// Write the AIS attribute to set the alpha source flag. CA and ca values as
// well as the SMask will be interpreted as shape instead of opacity. PDF
// 1.4+.
func (e *ExtGraphicsState) AlphaSource(source bool) *ExtGraphicsState {
	e.Pair(Name("AIS"), Bool(source))
	return e
}

// TextKnockout is ExtGraphicsState::text_knockout in pdf-writer
// src/content.rs.
//
// Write the TK attribute to set the text knockout flag. PDF 1.4+.
func (e *ExtGraphicsState) TextKnockout(knockout bool) *ExtGraphicsState {
	e.Pair(Name("TK"), Bool(knockout))
	return e
}

// ExtGraphics is Chunk::ext_graphics in pdf-writer src/chunk.rs.
//
// Start writing an external graphics state dictionary.
func (c *Chunk) ExtGraphics(id Ref) *ExtGraphicsState {
	return Start[ExtGraphicsState](c.Indirect(id))
}

// BlendMode is BlendMode in pdf-writer src/content.rs. The zero value is
// BlendModeNormal, the default of upstream.
//
// How to blend source and backdrop.
type BlendMode uint8

// The values of BlendMode.
const (
	BlendModeNormal BlendMode = iota
	BlendModeMultiply
	BlendModeScreen
	BlendModeOverlay
	BlendModeDarken
	BlendModeLighten
	BlendModeColorDodge
	BlendModeColorBurn
	BlendModeHardLight
	BlendModeSoftLight
	BlendModeDifference
	BlendModeExclusion
	BlendModeHue
	BlendModeSaturation
	BlendModeColor
	BlendModeLuminosity
)

// blendModeNames holds the PDF name of each value of BlendMode.
var blendModeNames = [...]Name{
	BlendModeNormal:     "Normal",
	BlendModeMultiply:   "Multiply",
	BlendModeScreen:     "Screen",
	BlendModeOverlay:    "Overlay",
	BlendModeDarken:     "Darken",
	BlendModeLighten:    "Lighten",
	BlendModeColorDodge: "ColorDodge",
	BlendModeColorBurn:  "ColorBurn",
	BlendModeHardLight:  "HardLight",
	BlendModeSoftLight:  "SoftLight",
	BlendModeDifference: "Difference",
	BlendModeExclusion:  "Exclusion",
	BlendModeHue:        "Hue",
	BlendModeSaturation: "Saturation",
	BlendModeColor:      "Color",
	BlendModeLuminosity: "Luminosity",
}

// String returns the PDF name of the blend mode, without the slash.
func (m BlendMode) String() string {
	return string(m.toName())
}

// toName is BlendMode::to_name in pdf-writer src/content.rs.
func (m BlendMode) toName() Name {
	if int(m) < len(blendModeNames) {
		return blendModeNames[m]
	}
	panic(fmt.Sprintf("pdfwriter: the blend mode %d is not known", uint8(m)))
}

// OverprintMode is OverprintMode in pdf-writer src/content.rs. The zero
// value is OverprintModeOverrideAllColorants, the default of upstream.
//
// How to behave when overprinting for colorants with the value zero.
type OverprintMode uint8

// The values of OverprintMode.
const (
	// OverprintModeOverrideAllColorants means that an overprint operation
	// will always discard the underlying color, even if one of the colorants
	// is zero.
	OverprintModeOverrideAllColorants OverprintMode = iota
	// OverprintModeIgnoreZeroChannel means that an overprint operation will
	// only discard the underlying colorant component (e.g. cyan in CMYK) if
	// the new corresponding colorant is non-zero.
	//
	// Note that this value is forbidden by PDF/A for ICCBased color spaces
	// when overprinting is enabled.
	OverprintModeIgnoreZeroChannel
)

// String returns the Rust name of the overprint mode.
func (m OverprintMode) String() string {
	switch m {
	case OverprintModeOverrideAllColorants:
		return "OverrideAllColorants"
	case OverprintModeIgnoreZeroChannel:
		return "IgnoreZeroChannel"
	}
	return "OverprintMode(?)"
}

// toInt is OverprintMode::to_int in pdf-writer src/content.rs.
func (m OverprintMode) toInt() int32 {
	switch m {
	case OverprintModeOverrideAllColorants:
		return 0
	case OverprintModeIgnoreZeroChannel:
		return 1
	}
	panic(fmt.Sprintf("pdfwriter: the overprint mode %d is not known", uint8(m)))
}

// SoftMask is SoftMask in pdf-writer src/content.rs.
//
// Writer for a soft mask dictionary.
//
// This struct is created by ExtGraphicsState.SoftMask.
type SoftMask struct {
	*Dict
}

// start is the macro writer! for SoftMask in pdf-writer src/content.rs.
func (m *SoftMask) start(obj Obj) {
	m.Dict = obj.Dict()
	m.Pair(Name("Type"), Name("Mask"))
}

// Subtype is SoftMask::subtype in pdf-writer src/content.rs.
//
// Write the S attribute to set the soft mask subtype. Required.
func (m *SoftMask) Subtype(subtype MaskType) *SoftMask {
	m.Pair(Name("S"), subtype.toName())
	return m
}

// Group is SoftMask::group in pdf-writer src/content.rs.
//
// Write the G attribute to set the transparency group XObject. The group has
// to have a color space set in the /CS attribute if the mask subtype is
// Luminosity. Required.
func (m *SoftMask) Group(group Ref) *SoftMask {
	m.Pair(Name("G"), group)
	return m
}

// Backdrop is SoftMask::backdrop in pdf-writer src/content.rs.
//
// Write the BC attribute to set the background color for the transparency
// group. Only applicable if the mask subtype is Luminosity. Has to be set in
// the group's color space.
func (m *SoftMask) Backdrop(color ...float32) *SoftMask {
	array := m.Insert(Name("BC")).Array()
	for _, v := range color {
		array.Item(Real(v))
	}
	return m
}

// TransferFunction is SoftMask::transfer_function in pdf-writer
// src/content.rs.
//
// Write the TR attribute, a function that maps from the group's output
// values to the mask opacity.
func (m *SoftMask) TransferFunction(function Ref) *SoftMask {
	m.Pair(Name("TR"), function)
	return m
}

// MaskType is MaskType in pdf-writer src/content.rs.
//
// What property in the mask influences the target alpha.
type MaskType uint8

// The values of MaskType.
const (
	// MaskTypeAlpha means that the alpha values from the mask are applied to
	// the target.
	MaskTypeAlpha MaskType = iota
	// MaskTypeLuminosity means that a single-channel luminosity value is
	// calculated for the colors in the mask.
	MaskTypeLuminosity
)

// String returns the PDF name of the mask type, without the slash.
func (t MaskType) String() string {
	return string(t.toName())
}

// toName is MaskType::to_name in pdf-writer src/content.rs.
func (t MaskType) toName() Name {
	switch t {
	case MaskTypeAlpha:
		return Name("Alpha")
	case MaskTypeLuminosity:
		return Name("Luminosity")
	}
	panic(fmt.Sprintf("pdfwriter: the mask type %d is not known", uint8(t)))
}
