# D32. `svg` walks the paint graph of COLRv1

Status: Decided, amends D9.

A COLRv1 glyph is a color glyph that a font draws as a graph of paints:
layers, fills, gradients, transforms and composites. usvg draws such a glyph
with `ColorGlyph::paint` of skrifa. That function walks the graph and calls
the methods of the trait `ColorPainter`, which `GlyphPainter` of usvg
`src/text/colr.rs` implements.

go-text parses the tables COLR and CPAL, but it does not walk the graph.
D9 ports no crate of text, so rule 3 of D9 put the gap in
`docs/BACKLOG.md`. Phase 9 ported the walk of skrifa 0.44 into
`svg/usvg_text_colr_paint.go`, so that the tests could pass, and asked the
maintainers where the walk lives: in `svg`, or in go-text as a contribution.

## The decision

1. The walk of the paint graph stays in `svg`, in
   `svg/usvg_text_colr_paint.go`. okage does not offer it to go-text.
2. This is the one part of a crate of text that okage ports. It is an
   exception to rule 1 of D9, and it covers only the walk:
   `ColorGlyph::paint` of `src/color/mod.rs`, and `traversal.rs`,
   `instance.rs` and `transform.rs` of `src/color` of skrifa. go-text still
   parses the tables, reads the outlines and shapes the text.
3. The walk is ported as any other upstream file is ported. The doc comment
   of each item names the item of skrifa that it replaces.
4. The walk follows skrifa 0.44. resvg now uses the font model of read-fonts
   0.46, through skrifa 0.49, and no test shows a difference.
   `docs/BACKLOG.md` keeps that difference under phase 10.
5. A later version of skrifa that changes the walk is not tracked in a ledger
   of its own, because okage has no upstream entry for skrifa. When a commit
   of resvg raises the version of skrifa, and a test of resvg then differs in
   a COLRv1 glyph, the port of that commit ports the change to the walk.
