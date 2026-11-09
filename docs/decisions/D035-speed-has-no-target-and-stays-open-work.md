# D35. Speed has no target, and stays open work

Status: Decided, amends D18.

D18 set a target: the SIMD build takes at most twice the time of upstream
with AVX2 on the benchmarks of fills. On 2026-11-09, after D33, the SIMD
build takes about 4 to 11 times the time of upstream with AVX2, and about 1.2
to 3 times the time of the scalar code of upstream. To reach the target needs
a new form of the pipeline, and GoAT was not taken (`docs/PLAN.md`).

## The decision

1. okage has no speed target. The target of D18 is closed, and no gate of the
   plan and no release waits for it.
2. Work that makes okage faster is always welcome. A change for speed keeps
   the rules of D16 and D33: the same bytes in every build, and a test that
   shows it.
3. The benchmarks of D18 stay. The nightly job of CI runs them (D24), and a
   change for speed records its times before and after.
4. `docs/BACKLOG.md` keeps the known steps that can make okage faster.
