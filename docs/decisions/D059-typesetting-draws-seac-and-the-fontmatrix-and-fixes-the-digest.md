# D59. typesetting draws seac and the FontMatrix, and fixes the digest

Status: Decided.

ivy draws the CFF fonts of PDF files and the text of Office documents with
`typesetting`, the copy of go-text in okage (D52). On 2026-11-18 a
maintainer asked for four changes to the copy, for ivy. Three change the
package `font/cff`. One applies a commit of go-text to `harfbuzz`. Each
change has a comment that starts with `okage:` and names this decision.

## The predefined charsets of CFF

A charset gives the SID (string ID) of each glyph of a CFF font. The Top
DICT can select one of three predefined charsets with the offset 0, 1 or 2.
go-text keeps their tables unexported.

1. `cff.CharsetISOAdobe`, `cff.CharsetExpert` and `cff.CharsetExpertSubset`
   are exported variables. Their values are the tables of Appendix C of
   Adobe Technical Note 5176.
2. Each variable is a copy of the table that the parser reads. A program
   that changes a variable does not change how `cff.Parse` reads a font.

## The seac of endchar

In a Type 2 charstring, `endchar` with four operands, after an optional
width, is the `seac` of Type 1. The operands are `adx ady bchar achar`.
`bchar` and `achar` are codes of StandardEncoding. The glyph is the base
glyph `bchar` at the origin and the accent `achar` at `adx`, `ady`. go-text
ignores these operands, so such a glyph has no outline.

1. `CFF.LoadGlyph` draws the base glyph, then the accent, after the outline
   of the glyph itself. This is the order of the Type 2 specification and of
   FreeType. skrifa draws the accent first. The order does not change the
   filled shape.
2. A code maps to a SID through StandardEncoding, in Appendix B of the
   specification. The SID maps to the first glyph with that SID in the
   charset, as in FreeType.
3. A code without a glyph adds nothing. FreeType returns an error for it.
   okage draws what it can.
4. A component that is itself a `seac` adds only its own outline. okage
   does not follow a second `seac`.
5. A CIDFont does not use `seac`. Its glyphs keep the go-text behavior.
6. The bounds of the glyph include the bounds of both components.

## The FontMatrix

The FontMatrix maps the units of a charstring to the em. The Top DICT can
give one, and each font dictionary of a CIDFont can give one. go-text
ignores both. A CIDFont often has the identity in its Top DICT and the
matrix `0.001 0 0 0.001 0 0` in each font dictionary.

1. The parser reads the FontMatrix of the Top DICT and of each font
   dictionary of the FDArray.
2. For each font dictionary, the matrix of the font dictionary applies
   first, then the linear part of the matrix of the Top DICT. The
   translation of the Top DICT does not apply. FreeType and skrifa combine
   the matrices in the same way.
3. A font dictionary without a FontMatrix uses the matrix of the Top DICT.
   A Top DICT without a FontMatrix uses the default matrix
   `0.001 0 0 0.001 0 0`.
4. `CFF.UnitsPerEm` is a new method. It returns 1 divided by the y scale of
   the matrix of the Top DICT. FreeType computes the units per em of a CFF
   font in the same way. It uses the absolute value of `yy`, or of `yx`
   if `yy` is 0. The value is 1000 for the default matrix.
5. `CFF.LoadGlyph` multiplies each point by the combined matrix and by
   `UnitsPerEm`. So a font with only the matrix of its Top DICT, and no skew
   or rotation, keeps the outlines of go-text. A font of an OpenType file
   usually has the unitsPerEm of its head table in its Top DICT.
6. A matrix that is not finite, or that has no inverse, does not apply.
7. The bounds are the bounds of the four corners of the bounds of go-text,
   after the matrix.
8. FreeType divides by the unitsPerEm of the head table of an OpenType font
   whose Top DICT has no FontMatrix. okage divides by 0.001. The results
   differ only if a font dictionary of such a font has a FontMatrix and the
   head table does not give 1000 units per em.
9. FreeType rounds the translation of a FontMatrix to whole units. okage
   keeps it as it is.
10. CFF2 does not change.

## The digest of a lookup

The package `harfbuzz` keeps a digest for each lookup and each subtable.
A digest is a small filter of the glyphs of a coverage. It can say that a
glyph is not in the coverage, and it skips the lookup for that glyph.

go-text added a range of glyphs to the digest only while none of its three
parts was full. A range of 63 glyphs or more fills one part. So the ranges
after it were lost, and the digest skipped glyphs that the coverage holds.
With Calibri, the subtable of the `kern` lookup that holds the class kerning
lists `t`, `a` and `n` after such a range. So the pairs t-a, a-t, t-e and
n-t did not kern. A-V is in an earlier range, so it kerned. The kerning of
lookups of type 9 and the flag IgnoreMarks were not the cause.

1. The commit `e8da365e` of go-text fixes the fault. It is pending in the
   ledger, after commits that okage did not apply. okage applies it out of
   order, because ivy needs the kerning of Calibri now.
2. Its row in `docs/upstream/typesetting.tsv` is `ported`. The column
   `matched` does not move, because older commits are still pending (D12).
3. okage leaves out the change of `go.mod` and the two tests that read the
   fonts of `typesetting-utils` (D52). `TestDigestKerningRoboto` tests the
   same fault with `font/testdata/Roboto-Regular.ttf`, whose `kern` lookup
   has the same form.

## What does not change

The charset, the encoding and the FDSelect of a parsed font stay
unexported. A program that maps a code or a CID to a glyph still reads them
itself.
