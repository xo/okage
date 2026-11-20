"""Write each value of the table MATH of a font as JSON, with fontTools.

The test of the reader of the table MATH of typesetting runs this script
and compares each value with the value that okage reads (D78). Run it as:

    python3 -I mathtable.py FONT

A glyph is its glyph ID. A MathValueRecord is a list of four numbers: the
value, and the three header fields of its device table, which are the start
size, the end size and the delta format. For the format 0x8000 the first two
fields are the outer and the inner index of the variation store. A record
without a device table has three zeros. A missing table is null. The field
glyphs holds the number of glyphs of the font.
"""

import json
import sys

from fontTools.ttLib import TTFont

# The names of the constants of the table MathConstants, in the order of the
# OpenType specification.
CONSTANTS = [
    "ScriptPercentScaleDown",
    "ScriptScriptPercentScaleDown",
    "DelimitedSubFormulaMinHeight",
    "DisplayOperatorMinHeight",
    "MathLeading",
    "AxisHeight",
    "AccentBaseHeight",
    "FlattenedAccentBaseHeight",
    "SubscriptShiftDown",
    "SubscriptTopMax",
    "SubscriptBaselineDropMin",
    "SuperscriptShiftUp",
    "SuperscriptShiftUpCramped",
    "SuperscriptBottomMin",
    "SuperscriptBaselineDropMax",
    "SubSuperscriptGapMin",
    "SuperscriptBottomMaxWithSubscript",
    "SpaceAfterScript",
    "UpperLimitGapMin",
    "UpperLimitBaselineRiseMin",
    "LowerLimitGapMin",
    "LowerLimitBaselineDropMin",
    "StackTopShiftUp",
    "StackTopDisplayStyleShiftUp",
    "StackBottomShiftDown",
    "StackBottomDisplayStyleShiftDown",
    "StackGapMin",
    "StackDisplayStyleGapMin",
    "StretchStackTopShiftUp",
    "StretchStackBottomShiftDown",
    "StretchStackGapAboveMin",
    "StretchStackGapBelowMin",
    "FractionNumeratorShiftUp",
    "FractionNumeratorDisplayStyleShiftUp",
    "FractionDenominatorShiftDown",
    "FractionDenominatorDisplayStyleShiftDown",
    "FractionNumeratorGapMin",
    "FractionNumDisplayStyleGapMin",
    "FractionRuleThickness",
    "FractionDenominatorGapMin",
    "FractionDenomDisplayStyleGapMin",
    "SkewedFractionHorizontalGap",
    "SkewedFractionVerticalGap",
    "OverbarVerticalGap",
    "OverbarRuleThickness",
    "OverbarExtraAscender",
    "UnderbarVerticalGap",
    "UnderbarRuleThickness",
    "UnderbarExtraDescender",
    "RadicalVerticalGap",
    "RadicalDisplayStyleVerticalGap",
    "RadicalRuleThickness",
    "RadicalExtraAscender",
    "RadicalKernBeforeDegree",
    "RadicalKernAfterDegree",
    "RadicalDegreeBottomRaisePercent",
]

# The corners of a MathKernInfoRecord, in the order of the specification.
CORNERS = ["TopRightMathKern", "TopLeftMathKern", "BottomRightMathKern", "BottomLeftMathKern"]


def record(font, rec):
    """Return a MathValueRecord, or a plain number, as a list."""
    if isinstance(rec, int):
        return [rec, 0, 0, 0]
    dev = rec.DeviceTable
    if dev is None:
        return [rec.Value, 0, 0, 0]
    return [rec.Value, dev.StartSize, dev.EndSize, dev.DeltaFormat]


def coverage(font, cov):
    """Return the glyph IDs of a coverage table, in coverage order."""
    return [font.getGlyphID(name) for name in cov.glyphs]


def values(font, cov, recs):
    """Return a map from a glyph ID to the record of its coverage index."""
    if cov is None:
        return None
    return {str(gid): record(font, recs[i]) for i, gid in enumerate(coverage(font, cov))}


def kern(font, mk):
    """Return one MathKern as its correction heights and its kern values."""
    if mk is None:
        return None
    return {
        "heights": [record(font, r) for r in mk.CorrectionHeight],
        "kerns": [record(font, r) for r in mk.KernValue],
    }


def construction(font, c):
    """Return one MathGlyphConstruction."""
    out = {
        "variants": [
            [font.getGlyphID(v.VariantGlyph), v.AdvanceMeasurement] for v in c.MathGlyphVariantRecord
        ],
        "assembly": None,
    }
    a = c.GlyphAssembly
    if a is not None:
        out["assembly"] = {
            "italics": record(font, a.ItalicsCorrection),
            "parts": [
                [
                    font.getGlyphID(p.glyph),
                    p.StartConnectorLength,
                    p.EndConnectorLength,
                    p.FullAdvance,
                    p.PartFlags,
                ]
                for p in a.PartRecords
            ],
        }
    return out


def constructions(font, cov, recs):
    """Return a map from a glyph ID to its MathGlyphConstruction."""
    if cov is None:
        return None
    return {str(gid): construction(font, recs[i]) for i, gid in enumerate(coverage(font, cov))}


def main():
    font = TTFont(sys.argv[1], fontNumber=0)
    out = {"math": "MATH" in font, "glyphs": len(font.getGlyphOrder())}
    if not out["math"]:
        json.dump(out, sys.stdout)
        return
    table = font["MATH"].table
    out["version"] = table.Version

    mc = table.MathConstants
    out["constants"] = [record(font, getattr(mc, name)) if mc is not None else None for name in CONSTANTS]

    gi = table.MathGlyphInfo
    out["italics"] = None
    out["accents"] = None
    out["extended"] = None
    out["kerns"] = None
    if gi is not None:
        ic = gi.MathItalicsCorrectionInfo
        if ic is not None:
            out["italics"] = values(font, ic.Coverage, ic.ItalicsCorrection)
        ta = gi.MathTopAccentAttachment
        if ta is not None:
            out["accents"] = values(font, ta.TopAccentCoverage, ta.TopAccentAttachment)
        if gi.ExtendedShapeCoverage is not None:
            out["extended"] = coverage(font, gi.ExtendedShapeCoverage)
        ki = gi.MathKernInfo
        if ki is not None:
            out["kerns"] = {
                str(gid): [kern(font, getattr(ki.MathKernInfoRecords[i], c)) for c in CORNERS]
                for i, gid in enumerate(coverage(font, ki.MathKernCoverage))
            }

    mv = table.MathVariants
    out["variants"] = None
    if mv is not None:
        out["variants"] = {
            "overlap": mv.MinConnectorOverlap,
            "vertical": constructions(font, mv.VertGlyphCoverage, mv.VertGlyphConstruction),
            "horizontal": constructions(font, mv.HorizGlyphCoverage, mv.HorizGlyphConstruction),
        }
    json.dump(out, sys.stdout, sort_keys=True)


if __name__ == "__main__":
    main()
