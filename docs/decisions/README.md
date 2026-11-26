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
| D5 | [The packages follow the crates](D005-the-packages-follow-the-crates.md) | Decided, amended by D38, D41, D47, D48, D49, D50, D51, D52, D62, D78 and D88 |
| D6 | [The goal is no dependencies](D006-the-goal-is-no-dependencies.md) | Decided, amended by D52 |
| D7 | [Each crate has a policy: port, subset, replace or none](D007-each-crate-has-a-policy-port-subset-replace-or-none.md) | Decided, amended by D41, D45, D48, D52, D62 and D78 |
| D8 | [The XML and CSS parsers are ports of roxmltree and simplecss](D008-the-xml-and-css-parsers-are-ports-of-roxmltree-and-simplecss.md) | Decided, amended by D47 and D50 |
| D9 | [Text uses go-text, and the layout of text of usvg is ported](D009-text-uses-go-text-and-the-layout-of-text-of-usvg-is-ported.md) | Decided, amended by D32, D36, D45, D52, D62 and D78 |
| D10 | [A name that collides takes the name of its module](D010-a-name-that-collides-takes-the-name-of-its-module.md) | Decided |
| D11 | [The upstream checkouts are in `upstream/`, and git ignores them](D011-the-upstream-checkouts-are-in-upstream-and-git-ignores-them.md) | Decided |
| D12 | [Each upstream has a base commit and a ledger of its own](D012-each-upstream-has-a-base-commit-and-a-ledger-of-its-own.md) | Decided, amended by D52 |
| D13 | [Each package keeps the license of its upstream](D013-each-package-keeps-the-license-of-its-upstream.md) | Decided, amended by D45, D49, D50, D52 and D78 |
| D14 | [okage uses SIMD, and scalar code is the fallback](D014-okage-uses-simd-and-scalar-code-is-the-fallback.md) | Decided |
| D15 | [The vector types keep fixed widths and check the CPU once](D015-the-vector-types-keep-fixed-widths-and-check-the-cpu-once.md) | Decided, amended by D28 |
| D16 | [The renderer gives the same bytes in every build](D016-the-renderer-gives-the-same-bytes-in-every-build.md) | Decided, amended by D34, D54, D71 and D74 |
| D17 | [A prototype chooses the form of the pipeline](D017-a-prototype-chooses-the-form-of-the-pipeline.md) | Decided, amended by D96 and D98 |
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
| D32 | [`svg` walks the paint graph of COLRv1](D032-svg-walks-the-paint-graph-of-colrv1.md) | Decided, amends D9, amended by D62 and D100 |
| D33 | [A hot stage computes its lanes in one loop](D033-a-hot-stage-computes-its-lanes-in-one-loop.md) | Decided, amends D28 |
| D34 | [Powf reads one NaN for every NaN](D034-powf-reads-one-nan-for-every-nan.md) | Decided, amends D16 |
| D35 | [Speed has no target, and stays open work](D035-speed-has-no-target-and-stays-open-work.md) | Decided, amends D18 |
| D36 | [`svg` has a renderer that loads the system fonts](D036-svg-has-a-renderer-that-loads-the-system-fonts.md) | Decided, amends D9, amended by D46 |
| D37 | [`svg` shapes a run of text without SVG data](D037-svg-shapes-a-run-of-text-without-svg-data.md) | Decided, amended by D39 and D57 |
| D38 | [`html` parses and lays out the HTML labels of Graphviz](D038-html-parses-and-lays-out-the-html-labels-of-graphviz.md) | Decided, amends D5, amended by D49 |
| D39 | [Text falls back past a missing character, and `ShapeText` draws color glyphs](D039-text-falls-back-past-a-missing-character-and-draws-color-glyphs.md) | Decided, amends D37, amended by D40 |
| D40 | [Text falls back for an emoji that the font does not draw as an emoji](D040-text-falls-back-for-an-emoji-that-the-font-does-not-draw-as-emoji.md) | Decided, amends D39 |
| D41 | [`lottie` renders Lottie animations, as a port of velato](D041-lottie-renders-lottie-animations-as-a-port-of-velato.md) | Decided, amends D5 and D7, amended by D42, D43, D44, D75, D79, D82 and D91 |
| D42 | [`lottie` draws merge paths by compositing](D042-lottie-draws-merge-paths-by-compositing.md) | Decided, amends D41, amended by D43 and D94 |
| D43 | [`lottie` draws as ThorVG where velato differs](D043-lottie-draws-as-thorvg-where-velato-differs.md) | Decided, amends D41 and D42 |
| D44 | [`lottie` loads files and draws frames for a program](D044-lottie-loads-and-draws-frames-for-a-program.md) | Decided, amends D41 |
| D45 | [`svg` loads WOFF, WOFF2 and EOT fonts, and reads their names](D045-svg-loads-woff-woff2-and-eot-fonts-and-reads-their-names.md) | Decided, amends D7, D9 and D13, amended by D51 and D91 |
| D46 | [`FontDatabase` loads the fonts of the system as the renderer does](D046-fontdatabase-loads-the-fonts-of-the-system-as-the-renderer-does.md) | Decided, amends D36 |
| D47 | [roxmltree is a public package](D047-roxmltree-is-a-public-package.md) | Decided, amends D5 and D8, amended by D58 |
| D48 | [kurbo is a public package](D048-kurbo-is-a-public-package.md) | Decided, amends D5 and D7 |
| D49 | [`html` parses HTML documents, as a port of tinyhtml5](D049-html-parses-html-documents-as-a-port-of-tinyhtml5.md) | Decided, amends D5, D13 and D38 |
| D50 | [tinycss2 and cssselect2 are public ports](D050-tinycss2-and-cssselect2-are-public-ports.md) | Decided, amends D5, D8 and D13, amended by D58 |
| D51 | [webfont is a public package](D051-webfont-is-a-public-package.md) | Decided, amends D5 and D45 |
| D52 | [typesetting is a public copy of go-text](D052-typesetting-is-a-public-copy-of-go-text.md) | Decided, amends D5, D6, D7, D9, D12 and D13, amended by D63 |
| D53 | [Huge paths fill, and aligned images skip the pipeline](D053-huge-paths-fill-and-aligned-images-skip-the-pipeline.md) | Decided |
| D54 | [`Paint` takes a finer coverage and stroke adjustment](D054-paint-takes-a-finer-coverage-and-stroke-adjustment.md) | Decided, amends D16 |
| D55 | [Mesh gradients and the decal spread mode](D055-mesh-gradients-and-the-decal-spread-mode.md) | Decided |
| D56 | [okage composites the transparency groups of PDF](D056-okage-composites-the-transparency-groups-of-pdf.md) | Decided |
| D57 | [`svg` gives the faces, the clusters and the color glyphs of text](D057-svg-gives-the-faces-the-clusters-and-the-color-glyphs-of-text.md) | Decided, amends D37 |
| D58 | [SVG images for a layout, and :has() in linear time](D058-svg-images-for-a-layout-and-has-in-linear-time.md) | Decided, amends D47 and D50 |
| D59 | [typesetting draws seac and the FontMatrix, and fixes the digest](D059-typesetting-draws-seac-and-the-fontmatrix-and-fixes-the-digest.md) | Decided |
| D60 | [A masked fill skips the pixels that its mask hides](D060-a-masked-fill-skips-the-pixels-that-its-mask-hides.md) | Decided, amended by D86 |
| D61 | [The font loader limits tables, and the matcher groups selectors by name](D061-the-font-loader-limits-tables-and-the-matcher-groups-selectors-by-name.md) | Decided |
| D62 | [`pdf` writes PDF files, as a port of krilla](D062-pdf-writes-pdf-files-as-a-port-of-krilla.md) | Decided, amends D5, D7, D9 and D32, amended by D66, D68, D80, D89 and D99 |
| D63 | [The tests of go-text read copies of their fonts](D063-the-tests-of-go-text-read-copies-of-their-fonts.md) | Decided, amends D52, amended by D76 |
| D64 | [Radial gradients with the extend rule of PDF](D064-radial-gradients-with-the-extend-rule-of-pdf.md) | Decided |
| D65 | [typesetting fixes the variable fonts of the subsetter](D065-typesetting-fixes-the-variable-fonts-of-the-subsetter.md) | Decided, amended by D67 and D84 |
| D66 | [`pdf` returns errors for large strokes and mixed stops](D066-pdf-returns-errors-for-large-strokes-and-mixed-stops.md) | Decided, amends D62 |
| D67 | [typesetting applies the fixes of malformed input out of order](D067-typesetting-applies-the-fixes-of-malformed-input-out-of-order.md) | Decided, amends D65 |
| D68 | [`pdf` adds what the PDF device of ivy needs](D068-pdf-adds-what-the-pdf-device-of-ivy-needs.md) | Decided, amends D62, amended by D69 and D89 |
| D69 | [Dashes, many stops and image masks for the PDF device of ivy](D069-dashes-many-stops-and-image-masks-for-the-pdf-device-of-ivy.md) | Decided, amends D68 |
| D70 | [shaping gives the flags of HarfBuzz and reuses its glyphs](D070-shaping-gives-the-flags-of-harfbuzz-and-reuses-its-glyphs.md) | Decided, amended by D77 and D93 |
| D71 | [The copy of go-text does not fuse a multiply and an add](D071-the-copy-of-go-text-does-not-fuse-a-multiply-and-an-add.md) | Decided, amends D16 |
| D72 | [typesetting skips work that left-to-right Latin text does not need](D072-typesetting-skips-work-that-left-to-right-latin-text-does-not-need.md) | Decided, amended by D81 and D92 |
| D73 | [typesetting fixes the faults that go-text still has](D073-typesetting-fixes-the-faults-that-go-text-still-has.md) | Decided, amended by D97 |
| D74 | [pdf reuses its compressor, limits a subset, and a stroke ends on NaN](D074-pdf-reuses-its-compressor-limits-a-subset-and-a-stroke-ends-on-nan.md) | Decided, amends D16 |
| D75 | [lottie draws image assets in the pixmap sink](D075-lottie-draws-image-assets-in-the-pixmap-sink.md) | Decided, amends D41, amended by D82 |
| D76 | [The test fonts of go-text are a module of their own](D076-the-test-fonts-of-go-text-are-a-module-of-their-own.md) | Decided, amends D63 |
| D77 | [shaping shapes a paragraph once for its lines](D077-shaping-shapes-a-paragraph-once-for-its-lines.md) | Decided, amends D70, amended by D93 |
| D78 | [`texmath` lays out math, as a port of go-tex/math](D078-texmath-lays-out-math-as-a-port-of-go-tex-math.md) | Decided, amends D5, D7, D9 and D13, amended by D95 |
| D79 | [lottie draws text layers, blurs and drop shadows](D079-lottie-draws-text-layers-blurs-and-drop-shadows.md) | Decided, amends D41, amended by D82, D85, D88 and D94 |
| D80 | [pdf draws a font as paths when its subset fails](D080-pdf-draws-a-font-as-paths-when-its-subset-fails.md) | Decided, amends D62 |
| D81 | [typesetting skips repeated work in the shaper](D081-typesetting-skips-repeated-work-in-the-shaper.md) | Decided, amends D72, amended by D83 and D90 |
| D82 | [lottie follows ThorVG at the edges and in the tracking](D082-lottie-follows-thorvg-at-the-edges-and-in-the-tracking.md) | Decided, amends D41, D75 and D79 |
| D83 | [typesetting reads the Unicode tables and the layout tables faster](D083-typesetting-reads-the-unicode-tables-and-the-layout-tables-faster.md) | Decided, amends D81 |
| D84 | [typesetting computes the deltas of variable fonts in fixed point](D084-typesetting-computes-the-deltas-of-variable-fonts-in-fixed-point.md) | Decided, amends D65, amended by D101 |
| D85 | [lottie draws text animators, capitals and text on a path](D085-lottie-draws-text-animators-capitals-and-text-on-a-path.md) | Decided, amends D79 |
| D86 | [A mask can hold a rectangle and no pixels](D086-a-mask-can-hold-a-rectangle-and-no-pixels.md) | Decided, amends D60 |
| D87 | [typesetting walks the glyphs and splits the lines with less work](D087-typesetting-walks-the-glyphs-and-splits-the-lines-with-less-work.md) | Decided |
| D88 | [The blur and the color matrix of the filters get a public API](D088-the-blur-and-the-color-matrix-of-the-filters-get-a-public-api.md) | Decided, amends D5 and D79 |
| D89 | [pdf makes a Type 3 font from images that the caller gives](D089-pdf-makes-a-type-3-font-from-images-that-the-caller-gives.md) | Decided, amends D62 and D68 |
| D90 | [harfbuzz scales the values of a font as HarfBuzz does, in 32 bits](D090-harfbuzz-scales-the-values-of-a-font-as-harfbuzz-does-in-32-bits.md) | Decided, amends D81, amended by D97 |
| D91 | [lottie ignores expressions, and webfont reads no MicroType Express](D091-lottie-ignores-expressions-and-webfont-reads-no-microtype-express.md) | Decided, amends D41 and D45 |
| D92 | [typesetting finds the line breaks alone and scans runes with SIMD](D092-typesetting-finds-the-line-breaks-alone-and-scans-runes-with-simd.md) | Decided, amends D72 |
| D93 | [shaping gives the flag unsafe-to-concat, and Line shapes reversed runs again](D093-shaping-gives-the-flag-unsafe-to-concat-and-line-shapes-reversed-runs-again.md) | Decided, amends D70 and D77, amended by D97 |
| D94 | [lottie strokes the result of a merge path, and blurs at each quality](D094-lottie-strokes-the-result-of-a-merge-path-and-blurs-at-each-quality.md) | Decided, amends D42 and D79 |
| D95 | [texmath gets the rest of the math backlog](D095-texmath-gets-the-rest-of-the-math-backlog.md) | Decided, amends D78 |
| D96 | [The speed work of phase 4, and the cost of the SIMD build](D096-the-speed-work-of-phase-4-and-the-cost-of-the-simd-build.md) | Decided, amends D17, amended by D98 |
| D97 | [harfbuzz follows HarfBuzz 15.2.0](D097-harfbuzz-follows-harfbuzz-15-2-0.md) | Decided, amends D73, D90 and D93 |
| D98 | [The pipeline runs fused copies of common programs of stages](D098-the-pipeline-runs-fused-copies-of-common-programs-of-stages.md) | Decided, amends D17 and D96 |
| D99 | [`pdf` writes XMP metadata and validates PDF/A](D099-pdf-writes-xmp-metadata-and-validates-pdf-a.md) | Decided, amends D62 |
| D100 | [svg follows the font model of read-fonts 0.46](D100-svg-follows-the-font-model-of-read-fonts-0-46.md) | Decided, amends D32, amended by D101 |
| D101 | [typesetting normalizes as skrifa, reads long deltas and the null offsets of trak](D101-typesetting-normalizes-as-skrifa-reads-long-deltas-and-null-offsets-of-trak.md) | Decided, amends D84 and D100 |
| D102 | [`pdf` gives a glyph a second CID for a second text](D102-pdf-gives-a-glyph-a-second-cid-for-a-second-text.md) | Decided, amends D62 |
| D103 | [texmath sets dots, primes, accent scripts and aligned cells as TeX](D103-texmath-sets-dots-primes-accent-scripts-and-aligned-cells-as-tex.md) | Decided, amends D78 |
| D104 | [`pdf` links to a named destination by its name alone](D104-pdf-links-to-a-named-destination-by-its-name-alone.md) | Decided, amends D62 |
