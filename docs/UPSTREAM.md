# Following upstream

This document holds the rules for porting each upstream repository to okage,
and for porting each change that an upstream makes after that.

The rules were written on 2026-11-09, before any code existed (D3). Each rule
names the decision that it comes from.

## The upstreams

`docs/upstream/upstreams.tsv` lists each upstream repository that okage ports
from (D12). It has these columns, separated by a tab:

| Column | What it holds |
| --- | --- |
| `repository` | the short name, which is also the name of its ledger and of its checkout |
| `url` | the URL to clone |
| `checkout` | the folder of the checkout (D11) |
| `crates` | the crates that okage ports from it, with their versions |
| `okage` | the okage package of each crate |
| `policy` | `port` or `subset` (D7) |
| `base` | the full hash of the base commit |
| `matched` | the full hash of the last commit that okage matches, or empty before the base port is done |

On 2026-11-09 it lists tiny-skia, strict-num, resvg, svgtypes, roxmltree,
simplecss, kurbo and xmlwriter. okage ports no crate for images, text or
fonts, so those crates are not in the list (D7, D9).

## The checkouts

Each upstream has a checkout in `upstream/<repository>/`, in the root of this
repository, and git ignores the folder `upstream/` (D11). A checkout is a
reference. Do not edit a file in it, and do not commit it.

If a checkout is missing, make it in the root of the repository with the URL
of the list:

```bash
git clone https://github.com/linebender/tiny-skia.git upstream/tiny-skia
```

Before you port anything, make sure that the checkout is clean. This command
must print nothing:

```bash
git -C upstream/tiny-skia status --short
```

No upstream holds a Go file on 2026-11-09. If one adds a Go file,
`go build ./...` finds it. After each fetch, run this command. It must print
nothing:

```bash
find upstream -name '*.go'
```

## The base commits

The base commit of each upstream is the commit of the version that the file
`Cargo.lock` of resvg `v0.49.0` names (D12). resvg `v0.49.0` is commit
`66ee0dc7d787d7e0b0ab3085b7849f78838cf2ba`, and it names tiny-skia 0.12.0,
commit `e48b9027762d33731e60e40eb17eaa4f583b0185`. The list holds the full
hash of each base commit.

The base port of each upstream is made from its base commit. The column
`matched` stays empty until the base port of that upstream is done.

## Where each upstream path goes

Each upstream path has one rule. A commit takes the strongest rule of the
paths that it changes: Port, then Review, then Not applicable.

### tiny-skia

| Upstream path | okage path | Rule |
| --- | --- | --- |
| `path/src/<name>.rs` | `path/<name>.go` | Port |
| `path/src/lib.rs` | `path/lib.go`, and the package comment in `path/doc.go`. `path/path.go` is the port of `path/src/path.rs` | Port |
| `path/src/f32x2_t.rs`, `path/src/f32x4_t.rs` | `path/f32x2.go`, `path/f32x4.go` | Port |
| `src/<name>.rs` | `<name>.go` in the root | Port |
| `src/lib.rs` | `doc.go` in the root, and the type aliases of `path` in `okage.go` | Port |
| `src/<folder>/<name>.rs` | `<folder>_<name>.go` in the root, such as `scan_path_aa.go` (D5) | Port |
| `src/<folder>/mod.rs` | `<folder>.go` in the root, such as `pipeline.go` | Port |
| `src/wide/<name>_t.rs` | `internal/wide/<name>.go`, `<name>_scalar.go` and `<name>_simd.go` (D15) | Port |
| `src/wide/mod.rs` | `internal/wide/wide.go` | Port |
| `tests/integration/<name>.rs` | `<name>_integration_test.go` in the root, or in `path/` for a test of a path only | Port |
| `tests/images/**` | `testdata/images/**`, copied byte for byte | Port |
| `benches/src/<name>.rs` | `bench_<name>_test.go` in the root | Port |
| `examples/<name>.rs` | an `Example` function in `example_test.go` | Port |
| `CHANGELOG.md`, `Cargo.toml`, `path/Cargo.toml` | none | Review. A new dependency or a new feature can change what okage ports |
| `LICENSE`, `path/LICENSE` | `LICENSE` | Review |
| `testing-tools/**` | none | Not applicable |
| `.github/**`, `.cargo/**`, `.typos.toml`, `.gitignore`, `README.md`, `path/README.md`, `tests/README.md` | none | Not applicable |

### strict-num

| Upstream path | okage path | Rule |
| --- | --- | --- |
| `src/lib.rs` | `path/strict_num.go` | Port |
| everything else | none | Review |

### resvg

| Upstream path | okage path | Rule |
| --- | --- | --- |
| `crates/usvg/src/<name>.rs` | `svg/usvg_<name>.go` | Port |
| `crates/usvg/src/<folder>/<name>.rs` | `svg/usvg_<folder>_<name>.go`, such as `usvg_parser_svgtree_names.go` for `parser/svgtree/names.rs` | Port |
| `crates/usvg/src/<folder>/mod.rs` | `svg/usvg_<folder>.go` | Port |
| `crates/usvg/src/main.rs` | none | Not applicable. It is the command line tool |
| `crates/resvg/src/<name>.rs` | `svg/resvg_<name>.go` | Port |
| `crates/resvg/src/<folder>/<name>.rs` | `svg/resvg_<folder>_<name>.go` | Port |
| `crates/resvg/src/main.rs` | none | Not applicable. It is the command line tool |
| `crates/resvg/tests/tests/**` | `svg/testdata/tests/**`, copied byte for byte | Port |
| `crates/resvg/tests/fonts/**`, `crates/resvg/tests/resources/**` | `svg/testdata/fonts/**`, `svg/testdata/resources/**` | Port |
| `crates/resvg/tests/integration/**` | `svg/integration_test.go` | Port |
| `crates/usvg/tests/<name>.rs` | `svg/usvg_<name>_test.go` | Port |
| `crates/usvg/tests/files/**` | `svg/testdata/usvg/**`, copied byte for byte | Port |
| `crates/usvg/codegen/**` | none | Review. It writes `parser/svgtree/names.rs`, which okage ports as a Go `map` (D7) |
| `crates/c-api/**` | none | Not applicable |
| `Cargo.toml`, `Cargo.lock`, `crates/*/Cargo.toml` | none | Review. A new version of a crate of D7 starts the steps of "A new version of a dependency" |
| `CHANGELOG.md`, `LICENSE-*`, `docs/**` | none | Review |
| `tools/**`, `.github/**`, `README.md` and the other files of the root | none | Not applicable |

### The crates under `internal/`

For `svgtypes`, `roxmltree`, `simplecss` and `xmlwriter`, the rule is the same
for each one:

| Upstream path | okage path | Rule |
| --- | --- | --- |
| `src/<name>.rs` | `internal/<crate>/<name>.go` | Port |
| `src/<folder>/<name>.rs` | `internal/<crate>/<folder>_<name>.go` | Port |
| `tests/**` | `internal/<crate>/<name>_test.go` and `internal/<crate>/testdata/**` | Port |
| `Cargo.toml`, `CHANGELOG.md`, `LICENSE*` | none | Review |
| everything else | none | Not applicable |

For `kurbo`, the policy is `subset` (D7). A Rust file that holds a type or a
function of the subset is Port, and the rest of `src/` is Not applicable.
`internal/kurbo/SUBSET.md` lists the types and functions of the subset, from
phase 6 on.

### File names

The name of a Go file uses the base name of the Rust file without the suffix
`_t`. Go reads a name that ends in `_test.go` as a test, and a name that
ends in a name of an operating system or an architecture, such as `_wasm.go`
or `_linux.go`, as a build constraint. If a Rust file name ends in such a
word, the table of its upstream gets a row for it.

## How Go code follows Rust code

The Go is idiomatic Go, and it still follows the Rust closely enough that a
port of a later upstream change is a small edit. These rules do both.

The link to the Rust:

1. One Rust file becomes one Go file, as the tables above say.
2. The functions in the Go file are in the same order as in the Rust file.
3. A Go name is the Rust name in MixedCaps. A public Rust function or method
   becomes an exported Go one, and a private one becomes an unexported one.
   `fill_path` becomes `FillPath`. A method of a Rust trait becomes a method
   of the Go type that implements it.
4. A private name that two modules of one Go package define goes apart as D10
   says. A stage of `lowp` or `highp` is a method of `lowpPipeline` or
   `highpPipeline`. Any other name that collides takes the name of its module
   as a suffix in MixedCaps: `convert` in `parser/clippath.rs` becomes
   `convertClipPath`.
5. The doc comment of each ported function names the Rust function, with its
   crate and path: `FillPath is Pixmap::fill_path in tiny-skia
   src/painter.rs.` Search the Go code for a Rust name to find its port.
6. A Rust local variable keeps its name, in Go form. `last_y` becomes
   `lastY`.
7. A comment in the Rust code is ported with the code that it describes.
8. The behavior is the same, including the faults of upstream (D1).

The idiomatic Go:

9. A Rust integer keeps its width and its sign, so that overflow and
   conversion behave as in Rust. A `u32` becomes a `uint32`. Rust panics on
   an overflow in a debug build and wraps in a release build. okage wraps, as
   the release build does.
10. An `f32` stays a `float32`, and an `f64` stays a `float64`. Do not compute
    in another width and convert, unless the Rust code converts.
    `math.Sqrt` and `math.Floor` take a `float64`, and they give the same
    `float32` result after the conversion, so they can be used for an `f32`.
11. Where Rust multiplies and adds, write `float32(x*y) + z`, or
    `float64(x*y) + z` for an `f64`, so that the Go compiler cannot fuse the
    two operations (D16).
12. A Rust `Option<T>` that a function returns becomes `(T, bool)`. A
    `Result<T, E>` becomes `(T, error)`. A Rust `panic!` or `unwrap` that
    upstream relies on becomes a Go `panic` with the same condition.
13. A Rust enum without data becomes a typed constant with a `String` method.
    A Rust enum with data becomes a struct with a kind field, or an interface
    with one type for each case, as D27 says.
14. A checked type, such as `NormalizedF32`, becomes a defined type with a
    constructor that checks the value and returns `(T, bool)`.
15. A Rust slice stays a Go slice. A fixed Rust array, such as `[f32; 8]`,
    stays a Go array, unless it is a large buffer, which becomes a slice that
    is allocated once (D25).
16. A Rust iterator that the API returns becomes an `iter.Seq` or an
    `iter.Seq2`.
17. `#[inline]` and `#[inline(always)]` have no port. If a benchmark shows
    that the Go compiler does not inline a small function that upstream
    inlines, write the idea in [`BACKLOG.md`](BACKLOG.md).
18. A port does not use `reflect`, `encoding/json`, `encoding/xml` or
    `runtime/debug`, so that TinyGo can build it (D25).
19. A port commit translates the Rust into idiomatic Go and keeps the
    behavior. It does not restructure the code more than idiomatic Go needs,
    because a change that does more makes the next port harder. If the Go code
    can be better in another way, write the idea in `BACKLOG.md`.

### The vector types

The pipeline calls the methods of the types of `internal/wide`, and never an
array index of a vector, so that the code does not change with the build.
[`SIMD.md`](SIMD.md) holds the rules of the two implementations.

A Rust method of a vector type that has a `cfg_if!` block with one branch for
each backend becomes one Go method in each implementation: the scalar branch
of upstream in `<name>_scalar.go`, and the code of `simd/archsimd` in
`<name>_simd.go`. The scalar branch of upstream is the reference for both.

## Porting an upstream change

These steps start for an upstream when its base port is done, in phase 5 for
tiny-skia and in phase 10 for the others. Until then, its ledger records each
new commit as it arrives, as "The ledger" below says.

1. Fetch the changes of the upstream:

   ```bash
   git -C upstream/<repository> fetch origin
   ```

2. Run the command from "The checkouts" that finds Go files. It must print
   nothing.

3. List the commits that okage has not ported, oldest first. Take the
   matched commit from `docs/upstream/upstreams.tsv`:

   ```bash
   git -C upstream/<repository> log --reverse --format='%H %cs %s' <matched>..origin/main
   ```

4. Take the oldest commit. List the paths that it changes:

   ```bash
   git -C upstream/<repository> show --stat --format='%H %s' <commit>
   ```

5. Find the rule of each path in the table of the upstream. The commit takes
   the strongest rule.

6. If the rule is Not applicable, set the status of the commit in the ledger
   to `not-applicable`, with the reason. Go to step 11.

7. If the rule is Review, read the diff:

   ```bash
   git -C upstream/<repository> show <commit>
   ```

   If the diff changes nothing that okage ports, set the status to
   `not-applicable`, with the reason. Go to step 11. If the diff raises the
   version of a crate of D7, follow "A new version of a dependency" first.
   Otherwise, continue at step 8.

8. Port the diff. Find each changed Rust function by its name in the Go code,
   and change the same lines. Port each test, each reference image and each
   test font that the commit adds or changes.

9. Run every test, as "Before you stage" in [`AGENTS.md`](../AGENTS.md) says,
   in the scalar build and in the SIMD build. Every test must pass.

10. Set the status of the commit in the ledger to `ported`, with the okage
    commit. For resvg, also set the `rev` of usvg and resvg in
    `test/oracle/rust/Cargo.toml` to the commit, so that the Rust oracle
    answers as upstream does at that commit.

11. Write the full hash of the commit in the column `matched` of
    `docs/upstream/upstreams.tsv`.

12. Stage the change, and give a maintainer a commit message in the form
    below. Do not commit. A maintainer commits (D2).

13. Go back to step 4 for the next commit.

Port one upstream commit in one okage commit. You can put a run of
`not-applicable` commits of one upstream in one okage commit, because each
one changes only its ledger and the list of upstreams.

### A new version of a dependency

When a commit of one upstream raises the version of another upstream that
okage ports, the dependency comes first (D12):

1. Find the commit of the new version in the checkout of the dependency.
2. Port the commits of the dependency up to that commit, with the steps
   above.
3. Change the column `crates` of the dependency in the list of upstreams to
   the new version.
4. Port the commit that raised the version.

If the new version is in a new repository, add the repository to the list of
upstreams and make its ledger first. A decision records the new upstream and
its policy (D7).

### The commit message

An okage commit that ports an upstream commit has this form:

```text
<area>: <the subject of the upstream commit>

<what changed in okage, in plain English, and why, if the upstream subject
does not say it>

Upstream: <owner>/<repository>@<full hash>
```

`<area>` is the okage package that the commit changes most, such as `path`,
`raster`, `pipeline`, `shaders`, `wide`, `svg`, `svgtypes`, `roxmltree`,
`simplecss`, `kurbo`, `xmlwriter`, `test` or `bench`. The `Upstream:` line is
the last line before the trailers that the agent adds, such as
`Upstream: linebender/resvg@66ee0dc7d787d7e0b0ab3085b7849f78838cf2ba`. A tool
can read the ledgers again from these lines.

A commit that records only `not-applicable` commits has this form:

```text
upstream: record <n> commits of <repository> that change nothing okage ports

Upstream: <owner>/<repository>@<first full hash>..<last full hash>
```

## The ledger

Each upstream has its ledger in `docs/upstream/<repository>.tsv` (D12). A
ledger lists every commit of its upstream after the base commit, oldest
first, and what okage did with it. It has these columns, separated by a tab:

| Column | What it holds |
| --- | --- |
| `upstream` | the full hash of the upstream commit |
| `date` | the commit date, as `YYYY-MM-DD` |
| `status` | `ported`, `not-applicable` or `pending` |
| `okage` | the okage commit that ported it, or empty |
| `note` | the subject of the upstream commit, and the reason for `not-applicable` |

A test of phase 1 makes sure that each ledger has one line for each commit
from its base commit on, with no gap, when the checkout exists. The test
skips an upstream whose checkout is missing.

The column `matched` of an upstream never moves past a commit that is still
`pending` (D12).

On 2026-11-09 eight ledgers exist:

1. `tiny-skia.tsv` holds the three commits after `v0.12.0`. `88c65f6e`
   renames a function in `examples/pattern.rs`, and it is `ported`, because
   okage ports the examples. `ed266088` drops the crate `arrayref`. It is
   `not-applicable`, because the Go code already has the form that it gives
   to `path/src/dash.rs` and `src/pipeline/mod.rs`. `5d475477` changes the CI
   of upstream only, and it is `not-applicable`. The column `matched` is
   `5d475477`.
2. `resvg.tsv` holds the 26 commits after `v0.49.0`, up to `847abd69` on
   2026-11-05. Each one changes code or tests that okage ports, so each one
   is `pending`, and its note says which parts it changes. `e1a4c92d` raises
   harfrust to 0.15 and skrifa to 0.49, and changes `usvg/src/text`. okage
   ports its change to `usvg/src/text`, with go-text in place of the crates,
   and not the new versions of the crates (D9).
3. `strict-num.tsv` holds the 7 commits after the base commit. None of them
   changes code that okage ports, so each one is `not-applicable`, and the
   column `matched` is `76d36feb`.
4. `svgtypes.tsv`, `roxmltree.tsv`, `simplecss.tsv`, `kurbo.tsv` and
   `xmlwriter.tsv` hold the commits after the base commits of phase 6. Each
   commit that changes ported code or tests is `pending`, and its note says
   which files it changes. The others are `not-applicable`, with the
   reason. okage ports the pending commits in phase 10, after usvg raises the
   version of the crate.

## A difference from upstream

okage differs from upstream only where a decision says so. Each difference
is in the list "Differences from upstream" in `README.md`, with its decision.
On 2026-11-09 the list holds three differences: the SIMD build computes a
reciprocal exactly, where the SIMD backends of tiny-skia approximate it
(D16), `svg` decodes images with other code (D7), and `svg` lays out text
with go-text (D9).
