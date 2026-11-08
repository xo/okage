# D17. A prototype chooses the form of the pipeline

Status: Decided.

Upstream runs the stages of the pipeline through a table of function
pointers, and each stage calls the next. Go has no tail calls, and a call
through a function value stops inlining. On WebAssembly such a call is a
`call_indirect`, which is slow. The pipeline must also run the SIMD or the
scalar implementation of the vector types, chosen when the program starts
(D15).

Phase 1 builds a prototype that runs the real pipelines of upstream for three
cases, in `lowp` and in `highp`:

1. A fill of a rectangle with a solid color.
2. A fill of a path with anti-aliasing and a linear gradient.
3. A fill of a path with a pattern, a clip mask and the blend mode color
   dodge.

It measures three choices:

1. The form of a stage: a table of `func(*pipeline)`, as upstream has, or one
   function that runs a loop with a `switch` on the stage number.
2. The way the pipeline runs the SIMD or the scalar implementation: generic
   functions over the vector types, two copies of the stages that a tool
   writes, or one choice at the entry of a pipeline.
3. The color channels in a struct, as upstream has, or in local variables.
   The documentation of `archsimd` advises against vectors in a struct and
   against the address of a vector.

It measures each choice in the scalar build and in the SIMD build, on amd64
and on arm64, for small and large images, and with TinyGo for `wasip1`. It
records the time, the allocations and the size of the binary, with repeated
runs. It also measures whether `GODEBUG=simd` changes what `archsimd` runs,
or what `archsimd.X86` reports.

The fastest form becomes the form of the port, and this decision records the
measurements. If two forms are within 10 percent of each other in every case,
the port uses the form that is closer to upstream. All three models said on
2026-11-09 that a prototype of 10 stages is too small to decide.
