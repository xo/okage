// Package texmath lays out mathematical formulas with the OpenType MATH table
// of a font, by the rules of TeX Appendix G. It is the port of
// github.com/go-tex/math at the commit 6a6d2729, and a fork of it (D78).
//
// A formula is a tree of nodes, such as Atom, Scripts and Fraction. Parse
// reads TeX math and returns its tree:
//
//	tree, err := texmath.Parse(`\frac{a}{b}`)
//
// A program can also build a tree itself, for example from MathML. Layout
// lays out a tree with a face that has a MATH table, such as STIX Two Math of
// the package texmath/stixtwomath, at a size and in a style. LayoutTeX
// parses and lays out in one call:
//
//	f, err := stixtwomath.Font()
//	...
//	box, err := texmath.LayoutTeX(`\frac{a}{b}`, font.NewFace(f), 16, texmath.Display)
//
// The result is a Box: its width, its height above the baseline, its depth
// below it, and its items. An item is a glyph of a face, a filled rectangle,
// or a filled path, such as the head of an arrow. Each length is in the unit
// of the size, such as pixels or points, and y grows down. Box.Draw fills the
// items into an okage Pixmap. Box.Path gives the outline of every item as one
// path, and Box.ItemPath the outline of one item, for a writer of SVG. They
// read the outlines from the faces of the items, and need no FontDatabase.
//
// An error of Parse is a *SyntaxError. It wraps okage.ErrTeXMathSyntax, and
// also okage.ErrTeXMathUnknownCommand for an unknown command, whose name is in
// its field Command. Layout gives okage.ErrMathNoMathTable for a face without
// a MATH table. A formula past a limit gives okage.ErrMathLimit. The limits
// are:
//
//   - The depth of nesting is at most 100 levels (WithMaxDepth). Each step
//     of the recursion of the parser is a level: a list, an atom, a command,
//     an argument, and the scripts of a nucleus. A tree of the parser is at
//     most twice as deep as its levels, and the layout allows twice the
//     depth, with a level for each node that holds other nodes. A
//     StyleChange takes no level. On TinyGo for wasip1, a goroutine has a
//     stack of 64 KiB that does not grow, and Parse takes at most 27 KiB
//     of it for the deepest formula.
//   - The parser makes at most 100,000 nodes (WithMaxNodes). A box has at
//     most 1,000,000 items.
//   - The macros of \def expand at most 100,000 times, the budget of the
//     source (WithMaxExpansions). The source and its expansion have at most
//     1,048,576 tokens.
//   - A length of \rule, \raisebox, \mkern, \mskip or \genfrac is at most
//     16,384 pt, em or ex, or 16,384 em in mu. A factor of \scalebox is a
//     finite number of at most 100. The size of Layout is a positive finite
//     number.
//
// # Text, colors and the KaTeX commands
//
// The additions of phase 26 of D78 give the commands that the labels of
// Mermaid use and that the source does not have.
//
// \text, \textrm, \textbf, \textit, \textsf, \texttt, \textup, \textmd,
// \textnormal and \emph set their argument as text, a TextRun, with its
// spaces. Each one changes one part of the text style in force, as in LaTeX:
// \textbf in \textit is bold italic. WithTextFace gives the four faces of
// the roman family, and WithTextFamily the faces of the sans-serif and the
// monospace families. The layout shapes a run with typesetting/shaping, so
// the ligatures and the kerning of the face apply, and a character that the
// face has no glyph for takes the math face. A style without a face takes
// the math alphabet of that style from the math face: the upright, bold,
// italic or bold italic letters and digits, and the sans-serif or the
// monospace ones. Its other characters, the spaces among them, are the
// glyphs of the math face. So without a text face, \text sets the upright
// glyphs of the math face, as the source does.
//
// \color sets the color of the rest of its group, and \textcolor of its
// argument, with a name of CSS, such as red, or a number such as #228B22, as
// KaTeX reads them. \colorbox, \fcolorbox and \fbox set text on a
// background, in a frame, or both.
//
// \cancel, \bcancel and \xcancel draw a line across their argument, as KaTeX
// does. \phase draws the angle mark of electrical engineering around its
// argument, a line under it and a slanted line at its left, as KaTeX does.
// \tag and \tag* set the label of an equation a quad after the formula, as
// TeX sets an equation number. With WithLineWidth, the label is at the
// right margin of the line, and the formula is centered in it. \hline draws
// a rule between the rows of array and of the matrices, and \hdashline a
// dashed rule. \cfrac sets a continued fraction in display style, and
// \cfrac[l] and \cfrac[r] set its numerator at the left or the right. \kern and
// \hspace give a space of any length, and \llap, \rlap and \clap, with
// \mathllap, \mathrlap and \mathclap, set their argument with no width.
// \scriptscriptstyle, \dddot and \ddddot work. \newcommand, \renewcommand
// and \providecommand define a macro in the formula, as \def does, and the
// first parameter can have a default value. The macros of KaTeX that
// stand for a symbol, a font or an operator, such as \R, \N, \Z, \reals,
// \larr and \bold, are known unless the formula defines the name.
//
// \overbrace and \underbrace draw the brace of the face, U+23DE or U+23DF,
// grown to the width of their argument with the horizontal variants and the
// horizontal assembly of the MATH table, and set their labels over and under
// it. \widehat, \widetilde and \widecheck take the widest horizontal variant
// of their combining mark that is not wider than their argument, as TeX
// does. \middle grows as its \left and \right do. \smash removes the height and
// the depth of its argument, \smash[t] the height only, and \smash[b] the
// depth only. The size commands, from \tiny to \Huge, set the rest of their
// group at a factor of the size of Layout, from 0.5 to 2.488, as LaTeX and
// KaTeX do.
//
// On 2026-11-25 the maintainers asked for the rest of the commands of KaTeX
// (D95). The symbols, the operators and the macros of KaTeX all parse, with
// the accents and the symbols of text, such as \text{\'{a}} and \ss. The
// arrows, the harpoons and the line segments go over or under a base, as
// \underrightarrow. \overgroup, \undergroup, \utilde and \Overrightarrow
// grow a character of the face, and \overbracket and \underbracket a
// square bracket. \sideset sets scripts at the left and the right of a large
// operator. \vcenter, \angl, \sout, \smallint, \above, \brace, \brack,
// \verb, \url and the extensible arrows of two shafts work, and align,
// alignat, gather, equation and rcases are environments. \href and the
// commands of HTML set their body without the link, and \includegraphics
// is an empty space of the size of the image. A \\ outside an environment
// makes no line break.
//
// The layout uses three more parts of the MATH table, and an option turns
// each one off for a comparison with the source. The italic correction of a
// glyph, such as the italic f, moves a superscript to the right, moves the
// limits of an operator apart, and follows the glyph in a list, as TeX
// Appendix G says (WithItalicCorrection). Box.Italic gives the correction of
// the last glyph. The top accent attachment puts an accent, which is the
// combining mark of the face, over the attachment point of a glyph
// (WithAccentAttachment). The math kerning of the corners of two glyphs moves
// a script into the shape of its nucleus, such as the subscript of an italic
// V (WithMathKern).
//
// On 2026-11-26 ivy asked for six changes, which follow TeX where the source
// and the MATH table do not (D103). \ldots, \dots, \mathellipsis, \cdots and
// the dots of amsmath are three dots of the class Punct in an atom of the
// class Inner, as fontmath.ltx and amsmath set them, so 1,\ldots,n has the
// width of TeX. \dots sets the dots at the center before a binary operator,
// a relation or a large operator. The primes of x' are at the size of x,
// after it, and the scripts attach to the primes. A superscript of \prime
// only is primes. An accent over one character takes its scripts as TeX
// Appendix G, rule 12, says, so the subscript of \hat y_i is where the
// subscript of y_i is. The drop of a subscript under a nucleus that is not
// one character, SubscriptBaselineDropMin, is at the size of the script.
// WithMathConstant sets a constant of the MATH table, and
// WithSubSupShiftDown gives a subscript with a superscript the shift sub2
// of TeX. The cells of aligned, alignedat, split and gathered, and of the
// environments of KaTeX that take their layout, are in display style.
//
// The commands that KaTeX parses and Parse does not are in
// test/texmath/katex_parse.txt, with their reasons.
//
// # Differences from go-tex/math
//
// The parser makes a tree and lays nothing out, and the layout makes items
// and not SVG text. The source lays out each command as it parses it. The
// layout reads exact font units with typesetting, scaled by a size that is a
// float64. It imports typesetting/shaping to shape the runs of a text face.
// The source asks go-opentype for a face at a whole number of
// pixels, which rounds each value, and takes the ink of a glyph from the
// points of an outline of lines. The hook of \xhookrightarrow is a filled
// outline, so that Box.Path holds every item. The source strokes it.
//
// The port also fixes these faults of the source. The files and the lines
// are those of the source. First the faults of the parser:
//
//   - parse.go:39 drops every space, also in \text, so \text{if } b sets
//     "ifb". The text of \text is a TextRun that keeps its spaces, as TeX
//     does: TeX skips the white space after a control word and after a
//     comment only. In \text, an apostrophe is a character and not a prime
//     (parse.go:80), and ~, \ , \%, \&, \#, \_, \$, \{ and \} are text.
//   - parse.go:39 sets a carriage return as a character. It is white space.
//   - parse.go:254, parse.go:499 and parse.go:526 recurse with no limit on the
//     depth. The depth is limited.
//   - The source has no limit on the nodes. The nodes are limited.
//   - macro.go:59 copies the rest of the input at each expansion, in
//     quadratic time, and nothing limits the tokens that an expansion makes.
//     The expansion is a stack, and the tokens are limited.
//   - parse.go:145 scans for the \fi of each \ifmmode and recurses into the
//     true branch, in quadratic time for nested conditionals. One pass with a
//     stack gives the same result.
//   - mukern.go:79 reads each part of the glue of \mskip with glue of its own,
//     so a run of "plus" parts recurses with no bound. A part of the glue has
//     no glue.
//   - scalebox.go:62 and scalebox.go:84 take NaN and Inf from
//     strconv.ParseFloat into the SVG text, and rule.go:94, mukern.go:43 and
//     parse.go:794 take any finite length. These are limits now.
//   - exat.go:22 converts a size of 2^47 points or more to an integer out of
//     range, and exat.go:37 loops once for each point of a negative size.
//     exAt returns 1.5 for a large size before the conversion, and stops the
//     loop when \vfuzz is 0, which gives the same value.
//   - parse.go:325 drops \limits, \nolimits and \displaylimits, and a script
//     after them then attaches to an empty atom after the operator. They now
//     set where the scripts of the operator go.
//   - parse.go:802 sets the style 3 of \genfrac in script style. It is
//     scriptscript, as in TeX.
//   - parse.go:1066 adds an empty row after a \\ before \end. As in LaTeX
//     and KaTeX, it starts no row.
//   - parse.go:599 sets \middle at one size, 1.8 em, parse.go:699 keeps the
//     extent of \smash, parse.go:329 ignores the size commands such as
//     \large, and parse.go:660 draws \overbrace and \underbrace as a rule.
//     The additions above replace each one.
//   - mathchoice.go:45 parses the chosen branch of \mathchoice only, so a
//     fault in another branch is not reported. Each branch is parsed, because
//     the style is not known before the layout.
//   - parse.go:316 and parse.go:411 set the denominator of \over, and the
//     fraction, in the style of a style command before \over. As in TeX, the
//     style command sets only the part of the numerator after it.
//
// Then the faults of the layout:
//
//   - math.go:163, math.go:191 and parse.go:323 set a script at the script
//     size of the size in force, so a script of a script is 70% of a script,
//     and each level is smaller again. TeX has three sizes: the text size, and
//     the script and scriptscript sizes of ScriptPercentScaleDown and
//     ScriptScriptPercentScaleDown. \displaystyle and \textstyle go back to
//     the text size.
//   - math.go:163 drops every space between atoms in a script, and
//     math.go:191 keeps every space in a fraction in text style, whose parts
//     are at the script size. In script and scriptscript style, TeX keeps
//     only the thin spaces next to an Op.
//   - math.go:443 sets a Bin as an Ord only at the start of a list and after
//     a Bin, Op, Rel, Open or Punct. As in TeX, it is an Ord also before a
//     Rel, a Close or a Punct, and at the end of a list.
//   - parse.go:1001 and mukern.go:33 make each space, such as \, or \mkern, an
//     Ord atom, so \int\mkern-6mu\int took two thin spaces. A space is not an
//     atom, as a kern or glue of TeX.
//   - math.go:475 and math.go:485 place a superscript and a subscript each on
//     its own, with SuperscriptShiftUp, SuperscriptBaselineDropMax and
//     SubscriptShiftDown only. The scripts follow Appendix G, rule 18, with
//     SuperscriptBottomMin, SubscriptBaselineDropMin, SubscriptTopMax,
//     SubSuperscriptGapMin, SuperscriptBottomMaxWithSubscript and
//     SpaceAfterScript. The drops of the baselines apply to a nucleus that is
//     a box, and not to one glyph of a character, as rule 18a and the
//     OpenType specification say.
//   - math.go:477 and math.go:487 count only the height of a superscript and
//     the depth of a subscript, so ^1 had no depth and _2 no height.
//   - math.go:512 and math.go:517 place a limit by UpperLimitGapMin and
//     LowerLimitGapMin only. Its baseline is also at least
//     UpperLimitBaselineRiseMin over the operator, or
//     LowerLimitBaselineDropMin under it.
//   - math.go:553 and math.go:555 place the numerator and the denominator by
//     the gaps only. Their baselines are also at least
//     FractionNumeratorShiftUp and FractionDenominatorShiftDown from the
//     baseline, or the constants of display style.
//   - math.go:570 places the parts of \atop and \binom with the gaps of a
//     fraction and the thickness of the rule. They take StackTopShiftUp,
//     StackBottomShiftDown and StackGapMin, as Appendix G, rule 15c, says.
//   - math.go:887 puts all the excess of a radical sign that is taller than
//     the body below the body. As Appendix G, rule 11, says, half of the
//     excess opens the gap over the body.
//   - parse.go:1007 sets a large operator on the baseline, so the variant of
//     display style hung from it. A large operator is centered on the math
//     axis, as Appendix G, rule 13, says.
//   - math.go:1083 grows the delimiters of \left, \right and the matrices to
//     the whole height around the axis, so a delimiter was often a size too
//     large. They cover 901/1000 of it and miss at most 0.5 em, with the
//     \delimiterfactor and \delimitershortfall of plain.tex.
//   - parse.go:581 draws the shaft and the heads of \xrightarrow at the size
//     and on the axis of the script style of its labels, so the arrow sat
//     under the math axis. They are those of the style in force.
//   - math.go:660 leaves the hook of \xhookrightarrow out of the depth.
//   - parse.go:935 and parse.go:1044 set the rows of \substack and the cells
//     of smallmatrix one style down from the style in force. They are in
//     script style, as \scriptstyle in the templates of amsmath sets.
//   - parse.go:983 draws the null delimiter of \big. as the glyph .notdef,
//     and math.go:933 draws .notdef for a delimiter without a glyph. Both are
//     empty boxes.
//   - math.go:1038 stacks the parts of an assembly from the last one up. The
//     OpenType specification lists the parts from the bottom to the top, so
//     the top part was at the bottom, and a tall parenthesis was upside
//     down. The first part is at the bottom.
//   - parse.go:1702 sets each accent with the spacing accent, such as
//     U+02C6, centered on the advance of its nucleus, and \vec with a
//     combining mark of no advance, so the arrow was too far to the left.
//     An accent is the combining mark of the face, at its attachment point.
package texmath
