# D78. `texmath` lays out math, as a port of go-tex/math

Status: Decided, amends D5, D7, D9 and D13.

On 2026-11-20 a maintainer brought a request of the programs ivy and zuzu.
ivy will read and write LaTeX. It will copy the TeX engine
`github.com/go-tex/engine` into ivy and draw its pages with okage. The
engine sets math with `github.com/go-tex/math`, which returns SVG text and
three metrics. ivy also lays out the math of Office Math (DOCX), of MathML
(HTML and EPUB) and of OpenDocument formulas. zuzu draws Mermaid diagrams
with okage, and Mermaid draws a label between `$$` as KaTeX math. Both
programs want one math layout in okage. The TeX engine, LaTeX, MathML and
Office Math stay in ivy, and Mermaid stays in zuzu.

This decision gives the package, its API, the reader of the OpenType table
MATH, the bundled font, the limits, the oracles, the upstream record and the
phases of the port. It was question 32. On 2026-11-20 the maintainers chose
answer 1, a port into `texmath` at the root of the module, with these
choices:

1. `texmath` is a fork of go-tex/math. okage fixes the faults of the source
   in `texmath`, and does not report them to go-tex. Hard rule 6 of
   `AGENTS.md`, which ports a behavior of upstream with its faults, does not
   apply to `texmath`. Each fault that okage fixes is in the ledger of
   go-tex/math and in "Differences from upstream" in `README.md`.
2. okage follows the commits of go-tex/math with a ledger, and takes the
   fixes and the features that fit (phase 27). It does not try to give the
   bytes or the rounded metrics of the source.
3. `texmath` reads fonts with `typesetting`. okage takes go-opentype in no
   module, the test module included. So the comparison with the SVG text of
   the source is not an oracle, and okage has no switch that rounds as the
   source does. fontTools reads the table MATH for the tests of phase 21.

The section "The phases" is the plan of the port.

## What the source is

The checkout `upstream/go-tex-math` was read on 2026-11-20, with its tests,
its fonts and their licenses.

| Repository | Commit | License | Lines | What it holds |
| --- | --- | --- | --- | --- |
| [go-tex/math][gotexmath] | `6a6d2729`, the head of main on 2026-11-18. It is 11 commits after the tag `v0.50.0`, which go-tex/engine requires. The 11 commits change only the version of go-opentype, of Go and of the CI | BSD-3-Clause, copyright the go-tex/math authors 2026 | 4,609 in 12 source files, and 5,286 in 29 test files | a parser of a subset of TeX math, a layout with the MATH table of an OpenType font, and a writer of SVG text |

The source files, with their lines:

| File | Lines | What it holds |
| --- | --- | --- |
| `parse.go` | 2,132 | the tokenizer, `\ifmmode`, the parser of lists, atoms and about 87 commands, the environments, and the tables: about 414 symbols, 32 named operators, 14 accents and 12 environments |
| `math.go` | 1,458 | the styles, the atom classes and the spacing table of TeX Appendix G, the boxes, scripts, limits, fractions, radicals, delimiters that grow, accents, bars, arrows, the grid of matrices and the writer of SVG |
| `ooalign.go`, `mathchoice.go`, `mukern.go`, `rule.go`, `macro.go`, `raisebox.go`, `scalebox.go`, `leading.go`, `exat.go`, `font_embed.go` | 1,019 | `\ooalign`, `\mathchoice`, `\mkern`, `\rule`, the expansion of `\def`, `\raisebox`, `\scalebox`, the three models of the space between rows, the unit `\ex@` of amsmath, and the embedded font |

The other files:

1. `stixtwomath.otf`, STIX Two Math, 838,508 bytes, with a CFF outline, GSUB,
   GPOS and a MATH table of 27,408 bytes. `STIXTwoMath-OFL.txt` holds its
   license, the SIL Open Font License 1.1. A part of the font is copyright
   MicroPress with the Reserved Font Name "TM Math".
2. `testdata/nomath.otf`, Source Serif 4, 241,392 bytes, for the test of a
   font without a MATH table. The README says that it is under the OFL, but
   the repository holds no license file for it.
3. `cmd/wasm/main.go`, a worker for a browser. `sheet.svg` and `sheet.png`,
   a sheet of samples.

The tests hold 177 test functions. No test is a fuzz test, and no test
compares a golden file. The tests read the SVG text and the sizes of boxes,
and many comments quote sizes that the authors measured with tectonic, a TeX
engine. The CI holds the coverage of statements at 100 percent.

### What the source calls in go-opentype

Every call of `github.com/go-opentype/opentype` is in `math.go`, 54
references in all:

1. `Parse`, `Font.HasMath`, `Font.UnitsPerEm`, `Font.GlyphIndex` and
   `Font.NewFace`, with a size in whole pixels.
2. `Face.MathConstant`, for 27 of the 56 constants of the table.
3. `Face.MathVariants`, for the vertical size variants and the glyph
   assembly of a glyph.
4. `Face.AdvanceIndex`, `Face.GlyphOutline` and `Face.GlyphSVGPath`.

### What typesetting gives for each call

| go-opentype | typesetting |
| --- | --- |
| `Parse`, `NewFace` | `font.ParseTTF`, or `font.NewFont` and `font.NewFace` |
| `UnitsPerEm` | `Font.Upem` |
| `GlyphIndex` | `Face.NominalGlyph` |
| `AdvanceIndex` | `Face.HorizontalAdvance`, in font units, with the variations of the face |
| the ink of `GlyphOutline` | `Face.GlyphExtents`, in font units |
| `GlyphSVGPath`, `GlyphOutline` | `Face.GlyphDataOutline`, with quadratic segments for `glyf` and cubic segments for CFF and CFF2 |
| `HasMath`, `MathConstant`, `MathVariants` | none. The copy of go-text reads no MATH table. Neither go-text nor its `harfbuzz` package ports `hb-ot-math` of HarfBuzz |

## What makes the port harder than the request says

The request describes the source as a parser and a layout on the MATH
table. The checkout shows eight more facts:

1. The parser lays out as it reads. `parseList`, `parseAtom` and
   `parseControl` return laid-out boxes, and no tree of nodes exists. A tree
   that ivy can build from MathML and Office Math needs each command of
   `parseControl` split into a part that parses and a part that lays out.
   That is more restructure than hard rule 5 allows, so it is a deliberate
   difference of this decision.
2. A box holds SVG text. `place` writes a `<g transform="translate">`, and
   `\scalebox` and `\reflectbox` write a `scale` that can mirror a box or
   scale it in one direction only. The extensible arrows, such as
   `\xrightarrow`, and the arrows over a base, such as `\overrightarrow`,
   draw their heads as triangles in SVG path data, and the hook of
   `\xhookrightarrow` as a stroked SVG arc. So the port needs a third kind of
   item, a path, beside a glyph and a rule, and each item needs a transform.
3. The source uses whole pixels. It asks go-opentype for a face at an
   integer size. `MathConstant`, `MathVariants` and `AdvanceIndex` round each
   value to a whole pixel, and the size of a script is `px * pct / 100`,
   truncated. go-opentype documents the rounding as a fault, and
   go-tex/engine rounds `\Large`, 14.4 points, to 14 because of it. typesetting
   gives exact font units, so the port cannot give the bytes of the SVG of
   the source.
4. go-opentype stores a CFF outline as lines, and the source takes the ink
   of a glyph from those lines. typesetting takes the bounds of the curves.
   The height and the depth of a glyph differ by a small amount.
5. The source reads only the constants and the variants of the MATH table.
   It does not use the italic correction, the top accent attachment, the
   math kerning, the extended shapes or the italic correction of an
   assembly. An accent is centered on the advance of its base, and a script
   does not move by the italic correction. Appendix G is approximated: a
   superscript uses `SuperscriptShiftUp` and `SuperscriptBaselineDropMax`
   only, and a fraction and a limit use only the minimum gaps. The full
   reader that the request asks for is more than the source uses, and a
   layout that uses it is new code.
6. `\text` switches the alphabet of the math face to upright. It does not
   use a text face. The tokenizer drops every space, also inside `\text`, so
   `\text{if } b` sets "ifb". Mermaid labels use `\text` with spaces.
7. The source has no `\color`, no `\textcolor` and no `\textbf`. It draws
   `\overbrace` and `\underbrace` as a plain rule, without a brace. It does
   not stretch `\widehat` and `\widetilde`, and it sets `\middle` at one fixed
   size.
8. The source has no limit on the depth of nesting, so the recursion of the
   parser can grow the stack without a bound. Its budget of 100,000 macro
   expansions does not limit the number of tokens, so `\def\a{\a\a}\a` grows
   the input to millions of tokens. `strconv.ParseFloat` reads `NaN` and
   `Inf` in `\scalebox`, `\rule` and `\mkern`, and the values reach the
   output. A character without a glyph gives an empty box and no error.

Two more facts matter for the plan. go-tex/engine embeds Latin Modern Math,
not STIX Two Math, so the look of LaTeX in ivy comes from Latin Modern Math.
The README of the source is older than its code: it says that the source
has no assembly and no user macros, but `math.go` builds assemblies and
`macro.go` expands `\def`.

## The decision

### 1. The package and its place

1. The port is the package `texmath`, at `github.com/xo/okage/texmath`, a
   public package beside `pdf` and `lottie`. It amends the table of D5:

   | Path | Package | What it holds |
   | --- | --- | --- |
   | `texmath` | `texmath` | the port of `github.com/go-tex/math`: the tree of a formula, the parser of TeX math, the layout and the items |
   | `texmath/stixtwomath` | `stixtwomath` | the font STIX Two Math, embedded, with its license |

2. The name follows the source. The package of the source is named `math`,
   and its own README imports it as `texmath`. The name `math` collides with
   the standard library, so the package takes the name that its module gives
   it, as D10 does for a Rust name that collides.
3. The package is not under `typesetting/`. That folder is the copy of
   go-text (D52). Its ledger applies the diffs of go-text to it, and the
   linters of style do not run on it. A port of another repository in it
   mixes two ledgers in one folder.
4. `texmath` imports `typesetting/font`, the root package for `Pixmap`,
   `Paint`, `Transform` and `Path`, and the standard library. It does not
   import `svg`, so a program that lays out math does not link the SVG
   parser or the `FontDatabase`. On 2026-11-20, phase 26 added the imports
   of `typesetting/shaping`, `typesetting/di`, `typesetting/language` and
   `golang.org/x/image/math/fixed`, which the root module already requires,
   to shape the runs of a text face.

### 2. The reader of the MATH table

1. typesetting reads the MATH table, version 1.0 of the OpenType
   specification. The reader is an addition to the copy of go-text, so D52
   applies: each file or change has a comment that starts with `okage:` and
   names this decision.
2. `typesetting/font/opentype/tables/okage_math.go` holds the parser. okage
   does not run the generator binarygen of go-text (D52), so the parser is
   written by hand, in the form of the generated `Parse` functions. It reads
   `MathConstants`, `MathGlyphInfo` with `MathItalicsCorrectionInfo`,
   `MathTopAccentAttachment`, the coverage of extended shapes and
   `MathKernInfo`, and `MathVariants` with the minimum connector overlap,
   the vertical and the horizontal constructions, the size variants and the
   glyph assemblies. A `MathValueRecord` keeps the offset of its device
   table. The file opens with the comment
   `okage: this file is not in go-text (D78).`, as `font/cff/okage.go` does
   for D59.
3. `typesetting/font/okage_math.go` holds the API on the font. It follows
   the API `hb-ot-math` of HarfBuzz, which the `harfbuzz` package of go-text
   ports in other parts. Every length is in font units, as the rest of
   `font`:

   ```go
   // MathConstant names one value of the table MathConstants, such as
   // AxisHeight, in the order of the OpenType specification.
   type MathConstant uint8

   // HasMath reports whether the font has a MATH table that parsed.
   func (f *Font) HasMath() bool

   // MathConstant returns the value c in font units, with the variation
   // deltas at the coordinates of the face. The three percentages,
   // ScriptPercentScaleDown, ScriptScriptPercentScaleDown and
   // RadicalDegreeBottomRaisePercent, are returned as they are.
   func (f *Face) MathConstant(c MathConstant) float32

   // MathItalicsCorrection returns the italic correction of g, or 0.
   func (f *Face) MathItalicsCorrection(g GID) float32

   // MathTopAccentAttachment returns the horizontal position over g where a
   // top accent is centered. It returns false when g has none.
   func (f *Face) MathTopAccentAttachment(g GID) (float32, bool)

   // IsMathExtendedShape reports whether g is an extended shape.
   func (f *Font) IsMathExtendedShape(g GID) bool

   // MathKernCorner is one of the four corners of a glyph.
   type MathKernCorner uint8

   const (
   	MathKernTopRight MathKernCorner = iota
   	MathKernTopLeft
   	MathKernBottomRight
   	MathKernBottomLeft
   )

   // MathKern returns the kern of g at corner for a script at height.
   func (f *Face) MathKern(g GID, corner MathKernCorner, height float32) float32

   // MathMinConnectorOverlap returns the minimum overlap of two parts of an
   // assembly.
   func (f *Face) MathMinConnectorOverlap() float32

   // MathGlyphVariant is one size variant of a glyph.
   type MathGlyphVariant struct {
   	Glyph   GID
   	Advance float32 // the height for a vertical variant, the width for a horizontal one
   }

   // MathGlyphVariants returns the size variants of g, smallest first, in
   // the vertical or in the horizontal direction.
   func (f *Face) MathGlyphVariants(g GID, vertical bool) []MathGlyphVariant

   // MathGlyphPart is one part of a glyph assembly.
   type MathGlyphPart struct {
   	Glyph                GID
   	StartConnectorLength float32
   	EndConnectorLength   float32
   	FullAdvance          float32
   	Extender             bool
   }

   // MathGlyphAssembly is the recipe that builds a glyph of any size.
   type MathGlyphAssembly struct {
   	ItalicsCorrection float32
   	Parts             []MathGlyphPart // bottom to top, or left to right
   }

   // MathGlyphAssembly returns the assembly of g in a direction. It returns
   // false when g has none.
   func (f *Face) MathGlyphAssembly(g GID, vertical bool) (MathGlyphAssembly, bool)
   ```

4. `NewFont` in `font/font.go` loads the table, as it loads GDEF, with an
   `okage:` comment. The one change to a file of go-text is that load and the
   field that holds the table. The limits of D61 apply to the table.
5. A malformed MATH table is an error of `tables.ParseMATH`, never a panic.
   `NewFont` then leaves the table out, as it does for another optional
   table, and `HasMath` returns false. Every offset, count and coverage index
   is compared with the length of the table.
6. A device table is read for its variation index only. The deltas come from
   the item variation store of GDEF at the coordinates of the face, as for a
   value record of GPOS. A device table for a size in pixels is ignored,
   because okage does not hint.
7. The tests read the MATH table of STIX Two Math. A new fuzz target,
   `FuzzParseMATH` in `tables`, gives random bytes to the parser, with the
   table of STIX Two Math as its seed. A second fuzz target, `FuzzMATHFace`,
   loads a font whose MATH table the fuzzer changes and calls each method of
   the API with random glyphs. Neither one panics or hangs.
8. If go-text later reads the MATH table, okage applies that commit,
   removes its own reader, and keeps the API names of go-text, as "A
   difference from upstream" in [`../UPSTREAM.md`](../UPSTREAM.md) says.

### 3. The API of `texmath`

The API has three parts: a tree of nodes, a parser of TeX math that makes a
tree, and a layout that makes a `Box` of items. ivy builds the tree from
MathML, Office Math and OpenDocument formulas without the parser. zuzu
calls `LayoutTeX`.

```go
// Package texmath lays out mathematical formulas with the OpenType MATH
// table of a font, by the rules of TeX Appendix G. It is the port of
// github.com/go-tex/math (D78).
package texmath

// Class is the class of an atom. It sets the space between two atoms, by
// the table of TeX Appendix G.
type Class uint8

const (
	Ord Class = iota
	Op
	Bin
	Rel
	Open
	Close
	Punct
	Inner
)

// Style is one of the four styles of TeX. The source has no cramped style,
// and the port has none either.
type Style uint8

const (
	Display Style = iota
	Text
	Script
	ScriptScript
)

// Node is one node of a formula. Each type below is a Node.
type Node interface{ node() }

// List is a row of nodes on one baseline. The empty List is valid.
type List []Node

// Atom is a nucleus with a class. Text holds its characters, which the
// layout maps through Variant to the glyphs of the math face, as one unit
// with no space inside, such as "sin". Body holds a nucleus that is a
// formula, such as {a+b}, when Text is empty.
type Atom struct {
	Class   Class
	Text    string
	Variant Variant
	Body    Node
	Large   bool   // an operator that grows in display style, such as \sum
	Limits  Limits // where the scripts of a large operator go
}

// Variant is a math alphabet: the block of Mathematical Alphanumeric
// Symbols that a Latin letter or a digit maps to.
type Variant uint8

const (
	Italic Variant = iota // the default of a letter
	Upright               // \mathrm, and an operator name
	Bold
	BoldItalic
	DoubleStruck
	Calligraphic // \mathcal and \mathscr
	Fraktur
	SansSerif
	SansSerifItalic
	SansSerifBoldItalic
	Monospace
)

// Limits says where the scripts of a large operator go.
type Limits uint8

const (
	LimitsAuto  Limits = iota // above and below in display style only
	LimitsAbove               // \limits
	LimitsSide                // \nolimits
)

// Scripts attaches a superscript, a subscript and primes to Base.
type Scripts struct {
	Base     Node
	Sup, Sub Node // nil when absent
	Primes   int
}

// Fraction is \frac, \over, \atop, \binom and \genfrac. \dfrac and \tfrac
// are a StyleChange around a Fraction.
type Fraction struct {
	Num, Den    Node
	NoRule      bool // \atop and \binom
	Open, Close rune // the delimiters of \binom and \genfrac, 0 for none
}

// Radical is \sqrt, with an optional degree.
type Radical struct {
	Body   Node
	Degree Node // nil for a square root
}

// Delimited is \left and \right: Body between two delimiters that grow to
// its height and depth around the math axis.
type Delimited struct {
	Open, Close rune // 0 for the null delimiter "."
	Body        Node
}

// Delimiter is one delimiter of a fixed size, such as \big( or \middle|.
type Delimiter struct {
	Rune  rune
	Class Class
	Size  float64 // the height in em, 0 for the natural size
}

// Accent sets an accent over Base, such as \hat. Stretch grows it to the
// width of Base with the horizontal variants, as \widehat.
type Accent struct {
	Base    Node
	Accent  rune
	Stretch bool
}

// OverUnder sets Over above and Under below Base, centered: \overset,
// \underset, \stackrel and \buildrel, and the munder and mover of MathML.
type OverUnder struct {
	Base        Node
	Over, Under Node // nil when absent
	Class       Class
}

// Bar is \overline or \underline.
type Bar struct {
	Base  Node
	Under bool
}

// Array is a grid of cells: the matrix environments, cases, array, aligned,
// gathered, smallmatrix and \substack, and the mtable of MathML.
type Array struct {
	Kind        ArrayKind
	Rows        [][]Node
	Align       []Align // per column, centered when short
	VRules      []int   // the column gaps, 0 to the column count, with a vertical rule
	Open, Close rune    // the delimiters of pmatrix and its kin, 0 for none
	Stretch     float64 // \arraystretch, 0 for 1
}

// Space is a horizontal space, such as \quad or \,, or a negative one.
type Space struct{ Width Length }

// Length is a TeX length. Pt and the other absolute units count one point
// as one unit of the size of the layout, as the source does.
type Length struct {
	Value float64
	Unit  Unit // Em, Ex, Mu or Pt
}

// StyleChange sets Body in Style, as \displaystyle or \scriptstyle.
type StyleChange struct {
	Style Style
	Body  Node
}

// TextRun is text in a text face, as \text and the mtext of MathML. Its
// spaces are kept.
type TextRun struct {
	Text         string
	Bold, Italic bool
}

// Color draws Body in a color, as \color and \textcolor.
type Color struct {
	Color okage.Color
	Body  Node
}

// The other nodes port the rest of the source: Rule (\rule), Phantom
// (\phantom, \hphantom, \vphantom, \mathstrut), Raise (\raisebox), Scale
// (\scalebox, \reflectbox), Boxed (\boxed), Negate (\not), Brace
// (\overbrace, \underbrace), OverArrow (\overrightarrow and its kin),
// XArrow (\xrightarrow and its kin), Lap (\lefteqn), Overlay (\ooalign)
// and Choice (\mathchoice).

// Parse reads TeX math, such as `\frac{a}{b}`, and returns its tree. An
// unknown command is an error that names it. Parse lays nothing out, so it
// needs no font.
func Parse(tex string, opts ...Option) (List, error)

// Layout lays out n with face, which must have a MATH table, at size, in
// style. The size is in any unit, such as pixels for zuzu or points for ivy.
// Every length of the result is in that unit. Layout reads face, which is
// not safe for concurrent use, so a caller gives each goroutine its own
// face.
func Layout(n Node, face *font.Face, size float64, style Style, opts ...Option) (*Box, error)

// LayoutTeX parses tex and lays out its tree, for a caller that has TeX
// only.
func LayoutTeX(tex string, face *font.Face, size float64, style Style, opts ...Option) (*Box, error)

// WithTextFace sets the face of TextRun and \text. Without it, a TextRun
// uses the upright glyphs of the math face, as the source does.
func WithTextFace(regular, bold, italic, boldItalic *font.Face) Option

// WithColor sets the color of each item outside a Color node. The default
// is opaque black.
func WithColor(c okage.Color) Option

// WithMaxDepth, WithMaxNodes and WithMaxExpansions change the limits of
// section 5.
func WithMaxDepth(n int) Option
func WithMaxNodes(n int) Option
func WithMaxExpansions(n int) Option

// WithLogger logs each character that has no glyph in its face (D20).
func WithLogger(l *slog.Logger) Option

// Box is a formula laid out. Its origin is the left end of its baseline,
// and y grows down, as in okage.
type Box struct {
	Width  float64 // the advance width
	Height float64 // the extent above the baseline
	Depth  float64 // the extent below the baseline
	Italic float64 // the italic correction of the last glyph, from the MATH table
	Items  []Item
}

// ItemKind is the kind of an Item.
type ItemKind uint8

const (
	ItemGlyph ItemKind = iota // one glyph of a face
	ItemRule                  // a filled rectangle
	ItemPath                  // a filled path, such as the head of an arrow
)

// Item is one thing that a Box draws.
type Item struct {
	Kind      ItemKind
	Face      *font.Face      // ItemGlyph
	Glyph     font.GID        // ItemGlyph
	Size      float64         // ItemGlyph: the em size of the glyph
	X, Y      float64         // the origin of a glyph or a path, or the top left corner of a rule
	W, H      float64         // ItemRule
	Path      *okage.Path     // ItemPath, relative to X and Y
	Text      string          // ItemGlyph: the characters of the cluster, for a text layer, on its first glyph
	Color     okage.Color
	Transform okage.Transform // applied after X and Y. The identity, except under Scale
}

// Draw fills each item into p, moved by t. paint gives the anti-aliasing
// and the blend mode, and the color of each item replaces its shader.
func (b *Box) Draw(p *okage.Pixmap, t okage.Transform, paint *okage.Paint)

// Path returns the outline of every item as one path, in the units of the
// size, with the baseline at y = 0. It reads the outlines from the faces of
// the items, and needs no FontDatabase. It returns nil for an empty box.
func (b *Box) Path() *okage.Path

// ItemPath returns the outline of item i, for a caller that colors each
// item, such as an SVG writer for a formula with \color.
func (b *Box) ItemPath(i int) *okage.Path
```

These rules go with the API:

1. The three ways out are `Draw` for okage, `Path` and `ItemPath` for an
   SVG writer such as the one of zuzu, and `Items` for the display list and
   the PDF writer of ivy. ivy gives a glyph item to `pdf.Surface.DrawGlyphs`
   with the face and the text of the item, so the PDF text can be copied.
2. The tree has no position of the source in it. An error of `Parse` gives
   the position of the fault in `tex`.
3. The layout ports the layout half of each command of the source with the
   rules of the source. The use of the italic correction, the top accent
   attachment and the math kerning is an addition of phase 26, a deliberate
   difference that a test can turn off for the comparison with the source.
4. Each glyph of an assembly is one item. The first item of an assembly or
   of a cluster carries the text, and the other items carry none.
5. `Parse` keeps `\def` as the source does. The macros of a document are
   the work of the TeX engine in ivy, and of KaTeX options in zuzu.

### 4. The bundled font

1. `texmath/stixtwomath` embeds `stixtwomath.otf` byte for byte, with
   `STIXTwoMath-OFL.txt` beside it as `OFL.txt`. It gives the bytes, as
   `DefaultFont` of the source does, and the font parsed once:

   ```go
   // Data returns the bytes of STIX Two Math.
   func Data() []byte

   // Font returns STIX Two Math, parsed once. A *font.Font is safe for
   // concurrent use, and each caller makes its faces with font.NewFace.
   func Font() (*font.Font, error)
   ```

2. A program that does not import `texmath/stixtwomath` does not carry the
   font. `texmath` does not import it, and a test of `texmath` reads it.
3. STIX Two Math adds 819 KiB to a binary. The font is not changed, so the
   Reserved Font Name of the OFL does not apply. A subset that `pdf` makes
   for a PDF file is an embedding in a document, which the OFL allows.
4. Latin Modern Math also fits. It has a MATH table, its license is the
   GUST Font License, which allows a copy that is not changed, and it is
   about 0.7 MB. The size was not measured, because the download was not
   approved. It is the font of go-tex/engine, and its glyphs are closer to
   the KaTeX fonts of Mermaid. This decision bundles STIX Two Math only,
   because the source and its tests use it. A package
   `texmath/latinmodernmath` is a later decision, when ivy or zuzu asks for
   it. Any other font with a MATH table works with `Layout`, such as XITS
   Math, Libertinus Math or Fira Math, which are under the OFL.
5. The test of a font without a MATH table reads a font that okage already
   holds, with its license, not `testdata/nomath.otf`, which has no license
   file.

### 5. Limits and errors

The root package declares these sentinel errors (D21):

| Error | Message | When |
| --- | --- | --- |
| `ErrTeXMathSyntax` | `parsing tex math` | the source returns an error, such as a missing `}`, a `\left` without `\right`, or `\begin{x}` closed by `\end{y}` |
| `ErrTeXMathUnknownCommand` | `parsing tex math: unknown command` | an unknown command or environment |
| `ErrMathLimit` | `typesetting math: limit reached` | a limit below |
| `ErrMathNoMathTable` | `laying out math: the face has no MATH table` | `Layout` gets a face without a MATH table, as `New` of the source |

1. An error of `Parse` is a `*texmath.SyntaxError`, as `*html.SyntaxError`
   wraps `okage.ErrHTMLSyntax`. It holds the position in `tex`, the message
   of the source, and the name of an unknown command in the field
   `Command`. Its text names the command, such as
   `parsing tex math: unknown command \foo`. go-tex/engine reads the name
   from the text of the error. The engine in ivy can read `Command` with
   `errors.As` instead.
2. The limits are additions, each a deliberate difference:
   - The depth of nesting of groups, arguments and environments is at most
     100. The stack of a program on WebAssembly is small (D25). The parser
     counts each step of its recursion as a level, so a plain run of nested
     groups stops at about 50. On 2026-11-20 the maintainers accepted that
     limit as enough for `texmath`.
   - A tree has at most 100,000 nodes, and a box at most 1,000,000 items.
   - The parser keeps the budget of 100,000 macro expansions of the source,
     and the expanded input has at most 1,048,576 tokens.
   - An assembly repeats its extenders at most 64 times, as the source
     does.
   - A length of `\rule`, `\raisebox`, `\mkern` or `\genfrac` and a factor of
     `\scalebox` are finite. A length is at most 16,384 em, about the
     `\maxdimen` of TeX, and a factor is at most 100.
3. Each limit is an error that wraps `ErrMathLimit`. No input makes
   `Parse` or `Layout` panic or loop without an end.
4. Two fuzz targets hold this. `FuzzParse` gives random text to `Parse`, and
   `FuzzLayout` gives random text to `LayoutTeX` with STIX Two Math in each
   style and at random sizes. Their seeds are the inputs of the tests of the
   source and the corpus of section 7. `FuzzMATHFace` of section 2 covers a
   malformed font.

### 6. The arithmetic

1. The layout computes in `float64`, as the source does, from exact font
   units scaled by the size. It does not round a metric to a whole pixel, and
   the size is a `float64`. This is a deliberate difference, because the
   rounding of the source is a fault that go-opentype documents, and
   because ivy sets math at sizes such as 14.4 points. okage has no switch
   that rounds as the source does.
2. D16 and D71 apply. Where the source multiplies and adds, the port writes
   `float64(x*y) + z`. The snapshots of section 7 must be the same on
   amd64, arm64, s390x, wasip1 and TinyGo.

### 7. The oracles and the tests

The test module holds the comparisons (D22):

1. The tests of the source are ported with their files. A test that reads
   the SVG text reads the items of the `Box` instead. A test that checks a
   rounded metric of the source checks the exact metric of okage, with the
   expected value computed again from the font, and says so in a comment.
2. Snapshots: for each formula of the corpus, a text file in
   `texmath/testdata/snapshots` lists the width, the height, the depth and
   each item, with each float written by `strconv.FormatFloat(v, 'g', -1,
   64)`. A changed byte fails the test, so a fused multiply and add on a
   platform shows. The corpus is the inputs of the tests of the source, the
   examples of the documentation of Mermaid, and the list of the functions
   that KaTeX supports.
3. fontTools as an oracle for the table MATH: a script in the test module
   reads each value of the table MATH of STIX Two Math and of the other
   math fonts of the tests with fontTools, and the reader of section 2 must
   give the same value. The source itself is not an oracle, because it
   needs go-opentype, which okage does not take.
4. KaTeX as an oracle: Node runs KaTeX, which is under the MIT license, at
   the version that Mermaid 12.3 bundles, from `test/texmath/katex`, with its
   `package.json` and its lock file. KaTeX in Node does not draw. It gives
   HTML, a parse error, and the height and the depth of a formula in em, but
   not its width, because a browser sets the width. So the gates are these:
   - Each formula of the corpus that KaTeX parses, `Parse` parses too,
     except a formula in a list in the test, with the missing command and
     the reason.
   - The height and the depth of each formula are within 15 percent of the
     values of KaTeX. The fonts of KaTeX are not STIX Two Math, so a closer
     gate measures the fonts and not the layout.
   - When a headless Chromium is on the machine, the test draws the HTML of
     KaTeX and the box of okage, and writes both images to a report for a
     person to look at. The report is not a gate.

   A test skips when `node`, the installed packages or Chromium is missing.
   The packages of npm are a download that a maintainer approves when phase
   20 starts.
5. The nightly jobs on arm64, s390x, wasip1 and TinyGo run the snapshots
   and the fuzz seeds (D24, D25). `texmath` and `texmath/stixtwomath` build
   with TinyGo for the host and for `wasip1`.

### 8. What a TeX engine needs from a font of okage

go-tex/engine reads its text fonts with go-opentype: `NewFaceAt`, `Scale`,
`UnitsPerEm`, `GlyphIndex`, `AdvanceUnits`, `AdvanceIndexUnits`,
`KernUnits`, `KerningIsContextual`, `Shape` with the feature `liga`,
`GlyphOutline` and `GlyphSVGPath`. typesetting already gives nearly all of
this, in font units, at any size:

| The engine needs | typesetting gives |
| --- | --- |
| the advance of a glyph | `Face.HorizontalAdvance` |
| the height and the depth of a glyph | `Face.GlyphExtents`: the height is `YBearing`, and the depth is `-(YBearing + Height)` |
| the italic correction of a glyph | `Face.MathItalicsCorrection` of section 2 for a math face. A text face has no such value, and the engine chooses its rule, as it does with go-opentype |
| the units per em, and a glyph for a character | `Font.Upem`, `Face.NominalGlyph` |
| the outline of a glyph | `Face.GlyphDataOutline` |
| the kern of a pair of glyphs | the pieces exist but no single call: `Font.GPOS` with `Layout.FindScript`, `FindFeatureIndex`, `tables.PairPos` and `PairSet.FindGlyph`, and `Font.Kern` with `KernPair` for the table `kern` |
| the ligature of a pair of glyphs | the pieces exist but no single call: `Font.GSUB` with `tables.LigatureSubs`. `shaping.HarfbuzzShaper` with the feature `liga` gives it for two glyphs, as `Shape` of go-opentype does |
| a contextual kern | `shaping.HarfbuzzShaper`, which applies GPOS in full |

The shaper fits every row, so no helper is required. Two small helpers are
proposed only if ivy measures that a shaping run for each pair costs too
much:

```go
// KernPair returns the kern of the pair left, right in font units, from the
// pair adjustments of the feature kern of GPOS for the script DFLT or
// latn, or else from the table kern. It applies no contextual lookup.
func (f *Face) KernPair(left, right GID) float32

// LigaturePair returns the ligature of the pair left, right from the
// feature liga of GSUB. It returns false when the pair has none.
func (f *Face) LigaturePair(left, right GID) (GID, bool)
```

They go in `typesetting/font/okage_pair.go`, marked as section 2 says.

### 9. The upstream record

1. `docs/upstream/upstreams.tsv` gets this row when phase 20 starts:

   | Column | Value |
   | --- | --- |
   | `repository` | `go-tex-math` |
   | `url` | `https://github.com/go-tex/math` |
   | `checkout` | `upstream/go-tex-math` |
   | `crates` | `github.com/go-tex/math v0.50.0+11` |
   | `okage` | `texmath, texmath/stixtwomath, typesetting/font` |
   | `policy` | `port` |
   | `base` | `6a6d2729f3b41117158da5b67fbaf3dbce2d84aa` |
   | `matched` | empty until the base port is done |

2. `docs/upstream/go-tex-math.tsv` is its ledger. On 2026-11-20 the base
   commit is the head of main, so the ledger holds no commit.
3. The policy is `port`, not `copy`. The source is Go code, but the port
   splits its parser from its layout and moves it to the fonts of
   typesetting, so a diff of the source does not apply as it is.
4. The code keeps the license of the source (D13). `texmath/LICENSE` holds
   the BSD-3-Clause license with the copyright line of go-tex/math and the
   copyright line of okage. Each file keeps the two header lines of the
   source, with its SPDX identifier. `texmath/stixtwomath/OFL.txt` holds the
   OFL of the font, byte for byte. The additions to `typesetting` keep the
   license of typesetting. This amends the table of D13.
5. The upstream paths go here, in a new section of `UPSTREAM.md`:

   | Upstream path | okage path | Rule |
   | --- | --- | --- |
   | `<name>.go` | `texmath/<name>.go`. A file keeps its name. The parse half of a command stays in the file of the source, and its layout half goes in the same file, in a function whose doc comment names the source function | Port |
   | `<name>_test.go` | `texmath/<name>_test.go` | Port. A test of SVG text becomes a test of items |
   | `stixtwomath.otf`, `STIXTwoMath-OFL.txt`, `font_embed.go` | `texmath/stixtwomath/` | Port, byte for byte for the font and the license |
   | `testdata/nomath.otf` | none | Not applicable. A font of okage replaces it |
   | `go.mod`, `go.sum` | none | Not applicable. okage reads fonts with `typesetting`, not go-opentype |
   | `cmd/**`, `sheet.*`, `README.md`, `.github/**`, `renovate.json` | none | Not applicable |

   The files of okage that have no source file are `texmath/node.go` for
   the tree, `texmath/layout.go` for the walk of the tree, and
   `texmath/item.go` for `Box`, `Item`, `Draw` and `Path`.
6. This amends D7: the table of D7 gets the row `go-tex/math`, used by ivy
   and zuzu, with the policy port and the Go form `texmath`. It amends D9:
   okage now lays out math, which no crate of usvg does, on `typesetting`,
   as it lays out the text of usvg.

### 10. The KaTeX commands that Mermaid users write

Mermaid calls KaTeX with `displayMode: true`. The examples of the
documentation of Mermaid and the usual labels use these commands. The
source covers them as follows:

| Coverage | Commands |
| --- | --- |
| covered | `^`, `_`, primes, `\frac`, `\dfrac`, `\tfrac`, `\sqrt`, `\sqrt[n]`, the Greek letters, `\sum`, `\prod`, `\int`, `\oint`, `\lim` with limits, `\left`, `\right`, `\cdot`, `\times`, `\pm`, `\le`, `\ge`, `\ne`, `\approx`, `\equiv`, `\in`, `\subset`, `\to`, `\rightarrow`, `\Rightarrow`, `\iff`, `\mapsto`, `\infty`, `\partial`, `\nabla`, `\hat`, `\bar`, `\vec`, `\tilde`, `\dot`, `\ddot`, `\overline`, `\underline`, `\mathbb`, `\mathcal`, `\mathbf`, `\mathrm`, `\mathit`, `\mathsf`, `\mathtt`, `\mathfrak`, `\operatorname`, `\sin`, `\cos`, `\log`, `\exp`, the environments `matrix`, `pmatrix`, `bmatrix`, `Bmatrix`, `vmatrix`, `Vmatrix`, `cases`, `array`, `aligned`, `gathered` and `smallmatrix`, `\binom`, `\overset`, `\underset`, `\stackrel`, `\xrightarrow`, `\boxed`, `\quad`, `\qquad`, `\,`, `\;`, `\!`, `\displaystyle`, `\ldots`, `\cdots`, `\vdots`, `\ddots`, `\not`, `\phantom`, `\substack`, `\pmod`, `\def` |
| approximated | `\text`, which loses its spaces, `\overbrace` and `\underbrace`, drawn as a rule, `\widehat` and `\widetilde`, not stretched, `\middle`, at one size, `\big` and its kin, at fixed multiples of the em, and `\smash`, which keeps the height |
| missing | `\color`, `\textcolor`, `\colorbox`, `\fcolorbox`, `\fbox`, `\textbf`, `\textit`, `\textsf`, `\texttt`, `\emph`, `\cancel`, `\bcancel`, `\xcancel`, `\tag`, `\hline` in an array, `\cfrac`, `\scriptscriptstyle`, `\kern`, `\hspace`, `\llap`, `\rlap`, `\clap`, `\underrightarrow`, `\dddot`, `\sideset`, `\newcommand`, the environments `align`, `equation` and `gather`, and the macros of KaTeX such as `\R` and `\N` |

Phase 26 adds `\color`, `\textcolor`, the text face of `\text` with its
spaces, `\textbf`, `\textit`, `\textsf` and `\texttt`, and `\overbrace` and
`\underbrace` with the horizontal assembly of the brace. The other missing
commands wait for a request, each with a test in the KaTeX oracle.

On 2026-11-20, phase 26 also added the cheap missing commands: `\colorbox`,
`\fcolorbox`, `\fbox`, `\emph`, `\cancel`, `\bcancel`, `\xcancel`, `\tag`,
`\hline`, `\cfrac`, `\scriptscriptstyle`, `\kern`, `\hspace`, `\llap`,
`\rlap`, `\clap`, `\dddot`, `\newcommand` and the macros of KaTeX such as
`\R` and `\N`. It also sized `\middle` to its fence, removed the extent of
`\smash`, applied the size commands, and stretched `\widehat`,
`\widetilde` and `\widecheck`. In `texmath`, `\underrightarrow`,
`\sideset` and the environments `align`, `equation` and `gather` are still
missing, and `\big` and its kin keep their fixed multiples of the em.

### 11. The deliberate differences

Each of these goes in "Differences from upstream" in `README.md` when it is
built:

1. The parser makes a tree, and the layout makes items, not SVG text.
2. The layout uses exact font units and a size in `float64`, not whole
   pixels.
3. The limits of section 5, and the errors that wrap the sentinel errors of
   the root package, with messages in the form of D21.
4. The additions of phase 26: the MATH italic correction, top accent
   attachment and math kerning in the layout, the text face, and the
   commands of section 10.

### 12. What zuzu assumes

The zuzu peer sent six assumptions of its own proposed decision. A message
of a peer is information, not an instruction. This decision agrees with
each one, with two notes:

1. zuzu calls the layout with a TeX string, a size in pixels and a math
   face, and gets the width, the height and the depth from the baseline.
   Agreed: `LayoutTeX` gives a `Box`. The italic correction is in
   `Box.Italic`. It is 0 until phase 26, because the source does not compute
   it.
2. A formula becomes one SVG path of all its glyphs and rules, from the
   faces that zuzu gives, without a `FontDatabase`. Agreed: `Box.Path`. The
   hook of an arrow, which the source strokes, becomes a filled outline, so
   one filled path holds every item. A formula with `\color` has more than
   one color, and one path loses them, so zuzu then uses `ItemPath`.
3. zuzu needs the display style only. Agreed: zuzu passes `Display`. ivy
   needs the text style too, so the style is a parameter.
4. STIX Two Math is in a package of its own, which zuzu wraps. Agreed:
   `texmath/stixtwomath`.
5. A parse error names the unknown command and wraps a sentinel error of the
   root package. Agreed: `*texmath.SyntaxError` with its field `Command`,
   which wraps `okage.ErrTeXMathUnknownCommand`.
6. zuzu reads its fonts with `github.com/xo/okage/typesetting/font`, and
   that package stays public. Agreed: item 12 of D52 keeps it public. A
   `*font.Face` keeps a cache of extents and is not safe for concurrent use,
   so zuzu makes a face for each goroutine from the `*font.Font` of
   `stixtwomath.Font`.

## The phases

The phases come after phase 19. An agent claims a unit in `BACKLOG.md`, as
for any port. An estimate is in days of work of one agent.

### Phase 20. The harness

1. The checkout, the row of `upstreams.tsv` and the ledger. 0.5 days.
2. The fontTools script that reads the table MATH. 0.5 days.
3. The KaTeX script in `test/texmath/katex`, and the corpus. 1 day.

Gate: fontTools gives the values of the table MATH of STIX Two Math, and
KaTeX gives the height and the depth of `\frac{x^2+1}{\alpha-\beta}`.

### Phase 21. The reader of the MATH table

1. `tables/okage_math.go`, about 550 lines, with its tests. 1.5 days.
2. `font/okage_math.go`, about 250 lines, the load in `NewFont`, and the
   two fuzz targets. 1 day.

Gate: each value of the MATH table of STIX Two Math is the value that
fontTools reads.

### Phase 22. The font package

1. `texmath/stixtwomath`, with the font, its license and `Font`. 0.5 days.

Gate: a program that imports `texmath` and not `texmath/stixtwomath` does
not hold the font, and the build with TinyGo for `wasip1` records the size
of both.

### Phase 23. The parser

1. The tokenizer, `\ifmmode`, `macro.go`, `mukern.go`, `exat.go` and the
   parse halves of `rule.go`, `raisebox.go` and `scalebox.go`, about 700
   lines. 1 day.
2. The parse half of `parse.go`, `mathchoice.go` and `ooalign.go`, with the
   tables and `texmath/node.go`, about 2,000 lines. 2.5 days.
3. The errors and the limits of section 5, and `FuzzParse`. 0.5 days.

Gate: every input of the tests of the source parses, or fails with the
error of the source.

### Phase 24. The layout

1. `math.go` and `leading.go` on typesetting, with items in place of SVG
   text, about 1,550 lines. 2.5 days.
2. The layout halves of the other files, and `texmath/layout.go`, about 700
   lines. 1 day.
3. `texmath/item.go`: `Draw`, `Path` and `ItemPath`. 0.5 days.

Gate: every input of the tests of the source lays out, and the height and
the depth of each formula of the corpus are within the KaTeX gate of
section 7.

### Phase 25. The tests

1. The 29 test files of the source, about 5,300 lines, moved to items. 2
   days.
2. The snapshots, `FuzzLayout`, the KaTeX gates and the nightly jobs. 1 day.

Gate: the tests, the snapshots on every platform of the nightly jobs, and
the oracles of fontTools and KaTeX.

### Phase 26. The additions

1. The text face, `TextRun`, `\text` with its spaces, `\textbf` and its
   kin, `\color` and `\textcolor`. 1 day.
2. The italic correction, the top accent attachment and the math kerning
   in the layout. 1.5 days.
5. The faults of the source that the port finds, such as the spaces that
   the tokenizer drops in `\text`, the missing limits of section 5 and a
   NaN or an infinity from the parser of numbers. Each fix goes in the
   ledger and in `README.md`, as the choices at the top say. They are fixed
   in the phase that finds them.
3. `\overbrace` and `\underbrace` with the horizontal assembly, and
   `\widehat` and `\widetilde` with the horizontal variants. 1 day.
4. The helpers of section 8, if ivy asks for them. 0.5 days.

Gate: the KaTeX gates with the new commands removed from the list of
missing commands, and the snapshots of each addition.

### Phase 27. Follow go-tex/math

Port the commits of go-tex/math after the base commit, as `UPSTREAM.md`
says.

### The total

Phases 20 to 26 are about 22 days of work of one agent: 2.5, 2.5, 0.5, 4,
4, 3 and 4. They port about 4,600 lines of Go and 5,300 lines of tests. The
reader of the MATH table adds about 800 lines to `typesetting`, and the
additions of phase 26 about 800 lines to `texmath`.

## When upstream changes

1. A commit of the source that changes a command changes its parse half and
   its layout half. Port each half to its function, and name the commit in
   the ledger.
2. A commit that adds what okage added in phase 26, such as `\color` or the
   use of the italic correction, is ported. If the result is the same as the
   addition of okage, remove the addition and its line in `README.md`. If it
   is not the same, a maintainer decides which one okage keeps.
3. A commit that only changes the rounding of the source, or the use of
   go-opentype, is not applicable, because okage uses exact units and
   `typesetting`.
4. A commit that fixes a fault that okage already fixed is compared with the
   fix of okage. okage keeps the fix that gives the result of TeX, and a
   maintainer decides when that is not clear.

## Why

ivy and zuzu need the same layout, and okage already holds the fonts and the
renderer that both use. go-tex/math is small, it is a Go module under the
BSD-3-Clause license, its layout follows TeX, and go-tex/engine already sets
its math with it. A port onto typesetting needs no outside package (D6). A
tree of nodes lets ivy reuse the layout for MathML, Office Math and
OpenDocument formulas, which have no TeX. Items let ivy write PDF text that a
reader can copy, and a path lets zuzu write SVG without a font database. The
two oracles measure the port against its source and against the renderer
that Mermaid uses.

[gotexmath]: https://github.com/go-tex/math
