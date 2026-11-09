# Backlog

This document holds the work that is known and not done, with where each item
came from. When an item is done, delete it, and record anything that was
decided in [`decisions/`](decisions/README.md).

Work on the port goes in units: one upstream file with its tests. Claim a unit
under "Units in progress" before you start, with the name of your session.
When the change is committed, delete the claim.

## Units in progress

None.

## Phase 1

### Add the tests of the agent setup and of the documents

From D2. The other `xo` repositories have `skills_test.go` and
`docs_test.go`. They make sure that the skills match `skills-lock.json`, that
the root holds only the four documents, that every decision has a row in
`docs/decisions/README.md`, that every decision reference exists, and that the
prose follows the rules of the `simple-english` skill that a machine can
check. They are Go code, so they come in phase 1 (D3).

### Add the ledger test

From D12. The test reads `docs/upstream/upstreams.tsv`, each ledger in
`docs/upstream/`, and each checkout in `upstream/`. It fails when a commit of
an upstream after its base commit has no line, or when the column `matched`
is past a commit that is `pending`. It skips an upstream whose checkout is
missing.

### Build the prototype of the pipeline

From D17 and D15. The real pipelines of three cases, in `lowp` and `highp`,
with the three choices of D17, in both builds, on amd64 and arm64, and with
TinyGo for `wasip1`. Record the measurements in D17. Also measure
whether `GODEBUG=simd` changes what `archsimd` runs, or what `archsimd.X86`
reports.

### Write the CI workflow

From the testing plan in `docs/PLAN.md`, D25 and D24: linux amd64 in both
builds on each push, and arm64, s390x under QEMU, `wasip1`, TinyGo, the
oracles, the comparison of builds and the benchmarks at night.

## Phase 3

### Make the f64 functions of crmath faster

From D26. `crmath.Acos64` and `crmath.Cos64` compute with `math/big` at 160
bits, about 9 microseconds for each call. Compute them in double-double
precision first, and use `math/big` only when that result is too close to
the middle of two `float64` values. Only the clipping of a cubic calls them.

## Phase 6

### Port the example select of simplecss

From phase 6. `examples/select.rs` of simplecss matches selectors against a
tree of roxmltree. It was not ported while `internal/roxmltree` was being
written. Port it as an example of `internal/simplecss`, with its output.

## Phase 10

### Follow the font model of read-fonts 0.46 more closely

From phase 10, resvg commit `04291b74`, and D9 rule 3. usvg now reads fonts
through the owned font model of read-fonts 0.46, which skrifa 0.49 exports.
`svg/usvg_text_cache.go` and `resolveFont` follow its rules on go-text. Three
differences stay:

1. read-fonts keeps a metric with its MVAR delta as an F48Dot16. Go keeps it
   as a float32, so a rounded metric can differ at a location with a delta.
2. The walk of the paint graph of COLRv1 in `svg/usvg_text_colr_paint.go`
   ports skrifa 0.44 (D32). usvg now walks the graph with the model of read-fonts
   0.46. No test shows a difference.
3. read-fonts reads the strikes of the table EBDT. usvg draws none of its
   glyphs, so Go leaves the strikes out. The result is the same.

## Phase 4

### Close the gap to the speed target

From D18 and D28. okage is about 10 to 20 times slower than the scalar code of
upstream. Measure and try, in this order, keeping the bytes of every test:

1. The forms of D17 that are not measured: one loop with a `switch` on the
   stage, and the color channels in local variables.
2. Methods of the vector types that the compiler does not inline, and bounds
   checks in the stages (`go build -gcflags='-m -d=ssa/check_bce'`).
3. A copy of the stages that keeps its values in `archsimd` registers through
   a run of stages, written by a tool, for the SIMD build (D17, D28).
4. Rounding and the like in other vector methods, as done for `Round` and
   `Floor`, each with a test on all 2^32 values of a lane.

### Measure the cost of GOEXPERIMENT=simd in each release of Go

From D18 and D28. In Go 1.27.3 the plain functions are slower in a build with
`GOEXPERIMENT=simd`. Measure it again with each release of Go, as the steps
of "A new release of Go" in `docs/SIMD.md` say, and turn the SIMD paths of
`internal/wide/internal/widegen` on or off again from the measurements.
