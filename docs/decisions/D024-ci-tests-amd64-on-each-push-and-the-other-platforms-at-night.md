# D24. CI tests amd64 on each push, and the other platforms at night

Status: Decided.

CI runs on GitHub Actions.

1. Each push runs the tests on linux amd64, in the scalar build and in the
   SIMD build, without the oracles.
2. A nightly job runs the tests on arm64, on s390x under QEMU for a big-endian
   platform, on `wasip1`, and with TinyGo (D25). It also runs the tests of the
   oracles, the comparison of the builds for WebAssembly (D22) and the
   benchmarks (D18).

The run on arm64 finds a multiply and an add that the compiler fuses (D16).
The run on s390x matches upstream, which tests on big-endian PowerPC.
