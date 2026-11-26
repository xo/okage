# D103. texmath sets dots, primes, accent scripts and aligned cells as TeX

Status: Decided, amends D78.

On 2026-11-26 ivy asked for six changes to `texmath`, as item 4 of "Work
for okage" in its backlog. ivy worked around each one in
`latex/internal/mathsvg`. The maintainers asked for them on the same day.
Each change follows TeX where the source, go-tex/math, or the MATH table
does not. Each change has a comment that starts with `okage:` and names
this decision.

## 1. The dots

fontmath.ltx defines `\ldots` and `\mathellipsis` as
`\mathinner{\ldotp\ldotp\ldotp}`, and `\cdots` as
`\mathinner{\cdotp\cdotp\cdotp}`. `\ldotp` and `\cdotp` are of the class
Punct, which puts a thin space after each dot in the text and display
styles. The source set U+2026 and U+22EF, one glyph of the class Ord.

The parser now gives each of these commands an atom of the class Inner
that holds three dots of the class Punct. `\dotsc` and `\dotso` are the low
dots, and `\dotsb` and `\dotsm` are the dots at the center, as in amsmath.
`\dotsi` is `\!\cdots`. `\dots` sets the dots at the center before a binary
operator, a relation or a large operator, and the low dots in the other
cases, as amsmath does. In `\text`, these commands stay one glyph.

`1,\ldots,n` at 10 points is 33.22 points wide in pdfLaTeX. With Latin
Modern Math, it was 28.26 points wide, and it is now 33.23 points wide.
With STIX Two Math, it was 27.70 points wide, and it is now 31.18 points
wide.

## 2. The primes

TeX reads `x'` as `x^{\prime}`, and the prime of Computer Modern is a large
glyph that the superscript makes small and high. The prime U+2032 of a
Unicode math font is already small and high. The source set it in a
superscript, so it came out half the size of the prime of pdfLaTeX, and too
high.

The layout now sets the primes at the size of the style of the nucleus,
after the nucleus and its italic correction. A superscript and a subscript
attach to the primes, as to one character. `x'^2` is then `x\prime^2`. A
superscript that holds only `\prime`, such as `x^\prime`, is primes. The
primes of an operator with limits, and of a brace, stay in the superscript.

## 3. The scripts of an accented character

TeX Appendix G, rule 12, attaches the scripts of an accent over one
character to the character. The accent stays where it is over the
character alone. The source attached the scripts to the box of the accent,
so the subscript of `\hat y_i` took the drop of a box and went too low.

The layout now attaches the scripts and the primes to the character, as for
`y_i`. A group of one node, such as `{y}` in `\hat{y}_i`, is one character,
as in TeX. An accent over a formula, such as `\hat{xy}_i`, keeps the drop
of a box.

## 4. The shifts of a subscript

TeX has two shifts for a subscript: sub1 for a subscript alone, and sub2
for a subscript with a superscript. The MATH table has one,
`SubscriptShiftDown`, and Latin Modern Math gives it the value of sub2. So
`K_n` had its `n` 1 point lower than pdfLaTeX at 10 points.

Two options give a caller the shifts of TeX:

```go
// WithMathConstant sets the constant c of the MATH table to v, in the
// units of the font, in place of the value of the face.
func WithMathConstant(c font.MathConstant, v float32) Option

// WithSubSupShiftDown sets the least shift down of a subscript that has a
// superscript, in the units of the font: sub2 of TeX.
func WithSubSupShiftDown(v float32) Option
```

The face does not change. Without `WithSubSupShiftDown`, a subscript takes
`SubscriptShiftDown` in both cases, as before.

## 5. The drop of a subscript

The drop of a subscript under a nucleus that is not one character,
`SubscriptBaselineDropMin`, is now at the size of the script. sub_drop of
TeX comes from the font of the script size. The source took it at the size
of the nucleus. The drop of a superscript, `SuperscriptBaselineDropMax`,
stays at the size of the nucleus. ivy did not ask for it, and the change
put two formulas of the corpus outside the gate of KaTeX in
`test/texmath`.

## 6. The cells of aligned and split

The templates of `aligned`, `alignedat`, `split` and `gathered` of amsmath
start each cell with `\displaystyle`, and KaTeX sets them in display style.
The source set them in text style, so an integral or a fraction in a
`split` was small. The cells of these environments, and of `align`,
`alignat`, `gather` and `equation`, which take their layout, are now in
display style.

## Tests

`texmath/texshift_test.go` tests each change. The snapshots of
`texmath/testdata/snapshots` were written again, and 33 of 2150 changed.
Three formulas of the corpus came inside the gate of the height and the
depth of KaTeX, and `test/texmath/katex_layout.txt` no longer lists them.
