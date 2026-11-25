# D95. texmath gets the rest of the math backlog

Status: Decided, amends D78.

On 2026-11-25 the maintainers asked for all of the section "Math" of
`docs/BACKLOG.md` at once. Before that day, the items of that section
waited for a request of ivy or zuzu. This decision records what was built,
and how. It amends sections 3, 8, 10 and 11 and phases 26 and 27 of D78.

## 1. Phase 27

On 2026-11-25 the one new commit of go-tex/math after the base commit is
`e33cf138b`. It updates the version of go-opentype, and okage takes
go-opentype in no module (D78, choice 3). So the commit is not applicable.
The lead session records it in `docs/upstream/go-tex-math.tsv`. Phase 27 is
not a backlog item any more. It is the rule of "When upstream changes" in
D78, which applies to each new commit.

## 2. The helpers of D78 section 8

`typesetting/font/okage_pair.go` holds the two helpers of D78 section 8,
marked as D52 says:

```go
// KernPair returns the kern of the pair left, right in font units.
func (f *Face) KernPair(left, right GID) float32

// LigaturePair returns the ligature of the pair left, right.
func (f *Font) LigaturePair(left, right GID) (GID, bool)
```

1. `KernPair` reads the lookups of the feature `kern` of GPOS in the default
   language system of the script `DFLT`, or else of `latn`. Each lookup
   applies its first pair subtable that holds the pair, as HarfBuzz does. The
   kern is the change of the advance of the left glyph, with the delta of
   its device table at the coordinates of the face. A font without the
   feature `kern` in GPOS takes the horizontal subtables of the table `kern`.
   No contextual lookup applies.
2. `LigaturePair` reads the lookups of the feature `liga` of GSUB in the
   same language system. It is a method of `Font`, so a `Face` has it too.
   D78 gives it on `Face`, and a ligature does not depend on the coordinates
   of a face.
3. The tests compare both helpers with `shaping.HarfbuzzShaper` on three
   fonts of `svg/testdata/fonts`.

## 3. The commands of KaTeX

Each formula of the corpus that KaTeX parses, `texmath.Parse` now parses.
`test/texmath/katex_parse.txt` holds no formula. The commands are these:

1. The symbols of `src/symbols.ts` of KaTeX 0.18.11, with the code point and
   the class of KaTeX, go in the table `symbols` of `parse.go`: the
   relations, the binary operators, the arrows and the ordinary symbols of
   AMS, `\lparen`, `\lmoustache`, `\Updownarrow`, `\oiint`, `\oiiint`,
   `\cdotp`, `\ldotp` and the others. `\llcorner`, `\lBrace`, `\rBrace` and
   `\minuso` take the character of the MathML of KaTeX. `\varGamma` and its
   kin take the code points of math italic, because the math alphabets of
   `texmath` map the Latin letters only.
2. The macros of KaTeX go in `katexMacros` of `macro.go`. A relation that
   KaTeX draws with a glyph of its own fonts, such as `\nsubseteqq`, is
   `\not` over the relation. A relation of two characters, such as
   `\Eqcolon`, is one `\mathrel` of both. A macro of KaTeX can now take
   parameters, as `\Bra{#1}` does. `\varinjlim` and its kin are `\mathop`
   with `\limits`. `\TeX`, `\LaTeX` and `\KaTeX` are the macros of KaTeX,
   with `\raisebox`.
3. The ten operators of KaTeX for the names of Russian books, such as
   `\tg` and `\arcctg`, go in `namedOps`.
4. In text, the symbols of the text mode of KaTeX, such as `\ss`, `\OE` and
   `\textendash`, are characters of the run. An accent of text, such as
   `\'{a}`, puts the combining mark after the first character of its
   argument, and a shaper composes them. `\textcircled` of one letter or
   digit is the circled character of Unicode.
5. `\underrightarrow`, `\underleftarrow`, `\underleftrightarrow`,
   `\overrightharpoon`, `\overleftharpoon`, `\overlinesegment` and
   `\underlinesegment` are an `OverArrow`, with the new fields `Under`,
   `Harpoon` and `Segment`. They keep the proportions of the arrows of the
   source.
6. `\overbracket` and `\underbracket` are a `Brace` with the new field
   `Rune`, U+23B4 or U+23B5. `\overgroup`, `\undergroup`, `\utilde` and
   `\Overrightarrow` are the new node `Stretch`, which grows a character of
   the face with the horizontal variants and the assembly of the MATH table.
7. `\sideset{left}{right}` is the new node `Sideset`, laid out as amsmath
   does: the operator with its limits, and each side set as the scripts of
   the operator alone.
8. `\vcenter` is the new node `VCenter`, and `\angl` the new node `Angl`, as
   KaTeX lays them out. `\sout` is a `Cancel` with the new field `Strike`.
   `\smallint` is the integral that does not grow.
9. The environments `align`, `align*`, `alignat`, `alignat*` and `alignedat`
   are `aligned`. `gather` and `gather*` are `gathered`, and `equation` and
   `equation*` are `gathered` with one row, as KaTeX sets them. `rcases` is
   `cases` with the brace at the right. The column count of `alignat` is
   read and dropped.
10. `\above`, `\brace` and `\brack` are infix fractions, as `\over` and
    `\choose` are. The thickness of `\above` is read. A thickness of zero
    leaves the rule out, as for `\genfrac`, and another thickness gives the
    rule of the face.
11. `\xtwoheadrightarrow` and `\xtwoheadleftarrow` add a second head.
    `\xrightleftharpoons`, `\xleftrightharpoons`, `\xtofrom` and
    `\xlongequal` draw two shafts.
12. A box cannot hold a link, an attribute of HTML or an image. So `\href`,
    `\htmlId`, `\htmlClass`, `\htmlStyle` and `\htmlData` set their body
    alone. `\url` sets its text in the monospace family. `\includegraphics`
    is an empty space of the size of the image, from its options, as KaTeX
    gives it.
13. `\verb` and `\verb*` read the characters of the source between two
    delimiters, as text in the monospace family.
14. A `\\` or a `\newline` outside an environment makes a space of no width.
    `texmath` sets a formula on one line.
15. The parser reads five forms as KaTeX does, where the source gave an
    error: a backtick and a character of its own token after `\char`, such
    as `` \char`^ ``, `\operatorname x`, `\substack x`, a fourth argument of
    one token of `\mathchoice`, and a length in another unit than mu in
    `\mkern` and `\mskip`, such as `\mkern3pt`. The lines of these inputs in
    `texmath/testdata/gate.txt` now say `ok`.

## 4. The three changes of the layout

1. `\cfrac[l]` and `\cfrac[r]` set a numerator that is narrower than the
   denominator at its left or its right, as the `\hfill` of amsmath does.
   `Fraction` gets the field `NumAlign`. Another alignment than `l`, `c` or
   `r` is an error, as in KaTeX.
2. The first parameter of `\newcommand`, `\renewcommand` and
   `\providecommand` can have a default value, as
   `\newcommand{\f}[2][x]{#1+#2}`. A use takes the `[argument]` that follows
   it, or the default value.
3. `\hdashline` draws a dashed rule: dashes of 0.4 em with gaps of 0.4 em,
   the 4pt of `\dashlinedash` and `\dashlinegap` of the package arydshln at
   10pt. `Array` gets the field `HDashed`, which is nil without a dashed rule.

## 5. The width of the line

A new option, a functional option of D20, gives the width of the line:

```go
// WithLineWidth sets the width of the line of a formula, in the unit of
// the size of the layout.
func WithLineWidth(w float64) Option
```

With it, a formula with `\tag` is as wide as the line. The body is centered
in the line, and the label is at the right margin, as TeX sets a display
with `\eqno`. When the centered body comes closer than a quad to the label,
the body moves left. When the line is too short for the body, a quad and
the label, the layout is the one without the option. A width that is not
positive or not finite is the default, 0. A formula without `\tag` does not
change.

## 6. The gates and the snapshots

1. The parse gate passes with an empty `katex_parse.txt`. The layout gate
   passes with 61 more formulas in `katex_layout.txt`, the formulas that
   now parse and are outside the gate: 45 for the glyphs of STIX Two Math, 8
   for the arrows, 5 for the boxes that KaTeX measures with a strut, 2 for
   the layout and 1 for the new reason `stretch`. KaTeX draws
   `\Overrightarrow` and its kin with SVG images of its own.
2. 273 snapshots changed. Each one held an error of an unknown command or of
   one of the five forms of item 15 of section 3, and now holds a layout. No
   other snapshot changed. The snapshots give the same bytes on amd64, on
   arm64 and on s390x under QEMU, with Go on wasip1, and with TinyGo on the
   host and on wasip1.
3. The deepest chain of `\over` must lay out in the stack of 64 KiB of a
   goroutine of TinyGo on wasip1. A test on `\cfrac` in `layoutFraction`
   grew the stack of each level too much, so the alignment of `\cfrac` is in
   `fraction`, which is not on the stack of a nested fraction. The layouts of
   the new nodes are in `nestedMore`, out of the switch of `nested`.

## Why

The maintainers asked for the work. Each command follows KaTeX, the oracle
of D78, because Mermaid draws its labels with KaTeX. Where a box cannot hold
what KaTeX makes, such as a link or an image, the command keeps the part
that a box can hold, so that a formula of a label does not fail.
