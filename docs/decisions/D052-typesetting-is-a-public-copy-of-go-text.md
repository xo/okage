# D52. typesetting is a public copy of go-text

Status: Decided, amends D5, D6, D7, D9, D12 and D13.

okage read fonts, shaped text and broke lines with the module
`github.com/go-text/typesetting` `v0.3.5` (D6, D9). ivy draws the text of its
documents with okage, and it uses the same module. A visual review of ivy
against MuPDF, Poppler and LibreOffice on 2026-11-17 found that the text of
ivy looks different. The maintainers want to fix such faults where the code
is, without a wait for a release of go-text. On 2026-11-17 they decided to
copy go-text into okage, as a public package, and to fix it there.

## The decision

1. The packages of go-text `v0.3.5`, the commit `19482bd5`, are in the module
   `github.com/xo/okage`, under `typesetting/`. Each package keeps its name
   and its place:

   | go-text | okage |
   | --- | --- |
   | `github.com/go-text/typesetting/<path>` | `github.com/xo/okage/typesetting/<path>` |
   | `bidi`, `di`, `font`, `font/cff`, `font/cff/interpreter`, `font/opentype`, `font/opentype/tables`, `harfbuzz`, `language`, `segmenter`, `shaping` | the same path under `typesetting/` |
   | `internal/unicodedata` | `typesetting/internal/unicodedata` |
   | `testutils` | `typesetting/testutils`. Only the tests use it, and it stays public, so that a commit of go-text applies as it is |
   | `fontscan` | none |

2. okage does not copy `fontscan`. It finds the fonts of a system in their
   folders and with the files of Fontconfig, and it keeps a cache on the
   disk. `svg` loads fonts with its own `FontDatabase` (D9, D36, D46), and ivy
   does not use `fontscan`.
3. The copy is a copy, not a port. Its Go code stays close to its source, so
   that a commit of go-text is easy to apply. A change of the copy has a
   comment that starts with `okage:` and names its decision. On 2026-11-17 the
   copy has these changes:
   - `font/svg.go` reads the root element of an SVG glyph document with the
     package `roxmltree` of okage, not with `encoding/xml` (D25). roxmltree
     refuses a document that is not well formed. `encoding/xml` reads only up
     to the root element.
   - The two `go:generate` lines are comments. Their generator is in the
     module `github.com/go-text/typesetting-utils`, which okage does not
     require.
   - `testutils` has no `Filenames`, because it reads the fonts of
     `typesetting-utils`.
   - `gofmt` adds the last newline of `internal/unicodedata/unicode_test.go`.

   The files keep the import groups of go-text, and the formatters of
   `golangci-lint` do not run on them.
4. 33 of the 54 test files of these packages read the fonts and the test
   data of `typesetting-utils`, which are about 165 MB. The copy leaves them
   out, and keeps the test of the view box of `font/svg_test.go`. The other 21
   test files pass, with the data of the tests of Unicode in `bidi/test` and
   `segmenter/test` and the fonts in `font/testdata`, copied byte for byte.
   Question 19 asks what the tests left out become.
5. `typesetting/LICENSE` is the license of go-text, the Unlicense or the
   BSD-3-Clause license, with the copyright line of okage.
   `typesetting/harfbuzz/LICENSE` keeps the MIT license of HarfBuzz, which the
   package `harfbuzz` ports.
6. The `go.mod` of okage does not require `github.com/go-text/typesetting`,
   and the test module does not either. The copy uses
   `golang.org/x/image/math/fixed` and `golang.org/x/image/tiff`, from the
   module `golang.org/x/image` that okage already requires (D6). A test of
   `shaping` uses `golang.org/x/image/font/gofont/goregular`. The list of
   depguard in `.golangci.yml` holds the three packages.
7. `svg` and a test of `webfont` import the copy. Their code does not change
   in other ways.
8. The policy of the copy is `copy`, a fifth policy beside the four of D7:
   okage keeps Go code of another project in its tree, close to its source,
   and applies the commits of the source from a ledger.
9. `docs/upstream/upstreams.tsv` lists go-text as the upstream `typesetting`,
   with the base commit `19482bd5`, and `docs/upstream/typesetting.tsv` is its
   ledger (D12). On 2026-11-17 it holds the 126 commits of go-text after
   `v0.3.5`, up to `bafa8730` of 2026-11-10. 110 are `pending`. Many of them
   fix a panic on a malformed font. 16 are `not-applicable`, because they
   change only `fontscan`, the CI or other files that okage does not copy, or
   because they are merge commits. The column `matched` stays empty until
   okage applies the pending commits in order.
10. The linters of `.golangci.yml` that ask for changes of style, or that
    find faults that the ledger holds as pending commits of go-text, do not
    run on `typesetting/`, as for `internal/brotli` (D45).
11. The copy builds with TinyGo. The tests of each package but `shaping` pass
    with TinyGo 0.44.0. A test of `shaping` calls a method of `reflect` that
    TinyGo does not have.
12. The packages are public, so that ivy imports them in place of go-text. A
    change to their API is a change to a public API of okage, and it follows
    the release rules of okage.

## How it changes the earlier decisions

1. D5: the folder `typesetting/` holds public packages beside the packages
   of the crates.
2. D6: the module needs `golang.org/x/image` only. The copy of go-text is not
   a dependency.
3. D7: `fontdb`, `harfrust`, `skrifa`, `read-fonts`, `unicode-bidi`,
   `unicode-script` and `unicode-vo` are replaced by `typesetting`, the copy
   of go-text.
4. D9: rule 1 reads `typesetting` for `github.com/go-text/typesetting`. okage
   still ports no crate of text. It copies Go code that does the same work.
5. D12: a ledger can hold the commits of an upstream whose policy is `copy`.
   A commit is applied as a diff of Go code, not ported from Rust or Python.
6. D13: the copy keeps the license of go-text in `typesetting/LICENSE`.

## What the copy does not change yet

The review of ivy measured the text of ivy against MuPDF, Poppler and Cairo,
which read the glyphs with FreeType, at 96 dpi. For the same font, the same size and the
same positions, the ink of each line of ivy is within 1.5% of the ink of the
references. The width of each line of a DOCX file that ivy lays out is the
width that LibreOffice gives, within one pixel. The faults that the review
found are in ivy: the fonts that ivy chooses and the height of its lines.
The coverage of okage moves in steps of a quarter of a pixel at an edge,
which FreeType does not. Since 2026-11-17 ivy draws small glyphs with a
finer coverage. So the copy has no change of behavior yet, and it adds no
hinting.
