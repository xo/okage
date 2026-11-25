# Differences from upstream

This document lists where okage differs from its upstream projects, and the
APIs that okage adds. Each item names the decision that sets it. A decision
is in [decisions/](decisions/README.md).

## Differences

okage differs from upstream only where a decision says so:

1. The SIMD build computes a reciprocal exactly. The SIMD backends of
   tiny-skia approximate it in the blend modes color dodge and color burn,
   and its scalar backend computes it exactly (D16).
2. `svg` decodes PNG, JPEG and GIF files with the standard library, and WebP
   files with `golang.org/x/image/webp`, in place of the decoders of resvg
   (D7).
3. `svg` finds fonts and shapes text with `typesetting`, the copy of
   [`go-text/typesetting`][typesetting] in okage, in place of the crates of
   resvg. Its text can differ from the text of resvg (D9, D52).
4. Each package logs to a `*slog.Logger` that the caller gives (D20).
5. An elliptical arc makes at most 4,096 cubic curves. kurbo has no limit, so
   an arc with a radius of about 4.23e20 or more does not end in upstream
   (D30).
6. The gamma stages give one result for every NaN. Upstream reads the bits
   of a NaN there, and they differ between x86 and ARM (D34).
7. The fallback of text skips a character that no font draws, and goes on
   with the characters after it. Upstream stops at that character, so the
   characters after it get no fallback (D39). The fallback also replaces
   whole clusters, where upstream copies glyphs one by one, so an emoji
   sequence such as a flag falls back with the text around it (D39).
8. The fallback of text also goes on for an emoji that the font does not
   draw as an emoji: a character with U+FE0F without a color glyph, such as
   "©️" in a text font, and an emoji sequence that the font draws as several
   glyphs. Upstream falls back only for a missing glyph (D40).

9. `lottie` draws as ThorVG where velato differs: the opacity, the opacity
   stops, the radius and the highlight of gradients, solid layers, time
   remap, easing, spatial positions, the closing of animated shapes, the
   opacity and the blend mode of layers, masks and stroke dashes (D43).
   `Renderer.MergePaths` draws merge paths as After Effects combines them,
   which velato and ThorVG do not, and it is off by default. A fill of the
   result draws by compositing (D42). A stroke or a trim path of the result
   draws the path of a geometric boolean operation of `internal/pathbool`
   (D94).
10. `html` builds a tree with a node for each run of text, where tinyhtml5
    builds an ElementTree of Python. It goes on where upstream raises an
    error, and options bound the depth, the nodes and the issues of a parse
    (D49).
11. `tinycss2` nests at most 256 blocks and `cssselect2` at most 32
    selectors, where upstream has no limit. Where upstream raises an
    exception for a selector or a color, the port returns a `SelectorError`
    or no color (D50).
12. A fill clamps each side of the bounds of a path to 2^26 pixels from the
    origin before it rounds them. Upstream panics or leaves out rows for an
    antialiased fill with a point past about 2^29 pixels, and draws nothing
    for a fill without antialiasing of a path wider or taller than 2^31
    pixels. `DrawPixmap` of an image that is aligned to the pixels skips the
    stages of the pipeline, with the same bytes (D53).
13. A hairline on a pixmap of 1 or 2 rows or columns draws its pixels.
    Upstream draws nothing there (D54).
14. The subsetter of fonts of `internal/subsetter` runs at most 1,000,000
    operators for one glyph of a CFF font, and makes at most 1,000,000
    instructions. Past either limit, it returns an error. Upstream has no
    such limit, so a small font can make it run for a very long time. It
    also returns `okage.ErrSubsetTooLarge` when a subset grows larger than
    8 times the font data plus 1 MiB (D74). Where
    subsetter panics on a font, the port returns an error. The package
    comment of `internal/subsetter` lists the other differences (D62).
15. `pdf` writes no XMP metadata, where krilla writes it by default, and it
    has no validators of PDF/A and PDF/UA. It compresses with
    `compress/zlib`, so the bytes of a compressed stream differ from krilla,
    and its data does not. It returns an error where krilla panics, such as
    for a box of a page that does not fit the coordinates of PDF, for a
    pop of a surface without a push, for a gradient with a transform that
    cannot be inverted, or for the stops of a gradient in two color spaces. It also returns an error
    for a path to stroke with a point that is NaN or a coordinate above
    2^60, where krilla can panic or not end. `Document.StartPage` and
    `Document.Finish` finish the page that is open, and `Page.Finish`
    finishes the surface of the page, which Rust does not let a program
    leave open (D62, D66). It also returns an error for a destination to a
    page that does not exist, and for a link with no target, where krilla
    panics.
16. The stroker of `path` treats a quadratic curve as a line where the
    products of `findQuadMaxCurvature` overflow to NaN. tiny-skia panics
    there (D66).
17. The images of `pdf` decode with `image/png`, `image/jpeg`, `image/gif`
    and `golang.org/x/image/webp`, where krilla uses the crates png,
    zune-jpeg, gif and image-webp. `pdf` reads the headers of the files as
    those crates do, so it writes the same objects. The decoded samples of
    an image hold at most 256 MiB, and above that a function that makes an
    image returns an error. krilla has no such limit. `ImageFromImage`
    takes an `image.Image` of Go in place of the trait `CustomImage` of
    krilla (D62).
18. The text of `pdf` reads the fonts with `typesetting`, by the rules of
    skrifa, and shapes a text with `typesetting/harfbuzz` in place of
    harfrust. It shapes a text with no language, as harfrust does, where
    `typesetting` takes the language of the locale. It returns an error for
    a glyph whose range is not a range of its text, where krilla panics.
    The package comment of `pdf` lists the other differences of the text
    (D62, D65).
19. `NewLinearGradient` takes the stops of a degenerate gradient with the
    spread mode repeat or reflect in the order of `newGradient`, and clamps
    the average color to 0..=1. tiny-skia panics there for stops that are
    not in order. Stops in order give the bytes of tiny-skia (D68).
20. `pdfsvg` draws the tree of `svg`, which follows a later usvg than
    krilla-svg. A group without a layer bounding box draws no filter. A
    group with the blend mode `PlusLighter`, which PDF does not have, is
    drawn into an image with `svg`, with the groups that it blends with.
    `svg` draws the filters into images, where krilla-svg uses resvg.
    `Draw` and `RenderGlyph` return false where krilla-svg panics. The
    package comment of `pdfsvg` lists the changes (D62).
21. `pdf` writes a stitching function of more than 256 functions as a tree
    of stitching functions. Each one holds at most 256 functions, because
    MuPDF 1.28 reads no more. krilla writes one stitching function. The
    tree draws the same gradient (D69).
22. `Path.Stroke` and `PathStroker.Stroke` of `path` return no path, and
    `false`, for a path with a point that is NaN or infinite. tiny-skia does
    not end for some of these paths. A path whose points are finite gives
    the bytes of tiny-skia (D74).
23. When the subset of a font fails, `pdf` draws the glyphs of that font
    as paths, with the fill and the stroke of the text. The text stays in
    the file as invisible text, in the font without its font program, so
    that a reader can copy and search it. `Document.Finish` returns no
    error for the font, and logs it to the logger of `pdf.WithLogger`.
    krilla returns an error and writes no file. `pdf.WithSubsetFallback`
    with `false` gives the error of krilla. `pdf.CheckGlyphs` checks the
    ranges and the glyph IDs of a run before `Surface.DrawGlyphs`. It
    returns the error that `Document.Finish` returns for the run (D80).
24. `texmath` is a fork of go-tex/math, and fixes its faults (D78). Its
    parser makes a tree, and its layout makes a box of glyphs, rectangles
    and paths, where the source writes SVG text. The layout reads exact font
    units with `typesetting` at a size that is a `float64`, where the source
    rounds each value to a whole pixel. Limits bound the depth, the nodes,
    the macros, the items and the lengths, and their errors wrap the
    sentinel errors of the root package. The layout follows TeX Appendix G
    where the source approximates it: three sizes of scripts, the spaces of
    script style, a kern that is not an atom, the rules of a binary
    operator, the shifts of scripts, limits, fractions and stacks with all
    the constants of the MATH table, a radical sign that centers its body, a
    large operator on the math axis, and delimiters with
    `\delimiterfactor` and `\delimitershortfall`. The parts of an assembly go
    from the bottom up, so a tall parenthesis is not upside down. The
    additions of phase 26 of D78 use the italic correction, the top accent
    attachment and the math kerning of the MATH table, and an option turns
    each one off. They set `\text` and its kin with a text face, shaped with
    `typesetting/shaping`, or with the math alphabets of the math face, and
    keep the spaces of `\text`. They add `\color`, `\textcolor`,
    `\colorbox`, `\fcolorbox`, `\fbox`, `\cancel`, `\phase`, `\tag`,
    `\hline`, `\cfrac`, `\kern`, `\hspace`, `\llap` and the other missing
    commands of KaTeX that Mermaid labels use, `\newcommand`, and the macros
    of KaTeX such as `\R`. `\overbrace` and `\underbrace` draw a brace that grows,
    the wide accents grow, `\middle` grows to its fence, `\smash` removes
    the extent, and the size commands such as `\large` change the size. The
    package comment of `texmath` lists each fault with its line in the
    source, and each addition. On 2026-11-25, D95 added the rest of the
    commands of KaTeX that `texmath` did not parse: the symbols and the
    operators of AMS, the accents and the symbols of text, the arrows under
    a base, `\sideset`, `\vcenter`, `\angl`, `\sout`, `\verb`, the
    environments `align`, `gather`, `equation` and `rcases`, and the others
    of D95. D95 also aligns `\cfrac[l]` and `\cfrac[r]`, gives the first
    parameter of `\newcommand` a default value, and dashes the rule of
    `\hdashline`. It reads `\char` followed by a backtick and `^`,
    `\operatorname x`, `\substack x`, a fourth argument of one token of
    `\mathchoice` and `\mkern3pt` as KaTeX does, where the source gives an
    error.

When upstream changes the code of a difference, okage removes its own code
and ports the change of upstream. D40 gives the steps for items 7 and 8, and
D42 and D43 give them for item 9.

## Additions

okage also adds APIs that upstream does not have:

1. A `Pixmap` gives an `*image.RGBA` that shares its bytes, and it implements
   `image.Image` and `draw.Image` (D19).
2. `svg.New` makes a `Renderer`, which renders SVG data into an
   `*image.RGBA` in one call, as `xo/resvg` does. It loads the fonts of the
   system on its first call, and sets each generic family to the first of a
   list of common fonts that is installed. The package registers the format
   `svg` with `image.RegisterFormat`, so `image.Decode` reads SVG data (D36).
   `FontDatabase.LoadOSFonts` loads the same fonts into a database of the
   program (D46).
3. `svg.ShapeText` lays out a run of text with the fonts, the fallback and
   the shaping of `svg`, and gives its advance width and the outlines of its
   glyphs as a path, so that a program can draw text without SVG data (D37).
   `TextRun.RenderColorGlyphs` draws its color glyphs, such as color emoji
   (D39).
4. `html.Parse` parses an HTML-like label of Graphviz, `label=<...>` in DOT,
   and `Label.Layout` lays it out as `dot` does: the boxes of the tables and
   the cells, the rules, the runs of text and the ports (D38).
5. `lottie.Parse` reads a Lottie file, and `lottie.RenderFrame` draws a
   frame of it onto an okage pixmap through `PixmapSink`, the sink of
   `lottie` that draws with okage (D41). The sink draws the image assets in
   PNG, JPEG, GIF and WebP, as velato leaves them to the sink. It decodes an
   image that the file embeds as a `data:` URL, and it reads an image file
   from the `fs.FS` of the option `lottie.WithFS` (D75). The sink also
   draws the Gaussian blur and the drop shadow of a layer, as ThorVG does.
   `lottie.Parse` reads the text layers, which velato does not parse. A text
   layer draws the glyphs of the list `chars` of the file. The sink shapes
   each other character with the fonts of the option
   `lottie.WithFontDatabase`. Without that option, it logs the character
   and draws nothing. The size, the fill and the stroke, the justification,
   the line height, the tracking and the box of a text document apply
   (D79). So do the text animators with the alignment of their groups, the
   capitals of the key `ca`, and the text on a path (D85). As in ThorVG, the
   blur and the shadow repeat the pixels at the edge of the canvas, the
   tracking of a text that the sink shapes multiplies its advances, and an
   image that turns by an angle fades its edge over one pixel of the image
   (D82). The option `lottie.WithQuality` sets the quality of the blur and
   the shadow, as the quality of ThorVG does, and the default is 50 (D94).
6. `lottie.Detect` finds a Lottie document from the start of a file, and
   `lottie.OpenDotLottie` reads a dotLottie archive. `lottie.Load` parses
   either one. `Composition.Image` draws a frame into an `*image.RGBA` of a
   given size, and `FrameCount` and `Duration` count and time the frames
   (D44). A composition of a dotLottie archive draws the image files of the
   archive (D75).
7. `FontDatabase.LoadFontData` loads a font in WOFF, WOFF2 or EOT, as well
   as TrueType and OpenType data. `svg.ConvertFontData` converts such a font
   to TrueType or OpenType data. `FontDatabase.FaceName` returns a string of
   the name table of a face, such as the version or the sample text (D45).
   The package `webfont` gives the same conversion to a program that does not
   use `svg` (D51).
8. `Paint.Coverage` fills a path with 8 by 8 samples in each pixel, in place
   of the 4 by 4 of tiny-skia, and `Paint.StrokeAdjust` moves a thin stroke
   onto the pixel grid, as stroke adjustment does in PDF (D54).
9. `MeshGradient`, which `NewMeshGradient` makes, fills a mesh of triangles
   with a color at each corner, for the shadings of the types 4 to 7 of PDF
   and PostScript. `SpreadModeDecal` makes a gradient transparent before its
   start and after its end, as the decal tile mode of Skia does (D55).
10. `MaskFromLuminance` and `Mask.ApplyTransfer` make the soft masks of PDF
    with a backdrop color and a transfer function. `Pixmap.RemoveBackdrop`
    and `Pixmap.Knockout` composite the groups of PDF that are not isolated
    and the knockout groups (D56).
11. `TextStyle` also takes the stretch, the variations, the features, the
    language and the spacing of a run, and `TextRun.Clusters` gives the
    bytes, the advance and the glyphs of each cluster. `FontDatabase` gives
    the parsed faces, the cmaps and the metrics that it keeps, and
    `AutoFontVariations` maps a CSS weight, stretch and style to the axes of
    a variable font. `FontDatabase.ColorGlyph` gives a glyph of COLR or of
    the table SVG as a tree (D57).
12. `svg.ReadDimensions` reads the size and the viewBox of an SVG image
    before the parse. `svg.FromXMLTree` and `svg.FromXMLNode` make a tree
    from a document of roxmltree, and `roxmltree.Builder` makes such a
    document without XML text. `svg.WithZeroSize` accepts an image of size
    zero (D58).
13. The package `typesetting/font/cff` exports the predefined charsets of
    CFF, and draws the accented glyphs of `seac` and the FontMatrix of each
    font dictionary (D59).
14. `internal/subsetter` returns an error where subsetter panics, such as on
    the glyph after 65,535 glyphs. Its closure of composite glyphs reads
    each glyph once, where upstream can take time and memory that grow with
    the square of the number of components, and it gives the same order. It
    normalizes the coordinates of a variable font in float32 with avar
    version 1, as `typesetting` does, where skrifa uses fixed point and
    avar version 2. It hashes the glyphs in little-endian order on every
    machine (D62).
15. `NewRadialGradientExtend` makes a radial gradient with the rule of the
    radial shadings of PDF and PostScript. A point takes the color of the
    largest t in 0..=1 whose circle has a radius of at least 0. Two flags
    let t go below 0 or above 1, as the key `Extend` of PDF does (D64).
16. `typesetting` applies five commits of go-text out of order, for the
    variable fonts of `internal/subsetter`. It maps a coordinate with the
    table avar as skrifa does, with a rounded division. A malformed font
    gives an empty glyph, an advance of 0 or an error where go-text panics
    (D65). It also applies the 23 commits of go-text that fix a panic, a
    loop or a missing bound on malformed input, out of order, and fixes two
    more panics of the feature indexes of GSUB and GPOS and of the table
    cmap (D67).
    `internal/subsetter` makes the instances of variable fonts with
    `typesetting`. `typesetting` computes the deltas of the variations in
    the 16.16 fixed-point numbers of OpenType, as skrifa does, so each
    instance is the instance of subsetter, byte for byte (D62, D84).
17. `pdf` adds what the PDF device of ivy needs and krilla does not have:
    `Surface.PushGroup` for the transparency groups with the flags `I` and
    `K`, `Mask.WithBackdrop` and `Mask.WithTransfer` for the entries `BC`
    and `TR` of a soft mask, the field `Extend` of a linear and a radial
    gradient, the paint `MeshGradient` as a shading of type 4,
    `Surface.FillStencil` for a stencil of 8 bits,
    `Surface.PushStrokeAdjustment` for the entry `SA`,
    `Surface.PaintFromShader`, which converts a shader of okage, and
    `Surface.DrawRaster`, which draws a part of a page with okage as an
    image, with the option `WithRasterArea`. `pdfsvg` draws the filters of
    SVG with `Surface.DrawRaster` too. The shaders of okage give their
    parameters with the method `Parameters` for that conversion (D68).
18. `StrokeDash` gives its dash array and its dash offset with the methods
    `Array` and `Offset`. A writer of PDF writes the dashes of a stroke
    with them (D69).
19. `typesetting/font` reads the OpenType MATH table, which go-text does not
    read. `Font.HasMath`, `Face.MathConstant` and the other methods of D78
    give its values in font units, as the API `hb-ot-math` of HarfBuzz does.
    A malformed MATH table is left out of the font, and never panics (D78).
    `Face.KernPair` and `Font.LigaturePair` give the kern and the ligature
    of one pair of glyphs, for a TeX engine (D95).
20. `texmath.Parse` reads TeX math into a tree of nodes, and a program can
    build such a tree itself, for example from MathML. `texmath.Layout` lays
    out a tree with a face that has a MATH table, and `texmath.LayoutTeX`
    parses and lays out TeX math. The `Box` that they return gives its
    width, its height, its depth and its items. `Box.Draw` fills it into a
    pixmap, and `Box.Path` and `Box.ItemPath` give its outlines without a
    `FontDatabase`. The options `WithColor` and `WithLogger` set the color of
    the items and log a character without a glyph. `WithTextFace` and
    `WithTextFamily` give the faces of `\text` and its kin (D78).
    `WithLineWidth` gives the width of the line, so that `\tag` puts its
    label at the right margin (D95).
21. `NewRectMask` makes a mask that is 255 on a rectangle and 0 elsewhere,
    and holds the rectangle in place of its pixels. A draw through it gives
    the same bytes as through a mask of those pixels, and skips the stage
    `mask_u8` inside the rectangle. The first method that reads or writes
    the pixels of the mask makes them (D86).
22. `segmenter.LineBreaks` computes only the line breaks of a paragraph,
    without the graphemes and the words of `Segmenter`, and
    `Paragraph.LineBreak` uses it (D92).
23. `HarfbuzzShaper.SetUnsafeToConcat` makes the shaper give the flag
    unsafe-to-concat of HarfBuzz, and `Glyph.UnsafeToConcat` reads it. The
    flag is the flag of HarfBuzz 15.1.0 for each line of `perf_reference`
    (D93).

## Tests

The output of `svg` is compared with resvg and with libvips within a
tolerance, not byte for byte (D23). The renderer gives the same bytes as
tiny-skia.

[typesetting]: https://github.com/go-text/typesetting
