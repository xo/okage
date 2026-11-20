// Package texmath holds the oracles of the math layout of okage (D78). It
// has no code of its own.
//
// fontTools reads the table MATH of a font with mathtable.py, and the test
// compares each value with the reader of typesetting. KaTeX, at the version
// that Mermaid 12.3.0 bundles, parses each formula of corpus.txt in Node with
// katex/katex.mjs, and gives its height and depth in em. Three tests use it:
//
//   - The parse gate: each formula that KaTeX parses, texmath.Parse parses
//     too, except a formula of katex_parse.txt, with its missing command
//     and its reason.
//   - The layout gate: the height and the depth of each formula that both
//     lay out are within 15 percent of the values of KaTeX, except a
//     formula of katex_layout.txt, with its reason.
//   - The report: when Chromium or Chrome is on the machine, the test writes
//     pages of HTML that set each formula with KaTeX and with texmath side
//     by side, and Chromium prints each page to a PDF file. The report is
//     not a gate. report_test.go says where it goes.
//
// To write katex_parse.txt and katex_layout.txt again, run the tests with
// the environment variable TEXMATH_KATEX_UPDATE=1, and read the diff. The
// snapshots of the layout are in the package texmath, in
// texmath/testdata/snapshots, so that the nightly jobs run them on each
// platform.
//
// A test skips when python3, fontTools, node or the packages of npm are
// missing. To install the packages of npm, run this command in the folder
// test/texmath/katex. It installs the versions of package-lock.json into
// test/texmath/katex/node_modules, which git ignores:
//
//	npm ci
package texmath
