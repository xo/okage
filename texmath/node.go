package texmath

import "github.com/xo/okage"

// This file ports no file of go-tex/math. It holds the tree of a formula of
// D78 section 3. The parser of TeX math in parse.go makes a tree, and a
// program such as ivy can build one from MathML or Office Math without the
// parser. Layout makes a box from a tree.

// Class is the class of an atom. It sets the space between two atoms, by the
// table of TeX Appendix G.
type Class uint8

// The eight classes of TeX.
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

// The four styles of TeX.
const (
	Display Style = iota
	Text
	Script
	ScriptScript
)

// Node is one node of a formula. Each type of this file, other than the
// enumerations, is a Node.
type Node interface{ node() }

// List is a row of nodes on one baseline. The empty List is valid.
//
// The class of a List, as a part of a formula, is the class of its only item
// when it has one item, and Ord in the other cases. As in TeX, a Bin item at
// the start or at the end of a list, after a Bin, Op, Rel, Open or Punct item,
// or before a Rel, Close or Punct item, is set as an Ord. A Space is not an
// atom: it takes no part in these rules or in the space between atoms.
type List []Node

// Atom is a nucleus with a class. Text holds its characters, which the layout
// maps through Variant to the glyphs of the math face, as one unit with no
// space inside, such as "sin". Body holds a nucleus that is a formula, such as
// {a+b}, when Text is empty. An Atom with no Text and no Body is an empty
// atom of its class.
type Atom struct {
	Class   Class
	Text    string
	Variant Variant
	Body    Node
	Large   bool   // an operator that grows in display style, such as \sum
	Limits  Limits // where the scripts of an operator go
}

// Variant is a math alphabet: the block of Mathematical Alphanumeric Symbols
// that a Latin letter or a digit maps to. The other characters do not change.
type Variant uint8

// The math alphabets.
const (
	Italic              Variant = iota // the default of a letter
	Upright                            // \mathrm, an operator name and a symbol
	Bold                               // \mathbf
	BoldItalic                         // \boldsymbol, \mathbfit
	DoubleStruck                       // \mathbb
	Calligraphic                       // \mathcal and \mathscr
	Fraktur                            // \mathfrak
	SansSerif                          // \mathsf
	SansSerifItalic                    // \mathsfit
	SansSerifBoldItalic                // \mathsfbfit
	Monospace                          // \mathtt
	SansSerifBold                      // the bold sans-serif style of text without a face for it
)

// Limits says where the scripts of an operator go.
type Limits uint8

// The places of the scripts of an operator.
const (
	LimitsAuto  Limits = iota // above and below in display style only
	LimitsAbove               // \limits
	LimitsSide                // \nolimits, and an operator such as \int or \sin
)

// Scripts attaches a superscript, a subscript and primes to Base. Base is
// never nil. The parser gives the empty List for a script with no nucleus,
// such as ^1.
type Scripts struct {
	Base     Node
	Sup, Sub Node // nil when absent
	Primes   int
}

// Fraction is \frac, \over, \atop, \binom and \genfrac. \dfrac and \tfrac are
// a StyleChange around a Fraction.
type Fraction struct {
	Num, Den    Node
	NoRule      bool  // \atop, \binom, and \genfrac with a thickness of zero
	Open, Close rune  // the delimiters of \binom and \genfrac, 0 for none
	NumAlign    Align // the place of a narrower Num over Den: \cfrac[l] and \cfrac[r]
}

// Radical is \sqrt, with an optional degree.
type Radical struct {
	Body   Node
	Degree Node // nil for a square root
}

// Delimited is \left and \right: Body between two delimiters that grow to its
// height and depth around the math axis.
type Delimited struct {
	Open, Close rune // 0 for the null delimiter "."
	Body        Node
}

// Delimiter is one delimiter of a fixed size, such as \big(, or a delimiter
// of \middle. A delimiter of \middle in the Body of a Delimited, as an item
// of its List, grows as the delimiters of the Delimited do. Outside a
// Delimited, it takes its Size.
type Delimiter struct {
	Rune   rune
	Class  Class
	Size   float64 // the height in em, centered on the math axis. 0 is the natural glyph on the baseline
	Middle bool    // \middle
}

// Accent sets an accent over Base, such as \hat. Stretch grows it to the width
// of Base with the horizontal variants and the horizontal assembly of the MATH
// table, as \widehat. The source does not stretch an accent.
type Accent struct {
	Base    Node
	Accent  rune
	Stretch bool
}

// OverUnder sets Over above and Under below Base, centered: \overset,
// \underset, \stackrel and \buildrel, and the munder and mover of MathML.
// Over and Under are set in script style.
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

// ArrayKind is the kind of an Array. It sets the space between the rows and
// the columns.
type ArrayKind uint8

// The kinds of an Array.
const (
	ArrayMatrix      ArrayKind = iota // matrix, pmatrix and its kin, and cases
	ArrayArray                        // array, with a column specification
	ArrayAligned                      // aligned and split
	ArrayGathered                     // gathered
	ArraySmallMatrix                  // smallmatrix, with its cells in script style
	ArraySubstack                     // \substack, with its rows in script style
)

// Align is the horizontal alignment of a column of an Array.
type Align uint8

// The alignments of a column.
const (
	AlignCenter Align = iota
	AlignLeft
	AlignRight
)

// Array is a grid of cells: the matrix environments, cases, array, aligned,
// gathered, smallmatrix and \substack, and the mtable of MathML. A row can be
// shorter than the others.
type Array struct {
	Kind        ArrayKind
	Rows        [][]Node
	Align       []Align // per column, centered when short
	VRules      []int   // the column gaps, 0 to the column count, with a vertical rule
	HRules      []int   // the row gaps, 0 to the row count, with a horizontal rule, as \hline
	HDashed     []bool  // for each rule of HRules, whether it is dashed, as \hdashline
	Open, Close rune    // the delimiters of pmatrix and its kin, 0 for none
	Stretch     float64 // \arraystretch, 0 for 1
}

// Space is a horizontal space, such as \quad or \,, or a negative one. In a
// List, it is not an atom, as a kern or glue of TeX.
type Space struct{ Width Length }

// Unit is the unit of a Length.
type Unit uint8

// The units of a Length.
const (
	Em Unit = iota // the size of the layout
	Ex             // the height of the letter x of the math face
	Mu             // 1/18 em
	Pt             // one unit of the size of the layout, as the source counts it
)

// Length is a TeX length. Pt and the other absolute units count one point as
// one unit of the size of the layout, as the source does. The parser converts
// in, pc, cm, mm, bp, dd, cc and sp to Pt.
type Length struct {
	Value float64
	Unit  Unit
}

// StyleChange sets Body in Style, as \displaystyle or \scriptstyle. When a
// StyleChange is an item of a List, the items of its Body, or Body itself when
// it is not a List, join that list for the space between atoms, as a style
// command does in TeX. The parser gives each style command the rest of its
// group, up to the next style command, as a Body. \dfrac is a StyleChange
// whose Body is a Fraction.
type StyleChange struct {
	Style Style
	Body  Node
}

// TextRun is text in a text face, as \text and the mtext of MathML. Its spaces
// are kept. The layout shapes it with the face that WithTextFace or
// WithTextFamily gives for its family and its style. Without such a face, it
// takes the math alphabet of its style from the math face, such as the bold
// letters for Bold, and the other characters of the math face, a space among
// them. The source sets \text with the upright glyphs of the math face.
type TextRun struct {
	Text         string
	Bold, Italic bool
	Family       Family
}

// Family is the family of the face of a TextRun.
type Family uint8

// The families of text.
const (
	FamilyRoman Family = iota // \text and \textrm
	FamilySans                // \textsf
	FamilyMono                // \texttt
)

// Color draws Body in a color, as \color and \textcolor. When a Color is an
// item of a List, the items of its Body, or Body itself when it is not a
// List, join that list for the space between atoms, as for a StyleChange.
type Color struct {
	Color okage.Color
	Body  Node
}

// SizeChange sets Body at Factor times the size of the layout, as the size
// commands \tiny to \Huge, from 0.5 for \tiny to 2.488 for \Huge. The
// factor is of the size that Layout gets, so a size command in another one
// does not multiply. A SizeChange is one atom, with the class of its Body.
type SizeChange struct {
	Factor float64
	Body   Node
}

// Rule is \rule[Lift]{Width}{Height}: a filled rectangle whose bottom edge is
// Lift above the baseline. A rule with a width or a height that is not
// positive draws nothing and keeps its size, as a strut.
type Rule struct {
	Width, Height, Lift Length
}

// Phantom keeps the size of Body and draws nothing: \phantom keeps the width,
// the height and the depth, \hphantom the width only, \vphantom and
// \mathstrut the height and the depth only.
type Phantom struct {
	Body                 Node
	Horizontal, Vertical bool // keep the width, keep the height and the depth
}

// Raise is \raisebox{Lift}[Height][Depth]{Body}: Body moved up by Lift. A
// Height or a Depth replaces the extent of the moved box when its flag is
// set.
type Raise struct {
	Body                Node
	Lift                Length
	Height, Depth       Length
	HasHeight, HasDepth bool
}

// Scale is \scalebox and \reflectbox: Body scaled by X and Y around the left
// end of its baseline. A negative X mirrors Body, and the box keeps it to the
// right of the origin. A negative Y swaps the height and the depth. The class
// of a Scale is the class of its Body.
type Scale struct {
	Body Node
	X, Y float64
}

// Boxed is \boxed, \fbox, \colorbox and \fcolorbox: Body in a frame, on a
// background, or both. Background is nil for no background. Border is the
// color of the frame, nil for the color in force. NoFrame leaves the frame
// out, as \colorbox does.
type Boxed struct {
	Body       Node
	Background *okage.Color
	Border     *okage.Color
	NoFrame    bool
}

// Cancel draws a line across Body, as \cancel, \bcancel and \xcancel. Up is
// the line from the bottom left to the top right corner, and Down the line
// from the top left to the bottom right corner. Strike is the horizontal
// line of \sout, half the height of the letter x over the baseline. The
// class of a Cancel is the class of its Body.
type Cancel struct {
	Body     Node
	Up, Down bool
	Strike   bool
}

// Phase is \phase: Body with the angle mark of electrical engineering, a line
// under Body and a slanted line at its left, as KaTeX draws it. Its class is
// Ord.
type Phase struct{ Body Node }

// Tag is \tag: Body with the label of an equation at its right, a quad
// away, as TeX sets an equation number. Label holds the parentheses of
// \tag, and none for \tag*. With WithLineWidth, the label is at the right
// margin of the line, and Body is centered in the line.
type Tag struct {
	Body, Label Node
}

// Negate is \not: a slash over Body. The class of a Negate is the class of its
// Body.
type Negate struct{ Body Node }

// Brace is \overbrace or \underbrace: a horizontal curly brace over or under
// Base, which grows to its width. Its class is Op. The Base of a Scripts that
// is a Brace takes its scripts as labels over and under the brace in each
// style, as limits, so \overbrace{x}^{n} sets n over the brace. The source
// draws a rule in place of the brace. Rune is the character of the brace,
// such as U+23B4 for \overbracket. It is the curly brace U+23DE or U+23DF
// when it is 0.
type Brace struct {
	Base  Node
	Under bool
	Rune  rune
}

// OverArrow is an arrow over Base that spans its width: \overrightarrow,
// \overleftarrow, \overleftrightarrow and \vv. Under sets it under Base, as
// \underrightarrow. Harpoon draws the upper half of each head, as
// \overrightharpoon. Segment draws a short vertical bar at each end that is
// asked for in place of a head, as \overlinesegment.
type OverArrow struct {
	Base        Node
	Left, Right bool // the ends that carry a head
	Under       bool
	Harpoon     bool
	Segment     bool
}

// Stretch is a character that grows to the width of Base over or under it,
// with the horizontal variants and the horizontal assembly of the MATH
// table: \overgroup, \undergroup, \utilde and \Overrightarrow. Its class is
// Ord.
type Stretch struct {
	Base  Node
	Rune  rune
	Under bool
}

// VCenter is \vcenter: Body centered on the math axis.
type VCenter struct{ Body Node }

// Angl is \angl: Body under a line, with a line at its right, the angle of
// the actuarial notation, as KaTeX draws it. Its class is Ord.
type Angl struct{ Body Node }

// Sideset is \sideset: the scripts of Left at the left of Base, and the
// scripts of Right at its right. Base is a large operator, which takes its
// own scripts as limits.
type Sideset struct {
	Base               Node
	LeftSup, LeftSub   Node // nil when absent
	RightSup, RightSub Node // nil when absent
}

// XArrow is an arrow that grows to the width of its labels, such as
// \xrightarrow[Under]{Over}. Arrow is the character that the command names,
// such as '→' for \xrightarrow and '↪' for \xhookrightarrow. Its class is Rel,
// and its labels are set in script style.
type XArrow struct {
	Arrow       rune
	Over, Under Node // Under is nil when absent
}

// Lap is Body with a width of zero, so that it overhangs to the right, as
// \lefteqn and \rlap, to the left, as \llap, or to both sides, as \clap.
type Lap struct {
	Body Node
	Side LapSide
}

// LapSide is the side that the Body of a Lap overhangs.
type LapSide uint8

// The sides of a Lap.
const (
	LapRight  LapSide = iota // \rlap and \lefteqn
	LapLeft                  // \llap
	LapCenter                // \clap
)

// Overlay is \ooalign: each row is set on the same baseline, over the others.
// The box is as wide as the widest row that is not hidden on both sides.
type Overlay struct{ Rows []OverlayRow }

// OverlayRow is one row of an Overlay. A row with \hidewidth on both sides is
// centered, and a row with \hidewidth on the left only is set at the right.
type OverlayRow struct {
	Body                Node
	HideLeft, HideRight bool
}

// Choice is \mathchoice: the layout sets the branch of the style in force. An
// empty branch makes no atom at all, so it does not change the space between
// its neighbors.
type Choice struct {
	Display, Text, Script, ScriptScript List
}

func (List) node()        {}
func (Atom) node()        {}
func (Scripts) node()     {}
func (Fraction) node()    {}
func (Radical) node()     {}
func (Delimited) node()   {}
func (Delimiter) node()   {}
func (Accent) node()      {}
func (OverUnder) node()   {}
func (Bar) node()         {}
func (Array) node()       {}
func (Space) node()       {}
func (StyleChange) node() {}
func (TextRun) node()     {}
func (Color) node()       {}
func (SizeChange) node()  {}
func (Cancel) node()      {}
func (Phase) node()       {}
func (Tag) node()         {}
func (Rule) node()        {}
func (Phantom) node()     {}
func (Raise) node()       {}
func (Scale) node()       {}
func (Boxed) node()       {}
func (Negate) node()      {}
func (Brace) node()       {}
func (OverArrow) node()   {}
func (XArrow) node()      {}
func (Lap) node()         {}
func (Overlay) node()     {}
func (Choice) node()      {}
func (Stretch) node()     {}
func (VCenter) node()     {}
func (Angl) node()        {}
func (Sideset) node()     {}
