# D39. Text falls back past a missing character, and `ShapeText` draws color glyphs

Status: Decided, amends D37.

zuzu draws text with `Renderer.ShapeText` (D37), and it wants color emoji from
a fallback font. Two faults blocked that:

- The fallback of usvg stops at the first missing character that no font
  draws. The characters after that character get no fallback, even when a
  font draws them. For example, in `A😀🐶B`, with no font for 😀, the 🐶 stays
  missing.
- The fallback of usvg copies the glyphs of a fallback font one by one, and
  only when that font gives as many glyphs as the first font. An emoji
  sequence, such as the flag 🇯🇵 or a family that zero width joiners (U+200D)
  join, is several glyphs in a text font and one glyph in an emoji font. So
  the sequence stays missing. With the emoji font first, the Latin letters
  stay missing for the same reason.
- `TextRun.AppendPath` transformed the paths of the run in place, because
  `path.Path.Transform` changes its path, as the method of tiny-skia does that
  takes `self`. After a call at an origin other than (0, 0), the outlines and
  the color glyphs of the run were in the wrong place. `RenderNode` of the
  flattened group then drew nothing for a COLR glyph. `AppendPath` also
  appended the layers of a COLR glyph as outlines.

## The decision

1. The fallback skips a missing character that no font draws, and goes on
   with the next missing character. Upstream stops there. This is a
   deliberate difference from upstream, and it applies to `ShapeText` and to
   the `<text>` elements of SVG data.
2. The fallback replaces clusters, not single glyphs. It splits the glyphs of
   the first font and of the fallback font at each byte where both start a
   cluster. In each piece where the first font has a missing glyph and the
   fallback font has none, it takes the glyphs of the fallback font. The two
   fonts can give different numbers of glyphs. Upstream copies glyphs one by
   one, stops when the numbers differ, and has a TODO to replace clusters.
   This is also a deliberate difference from upstream.
3. A fallback selector that returns a font that the run already uses does not
   stop the fallback. The fallback skips the character.
4. A character that no font draws keeps the glyph 0, and the layout warns
   about it on the logger (D20). It is not an error.
5. `TextRun.AppendPath` does not change the run. It appends only the outlines
   of the glyphs, the underline and the overline. It leaves out each glyph of
   the tables COLR, SVG, CBDT and sbix.
6. `TextRun.HasColorGlyphs` reports if the run has such a glyph.
   `TextRun.RenderColorGlyphs(pixmap, transform)` draws those glyphs onto a
   pixmap, with the origin of the run at the origin of the transform. It gives
   the same bytes as the renderer for the same `<text>` element.
7. A program draws a whole run in two steps: it fills the path of
   `AppendPath` with its paint, and then it calls `RenderColorGlyphs` with the
   same origin.

## Why

A line of a label can hold an emoji that a font draws and a character that no
font draws. With the fallback of upstream, one missing character removes the
emoji after it, and the caller cannot fix that without splitting the run
itself. With the copy of single glyphs, a flag or a ZWJ sequence never falls
back, so the caller has to split the run at each emoji. The test fonts of
resvg do not hold such a line, so the change gives the same images as resvg
for its tests.

`AppendPath` and `RenderColorGlyphs` split a run into what the caller paints
and what the font paints. A COLR glyph has its own colors, and a fill with the
paint of the caller under it is wrong.
