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

### Make the checkouts and the ledgers of the other upstreams

From D11 and D12. Clone strict-num, svgtypes, roxmltree, simplecss, kurbo and
xmlwriter into `upstream/`, and make the ledger of each one from its base
commit in `docs/upstream/upstreams.tsv`.

### Build the Rust oracle and the test module

From D22. One scene renders in Go and in Rust, and the test compares the
bytes.

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

### Measure the math functions of Go against Rust

From risk 3 in `docs/PLAN.md` and D16. tiny-skia calls `cos`, `sin`, `acos`
and `powf`. Compare `math.Cos`, `math.Sin`, `math.Acos` and `math.Pow` of Go
with the functions that Rust calls on Linux, for the arguments that the tests
of tiny-skia pass. If the bits differ in a way that changes a pixel, write a
decision. This belongs to phase 3, before its gate.
