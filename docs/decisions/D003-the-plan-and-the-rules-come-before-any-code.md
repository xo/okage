# D3. The plan and the rules come before any code

Status: Decided.

No code is written before the plan is ready. Three documents come first:

1. [`docs/PLAN.md`](../PLAN.md), which holds the plan and the open questions.
2. [`docs/UPSTREAM.md`](../UPSTREAM.md), which holds the rules for porting
   each upstream and each change that it makes.
3. [`docs/SIMD.md`](../SIMD.md), which holds the rules for the vector code.

Code includes Go source, Go tests, scripts and the CI workflow. A document,
the license, `go.mod` and the files that git reads, such as `.gitignore`, are
not code.

The rules for following upstream, for the two builds and for the same bytes
in each build touch every file that is ported. If they are not settled before
the first file, each file that comes before them follows a different rule.
transit D3 made the same rule for the same reason.

The maintainers decided on 2026-11-09 that the plan is ready when its
decisions are consolidated. This set of decisions is the consolidated one, so
phase 1 starts when it is committed.
