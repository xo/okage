# D33. A hot stage computes its lanes in one loop

Status: Decided, amends D28.

D28 keeps the channels of the pipeline in arrays, and its rule says that a
stage calls the methods of the vector types and indexes a lane only where
upstream does. Each method is a loop on the lanes that reads two arrays and
writes one. The Go compiler keeps no array of more than one element in
registers, so a chain such as `div255(d.Mul(inv(sa)))` makes several loops
and several trips through memory. LLVM fuses the same chain in Rust into a
few instructions. This was the first cause of the gap of D18.

## The decision

1. A helper or a stage of the pipeline can compute its result in one loop on
   the lanes, with scalar Go for each lane, in place of a chain of methods of
   the vector types.
2. The loop must give the same bits as the chain for every input:
   - Integer code keeps the operations of upstream. The `uint16` arithmetic of
     Go wraps as `u16` wraps in a release build of Rust.
   - Float code keeps the order of each operation of upstream, writes
     `float32(x*y) + z` where upstream multiplies and adds (D16), and uses the
     lane semantics of each method that it replaces: Rust `min` and `max`,
     saturating casts and the rounding of `internal/wide`.
3. The function keeps the name, the signature and the place that upstream
   gives it. A comment that starts with `Port (D33)` quotes the expression of
   upstream. A function for one lane, such as `div255l`, sits next to the
   helper for the vector.
4. A test keeps the chain of methods of each such stage in a reference copy,
   and compares the two, bit for bit, on random values. The values include
   values outside the range that the pipeline holds, so that a difference in
   wrapping or in rounding shows. `pipeline_lowp_ref_test.go` and
   `TestLowpFused` do this for lowp.
5. Only a stage that a profile shows to be hot is written this way. The other
   stages keep the methods of the vector types.

## What was measured

On 2026-11-09, on an AMD Ryzen 9 9950X, in ms for each fill of the
benchmarks of D18. The stages of lowp and of highp that a profile showed to
be hot are written this way, and the lane functions that they call are in
`internal/wide`. The lerp of lowp alone went from 67 ns to 12 ns for 16
lanes. Before the change, the SIMD build was slower than the scalar build.
After it, the two builds take about the same time.

| Benchmark | Before, scalar | After, scalar | After, SIMD | upstream, scalar |
| --- | --- | --- | --- | --- |
| `BlendSourceOver` | 7.06 | 1.85 | 1.67 | 0.61 |
| `BlendColorDodge` | 53.3 | 6.75 | 6.42 | 3.62 |
| `BlendHue` | 75.9 | 7.48 | 7.37 | 4.75 |
| `GradientsTwoStopsLinearPad` | 29.7 | 6.34 | 6.31 | 2.47 |
| `GradientsTwoStopsLinearPadHQ` | 37.7 | 4.55 | 4.57 | 3.78 |
| `PatternsPlain` | 39.0 | 6.84 | 6.86 | |
| `PatternsHQ` | 307 | 73.6 | 73.6 | |
| `FillRect` | 10.6 | 2.61 | 2.57 | |
| `FillPathAA` | 6.37 | 1.66 | 1.72 | |
| `HairlineAA` | 33.5 | 8.61 | 8.60 | |

The tests `TestLowpFused`, `TestLowpGradientFused` and `TestHighpFused`
compare each such stage with its chain of methods. The two tests of floats
take any two NaN values as equal, as the comparison of the builds in
`internal/wide` does, because the Go compiler can swap the operands of an
addition or a multiplication, and that changes the payload of a NaN. A NaN
does not reach a pixel, because `unnorm` turns it into 0.
