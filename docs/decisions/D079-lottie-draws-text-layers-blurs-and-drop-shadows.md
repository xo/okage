# D79. lottie draws text layers, blurs and drop shadows

Status: Decided, amends D41, amended by D82.

velato does not parse a text layer of Lottie, so `lottie.Parse` returns an
error for a file with one, and the whole file does not draw. velato parses
the effects Gaussian blur and drop shadow, but `PixmapSink` draws neither
("lottie" in `docs/BACKLOG.md`). ThorVG, the second oracle of `lottie`
(D43), draws all three. The maintainers want `lottie` to draw them.

## The decision

1. `lottie` parses a text layer (`ty` 5): its text document with its
   keyframes, the font list `fonts` and the glyph list `chars` of the file.
   This is an addition to the port of velato, in files of okage that start
   with `okage_`, and the schema follows the format of Lottie as ThorVG reads
   it.
2. A text layer draws its glyphs as paths. When the file holds the glyph in
   `chars`, the layer draws that shape. Otherwise it shapes the text with
   `typesetting/shaping` and the fonts of a `*svg.FontDatabase` that a new
   option of the sink gives (D20). Without that option, a glyph that is not
   in `chars` draws nothing, and the sink logs it to its logger, if one is
   set.
3. A text layer supports the size, the fill and the stroke color, the
   justification, the line height and the tracking of its text document.
   Text animators and text on a path stay in the backlog.
4. `PixmapSink` draws the effects Gaussian blur and drop shadow of a layer,
   as ThorVG does: the blur with the box blur of the filters of `svg` and the
   size that ThorVG gives the sigma, and the shadow with the color, the
   opacity, the angle, the distance and the softness of the effect.
5. Each part has a test with a file made in the test, and the test module
   compares it with ThorVG within the tolerance of D43. A file whose text
   needs a font that ThorVG cannot load is compared only where both draw.

## When upstream changes

If velato parses text layers or draws these effects, port its commit as
`docs/UPSTREAM.md` says, and remove the addition of this decision that it
replaces. If the result is not the same, a maintainer decides which one
okage keeps.

## Why

A file with a text layer is common in Lottie, for titles and captions, and
today it does not draw at all. A blur and a shadow are the two effects that
ThorVG draws and okage does not.
