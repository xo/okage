# D16. The renderer gives the same bytes in every build

Status: Decided.

The packages of tiny-skia, which are the root package, `path` and
`internal/wide`, give the same bytes as the scalar build of upstream, which is
upstream built with `--no-default-features`. The SIMD build gives the same
bytes as the scalar build, on every platform, and when it falls back to the
scalar implementation (D15). A test of the renderer compares bytes, with no
tolerance.

This is stricter than upstream. The backends of upstream can differ in a pixel
that no test reads, because two of them approximate a reciprocal. The
reference images of upstream are the same for every backend, so they do not
show such a pixel.

`svg` does not give the same bytes as resvg, because it decodes images and
lays out text with other code (D7, D9). Its two builds still give the same
bytes as each other, because they share the renderer. D23 says how its output
is compared.

## The rules

1. A reciprocal and a reciprocal square root are exact, `1 / x` and
   `1 / sqrt(x)`. Upstream approximates them with `rcp_ps` on x86 and with
   `vrecpeq` and one step of refinement on NEON, only in the blend modes color
   dodge and color burn. Its scalar code and its WebAssembly code are exact.
   This is a difference from upstream, and `README.md` lists it.
2. A multiply and an add are never fused. Rust does not fuse `f * m + a`, and
   the helper `mad` of upstream is an unfused multiply and add. The Go
   compiler can fuse `x*y + z` on architectures such as arm64, also across
   statements. An explicit conversion stops it, so okage writes
   `float32(x*y) + z`. okage never calls `MulAdd`.
3. A conversion from a float to an integer uses a helper of `internal/wide`
   that gives the Rust result: truncation that saturates, or rounding half to
   even. Go gives an implementation-specific value for a float that does not
   fit, and `archsimd` on amd64 gives `0x80000000`.
4. `min` and `max` port the `fast_min` and `fast_max` of upstream, with their
   rule for NaN.

## The math functions

tiny-skia calls `cos`, `sin`, `acos` and `powf`. If the functions of `math`
give other bits than the functions that Rust calls on Linux, a pixel can
differ. The measurement and what okage does are in D26.

## Why

One reference makes each test simple: a test compares bytes and needs no
tolerance. A test that passes on amd64 also passes on arm64 and in the SIMD
build, and a fault in one build shows as a difference from the other. An
exact reciprocal costs a division in two blend modes, and the rules 2 and 3
cost little.
