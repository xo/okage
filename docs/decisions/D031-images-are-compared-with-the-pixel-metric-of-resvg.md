# D31. Images are compared with the pixel metric of resvg

Status: Proposed.

D23 says that the images of `svg` are compared within a tolerance, and
question 16 asked how. This decision answers it. It builds on the metric that
the tests of resvg use, so that a result of okage reads the same way as a
result of upstream.

## The metric

1. Both images are demultiplied to RGBA, as `tests/integration/main.rs` of
   resvg does.
2. A pixel differs when one of its four channels differs by more than 1. A
   pixel that one image has and the other does not, because the sizes differ,
   also differs. This is `get_diff` of resvg, with its `DIFF_THRESHOLD` of 1.
3. The result of a test is the number of pixels that differ. resvg requires 0
   for each of its tests.

## The gates

1. A test file of resvg without text and without a raster image must give the
   same bytes as the Rust oracle, and 0 pixels that differ from the reference
   image of resvg. The packages of tiny-skia and the tree of usvg give the
   same bytes (D16, D23), so `svg` can too.
2. A test file with a raster image passes when at most 1 percent of its pixels
   differ from the reference image. okage decodes images with other code
   (D7).
3. A test file with text passes when at most 1 percent of its pixels differ
   from the reference image. okage lays out text on go-text (D9).
4. Each test that does not pass a gate of 2 or 3 is in a list in the test,
   with its number of pixels and the reason, so that a change shows.
5. libvips renders with librsvg, which has its own anti-aliasing. Its
   comparison counts the pixels where a channel differs by more than 32, and
   it reports the result of each file without a gate. A file where more than
   10 percent of the pixels differ is listed, because it can point to a fault
   of okage.

The numbers of gates 2, 3 and 5 are a first choice. They change when the
measurements of phases 8 and 9 show that a number is wrong, and the change is
recorded here.
