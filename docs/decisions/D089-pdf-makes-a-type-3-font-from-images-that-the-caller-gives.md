# D89. pdf makes a Type 3 font from images that the caller gives

Status: Proposed, amends D62 and D68.

## Context

On 2026-11-19 ivy asked okage for a Type 3 font made from images that the
caller gives, each with its text. Item 1 of "The PDF writer" in
[`../BACKLOG.md`](../BACKLOG.md) holds the request. A Type 3 font is a font
of PDF whose glyphs are content streams: each glyph draws paths or images.

`pdf` follows krilla, and makes a Type 3 font only from a `*Font`, the bytes
of a font file. A glyph with `COLR`, `sbix`, `CBDT` or `SVG` data goes into a
Type 3 font of its font, and `text_type3.go` draws it there (D62 item 9).
krilla draws a bitmap glyph only from PNG data. Each glyph of such a font
starts with the operator `d0`. That operator makes a colored glyph, whose
content sets its own colors. So krilla keys each glyph by its glyph ID and
the color of the fill. A glyph in two colors takes two codes of the font.

ivy has two cases with no font file:

1. The PDF files that ivy reads hold Type 3 fonts of bitmaps, such as the
   bitmap fonts of TeX. Each glyph draws an image mask: an image of 1 bit
   that the viewer fills with the current color. The port of hayro gives
   ivy each such glyph as a stencil of 0 and 255, with the pixels of the
   file. `pdfenc` writes it with `Surface.FillStencil`, so the written file
   holds an image mask on the page, not a glyph (D69). Each viewer scales
   an image of a page down in another way than a glyph of a Type 3 font. At
   72 dots for each inch, `repo/pdf/font-type3-widths-with-matrix.pdf`
   differs from its written copy by 13.9 through ivy, 27.5 through MuPDF
   and 39.4 through Poppler. The written glyphs also have no text, so a
   reader cannot copy or search them.
2. The layout of ivy draws a color glyph or a bitmap glyph, such as an
   emoji, as an image. So the glyph has no text in the file. ivy draws
   such a glyph from its own reader of fonts, and a bitmap that is not PNG
   data has no path through krilla.

Question 25 asked whether `pdf` makes a Type 3 font from the outlines that
the caller gives, and recommended it after the base port. No decision
records its API.

## Facts that hold for each option

1. A Type 3 font holds at most 256 codes. `type3FontMapper` already splits
   the glyphs of one `*Font` into as many Type 3 fonts as it needs.
2. The ToUnicode map, the `ActualText` spans and the checks of the ranges
   of the text come from `DrawGlyphs` and `CheckGlyphs` (D80). A caller that
   draws through `DrawGlyphs` gets them with no new code.
3. The operator `d1` makes a shape glyph. Its content sets no color, and the
   viewer fills it with the color of the text. A viewer can cache such a
   glyph as a bitmap. pdfTeX writes the glyphs of its bitmap fonts with
   `d1` and an image mask. `internal/pdfwriter` already has
   `StartShapeGlyph`, which writes `d1`.
4. krilla does not write `d1`. Its comment in `type3.rs` says that viewers
   draw a shape glyph badly when the fill is a gradient or when the text
   is stroked.
5. `FillStencil` writes an image mask of 1 bit only for a stencil of 0 and
   255 with a color (D69). An image mask cannot hold the coverage of an
   edge that is smooth.

## The options

### Option 1. No change to okage

ivy keeps the image masks of the page. It can draw the text again over
each glyph as invisible text, in a font of its own choice.

- Cost: none in okage. ivy must choose a font for the invisible text, and
  place each character over its glyph.
- Trade-off: the written file keeps the scaling of an image of a page, so
  the differences of case 1 stay. Each place of a glyph is an image on
  the page, and not a code of a font.

### Option 2. A `*Font` whose glyphs are images

A new constructor returns a `*Font` that holds no font file. The caller adds
its glyphs, and draws them with `DrawGlyphs`, as it draws any font. Each
glyph goes into a Type 3 font of that font.

```go
// NewImageFont returns a font whose glyphs are images that the caller
// gives with AddImageGlyph. unitsPerEm is the size of an em in the units
// of the glyphs. DrawGlyphs draws its glyphs into Type 3 fonts.
func NewImageFont(unitsPerEm float32, opts ...ImageFontOption) (*Font, error)

// ImageFontOption is an option of NewImageFont (D20).
type ImageFontOption func(*imageFontOptions)

// WithImageFontName sets the name of the Type 3 fonts in the file. The
// default is "unknown", as krilla names a font without a PostScript name.
func WithImageFontName(name string) ImageFontOption

// ImageGlyph is one glyph of an image font. Exactly one of Mask and Image
// is set.
type ImageGlyph struct {
	// Advance is the advance of the glyph, in the units of the font.
	Advance float32
	// Mask is a stencil of 0 and 255. A pixel of 255 takes the fill of the
	// text. The glyph is a shape glyph, with d1.
	Mask *image.Alpha
	// Image is the image of a color glyph, with d0.
	Image *Image
	// Transform maps the pixels of the mask or of the image, with y down,
	// onto the units of the font, with y up.
	Transform Transform
}

// AddImageGlyph adds a glyph to an image font and returns its ID. The IDs
// count from 0 in the order of the calls. A program can add glyphs until
// Document.Finish.
func (f *Font) AddImageGlyph(g ImageGlyph) (GlyphID, error)
```

The details:

1. The code is in `pdf/ivy_type3.go`, as the additions of D68 are. The
   port changes in `text.go`, for the choice of the font, and in
   `text_type3.go`, for the content of a glyph. Each such change has a
   comment that starts with `okage:` and names this decision.
2. Each glyph of an image font goes into a Type 3 font, and never into a
   CID font. The matrix of the Type 3 font scales by 1 / `unitsPerEm`, as
   krilla does. The widths are the advances of the glyphs. The bbox of the
   font holds the images of its glyphs.
3. A glyph with a mask starts with `d1`, and its content draws the image
   mask with no color (fact 3). Its key in the font has no color, so one
   code serves each color of the text. A glyph with an image starts with
   `d0`, and draws the image as krilla draws a bitmap glyph.
4. The fill of the text can be a color or a pattern, as PDF allows for a
   shape glyph. The gate tests a gradient fill with MuPDF and Poppler
   (fact 4). If a viewer draws it wrong, `DrawGlyphs` draws such a run
   through `FillStencil`, and the run then has no text. Stroked text of an
   image font is filled, as krilla fills each Type 3 font.
5. The font has a hash that comes from a number that each new image font
   takes in order. So two image fonts never share a font in the file. The
   hash does not appear in the file.
6. A `*Font` stays safe for concurrent use. `AddImageGlyph` and the
   serialization of the font share a mutex.

The errors wrap the sentinel errors that `pdf` already has (D21):

1. `NewImageFont` returns `okage.ErrPDFInvalidFont` for a `unitsPerEm` that
   is not finite or not above 0.
2. `AddImageGlyph` returns `okage.ErrPDFInvalidGlyph` for a glyph with
   both or neither of `Mask` and `Image`. It returns the same error for an
   advance or a transform that is not finite, and for a mask that is not
   all 0 and 255. It also returns it when the font holds 65,536 glyphs. It returns `okage.ErrPDFImage` for a mask that is
   larger than the limit of `FillStencil`.
3. `AddImageGlyph` on a font of `NewFont` returns
   `okage.ErrPDFInvalidFont`.

- Cost: about 3 days of one agent. The methods of `*Font` that read its
  tables need a branch for an image font. The serialization of a Type 3
  font draws a glyph of the caller. The gate needs new scenes.
- Trade-off: a caller draws an image font with the calls that it already
  makes. It gets the text, the fonts of 256 codes and the `ActualText` of
  krilla with no new code. `*Font` gets a second form, as `Mask` did in
  D86, and each method of `*Font` must handle it. `d1` is new to the port.

### Option 3. A separate type and a separate draw call

A new type `Type3Font` holds the glyphs, and a new method draws them:

```go
func NewType3Font(unitsPerEm float32, opts ...ImageFontOption) *Type3Font
func (f *Type3Font) AddGlyph(g ImageGlyph) (GlyphID, error)
func (s *Surface) DrawType3Glyphs(start Point, glyphs []Glyph, font *Type3Font, text string, fontSize float32)
```

- Cost: about 4 days of one agent. The new call needs its own copy of the
  code of a text run. That code makes the groups, the ToUnicode map, the
  `ActualText` spans and the split into fonts of 256 codes.
- Trade-off: `*Font` and the port do not change. But the code of a text run
  exists twice, and a caller has two calls for glyphs.

### Option 4. Glyphs that the caller draws as streams

The caller draws each glyph onto the surface of a `StreamBuilder`, with
paths, images and image masks, and gives the stream with its advance:

```go
func NewStreamFont(unitsPerEm float32, opts ...ImageFontOption) (*Font, error)
func (f *Font) AddStreamGlyph(content *Stream, advance float32) (GlyphID, error)
```

- Cost: about 4 days of one agent.
- Trade-off: this covers the images, the outlines of question 25 and any
  glyph of a Type 3 font of a PDF file that ivy reads. But a stream of
  `pdf` always sets its colors, so each glyph is a colored glyph with `d0`.
  An image mask then takes its color inside the glyph, and a glyph in two
  colors takes two codes, as in krilla. A viewer does not cache such a
  glyph, so the scaling of case 1 can still differ. A shape glyph of paths
  needs a surface that writes no color, which is a larger change.

## What holds for options 2, 3 and 4

1. Same bytes: a document with no font of the caller keeps the bytes and
   the hashes of krilla. So the oracle of `test/krilla` does not change.
   The widths, the matrix and the bbox are floats that `internal/pdfwriter`
   writes as it writes each float of krilla (D16).
2. The gate: `TestIvyAdditions` of `test/krilla` gets four scenes of an
   image font. They hold mask glyphs in two colors, mask glyphs with a
   gradient fill, color glyphs, and more than 256 glyphs. MuPDF 1.28.6 and
   Poppler 26.11 render each scene within the threshold of 32 of D31, and
   `qpdf --check` accepts each file. `pdftotext` gives the text of each glyph.
   `pdf/ivy_test.go` tests the objects of each font, and `FuzzDocument`
   makes the new calls.
3. TinyGo (D25): the code uses `image`, which `pdf` already uses, and no
   `reflect`.
4. This is a deliberate difference from krilla, and `README.md` lists it.

## The recommendation

The recommendation is option 2. `pdfenc` can then write each bitmap glyph
of a source file as a shape glyph with its text, as the source file held
it. The layout can write an emoji as a color glyph with its text. Both
paths reuse the text code of krilla. A later decision can add a glyph of
paths to `ImageGlyph` for the outlines of question 25.

When the maintainers decide this decision, the statuses of D62 and D68 get
"amended by D89". Question 40 at the end of [`../PLAN.md`](../PLAN.md) asks
the maintainers to choose.
