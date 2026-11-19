// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"encoding/binary"
	"slices"

	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"

	"github.com/xo/okage/path"
)

// This file ports krilla src/text/glyph/bitmap.rs, with the part of skrifa
// 0.44 src/bitmap.rs that it calls: BitmapStrikes::glyph_for_size with the
// size unscaled, on the tables sbix and CBDT. skrifa also reads the table
// EBDT, whose glyphs are never PNG data, so krilla draws none of them. The
// port leaves the strikes of EBDT out, with the same result.

// hasBitmapData is has_bitmap_data in krilla src/text/glyph/bitmap.rs.
func hasBitmapData(font *Font, glyph GlyphID) bool {
	// We only support PNG.
	g, ok := bitmapGlyphOf(font, glyph)
	return ok && g.kind == bitmapDataPNG
}

// drawBitmapGlyph is draw_glyph in krilla src/text/glyph/bitmap.rs. It
// returns false where upstream returns None.
//
// Draw a bitmap-based glyph on a surface.
func drawBitmapGlyph(font *Font, glyph GlyphID, surface *Surface) bool {
	upem := font.UnitsPerEm()

	g, ok := bitmapGlyphOf(font, glyph)
	if !ok || g.kind != bitmapDataPNG {
		return false
	}

	img, err := ImageFromPNG(g.data, false)
	if err != nil {
		return false
	}
	width, height := img.Size()
	size, ok := path.SizeFromWH(float32(width), float32(height))
	if !ok {
		return false
	}

	// Adapted from vello.
	outerX, outerY := g.bearingX, -g.bearingY
	if font.isAppleColorEmoji() {
		outerX, outerY = 0.0, upem/8.0
	}
	transform := path.TransformFromTranslate(outerX, outerY).
		PreConcat(path.TransformFromScale(upem/g.ppemX, upem/g.ppemY)).
		PreConcat(path.TransformFromTranslate(g.innerBearingX, -g.innerBearingY))

	if g.bottomLeft {
		transform = transform.PreConcat(path.TransformFromTranslate(0.0, -float32(height)))
	}

	surface.PushTransform(transform)
	surface.DrawImage(img, size)
	surface.Pop()

	return true
}

// bitmapFormat is BitmapFormat of skrifa src/bitmap.rs, for the formats
// that the port reads.
type bitmapFormat uint8

// The values of bitmapFormat. bitmapFormatNone is a font without strikes.
const (
	bitmapFormatNone bitmapFormat = iota
	bitmapFormatSbix
	bitmapFormatCBDT
)

// bitmapDataKind is the case of BitmapData of skrifa src/bitmap.rs:
// bitmapDataPNG for BitmapData::Png, and bitmapDataOther for the BGRA and
// the mask data, which krilla does not draw.
type bitmapDataKind uint8

// The values of bitmapDataKind.
const (
	bitmapDataOther bitmapDataKind = iota
	bitmapDataPNG
)

// bitmapGlyph is BitmapGlyph of skrifa src/bitmap.rs, with the fields that
// krilla reads.
type bitmapGlyph struct {
	kind          bitmapDataKind
	data          []byte
	bearingX      float32
	bearingY      float32
	innerBearingX float32
	innerBearingY float32
	ppemX         float32
	ppemY         float32
	// bottomLeft is Origin::BottomLeft. Otherwise the origin is
	// Origin::TopLeft.
	bottomLeft bool
}

// bitmapStrike is BitmapStrike of skrifa src/bitmap.rs: its size and the
// lookup of a glyph in it.
type bitmapStrike struct {
	ppem   float32
	lookup func(glyph GlyphID) (bitmapGlyph, bool)
}

// bitmapStrikesFormat is BitmapStrikes::format of skrifa src/bitmap.rs.
func bitmapStrikesFormat(font *Font) bitmapFormat {
	format, _ := bitmapStrikes(font)
	return format
}

// bitmapStrikes is BitmapStrikes::new of skrifa src/bitmap.rs, with
// BitmapStrikes::iter. It prefers the table sbix to the table CBDT.
func bitmapStrikes(font *Font) (bitmapFormat, []bitmapStrike) {
	font.strikesOnce.Do(func() {
		font.strikesFormat, font.strikes = readBitmapStrikes(font)
	})
	return font.strikesFormat, font.strikes
}

// readBitmapStrikes reads the strikes of bitmapStrikes.
func readBitmapStrikes(font *Font) (bitmapFormat, []bitmapStrike) {
	numGlyphs := 0
	if maxp := font.table("maxp"); len(maxp) >= 6 {
		numGlyphs = int(binary.BigEndian.Uint16(maxp[4:]))
	}
	if raw := font.table("sbix"); raw != nil {
		if sbix, _, err := tables.ParseSbix(raw, numGlyphs); err == nil {
			strikes := make([]bitmapStrike, 0, len(sbix.Strikes))
			for _, st := range sbix.Strikes {
				strikes = append(strikes, bitmapStrike{
					ppem:   float32(st.Ppem),
					lookup: func(glyph GlyphID) (bitmapGlyph, bool) { return sbixGlyph(font, st, glyph) },
				})
			}
			return bitmapFormatSbix, strikes
		}
	}
	cblcRaw, cbdt := font.table("CBLC"), font.table("CBDT")
	if cblcRaw != nil && cbdt != nil {
		if cblc, _, err := tables.ParseCBLC(cblcRaw); err == nil {
			strikes := make([]bitmapStrike, 0, len(cblc.BitmapSizes))
			for i, size := range cblc.BitmapSizes {
				// typesetting keeps the bit depth private. It is the byte 46
				// of the record of the size, which starts at 8.
				var bitDepth uint8
				if off := 8 + 48*i + 46; off < len(cblcRaw) {
					bitDepth = cblcRaw[off]
				}
				subtables := cblc.IndexSubTables[i]
				strikes = append(strikes, bitmapStrike{
					ppem: float32(size.PpemY),
					lookup: func(glyph GlyphID) (bitmapGlyph, bool) {
						return cbdtGlyph(size, bitDepth, subtables, cbdt, glyph)
					},
				})
			}
			return bitmapFormatCBDT, strikes
		}
	}
	return bitmapFormatNone, nil
}

// bitmapGlyphOf is get_bitmap_glyph in krilla src/text/glyph/bitmap.rs: the
// glyph of BitmapStrikes::glyph_for_size of skrifa with the size unscaled,
// which returns the glyph of the largest strike that has it.
func bitmapGlyphOf(font *Font, glyph GlyphID) (bitmapGlyph, bool) {
	if glyph > 0xFFFF {
		return bitmapGlyph{}, false
	}
	_, strikes := bitmapStrikes(font)
	var best bitmapGlyph
	hasBest := false
	for _, entry := range strikes {
		if !hasBest {
			best, hasBest = entry.lookup(glyph)
			continue
		}
		// The size of an unscaled request is f32::MAX, so a larger strike
		// wins.
		if entry.ppem > best.ppemY {
			if g, ok := entry.lookup(glyph); ok {
				best = g
			}
		}
	}
	return best, hasBest
}

// sbixGlyph is the case StrikeKind::Sbix of BitmapStrike::get of skrifa
// src/bitmap.rs.
func sbixGlyph(font *Font, strike tables.Strike, glyph GlyphID) (bitmapGlyph, bool) {
	if int(glyph) >= len(strike.GlyphDatas) {
		return bitmapGlyph{}, false
	}
	entry := strike.GlyphDatas[glyph]
	if len(entry.Data) == 0 || entry.GraphicType != ot.MustNewTag("png ") {
		return bitmapGlyph{}, false
	}
	// Note that this calculation does not entirely correspond to the
	// description in the specification, but it's implemented this way in
	// Skia, the implementation of which has been tested against behavior in
	// CoreText.
	yMin := glyphBoundsYMin(font, glyph)
	lsb := leftSideBearing(font, glyph)
	ppem := float32(strike.Ppem)
	// PNG format: 8 byte header, IHDR chunk (4 byte length, 4 byte chunk
	// type), width, height.
	if len(entry.Data) < 24 {
		return bitmapGlyph{}, false
	}
	return bitmapGlyph{
		kind:          bitmapDataPNG,
		data:          entry.Data,
		bearingX:      lsb,
		bearingY:      yMin,
		innerBearingX: float32(entry.OriginOffsetX),
		innerBearingY: float32(entry.OriginOffsetY),
		ppemX:         ppem,
		ppemY:         ppem,
		bottomLeft:    true,
	}, true
}

// cbdtGlyph is the case StrikeKind::Cbdt of BitmapStrike::get of skrifa
// src/bitmap.rs, with BitmapGlyph::from_bdt.
func cbdtGlyph(size tables.BitmapSize, bitDepth uint8, subtables []tables.BitmapSubtable, cbdt []byte, glyph GlyphID) (bitmapGlyph, bool) {
	gid := tables.GlyphID(glyph)
	for _, st := range subtables {
		if gid < st.FirstGlyph || gid > st.LastGlyph {
			continue
		}
		var start, end int
		var bigMetrics *tables.BigGlyphMetrics
		base := int(st.ImageDataOffset)
		index := int(gid - st.FirstGlyph)
		switch data := st.IndexData.(type) {
		case tables.IndexData1:
			if index+1 >= len(data.SbitOffsets) {
				return bitmapGlyph{}, false
			}
			start, end = base+int(data.SbitOffsets[index]), base+int(data.SbitOffsets[index+1])
		case tables.IndexData3:
			if index+1 >= len(data.SbitOffsets) {
				return bitmapGlyph{}, false
			}
			start, end = base+int(data.SbitOffsets[index]), base+int(data.SbitOffsets[index+1])
		case tables.IndexData2:
			start = base + int(data.ImageSize)*index
			end = start + int(data.ImageSize)
			bigMetrics = &data.BigMetrics
		case tables.IndexData4:
			i := slices.IndexFunc(data.GlyphArray, func(p tables.GlyphIdOffsetPair) bool { return p.GlyphID == gid })
			if i < 0 || i+1 >= len(data.GlyphArray) {
				return bitmapGlyph{}, false
			}
			start, end = base+int(data.GlyphArray[i].SbitOffset), base+int(data.GlyphArray[i+1].SbitOffset)
		case tables.IndexData5:
			i := slices.Index(data.GlyphIdArray, gid)
			if i < 0 {
				return bitmapGlyph{}, false
			}
			start = base + int(data.ImageSize)*i
			end = start + int(data.ImageSize)
			bigMetrics = &data.BigMetrics
		default:
			return bitmapGlyph{}, false
		}
		if start < 0 || end > len(cbdt) || start > end {
			return bitmapGlyph{}, false
		}
		b := cbdt[start:end]

		g := bitmapGlyph{ppemX: float32(size.PpemX), ppemY: float32(size.PpemY)}
		readPNG := func(b []byte) ([]byte, bool) {
			if len(b) < 4 {
				return nil, false
			}
			n := int(binary.BigEndian.Uint32(b))
			if n < 0 || len(b)-4 < n {
				return nil, false
			}
			return b[4 : 4+n], true
		}
		var ok bool
		switch st.ImageFormat {
		case 17:
			if len(b) < 5 {
				return bitmapGlyph{}, false
			}
			g.innerBearingX = float32(int8(b[2]))
			g.innerBearingY = float32(int8(b[3]))
			g.data, ok = readPNG(b[5:])
		case 18:
			if len(b) < 8 {
				return bitmapGlyph{}, false
			}
			g.innerBearingX = float32(int8(b[2]))
			g.innerBearingY = float32(int8(b[3]))
			g.data, ok = readPNG(b[8:])
		case 19:
			if bigMetrics == nil {
				return bitmapGlyph{}, false
			}
			g.innerBearingX = float32(bigMetrics.BearingX)
			g.innerBearingY = float32(bigMetrics.BearingY)
			g.data, ok = readPNG(b)
		default:
			// The other formats hold BGRA or mask data, which krilla does
			// not draw. skrifa still returns a glyph for them when the bit
			// depth is valid, and it can win the choice of the largest
			// strike.
			switch bitDepth {
			case 1, 2, 4, 8, 32:
				if st.ImageFormat >= 1 && st.ImageFormat <= 7 && st.ImageFormat != 3 && st.ImageFormat != 4 {
					return bitmapGlyph{kind: bitmapDataOther, ppemX: g.ppemX, ppemY: g.ppemY}, true
				}
			}
			return bitmapGlyph{}, false
		}
		if !ok || bitDepth != 32 {
			return bitmapGlyph{}, false
		}
		g.kind = bitmapDataPNG
		return g, true
	}
	return bitmapGlyph{}, false
}

// leftSideBearing is GlyphMetrics::left_side_bearing of skrifa at the
// default location, as BitmapStrikes::new makes the metrics. It returns 0
// where skrifa returns None.
func leftSideBearing(font *Font, glyph GlyphID) float32 {
	maxp, hhea, hmtx := font.table("maxp"), font.table("hhea"), font.table("hmtx")
	if len(maxp) < 6 || uint32(glyph) >= uint32(binary.BigEndian.Uint16(maxp[4:])) || len(hhea) < 36 {
		return 0
	}
	n := int(binary.BigEndian.Uint16(hhea[34:]))
	var off int
	if int(glyph) < n {
		off = int(glyph)*4 + 2
	} else {
		off = n*4 + (int(glyph)-n)*2
	}
	if off+2 > len(hmtx) {
		return 0
	}
	return float32(int16(binary.BigEndian.Uint16(hmtx[off:])))
}

// glyphBoundsYMin is the y_min of GlyphMetrics::bounds of skrifa at the
// default location, as BitmapStrikes::new makes the metrics: the bounds of
// the control points of the outline for a font with gvar, CFF or CFF2, and
// the bounds in the header of the glyph in glyf otherwise. It returns 0
// where skrifa returns None.
func glyphBoundsYMin(font *Font, glyph GlyphID) float32 {
	if font.ld.HasTable(ot.MustNewTag("gvar")) || font.ld.HasTable(ot.MustNewTag("CFF ")) || font.ld.HasTable(ot.MustNewTag("CFF2")) {
		if p, ok := glyphPath(font, glyph); ok {
			return p.Bounds().Top()
		}
		return 0
	}
	head, loca, glyf := font.table("head"), font.table("loca"), font.table("glyf")
	if len(head) < 54 || loca == nil || glyf == nil {
		return 0
	}
	var start, end int
	if int16(binary.BigEndian.Uint16(head[50:])) == 0 {
		i := int(glyph) * 2
		if i+4 > len(loca) {
			return 0
		}
		start = int(binary.BigEndian.Uint16(loca[i:])) * 2
		end = int(binary.BigEndian.Uint16(loca[i+2:])) * 2
	} else {
		i := int(glyph) * 4
		if i+8 > len(loca) {
			return 0
		}
		start = int(binary.BigEndian.Uint32(loca[i:]))
		end = int(binary.BigEndian.Uint32(loca[i+4:]))
	}
	if end <= start || start+10 > len(glyf) {
		return 0
	}
	return float32(int16(binary.BigEndian.Uint16(glyf[start+4:])))
}
