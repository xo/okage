# D41. `lottie` renders Lottie animations, as a port of velato

Status: Decided, amends D5 and D7, amended by D42, D43 and D44.

Lottie is a JSON format for vector animations. The package
`github.com/xo/lottie` renders it today. It runs ThorVG, compiled to
WebAssembly, on wazero. It has no cgo, but it holds a binary of about 900 KB
that okage cannot test, and each frame crosses into WebAssembly memory. The
maintainers want a renderer in Go that draws with okage.

The survey of other formats ([`../FORMATS.md`](../FORMATS.md)) names
[velato](https://github.com/linebender/velato) as the source for Lottie.
velato is the Lottie renderer of linebender, under Apache-2.0 or MIT. Its
`src` folder is about 265 KB of Rust, a third of the size of usvg and resvg.
It parses a file, evaluates each property at a frame, and sends fills,
strokes, layers and clips to a trait `RenderSink`.

## The scan of the Noto animated emoji

On 2026-11-12, the 879 Noto animated emoji were compared with the source
of velato at `83336b65`. They are Lottie files that Google publishes under
CC BY 4.0. The script `scripts/fetch-noto-emoji.sh` of `xo/lottie`
downloads them. velato was not run, so these results come from the files and
from the source.

velato supports each feature that most files use: groups, paths and fills
(879 files), track mattes (537), null layers (383), precompositions (251),
strokes (245), gradient fills (217), the split position of a layer (184),
time remap (75), time stretch (50), masks (49) and trim paths (44).

These features of the corpus are missing or wrong in velato:

| Files | Feature | State in velato |
| --- | --- | --- |
| 94 | Merge paths | Skipped. Mode 1 (71 files) draws the same image without it. Intersect (19 files), add (6) and subtract (3) need path boolean operations |
| 76 | Opacity stops of a gradient | The alpha is read only at the offsets of the color stops. The code calls this a hack |
| 29 | Expressions | Ignored. Almost all are references of the form `thisComp.layer(...)` |
| 1 to 5 each | Mask modes other than add, mask expansion, stroke dashes, the effects Matte3 and fill, some blend modes | Mask modes and expansion are ignored. The others need support in the sink |

No file uses text layers or image layers, which velato does not support.

## The decision

1. A new top-level package `lottie` ports the crate `velato`. Its base
   commit is `83336b6519ac7289beea53fcc2ec1e9603f25124` on main, from
   2026-09-16. It is 12 commits after the tag `v0.12.0`, and those commits
   add time remap, layer effects, and fixes of trim paths and mattes.
   After the base port, okage follows each commit of velato, with the
   checkout `upstream/velato` (D11) and the ledger
   `docs/upstream/velato.tsv` (D12).
2. A file of `lottie` starts with the name of its folder in `src`, as the
   root package does, such as `runtime_evaluate.go` and
   `schema_shapes.go`. A file that ports no file of velato, such as
   `sink.go`, has a name of its own.
3. The crates of velato get these policies. They extend the table of D7:

   | Crate | Policy | Go form |
   | --- | --- | --- |
   | `kurbo` | Subset. The subset of D7 grows by the items that velato calls: `BezPath`, `PathEl`, `Affine`, `Ellipse`, `RoundedRect`, `Arc`, the trait `Shape` for them, `Stroke`, `Cap`, `Join`, `solve_cubic` and `cubic_bez_coefs` | `internal/kurbo` |
   | `peniko` | Replace. velato uses only `Color`, `Brush`, `Gradient`, `BlendMode` and `Mix`. Small types in `lottie` hold them | none |
   | `serde`, `serde_json`, `serde_repr` | Replace with `encoding/json`. A value that is a constant or a list of keyframes gets an `UnmarshalJSON` method | none |
   | `vello` | None. okage does not port the sinks for Vello. `lottie` has a sink that draws with okage | none |

4. The trait `RenderSink` becomes the interface `Sink`. The package has a
   sink that draws onto an `okage.PixmapMut`. It draws fills, strokes and
   gradients, a layer with a blend mode and an opacity, and a clip with a
   mask of okage. Its `PushFilter` returns that the filter is not supported,
   as the sinks of velato for Vello do. A later decision can add the blur
   and the drop shadow of `svg`.
5. The API follows velato, with Go names: `Parse` returns a `*Composition`
   with its frames, its frame rate and its size, and a `Renderer` appends a
   frame to a `Sink`. One helper draws a frame onto a pixmap. A panic of
   velato, such as `todo!()` or `unimplemented!()`, becomes an error that
   wraps a sentinel error of the root package (D21), not a panic.
6. The package is pure Go, holds no package-level configuration, and builds
   with TinyGo (D25). It keeps the license of velato in `lottie/LICENSE`
   (D13).
7. The test module compares `lottie` with velato (D22). The Rust oracle links
   velato at the matched commit with a sink that records each call, and the
   test compares the calls of each frame within a tolerance. The test also
   renders the frames with okage. A few Noto animated emoji go in
   `lottie/testdata`, with a file `NOTICE` for CC BY 4.0. The test module
   downloads the full corpus into a folder that git ignores, and a test
   skips when the folder is missing.
8. The base port has the features of velato and nothing more. Merge paths
   with path boolean operations, expressions, and the fix of the opacity
   stops each need a decision of their own. D42 proposes merge paths by
   compositing. Each one is a deliberate
   difference from upstream, and its decision has a section "When upstream
   changes", as D40 has. The gaps go in [`../BACKLOG.md`](../BACKLOG.md).
9. `xo/lottie` stays as it is. This decision does not replace it. A program
   moves to `lottie` when `lottie` draws the files that the program needs.

## Why

A port gives a renderer that okage can test against its source, in the same
way as resvg, with no binary and no WebAssembly. okage already has the
paths, the strokes, the gradients, the blend modes, the masks and
`internal/kurbo` that the sink needs. So most of the work is the schema and
the runtime of velato.

The scan of the corpus shows that velato draws about 95% of the Noto
animated emoji without a change. The rest needs path boolean operations or
expressions, which no part of okage has. Those are large on their own, so
they wait for their own decisions, after the base port works.

velato is the source of the survey, and its sink fits okage better than a
conversion to the tree of `svg`, which has no idea of time.
