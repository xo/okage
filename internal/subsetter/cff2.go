// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff2.rs. It sets the seam subsetCFF2Table of
// lib.go. doc.go describes the seams.

func init() {
	subsetCFF2Table = subsetCFF2
}

// subsetCFF2 is subset in subsetter src/cff2.rs. CFF2 fonts will currently
// be converted into TTF fonts.
func subsetCFF2(ctx *context) error {
	maxpData := newMaxpData()
	hmtxData := make([]hmtxMetric, 0, int(ctx.mapper.NumGids()))

	err := subsetWith(ctx, func(oldGid uint16, ctx *context) ([]byte, error) {
		// We reject CFF2 fonts earlier if `variable-fonts` feature is not
		// enabled. Upstream marks the dummy interjector unreachable, and the
		// port returns an error where upstream panics.
		if !ctx.interjector.isSkrifa() {
			return nil, errSubset
		}
		advance, lsb, data, ok := interject(ctx.interjector.skrifa, &maxpData, oldGid)
		if !ok {
			return nil, errMalformedFont
		}
		hmtxData = append(hmtxData, hmtxMetric{advance, lsb})
		return data, nil
	})
	if err != nil {
		return err
	}

	ctx.customMaxpData = &maxpData
	ctx.customHmtxData = hmtxData

	return nil
}
