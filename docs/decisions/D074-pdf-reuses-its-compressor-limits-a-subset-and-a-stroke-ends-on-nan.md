# D74. pdf reuses its compressor, limits a subset, and a stroke ends on NaN

Status: Decided, amends D16.

"The PDF writer" in `docs/BACKLOG.md` lists three faults that make a program
run out of memory or wait forever:

1. `deflateEncode` makes a new zlib writer for each stream, as krilla makes
   a new encoder. Each writer allocates about 700 KB. A document of 8,192
   pages allocated 5.8 GB, and on wasip1 the program ran out of memory
   (item 7).
2. A subset of a font copies the data of a glyph once for each glyph ID that
   points to it, so a hostile font can make a subset up to 65,536 times the
   size of its table `glyf`. subsetter has the same property (item 1).
3. `path.Path.Stroke` does not end for some paths with a point that is NaN
   or infinite. tiny-skia has the same fault. `pdf` checks each point, but a
   program that strokes such a path with okage waits forever (item 3).

The maintainers want these faults fixed.

## The decision

1. `pdf` keeps its zlib writers in a `sync.Pool`, and resets a writer for
   each stream. A pool, not one writer for each document, is needed because
   `NewICCProfile` and the constructors of images compress data outside a
   document, and a program can call them from many goroutines at once. The
   bytes of each stream do not change. The tests of `pdf` and
   the oracle of krilla give the same files as before.
2. `internal/subsetter` stops a subset when its output is larger than 8 times
   the size of the font data plus 1 MiB, and returns a new sentinel error of
   the root package, which `Document.Finish` wraps (D21). No font of the
   tests comes near this limit.
3. `Path.Stroke` returns no path, and `false`, when a point of the path is
   NaN or infinite. It checks the points before it strokes the path. This is
   a deliberate difference from tiny-skia, and it amends D16: the bytes are
   the same as tiny-skia for each path whose points are finite, and
   tiny-skia gives no result for the other paths, because it does not end.
   `README.md` lists the difference.
4. Each change has a test. The test of item 3 strokes the path of item 3 of
   "The PDF writer" in `docs/BACKLOG.md`, and must end.

## When upstream changes

If krilla reuses its encoder, if subsetter limits its output, or if
tiny-skia makes a stroke end on such a path, follow these steps:

1. Port the commit of upstream as `docs/UPSTREAM.md` says.
2. If upstream gives the same result, remove the change of this decision.
3. If the result is not the same, a maintainer decides which one okage
   keeps. Record the result here or in a new decision.

## Why

A writer of documents and a renderer must not run out of memory or wait
forever on hostile or broken input. The first change gives the same bytes,
and the other two change only inputs where upstream fails.
