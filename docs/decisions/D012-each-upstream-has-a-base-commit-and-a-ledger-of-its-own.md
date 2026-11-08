# D12. Each upstream has a base commit and a ledger of its own

Status: Decided.

okage ports from several upstream repositories. Each has its own history, so
one timeline of all of them has no meaning. Astra, Gemini Pro and DeepSeek Pro
each advised one ledger for each repository on 2026-11-09.

1. `docs/upstream/upstreams.tsv` lists each upstream repository: its URL, its
   checkout (D11), the crates that okage ports from it with their versions,
   the okage package of each, its policy (D7), its base commit and the last
   commit that okage matches.
2. The base commit of each repository is the commit of the version that the
   file `Cargo.lock` of resvg `v0.49.0` names. resvg `v0.49.0` is commit
   `66ee0dc7d787d7e0b0ab3085b7849f78838cf2ba`, and it names tiny-skia 0.12.0,
   which is commit `e48b9027762d33731e60e40eb17eaa4f583b0185`. The renderer
   and the SVG code start from the same code.
3. Each repository has its ledger in `docs/upstream/<repository>.tsv`, with
   the columns `upstream`, `date`, `status`, `okage` and `note`. A test makes
   sure that each ledger has one line for each commit after its base commit,
   with no gap, when the checkout exists.
4. Each new upstream commit is sorted as it arrives: `ported`,
   `not-applicable` or `pending`.
5. After a base port, okage ports the commits of a repository in the order of
   that repository, one okage commit for each upstream commit.
6. A commit of one upstream can raise the version of another upstream that
   okage ports. Then okage first ports the commits of the other upstream up to
   the new version, and after that the commit that raises it.
7. The column `matched` of a repository never moves past a commit that is
   still `pending`.
8. okage follows upstream when a maintainer asks. No scheduled task fetches or
   ports upstream.

On 2026-11-09 tiny-skia has three commits after its base commit, and resvg has
26. tiny-skia made about 10 commits in a year, and resvg about 96.

This follows transit D30, D116 and D117.
