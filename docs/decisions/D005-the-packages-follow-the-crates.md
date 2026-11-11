# D5. The packages follow the crates

Status: Decided, amended by D38.

| Path | Package | What it holds |
| --- | --- | --- |
| `github.com/xo/okage` | `okage` | the port of the crate `tiny-skia`, with the edges, the scan converters, the shaders and the raster pipeline, and the sentinel errors of every package (D21) |
| `path` | `path` | the port of the crates `tiny-skia-path` and `strict-num` |
| `svg` | `svg` | the port of the crates `usvg` and `resvg`, as one package |
| `html` | `html` | the parser and the layout of the HTML-like labels of Graphviz, which port no upstream (D38) |
| `internal/wide` | `wide` | the vector types of `src/wide` of tiny-skia (D15) |
| `internal/svgtypes` | `svgtypes` | the parsers of SVG values: lengths, colors, path data, transforms, filter functions and the font shorthand |
| `internal/roxmltree` | `roxmltree` | the XML tree, with entities and namespaces (D8) |
| `internal/simplecss` | `simplecss` | the CSS parser and selectors (D8) |
| `internal/kurbo` | `kurbo` | the subset of kurbo that usvg uses (D7) |
| `internal/xmlwriter` | `xmlwriter` | the XML writer that the writer of usvg uses |
| `test` | | the test module, with its own `go.mod` (D22) |

## Why `path` is a package of its own

usvg imports the crate `tiny-skia-path` and not the crate `tiny-skia`, so
`svg` can import `path` for the code that parses. The root package declares a
type alias for each type of `path` that the crate `tiny-skia` exports again,
such as `type Path = path.Path`, so that a user needs one import.
`strict-num` gives the checked float types of `tiny-skia-path`, and it is
small, so it goes in `path`.

## Why the rest of tiny-skia stays in the root package

The folders `scan`, `shaders`, `pipeline` and `path64` of `src` are modules of
one crate, and they read the private items of each other. The pipeline reads
`Paint`, `Pixmap`, `Shader` and the mask, and the shaders push stages into the
pipeline. A Go package for any of them makes an import cycle, or makes private
items public. D10 says how their names stay apart. `src/wide` imports nothing
from the rest of the crate, so `internal/wide` is a package of its own.

## Why `svg` is one package

A user parses and renders with one import, as with the C API of resvg. The
tree of usvg is the input of resvg, and the two crates share many types, so
one package needs no exported types between them. The name of each Go file
starts with the name of its crate, then its folder, such as
`usvg_parser_svgtree.go` and `resvg_filter_lighting.go`. The tree types of
usvg are exported, because a user reads them, as upstream does.

## Why each crate that `svg` uses is a package of its own

Each such crate is a repository of its own with a ledger of its own (D12). A
package for each crate keeps one Go file for each Rust file, keeps the names
of each crate apart, and runs the tests of each crate as the tests of its
package. The packages are internal, because their types are not part of the
API of resvg. If `svg` must export one of their types, it declares a type
alias.

Astra, Gemini Pro and DeepSeek Pro each proposed one public package for usvg
and resvg, with the crates that they use in internal packages, on 2026-11-09.
