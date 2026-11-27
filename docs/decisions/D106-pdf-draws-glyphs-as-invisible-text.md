# D106. `pdf` draws glyphs as invisible text

Status: Decided, amends D68.

## The problem

The PDF writer of ivy, `pdf/pdfenc`, draws the text of the CSS property
`-ivy-text-outline` as filled outlines, which are paths. It also draws the
same glyphs as text over the outlines, so that a reader can search and copy
the text. That text must not paint.

krilla has no public way to write text that does not paint. ivy draws the
text with a fill opacity of 0. Each such run then adds a graphics state with
`/ca 0`, and a reader still treats the text as painted. In PDF, text that
does not paint takes the text rendering mode 3, as the text layer of a
scanned page does. okage writes the mode 3 only in the fallback of D80,
which a program cannot call.

ivy asked for a change on 2026-11-28, as item 2 of "Work for okage" in its
backlog. The maintainers asked for it on the same day.

## The rule

1. The new method `Surface.DrawInvisibleGlyphs` takes the arguments of
   `DrawGlyphs` without `outlined`:

   ```go
   func (s *Surface) DrawInvisibleGlyphs(start Point, glyphs []Glyph, font *Font, text string, fontSize float32)
   ```

2. The run is a text object with the text rendering mode 3. It embeds the
   font and writes the ToUnicode map, the ActualText spans and the second
   CIDs of D102, as a visible run does.
3. The fill and the stroke of the surface do not apply. The run writes no
   color, no width of a line and no graphics state, inside its own pair of
   `q` and `Q`. The runs after it do not change.
4. A run of a font of Type 3 also takes the mode 3. Examples are a font of
   color glyphs and an image font of D89. krilla fills a visible run of
   such a font in every case.
5. The method runs `CheckGlyphs` as `DrawGlyphs` does. For a glyph that
   fails the check, it records the error, which `Document.Finish` returns,
   and draws nothing.
6. In the fallback of D80, the run stays in the mode 3, and draws no path.
7. Inside an XObject, the bbox holds the glyphs of the run, as for a
   visible run.

A separate method keeps the signature of `DrawGlyphs`, which follows
krilla. A rendering mode as an argument of `DrawGlyphs` changes each call
of it.

## The code

`pdf/ivy_text.go` holds the method. `fillStrokeGlyphGroup` of
`pdf/content.go` writes the mode 3 before it looks at `forceFill`, with a
comment that starts with `okage:` and names this decision. No input of
krilla reaches the mode 3, so the bytes of krilla do not change.

## Tests

`pdf/ivy_text_test.go` checks the content stream. It finds one `3 Tr`, no
color and no `gs`, and no change to the runs after it. It also covers the
fonts of Type 3, the fallback of D80 and the error of `CheckGlyphs`.

`TestInvisibleText` of `test/krilla` draws a TrueType font, a CFF font and
a font of color glyphs. MuPDF 1.28 and Poppler 26.11 paint no pixel of the
page, and `pdftotext` and `mutool draw -F txt` extract the text.
