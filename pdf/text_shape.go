// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"github.com/xo/okage/typesetting/harfbuzz"
)

// This file ports krilla src/text/shape.rs. harfrust shapes the text in
// upstream, and typesetting/harfbuzz shapes it in the port (D52, D62).

// TextDirection is TextDirection in krilla src/text/shape.rs.
//
// The direction of a text.
type TextDirection uint8

// The values of TextDirection.
const (
	// TextDirectionAuto determines the direction automatically.
	TextDirectionAuto TextDirection = iota
	// TextDirectionLeftToRight is left to right.
	TextDirectionLeftToRight
	// TextDirectionRightToLeft is right to left.
	TextDirectionRightToLeft
	// TextDirectionTopToBottom is top to bottom.
	TextDirectionTopToBottom
	// TextDirectionBottomToTop is bottom to top.
	TextDirectionBottomToTop
)

// naiveShape is naive_shape in krilla src/text/shape.rs.
//
// harfrust takes the byte offset of each character as its cluster, and
// typesetting takes the index of the rune, which the port maps back to the
// byte offset. guess_segment_properties of harfrust sets the script and the
// direction, and no language. GuessSegmentProperties of typesetting also
// sets the language of the locale of the process, so the port sets no
// language either.
//
// Naively shape some text with a single font.
func naiveShape(text string, font *Font, direction TextDirection) []Glyph {
	runes := []rune(text)
	byteOffsets := make([]int, 0, len(runes))
	for i := range text {
		byteOffsets = append(byteOffsets, i)
	}

	buffer := harfbuzz.NewBuffer()
	buffer.AddRunes(runes, 0, -1)
	// A language that is set keeps GuessSegmentProperties from taking the
	// language of the locale, and the port then removes it.
	buffer.Props.Language = "und"
	buffer.GuessSegmentProperties()
	buffer.Props.Language = ""

	switch direction {
	case TextDirectionLeftToRight:
		buffer.Props.Direction = harfbuzz.LeftToRight
	case TextDirectionRightToLeft:
		buffer.Props.Direction = harfbuzz.RightToLeft
	case TextDirectionTopToBottom:
		buffer.Props.Direction = harfbuzz.TopToBottom
	case TextDirectionBottomToTop:
		buffer.Props.Direction = harfbuzz.BottomToTop
	}

	dir := buffer.Props.Direction

	font.mu.Lock()
	if font.hb == nil {
		font.hb = harfbuzz.NewFont(font.face)
	}
	buffer.Shape(font.hb, nil)
	font.mu.Unlock()

	positions := buffer.Pos
	infos := buffer.Info
	cluster := func(i int) int {
		c := infos[i].Cluster
		if c < 0 || c >= len(byteOffsets) {
			return len(text)
		}
		return byteOffsets[c]
	}
	upem := font.UnitsPerEm()

	glyphs := make([]Glyph, 0, len(infos))
	for i := range infos {
		pos := positions[i]
		startCluster := infos[i].Cluster
		start := cluster(i)

		end := len(text)
		if dir == harfbuzz.LeftToRight || dir == harfbuzz.TopToBottom {
			e := i + 1
			for e < len(infos) && infos[e].Cluster == startCluster {
				e++
			}
			if e < len(infos) {
				end = cluster(e)
			}
		} else {
			e := i - 1
			for e >= 0 && infos[e].Cluster == startCluster {
				e--
			}
			if e >= 0 {
				end = cluster(e)
			}
		}

		glyphs = append(glyphs, Glyph{
			GlyphID:   GlyphID(infos[i].Glyph),
			XAdvance:  float32(pos.XAdvance) / upem,
			XOffset:   float32(pos.XOffset) / upem,
			YOffset:   float32(pos.YOffset) / upem,
			YAdvance:  float32(pos.YAdvance) / upem,
			TextRange: TextRange{Start: start, End: end},
		})
	}

	return glyphs
}
