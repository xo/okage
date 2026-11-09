# okage Plan

This document holds the plan for `github.com/xo/okage`. The decisions are in
[`decisions/`](decisions/README.md), one file each (D4).

This plan was written on 2026-11-09, before any code existed (D3). A part of
this plan that names a decision follows it. The maintainers accepted the
decisions D1 to D25 on that date. The open questions are at the end of this
document.

## Status

On 2026-11-09 phases 0 to 9 have their code, and every gate holds but the
speed target of D18:

1. Phase 1: the tests of the repository and of the ledgers, the Rust oracle in
   the test module, the lint configuration and CI. The prototype of D17 was
   not built before the port, and D17 records why.
2. Phase 2: `path`, the port of tiny-skia-path and strict-num. Its tests pass,
   and the oracle gives the same bits for each path.
3. Phase 3: the root package, the port of tiny-skia. The 157 integration tests
   pass, and the oracle gives the same bytes on each random scene with no
   cubic curve. The math functions are correctly rounded (D26).
4. Phase 4: the SIMD paths of `internal/wide`, with the CPU check (D15, D28).
   The tests pass in both builds, on arm64 and on WebAssembly, and with
   TinyGo. okage is about 10 to 20 times slower than the scalar code of
   upstream (D18), and `BACKLOG.md` holds the work that closes the gap.
5. Phase 5: the three commits of tiny-skia after the base commit, up to
   `5d475477` on 2026-08-20. One commit is ported, and two are not
   applicable. The release `v0.1.0` waits for the target of D18 in phase 4.
6. Phase 6: `internal/roxmltree`, `internal/simplecss`, `internal/xmlwriter`,
   `internal/svgtypes` and the subset of kurbo in `internal/kurbo`, with
   every test of upstream. `internal/crmath` has the correctly rounded f64
   functions that kurbo needs (D26). The oracle gives the same tree for each
   of the 1,766 SVG files of the tests of resvg, and the same bits for each
   value that svgtypes parses in them. Random mutations of the files and of
   the values give the same results, but where glibc is not correctly
   rounded (D26).
7. Phase 7: `svg` has the port of usvg without the layout of text: the tree,
   the parser and the writer, with the tests of usvg that need no text. The
   oracle gives the same tree, written by the writer of usvg, for each of the
   1,766 SVG files of the tests of resvg. The files with text agree because
   usvg without its feature `text` drops the text, as `svg` does until phase
   9. Random mutations of the files give the same trees, but where a mutation
   breaks the data of a raster image, which okage reads with other code (D7).

8. Phase 8: `svg` has the port of the renderer of resvg, with its 1,742
   integration tests and the 15 tests of `extra.rs`, and D31 proposes how the
   images are compared. Each test file without text and without a raster
   image gives 0 pixels that differ from the reference image of resvg, and
   the same bytes as the Rust oracle. The tests with a JPEG image differ in
   up to 0.4 percent of their pixels, because `image/jpeg` decodes in another
   way (D7). libvips, the second oracle, runs with the tag `vips`.
9. Phase 9: the layout of text of usvg is ported on go-text (D9), with a
   font database that matches fonts as fontdb does. Of the 428 tests with
   text, 413 give 0 pixels that differ, and 15 differ in less than 1 percent
   of their pixels (D31): a stack of combining marks, and the variable font
   of the tests of `font-variation-settings`.

Phase 10, the commits of resvg after the base commit, comes next.

These documents hold the rules and the references that come from this plan:

- [`UPSTREAM.md`](UPSTREAM.md) says how each upstream repository is ported,
  and how each upstream change is ported after that.
- [`SIMD.md`](SIMD.md) says how the vector code is built, how it maps to the
  SIMD packages of Go, and how okage follows the changes of those packages.
- [`BACKLOG.md`](BACKLOG.md) holds the work that is known and not done.

## Purpose

okage is a pure Go port of [tiny-skia][ts] and of [resvg][resvg] (D1).

1. tiny-skia is a small 2D renderer in Rust. It is a subset of Skia, the
   renderer of Chrome. It fills and strokes paths with a solid color, a
   gradient or a pattern. It dashes strokes, clips with a mask and blends
   images. It renders on the CPU only.
2. resvg renders SVG files with tiny-skia. Its crate usvg parses an SVG file
   into a simple tree, and its crate resvg renders the tree.

Today [`xo/resvg`][xoresvg] wraps the C API of resvg through cgo. It links a
static archive for each platform, and each archive is about 37 MiB. A program
that imports okage needs the Go toolchain and nothing else.

okage uses SIMD where Go can, and its scalar code is the fallback (D14). SIMD
(single instruction, multiple data) means one instruction that works on
several values at once. Go 1.27 has SIMD only as an experiment. okage is built
so that its SIMD code becomes the default when Go makes SIMD a standard
feature, with a change to build tags only (D15).

A distant goal is a build with [TinyGo][tinygo], a second compiler for Go
that makes small programs, most of all for WebAssembly (D25). Each
implementation keeps it in mind.

[ts]: https://github.com/linebender/tiny-skia
[resvg]: https://github.com/linebender/resvg
[xoresvg]: https://github.com/xo/resvg
[tinygo]: https://tinygo.org

## What upstream holds

These facts were measured on 2026-11-09, in the checkouts in `upstream/`
(D11). A crate is a Rust package. The line counts leave out comments and blank
lines.

### tiny-skia

[`linebender/tiny-skia`][ts] holds two crates:

| Crate | Path | Lines | What it holds |
| --- | --- | --- | --- |
| `tiny-skia-path` | `path/src` | about 5,200 | paths, the path builder, the stroker, dashes, rectangles, sizes, transforms, and the small vectors `f32x2` and `f32x4` |
| `tiny-skia` | `src` | about 11,600 | colors, the pixmap, the mask, edges, the scan converters, the shaders, the raster pipeline, the vector types and the painter |

The stroker is the largest file, with 1,674 lines. `tiny-skia-path` uses the
small crate `strict-num` for its checked float types.

A pixmap is a buffer of pixels. Each pixel is four bytes, red, green, blue and
alpha, and the color is premultiplied by the alpha. A mask is a buffer of one
byte for each pixel.

The raster pipeline is the part that colors the pixels. It is a list of
stages. A stage is one function, such as "load the destination" or "blend with
the source-over mode". Upstream has 78 stages, in two forms:

1. The low precision form, `lowp`, works on 16 pixels at a time. Each color
   channel is a `u16x16`, sixteen 16-bit integers.
2. The high precision form, `highp`, works on 8 pixels at a time. Each color
   channel is an `f32x8`, eight 32-bit floats.

The painter uses `lowp` when the stages of a paint allow it, and `highp` when
they do not. Each stage is a `fn(&mut Pipeline)`. The `Pipeline` struct holds
the color channels and a program counter, and each stage calls the next.

The vector types are in `src/wide`, about 2,000 lines. Upstream chooses one
implementation of each type when it compiles. The choices are SSE2, SSE4.1,
AVX or AVX2 on x86, NEON on arm64, SIMD128 and relaxed SIMD on WebAssembly,
and plain scalar code everywhere else. Upstream does not detect the CPU at
run time. `u16x16` is a plain array of 16 values on x86, because the Rust
compiler makes better vector code from it than the explicit AVX2 code did. It
uses NEON on arm64.

The tests are:

1. 157 integration tests in `tests/integration`, which render a scene and
   compare it with one of 121 reference PNG files in `tests/images`. The
   comparison is exact: every byte of every pixel must match.
2. 30 unit tests in the source files.
3. The tests whose names start with `skia_` port tests of Skia.

Upstream CI runs the same integration tests with the same reference images
for the scalar code, SSE2, SSE4.1, AVX, AVX2, WebAssembly SIMD128, relaxed
SIMD, NEON, and big-endian PowerPC. So each reference image is the same for
every backend of upstream.

tiny-skia changes slowly. The tag `v0.12.0` is commit `e48b9027`, from
2026-02-02. Three commits came after it, up to `5d475477` on 2026-08-20.
About 10 commits came in the 12 months to 2026-11-09.

### resvg

[`linebender/resvg`][resvg] holds three crates:

| Crate | Lines | What it holds |
| --- | --- | --- |
| `usvg` | about 14,700 | the parser: the XML tree, the styles, the units, the paint servers, the filters, the clip paths, the masks, the markers, the images, the layout of text (about 2,300 lines), the tree, and a writer of SVG |
| `resvg` | about 3,700 | the renderer of the tree: the paths with their paints, the clips, the masks, the filters (about 1,500 lines) and the images |
| `c-api` | | the C API that `xo/resvg` wraps |

usvg imports `tiny-skia-path`, and resvg imports `tiny-skia`. D7 lists the
other crates that they use, with the versions that the file `Cargo.lock` of
`v0.49.0` names.

The tests of resvg are 1,742 integration tests. Each one renders an SVG file
in `crates/resvg/tests/tests` and compares it with a PNG file, and each one
requires zero pixels that differ. 386 of them render text, with the fonts in
`crates/resvg/tests/fonts`. 10 read a JPEG file, 4 a PNG file, 1 a GIF file
and 1 a WebP file, and 31 hold an image in a `data:` URL.

resvg changes fast. The tag `v0.49.0` is commit `66ee0dc7`, from 2026-10-24.
26 commits came after it, up to `847abd69` on 2026-11-05. About 96 commits came
in the 12 months to 2026-11-09.

## Scope

okage ports these parts:

1. The crates `tiny-skia-path`, `strict-num` and `tiny-skia`, with their
   tests, their integration tests and their reference images.
2. The crates `usvg` and `resvg`, with their tests, their integration tests,
   their reference images and their test fonts. This includes the layout of
   text of usvg (D9).
3. The crates that usvg and resvg use, as D7 says for each one.
4. The benchmarks of tiny-skia in `benches/src`, so that the speed of okage is
   measured against the speed of upstream.
5. The examples of tiny-skia in `examples/`, as Go examples.

okage does not port these parts:

1. `testing-tools/skia-rs` of tiny-skia, the bindings to Skia that upstream
   uses to make some reference images.
2. The `no_std` build and the feature `no-std-float`. Go has no such build.
3. The crate `c-api` of resvg, and the command line tools of usvg and resvg.
4. Any crate that decodes an image, and any crate for text, fonts or the
   shaping of text (D7, D9). The standard library, `golang.org/x/image/webp`
   and `github.com/go-text/typesetting` do that work (D6).

## Architecture

### The packages

D5 sets the packages:

| Path | Package | What it holds |
| --- | --- | --- |
| `github.com/xo/okage` | `okage` | the port of the crate `tiny-skia`, which is the API that a user calls to render, and the sentinel errors of every package (D21) |
| `path` | `path` | the port of the crates `tiny-skia-path` and `strict-num` |
| `svg` | `svg` | the port of the crates `usvg` and `resvg`, as one package |
| `internal/wide` | `wide` | the vector types of `src/wide` (D15) |
| `internal/svgtypes`, `internal/roxmltree`, `internal/simplecss`, `internal/kurbo`, `internal/xmlwriter` | the name of the crate | the crates that `svg` uses (D7, D8) |
| `test` | | the test module, with its own `go.mod`: the oracles, the comparison of the builds for WebAssembly, and the benchmarks against other libraries (D22) |

The packages of tiny-skia import only the standard library. Each outside
package that `svg` needs is a dependency of the module `github.com/xo/okage`
(D6).

### The port is idiomatic Go

The Go is idiomatic Go, and the `go-pedantry` skill applies to it. Names are
MixedCaps. A Rust `Option<T>` that the API returns becomes `(T, bool)`. A
Rust `Result<T, E>` becomes `(T, error)`, with the sentinel errors of the root
package (D21). A Rust enum becomes a typed constant with a `String` method. A
Rust trait becomes a Go interface only where Go needs one. An iterator that
the API returns is an `iter.Seq`. A Rust struct of options becomes functional
options (D20).

Upstream uses types that check a value when they make it, such as
`NonZeroPositiveF32`, `NormalizedF32` and `FiniteF32`. okage keeps each one as
a defined type with a constructor that checks the value, because upstream
reads the type to know that the check happened.

### The port still follows upstream file by file

Idiomatic Go does not break the link to upstream. The rules are in
[`UPSTREAM.md`](UPSTREAM.md). In short:

1. One Rust file becomes one Go file. Where the files of several folders or
   crates go into one package, the name of the Go file starts with the name
   of the crate or the folder, such as `scan_path_aa.go` and
   `usvg_parser_svgtree.go`.
2. The functions of the Go file are in the same order as in the Rust file.
3. The doc comment of each ported function names the Rust function and its
   path.
4. The comments of the Rust code are ported with the code.
5. The behavior is the same, including the faults of upstream.
6. A private name that two modules define goes apart as D10 says. A stage of
   `lowp` or `highp` is a method of `lowpPipeline` or `highpPipeline`. Any
   other name that collides takes the name of its module as a suffix.

### The vector types

okage keeps the fixed widths of upstream (D15). The package `internal/wide`
declares `F32x4`, `F32x8`, `F32x16`, `I32x4`, `I32x8`, `U32x4`, `U32x8` and
`U16x16`, each with the methods of the Rust type. The pipeline code calls
these methods, and it does not change with the build.

Each type has two implementations:

1. The scalar implementation is an array, such as `[8]float32`, with a loop
   or one statement for each lane. A lane is one value of a vector. It is in
   every build.
2. The SIMD implementation uses the package `simd/archsimd` of Go 1.27. It
   exists only with `GOEXPERIMENT=simd`, on amd64, arm64 and WebAssembly. On
   amd64 an `F32x8` is one `archsimd.Float32x8`. On arm64 and WebAssembly,
   where the widest vector is 128 bits, an `F32x8` is two
   `archsimd.Float32x4`.

On amd64, every operation of `archsimd` needs AVX, and the 256-bit integer
operations need AVX2. So the SIMD build reads the CPU once when the program
starts, and it runs the scalar implementation on a CPU without AVX2.

[`SIMD.md`](SIMD.md) maps each method to its Go form.

### The same bytes in every build

The renderer gives the same bytes as the scalar build of tiny-skia, and the
SIMD build gives the same bytes as the scalar build (D16). This is stricter
than upstream, whose backends can differ in a pixel that no test reads. Four
rules make it possible:

1. okage computes a reciprocal and a reciprocal square root exactly. Upstream
   uses the approximate instructions `rcp_ps` on x86 and `vrecpeq` with one
   step of refinement on NEON, but its scalar code and its WebAssembly code
   compute `1.0 / x`. Upstream uses the approximation only in the blend modes
   color dodge and color burn.
2. okage never fuses a multiply and an add. Rust never fuses `f * m + a`, and
   the helper `mad` of upstream is an unfused multiply and add. The Go
   compiler can fuse `x*y + z` into one instruction on architectures such as
   arm64, ppc64le, s390x and riscv64, and that changes the rounding. The
   compiler can fuse across statements too, so a temporary variable does not
   stop it. An explicit conversion does, so okage writes `float32(x*y) + z`
   where upstream multiplies and adds. okage never calls `MulAdd`.
3. A conversion from a float to an integer follows the Rust code that it
   ports. Go truncates toward zero, as `as i32` does in Rust, but Go gives an
   implementation-specific value for a float that does not fit, and Rust
   saturates. On amd64, `archsimd` gives `0x80000000` for such a float.
   Upstream's round-to-nearest conversion, `cvtps_epi32`, rounds half to
   even. The package `internal/wide` holds one helper for each form.
4. `min` and `max` follow upstream when a value is NaN. Upstream has its own
   `fast_min` and `fast_max`, which do not treat NaN as the standard library
   does, and okage ports them.

tiny-skia also calls `cos`, `sin`, `acos` and `powf`. Phase 3 measures
whether the functions of `math` give the same bits as the functions that Rust
calls on Linux.

### The pipeline

Upstream runs a stage through a table of function pointers, and each stage
calls the next. Go has no tail calls, and a call through a function value
stops inlining. On WebAssembly such a call is a `call_indirect`.

Phase 1 builds a prototype with the real pipelines of three cases (D17). The
prototype measures three choices:

1. A table of functions, or one loop with a `switch`.
2. The way the pipeline runs the SIMD or the scalar implementation.
3. The color channels in a struct, or in local variables.

The fastest form becomes the form of the port.

### The public API of the renderer

The API follows the Rust API, in Go form. `Pixmap`, `Paint`, `Shader`,
`Path`, `PathBuilder`, `Stroke`, `Transform`, `Mask` and the rest keep their
names. A Rust method `fill_path` becomes `FillPath`.

The bytes of a `Pixmap` have the layout of an `image.RGBA` of the standard
library: four bytes for each pixel, in the order red, green, blue and alpha,
premultiplied by the alpha. A `Pixmap` gives an `*image.RGBA` that shares its
bytes, and it implements `image.Image` and `draw.Image` (D19).

Each part that logs takes a `*slog.Logger` through a functional option, and
it logs nothing without one (D20).

### The package `svg`

`svg` holds usvg and resvg in one package (D5). A user parses an SVG file
into a tree and renders the tree into a `Pixmap`, with one import, as the C
API of resvg does. The tree types of usvg are exported, because a user reads
them, as upstream does.

The crates that usvg uses are internal packages, and each has a policy:
port, subset, replace or none (D7). The XML parser is a port of roxmltree, and
the CSS parser is a port of simplecss, because no Go package gives what usvg
reads from them (D8).

`svg` decodes PNG, JPEG and GIF files with the standard library, and WebP
files with `golang.org/x/image/webp` (D7). It finds fonts and shapes text with
`github.com/go-text/typesetting`, and the layout of text of usvg is ported on
top of it (D9). Fonts come from the caller, as bytes or as an `fs.FS`, so
that a program on WebAssembly or with TinyGo works the same way as any other.
The text and the images of `svg` can differ from resvg, which is an accepted
difference, and its output is compared within a tolerance (D23).

## Testing plan

### 1. The upstream tests, ported

okage ports every unit test and every integration test of upstream. The
integration tests read the reference images of upstream, copied into
`testdata/` with the licenses of upstream (D13). Each test of the renderer
must match its image exactly, in the scalar build and in the SIMD build. The
tests of `svg` use the tolerance of D23.

### 2. The oracles

An oracle is a second program that gives the right answer for a test. The
test module holds two (D22):

1. A small Rust program that links the crates of upstream from the
   checkouts. It renders a scene of the renderer, answers a question of
   `path`, and from phase 6 on, parses and renders an SVG file. It uses the
   scalar code of tiny-skia, because that code computes as okage does.
2. libvips, through `github.com/cshum/vipsgen/v2`, which renders an SVG file
   with its own code.

For the renderer and for `path`, the tests make random scenes and random
paths, ask okage and the Rust oracle, and compare every byte. A scene that
fails is saved in `testdata/` as JSON. The images of `svg` are compared with
both oracles within a tolerance, which is chosen after the port of tiny-skia
(D23). cargo builds the Rust oracle offline, and a test skips when cargo, a
checkout or libvips is missing.

### 3. Fuzzing

A Go fuzz test makes scenes and SVG files from random bytes. It makes sure
that okage does not panic, does not hang and does not read outside a buffer.
The oracle can replay each input that the fuzzer finds.

### 4. The builds

Each test runs in these builds:

1. The scalar build, on amd64 and on arm64. The run on arm64 catches a
   multiply and an add that the compiler fuses.
2. The SIMD build, with `GOEXPERIMENT=simd`, on amd64 and on arm64.
3. The SIMD build on amd64 with a CPU check that reports no AVX2, so that the
   fallback of D15 runs.
4. The scalar build on a big-endian platform, s390x under QEMU, because
   upstream tests on big-endian PowerPC.
5. Both builds for `GOOS=wasip1` with `GOARCH=wasm`.
6. The scalar build with TinyGo, for the host and for `wasip1`, from phase 2
   on (D25).

### 5. The builds for WebAssembly, compared

The test module compares the builds of the same renderer for WebAssembly, with
the same scenes, in wazero (D22):

1. okage built by Go for `wasip1`, in the scalar build and in the SIMD build.
2. okage built by TinyGo for `wasip1`, and with SIMD128 turned on through the
   flag `-llvm-features`.
3. upstream built by cargo for `wasm32-wasip1`, without SIMD and with
   SIMD128.

The test records the time of each scene and the size of each module. The
okage builds must give the same bytes. From phase 8 on, the same comparison
renders SVG files.

### Speed

okage ports the benchmarks of tiny-skia. The SIMD build takes at most twice
the time of upstream with AVX2 on the benchmarks of fills, and the speed of
the scalar build is recorded (D18). D17 records the measurements of the
prototype.

When a comparison with another Go library is needed, its benchmark goes in
the test module, and the library needs the approval of the maintainers (D22).

### CI

CI runs on GitHub Actions (D24). Each push runs the scalar build and the SIMD
build on linux amd64, without the oracles. A nightly job runs arm64, s390x
under QEMU, `wasip1` and TinyGo, the tests of the oracles, the comparison of
the builds for WebAssembly and the benchmarks.

## TinyGo

A build with TinyGo is a distant goal, and each implementation keeps it in
mind (D25). TinyGo 0.44.0 is the lowest version that okage supports. On
2026-11-09 a probe with `image/png`, `iter.Seq`, `math` and methods on a named
`[8]float32` type built with it for the host, for `wasip1` and for `wasm`. A
probe with go-text and `golang.org/x/image/webp` built for the host and for
`wasip1`.

D25 holds the rules, which hold from the first file. In short:

1. TinyGo never builds the SIMD implementation (D15).
2. The packages that a user imports do not use `reflect`, `encoding/json`,
   `encoding/xml` or `runtime/debug`.
3. The pipeline does not allocate on its hot path, and it uses no closure or
   interface there.
4. A large buffer is a slice that is allocated once, not an array on the
   stack.
5. Each limit on the input is explicit.
6. A fault with TinyGo goes in `BACKLOG.md` and is fixed, or it gets a
   decision for a degraded build.

## Phases

Each phase ends when its gate holds. Agents split each phase into units, one
upstream file with its tests each. An agent claims a unit in `BACKLOG.md`.
Phases 0 to 5 port tiny-skia, and phases 6 to 10 port resvg.

### Phase 0. The plan and the rules

This phase writes this plan, `UPSTREAM.md`, `SIMD.md`, the decisions and the
setup of the repository. It ends when the consolidated decisions are
committed (D3).

### Phase 1. The harness and the prototype

1. The tests of the agent setup and of the documents, as the other `xo`
   repositories have them.
2. The ledger test (D12), and the checkouts and ledgers of the other
   upstreams.
3. The test module and the Rust oracle, with one scene that renders.
4. The prototype of the pipeline of D17.
5. CI (D24).

Gate: the oracle renders a scene, the ledger test passes, and D17 records the
measurements and the forms that the port uses.

### Phase 2. The package `path`

Port `tiny-skia-path` and `strict-num`, one file at a time, with their tests.

Gate: every ported test passes, and the oracle finds no difference in the
results of 100,000 random paths, strokes and dashes, on amd64 and on arm64.
`path` builds with TinyGo for the host and for `wasip1`, and each test that
TinyGo cannot run is in `BACKLOG.md` with the reason.

### Phase 3. The renderer, scalar

Port `tiny-skia` with the scalar implementation of `internal/wide`: the
colors, the pixmap and the mask, the edges and the scan converters, the
pipeline, the shaders and the painter. Measure the math functions of Go
against Rust (D16).

Gate: the 157 integration tests and the unit tests pass, and the oracle finds
no difference in 100,000 random scenes, on amd64, on arm64 and on s390x. The
root package builds with TinyGo for the host and for `wasip1`, or each fault
has a decision for a degraded build.

### Phase 4. The SIMD implementation

Write the SIMD implementation of `internal/wide` for amd64, arm64 and
WebAssembly, with the CPU check of D15.

Gate: every test of phase 3 passes in the SIMD build, and with the fallback of
D15. The SIMD build gives the same bytes as the scalar build, and it meets the
target of D18. The comparison of the builds for WebAssembly runs, and its
numbers are recorded.

### Phase 5. Follow tiny-skia

Port the commits of tiny-skia after its base commit, in order, as
[`UPSTREAM.md`](UPSTREAM.md) says. The first release, `v0.1.0`, ends this
phase.

### Phase 6. The parsers that `svg` uses

Port `svgtypes`, `roxmltree`, `simplecss`, `xmlwriter` and the subset of
`kurbo`, each into its package under `internal/`, with its tests.

Gate: every ported test passes. The oracle finds no difference in the values
that `svgtypes` parses, and in the trees that `roxmltree` builds, for the 1,742
SVG files of the tests of resvg.

### Phase 7. The tree of usvg, without text

Port usvg into `svg`, without the layout of text.

Gate: for each of the 1,742 SVG files that has no text and no raster image,
the tree of `svg`, written as text, is the same as the tree of the Rust oracle
(D23).

### Phase 8. The renderer of resvg, without text

Before this phase, a decision chooses how two images are compared within a
tolerance (D23, D31).

Port resvg into `svg`: the paths with their paints, the clips, the masks, the
filters and the images.

Gate: each integration test of resvg that has no text passes within the
tolerance against the Rust oracle and the vips oracle, in the scalar build
and in the SIMD build. The tests with no text and no raster image record how
many pass with zero pixels that differ.

### Phase 9. Text

Port the layout of text of usvg on go-text (D9).

Gate: the 386 tests of text of resvg pass within the tolerance of D23, and
each test that does not pass is in `BACKLOG.md` with the reason.

### Phase 10. Follow resvg

Port the commits of resvg and of the crates of D7 after their base commits,
in order, as `UPSTREAM.md` says. A crate comes before the commit of resvg that
raises its version (D12). After that, okage follows its upstreams when a
maintainer asks.

## Following upstream

okage follows several upstream repositories, each with its own ledger (D12).
`docs/upstream/upstreams.tsv` lists them, with the base commit and the last
matched commit of each one. The base commits are the versions that the file
`Cargo.lock` of resvg `v0.49.0` names. `UPSTREAM.md` holds the steps.

tiny-skia made about 10 commits in a year, and resvg about 96. Most commits of
resvg change usvg or resvg, so the ledger of resvg is the largest.

okage does not follow Skia or HarfBuzz. An upstream ports from them when it
chooses to, and okage ports what the upstream ports.

okage ports no crate for images or text (D7, D9). A commit of resvg that only
raises the version of such a crate changes nothing that okage ports. A commit
that changes the code of usvg that calls such a crate is ported, with go-text
in place of the crate. The outside Go packages that okage uses are updated in
`go.mod` when a maintainer asks.

okage also follows the SIMD packages of Go. They are an experiment, and their
API can change in each release of Go. [`SIMD.md`](SIMD.md) holds the steps
for a new release of Go.

## Risks

This list puts the most serious risk first:

1. The SIMD packages of Go change, or Go never makes them standard. Only the
   files of `internal/wide` with the SIMD tag use them, so a change touches
   only those files. The scalar build does not change.
2. A pixel of the renderer differs from upstream, because of a fused multiply
   and add, a conversion from a float, or the order of operations. The oracle
   and the run on arm64 find such a pixel. The rules of "The same bytes in
   every build" prevent most of them.
3. `math.Cos`, `math.Sin`, `math.Acos`, `math.Pow` and the other functions of
   Go do not give the same bits as the functions of the C library that Rust
   calls on Linux. tiny-skia calls `cos`, `sin`, `acos` and `powf`, so this
   can break the same bytes of D16. Phase 3 measures it. If a pixel differs,
   a decision ports the functions that Rust calls.
4. The scalar build is slow. Rust makes vector code from plain arrays, and
   the Go compiler does not. The scalar build can be several times slower
   than upstream, and only the SIMD build closes the gap. The prototype of
   phase 1 measures it.
5. Bounds checks, values that escape to the heap, and vectors in a struct
   slow the pipeline. The prototype measures them, and
   `go build -gcflags=-d=ssa/check_bce` and `-gcflags=-m` find them.
6. The text of go-text differs from the text of resvg more than the tolerance
   of D23 allows, in some of the 386 tests of text. That difference is
   accepted (D9), and phase 9 records each test that does not pass. A call of
   a crate of text can have no match in go-text, and the maintainers decide
   what okage does then.
7. resvg changes fast, about 96 commits a year, and the port of usvg and resvg
   takes months. The ledger of resvg grows while phases 6 to 9 run. The
   ledger sorts each commit as it arrives, so phase 10 starts from a sorted
   list.
8. The decoders of the standard library give other pixels than the decoders
   of upstream. That difference is accepted (D7), and the tolerance of D23
   covers the tests with a raster image: 10 with JPEG, 4 with PNG, 1 with
   GIF, 1 with WebP and 31 with a `data:` URL.
9. TinyGo cannot build a part of the standard library or of an outside package
   that okage uses, or it builds it slowly. The CI job of TinyGo finds such a
   fault from phase 2 on, while the code is small (D25).

## Prior art

These projects exist on 2026-11-09. okage is not based on any of them.

1. [`lumifloat/tinyskia`][lumi], a Go port of tiny-skia under the BSD-3-Clause
   license, created on 2026-03-22. It gives an API in the form of the HTML
   canvas, and it keeps the port in internal packages. It does not keep the
   API or the files of upstream, it has no SIMD code beyond `f32x2`, and it
   imports `github.com/chewxy/math32`, `github.com/go-text/typesetting` and
   `golang.org/x/image`.
2. [`golang.org/x/image/vector`][xvector], a rasterizer for filled paths with
   anti-aliasing. It does not stroke, dash, shade or blend.
3. [`srwiley/rasterx`][rasterx] and [`tdewolff/canvas`][canvas], renderers of
   vector graphics in Go with designs of their own. `srwiley/oksvg` renders a
   subset of SVG on rasterx.
4. [`xo/resvg`][xoresvg], the wrapper of the C API of resvg through cgo,
   which okage will replace.

okage can read these projects, and it copies no code from them. It differs
from the first in three ways. It follows upstream file by file, so that each
upstream commit can be ported. It keeps the API of upstream, which the port of
resvg needs. It has SIMD code.

[lumi]: https://github.com/lumifloat/tinyskia
[xvector]: https://pkg.go.dev/golang.org/x/image/vector
[rasterx]: https://github.com/srwiley/rasterx
[canvas]: https://github.com/tdewolff/canvas

## The reviews

Astra, Gemini Pro and DeepSeek Pro reviewed this plan on 2026-11-09. A long
answer took longer than the time that the host allows a tool, so each model
answered short questions on four topics: the rules for TinyGo, how to follow
many upstreams, the layout of `svg` and its text, and the core rules of the
vector types, the same bytes and the pipeline. DeepSeek Pro answered the last
one only in short.

### What the plan took from the reviews

1. One ledger for each upstream repository, a list of the upstreams with the
   base commit of each, base commits from the `Cargo.lock` of resvg, and a
   dependency ported before the commit that raises its version. All three
   (D12).
2. One public package for usvg and resvg, with the crates that they use in
   internal packages. All three (D5).
3. A policy for each dependency: port, subset or replace (D7).
4. A suffix or a prefix for a name that collides. All three (D10).
5. No port of harfrust and skrifa, and fonts from the caller. All three (D9).
6. A CPU check in the SIMD build, because every operation of `archsimd` on
   amd64 needs AVX. Astra raised it, and the documentation of `archsimd`
   confirmed it (D15).
7. A prototype of the pipeline with real pipelines, not 10 stages, measured
   with TinyGo too. All three (D17).
8. CI that runs the tests with TinyGo, not only builds them, and records the
   size of each module for WebAssembly. Astra and Gemini Pro (D25).
9. No `encoding/xml` in place of roxmltree. Astra (D8).
10. A build with TinyGo and SIMD128, through `-llvm-features`, in the
    comparison of builds. Gemini Pro and Astra (D22). DeepSeek Pro said to
    wait.
11. The risk that the math functions of Go differ from the C library. Gemini
    Pro (risk 3).

### What the reviews got wrong

Each of these was measured on 2026-11-09:

1. Gemini Pro said that resvg shapes text with rustybuzz. The file
   `Cargo.toml` of usvg `v0.49.0` names harfrust 0.12.0.
2. Gemini Pro said that Go fuses `float32(x*y) + z`, and that a temporary
   variable stops the fusion. The Go specification says the opposite: an
   explicit conversion stops a fusion, and a fusion can cross statements.
3. Gemini Pro said to drop the list of architectures from the SIMD tag. On
   riscv64, a program that sets `GOEXPERIMENT=simd` then does not build,
   because `archsimd` has no types there.
4. Gemini Pro said that the brief put methods on an unnamed array type. The
   probe used a named type.
5. DeepSeek Pro said that TinyGo computes `float32` in software. On amd64 and
   on WebAssembly, LLVM uses the float instructions of the machine.
6. Astra said that the date 2026-11-09 was in the future. It is the date of
   the machine that measured.

### What the plan did not take

1. Gemini Pro said to let the SIMD build match the SIMD backends of upstream,
   with the approximate reciprocal. That is not taken, because the
   approximation differs between x86 and NEON, and D16 asks for one result.
2. DeepSeek Pro said to put usvg, resvg, `lowp` and `highp` in internal
   packages of their own. That is not taken, because the port of resvg is one
   package and the pipeline needs the private types of the root package (D5).
3. Gemini Pro said to use a `switch` for the pipeline from the start. The
   prototype of D17 decides it.
4. Astra and DeepSeek Pro said to port zune-jpeg if the bytes of `svg` must
   match. The maintainers chose to port no image decoder (D7), and to compare
   the output of `svg` within a tolerance (D23).

## Open questions

An open question stays here until the maintainers answer it. Then it becomes
a decision in [`decisions/`](decisions/README.md), and it is deleted from this
list. The numbers do not repeat. The next question is question 18.

The maintainers answered questions 1 to 15 and 17 on 2026-11-09, and the
decisions record the answers. Question 16, how two images are compared
within a tolerance, has the proposed answer D31.

No question is open.
