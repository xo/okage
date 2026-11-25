# Backlog

This document holds the work that is known and not done, with where each item
came from. When an item is done, delete it, and record anything that was
decided in [`decisions/`](decisions/README.md).

Work on the port goes in units: one upstream file with its tests. Claim a unit
under "Units in progress" before you start, with the name of your session.
When the change is committed, delete the claim.

## Units in progress

## Phase 6

### Port the example select of simplecss

From phase 6. `examples/select.rs` of simplecss matches selectors against a
tree of roxmltree. It was not ported while `roxmltree` was being
written. Port it as an example of `internal/simplecss`, with its output.

## Phase 10

### Follow the font model of read-fonts 0.46 more closely

From phase 10, resvg commit `04291b74`, and D9 rule 3. usvg now reads fonts
through the owned font model of read-fonts 0.46, which skrifa 0.49 exports.
`svg/usvg_text_cache.go` and `resolveFont` follow its rules on go-text. Three
differences stay:

1. read-fonts keeps a metric with its MVAR delta as an F48Dot16. Go keeps it
   as a float32, so a rounded metric can differ at a location with a delta.
2. The walk of the paint graph of COLRv1 in `internal/colr` ports skrifa
   0.44 (D32, D62). usvg now walks the graph with the model of read-fonts
   0.46. No test shows a difference.
3. read-fonts reads the strikes of the table EBDT. usvg draws none of its
   glyphs, so Go leaves the strikes out. The result is the same.

## Filters

1. Give the blur and the color matrix of the filters of `svg` an API on a
   `Mask` and on a `Pixmap`. ivy blurs the masks of the shadows, the glows and
   the soft edges of DrawingML with box blurs of its own, and changes the
   colors of pictures for the picture effects of Word itself. Both are in
   `internal/drawingml` of ivy, in `blur.go` and `recolor.go`. Source: ivy,
   2026-11-19. D88 proposes the API, and question 39 at the end of
   [`PLAN.md`](PLAN.md) asks the maintainers to choose.

## The PDF writer

From D62.

1. Make a Type 3 font from images that the caller gives, each with its text.
   The writer makes a Type 3 font only from the glyphs of a font file. ivy
   then writes the glyphs of a Type 3 font of bitmaps in a PDF file as image
   masks of the page, which each viewer scales down in another way, and it
   writes a color glyph or a bitmap glyph, such as an emoji, as an image
   without its text. Source: ivy, "Left by phase 7a" item 3 and "Left by
   phase 7b" item 2, 2026-11-19. D89 proposes the API, and question 40 at
   the end of [`PLAN.md`](PLAN.md) asks the maintainers to choose.
