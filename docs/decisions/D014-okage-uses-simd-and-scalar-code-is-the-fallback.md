# D14. okage uses SIMD, and scalar code is the fallback

Status: Decided.

okage uses SIMD, and it prepares for a future in which Go has SIMD as a
standard feature. Code without SIMD is the fallback. SIMD (single instruction,
multiple data) means one instruction that works on several values at once.

Go 1.27 has SIMD only as the experiment `GOEXPERIMENT=simd`, with the packages
`simd` and `simd/archsimd`. A library cannot turn on an experiment for the
programs that import it. So okage has two builds:

1. The scalar build, which every program gets today.
2. The SIMD build, which a program gets when it builds with
   `GOEXPERIMENT=simd`.

When Go makes SIMD a standard feature, the SIMD build becomes the default.
D15 says how the vector types do this, and [`docs/SIMD.md`](../SIMD.md) holds
the details.
