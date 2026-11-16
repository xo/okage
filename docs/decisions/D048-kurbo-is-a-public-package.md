# D48. kurbo is a public package

Status: Decided, amends D5 and D7.

The port of the subset of the crate kurbo moves from `internal/kurbo` to
`kurbo`, a public package of the module `github.com/xo/okage`. The
maintainers decided it on 2026-11-16, for ivy, as D47 did for roxmltree.

ivy ports the PDF library hayro, which draws with the geometry of kurbo, and
ivy D7 says that ivy uses the port of kurbo in okage, not a second one. Go
does not let another module import an internal package, so the port must be
public.

1. The code of the package does not change. Only its import path changes, to
   `github.com/xo/okage/kurbo`, and the comments that named the old path.
2. The package keeps its subset, its upstream, its ledger and its license
   (D7, D12, D13). The subset grows only for a user of okage. An item that
   only ivy needs is ported in ivy, and the maintainers decide later whether
   it moves here.
3. It still builds with TinyGo (D25).

A change to the API of the package is now a change to a public API, so it
follows the release rules of okage.
