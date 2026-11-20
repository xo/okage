package harfbuzz

import (
	"fmt"
	"math"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

func (c *aatApplyContext) applyKernx(kerx font.Kernx, accelerators []kernxSubtableAccelerator) {
	var ret, seenCrossStream bool

	c.buffer.unsafeToConcat(0, maxInt)

	c.setupBufferGlyphSet()

	for i, st := range kerx {
		var reverse bool

		if !st.IsExtended && st.IsVariation() {
			continue
		}

		if c.buffer.Props.Direction.isHorizontal() != st.IsHorizontal() {
			continue
		}

		c.firstSet = accelerators[i].first_set
		c.secondSet = accelerators[i].second_set
		c.machineClassCache = accelerators[i].class_cache

		if !c.bufferIntersectsMachine() {
			if debugMode {
				fmt.Printf("AAT kerx : skipped subtable %d because no glyph matches\n", i)
			}
			continue
		}

		reverse = st.IsBackwards() != c.buffer.Props.Direction.isBackward()

		// okage: a subtable takes the length of the buffer in operations,
		// and the table stops when the buffer is in error, as in HarfBuzz
		// 15.1.0 (D73).
		if !c.chargeBufferPass() {
			break
		}

		if debugMode {
			fmt.Printf("AAT kerx : start subtable %d\n", i)
		}

		if !seenCrossStream && st.IsCrossStream() {
			/* Attach all glyphs into a chain. */
			seenCrossStream = true
			pos := c.buffer.Pos
			for i := range pos {
				pos[i].attachType = attachTypeCursive
				if c.buffer.Props.Direction.isForward() {
					pos[i].attachChain = -1
				} else {
					pos[i].attachChain = +1
				}
				/* We intentionally don't set HB_BUFFER_SCRATCH_FLAG_HAS_GPOS_ATTACHMENT,
				 * since there needs to be a non-zero attachment for post-positioning to
				 * be needed. */
			}
		}

		if reverse != c.bufferIsReversed {
			c.reverseBuffer()
		}

		applied := c.applyKerxSubtable(st)
		ret = ret || applied

		if debugMode {
			fmt.Printf("AAT kerx : end subtable %d\n", i)
			fmt.Println(c.buffer.Pos)
		}

		if c.buffer.inError {
			break
		}
	}

	if c.bufferIsReversed {
		c.reverseBuffer()
	}
}

func (c *aatApplyContext) applyKerxSubtable(st font.KernSubtable) bool {
	if debugMode {
		fmt.Printf("\tKERNX table %T\n", st.Data)
	}
	switch data := st.Data.(type) {
	case font.Kern0:
		if !c.plan.requestedKerning {
			return false
		}
		if st.IsBackwards() {
			return false
		}
		kern(kern0Accelerator{data, c}, st.IsCrossStream(), c.font, c.buffer, c.plan.kernMask, true)
	case font.Kern1:
		crossStream := st.IsCrossStream()
		if !c.plan.requestedKerning && !crossStream {
			return false
		}
		// okage: a subtable of kerx walks a history of glyphs, and reads
		// its tuple count, as in HarfBuzz 15.1.0 (D73).
		dc := driverContextKerx1{
			c:           c,
			table:       data,
			tupleCount:  st.TupleCount,
			extended:    st.IsExtended,
			crossStream: crossStream,
		}
		driver := newStateTableDriver(data.Machine, c.face)
		driver.drive(&dc, c)
	case font.Kern2:
		if !c.plan.requestedKerning {
			return false
		}
		if st.IsBackwards() {
			return false
		}
		kern(kern2Accelerator{data, c}, st.IsCrossStream(), c.font, c.buffer, c.plan.kernMask, true)
	case font.Kern3:
		if !c.plan.requestedKerning {
			return false
		}
		if st.IsBackwards() {
			return false
		}
		kern(data, st.IsCrossStream(), c.font, c.buffer, c.plan.kernMask, true)
	case font.Kern4:
		crossStream := st.IsCrossStream()
		if !c.plan.requestedKerning && !crossStream {
			return false
		}
		dc := driverContextKerx4{c: c, table: data, actionType: data.ActionType()}
		driver := newStateTableDriver(data.Machine, c.face)
		driver.drive(&dc, c)
	case font.Kern6:
		if !c.plan.requestedKerning {
			return false
		}
		if st.IsBackwards() {
			return false
		}
		kern(kern6Accelerator{data, c}, st.IsCrossStream(), c.font, c.buffer, c.plan.kernMask, true)
	}
	return true
}

// add a cache layout
type kern0Accelerator struct {
	table   font.Kern0
	context *aatApplyContext
}

func (k0 kern0Accelerator) KernPair(left, right GID) int16 {
	if !k0.context.firstSet.hasGlyph(left) || !k0.context.secondSet.hasGlyph(right) {
		return 0
	}
	return k0.table.KernPair(left, right)
}

// Kernx1 state entry flags
const (
	kerx1Push        = 0x8000 // If set, push this glyph on the kerning stack.
	kerx1DontAdvance = 0x4000 // If set, don't advance to the next glyph before going to the new state.
	// okage: HarfBuzz 15.1.0 ignores the flag Reset, 0x2000, as CoreText
	// does (D73).
	kern1Offset = 0x3FFF // Byte offset from beginning of subtable to the  value table for the glyphs on the kerning stack.
)

type driverContextKerx1 struct {
	c     *aatApplyContext
	table font.Kern1
	stack [8]int
	depth int
	// okage: stackPos is the next slot of the history of a subtable of
	// kerx, extended is true for a subtable of kerx, and tupleCount is
	// the tuple count of a subtable of kerx, as in HarfBuzz 15.1.0 (D73).
	stackPos    int
	tupleCount  int
	extended    bool
	crossStream bool
}

func (driverContextKerx1) inPlace() bool { return true }

func (dc driverContextKerx1) isActionable(entry tables.AATStateEntry) bool {
	return entry.AsKernxIndex() != 0xFFFF
}

// okage: transition follows HarfBuzz 15.1.0. For a subtable of kerx, the
// stack is a history of the last eight glyphs, and each list of actions
// walks it again from the last glyph. A slot that no glyph filled holds
// the first glyph. A list ends with the value 0xFFFF, and each action takes
// one operation and marks the glyphs unsafe to break. A subtable of kern
// keeps the older stack, which a list empties, and whose list ends with an
// odd value. go-text used the stack of kern for kerx too, and also
// emptied the stack on the flag Reset (D73).
func (dc *driverContextKerx1) transition(buffer *Buffer, driver stateTableDriver, entry tables.AATStateEntry) {
	flags := entry.Flags

	/* CoreText ignores the kerx Reset flag. */
	if flags&kerx1Push != 0 {
		if dc.extended {
			dc.stack[dc.stackPos] = buffer.idx
			dc.stackPos = (dc.stackPos + 1) % len(dc.stack)
			dc.depth = min(dc.depth+1, len(dc.stack))
		} else if dc.depth < len(dc.stack) {
			dc.stack[dc.depth] = buffer.idx
			dc.depth++
		} else {
			dc.depth = 0 /* Probably not what CoreText does, but better? */
		}
	}

	if !dc.isActionable(entry) || dc.depth == 0 {
		return
	}

	tupleCount := 1
	if dc.extended {
		tupleCount = max(1, dc.tupleCount)
	}

	var actions []int16
	if kernIdx := int(entry.AsKernxIndex()); kernIdx < len(dc.table.Values) {
		actions = dc.table.Values[kernIdx:]
	}
	if !dc.extended && len(actions) < tupleCount*dc.depth {
		dc.depth = 0
		return
	}

	kernMask := dc.c.plan.kernMask

	/* Contrary to the kerx spec's destructive pops, CoreText retains an
	 * eight-slot circular history and walks it anew for each action list. */
	cursor := dc.stackPos
	var last bool
	for !last && dc.depth != 0 {
		if dc.extended {
			if buffer.maxOps <= 0 {
				buffer.maxOps--
				buffer.inError = true
				return
			}
			buffer.maxOps--
			if len(actions) == 0 {
				dc.depth = 0
				return
			}
			/* kerx ends a list with a single 0xFFFF, without popping a glyph. */
			if actions[0] == -1 {
				break
			}
			if len(actions) < tupleCount {
				dc.depth = 0
				return
			}
		}

		var idx int
		if dc.extended {
			cursor = (cursor + len(dc.stack) - 1) % len(dc.stack)
			idx = dc.stack[cursor]
		} else {
			dc.depth--
			idx = dc.stack[dc.depth]
		}
		v := actions[0]
		actions = actions[tupleCount:]
		if idx >= len(buffer.Pos) {
			continue
		}
		if dc.extended {
			buffer.unsafeToBreak(idx, buffer.idx+1)
		}

		/* kern uses the low bit of the final value as the list terminator. */
		if !dc.extended {
			last = v&1 != 0
			v &= ^1
		}

		o := &buffer.Pos[idx]
		if buffer.Props.Direction.isHorizontal() {
			if dc.crossStream {
				/* The following flag is undocumented in the spec, but described
				 * in the 'kern' table example. */
				if v == -0x8000 {
					o.attachType = attachTypeNone
					o.attachChain = 0
					o.YOffset = 0
				} else if o.attachType != 0 {
					o.YOffset = saturateAdd(o.YOffset, dc.c.font.emScaleY(v))
					buffer.scratchFlags |= bsfHasGPOSAttachment
				}
			} else if buffer.Info[idx].Mask&kernMask != 0 {
				scaled := dc.c.font.emScaleX(v)
				o.XAdvance = saturateAdd(o.XAdvance, scaled)
				o.XOffset = saturateAdd(o.XOffset, scaled)
			}
		} else {
			if dc.crossStream {
				/* CoreText doesn't do crossStream kerning in vertical.  We do. */
				if v == -0x8000 {
					o.attachType = attachTypeNone
					o.attachChain = 0
					o.XOffset = 0
				} else if o.attachType != 0 {
					o.XOffset = saturateAdd(o.XOffset, dc.c.font.emScaleX(v))
					buffer.scratchFlags |= bsfHasGPOSAttachment
				}
			} else if buffer.Info[idx].Mask&kernMask != 0 {
				scaled := dc.c.font.emScaleY(v)
				o.YAdvance = saturateAdd(o.YAdvance, scaled)
				o.YOffset = saturateAdd(o.YOffset, scaled)
			}
		}
	}
}

// saturateAdd adds two positions, and clamps the sum to the range of a
// Position, as hb_saturate_add of HarfBuzz 15.1.0.
//
// okage: a subtable of format 1 of kerx adds its values with it (D73).
func saturateAdd(a, b Position) Position {
	return Position(min(max(int64(a)+int64(b), math.MinInt32), math.MaxInt32))
}

type kern2Accelerator struct {
	table   font.Kern2
	context *aatApplyContext
}

func (k2 kern2Accelerator) KernPair(left, right GID) int16 {
	if !k2.context.firstSet.hasGlyph(left) || !k2.context.secondSet.hasGlyph(right) {
		return 0
	}
	return k2.table.KernPair(left, right)
}

type driverContextKerx4 struct {
	c          *aatApplyContext
	table      font.Kern4
	mark       int
	markSet    bool
	actionType uint8
}

func (driverContextKerx4) inPlace() bool { return true }

func (driverContextKerx4) isActionable(entry tables.AATStateEntry) bool {
	return entry.AsKernxIndex() != 0xFFFF
}

func (dc *driverContextKerx4) transition(buffer *Buffer, driver stateTableDriver, entry tables.AATStateEntry) {
	ankrActionIndex := entry.AsKernxIndex()
	if dc.markSet && ankrActionIndex != 0xFFFF && buffer.idx < len(buffer.Pos) {
		o := buffer.curPos(0)
		switch dc.actionType {
		case 0: /* Control Point Actions.*/
			/* Indexed into glyph outline. */
			action := dc.table.Anchors.(tables.KerxAnchorControls).Anchors[ankrActionIndex]
			// okage: the two lookups of contour points take 2 × 256
			// operations, as in HarfBuzz 15.1.0 (D73).
			buffer.maxOps -= 2 * 256
			if buffer.maxOps <= 0 {
				return
			}

			markX, markY, okMark := dc.c.font.getGlyphContourPointForOrigin(dc.c.buffer.Info[dc.mark].Glyph,
				action.Mark, LeftToRight)
			currX, currY, okCurr := dc.c.font.getGlyphContourPointForOrigin(dc.c.buffer.cur(0).Glyph,
				action.Current, LeftToRight)
			if !okMark || !okCurr {
				return
			}

			o.XOffset = markX - currX
			o.YOffset = markY - currY

		case 1: /* Anchor Point Actions. */
			/* Indexed into 'ankr' table. */
			action := dc.table.Anchors.(tables.KerxAnchorAnchors).Anchors[ankrActionIndex]

			markAnchor := dc.c.ankrTable.GetAnchor(gID(dc.c.buffer.Info[dc.mark].Glyph), int(action.Mark))
			currAnchor := dc.c.ankrTable.GetAnchor(gID(dc.c.buffer.cur(0).Glyph), int(action.Current))

			o.XOffset = dc.c.font.emScaleX(markAnchor.X) - dc.c.font.emScaleX(currAnchor.X)
			o.YOffset = dc.c.font.emScaleY(markAnchor.Y) - dc.c.font.emScaleY(currAnchor.Y)

		case 2: /* Control Point Coordinate Actions. */
			action := dc.table.Anchors.(tables.KerxAnchorCoordinates).Anchors[ankrActionIndex]
			o.XOffset = dc.c.font.emScaleX(action.MarkX) - dc.c.font.emScaleX(action.CurrentX)
			o.YOffset = dc.c.font.emScaleY(action.MarkY) - dc.c.font.emScaleY(action.CurrentY)
		}
		o.attachType = attachTypeMark
		o.attachChain = int16(dc.mark - buffer.idx)
		if dc.c.bufferIsReversed {
			o.attachChain = -o.attachChain
		}
		buffer.scratchFlags |= bsfHasGPOSAttachment
	}

	const Mark = 0x8000 /* If set, remember this glyph as the marked glyph. */
	if entry.Flags&Mark != 0 {
		dc.markSet = true
		dc.mark = buffer.idx
	}
}

type kern6Accelerator struct {
	table   font.Kern6
	context *aatApplyContext
}

func (k6 kern6Accelerator) KernPair(left, right GID) int16 {
	if !k6.context.firstSet.hasGlyph(left) || !k6.context.secondSet.hasGlyph(right) {
		return 0
	}
	return k6.table.KernPair(left, right)
}

type kernxSubtableAccelerator struct {
	first_set   intSet
	second_set  intSet
	class_cache aatClassCache
}

func newKernxSubtableAccelerator(subtable font.KernSubtable) kernxSubtableAccelerator {
	var out kernxSubtableAccelerator
	out.class_cache.clear()
	switch st := subtable.Data.(type) {
	case font.Kern0:
		for _, pair := range st {
			out.first_set.add(uint32(pair.Left))
			out.second_set.add(uint32(pair.Right))
		}
	case font.Kern1:
		// okage: collect only the glyphs whose class can start an action,
		// as HarfBuzz 15.1.0 (D73).
		collectInitialGlyphs(&out.first_set, &st.Machine, driverContextKerx1{}, subtable.IsExtended)
		// collectLookupGlyphs (second_set, num_glyphs); // second_set is unused for machine kerning
	case font.Kern2:
		collectLookupGlyphs(&out.first_set, st.Left)
		collectLookupGlyphs(&out.second_set, st.Right)
	case font.Kern3:
		if L := len(st.LeftClass); L != 0 {
			out.first_set.addRange(0, uint32(L-1))
			out.second_set.addRange(0, uint32(L-1))
		}
	case font.Kern4:
		// okage: as for Kern1 (D73).
		collectInitialGlyphs(&out.first_set, &st.Machine, driverContextKerx4{}, true)
		// collectLookupGlyphs (second_set, num_glyphs); // second_set is unused for machine kerning
	case font.Kern6:
		collectLookupGlyphs(&out.first_set, st.Row)
		collectLookupGlyphs(&out.second_set, st.Column)
	}
	return out
}
