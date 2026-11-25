// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text. It holds the helpers KernPair and
// LigaturePair of section 8 of D78, for a TeX engine that asks for the kern
// and the ligature of one pair of glyphs (D95).

import (
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// pairScripts are the scripts whose default language system KernPair and
// LigaturePair read, in order.
var pairScripts = [...]Tag{ot.MustNewTag("DFLT"), ot.MustNewTag("latn")}

// pairLookups returns the lookup indices of the feature tag in the default
// language system of the first script of pairScripts that la has. It
// returns false when la has none of the scripts, or when the language system
// has no such feature.
func pairLookups(la *Layout, tag Tag) ([]uint16, bool) {
	for _, script := range pairScripts {
		i := la.FindScript(script)
		if i < 0 {
			continue
		}
		langSys := la.Scripts[i].DefaultLangSys
		if langSys == nil {
			return nil, false
		}
		var lookups []uint16
		found := false
		for _, index := range langSys.FeatureIndices {
			if f := la.Features[index]; f.Tag == tag {
				found = true
				lookups = append(lookups, f.LookupListIndices...)
			}
		}
		return lookups, found
	}
	return nil, false
}

// KernPair returns the kern of the pair left, right in font units, from the
// pair adjustments of the feature kern of GPOS for the script DFLT or latn,
// or else from the table kern. It applies no contextual lookup. The kern is
// the change of the advance of left, with the variation deltas at the
// coordinates of the face. It is negative when the glyphs move closer.
func (f *Face) KernPair(left, right GID) float32 {
	if lookups, ok := pairLookups(&f.GPOS.Layout, ot.MustNewTag("kern")); ok {
		var kern float32
		for _, index := range lookups {
			if int(index) >= len(f.GPOS.Lookups) {
				continue
			}
			for _, subtable := range f.GPOS.Lookups[index].Subtables {
				pair, isPair := subtable.(tables.PairPos)
				if !isPair {
					continue
				}
				if v, applied := f.pairPosKern(pair, gID(left), gID(right)); applied {
					kern += v
					break
				}
			}
		}
		return kern
	}
	var kern int32
	for _, subtable := range f.Kern {
		if !subtable.IsHorizontal() || subtable.IsCrossStream() || subtable.IsVariation() {
			continue
		}
		if simple, ok := subtable.Data.(SimpleKerns); ok {
			kern += simple.KernPair(left, right)
		}
	}
	return float32(kern)
}

// pairPosKern returns the change of the advance of left that pair gives. It
// returns false when the subtable does not hold the pair, so that the
// lookup tries its next subtable, as HarfBuzz does.
func (f *Face) pairPosKern(pair tables.PairPos, left, right gID) (float32, bool) {
	covIndex, ok := pair.Cov().Index(left)
	if !ok {
		return 0, false
	}
	switch data := pair.Data.(type) {
	case tables.PairPosData1:
		if covIndex >= len(data.PairSets) {
			return 0, false
		}
		rec, ok := data.PairSets[covIndex].FindGlyph(right)
		if !ok {
			return 0, false
		}
		return f.advanceAdjustment(rec.ValueRecord1), true
	case tables.PairPosData2:
		class1, _ := data.ClassDef1.Class(left)
		class2, _ := data.ClassDef2.Class(right)
		if class1 >= data.Class1Count() || class2 >= data.Class2Count() {
			return 0, false
		}
		return f.advanceAdjustment(data.Record(class1, class2).ValueRecord1), true
	}
	return 0, false
}

// advanceAdjustment returns the horizontal advance of v, with the delta of
// its device table at the coordinates of the face.
func (f *Face) advanceAdjustment(v tables.ValueRecord) float32 {
	adv := float32(v.XAdvance)
	if index, ok := v.XAdvDevice.(tables.DeviceVariation); ok && len(f.coords) != 0 {
		adv += f.GDEF.ItemVarStore.GetDelta(tables.VariationStoreIndex(index), f.coords)
	}
	return adv
}

// LigaturePair returns the ligature of the pair left, right from the
// feature liga of GSUB for the script DFLT or latn. It returns false when
// the pair has none.
func (f *Font) LigaturePair(left, right GID) (GID, bool) {
	lookups, ok := pairLookups(&f.GSUB.Layout, ot.MustNewTag("liga"))
	if !ok {
		return 0, false
	}
	for _, index := range lookups {
		if int(index) >= len(f.GSUB.Lookups) {
			continue
		}
		for _, subtable := range f.GSUB.Lookups[index].Subtables {
			liga, isLiga := subtable.(tables.LigatureSubs)
			if !isLiga {
				continue
			}
			covIndex, ok := liga.Coverage.Index(gID(left))
			if !ok || covIndex >= len(liga.LigatureSets) {
				continue
			}
			for _, lig := range liga.LigatureSets[covIndex].Ligatures {
				if len(lig.ComponentGlyphIDs) == 1 && lig.ComponentGlyphIDs[0] == gID(right) {
					return GID(lig.LigatureGlyph), true
				}
			}
		}
	}
	return 0, false
}
