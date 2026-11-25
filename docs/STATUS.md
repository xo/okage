# Status

This document holds the state of each part of okage, and the upstream
commit that each part follows. [PLAN.md](PLAN.md) holds the plan, the phases
and the open questions.

## The packages

On 2026-11-25 each part of okage is in this state:

1. The port of tiny-skia is done, in the root package and in `path`. It gives
   the same bytes as tiny-skia in the scalar build, in the SIMD build and
   with TinyGo. The 157 integration tests of tiny-skia pass. The Rust
   oracle in `test/oracle` gives the same bytes on each random scene that
   has no cubic curve. okage is about 1.2 to 3 times slower than the scalar
   code of tiny-skia. okage has no speed target, and work that makes it
   faster is always welcome (D35, D96).
2. The package `svg` is the port of usvg and resvg. It parses and renders
   each test file of resvg. A file without text and without a raster image
   gives the same pixels as resvg. A file with text or an image is within
   the tolerance of D31. The package `filter` holds the Gaussian blur, the
   color matrix and the transfer functions of resvg for a caller, and `svg`
   and `lottie` call it (D88).
3. The package `lottie` is the port of velato. It gives the same calls to
   its sink as velato for each of the 879 Noto animated emoji. Each sampled
   frame of them is within the tolerance of D43 of ThorVG.
4. The package `html` lays out the HTML-like labels of Graphviz, with `dot`
   as its oracle (D38). It also parses HTML documents, as the port of
   tinyhtml5. It passes each test of html5lib-tests that tinyhtml5 passes
   (D49).
5. The packages `tinycss2` and `cssselect2` are the ports of tinycss2 and
   cssselect2, with the tests of their upstreams (D50).
6. The packages `pdf` and `pdfsvg` are the ports of krilla and krilla-svg.
   The test module compares their files with the files of krilla, and
   renders both with MuPDF and Poppler (D62). `pdf` writes the XMP metadata
   and the custom XMP metadata of krilla, embeds files, and has the
   validators of PDF/A, but not the validator of PDF/UA (D99).
7. `typesetting` is a copy of go-text. Its shaper gives the same glyphs and
   positions as HarfBuzz 15.2.0 for each of the 413,399 lines of
   `perf_reference`. This holds at the units per em of the font and at
   other scales (D73, D90, D97).
8. The package `texmath` is a fork of go-tex/math. It parses each formula of
   its corpus that KaTeX parses (D78, D95).

## The upstreams

The port starts from resvg `v0.49.0` and from the versions of the crates
that it uses, such as tiny-skia `v0.12.0`. After each base port, okage
follows each commit of the upstream. The column "Matched commit" names the
last upstream commit that okage matches on 2026-11-25. A ledger in
[upstream/](upstream/) records what okage did with each commit after the
base commit. [UPSTREAM.md](UPSTREAM.md) gives the rules.

| Upstream | Version of the base port | okage packages | Matched commit |
| --- | --- | --- | --- |
| [tiny-skia](https://github.com/linebender/tiny-skia) | tiny-skia 0.12.0, tiny-skia-path 0.12.0 | the root package, `path` | `5d475477` of 2026-08-20 |
| [strict-num](https://github.com/RazrFalcon/strict-num) | strict-num 0.1.1 | `path` | `76d36feb` of 2024-04-25 |
| [resvg](https://github.com/linebender/resvg) | usvg 0.49.0, resvg 0.49.0 | `svg`, `filter` | `1938e970` of 2026-11-23 |
| [svgtypes](https://github.com/linebender/svgtypes) | svgtypes 0.16.2 | `internal/svgtypes` | `723dad9c` of 2026-11-13 |
| [roxmltree](https://github.com/RazrFalcon/roxmltree) | roxmltree 0.21.1 | `roxmltree` | none yet, see the ledger |
| [simplecss](https://github.com/linebender/simplecss) | simplecss 0.2.2 | `internal/simplecss` | none yet, see the ledger |
| [kurbo](https://github.com/linebender/kurbo) | kurbo 0.13.1 | `kurbo` | none yet, see the ledger |
| [xmlwriter](https://github.com/RazrFalcon/xmlwriter) | xmlwriter 0.1.0 | `internal/xmlwriter` | none yet, see the ledger |
| [velato](https://github.com/linebender/velato) | velato 0.12.0 | `lottie` | `83336b65` of 2026-09-16 |
| [tinyhtml5](https://github.com/CourtBouillon/tinyhtml5) | tinyhtml5 2.1.0 | `html` | `2a933f5b` of 2026-10-25 |
| [tinycss2](https://github.com/Kozea/tinycss2) | tinycss2 1.5.1 | `tinycss2` | `2cab2705` of 2026-10-25 |
| [cssselect2](https://github.com/Kozea/cssselect2) | cssselect2 0.10.1 | `cssselect2` | `fc22accd` of 2026-10-25 |
| [typesetting](https://github.com/go-text/typesetting) | github.com/go-text/typesetting v0.3.5 | `typesetting` | `79cc91be` of 2026-11-20 |
| [krilla](https://github.com/LaurenzV/krilla) | krilla 0.8.2, krilla-svg 0.8.1 | `pdf`, `pdfsvg` | `7b15610d` of 2026-11-24 |
| [xmp-writer](https://github.com/typst/xmp-writer) | xmp-writer 0.3.3 at `9498ae2b` | `internal/xmpwriter` | `9498ae2b` of 2026-11-19 |
| [pdf-writer](https://github.com/typst/pdf-writer) | pdf-writer 0.15.0 | `internal/pdfwriter` | none yet, see the ledger |
| [subsetter](https://github.com/typst/subsetter) | subsetter 0.2.6 | `internal/subsetter` | `8221117c` of 2026-11-23 |
| [siphasher](https://github.com/jedisct1/rust-siphash) | siphasher 1.0.2 | `internal/siphasher` | none yet, see the ledger |
| [write-fonts](https://github.com/googlefonts/fontations) | write-fonts 0.48.0 | `internal/subsetter` | none yet, see the ledger |
| [go-tex-math](https://github.com/go-tex/math) | github.com/go-tex/math v0.50.0+11 | `texmath`, `texmath/stixtwomath`, `typesetting/font` | `e33cf138` of 2026-11-22 |
