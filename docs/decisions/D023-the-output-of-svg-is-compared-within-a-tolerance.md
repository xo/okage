# D23. The output of `svg` is compared within a tolerance

Status: Decided.

okage does not expect `svg` to give the same bytes as resvg or as libvips.
`svg` decodes images with the standard library and `golang.org/x/image/webp`
(D7), and it shapes text with go-text (D9). Both give other pixels than
upstream.

1. The packages of tiny-skia give the same bytes as upstream (D16).
2. The parsers under `internal/` and the tree of usvg are compared exactly
   with the Rust oracle where no image and no text is involved, because they
   do not touch pixels.
3. The images that `svg` renders are compared within a tolerance with two
   oracles: resvg, through the Rust oracle, and libvips, through the vips
   oracle (D22).
4. How to compare two images within a reasonable tolerance is not decided. It
   is chosen after the port of tiny-skia and before phase 8, and a decision
   records it. D31 proposes it.

The tests of resvg require zero pixels that differ. The tests with no text and
no raster image record how many still pass with zero pixels that differ.
