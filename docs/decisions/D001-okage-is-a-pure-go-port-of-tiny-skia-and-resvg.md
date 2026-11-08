# D1. okage is a pure Go port of tiny-skia and resvg

Status: Decided.

`github.com/xo/okage` is a pure Go port of two Rust projects:

1. [tiny-skia](https://github.com/linebender/tiny-skia), a small 2D renderer
   and a subset of Skia. It fills and strokes paths with a solid color, a
   gradient or a pattern, dashes strokes, clips with a mask and blends images.
   It renders on the CPU only.
2. [resvg](https://github.com/linebender/resvg), which renders SVG files with
   tiny-skia. Its crate `usvg` parses an SVG file into a simple tree, and its
   crate `resvg` renders the tree. The crate `c-api` is not ported.

The port of resvg is one package, `svg`, in this repository (D5).

## What pure Go means

A program that imports okage needs the Go toolchain and nothing else. The
module does not use cgo, it does not link a C or a Rust library, and it does
not run WebAssembly. It builds for every `GOOS` and `GOARCH` that Go supports,
with `CGO_ENABLED=0`. Only the test module can use cgo or cargo (D22).

## What a port means

okage is a translation of the upstream source, not a new design with the same
purpose. The Go code follows the Rust code closely enough that a person can
port an upstream change by reading its diff. The behavior is the same,
including the faults of upstream. A deliberate difference is a decision of its
own, and `README.md` lists it. [`docs/UPSTREAM.md`](../UPSTREAM.md) says how
close the code follows upstream.

okage ports no crate that decodes an image, and no crate for text, fonts or
the shaping of text (D7, D9). Other Go code does that work, and the output of
`svg` can differ from resvg because of it (D23).
