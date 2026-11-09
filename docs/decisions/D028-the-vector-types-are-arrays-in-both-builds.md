# D28. The vector types are arrays in both builds

Status: Proposed, amends D15.

D15 says that an `F32x8` of the SIMD build is one `archsimd.Float32x8` on
amd64, and that the pipeline never indexes a lane. The port of tiny-skia
showed on 2026-11-09 that neither holds:

1. The pipeline of upstream indexes lanes in 54 places in `lowp` and 11 in
   `highp`, in the loads, the stores and the gradients. A faithful port
   indexes them too.
2. A vector type that is an `archsimd` type cannot be indexed, and it cannot
   be a field of a struct without a cost, as the documentation of `archsimd`
   says.

## The decision

1. Each vector type of `internal/wide` is an array in every build, such as
   `type F32x8 [8]float32`. The pipeline indexes a lane where upstream does.
2. Each method has a plain function, in `<type>.go`, that every build has.
   The scalar build calls it from the method in `<type>_scalar.go`.
3. In the SIMD build, a method loads its arrays into `archsimd` registers,
   works on them and stores the result, but only where that is faster than
   the plain function. The program reads the CPU once (D15), and a method
   with a SIMD path checks that flag. A method without one calls its plain
   function.
4. The tool `internal/wide/internal/widegen` writes the files of the two
   builds from the plain functions and from one table that turns the SIMD
   path of each method on or off. A test makes sure that the files are what
   the tool writes.
5. The SIMD path of a method is on only where it is at least 1.2 times as
   fast as the scalar build on amd64.

## What was measured

On 2026-11-09, on amd64 with AVX2, in ns for each call:

| Method | Scalar build | SIMD path |
| --- | --- | --- |
| `F32x8.Add` | 9.0 | 13.6 |
| `F32x8.Round` | 45 | 32 |
| `F32x8.Powf` | 647 | 31 |
| `F32x16.Floor` | 707 | 54 |
| `U16x16.Blend` | 31.7 | 12.1 |

The scalar `Round` took 337 ns before it became a loop on the lanes (D18).
The load and the store of each call cost more than a simple operation saves.
Only the methods that do much work in one call gain. The paths that are on
are `Floor`, `Round`, `RoundInt` and the like, `Powf`, `Normalize` and
`U16x16.Blend`.

In Go 1.27.3, a build with `GOEXPERIMENT=simd` also makes ordinary code
slower. A method that calls its plain function takes about 13 ns there,
against 9.7 ns in the scalar build. The 1.2 times of point 5 is measured
against the scalar build, so a path that is on gains even with that cost.

## What it leaves

The pipeline keeps its values in arrays between two stages, so the SIMD build
cannot keep them in registers. The benchmarks of D18 show the cost. A form of
the pipeline that keeps its values in `archsimd` registers through a run of
stages is the work of D17, and `docs/BACKLOG.md` holds it.
