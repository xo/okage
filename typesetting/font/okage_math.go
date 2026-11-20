// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text (D78).
//
// It gives the values of the table MATH, as the API hb-ot-math of HarfBuzz
// does. Every length is in font units. A value with a VariationIndex table
// gets its delta from the item variation store of GDEF, at the coordinates
// of the face. A device table for a size in pixels is ignored, because okage
// does not hint.

import (
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// MathConstant names one value of the table MathConstants, such as
// AxisHeight, in the order of the OpenType specification.
type MathConstant uint8

// The constants of the table MathConstants. ScriptPercentScaleDown,
// ScriptScriptPercentScaleDown and RadicalDegreeBottomRaisePercent are
// percentages. The others are lengths in font units.
const (
	ScriptPercentScaleDown MathConstant = iota
	ScriptScriptPercentScaleDown
	DelimitedSubFormulaMinHeight
	DisplayOperatorMinHeight
	MathLeading
	AxisHeight
	AccentBaseHeight
	FlattenedAccentBaseHeight
	SubscriptShiftDown
	SubscriptTopMax
	SubscriptBaselineDropMin
	SuperscriptShiftUp
	SuperscriptShiftUpCramped
	SuperscriptBottomMin
	SuperscriptBaselineDropMax
	SubSuperscriptGapMin
	SuperscriptBottomMaxWithSubscript
	SpaceAfterScript
	UpperLimitGapMin
	UpperLimitBaselineRiseMin
	LowerLimitGapMin
	LowerLimitBaselineDropMin
	StackTopShiftUp
	StackTopDisplayStyleShiftUp
	StackBottomShiftDown
	StackBottomDisplayStyleShiftDown
	StackGapMin
	StackDisplayStyleGapMin
	StretchStackTopShiftUp
	StretchStackBottomShiftDown
	StretchStackGapAboveMin
	StretchStackGapBelowMin
	FractionNumeratorShiftUp
	FractionNumeratorDisplayStyleShiftUp
	FractionDenominatorShiftDown
	FractionDenominatorDisplayStyleShiftDown
	FractionNumeratorGapMin
	FractionNumDisplayStyleGapMin
	FractionRuleThickness
	FractionDenominatorGapMin
	FractionDenomDisplayStyleGapMin
	SkewedFractionHorizontalGap
	SkewedFractionVerticalGap
	OverbarVerticalGap
	OverbarRuleThickness
	OverbarExtraAscender
	UnderbarVerticalGap
	UnderbarRuleThickness
	UnderbarExtraDescender
	RadicalVerticalGap
	RadicalDisplayStyleVerticalGap
	RadicalRuleThickness
	RadicalExtraAscender
	RadicalKernBeforeDegree
	RadicalKernAfterDegree
	RadicalDegreeBottomRaisePercent

	// MathConstantCount is the number of constants.
	MathConstantCount
)

// loadMATH reads the table MATH of ld. A missing or malformed table gives
// nil, as for the other optional tables.
func loadMATH(ld *ot.Loader) *tables.MATH {
	raw, err := ld.RawTable(ot.MustNewTag("MATH"))
	if err != nil {
		return nil
	}
	table, _, err := tables.ParseMATH(raw)
	if err != nil {
		return nil
	}
	return &table
}

// HasMath reports whether the font has a MATH table that parsed.
func (f *Font) HasMath() bool { return f.MATH != nil }

// mathValue returns the value of rec, with the variation delta at the
// coordinates of the face.
func (f *Face) mathValue(rec tables.MathValueRecord) float32 {
	v := float32(rec.Value)
	if len(f.coords) == 0 {
		return v
	}
	if index, ok := rec.Device.VariationIndex(); ok {
		v += f.GDEF.ItemVarStore.GetDelta(index, f.coords)
	}
	return v
}

// MathConstant returns the value c in font units, with the variation
// deltas at the coordinates of the face. The three percentages,
// ScriptPercentScaleDown, ScriptScriptPercentScaleDown and
// RadicalDegreeBottomRaisePercent, are returned as they are. It returns 0
// when the font has no MATH table, or when c is not a constant.
func (f *Face) MathConstant(c MathConstant) float32 {
	if f.MATH == nil {
		return 0
	}
	mc := &f.MATH.MathConstants
	switch {
	case c == ScriptPercentScaleDown:
		return float32(mc.ScriptPercentScaleDown)
	case c == ScriptScriptPercentScaleDown:
		return float32(mc.ScriptScriptPercentScaleDown)
	case c == DelimitedSubFormulaMinHeight:
		return float32(mc.DelimitedSubFormulaMinHeight)
	case c == DisplayOperatorMinHeight:
		return float32(mc.DisplayOperatorMinHeight)
	case c == RadicalDegreeBottomRaisePercent:
		return float32(mc.RadicalDegreeBottomRaisePercent)
	case c < RadicalDegreeBottomRaisePercent:
		return f.mathValue(mc.Records[c-MathLeading])
	}
	return 0
}

// MathItalicsCorrection returns the italic correction of g, or 0.
func (f *Face) MathItalicsCorrection(g GID) float32 {
	if f.MATH == nil || g > 0xFFFF {
		return 0
	}
	rec, ok := f.MATH.MathGlyphInfo.MathItalicsCorrectionInfo.Value(tables.GlyphID(g))
	if !ok {
		return 0
	}
	return f.mathValue(rec)
}

// MathTopAccentAttachment returns the horizontal position over g where a
// top accent is centered. It returns false when g has none.
func (f *Face) MathTopAccentAttachment(g GID) (float32, bool) {
	if f.MATH == nil || g > 0xFFFF {
		return 0, false
	}
	rec, ok := f.MATH.MathGlyphInfo.MathTopAccentAttachment.Value(tables.GlyphID(g))
	if !ok {
		return 0, false
	}
	return f.mathValue(rec), true
}

// IsMathExtendedShape reports whether g is an extended shape.
func (f *Font) IsMathExtendedShape(g GID) bool {
	if f.MATH == nil || g > 0xFFFF {
		return false
	}
	cov := f.MATH.MathGlyphInfo.ExtendedShapeCoverage
	if cov == nil {
		return false
	}
	_, ok := cov.Index(tables.GlyphID(g))
	return ok
}

// MathKernCorner is one of the four corners of a glyph.
type MathKernCorner uint8

// The corners of a glyph, in the order of the table MathKernInfo.
const (
	MathKernTopRight MathKernCorner = iota
	MathKernTopLeft
	MathKernBottomRight
	MathKernBottomLeft
)

// MathKern returns the kern of g at corner for a script at height. The kern
// is the value of the first correction height above height, or the last
// value when height is at or above every correction height, as HarfBuzz
// chooses it. It returns 0 when g has no kern at corner.
func (f *Face) MathKern(g GID, corner MathKernCorner, height float32) float32 {
	if f.MATH == nil || g > 0xFFFF || corner > MathKernBottomLeft {
		return 0
	}
	rec, ok := f.MATH.MathGlyphInfo.MathKernInfo.Record(tables.GlyphID(g))
	if !ok {
		return 0
	}
	kern := rec[corner]
	if len(kern.KernValues) != len(kern.CorrectionHeights)+1 {
		return 0
	}
	// The index is the number of correction heights at or below height. The
	// heights are sorted, so a binary search finds the first height above it.
	lo, hi := 0, len(kern.CorrectionHeights)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if f.mathValue(kern.CorrectionHeights[mid]) <= height {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return f.mathValue(kern.KernValues[lo])
}

// MathMinConnectorOverlap returns the minimum overlap of two parts of an
// assembly.
func (f *Face) MathMinConnectorOverlap() float32 {
	if f.MATH == nil {
		return 0
	}
	return float32(f.MATH.MathVariants.MinConnectorOverlap)
}

// MathGlyphVariant is one size variant of a glyph.
type MathGlyphVariant struct {
	Glyph   GID
	Advance float32 // the height for a vertical variant, the width for a horizontal one
}

// MathGlyphVariants returns the size variants of g, smallest first, in
// the vertical or in the horizontal direction. It returns nil when g has
// none.
func (f *Face) MathGlyphVariants(g GID, vertical bool) []MathGlyphVariant {
	if f.MATH == nil || g > 0xFFFF {
		return nil
	}
	c, ok := f.MATH.MathVariants.Construction(tables.GlyphID(g), vertical)
	if !ok || len(c.Variants) == 0 {
		return nil
	}
	out := make([]MathGlyphVariant, len(c.Variants))
	for i, v := range c.Variants {
		out[i] = MathGlyphVariant{Glyph: GID(v.VariantGlyph), Advance: float32(v.AdvanceMeasurement)}
	}
	return out
}

// MathGlyphPart is one part of a glyph assembly.
type MathGlyphPart struct {
	Glyph                GID
	StartConnectorLength float32
	EndConnectorLength   float32
	FullAdvance          float32
	Extender             bool
}

// MathGlyphAssembly is the recipe that builds a glyph of any size.
type MathGlyphAssembly struct {
	ItalicsCorrection float32
	Parts             []MathGlyphPart // bottom to top, or left to right
}

// MathGlyphAssembly returns the assembly of g in a direction. It returns
// false when g has none.
func (f *Face) MathGlyphAssembly(g GID, vertical bool) (MathGlyphAssembly, bool) {
	if f.MATH == nil || g > 0xFFFF {
		return MathGlyphAssembly{}, false
	}
	c, ok := f.MATH.MathVariants.Construction(tables.GlyphID(g), vertical)
	if !ok || c.GlyphAssembly == nil {
		return MathGlyphAssembly{}, false
	}
	a := c.GlyphAssembly
	out := MathGlyphAssembly{
		ItalicsCorrection: f.mathValue(a.ItalicsCorrection),
		Parts:             make([]MathGlyphPart, len(a.Parts)),
	}
	for i, p := range a.Parts {
		out.Parts[i] = MathGlyphPart{
			Glyph:                GID(p.GlyphID),
			StartConnectorLength: float32(p.StartConnectorLength),
			EndConnectorLength:   float32(p.EndConnectorLength),
			FullAdvance:          float32(p.FullAdvance),
			Extender:             p.Extender(),
		}
	}
	return out, true
}
