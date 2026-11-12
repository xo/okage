# D39. Text falls back past a missing character, and `ShapeText` draws color glyphs

Status: Decided, amends D37.

zuzu draws text with `Renderer.ShapeText` (D37), and it wants color emoji from
a fallback font. Two faults blocked that:

- The fallback of usvg stops at the first missing character that no font
  draws. The characters after that character get no fallback, even when a
  font draws them. For example, in `A😀🐶B`, with no font for 😀, the 🐶 stays
  missing.
- `TextRun.AppendPath` transformed the paths of the run in place, because
  `path.Path.Transform` changes its path, as the method of tiny-skia does that
  takes `self`. After a call at an origin other than (0, 0), the outlines and
  the color glyphs of the run were in the wrong place. `RenderNode` of the
  flattened group then drew nothing for a COLR glyph. `AppendPath` also
  appended the layers of a COLR glyph as outlines.

## The decision

1. The fallback skips a missing character that no font draws, and goes on
   with the next missing character. When a fallback font gives another number
   of glyphs than the first font, the fallback leaves that font out and tries
   the next font. Upstream stops in both cases. This is a deliberate
   difference from upstream, and it applies to `ShapeText` and to the
   `<text>` elements of SVG data.
2. A fallback selector that returns a font that the run already uses does not
   stop the fallback. The fallback skips the character.
3. A character that no font draws keeps the glyph 0, and the layout warns
   about it on the logger (D20). It is not an error.
4. `TextRun.AppendPath` does not change the run. It appends only the outlines
   of the glyphs, the underline and the overline. It leaves out each glyph of
   the tables COLR, SVG, CBDT and sbix.
5. `TextRun.HasColorGlyphs` reports if the run has such a glyph.
   `TextRun.RenderColorGlyphs(pixmap, transform)` draws those glyphs onto a
   pixmap, with the origin of the run at the origin of the transform. It gives
   the same bytes as the renderer for the same `<text>` element.
6. A program draws a whole run in two steps: it fills the path of
   `AppendPath` with its paint, and then it calls `RenderColorGlyphs` with the
   same origin.

## Why

A line of a label can hold an emoji that a font draws and a character that no
font draws. With the fallback of upstream, one missing character removes the
emoji after it, and the caller cannot fix that without splitting the run
itself. The test fonts of resvg do not hold such a line, so the change gives
the same images as resvg for its tests.

`AppendPath` and `RenderColorGlyphs` split a run into what the caller paints
and what the font paints. A COLR glyph has its own colors, and a fill with the
paint of the caller under it is wrong.
