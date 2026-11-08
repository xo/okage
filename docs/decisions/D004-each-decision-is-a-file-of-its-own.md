# D4. Each decision is a file of its own

Status: Decided.

dbmeta D111 says that a large project keeps each decision in a file of its
own under `docs/decisions/`, and that a small library keeps its decisions in
its plan. okage is a large project by that measure. The port of tiny-skia is
about 17,000 lines of Rust, and the port of usvg, resvg and the crates that
they use is about 29,000 more. It has 157 and 1,742 integration tests, two
builds, two oracles and a ledger for each upstream repository.

Each decision is `docs/decisions/D<nnn>-<title>.md`, and
[`README.md`](README.md) in this folder is the index. The decisions were
consolidated on 2026-11-09, before the first commit, into the 25 of this
folder.
