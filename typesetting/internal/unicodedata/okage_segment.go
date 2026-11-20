// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package unicodedata

import "unicode"

// okage: D83. This file is okage's own. The segmenter reads six properties
// of each rune. Each one has a table of its own, and each lookup walks
// five levels of its table. LookupSegmentProperties gives the six in one
// lookup of two levels, from the table of okage_segment_table.go. The test
// TestGenerateSegmentTable writes that table from the lookups of each
// property, and TestSegmentProperties compares it with them for each rune.

// SegmentProperties holds the properties of a rune that the segmenter
// reads.
type SegmentProperties struct {
	LineBreak            LineBreak
	GraphemeBreak        GraphemeBreak
	WordBreak            WordBreak
	GeneralCategory      GeneralCategory
	IndicConjunctBreak   IndicConjunctBreak
	ExtendedPictographic bool
}

// segmentBlockBits is the number of the low bits of a rune that select the
// rune in its block of segmentBlocks.
const segmentBlockBits = 8

// The fields of a value of segmentRaw, from the low bits: the general
// category, then the indices of the line break, the grapheme break, the
// word break and the Indic conjunct break, as the tables of each property
// give them, and 1 for an extended pictographic rune.
const (
	segmentLineBreakShift     = 5
	segmentGraphemeBreakShift = 11
	segmentWordBreakShift     = 15
	segmentIndicShift         = 19
	segmentPictographicShift  = 21
)

// packSegment packs the properties of r into a value of segmentRaw. Only
// the generator and the tests call it.
func packSegment(r rune) uint32 {
	return uint32(gcLookup(r)) |
		uint32(lbLookup(r))<<segmentLineBreakShift |
		uint32(gbLookup(r))<<segmentGraphemeBreakShift |
		uint32(wbLookup(r))<<segmentWordBreakShift |
		uint32(indicCBLookup(r))<<segmentIndicShift |
		uint32(emojiLookup(r))<<segmentPictographicShift
}

// segmentProps holds the properties of each value of segmentRaw.
var segmentProps = func() (t [len(segmentRaw)]SegmentProperties) {
	flag := func(i uint32) uint64 {
		if i == 0 {
			return 0
		}
		return 1 << (i - 1)
	}
	for i, raw := range segmentRaw {
		t[i] = SegmentProperties{
			LineBreak:            LineBreak(flag(raw >> segmentLineBreakShift & 0x3F)),
			GraphemeBreak:        GraphemeBreak(flag(raw >> segmentGraphemeBreakShift & 0xF)),
			WordBreak:            WordBreak(flag(raw >> segmentWordBreakShift & 0xF)),
			GeneralCategory:      GeneralCategory(raw & 0x1F),
			IndicConjunctBreak:   IndicConjunctBreak(flag(raw >> segmentIndicShift & 0x3)),
			ExtendedPictographic: raw>>segmentPictographicShift&1 == 1,
		}
	}
	return t
}()

// LookupSegmentProperties returns the properties of r that the segmenter
// reads. Each field is the result of the lookup of its property, such as
// [LookupLineBreak] and [LookupGeneralCategory]. The caller must not
// change the result.
func LookupSegmentProperties(r rune) *SegmentProperties {
	if uint32(r) > unicode.MaxRune {
		// The lookups give 0 for such a rune, and segmentRaw[0] is 0.
		return &segmentProps[0]
	}
	block := int(segmentBlocks[r>>segmentBlockBits])
	return &segmentProps[segmentIndex[block<<segmentBlockBits|int(r&(1<<segmentBlockBits-1))]]
}
