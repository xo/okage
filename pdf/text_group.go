// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

// This file ports krilla src/text/group.rs. The iterators GlyphSpanner and
// GlyphGrouper of upstream become functions that return the next span or
// the next group with the rest of the glyphs.

// glyphSpan is GlyphSpan in krilla src/text/group.rs. A span with
// hasActualText is GlyphSpan::Spanned, with its actual text, and other
// spans are GlyphSpan::Unspanned.
type glyphSpan struct {
	glyphs        []Glyph
	actualText    string
	hasActualText bool
}

// glyphSpanner is GlyphSpanner in krilla src/text/group.rs.
//
// In PDF, correspondences between glyphs and Unicode codepoints are
// expressed via a CMAP. In a CMAP, you can assign a sequence of Unicode
// codepoints to each glyph. There are two issues with this approach:
//   - How to deal with the fact that the same glyph might be assigned two
//     different codepoints in different contexts (i.e. space and NZWJ).
//   - How to deal with complex shaping scenarios, where there is not a
//     one-to-one or one-to-many correspondence between glyphs and
//     codepoints, but instead a many-to-one or many-to-many mapping.
//
// The answer to this is the ActualText feature of PDF, which allows to
// define some custom actual text for a number of glyphs, which overrides
// anything else. Unfortunately, this is seemingly only supported in Acrobat
// and Chrome, but it's the only proper way of addressing this issue.
//
// This is the task of the TextSpanner. Given a sequence of glyphs, it
// segments the sequence into subruns of glyphs that either do need to be
// wrapped in an actual text attribute, or not.
type glyphSpanner struct {
	slice                   []Glyph
	contextColor            ColorRGB
	reversed                bool
	forbidInvalidCodepoints bool
	fontContainer           *fontContainer
	text                    string
}

// newGlyphSpanner is GlyphSpanner::new in krilla src/text/group.rs.
func newGlyphSpanner(slice []Glyph, text string, reversed, forbidInvalidCodepoints bool, contextColor ColorRGB, fc *fontContainer) *glyphSpanner {
	return &glyphSpanner{
		slice:                   slice,
		contextColor:            contextColor,
		reversed:                reversed,
		forbidInvalidCodepoints: forbidInvalidCodepoints,
		text:                    text,
		fontContainer:           fc,
	}
}

// spannerGlyph is the function func of the impl Iterator for GlyphSpanner in
// krilla src/text/group.rs. It returns the range of the glyph and whether
// its codepoints are incompatible with the codepoints of an earlier glyph.
func spannerGlyph(g *Glyph, contextColor ColorRGB, previousRange *TextRange, forbidInvalidCodepoints bool, fc *fontContainer, text string) (TextRange, bool) {
	fg := fc.addGlyph(coloredGlyph{glyphID: g.GlyphID, contextColor: contextColor})
	pdfFont, _ := fc.getFromIdentifier(fg.id)

	r := g.TextRange
	glyphText := text[r.Start:r.End]
	codepoints, hasCodepoints := pdfFont.getCodepoints(fg.glyph)
	// Check if the glyph has already been assigned codepoints that don't
	// match the one we are seeing right now.
	incompatibleCodepoint := hasCodepoints && codepoints != glyphText

	// Only set the codepoint if there isn't a previous, different mapping.
	//
	// If we could set it, we only want to insert a codepoint if we are not
	// already building a spanned run (which is the case if the previous
	// range is the same). If we are building a spanned run, it means that
	// the glyphs are part of the same cluster, in which case only the first
	// glyph should be assigned the codepoint, while all other glyphs in the
	// same cluster should not be assigned anything. Otherwise, when copying
	// text from the PDF, we will get the same codepoint multiple times in
	// viewers that don't support ActualText.
	//
	// However, in case we are for example exporting to PDF/UA, every glyph
	// is required to have a valid codepoint mapping. So in this case, we
	// still add the codepoints to each glyph in the cluster, this will
	// result in worse copy-pasting in viewers that don't support ActualText.
	if !incompatibleCodepoint &&
		(previousRange == nil || *previousRange != r || forbidInvalidCodepoints) &&
		(!hasCodepoints || codepoints != glyphText) {
		pdfFont.setCodepoints(fg.glyph, glyphText, g.Location)
	}

	return r, incompatibleCodepoint
}

// next is Iterator::next for GlyphSpanner in krilla src/text/group.rs. It
// returns false at the end of the glyphs.
func (s *glyphSpanner) next() (glyphSpan, bool) {
	if len(s.slice) == 0 {
		return glyphSpan{}, false
	}
	// useSpan is the Option<bool> of upstream: 0 is None, 1 is false and 2
	// is true.
	const (
		spanNone = iota
		spanFalse
		spanTrue
	)
	useSpan := spanNone
	toSpan := func(b bool) int {
		if b {
			return spanTrue
		}
		return spanFalse
	}
	count := 1

	// Get the range of the first glyph, as well as whether it's
	// incompatible.
	firstRange, firstIncompatible := spannerGlyph(&s.slice[0], s.contextColor, nil, s.forbidInvalidCodepoints, s.fontContainer, s.text)

	prevRange := firstRange

	for i := 1; i < len(s.slice); i++ {
		nextRange, nextIncompatible := spannerGlyph(&s.slice[i], s.contextColor, &prevRange, s.forbidInvalidCodepoints, s.fontContainer, s.text)

		stop := false
		switch useSpan {
		// In this case, we just started and we are looking at the first two
		// glyphs. This decides whether the current run will be spanned, or
		// not.
		case spanNone:
			if prevRange == nextRange {
				// The two glyphs are in the same range, so we definitely
				// want this run to be spanned, and also want to include both
				// glyphs in that run.
				useSpan = spanTrue
			} else {
				// Else, whether we use a span depends on whether the first
				// glyph is incompatible.
				useSpan = toSpan(firstIncompatible)

				// If either the first glyph or the second glyph are
				// incompatible, they need to be in separate runs, since they
				// are not part of the same cluster.
				if firstIncompatible || nextIncompatible {
					stop = true
				}

				// If none are incompatible, then useSpan is false, and we can
				// also include the next glyph in that unspanned run.
			}
		// We are currently building a spanned range, and all glyphs are part
		// of the same cluster.
		case spanTrue:
			// If the next glyph is not part of the same cluster, terminate
			// the current span and don't include the next one.
			if prevRange != nextRange {
				stop = true
			}
		// We are currently building an unspanned range, meaning the glyphs
		// are not part of the same cluster.
		case spanFalse:
			// If the previous and next glyph are part of the same range this
			// means that they are part of the same cluster. This means that
			// the previous AND the next glyph should be part of the upcoming
			// spanned range, not the current one. To exclude the next glyph,
			// we need to do `count -= 1` before terminating.
			if prevRange == nextRange {
				count--
				stop = true
			} else if nextIncompatible {
				// If the next one is incompatible, terminate the current
				// run, since the next one needs to be spanned.
				stop = true
			}
		}
		if stop {
			break
		}

		prevRange = nextRange
		count++

		// PDF 1.7 § 14.8.2.3.3 says that /ReversedChars show strings shall
		// not contain interior space glyphs. Thus, we check whether we have
		// a glyph that is not the first one and which has a space as its
		// text mapping, and if so, stop after it. That a space is in the
		// same cluster as another codepoint should be exceedingly rare, so
		// we do a literal comparison against " " instead of a containment
		// check and prefer to continue otherwise.
		if s.reversed && count > 1 && s.text[prevRange.Start:prevRange.End] == " " {
			break
		}
	}

	// If we only had one glyph to begin with (and never entered the for
	// loop), then it should be spanned if its codepoint is incompatible.
	if count == 1 {
		useSpan = toSpan(firstIncompatible)
	}

	head, tail := s.slice[:count], s.slice[count:]
	s.slice = tail

	if useSpan == spanTrue {
		return glyphSpan{glyphs: head, actualText: s.text[firstRange.Start:firstRange.End], hasActualText: true}, true
	}
	return glyphSpan{glyphs: head}, true
}

// glyphGroup is GlyphGroup in krilla src/text/group.rs.
type glyphGroup struct {
	fontIdentifier fontIdentifier
	glyphs         []Glyph
	// yOffset is stored in normalized form (i.e. at a font size of 1).
	yOffset float32
	// yAdvance is stored in normalized form (i.e. at a font size of 1).
	yAdvance float32
}

// glyphProps is GlyphProps in krilla src/text/group.rs.
type glyphProps struct {
	fontIdentifier fontIdentifier
	yOffset        float32
	yAdvance       float32
}

// getGlyphProps is get_glyph_props in krilla src/text/group.rs.
func getGlyphProps(g *Glyph, contextColor ColorRGB, fc *fontContainer) glyphProps {
	// Safe because we've already added all glyphs in the text spanner.
	id, _ := fc.fontIdentifier(coloredGlyph{glyphID: g.GlyphID, contextColor: contextColor})

	return glyphProps{
		fontIdentifier: id,
		yOffset:        g.yOffset(1.0),
		yAdvance:       g.yAdvance(1.0),
	}
}

// nextGlyphGroup is Iterator::next for GlyphGrouper in krilla
// src/text/group.rs. It returns the next group and the rest of the glyphs.
//
// The GlyphGrouper further segments glyph runs (that already have been
// segmented by TextSpanner into subruns that can be encoded as one
// consecutive run in PDF. This is necessary because:
//   - The user provides a font for the whole glyph run, but in PDF, the font
//     might have to be switched if the glyph maps to a different Type3 font.
//   - The glyph contains a y_offset/y_advance, which cannot be expressed as
//     an adjustment and requires us to start a new run with a
//     transformation matrix that takes this adjustment into account.
func nextGlyphGroup(fc *fontContainer, contextColor ColorRGB, slice []Glyph) (glyphGroup, []Glyph) {
	// Guarantees: All glyphs in head have the font identifier that is given
	// in props, the same size and the same y offset.
	count := 1
	first := getGlyphProps(&slice[0], contextColor, fc)

	for i := 1; i < len(slice); i++ {
		temp := getGlyphProps(&slice[i], contextColor, fc)

		// If either of those is different, we need to start a new subrun.
		if first.fontIdentifier != temp.fontIdentifier ||
			first.yOffset != temp.yOffset ||
			first.yAdvance != 0.0 ||
			temp.yAdvance != 0.0 {
			break
		}

		count++
	}

	head, tail := slice[:count], slice[count:]
	return glyphGroup{
		fontIdentifier: first.fontIdentifier,
		glyphs:         head,
		yOffset:        first.yOffset,
		yAdvance:       first.yAdvance,
	}, tail
}

// useTextSpanner is use_text_spanner in krilla src/text/group.rs.
func useTextSpanner(glyphs []Glyph, text string, contextColor ColorRGB, fc *fontContainer) bool {
	if len(glyphs) == 0 {
		return false
	}

	doTextSpan := false
	// We might need to group glyphs either if they are mapped to different
	// PDF fonts (which can only happen for Type3 fonts, not CID fonts), or
	// if a glyph has a different y/x offset. Note that it's of course
	// possible that we always use the same Type3 font, but we need to be
	// conservative here.

	checkSingle := func(g *Glyph) {
		// As soon as we know that the glyph run requires a text span, we do
		// not insert any codepoints anymore, because otherwise we might
		// unnecessarily pollute the cmap with entries that wouldn't be
		// necessary. The text spanner will then iterate over the all glyphs
		// again and thus take care of inserting cmap entries for any
		// remaining glyphs we ignore now. The only reason we keep going and
		// don't early abort is in order to fully check the
		// do_glyph_grouping property.
		if !doTextSpan {
			checkTextSpanProp(g, text, contextColor, fc, &doTextSpan)
		}
	}

	for i := 0; i+1 < len(glyphs); i++ {
		prev, next := &glyphs[i], &glyphs[i+1]

		// If two glyphs are part of the same cluster, we definitely need a
		// text span.
		doTextSpan = doTextSpan || prev.TextRange == next.TextRange

		checkSingle(prev)
	}

	// Since windows checks groups of two, we need to manually check the
	// last glyph.
	checkSingle(&glyphs[len(glyphs)-1])

	return doTextSpan
}

// checkTextSpanProp is check_text_span_prop in krilla src/text/group.rs.
func checkTextSpanProp(g *Glyph, text string, contextColor ColorRGB, fc *fontContainer, doTextSpan *bool) {
	fg := fc.addGlyph(coloredGlyph{glyphID: g.GlyphID, contextColor: contextColor})
	pdfFont, _ := fc.getFromIdentifier(fg.id)

	r := g.TextRange
	glyphText := text[r.Start:r.End]
	codepoints, ok := pdfFont.getCodepoints(fg.glyph)

	switch {
	case ok && codepoints != glyphText:
		// If there already exists an entry, then the codepoints are
		// invalid, and we will have to use a text span.
		*doTextSpan = true
	case !ok:
		pdfFont.setCodepoints(fg.glyph, glyphText, g.Location)
	}
}

// requiresCodepointMappings is Validators::requires_codepoint_mappings in
// krilla src/configure/validate.rs. No validator requires them.
func (validators) requiresCodepointMappings() bool {
	return false
}
