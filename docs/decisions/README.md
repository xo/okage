# Decisions

Every decision of this project is a file in this folder, named by its number
and its title. This table is the index. Find the number here, then open the
file.

Each file opens with its status. `Decided` means that the maintainers chose
it. `Proposed` means that it waits for the maintainers. A decision that
changes an earlier one says so in its status, as `Amends D3`, and the earlier
one says it back, as `Amended by D9`. Read the status before the decision.

A bare number, such as D3, names a decision in this folder. A decision of
another repository names that repository, such as dbmeta D110.

A new decision gets the next number and a file of its own. Add its row here.
An open question is not a decision. It goes at the end of
[`../PLAN.md`](../PLAN.md) until the maintainers answer it.

The decisions were consolidated on 2026-11-09, before the first commit (D4).

| # | Decision | Status |
| --- | --- | --- |
| D1 | [okage is a pure Go port of tiny-skia and resvg](D001-okage-is-a-pure-go-port-of-tiny-skia-and-resvg.md) | Decided |
| D2 | [okage is set up for coding agents as dbmeta D110 says](D002-okage-is-set-up-for-coding-agents-as-dbmeta-d110-says.md) | Decided |
| D3 | [The plan and the rules come before any code](D003-the-plan-and-the-rules-come-before-any-code.md) | Decided |
| D4 | [Each decision is a file of its own](D004-each-decision-is-a-file-of-its-own.md) | Decided |
| D5 | [The packages follow the crates](D005-the-packages-follow-the-crates.md) | Decided, amended by D38 and D41 |
| D6 | [The goal is no dependencies](D006-the-goal-is-no-dependencies.md) | Decided |
| D7 | [Each crate has a policy: port, subset, replace or none](D007-each-crate-has-a-policy-port-subset-replace-or-none.md) | Decided, amended by D41 |
| D8 | [The XML and CSS parsers are ports of roxmltree and simplecss](D008-the-xml-and-css-parsers-are-ports-of-roxmltree-and-simplecss.md) | Decided |
| D9 | [Text uses go-text, and the layout of text of usvg is ported](D009-text-uses-go-text-and-the-layout-of-text-of-usvg-is-ported.md) | Decided, amended by D32 and D36 |
| D10 | [A name that collides takes the name of its module](D010-a-name-that-collides-takes-the-name-of-its-module.md) | Decided |
| D11 | [The upstream checkouts are in `upstream/`, and git ignores them](D011-the-upstream-checkouts-are-in-upstream-and-git-ignores-them.md) | Decided |
| D12 | [Each upstream has a base commit and a ledger of its own](D012-each-upstream-has-a-base-commit-and-a-ledger-of-its-own.md) | Decided |
| D13 | [Each package keeps the license of its upstream](D013-each-package-keeps-the-license-of-its-upstream.md) | Decided |
| D14 | [okage uses SIMD, and scalar code is the fallback](D014-okage-uses-simd-and-scalar-code-is-the-fallback.md) | Decided |
| D15 | [The vector types keep fixed widths and check the CPU once](D015-the-vector-types-keep-fixed-widths-and-check-the-cpu-once.md) | Decided, amended by D28 |
| D16 | [The renderer gives the same bytes in every build](D016-the-renderer-gives-the-same-bytes-in-every-build.md) | Decided, amended by D34 |
| D17 | [A prototype chooses the form of the pipeline](D017-a-prototype-chooses-the-form-of-the-pipeline.md) | Decided |
| D18 | [The SIMD build is within twice the time of upstream](D018-the-simd-build-is-within-twice-the-time-of-upstream.md) | Decided, amended by D35 |
| D19 | [A `Pixmap` works with the package `image`](D019-a-pixmap-works-with-the-image-package.md) | Decided |
| D20 | [Options use the functional options pattern, and logs go to `slog`](D020-options-use-the-functional-options-pattern-and-logs-go-to-slog.md) | Decided |
| D21 | [Sentinel errors live in the root package](D021-sentinel-errors-live-in-the-root-package.md) | Decided |
| D22 | [The test module holds the oracles and the comparisons](D022-the-test-module-holds-the-oracles-and-the-comparisons.md) | Decided |
| D23 | [The output of `svg` is compared within a tolerance](D023-the-output-of-svg-is-compared-within-a-tolerance.md) | Decided |
| D24 | [CI tests amd64 on each push, and the other platforms at night](D024-ci-tests-amd64-on-each-push-and-the-other-platforms-at-night.md) | Decided |
| D25 | [okage builds with TinyGo](D025-okage-builds-with-tinygo.md) | Decided |
| D26 | [The math functions are correctly rounded](D026-the-math-functions-are-correctly-rounded.md) | Decided |
| D27 | [A Rust enum with data becomes a struct or an interface](D027-a-rust-enum-with-data-becomes-a-struct-or-an-interface.md) | Decided |
| D28 | [The vector types are arrays in both builds](D028-the-vector-types-are-arrays-in-both-builds.md) | Decided, amends D15, amended by D33 |
| D29 | [The filter types of `svg` take the suffix `Filter`](D029-the-filter-types-of-svg-take-the-suffix-filter.md) | Decided |
| D30 | [An arc makes at most 4,096 curves](D030-an-arc-makes-at-most-4096-curves.md) | Decided |
| D31 | [Images are compared with the pixel metric of resvg](D031-images-are-compared-with-the-pixel-metric-of-resvg.md) | Decided |
| D32 | [`svg` walks the paint graph of COLRv1](D032-svg-walks-the-paint-graph-of-colrv1.md) | Decided, amends D9 |
| D33 | [A hot stage computes its lanes in one loop](D033-a-hot-stage-computes-its-lanes-in-one-loop.md) | Decided, amends D28 |
| D34 | [Powf reads one NaN for every NaN](D034-powf-reads-one-nan-for-every-nan.md) | Decided, amends D16 |
| D35 | [Speed has no target, and stays open work](D035-speed-has-no-target-and-stays-open-work.md) | Decided, amends D18 |
| D36 | [`svg` has a renderer that loads the system fonts](D036-svg-has-a-renderer-that-loads-the-system-fonts.md) | Decided, amends D9 |
| D37 | [`svg` shapes a run of text without SVG data](D037-svg-shapes-a-run-of-text-without-svg-data.md) | Decided, amended by D39 |
| D38 | [`html` parses and lays out the HTML labels of Graphviz](D038-html-parses-and-lays-out-the-html-labels-of-graphviz.md) | Decided, amends D5 |
| D39 | [Text falls back past a missing character, and `ShapeText` draws color glyphs](D039-text-falls-back-past-a-missing-character-and-draws-color-glyphs.md) | Decided, amends D37, amended by D40 |
| D40 | [Text falls back for an emoji that the font does not draw as an emoji](D040-text-falls-back-for-an-emoji-that-the-font-does-not-draw-as-emoji.md) | Decided, amends D39 |
| D41 | [`lottie` renders Lottie animations, as a port of velato](D041-lottie-renders-lottie-animations-as-a-port-of-velato.md) | Decided, amends D5 and D7, amended by D42 and D43 |
| D42 | [`lottie` draws merge paths by compositing](D042-lottie-draws-merge-paths-by-compositing.md) | Decided, amends D41, amended by D43 |
| D43 | [`lottie` draws as ThorVG where velato differs](D043-lottie-draws-as-thorvg-where-velato-differs.md) | Decided, amends D41 and D42 |
