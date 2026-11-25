package harfbuzz

import fontP "github.com/xo/okage/typesetting/font"

func simpleKern(kernTable fontP.Kernx) fontP.SimpleKerns {
	for _, subtable := range kernTable {
		if simple, ok := subtable.Data.(fontP.SimpleKerns); ok {
			return simple
		}
	}
	return nil
}

func kern(driver fontP.SimpleKerns, crossStream bool, font *Font, buffer *Buffer, kernMask GlyphMask, scale bool) {
	buffer.unsafeToConcat(0, maxInt)

	var c otApplyContext

	c.reset(1, font, buffer)
	c.setLookupMask(kernMask)
	c.setLookupProps(uint32(otIgnoreMarks))
	skippyIter := &c.iterInput
	horizontal := buffer.Props.Direction.isHorizontal()
	info := buffer.Info
	pos := buffer.Pos
	for idx := 0; idx < len(pos); {
		if info[idx].Mask&kernMask == 0 {
			idx++
			continue
		}

		i := idx
		j := i + 1
		// okage: as in HarfBuzz 15.2.0, a next glyph that the kerning does
		// not skip is the pair at once, without the skipping iterator,
		// which takes operations for the glyphs that it skips (D97).
		if j == len(pos) || info[j].Mask&kernMask == 0 || info[j].isMark() || info[j].isDefaultIgnorable() {
			skippyIter.resetFast(i)
			if ok, unsafeTo := skippyIter.next(); !ok {
				idx = unsafeTo
				continue
			}
			j = skippyIter.idx
		}

		rawKern := driver.KernPair(info[i].Glyph, info[j].Glyph)
		kern := Position(rawKern)

		if rawKern == 0 {
			goto skip
		}

		// okage: as in HarfBuzz 15.2.0, the value -0x8000 of a cross-stream
		// subtable resets the offset across the line and the attachment,
		// a value across the line is scaled on the axis across the line,
		// and the sums are clamped. go-text took -0x8000 as an offset, and
		// scaled a value across the line on the axis of the line (D97).
		if crossStream && rawKern == -0x8000 {
			pos[j].attachType = attachTypeNone
			pos[j].attachChain = 0
			if horizontal {
				pos[j].YOffset = 0
			} else {
				pos[j].XOffset = 0
			}
		} else if horizontal {
			if crossStream {
				if scale {
					kern = font.emScaleY(rawKern)
				}
				pos[j].YOffset = kern
				buffer.scratchFlags |= bsfHasGPOSAttachment
			} else {
				if scale {
					kern = font.emScaleX(rawKern)
				}
				kern1 := kern >> 1
				kern2 := kern - kern1
				pos[i].XAdvance = saturateAdd(pos[i].XAdvance, kern1)
				pos[j].XAdvance = saturateAdd(pos[j].XAdvance, kern2)
				pos[j].XOffset = saturateAdd(pos[j].XOffset, kern2)
			}
		} else {
			if crossStream {
				if scale {
					kern = font.emScaleX(rawKern)
				}
				pos[j].XOffset = kern
				buffer.scratchFlags |= bsfHasGPOSAttachment
			} else {
				if scale {
					kern = font.emScaleY(rawKern)
				}
				kern1 := kern >> 1
				kern2 := kern - kern1
				pos[i].YAdvance = saturateAdd(pos[i].YAdvance, kern1)
				pos[j].YAdvance = saturateAdd(pos[j].YAdvance, kern2)
				pos[j].YOffset = saturateAdd(pos[j].YOffset, kern2)
			}
		}

		buffer.unsafeToBreak(i, j+1)

	skip:
		idx = j
	}
}

func (sp *otShapePlan) otApplyFallbackKern(font *Font, buffer *Buffer) {
	reverse := buffer.Props.Direction.isBackward()

	if reverse {
		buffer.Reverse()
	}

	if driver := simpleKern(font.face.Kern); driver != nil {
		kern(driver, false, font, buffer, sp.kernMask, false)
	}

	if reverse {
		buffer.Reverse()
	}
}
