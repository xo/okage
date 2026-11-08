# D2. okage is set up for coding agents as dbmeta D110 says

Status: Decided.

okage takes the structure of dbmeta and transit. dbmeta D110 sets the layout
and the base rules for coding agents in every `xo` repository, and this
decision adopts it. okage writes its own rules in the same form, and it does
not copy the policies of dbmeta word for word. Its `README.md` has the badges
and the links to the other `xo` projects, as the other `xo` repositories have.

The repository has these files in its root:

- `README.md`, for a person who uses okage.
- `AGENTS.md`, which holds the rules for a coding agent and for a person.
- `CLAUDE.md`, which holds one line, `@AGENTS.md`.
- `CONTRIBUTING.md`, for a person who changes okage.
- `LICENSE`, `go.mod`, `skills-lock.json`, `.gitignore` and `.gitattributes`.

The `docs/` folder holds the plan, the rules, the backlog, the decisions and
the ledgers. The `simple-english` and `go-pedantry` skills are ordinary
folders in `.agents/skills/` and in `.claude/skills/`, as dbmeta D89 says.
They are copies of the skills of transit, at the hashes that
`skills-lock.json` records.

`AGENTS.md` opens with the three standing rules of dbmeta D110. The first rule
is to stage every change for review, and to commit only when a maintainer says
so.

The other `xo` repositories have `skills_test.go` and `docs_test.go`, which
make sure that the skills are copies, that the root holds only four
documents, and that each decision reference exists. They are Go code, so they
come in phase 1 (D3).
