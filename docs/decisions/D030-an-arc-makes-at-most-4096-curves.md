# D30. An arc makes at most 4,096 curves

Status: Decided.

kurbo splits an elliptical arc into cubic curves in `Arc::append_iter`. The
number of curves grows with the radius of the arc, and kurbo has no limit:

1. The number is the ceiling of `(1.1163 * r / tolerance)^(1/6)` for a full
   turn, where `r` is the larger radius. svgtypes uses the tolerance 0.1.
2. A path such as `M 30 40 A 40 30 20 1 1 1e50 100` gives about 3.3e8 curves,
   and a larger radius gives up to 2^64. Upstream then runs until the memory
   is gone. Phase 6 found this with random values, and usvg does not limit the
   values before it calls kurbo, so `svg.FromData` did not end either.

## The decision

1. `internal/kurbo` makes at most `MaxArcSubdivisions`, 4,096, curves for one
   arc. A larger number is cut to 4,096 before the angle of each curve is
   computed, so the curves still cover the whole sweep of the arc.
2. The limit changes an arc only when `r / tolerance` is about 4.23e21 or
   more: at the tolerance of svgtypes, a radius of about 4.23e20. Each smaller
   arc gives the same curves as upstream.
3. The limit is 4,096 and not higher, because each curve calls the correctly
   rounded sine and cosine of `internal/crmath` (D26). One arc at the limit
   takes about 0.2 seconds on the machine that measured on 2026-11-09, and
   one at 65,536 curves took about 3.6 seconds.
4. The oracle of the test module still skips such arcs, because upstream does
   not end on them. A unit test of `internal/kurbo` makes sure that an arc of
   radius 1e50, 1e300 or the largest float64 ends with 4,096 curves that reach
   the end of the arc.

This is a difference from upstream, in the list of `README.md`.
