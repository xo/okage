// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import "math"

// This file ports subsetter src/hmtx.rs.
//
// The `hmtx` table contains the horizontal metrics for each glyph. All we
// need to do is to rewrite the table so that it matches the sequence of the
// new glyphs. A minor pain point is that the table allows omitting the
// advance width for the last few glyphs if it is the same. In order to keep
// the code simple, we do not keep this optimization when rewriting the
// table. While doing so, we also rewrite the `hhea` table, which contains the
// number of glyphs that contain both, advance width and left side bearing
// metrics.
//
// The parsing logic was taken from ttf-parser.

// hmtxMetric is a pair (u16, i16) of subsetter src/hmtx.rs: the advance
// width and the left side bearing of a glyph.
type hmtxMetric struct {
	advance uint16
	lsb     int16
}

// subsetHmtx is subset in subsetter src/hmtx.rs.
func subsetHmtx(ctx *context) error {
	hmtx, ok := ctx.expectTable(tagHMTX)
	if !ok {
		return errMalformedFont
	}

	var newMetrics []hmtxMetric
	if ctx.interjector.isSkrifa() {
		if ctx.customHmtxData == nil {
			return errSubset
		}
		newMetrics = ctx.customHmtxData
		ctx.customHmtxData = nil
	} else {
		var err error
		if newMetrics, err = extractMetrics(hmtx, ctx); err != nil {
			return err
		}
	}

	// Find out the last index we need to include the advance width for.
	if len(newMetrics) > math.MaxUint16 {
		return errOverflow
	}
	// Upstream subtracts 1 in a u16, which wraps for an empty list, and then
	// panics on the index. The list holds the glyph 0, so it is not empty,
	// and the port returns an error where upstream panics.
	if len(newMetrics) == 0 {
		return errSubset
	}
	lastAdvanceWidthIndex := uint16(len(newMetrics)) - 1
	lastAdvanceWidth := newMetrics[lastAdvanceWidthIndex].advance

	for i := len(newMetrics) - 2; i >= 0; i-- {
		if newMetrics[i].advance != lastAdvanceWidth {
			break
		}
		lastAdvanceWidthIndex--
	}

	subHmtx := newWriter()

	for index, metric := range newMetrics {
		if uint16(index) <= lastAdvanceWidthIndex {
			subHmtx.writeU16(metric.advance)
		}

		subHmtx.writeI16(metric.lsb)
	}

	ctx.push(tagHMTX, subHmtx.finish())

	hhea, ok := ctx.expectTable(tagHHEA)
	if !ok || len(hhea) < 2 {
		return errMalformedFont
	}
	subHhea := newWriter()
	subHhea.extend(hhea[:len(hhea)-2])
	subHhea.writeU16(lastAdvanceWidthIndex + 1)

	ctx.push(tagHHEA, subHhea.finish())

	return nil
}

// extractMetrics is extract_metrics in subsetter src/hmtx.rs. It returns
// the metrics of the glyphs of the remapper, in the order of the remapping.
func extractMetrics(hmtx []byte, ctx *context) ([]hmtxMetric, error) {
	// Extract the number of horizontal metrics from the `hhea` table.
	hhea, ok := ctx.expectTable(tagHHEA)
	if !ok {
		return nil, errMalformedFont
	}
	r := newReader(hhea)
	r.skipBytes(34)
	numHMetrics, ok := r.readU16()
	if !ok {
		return nil, errMalformedFont
	}

	if numHMetrics == 0 {
		return nil, errOverflow
	}
	index := 4 * int(numHMetrics-1)
	if index > len(hmtx) {
		return nil, errMalformedFont
	}
	r = newReader(hmtx[index:])
	lastAdvanceWidth, ok := r.readU16()
	if !ok {
		return nil, errMalformedFont
	}

	newMetrics := make([]hmtxMetric, 0, int(ctx.mapper.NumGids()))
	for oldGid := range ctx.mapper.RemappedGids() {
		hasAdvanceWidth := oldGid < numHMetrics

		var offset int
		if hasAdvanceWidth {
			offset = int(oldGid) * 4
		} else {
			numHMetrics := int(numHMetrics)
			offset = numHMetrics*4 + (int(oldGid)-numHMetrics)*2
		}

		if offset > len(hmtx) {
			return nil, errMalformedFont
		}
		r := newReader(hmtx[offset:])

		if hasAdvanceWidth {
			adv, ok := r.readU16()
			if !ok {
				return nil, errMalformedFont
			}
			lsb, ok := r.readI16()
			if !ok {
				return nil, errMalformedFont
			}
			newMetrics = append(newMetrics, hmtxMetric{adv, lsb})
		} else {
			lsb, ok := r.readI16()
			if !ok {
				return nil, errMalformedFont
			}
			newMetrics = append(newMetrics, hmtxMetric{lastAdvanceWidth, lsb})
		}
	}

	return newMetrics, nil
}
