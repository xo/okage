package harfbuzz

// https://developer.apple.com/documentation/coretext/1508745-ctfontcreatewithgraphicsfont
const coretextDefaultFontSize = 12.

func (font *Font) getHTracking(track float32) Position {
	ptem := font.Ptem
	if ptem <= 0. {
		ptem = coretextDefaultFontSize
	}
	tr := font.face.Trak.Horiz.GetTracking(ptem, track)
	return font.emScalefX(tr)
}

func (font *Font) getVTracking(track float32) Position {
	ptem := font.Ptem
	if ptem <= 0. {
		ptem = coretextDefaultFontSize
	}
	tr := font.face.Trak.Vert.GetTracking(ptem, track)
	return font.emScalefY(tr)
}

// track defaults to 0
func (c *aatApplyContext) applyTrak(track float32) {
	ptem := c.font.Ptem
	if ptem <= 0. {
		// https://developer.apple.com/documentation/coretext/1508745-ctfontcreatewithgraphicsfont
		ptem = coretextDefaultFontSize
	}

	buffer := c.buffer
	if buffer.Props.Direction.isHorizontal() {
		advanceToAdd := c.font.getHTracking(track)
		iter, count := buffer.graphemesIterator()
		for start, _ := iter.next(); start < count; start, _ = iter.next() {
			// okage: the sum is clamped, as in HarfBuzz 15.2.0 (D97).
			buffer.Pos[start].XAdvance = saturateAdd(buffer.Pos[start].XAdvance, advanceToAdd)
		}

	} else {
		advanceToAdd := c.font.getVTracking(track)
		iter, count := buffer.graphemesIterator()
		for start, _ := iter.next(); start < count; start, _ = iter.next() {
			// okage: a vertical advance is negative, so the tracking is
			// subtracted, as in HarfBuzz 15.2.0. go-text added it, and a
			// positive tracking made the spacing smaller (D97).
			buffer.Pos[start].YAdvance = saturateSub(buffer.Pos[start].YAdvance, advanceToAdd)
		}
	}
}
