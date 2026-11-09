# Contributing to okage

Before you change anything, read three things:

1. [`AGENTS.md`](AGENTS.md) holds the rules. It is written for a coding agent,
   and every rule in it holds for a person too. `CLAUDE.md` holds one line
   that imports it, for Claude Code.
2. [`docs/PLAN.md`](docs/PLAN.md) holds the plan, and the open questions at
   its end.
3. [`docs/decisions/`](docs/decisions/README.md) holds every decision, one
   file each. Read the status of a decision before the decision.

Do not decide an open question on your own. Add it to the end of
`docs/PLAN.md`, and ask a maintainer.

On 2026-11-09 the port of tiny-skia is done, and its speed is the open work.
`svg` has the tree of usvg without text, and the renderer has not started.
`docs/PLAN.md` holds the status of each phase.

## The upstream checkouts

okage is a port, and the port is made from a checkout of each upstream
repository in `upstream/<repository>/`, in the root of the repository. Git
ignores the folder (D11). `docs/upstream/upstreams.tsv` lists each repository
and its URL. If a checkout is missing, make it in the root of the repository:

```bash
git clone https://github.com/linebender/tiny-skia.git upstream/tiny-skia
```

```bash
git clone https://github.com/linebender/resvg.git upstream/resvg
```

Do not edit a file in a checkout. [`docs/UPSTREAM.md`](docs/UPSTREAM.md) says
how to port upstream code.

## The two builds

okage has a scalar build and a SIMD build (D15). Go 1.27 builds the SIMD
build only with `GOEXPERIMENT=simd`. Run each test in both builds:

```bash
go test ./...
```

```bash
GOEXPERIMENT=simd go test ./...
```

[`docs/SIMD.md`](docs/SIMD.md) says how the builds differ, and why both must
give the same bytes (D16). From phase 2 on, run the tests with TinyGo too:

```bash
tinygo test ./...
```

## Agent skills

The repository carries two agent skills. A skill is a set of instructions
that a coding agent loads for a task. `simple-english` sets how prose is
written, and `go-pedantry` sets how Go is written.

`skills-lock.json` names the source of each skill. The `skills` command from
npm writes that file, and version 1.7.0 is the one that the other `xo`
repositories measured. It writes each skill into two folders. Codex and the
other agents read `.agents/skills/<name>`, and Claude Code reads
`.claude/skills/<name>`.

To add a skill or to update one, run the command for it in the repository
root:

```bash
npx skills@1.7.0 add AminBlg/SimpleEnglish --skill simple-english --agent codex claude-code --copy -y
```

```bash
npx skills@1.7.0 add oborchers/fractional-cto --skill go-pedantry --agent codex claude-code --copy -y
```

Keep `--copy`. Without it, the command writes `.claude/skills/<name>` as a
symbolic link. A Windows checkout writes a symbolic link as a text file, and
Claude Code then loads no skill and reports nothing. See dbmeta D89.

`.claude/settings.local.json` holds the Claude Code permissions of one
person. The root `.gitignore` ignores it.

## Before you send a change

Run the commands under "Before you stage" in [`AGENTS.md`](AGENTS.md). Then
stage the change for review. A maintainer commits.

## Releases

A maintainer makes each release. The first tag, `v0.1.0`, comes when the port
of tiny-skia is done: at the end of phase 5, and after the gate of phase 4.
