# D10. A name that collides takes the name of its module

Status: Decided.

Rust keeps the private items of each module apart. A Go package has one name
space, so a private function that two modules of one crate define collides
when both modules go into one Go package. On 2026-11-09:

1. tiny-skia has 47 such names. 39 of them come from `pipeline/lowp.rs` and
   `pipeline/highp.rs`, which define the same stages. The other 8 come from
   `scan/path.rs` and `scan/path_aa.rs`, and from `path64`.
2. usvg and resvg have 16 such names. `convert` is defined in 12 modules, and
   `apply` in 12.

The rules:

1. A stage of `lowp` or `highp` takes a `*Pipeline` of its own module in Rust.
   In Go, each stage is a method of `lowpPipeline` or of `highpPipeline`, and
   it keeps the Rust name: `store_8888` becomes `(*lowpPipeline).store8888`
   and `(*highpPipeline).store8888`.
2. Any other free function whose name collides takes the name of its module
   as a suffix in MixedCaps: `convert` in `usvg/src/parser/clippath.rs`
   becomes `convertClipPath`, and `fill_path` in `src/scan/path_aa.rs`
   becomes `fillPathPathAA`.
3. The doc comment names the full Rust path, so a search for the Rust name
   still finds the Go function.
4. A name that does not collide keeps its Rust name. A later upstream commit
   that makes a new collision renames only the function that it adds.

A suffix keeps the Rust name first, so a sorted list of Go names keeps the
Rust names together. The three models each proposed a prefix or a suffix on
2026-11-09.
