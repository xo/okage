# D37. `svg` shapes a run of text without SVG data

Status: Decided.

A program that draws with the root package, such as a chart package, can draw
paths and shapes without SVG data, but not text. The layout of text of usvg is
private in `svg`. To draw text, such a program must write SVG data and parse
it, or shape the text with go-text itself, which gives other results than
`svg`. The maintainers want a small API that gives the size and the outlines
of a run of text, with the fonts, the fallback and the shaping of `svg`.

## The decision

1. `ShapeText(text, style, opts...)` lays out one run of text from an origin
   on the baseline and returns a `*TextRun`. `TextStyle` holds the families
   as a CSS `font-family` list, the size, the weight and the style. The
   options are the options of the parser, so `WithFontDatabase` gives the
   fonts. `Renderer.ShapeText` does the same with the fonts and the generic
   families of the renderer (D36).
2. `TextRun.Width` returns the advance width, and `TextRun.Bounds` returns the
   box from the metrics of the glyphs. `TextRun.AppendPath` appends the
   outlines of the glyphs to a `path.PathBuilder` at an origin.
   `TextRun.Text` returns the `Text` node, with its layout and its flattened
   group, for the glyphs that have no outline, such as color emoji.
3. `ShapeText` builds the `Text` node that the parser builds for a `<text>`
   element with the same attributes, and calls the same layout and flatten.
   A test compares its box and its outlines with those of the parser, point
   for point.
4. The text is laid out as it is. The parser collapses the white space of a
   `<text>` element, and `ShapeText` does not.
5. An invalid size gives `okage.ErrSVGTextInvalidSize`, and a text that no
   font draws gives `okage.ErrSVGTextNoFont`.
6. The code is in `svg/text_run.go`. It ports no upstream file. The mapping of
   the weight and the style to the axes of a variable font moves from
   `convertFont` into `autoFontVariations`, which both call.

## Why

The layout of text is the largest part of `svg` that a program cannot reach.
One API that reuses it gives the same text as the SVG path, so a program can
mix its direct drawing with `svg` and get the same pixels. A public tree
builder was not chosen, because it differs more from usvg and is larger.
