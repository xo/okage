// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"iter"
	"slices"
)

// This file ports subsetter src/glyf.rs.
//
// The `glyf` table contains the main description of the glyphs. In order to
// subset it, there are 5 things we need to do:
//
//  1. We need to form the glyph closure. Glyphs can reference other glyphs,
//     meaning that if a user for example requests the glyph 1, and this glyph
//     references the glyph 2, then we need to include both of them in our
//     subset.
//  2. We need to remove glyph descriptions that are not needed for the
//     subset, and reorder the existing glyph descriptions to match the order
//     defined by the remapper.
//  3. For component glyphs, we need to rewrite their description so that
//     they reference the new glyph ID of the glyphs they reference.
//  4. We need to calculate which format to use in the `loca` table.
//  5. We need to update the `loca` table itself with the new offsets.

// closure is closure in subsetter src/glyf.rs. It forms the glyph closure of
// all glyphs in the remapper.
//
// The port differs from upstream in three ways, and it remaps the same glyphs
// in the same order:
//
//  1. Upstream panics when the closure adds a glyph to a remapper that
//     already holds 65535 glyphs. The port returns okage.ErrSubsetOverflow.
//  2. Upstream reads the components of a glyph each time it pops the glyph
//     from the stack. A glyph can be on the stack many times, because
//     upstream pushes each component that is not remapped yet. The port
//     reads a glyph once, and skips it when it pops it again. The second
//     read of upstream cannot push a glyph: every component of the glyph
//     was remapped before the stack gets back to it. It cannot fail either,
//     because the first read of the same data did not fail.
//  3. When the stack holds more than twice the entries that it can need, the
//     port removes each entry that is processed, and each entry that has a
//     copy nearer to the top. Upstream skips such an entry when it pops it.
//
// So the memory of the closure has a bound that does not depend on the
// glyphs of the font, and a font of a few kilobytes cannot make it run for a
// long time. A composite glyph whose components refer to it again makes
// upstream push its components many times.
func closure(face *face, glyphRemapper *GlyphRemapper) error {
	table, ok := newTableGlyf(face)
	if !ok {
		return errMalformedFont
	}

	processGlyphs := slices.Collect(glyphRemapper.RemappedGids())

	// processed holds a bit for each glyph that the loop read. limit is the
	// length of the stack at which the loop removes the entries that it
	// skips.
	var processed glyphSet
	const minLimit = 1 << 17
	limit := minLimit

	for len(processGlyphs) > 0 {
		glyph := processGlyphs[len(processGlyphs)-1]
		processGlyphs = processGlyphs[:len(processGlyphs)-1]

		if _, ok := glyphRemapper.r.tryRemap(glyph); !ok {
			return errOverflow
		}
		if processed.has(glyph) {
			continue
		}
		processed.add(glyph)

		glyphData, ok := table.glyphData(glyph)
		if !ok {
			continue
		}

		if len(glyphData) == 0 {
			continue
		}

		r := newReader(glyphData)
		numContours, ok := r.readI16()
		if !ok {
			return errMalformedFont
		}

		// If we have a composite glyph, add its components to the closure.
		if numContours < 0 {
			components, ok := componentGlyphs(glyphData)
			if !ok {
				return errMalformedFont
			}
			for component := range components {
				if _, ok := glyphRemapper.Get(component); !ok {
					processGlyphs = append(processGlyphs, component)
				}
			}
			if len(processGlyphs) > limit {
				processGlyphs = compactGlyphStack(processGlyphs, &processed)
				limit = max(minLimit, 2*len(processGlyphs))
			}
		}
	}

	return nil
}

// compactGlyphStack removes from the stack of closure each glyph that is
// processed, and each glyph that has a copy nearer to the top. It keeps the
// order of the other entries. Upstream has no such function.
func compactGlyphStack(stack []uint16, processed *glyphSet) []uint16 {
	var seen glyphSet
	kept := make([]uint16, 0, 1<<16)
	for _, g := range slices.Backward(stack) {
		if processed.has(g) || seen.has(g) {
			continue
		}
		seen.add(g)
		kept = append(kept, g)
	}
	slices.Reverse(kept)
	return kept
}

// glyphSet is a set of glyph IDs, with one bit for each ID.
type glyphSet [1 << 16 / 64]uint64

// has tells whether the set holds g.
func (s *glyphSet) has(g uint16) bool {
	return s[g/64]&(1<<(g%64)) != 0
}

// add puts g in the set.
func (s *glyphSet) add(g uint16) {
	s[g/64] |= 1 << (g % 64)
}

// subsetGlyf is subset in subsetter src/glyf.rs.
func subsetGlyf(ctx *context) error {
	table, ok := newTableGlyf(ctx.face)
	if !ok {
		return errMalformedFont
	}

	maxpData := newMaxpData()
	hmtxData := make([]hmtxMetric, 0, int(ctx.mapper.NumGids()))

	err := subsetWith(ctx, func(oldGid uint16, ctx *context) ([]byte, error) {
		if !ctx.interjector.isSkrifa() {
			data, ok := table.glyphData(oldGid)
			if !ok {
				return nil, errMalformedFont
			}
			return data, nil
		}
		if interjectGlyph == nil {
			return nil, notPorted("the instance of a variable font")
		}
		advance, lsb, data, ok := interjectGlyph(ctx.interjector.skrifa, &maxpData, oldGid)
		if !ok {
			return nil, errMalformedFont
		}
		hmtxData = append(hmtxData, hmtxMetric{advance, lsb})
		return data, nil
	})
	if err != nil {
		return err
	}

	if ctx.interjector.isSkrifa() {
		ctx.customMaxpData = &maxpData
		ctx.customHmtxData = hmtxData
	}

	return nil
}

// subsetWith is subset_with in subsetter src/glyf.rs. It writes the glyf and
// the loca tables, with the description of each glyph of the remapper that
// glyphDataFn gives.
func subsetWith(ctx *context, glyphDataFn func(oldGid uint16, ctx *context) ([]byte, error)) error {
	subsettedEntries, err := subsetGlyfEntries(ctx, glyphDataFn)
	if err != nil {
		return err
	}

	subGlyf := newWriter()
	subLoca := newWriter()

	writeOffset := func(offset int) {
		if ctx.longLoca {
			subLoca.writeU32(uint32(offset))
		} else {
			subLoca.writeU16(uint16(offset / 2))
		}
	}

	for _, entry := range subsettedEntries {
		writeOffset(subGlyf.len())
		subGlyf.extend(entry)

		if !ctx.longLoca {
			subGlyf.align(2)
		}
	}

	// Write the final offset.
	writeOffset(subGlyf.len())

	ctx.push(tagLOCA, subLoca.finish())
	ctx.push(tagGLYF, subGlyf.finish())

	return nil
}

// tableGlyf is Table in subsetter src/glyf.rs. A glyf + loca table.
type tableGlyf struct {
	loca []byte
	glyf []byte
	long bool
}

// newTableGlyf is Table::new in subsetter src/glyf.rs.
func newTableGlyf(face *face) (tableGlyf, bool) {
	loca, ok := face.table(tagLOCA)
	if !ok {
		return tableGlyf{}, false
	}
	glyf, ok := face.table(tagGLYF)
	if !ok {
		return tableGlyf{}, false
	}
	head, ok := face.table(tagHEAD)
	if !ok {
		return tableGlyf{}, false
	}

	r := newReaderAt(head, 50)
	long, ok := r.readI16()
	if !ok {
		return tableGlyf{}, false
	}
	return tableGlyf{loca: loca, glyf: glyf, long: long != 0}, true
}

// glyphData is Table::glyph_data in subsetter src/glyf.rs. It returns the
// description of a glyph, and false when the loca table does not hold the
// glyph or gives a range out of the glyf table.
func (t tableGlyf) glyphData(id uint16) ([]byte, bool) {
	readOffset := func(n int) (int, bool) {
		if t.long {
			r := newReaderAt(t.loca, 4*n)
			v, ok := r.readU32()
			// A u32 offset larger than the glyf table cannot give a glyph,
			// and on a machine of 32 bits it does not fit in an int.
			if !ok || uint64(v) > uint64(len(t.glyf)) {
				return len(t.glyf) + 1, ok
			}
			return int(v), true
		}
		r := newReaderAt(t.loca, 2*n)
		v, ok := r.readU16()
		return 2 * int(v), ok
	}

	from, ok := readOffset(int(id))
	if !ok {
		return nil, false
	}
	to, ok := readOffset(int(id) + 1)
	if !ok {
		return nil, false
	}
	if from > to || to > len(t.glyf) {
		return nil, false
	}
	return t.glyf[from:to], true
}

// subsetGlyfEntries is subset_glyf_entries in subsetter src/glyf.rs.
func subsetGlyfEntries(ctx *context, glyphDataFn func(oldGid uint16, ctx *context) ([]byte, error)) ([][]byte, error) {
	size := 0
	glyfEntries := make([][]byte, 0, int(ctx.mapper.NumGids()))

	for oldGid := range ctx.mapper.RemappedGids() {
		glyphData, err := glyphDataFn(oldGid, ctx)
		if err != nil {
			return nil, err
		}

		// Empty glyph.
		if len(glyphData) == 0 {
			glyfEntries = append(glyfEntries, glyphData)
			continue
		}

		r := newReader(glyphData)
		numContours, ok := r.readI16()
		if !ok {
			return nil, errMalformedFont
		}

		// okage: upstream has no limit. Many glyph IDs can point at one
		// large glyph, and subsetWith copies it once for each ID. Stop
		// before the copy, and before the remap that copies a composite
		// glyph, which makes the glyph no longer (D74). size is at most 1
		// more than maxGlyfSize, so the subtraction cannot overflow.
		if len(glyphData) > ctx.maxGlyfSize-size {
			return nil, errTooLarge
		}

		if numContours < 0 {
			if glyphData, err = remapComponentGlyph(ctx.mapper, glyphData); err != nil {
				return nil, err
			}
		}
		// Simple glyphs don't need any subsetting.

		length := len(glyphData)
		length += length % 2
		size += length

		glyfEntries = append(glyfEntries, glyphData)
	}

	// Decide on the loca format.
	ctx.longLoca = size > 2*0xFFFF

	return glyfEntries, nil
}

// The flags of a component of a composite glyph, which remapComponentGlyph
// and componentGlyphs read.
const (
	arg1And2AreWords     uint16 = 0x0001
	weHaveAScale         uint16 = 0x0008
	moreComponents       uint16 = 0x0020
	weHaveAnXAndYScale   uint16 = 0x0040
	weHaveATwoByTwo      uint16 = 0x0080
	weHaveInstructions   uint16 = 0x0100
	compositeHeaderSize         = 10
	compositeHeaderCount        = compositeHeaderSize / 2
)

// remapComponentGlyph is remap_component_glyph in subsetter src/glyf.rs. It
// rewrites a composite glyph with the new IDs of its components.
func remapComponentGlyph(mapper *GlyphRemapper, data []byte) ([]byte, error) {
	r := newReader(data)
	w := newWriterWithCapacity(len(data))

	// number of contours
	// xMin, yMin, xMax, yMax
	for range compositeHeaderCount {
		v, ok := r.readI16()
		if !ok {
			return nil, errMalformedFont
		}
		w.writeI16(v)
	}

	copyF2Dot14 := func(n int) bool {
		for range n {
			v, ok := readF2Dot14(&r)
			if !ok {
				return false
			}
			v.write(w)
		}
		return true
	}

	for {
		flags, ok := r.readU16()
		if !ok {
			return nil, errMalformedFont
		}
		w.writeU16(flags)
		oldComponent, ok := r.readU16()
		if !ok {
			return nil, errMalformedFont
		}
		newComponent, ok := mapper.Get(oldComponent)
		if !ok {
			return nil, errMalformedFont
		}
		w.writeU16(newComponent)

		if flags&arg1And2AreWords != 0 {
			for range 2 {
				v, ok := r.readI16()
				if !ok {
					return nil, errMalformedFont
				}
				w.writeI16(v)
			}
		} else {
			v, ok := r.readU16()
			if !ok {
				return nil, errMalformedFont
			}
			w.writeU16(v)
		}

		var scales int
		switch {
		case flags&weHaveAScale != 0:
			scales = 1
		case flags&weHaveAnXAndYScale != 0:
			scales = 2
		case flags&weHaveATwoByTwo != 0:
			scales = 4
		}
		if !copyF2Dot14(scales) {
			return nil, errMalformedFont
		}

		done := flags&moreComponents == 0

		if done {
			if flags&weHaveInstructions != 0 {
				tail, ok := r.tail()
				if !ok {
					return nil, errMalformedFont
				}
				w.extend(tail)
			}

			break
		}
	}

	return w.finish(), nil
}

// componentGlyphs is component_glyphs in subsetter src/glyf.rs. It returns
// an iterator over the component glyphs of a glyph, and false when the glyph
// is shorter than its header.
func componentGlyphs(glyphData []byte) (iter.Seq[uint16], bool) {
	r := newReader(glyphData)

	// Number of contours
	// xMin, yMin, xMax, yMax
	for range compositeHeaderCount {
		if _, ok := r.readI16(); !ok {
			return nil, false
		}
	}

	return func(yield func(uint16) bool) {
		for {
			flags, ok := r.readU16()
			if !ok {
				return
			}
			component, ok := r.readU16()
			if !ok {
				return
			}

			if flags&arg1And2AreWords != 0 {
				r.readI16()
				r.readI16()
			} else {
				r.readU16()
			}

			switch {
			case flags&weHaveAScale != 0:
				readF2Dot14(&r)
			case flags&weHaveAnXAndYScale != 0:
				readF2Dot14(&r)
				readF2Dot14(&r)
			case flags&weHaveATwoByTwo != 0:
				readF2Dot14(&r)
				readF2Dot14(&r)
				readF2Dot14(&r)
				readF2Dot14(&r)
			}

			done := flags&moreComponents == 0
			if !yield(component) || done {
				return
			}
		}
	}, true
}
