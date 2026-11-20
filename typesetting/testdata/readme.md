# The test data of go-text

This folder holds copies of files of the module
`github.com/go-text/typesetting-utils`, byte for byte. The tests of the
packages of `typesetting` read them (D63). The version is
`v0.0.0-20261009215314-bb530f4842f3`, the commit `bb530f48`, which go-text
requires at its commit `bafa8730`. Each file keeps its path in the module.
For example, `opentype/common/Roboto-BoldItalic.ttf` of the module is
`opentype/common/Roboto-BoldItalic.ttf` here.

The package `typesetting/internal/testfonts` reads this folder from the
disk. Its packages `opentype` and `harfbuzz` take the place of the packages
of the same names of typesetting-utils, so a test of go-text changes only its
imports.

This folder is a module of its own, with its own `go.mod`, and it holds no Go
package (D76). So the module `github.com/xo/okage` does not hold it. A
program that requires okage does not download these files, and no binary
holds them. In the module cache, the folder is not there, and a test that
reads it skips.

okage copies only the files that the tests and the benchmarks read. The
module typesetting-utils holds about 165 MB. This folder holds about 170 MB
in 1,172 files: the fonts, the tests of the shaper, the texts of the
benchmarks, the license files, `go.mod` and this file.

## How the files were chosen

The tests of go-text at `bafa8730` ran with a copy of typesetting-utils whose
`Files` writes the name of each file that a test or a benchmark opens. Each
file in that list is here, also a font that is proprietary or that states no
license (D76).

If go-text requires a new version of typesetting-utils, do these steps:

1. Run the tests of go-text again with such a copy of typesetting-utils.
2. Copy each new file that the tests read. If no row of the tables below
   covers the file, add a row.
3. Remove each file that no test reads.

## Licenses

`LICENSE` is the license of typesetting-utils, the Unlicense or the
BSD-3-Clause license. It covers the files that are not fonts. Each font keeps
its own license, from its name table or from `opentype/attributions.md`,
which is the file of the module. The folder `licenses` holds the texts. The
fonts in "Files without a license" below have no license that lets okage
copy them.

| Files | License | Text in `licenses` |
| --- | --- | --- |
| `harfbuzz/harfbuzz_reference/aots/**` | Apache-2.0, Adobe | `aots/COPYING` beside the files |
| `harfbuzz/harfbuzz_reference/text-rendering-tests/**` | Apache-2.0, Unicode | `text-rendering-tests/COPYING` beside the files |
| `harfbuzz/harfbuzz_reference/in-house/**` | the test suite of HarfBuzz. Its `COPYING` says that the OFL-1.1 covers many of the fonts | `in-house/COPYING` beside the files |
| `harfbuzz/fonts/aat-feat.ttf`, `opentype/toys/Feat.ttf`, `opentype/toys/Sbix2.ttf`, `opentype/toys/chromacheck-svg.ttf` | MIT, ChromaCheck | `MIT-ChromaCheck.txt` |
| the other files of `harfbuzz/fonts` | OFL-1.1 | `OFL-1.1.txt` |
| `harfbuzz/perf_reference/fonts/Roboto-Regular.ttf` | Apache-2.0 | `Apache-2.0.txt` |
| the other fonts of `harfbuzz/perf_reference/fonts` | OFL-1.1 | `OFL-1.1.txt` |
| `opentype/toys/gpos/**`, `opentype/toys/gsub/**`, `opentype/cmap/CMAP12.otf` | Apache-2.0, the fonts of the Adobe test suite in `aots` | `Apache-2.0.txt` |
| `opentype/morx/**`, `opentype/cmap/CMAP14.otf`, `opentype/toys/GVAR-no-HVAR.ttf`, `opentype/toys/Trak.ttf`, `opentype/cff/ToyFD0.cff` | OFL-1.1, the fonts of the Unicode text rendering tests | `OFL-1.1.txt` |
| `opentype/common/Roboto-BoldItalic.ttf`, `opentype/common/open-sans-v15-latin-regular.woff`, `opentype/toys/GDEFCaretList3.ttf` | Apache-2.0 | `Apache-2.0.txt` |
| `opentype/common/DejaVuSans.ttf`, `opentype/common/DejaVuSansMono.ttf` | the license of Bitstream Vera and Arev | `DejaVu.txt` |
| `opentype/common/Go-Mono-Bold-Italic.ttf` | BSD-3-Clause, Bigelow & Holmes | `BSD-3-Clause-Go-fonts.txt` |
| `opentype/toys/CFFTest.otf`, `opentype/toys/segments.ttf` | BSD-3-Clause, The Go Authors | `BSD-3-Clause-Go.txt` |
| `opentype/toys/3cmaps.ttc` | MIT, fontTools | `MIT-fonttools.txt` |
| `opentype/common/FreeSerif.ttf`, `opentype/cff/Free*.cff` | GPL-3.0 or later, with the font exception | `GPL-3.0.txt`, `GPL-font-exception.txt` |
| `opentype/common/OldaniaADFStd-Bold.otf`, `opentype/cff/*ADF*.cff` | GPL-2.0 or later, with the font exception | `GPL-2.0.txt`, `GPL-font-exception.txt` |
| `opentype/toys/KacstQurn.ttf` | GPL-2.0 | `GPL-2.0.txt` |
| `opentype/cff/C059-*.cff`, `opentype/cff/D050000L.cff`, `opentype/cff/Nimbus*.cff`, `opentype/cff/P052-*.cff`, `opentype/cff/URWGothic-DemiOblique.cff`, `opentype/cff/Z003-MediumItalic.cff` | AGPL-3.0 with the font exception, (URW)++ | `AGPL-3.0.txt`, `GPL-font-exception.txt` |
| `opentype/common/LiberationMono-Italic.ttf` | the Liberation Fonts license 1, which is GPL-2.0 with exceptions | `Liberation-1.txt`, `GPL-2.0.txt` |
| `opentype/common/Lmmono-italic.otf` | the GUST Font License, which is LPPL-1.3c | `GUST-FONT-LICENSE.txt`, `LPPL-1.3c.txt` |
| `opentype/common/mplus-1p-regular.ttf` | the M+ FONTS license | `mplus.txt` |
| `opentype/bitmap/cherry-10-r.otb` | 0BSD | `0BSD-cherry.txt` |
| `opentype/bitmap/unifont-15.1.05.otf` | OFL-1.1, or GPL-2.0 or later with the font exception | `OFL-1.1.txt` |
| each other font | OFL-1.1, as its name table or `opentype/attributions.md` says | `OFL-1.1.txt` |

## Files without a license

These files have no license that lets okage copy them. okage holds them for
the tests and the benchmarks of `typesetting` only, as a fair use (D76). Do
not use them for any other work.

| File | What it is |
| --- | --- |
| `opentype/bitmap/simsun.ttc` | a font of Microsoft |
| `opentype/collections/msgothic.ttc` | a font of Microsoft |
| `opentype/collections/Courier.dfont` | a font of Apple |
| `opentype/collections/Gacha_9.dfont` | a font of Apple |
| `opentype/cff/AdobeMingStd-Light-Identity-H.cff` | a font of Adobe that is not free |
| `opentype/bitmap/IBM3161-bitmap.otb` | a font that states no license |
| `opentype/cmap/Brushstroke-Plain.otf` | a font that states no license |
| `opentype/toys/tables/*` | tables and glyphs of fonts that the module does not name |
| `opentype/cmap/table/*` | tables of fonts that the module does not name |
| `harfbuzz/fonts/TestGVAREight.ttf` | a font whose license gives no right to distribute it |
| `harfbuzz/fonts/adwaita.ttf`, `harfbuzz/fonts/cv01.otf` | fonts of the tests of HarfBuzz, with no license file |
| `harfbuzz/harfbuzz_reference/in-house/fonts/5af5361ed4d1e8305780b100e1730cb09132f8d1.ttf` | Iskoola Pota, a font of Microsoft |
| `harfbuzz/harfbuzz_reference/in-house/fonts/ab14b4eb9d7a67e293f51d30d719add06c9d6e06.ttf` | Zawgyi-One, which holds glyphs of Tahoma, a font of Microsoft |
| `harfbuzz/perf_reference/texts/*` | the texts of the benchmarks of HarfBuzz, with no license file. Some are translations of a book under copyright |

The other fonts of `harfbuzz/harfbuzz_reference/in-house` come from the tests
of HarfBuzz. Its `COPYING` says that the OFL-1.1 covers many of them, not all.
