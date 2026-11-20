# D62. `pdf` writes PDF files, as a port of krilla

Status: Decided, amends D5, D7, D9 and D32, amended by D66, D68 and D80.

On 2026-11-18 a maintainer asked okage to write PDF output. The survey of
[`../FORMATS.md`](../FORMATS.md) rates a PDF writer as the easiest item of
high value, and names two sources: svg2pdf and krilla. This decision
chooses the source, the packages, the API, the tests and what okage does
where PDF and okage differ. "The PDF writer" in [`../PLAN.md`](../PLAN.md)
holds the phases and the units. Questions 20 to 27 at the end of that
document hold the choices that only the maintainers can make. This decision
records the recommendation for each one.

## What the sources are

The checkouts in `upstream/` were read on 2026-11-18. A line count leaves
out comments and blank lines, and it counts the tests inside a source file.

| Repository | Commit | License | Lines | What it holds |
| --- | --- | --- | --- | --- |
| [LaurenzV/krilla][krilla] | `e3459d06`, main, 2026-11-11. The last tag, `krilla-v0.8.2`, is 47 commits older | MIT or Apache-2.0 | 19,854 in `crates/krilla`, 961 in `crates/krilla-svg`, 8,239 of tests | `krilla`: the document, the pages, a surface to draw on, colors, gradients, patterns, masks, images, fonts and text, metadata, outlines, annotations, forms, tagging and the checks of PDF/A and PDF/UA. `krilla-svg`: draws a usvg tree onto a surface |
| [typst/pdf-writer][pdf-writer] | `fdf09012`, which krilla pins | MIT or Apache-2.0 | 9,574 | the low level writer: objects, dictionaries, streams, content operators, fonts, the ToUnicode map, functions, shadings, annotations and the structure tree |
| [typst/subsetter][subsetter] | `c3f23c67`, the tag `v0.2.6` that krilla requires | MIT or Apache-2.0 | 3,678 | the subsets of TrueType, CFF and CFF2 fonts for PDF. It converts a variable font at its coordinates into a static TrueType font |
| [typst/svg2pdf][svg2pdf] | `55723cb6`, 2026-07-10 | MIT or Apache-2.0 | 2,307 | an older converter of usvg trees on pdf-writer. Its README says that it is not maintained, and that a user takes krilla with krilla-svg |

krilla uses these crates. The column "okage" says what does the work in okage:

| Crate | Used by | okage |
| --- | --- | --- |
| `pdf-writer` | krilla | a port of the part that krilla calls |
| `subsetter` | krilla | a port |
| `siphasher` | krilla | a port of `sip128` only. krilla hashes the glyphs of a subset into the tag of its font name, so the hash is visible in the output |
| `xmp-writer` (2,150 lines) | krilla | a port, for the metadata of PDF/A. It waits for question 22 |
| `kurbo` | subsetter | the subset of D48 grows by `CubicBez::to_quads`, `approx_spline` and `QuadSpline` |
| `write-fonts` | subsetter | a port of `SimpleGlyph::from_bezpath` and the code that writes a simple glyph of `tables/glyf/simple.rs`, about 550 lines |
| `skrifa`, `read-fonts` | krilla, subsetter | `typesetting` (D52). The walk of the paint graph of COLRv1 is the port of D32 |
| `rustybuzz`, `harfrust` | krilla, for `draw_text` | `typesetting/harfbuzz` |
| `fontdb` | krilla-svg | the `FontDatabase` of `svg` |
| `usvg`, `resvg`, `tiny-skia`, `tiny-skia-path` | krilla, krilla-svg | `svg`, the root package and `path` |
| `flate2` | krilla | `compress/zlib`, at level 6 as krilla |
| `png`, `zune-jpeg`, `gif`, `image-webp`, `imagesize` | krilla | `image/png`, `image/jpeg`, `image/gif` and `golang.org/x/image/webp` (D6, D7), and a small reader of PNG chunks in `pdf` for the images that krilla copies without a decode |
| `base64` | krilla | `encoding/base64` |
| `bumpalo`, `yoke`, `smallvec`, `rustc-hash`, `indexmap`, `float-cmp`, `comemo`, `rayon` | krilla | none. Go does the work without them |
| `hayro-write` | krilla, with its feature `pdf` | none. It embeds the pages of another PDF file. A later decision can add it |

No source is under the GPL. The file `NOTICE.md` of krilla says that
`content_draw_path` came from resvg when resvg was under the MPL. resvg is
now under Apache-2.0 or MIT, and okage already ports it.

## The measurements

1. krilla built with cargo on 2026-11-18, and its examples wrote PDF files
   that `qpdf --check` accepts. `pdffonts` shows the fonts as embedded
   subsets of CID TrueType fonts, with a ToUnicode map.
2. A small program converted the 1,780 SVG files of the tests of resvg in
   `svg/testdata/tests` with krilla-svg, and 1,768 became PDF files. MuPDF
   rendered each one at the size of the reference image of resvg. The
   metric of D31 counts a pixel when a channel differs by more than 1. With
   it, only 6 files differ in 1 percent of their pixels or less. With a
   threshold of 32, 1,474 files differ in 1 percent or less, and 695 do not
   differ. MuPDF draws the edges of shapes with another anti-aliasing than
   tiny-skia. So a test cannot compare a PDF file of okage with the pixels of
   okage. It compares two PDF files through the same renderer.
3. krilla-svg 0.8.1 requires usvg 0.47. okage follows resvg up to
   `5bdbae64`, after 0.49. krilla-svg built against usvg at `500d8605` gives
   10 errors. The layer bounding boxes of a group are now optional. usvg has
   the new blend mode `PlusLighter`. usvg no longer gives the crate `fontdb`
   to its users.

## The decision

1. okage ports krilla and krilla-svg, not svg2pdf. svg2pdf is not
   maintained, and its own README sends a user to krilla. krilla also has
   the drawing API that ivy needs, which svg2pdf does not have. The base
   port holds the document, the pages, the surface, the graphics, the
   images and the text. It also holds the information dictionary, the
   outline, the page labels and the links. It leaves out these parts, and
   each one waits for a later decision:
   - the tagging for PDF/UA,
   - the forms,
   - the checks of PDF/A and PDF/UA, with their XMP metadata,
   - the embedded files and the separation colors,
   - the embedding of the pages of another PDF file.

   Question 22 asks for this scope.
2. The packages amend the table of D5:

   | Path | Package | What it holds |
   | --- | --- | --- |
   | `pdf` | `pdf` | the port of the crate `krilla`. A file starts with the name of its folder in `src`, such as `graphics_paint.go` and `text_cid.go` |
   | `pdfsvg` | `pdfsvg` | the port of the crate `krilla-svg`. It imports `pdf` and `svg`, so that a program that writes PDF without SVG does not link `svg` |
   | `internal/pdfwriter` | `pdfwriter` | the port of the part of `pdf-writer` that krilla calls |
   | `internal/subsetter` | `subsetter` | the port of `subsetter`, with the code of `write-fonts` that it calls |
   | `internal/siphasher` | `siphasher` | the port of `sip128` of `siphasher` |
   | `internal/colr` | `colr` | the walk of the paint graph of COLRv1 of D32, moved out of `svg`, so that `svg` and `pdf` share it |

   Question 21 asks for these names. The other choices are `krilla` in
   place of `pdf`, and the files of krilla-svg in `svg`.
3. The policies amend the table of D7. `pdf-writer` is a subset, and the
   subset grows with each phase. `subsetter` is a port, with the part of
   `write-fonts` that it calls. They are the second and the third crate for
   fonts that okage ports, after the walk of D32, so this decision amends D7
   and D9 for them. The table "What the sources are" above gives the
   policy of every other crate. Each ported repository gets a row in
   `docs/upstream/upstreams.tsv` and a ledger (D12) when the port starts.
4. The base commit of krilla is the head of its main branch, `e3459d06`.
   The base commit of pdf-writer is `fdf09012`, which krilla pins. The base
   commit of subsetter is `v0.2.6`, the version in the `Cargo.lock` of
   krilla. Question 20 asks for them.
5. The API follows krilla, in Go form. A `Document` starts pages. A `Page`
   gives a `Surface`. A `Surface` draws paths, glyphs, images and shadings,
   and pushes and pops transforms, clips, masks, opacities, blend modes and
   isolated groups. A `Font` is the bytes of a TrueType, OpenType or
   collection font, with an index and the coordinates of its variations. The
   struct `SerializeSettings` becomes functional options (D20). A Rust panic
   of krilla becomes an error that wraps a sentinel error of the root
   package (D21). The sketch:

   ```go
   doc := pdf.NewDocument(pdf.WithPDFVersion(pdf.PDF17), pdf.WithSVGGlyphs(pdfsvg.RenderGlyph))
   page := doc.StartPage(pdf.PageSettingsFromWH(595, 842))
   s := page.Surface()
   s.SetFill(&pdf.Fill{Paint: pdf.RGB(0, 0, 0), Opacity: 1, Rule: pdf.FillRuleNonZero})
   s.PushTransform(t)              // t is an okage.Transform
   s.DrawPath(p)                   // p is an *okage.Path
   s.DrawGlyphs(start, glyphs, font, text, 12, false)
   ok := pdfsvg.Draw(s, tree, size) // tree is an *svg.Tree
   s.Pop()
   s.Finish()
   page.Finish()
   err := doc.Finish(w)            // w is an io.Writer. krilla returns a Vec<u8>
   ```

6. `Document.Finish` writes the file to an `io.Writer`. krilla keeps every
   object in memory until it finishes, because the subsets of the fonts need
   every glyph of the document. The port keeps that model. Question 24 asks
   whether a later decision writes the streams of the pages and of the
   images while the program draws.
7. okage compresses with `compress/zlib`. Its bytes differ from the bytes of
   `flate2` of Rust, so the file of okage is not the file of krilla byte for
   byte. The tests compare the objects after they decode each stream. The
   objects, the content streams, the subsets of the fonts and the tags of
   the font names follow krilla byte for byte. Each float follows the rules of D16, and `siphasher` gives the
   same tags. The ID of the document is a hash of the compressed bytes, so
   it differs, and the tests skip it.
8. krilla-svg is ported to the tree of `svg`, which follows a later usvg
   than krilla-svg. Each change that the newer tree needs is a difference
   from upstream, with a comment that starts with `okage:` and names this
   decision. `PlusLighter` has no blend mode in PDF, so the group is drawn
   by the fallback of item 10. When krilla-svg moves to a newer usvg, follow
   the steps of "When upstream changes" below.
9. Text goes to PDF as krilla writes it:
   - A glyph of a font with a `glyf`, `CFF` or `CFF2` table and no color
     data goes into a CID font. `subsetter` makes the subset. A CFF font is
     a `CIDFontType0` with its CFF program. Any other font is a
     `CIDFontType2`. A variable font is instanced at its coordinates, and a
     CFF2 font becomes a TrueType font.
   - A glyph with `COLR`, `sbix`, `CBDT` or `SVG` data goes into a Type 3
     font. The paths and gradients of COLR become PDF graphics, and a
     bitmap becomes an image. The function of `WithSVGGlyphs`, which
     `pdfsvg` gives, draws an SVG glyph.
   - The ToUnicode map comes from the text that the caller gives with the
     glyphs, as the `UnicodeCmap` of pdf-writer writes it. Where the text of
     a cluster does not map to its glyphs one to one, krilla writes an
     `ActualText` span. If the caller gives no text, the glyph has no entry
     in the map, and a reader cannot copy or search it. Question 27 asks
     whether okage then takes the text from the `cmap` table of the font.
   - A font that is restricted by its `OS/2` table is still embedded, and
     only a check of PDF/A reports it, as krilla does.
   - A font that is not TrueType or OpenType, such as a Type 1 font of a PDF
     file that ivy reads, is out of scope of the base port. The caller draws
     its glyphs as paths. Question 25 asks whether okage later makes a
     Type 3 font from the outlines that the caller gives.
   - A WOFF, WOFF2 or EOT font is converted with `webfont` before the caller
     makes a `Font`.
10. Some things that okage draws have no form in PDF. The writer draws such
    a part with okage into a pixmap, at a scale that an option sets. Then it
    embeds the pixmap as an image. krilla-svg does this for the filters of SVG
    with the scale `filter_scale`, and the writer uses the same code. The
    mapping of "The PDF writer" in [`../PLAN.md`](../PLAN.md) lists each case.
11. ivy keeps its own device for PDF output, which maps the calls of its
    display list onto a `pdf.Surface`. okage adds to `pdf` only what that
    device needs and krilla does not have. Each addition is a deliberate
    difference from upstream. It goes in the list "Differences from
    upstream" of `README.md`, and in a section of this decision, when it is
    built. The additions are:
    - a transparency group with the knockout flag,
    - a soft mask with a backdrop color and a transfer function,
    - a Type 4 shading for the mesh gradients of D55,
    - a fill through a mask of 8 bits, for a glyph bitmap or a stencil,
    - the stroke adjustment of the graphics state,
    - a conversion from an `okage.Shader` to a paint of `pdf`,
    - the fallback of item 10, for a caller that is not `pdfsvg`.

    Question 23 asks for this split. D68 holds the section of each
    addition, with its API and the results of its gate.
12. The test module holds the comparison (D22). The Rust oracle links krilla,
    krilla-svg and subsetter at their matched commits. The tests use MuPDF
    `mutool`, Poppler `pdftoppm` and `qpdf`, which run as outside programs,
    as `dot` runs for `html` (D38). A test skips when a program is missing.
    "Testing" under "The PDF writer" in [`../PLAN.md`](../PLAN.md) holds the
    gates. Question 26 asks for this.
13. `pdf`, `pdfsvg` and the internal packages build with TinyGo (D25). Each
    one keeps the MIT license of its crate, with the copyright lines of the
    crate and of okage (D13).
14. When the maintainers decide this decision, the statuses of D5, D7, D9
    and D32 get "amended by D62".

## When upstream changes

The port follows krilla, pdf-writer and subsetter with their ledgers (D12).
For a difference of item 8 or item 11, follow these steps:

1. If krilla moves krilla-svg to the usvg that okage follows, port that
   commit, and remove each `okage:` change that the commit makes the same.
2. If krilla adds a feature of item 11, such as knockout groups, port it.
   If the result is the same as the addition of okage, remove the addition
   and its line in `README.md`. If it is not the same, a maintainer decides
   which one okage keeps, and the choice is recorded here.

## Why

krilla is the maintained source, and it holds what svg2pdf held. Its surface
is close to the display list of ivy, so ivy needs a small device and no
second writer. The crates that it uses for fonts and images are already
replaced in okage, except the subsets of fonts, which no Go package in okage
makes. subsetter is small, at 3,678 lines, and a port of it gives the same
font bytes as krilla, which a test can compare byte for byte. A PDF viewer
draws the file with its own renderer, so the pixels of tiny-skia cannot be a
gate. Two PDF files through the same viewer can.

[krilla]: https://github.com/LaurenzV/krilla
[pdf-writer]: https://github.com/typst/pdf-writer
[subsetter]: https://github.com/typst/subsetter
[svg2pdf]: https://github.com/typst/svg2pdf
