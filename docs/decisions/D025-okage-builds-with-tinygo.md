# D25. okage builds with TinyGo

Status: Decided.

A build with [TinyGo](https://tinygo.org) is a distant goal of okage, and each
implementation keeps it in mind. TinyGo is a second compiler for Go, built on
LLVM. It makes small programs, and it is used most for WebAssembly and for
small devices. The build for WebAssembly matters most.

1. The scalar build must build with TinyGo. TinyGo does not support the SIMD
   packages of Go, and okage does not expect it to.
2. If a part of okage cannot build or cannot run with TinyGo, each fault is
   recorded in `docs/BACKLOG.md` and fixed one at a time.
3. If a fault cannot be fixed without a change to the port, okage can have a
   degraded build for TinyGo. Each degradation is a decision of its own, and
   `README.md` lists it.
4. TinyGo 0.44.0 is the lowest version that okage supports. CI builds for the
   host and for `wasip1`.

## What was measured

On 2026-11-09 the machine that measured had TinyGo 0.44.0, built on Go 1.27.3
and LLVM 22.1.4.

1. A probe with `image/png`, `iter.Seq`, `math` and methods on a named
   `[8]float32` type built for `wasip1`, `wasm` and the host, and it ran on
   the host.
2. A probe with `github.com/go-text/typesetting`, `golang.org/x/image/webp`
   and `golang.org/x/text/unicode/bidi` built for the host and for `wasip1`.
   Its module for `wasip1` was 1,435 KiB with TinyGo and 6,325 KiB with Go.

TinyGo sets the build tags `tinygo` and `purego`, and it never sets
`goexperiment.simd`. Its targets `wasip1`, `wasm` and `wasm-unknown` do not
turn on SIMD128. The flag `-llvm-features` of `tinygo build` can turn it on,
so that LLVM can make vector code from the scalar arrays. The comparison of
D22 measures that build. It is not a target of CI until it gives the same bytes
and is faster.

## The rules for the code

These rules come from the advice of Astra, Gemini Pro and DeepSeek Pro on
2026-11-09, and they hold from the first file:

1. The tags of `internal/wide` keep the SIMD implementation out of a build
   with TinyGo (D15).
2. The packages that a user imports do not use `reflect`, `encoding/json`,
   `encoding/xml`, `runtime/debug`, or `unsafe` beyond what the standard
   library does. The test module can use them.
3. The pipeline does not allocate for each pixel or for each span, and it
   uses no closure or interface on its hot path. The garbage collector of
   TinyGo is slower than the one of Go.
4. A large buffer is a slice that is allocated once, not an array on the
   stack. The stack of a program on WebAssembly is small.
5. A limit on the input, such as the size of an image, the depth of nesting in
   an SVG file, or the size of a decompressed SVGZ file, is explicit, as in
   upstream.

## How CI checks it

From phase 2 on, the nightly job of CI (D24):

1. Builds each package that a user imports with TinyGo, for the host and for
   `wasip1`.
2. Runs `tinygo test` for the host, and runs a test program for `wasip1` in
   wazero (D22). The program compares the decoded pixels with the scalar build
   of Go.
3. Records the size of each module for WebAssembly, so that a new import that
   makes it larger shows.
