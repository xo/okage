<div align="center">
  <a href="#status" title="Status">Status</a> |
  <a href="#installing" title="Installing">Installing</a> |
  <a href="#simd" title="SIMD">SIMD</a> |
  <a href="#documents" title="Documents">Documents</a> |
  <a href="#differences-from-upstream" title="Differences from upstream">Differences from upstream</a> |
  <a href="https://pkg.go.dev/github.com/xo/okage" title="Go Reference">Reference</a> |
  <a href="#license" title="License">License</a>
</div>

<br/>

[![Unit Tests][okage-ci-status]][okage-ci]
[![Go Reference][goref-okage-status]][goref-okage]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[okage-ci]: https://github.com/xo/okage/actions/workflows/test.yml "Test CI"
[okage-ci-status]: https://github.com/xo/okage/actions/workflows/test.yml/badge.svg "Test CI"
[goref-okage]: https://pkg.go.dev/github.com/xo/okage "Go Reference"
[goref-okage-status]: https://pkg.go.dev/badge/github.com/xo/okage.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/xo/okage?display_name=tag "Latest Release"
[releases]: https://github.com/xo/okage/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

# okage

`okage` is a pure Go port of [tiny-skia][ts] and of [resvg][resvg].

1. tiny-skia is a small 2D renderer in Rust, and it is a subset of
   [Skia][skia], the renderer of Chrome. It fills and strokes paths with a
   solid color, a gradient or a pattern. It dashes strokes, clips with a mask
   and blends images. It renders on the CPU only. okage ports it into the root
   package and the package `path`.
2. resvg renders SVG files with tiny-skia. okage ports it into the package
   `svg`.

Today [`xo/resvg`][xoresvg] wraps resvg through cgo and a static archive for
each platform. A program that imports okage needs the Go toolchain and nothing
else.

okage uses SIMD where Go can, and its scalar code is the fallback. SIMD
(single instruction, multiple data) means one instruction that works on
several values at once. A build with [TinyGo][tinygo], most of all for
WebAssembly, is a goal that the port keeps in mind.

## Status

The port of tiny-skia is done, in the root package and in `path`. It gives
the same bytes as tiny-skia in the scalar build, in the SIMD build and with
TinyGo, and the 157 integration tests of tiny-skia pass. It is still about 10
to 20 times slower than tiny-skia. The package `svg` is the port of usvg and
resvg. It parses and renders each test file of resvg: without text and
without a raster image, with the same pixels as resvg, and with text or an
image, within the tolerance of D31. [docs/PLAN.md](docs/PLAN.md) holds the plan, the phases and the
open questions.

The port starts from resvg `v0.49.0` and from the versions of the crates that
it uses, such as tiny-skia `v0.12.0`. After each base port, okage follows each
upstream commit. `docs/upstream/upstreams.tsv` lists the upstream
repositories, and one ledger for each records what okage did with each commit.

## Installing

When the first release exists, install okage with this command:

```sh
go get github.com/xo/okage
```

The renderer imports only the Go standard library. The package `svg` also
uses `github.com/go-text/typesetting` for text, and `golang.org/x/image/webp`
for WebP files. okage does not use cgo, so a program that imports it builds with
`CGO_ENABLED=0` for every platform that Go supports.

## SIMD

Go 1.27 has SIMD only as an experiment, so okage has two builds:

1. The scalar build is the default.
2. The SIMD build uses the package `simd/archsimd` of Go on amd64, arm64 and
   WebAssembly. A program gets it when it builds with `GOEXPERIMENT=simd`.

```sh
GOEXPERIMENT=simd go build ./...
```

Both builds give the same bytes for every image. On an amd64 CPU without
AVX2, the SIMD build uses the scalar code. TinyGo always uses the scalar code.
When Go makes SIMD a standard feature, the SIMD build becomes the default.
[docs/SIMD.md](docs/SIMD.md) holds the details.

## Documents

| Document | What it holds |
| --- | --- |
| [docs/PLAN.md](docs/PLAN.md) | the plan, the testing plan, the phases and the open questions |
| [docs/UPSTREAM.md](docs/UPSTREAM.md) | the rules for porting each upstream repository and each change that it makes |
| [docs/SIMD.md](docs/SIMD.md) | the rules for the vector code, the two builds and the SIMD packages of Go |
| [docs/BACKLOG.md](docs/BACKLOG.md) | the work that is known and not done |
| [docs/decisions/](docs/decisions/README.md) | every decision, one file each, with an index |
| [CONTRIBUTING.md](CONTRIBUTING.md) | how to change okage |
| [AGENTS.md](AGENTS.md) | the rules for a coding agent, which also hold for a person |

## Differences from upstream

okage differs from upstream only where a decision says so:

1. The SIMD build computes a reciprocal exactly. The SIMD backends of
   tiny-skia approximate it in the blend modes color dodge and color burn,
   and its scalar backend computes it exactly (D16).
2. `svg` decodes PNG, JPEG and GIF files with the standard library, and WebP
   files with `golang.org/x/image/webp`, in place of the decoders of resvg
   (D7).
3. `svg` finds fonts and shapes text with
   [`go-text/typesetting`][typesetting], in place of the crates of resvg. Its
   text can differ from the text of resvg (D9).
4. Each package logs to a `*slog.Logger` that the caller gives (D20).
5. An elliptical arc makes at most 4,096 cubic curves. kurbo has no limit, so
   an arc with a radius of about 4.23e20 or more does not end in upstream
   (D30).

okage also adds APIs that upstream does not have:

1. A `Pixmap` gives an `*image.RGBA` that shares its bytes, and it implements
   `image.Image` and `draw.Image` (D19).

The output of `svg` is compared with resvg and with libvips within a
tolerance, not byte for byte (D23). The renderer gives the same bytes as
tiny-skia.

## License

The root package and `path` are under the BSD-3-Clause license of tiny-skia.
See [LICENSE](LICENSE), which keeps the copyright lines of upstream. The
package `svg` and the packages that it uses take the MIT license of resvg and
of its crates, in a `LICENSE` file in each folder (D13).

<div align="center">
  <a href="https://github.com/xo/usql" title="A command line client for many databases">usql</a> |
  <a href="https://github.com/xo/dburl" title="Database connection URLs">dburl</a> |
  <a href="https://github.com/xo/dbmeta" title="Database metadata">dbmeta</a> |
  <a href="https://github.com/xo/dbimp" title="Database drivers in pure Go">dbimp</a> |
  <a href="https://github.com/xo/cassandra" title="A database/sql driver for Cassandra">cassandra</a> |
  <a href="https://github.com/xo/dbtpl" title="Go code generated from a database">dbtpl</a> |
  <a href="https://github.com/xo/tblfmt" title="Tables of database results">tblfmt</a> |
  <a href="https://github.com/xo/rline" title="The line editor of usql">rline</a> |
  <a href="https://github.com/xo/transit" title="tree-sitter in pure Go">transit</a> |
  <a href="https://github.com/xo/okage" title="tiny-skia in pure Go, this project">okage</a>
</div>

[ts]: https://github.com/linebender/tiny-skia
[skia]: https://skia.org
[resvg]: https://github.com/linebender/resvg
[xoresvg]: https://github.com/xo/resvg
[tinygo]: https://tinygo.org
[typesetting]: https://github.com/go-text/typesetting
