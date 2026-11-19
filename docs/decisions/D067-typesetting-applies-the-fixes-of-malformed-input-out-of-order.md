# D67. typesetting applies the fixes of malformed input out of order

Status: Decided, amends D65.

`pdf.NewFont` gives the bytes of its caller to `typesetting`, and ivy reads
fonts from strangers. The merge of phases 15 and 16 of the PDF writer ran
`FuzzText` of `pdf`, and it found panics of `typesetting` in a minute. Many
pending commits of go-text fix such faults, but `typesetting` applies the
commits of go-text in order (D52), and 97 commits were pending. On
2026-11-19 the maintainers decided to apply these fixes now, out of order,
as D59, D61 and D65 did.

## The commits that okage applies

okage applies each pending commit of `docs/upstream/typesetting.tsv` that
fixes a panic, a loop that does not end, or a missing bound on malformed
input, with the commits that it needs. okage leaves the other pending
commits alone. The commits are these, in the order of the ledger:

1. `87445a20`, the panics of `harfbuzz` on malformed fonts.
2. `1e1f4714`, `1fe568f5`, `9b75198f` and `c5a303fa`, the ranges of the
   tables glyf, COLR and CBLC.
3. `31064bfc`, `a13b8655` and `6bd6edb1`, the anchors, the mark classes
   and the extension subtables of GPOS and GSUB.
4. `c6965dda` and `fbe0dbae`, the regions of an item variation store and
   the named instances of fvar.
5. `f7b0a668`, the checksum of `WriteTTF`.
6. `1b47353e` and `45139db4`, the subtables of format 4 and 10 of cmap.
7. `cc144d56`, `40afd1a9` and `209b0896`, the tables SVG, post and the
   bitmap index subtables.
8. `4bc3723f`, `a2c0a64f` and `82a95b99`, the recursion of the cursive
   attachments and the insertions of AAT at the end of a text.
9. `7cbf7b37`, `d64a4a9a`, `56a2aae7` and `47ed8cab`, which bound the work
   of the moves of a buffer, of kern, of a syllable and of the stretch of
   Arabic.

Each commit is `ported` in the ledger. The column `matched` does not move,
because older commits are still pending (D12). A test file that reads
`typesetting-utils` is left out (D52). After these commits, 74 commits are
pending.

## Where a commit meets a change of okage

1. `1e1f4714` makes `ParseGlyf` return an error for a range of loca out of
   the table glyf, and `NewFont` then drops the whole table. okage keeps
   its change of D65: such a glyph has no outline, as in read-fonts and
   FreeType, and the other glyphs keep theirs.
2. `c6965dda` also makes `VariationRegion.Evaluate` read only the axes of
   the region. The change of D61 already does that, so okage keeps it.
3. `f7b0a668` fixes the checksum of `WriteTTF` as D65 did. okage follows
   go-text, as D65 says, and drops its own change and its item in D65.

## The faults that no commit fixes

`FuzzText` found two more panics that no commit of go-text fixes. okage
fixes them in `typesetting`, each with a comment that starts with `okage:`
and names this decision, and with a test in
`typesetting/font/okage_test.go`:

1. A language system of GSUB or GPOS can hold a feature index out of the
   feature list. harfbuzz indexed the list with it and panicked.
   `sanitizeLangSys` now drops such an index. HarfBuzz reads it as the Null
   feature, whose tag matches no feature, so the result is the same.
2. A segment of format 4 of cmap from 0 to 0xFFFF with a glyph array has
   65,536 codes. go-text counted them in uint16, which gave 0, and
   `Lookup` and the iterator indexed the empty array and panicked.
   `newCmap4` now counts in int, and returns an error for a segment that
   ends before it starts.

## When upstream changes

1. When okage applies a pending commit of go-text that changes one of
   these functions, keep the change of okage, as "typesetting" in
   [`UPSTREAM.md`](../UPSTREAM.md) says.
2. If go-text fixes a fault in the same way, follow go-text, and remove
   the comment `okage:` and the item of this decision.
