// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/maxp.rs.
//
// The `maxp` table contains the number of glyphs (and some additional
// information depending on the version). All we need to do is rewrite the
// number of glyphs, the rest can be copied from the old table.

// subsetMaxp is subset in subsetter src/maxp.rs.
func subsetMaxp(ctx *context) error {
	const (
		postTrueTypeVersion uint32 = 0x00010000
		postCFFVersion      uint32 = 0x00005000
	)

	maxp, ok := ctx.expectTable(tagMAXP)
	if !ok {
		return errMalformedFont
	}
	r := newReader(maxp)
	// Version
	if _, ok := r.readU32(); !ok {
		return errMalformedFont
	}
	// number of glyphs
	if _, ok := r.readU16(); !ok {
		return errMalformedFont
	}

	var version uint32
	switch ctx.flavor {
	case fontFlavorTrueType:
		version = postTrueTypeVersion
	case fontFlavorCFF:
		version = postCFFVersion
	case fontFlavorCFF2:
		// Since we convert to TrueType.
		version = postTrueTypeVersion
	}

	subMaxp := newWriter()
	subMaxp.writeU32(version)
	subMaxp.writeU16(ctx.mapper.NumGids())

	if version == postTrueTypeVersion {
		if customData := ctx.customMaxpData; customData != nil {
			subMaxp.writeU16(customData.maxPoints)
			subMaxp.writeU16(customData.maxContours)
			subMaxp.writeU16(customData.maxCompositePoints)
			subMaxp.writeU16(customData.maxCompositeContours)
			subMaxp.writeU16(customData.maxZones)
			subMaxp.writeU16(customData.maxTwilightPoints)
			subMaxp.writeU16(customData.maxStorage)
			subMaxp.writeU16(customData.maxFunctionDefs)
			subMaxp.writeU16(customData.maxInstructionDefs)
			subMaxp.writeU16(customData.maxStackElements)
			subMaxp.writeU16(customData.maxSizeOfInstructions)
			subMaxp.writeU16(customData.maxComponentElements)
			subMaxp.writeU16(customData.maxComponentDepth)
		} else {
			tail, ok := r.tail()
			if !ok {
				return errMalformedFont
			}
			subMaxp.extend(tail)
		}
	}

	ctx.push(tagMAXP, subMaxp.finish())
	return nil
}

// maxpData is MaxpData in subsetter src/maxp.rs. When converting CFF2 to TTF,
// we need to write a version 1.0 MAXP table.
//
// The zero value is not the value of upstream. Make one with newMaxpData,
// which is MaxpData::default.
type maxpData struct {
	maxPoints             uint16
	maxContours           uint16
	maxCompositePoints    uint16
	maxCompositeContours  uint16
	maxZones              uint16
	maxTwilightPoints     uint16
	maxStorage            uint16
	maxFunctionDefs       uint16
	maxInstructionDefs    uint16
	maxStackElements      uint16
	maxSizeOfInstructions uint16
	maxComponentElements  uint16
	maxComponentDepth     uint16
}

// newMaxpData is the impl Default of MaxpData in subsetter src/maxp.rs.
func newMaxpData() maxpData {
	return maxpData{
		maxZones: 1,
		// Could probably be 0 too since we only use simple glyphs when
		// converting?
		maxComponentDepth: 1,
	}
}
