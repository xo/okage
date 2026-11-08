# D11. The upstream checkouts are in `upstream/`, and git ignores them

Status: Decided.

Each upstream repository has a checkout in `upstream/<repository>/`, in the
root of this repository, such as `upstream/tiny-skia/` and `upstream/resvg/`.
`.gitignore` holds `/upstream`. The column `checkout` of
`docs/upstream/upstreams.tsv` names the folder of each one.

A checkout is the reference for the port. A person or an agent reads it, runs
`git log` and `git diff` in it, and builds the Rust oracle from it (D22).
Nobody edits a file in it.

No upstream holds a Go file on 2026-11-09, so `go build ./...` finds nothing
in the checkouts. [`docs/UPSTREAM.md`](../UPSTREAM.md) holds the command that
finds a Go file that an upstream adds.

This follows transit D4, with one folder for many checkouts.
