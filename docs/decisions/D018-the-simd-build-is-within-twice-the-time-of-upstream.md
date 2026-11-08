# D18. The SIMD build is within twice the time of upstream

Status: Decided.

1. The SIMD build of okage takes at most twice the time of upstream with AVX2
   on the ported benchmarks of fills, on amd64.
2. The speed of the scalar build is measured and recorded, with no target.

Phase 4 measures both, with the benchmarks of tiny-skia that okage ports. The
nightly job of CI runs them (D24).
