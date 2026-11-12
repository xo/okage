# D7. Each crate has a policy: port, subset, replace or none

Status: Decided, amended by D41.

Each crate that tiny-skia, usvg and resvg use has one policy:

1. Port: okage ports the crate file by file, with a ledger (D12).
2. Subset: okage ports only the files and functions that usvg or resvg calls,
   with a ledger. A commit that changes only other files is `not-applicable`.
3. Replace: the Go standard library, or an outside package that the
   maintainers approve (D6), does the work. okage ports no code of the crate.
4. None: Go does the work without a crate.

okage ports no crate that decodes or encodes an image, and no crate for text,
fonts or the shaping of text. A difference from upstream that comes from the
code that replaces them is accepted.

| Crate | Used by | Policy | Go form |
| --- | --- | --- | --- |
| `tiny-skia`, `tiny-skia-path` | resvg, usvg | Port | root package, `path` |
| `strict-num` | tiny-skia-path, usvg | Port | `path` |
| `svgtypes` | usvg, resvg | Port | `internal/svgtypes` |
| `roxmltree` | usvg | Port (D8) | `internal/roxmltree` |
| `simplecss` | usvg | Port (D8) | `internal/simplecss` |
| `xmlwriter` | usvg | Port | `internal/xmlwriter` |
| `kurbo` | usvg | Subset: `CubicBez`, `QuadBez`, `Line`, `SvgArc`, `Point`, `Vec2` and the traits `ParamCurve`, `ParamCurveArclen` and `ParamCurveDeriv` | `internal/kurbo` |
| `siphasher` | usvg | Replace. usvg hashes the names of elements and attributes into a table. A Go `map` or `switch` gives the same lookup, and the hash is never visible | none |
| `base64`, `data-url` | usvg | Replace with `encoding/base64` and a small parser of `data:` URLs in `svg` | none |
| `flate2` | usvg | Replace with `compress/gzip`, for SVGZ files | none |
| `imagesize` | usvg | Replace with `image.DecodeConfig` and the decoders below | none |
| `png` | tiny-skia, resvg | Replace with `image/png` | none |
| `gif`, `zune-jpeg` | resvg | Replace with `image/gif` and `image/jpeg` | none |
| `image-webp` | resvg | Replace with `golang.org/x/image/webp` (D6) | none |
| `fontdb`, `harfrust`, `skrifa`, `read-fonts`, `unicode-bidi`, `unicode-script`, `unicode-vo` | usvg | Replace with `github.com/go-text/typesetting` (D9) | none |
| `log` | tiny-skia, usvg, resvg | Replace with a `*slog.Logger` that the caller gives (D20) | none |
| `arrayvec` | tiny-skia | None. A fixed array with a length, or a slice with a capacity, does the work | none |
| `libm` | tiny-skia-path | None. It gives float functions to a `no_std` build, which okage does not port | none |
| `pico-args`, `bytemuck`, `rgb`, `once_cell`, `arrayref`, `cfg-if` | usvg, resvg, tiny-skia | None. They read command line flags or convert types, which Go does without a crate | none |

The versions are the ones that the file `Cargo.lock` of resvg `v0.49.0` names
(D12).

A replacement in the packages of tiny-skia must give the same bytes (D16). A
replacement in `svg` can give other pixels than upstream, and the tolerance of
D23 measures it.

Astra, Gemini Pro and DeepSeek Pro agreed on the split of port, subset and
replace on 2026-11-09.
