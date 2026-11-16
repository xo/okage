# D51. webfont is a public package

Status: Decided, amends D5 and D45.

The conversion of WOFF, WOFF2 and EOT fonts moves from `internal/webfont`
to `webfont`, a public package of the module `github.com/xo/okage`. The
maintainers decided it on 2026-11-16, for ivy, as D47 did for roxmltree.

ivy lays out ebooks and web pages with a port of WeasyPrint, and an
`@font-face` rule of a stylesheet loads fonts in these formats. ivy D21 asks
for the readers of okage, not a second copy. Go does not let another module
import an internal package, so the package must be public.

1. The code of the package does not change. Only its import path changes,
   to `github.com/xo/okage/webfont`, and the comments that named the old
   path.
2. The public API is the one that `svg` already used: the type `Format`
   with its constants, `DetectFormat` and `Convert`.
3. The test fonts move with the package, to `webfont/testdata`. The tests of
   `svg` read them from there.
4. Its errors still wrap `okage.ErrFontData` and `okage.ErrFontUnsupported`
   (D21).
5. `internal/brotli` stays internal. `webfont` can import it, because both
   are in the module `github.com/xo/okage`.
6. The package ports no upstream, so it has no ledger. It follows the
   recommendations of the W3C that D45 names.
7. It still builds with TinyGo (D25).

A change to the API of the package is now a change to a public API, so it
follows the release rules of okage.
