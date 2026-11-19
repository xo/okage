# D71. The copy of go-text does not fuse a multiply and an add

Status: Decided, amends D16.

The maintainers answered question 31 on 2026-11-20.

Rule 2 of D16 says that okage never fuses a multiply and an add. The Go
compiler fuses `x*y + z` into one instruction on arm64, s390x, ppc64le,
riscv64 and loong64, and the result is then rounded once instead of twice.
The nightly run of 2026-11-19 found such a fusion in the TJ adjustments of
`pdf`. The fix for that run stopped each fusion in the code that okage
ports from Rust, and in the additions of D59 to `typesetting`. The copy of
go-text in `typesetting` still had fusions on 12 lines, and go-text has
the same fusions on those platforms.

## The decision

1. Rule 2 of D16 applies to `typesetting` too. Each product that the
   compiler can fuse with an add gets an explicit conversion, such as
   `float32(x*y)`.
2. Each such change has a comment that starts with `okage:` and names D16
   and this decision (D52).
3. On 2026-11-20 the copy has these changes:
   - The deltas of gvar in `font/variations.go`: the sums of the scaled
     deltas of a tuple, and the interpolation of an inferred delta.
   - The deltas of the item variation store, which HVAR reads, in
     `font/opentype/tables/xvar_src.go`.
   - The blends of CFF2 in `font/cff/charstring.go`.
   - The scale of the points of a composite glyph in `font/glyphs.go`.
   - The tracking of AAT in `font/opentype/tables/aat_trak_src.go`: the
     interpolation between two tracks and between two sizes.
   - The extents of a font in `harfbuzz/font.go`: the extents of a font
     without metrics, and the extent of an emboldened font.

## Why

A variable font, a scaled composite glyph or AAT tracking can otherwise
give other outlines or advances on arm64 than on amd64, in `svg` and in
`pdf`. With this rule, every platform gives the same output. The nightly
run on arm64 and s390x can then find a new fusion in any package.

A scan of the assembly of okage finds each fusion. Build okage with
`go build -a -gcflags='github.com/xo/okage/...=-S' ./...` for each of those
platforms, and search the output for the fused instructions, such as
`FMADDS` and `FMSUBD`. On 2026-11-20 only the calls of `math.FMA` in
`internal/crmath` and `kurbo` give one. They are on purpose.

## When upstream changes

1. When okage applies a pending commit of go-text that changes one of these
   lines, keep the conversion, as "typesetting" in
   [`UPSTREAM.md`](../UPSTREAM.md) says.
2. After a new commit of go-text, scan the assembly again. A new fusion in
   `typesetting` gets a conversion and a comment, as this decision says.
