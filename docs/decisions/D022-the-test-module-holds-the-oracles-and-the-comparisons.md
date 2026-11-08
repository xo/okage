# D22. The test module holds the oracles and the comparisons

Status: Decided.

okage has a module in `test/` with its own `go.mod`, as dbmeta, transit,
tblfmt, odbc and dbfwd have. No program that imports okage imports it, so
nothing in it reaches a user. It can use cgo and cargo. It holds:

1. Each dependency that only the tests need. A package in it still needs the
   approval of the maintainers (D6). On 2026-11-09 two are approved:
   `github.com/tetratelabs/wazero`, to run the builds for WebAssembly, and
   `github.com/cshum/vipsgen/v2`, the vips oracle. A library for a benchmark
   against another Go library is approved when that benchmark is needed.
2. The Rust oracle.
3. The vips oracle.
4. The comparison of the builds for WebAssembly.
5. The benchmarks against other libraries, when they are needed.

## The Rust oracle

An oracle is a second program that gives the right answer for a test. The
Rust oracle is a small Rust program that links the crates of upstream from the
checkouts (D11). cargo builds it offline into its cache, with the scalar code
of tiny-skia, `--no-default-features`, because that code computes as okage
does (D16). It answers three kinds of question:

1. A scene of the renderer, as JSON: a path, a paint, a shader, a transform, a
   blend mode, a stroke, a dash, a clip mask and a size. It renders the scene
   and writes the pixels.
2. A question of `path`: the points of a stroke, a dash or a transformed
   path, and the bounds of a path, as the bits of each `float32`.
3. From phase 6 on, an SVG file: the tree that usvg makes, written as text,
   and the pixels that resvg renders.

For the renderer and for `path`, the Go tests make random scenes and random
paths, ask okage and the oracle, and compare every byte. A scene that fails is
saved in `testdata/` as JSON, so that it can be replayed.

## The vips oracle

libvips renders an SVG file with its own code, so it gives a second opinion on
how an image looks. `github.com/cshum/vipsgen/v2` is its Go binding, and it
uses cgo. The images of `svg` are compared with both oracles within a
tolerance (D23).

## The comparison of the builds for WebAssembly

A Go test runs each build in wazero, renders the same scenes, and records the
time and the size of each module:

1. okage built by Go for `wasip1`, in the scalar build and in the SIMD build.
2. okage built by TinyGo for `wasip1`, and with SIMD128 turned on through the
   flag `-llvm-features` (D25).
3. tiny-skia built by cargo for `wasm32-wasip1`, without SIMD and with
   SIMD128.

The okage builds must give the same bytes. The images of upstream with
SIMD128 can differ, because upstream approximates a reciprocal. From phase 8
on, the same comparison renders SVG files with `svg` and with resvg. The port
of resvg will run in browsers and in other hosts of WebAssembly, where the
size and the speed of each build matter.

A test skips when cargo, a checkout or libvips is missing, so that CI on each
push runs without them. A nightly job runs them (D24).

This follows transit D12 and D71.
