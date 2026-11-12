# Other vector formats

This document records a survey of the work to read, render or write other
vector formats in okage, such as PDF, EPS, Adobe Illustrator and DWG. The
survey is from 2026-11-11. It is input for a discussion. It is not a plan and
not a decision. Nothing in it is decided, and work on a format starts only
after the maintainers decide it.

## How the survey was made

Two outside models gave estimates: Gemini Pro and GPT-6 Astra. Other models
did not answer in time. The source repositories that the models named were
then examined on GitHub, for their license, size and scope.

The two models disagree by about four times on the cost. Gemini assumes a
port of existing code. GPT assumes production quality on hostile input. The
estimates are in months of work for one person.

## The formats

| Format | Source to port (license) | Gemini | GPT-6 | Assessment |
| --- | --- | --- | --- | --- |
| PDF writer | [krilla][krilla] (Apache-2.0, 1.4 MB of Rust) and [svg2pdf][svg2pdf] (Apache-2.0, 0.35 MB). svg2pdf already takes a usvg tree | 1.5 | 8 to 14 | The easiest item of high value. svg2pdf maps directly onto the tree of `svg`. The subsets of fonts are most of the work |
| PDF reader and renderer | [hayro][hayro] (Apache-2.0 or MIT, 2.6 MB of Rust), with the crates syntax, interpret, font, cmap, ccitt, jbig2, jpeg2000, svg and write | 5 | 18 to 30 | About 9 to 15. Its README says that it renders almost every file that Chrome and Firefox render. It does not do knockout groups or the blending of CMYK |
| Adobe Illustrator (`.ai`) | The PDF reader | 0.5 | 3 to 6 | Small after the PDF reader, but only for files that Illustrator saves as compatible with PDF. An older `.ai` file is PostScript |
| EPS and PostScript | No good source in Rust. [hayro-postscript][hayro-postscript] only reads the tokens of CMaps. In Go: [seehuhn/go-postscript][go-postscript] (BSD-3, "rudimentary") and [llgcode/ps][llgcode-ps] (BSD-2, from 2021) | 2.5 | 18 to 30 | A full virtual machine for a language, with no source to port. A small version reads only the EPS that Illustrator and Inkscape write |
| DXF | [dxf-rs][dxf-rs] (MIT, 0.7 MB) | 1 | 8 to 14 | The parser is easy. Blocks, line types, hatches, dimensions, SHX fonts and paper space take most of the time |
| DWG | [acadrust][acadrust] (MPL-2.0, DXF and DWG from R12 to R2018 and later, read and write). [unitedrhino/go-cad][go-cad] (Apache-2.0, pure Go, 2.4 MB, imports `x/image` and `x/text`). [LibreDWG][libredwg] is GPL-3, and okage cannot use it | 1.5 | 24 to 42 | acadrust makes this a port, not reverse engineering. It is still large. Read its notice about the source of its DWG code first |
| EMF, EMF+ and WMF | [metafile-rs][metafile-rs] (Apache-2.0, 0.5 MB, 3 stars). [emfsdk][emfsdk] has 0 stars | 1 | 12 to 22 | The work is to replay the state of GDI and GDI+. Some raster operations need the pixels of the target |
| Lottie | [velato][velato] (Apache-2.0, 0.27 MB, from linebender) | 1.5 | 6 to 10 | Small and clean. It needs a parameter for time, because the tree of `svg` does not change over time. The package `lottie` is the port, as [D41][D41] decides, with the changes of D42 and D43 |
| XPS, VSDX and CDR | No good source. [libcdr][libcdr] is C++ under MPL | 1 to 3 each | 7 to 12 for XPS | Low value. Do them last |

## What matters more than the estimates

1. [D7][D7] forbids a port of a crate that decodes an image, and [D9][D9] forbids a port
   of a crate for fonts. A PDF reader needs [hayro-jbig2][hayro-jbig2],
   [hayro-jpeg2000][hayro-jpeg2000], [hayro-ccitt][hayro-ccitt] and
   [hayro-font][hayro-font], because [go-text][go-text] has no Type 1 or Type 3
   fonts. So the PDF reader needs the maintainers to amend D7 and D9 first.
   [seehuhn/go-jpx][go-jpx] (BSD-3) is a pure Go codec for JPEG 2000 that can replace
   the port of hayro-jpeg2000.
2. The tree of usvg cannot hold CMYK, ICC profiles, overprint, knockout
   groups, or the mesh shadings of types 4 to 7. [hayro-interpret][hayro-interpret] sends its
   output to an abstract `Device`. A `Device` that draws with okage fits
   better than a conversion of PDF into the tree of usvg. [hayro-svg][hayro-svg]
   also exists, so PDF to SVG to okage is a shortcut with less fidelity.
3. go-cad imports `x/image` and `x/text`, which is against [D6][D6]. It is also Go,
   so it does not fit the process of a port from Rust with a Rust oracle.
   acadrust fits that process.
4. hayro has 1,600 PDF files for regression tests, and svg2pdf has its own
   tests. Both are Rust, so the oracle of [D22][D22] works for them. Nothing
   similar exists for EPS or EMF.

## The suggested order

Both models suggest the PDF writer first. The survey suggests this order,
with the smallest ports and the cleanest licenses first:

1. The PDF writer, a port of svg2pdf and krilla.
2. Lottie, a port of velato.
3. DXF, a port of dxf-rs.
4. The PDF reader, after the maintainers amend D7 and D9, and `.ai` on top
   of it.
5. DWG, a port of acadrust, if CAD files matter.
6. EPS, EMF, XPS, VSDX and CDR, only when a user asks for them.

[krilla]: https://github.com/LaurenzV/krilla
[svg2pdf]: https://github.com/typst/svg2pdf
[hayro]: https://github.com/LaurenzV/hayro
[hayro-ccitt]: https://github.com/LaurenzV/hayro/tree/main/hayro-ccitt
[hayro-font]: https://github.com/LaurenzV/hayro/tree/main/hayro-font
[hayro-interpret]: https://github.com/LaurenzV/hayro/tree/main/hayro-interpret
[hayro-jbig2]: https://github.com/LaurenzV/hayro/tree/main/hayro-jbig2
[hayro-jpeg2000]: https://github.com/LaurenzV/hayro/tree/main/hayro-jpeg2000
[hayro-postscript]: https://github.com/LaurenzV/hayro/tree/main/hayro-postscript
[hayro-svg]: https://github.com/LaurenzV/hayro/tree/main/hayro-svg
[go-postscript]: https://github.com/seehuhn/go-postscript
[llgcode-ps]: https://github.com/llgcode/ps
[go-jpx]: https://github.com/seehuhn/go-jpx
[dxf-rs]: https://github.com/IxMilia/dxf-rs
[acadrust]: https://github.com/hakanaktt/acadrust
[go-cad]: https://github.com/unitedrhino/go-cad
[libredwg]: https://github.com/LibreDWG/libredwg
[metafile-rs]: https://github.com/Dantusaikamal/metafile-rs
[emfsdk]: https://github.com/KaiserY/emfsdk
[velato]: https://github.com/linebender/velato
[libcdr]: https://github.com/LibreOffice/libcdr
[go-text]: https://github.com/go-text/typesetting
[D6]: decisions/D006-the-goal-is-no-dependencies.md
[D7]: decisions/D007-each-crate-has-a-policy-port-subset-replace-or-none.md
[D9]: decisions/D009-text-uses-go-text-and-the-layout-of-text-of-usvg-is-ported.md
[D41]: decisions/D041-lottie-renders-lottie-animations-as-a-port-of-velato.md
[D22]: decisions/D022-the-test-module-holds-the-oracles-and-the-comparisons.md
