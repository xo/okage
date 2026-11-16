# D47. roxmltree is a public package

Status: Decided, amends D5 and D8.

The port of the crate roxmltree moves from `internal/roxmltree` to
`roxmltree`, a public package of the module `github.com/xo/okage`. The
maintainers decided it on 2026-11-16, for ivy.

ivy decodes formats that are XML, such as XPS, Office Open XML and EPUB, and
ivy D7 says that ivy reads XML with the XML parser of okage, not with
`encoding/xml`. Go does not let another module import an internal package,
so the parser must be public.

1. The code of the package does not change. Only its import path changes, to
   `github.com/xo/okage/roxmltree`, and the comments that named the old path.
2. The package keeps its upstream, its ledger and its license (D12, D13).
3. Its errors still wrap the sentinel errors of the root package, such as
   `okage.ErrXMLUnknownToken` (D21).
4. It still builds with TinyGo (D25).
5. The function `fromXMLTree` of `svg` stays unexported. It was unexported
   because a user was not able to make a document of roxmltree. Its export is open
   work in [`docs/BACKLOG.md`](../BACKLOG.md).

A change to the API of the package is now a change to a public API, so it
follows the release rules of okage.
