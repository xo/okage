# D34. Powf reads one NaN for every NaN

Status: Decided, amends D16.

D16 asks for the same bytes in every build. A NaN can have many bit patterns,
and the bits of a NaN are not fixed:

1. A NaN that an invalid operation such as 0/0 makes is `0xffc00000` on x86
   and `0x7fc00000` on ARM, in Rust and in Go alike.
2. An operation on two NaN values passes one of them on. Which one depends
   on the order of the operands, and LLVM and the Go compiler can each swap
   the operands of an addition or a multiplication.

Almost no code of the pipeline reads the bits of a NaN. `unnorm` and the
saturating casts turn a NaN into 0 before it reaches a pixel. `Powf` is the
exception. It approximates log2 from the bits of each lane, so a NaN gives a
finite result that depends on its bits. `Powf` runs only in the stages of
gamma, which a `Paint` with a `Colorspace` other than `Linear` adds. `svg`
does not set one.

## The decision

1. `Powf` replaces each NaN lane with the NaN `0xffc00000` before it reads
   the bits, in the scalar code and in the SIMD code of `internal/wide`.
   Every build and every architecture then gives the same result for a NaN.
2. `0xffc00000` is the NaN that x86 makes for an invalid operation. The Rust
   oracle runs on amd64, so the usual NaN gives the result of upstream there.
3. Any other code that reads the bits of a float in the pipeline does the
   same, if it can read a NaN.
4. A test that compares two floats of the pipeline, such as the tests of D33,
   takes any two NaN values as equal.

This is a difference from upstream for a NaN that is not `0xffc00000`, in
the list of `README.md`. `TestF32x8PowfNaN` checks that each NaN gives the
result of `0xffc00000`.
