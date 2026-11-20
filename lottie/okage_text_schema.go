// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the schema of a text layer, of
// the fonts and of the glyphs of a Lottie file, which velato does not parse
// (D79). The keys and their meaning follow the Lottie loader of ThorVG, in
// src/loaders/lottie/tvgLottieParser.cpp.

import (
	"encoding/json"
)

// TextLayer is a text layer, ty 5.
type TextLayer struct {
	VisualLayer

	Text TextData `json:"t"`
}

// TextData is the text of a text layer, its key t. okage reads only the
// text document. The text animators, the key a, the alignment of the
// animators, the key m, and the path of the text, the key p, are not
// drawn (D79).
type TextData struct {
	Document TextDocumentProperty `json:"d"`
}

// TextDocumentProperty is the animated text document of a text layer, its
// key d. Each keyframe holds a whole document, and the documents do not
// blend.
type TextDocumentProperty struct {
	Keyframes []TextDocumentKeyframe `json:"k"`
}

// UnmarshalJSON requires the key "k".
func (p *TextDocumentProperty) UnmarshalJSON(data []byte) error {
	type plain TextDocumentProperty
	if err := requireKeys(data, "k"); err != nil {
		return err
	}
	return json.Unmarshal(data, (*plain)(p))
}

// TextDocumentKeyframe is a keyframe of a text document: the document from
// the frame Time on.
type TextDocumentKeyframe struct {
	Time     float64            `json:"t"`
	Document TextDocumentSchema `json:"s"`
}

// TextDocumentSchema is a text document, as getValue(TextDocument&) of
// ThorVG reads it.
type TextDocumentSchema struct {
	// Text is the text. A carriage return or the character 3 starts a new
	// line.
	Text string `json:"t"`
	// FontName is the name of the font, the key fName of an entry of the
	// font list.
	FontName string `json:"f"`
	// Size is the font size, in pixels.
	Size float64 `json:"s"`
	// Justify is 0 for the left, 1 for the right and 2 for the center.
	Justify int `json:"j"`
	// Tracking is the space after each glyph, in thousandths of an em.
	Tracking float64 `json:"tr"`
	// LineHeight is the distance from one baseline to the next, in pixels.
	LineHeight float64 `json:"lh"`
	// BaselineShift moves the baseline up, in pixels.
	BaselineShift float64 `json:"ls"`
	// FillColor and StrokeColor are colors of red, green and blue, from 0 to
	// 1.
	FillColor   []float64 `json:"fc"`
	StrokeColor []float64 `json:"sc"`
	// StrokeWidth is the width of the stroke, in pixels. 0 is no stroke.
	StrokeWidth float64 `json:"sw"`
	// StrokeOverFill draws the stroke over the fill.
	StrokeOverFill *bool `json:"of"`
	// BoxPosition and BoxSize are the box of a box text. A text without
	// a box has no BoxSize.
	BoxPosition []float64 `json:"ps"`
	BoxSize     []float64 `json:"sz"`
}

// FontList is the list of the fonts of a file, its key fonts.
type FontList struct {
	List []FontSchema `json:"list"`
}

// FontSchema is a font of the font list.
type FontSchema struct {
	// Name is the name that a text document names the font by.
	Name string `json:"fName"`
	// Family and Style are the family and the style of the font, such as
	// "Noto Sans" and "Bold Italic".
	Family string `json:"fFamily"`
	Style  string `json:"fStyle"`
	// Path is the place of the font file, or a data URL.
	Path string `json:"fPath"`
	// Ascent is the height of the font above the baseline, in hundredths of
	// the font size.
	Ascent float64 `json:"ascent"`
	// Origin is 0 when the glyphs of the font are in the list chars.
	Origin int `json:"origin"`
}

// CharData is a glyph of the list chars of a file.
type CharData struct {
	// Char is the text that the glyph draws, often one character.
	Char string `json:"ch"`
	// Size is the font size of the outline, often 100.
	Size float64 `json:"size"`
	// Family and Style name the font of the glyph, with the keys fFamily
	// and fStyle of the font list.
	Family string `json:"fFamily"`
	Style  string `json:"style"`
	// Width is the advance of the glyph, at the font size 100.
	Width float64 `json:"w"`
	// Data holds the shapes of the outline.
	Data CharShapes `json:"data"`
}

// CharShapes holds the outline of a glyph as a list of shapes, groups of
// paths, at the font size 100.
type CharShapes struct {
	Shapes []AnyShape `json:"shapes"`
}
