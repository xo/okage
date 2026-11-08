# D6. The goal is no dependencies

Status: Decided.

The goal of okage is no dependency outside the Go standard library. The
maintainers decide each exception, one case at a time, when it is necessary.

1. The packages of tiny-skia, which are the root package, `path` and
   `internal/wide`, import only the standard library.
2. Each outside package that okage needs is a dependency of the module
   `github.com/xo/okage`, in the `go.mod` of the root. `svg` is a package of
   that module, not a module of its own.
3. Do not add a Go package from outside this repository without the approval
   of the maintainers, and that includes a package that only a test imports.
4. A dependency that only the tests need goes in the test module (D22).
5. Each outside package must build with TinyGo for the host and for `wasip1`
   (D25).

On 2026-11-09 the module needs these outside packages:

| Package | For | Version on 2026-11-09 |
| --- | --- | --- |
| `github.com/go-text/typesetting` | fonts, the shaping of text and the outlines of glyphs (D9) | `v0.3.5` |
| `golang.org/x/image/webp` | the decoding of WebP files (D7) | the module `golang.org/x/image`, which go-text requires |

go-text also requires `github.com/go-text/typesetting-utils`. A probe that
imports go-text and `golang.org/x/image/webp` built on 2026-11-09 with Go and
with TinyGo 0.44.0, for the host and for `wasip1`.

A program that imports only the packages of tiny-skia does not link these
packages. Its `go.sum` still lists the modules that okage requires.

This follows dbmeta hard rule 1 and transit D15.
