# D38. `html` parses and lays out the HTML labels of Graphviz

Status: Decided, amends D5.

zuzu draws graphs in the DOT language of Graphviz. The schema graphs that
dbtpl and usql write use HTML-like labels, `label=<...>`, with tables and
ports. zuzu draws each such node as its name only, and that blocks the move of
iv from go-graphviz to zuzu. okage already has an XML parser
(`internal/roxmltree`) and a text API (D37), so the maintainers chose to put
the parser and the layout of these labels in okage.

## The decision

1. A new top-level package `html` parses the text of an HTML-like label and
   lays it out. zuzu keeps the DOT syntax: its reader cuts the text between
   the outer `<` and `>` of `label=<...>` and gives that text to `html`. A
   parser for another kind of HTML, if okage adds one later, also goes in
   `html`.
2. `Parse(src)` returns a `*Label`, a list of `Issue`, and an error. The label
   holds lines of text or one table. The tree keeps each color as its raw
   string, because the caller resolves the color names of Graphviz.
3. `Parse` follows the language of Graphviz 16.1.0, as `shapes.html` of
   Graphviz describes it and as `dot` reads it:
   - The label is XML. Tags and attribute names are case-insensitive, but a
     start tag and its end tag use the same case. The named entities of HTML
     work, such as `&nbsp;` and `&eacute;`. A bare `&` or an unknown entity is
     a syntax error.
   - Text keeps its spaces as they are, and drops each character below
     U+0020, such as a newline or a tab. `dot` does the same, and it does not
     collapse or trim spaces.
   - A `<br/>` ends a line, and its `align` places the line that it ends.
   - The text tags are `b`, `i`, `u`, `o`, `s`, `sub`, `sup` and `font`. They
     nest, and they can wrap a table. A table takes the font around it.
   - Text next to a table, a row with no cell, a table with no row, and a
     rule that is not between two rows or two cells are syntax errors. Text
     between the rows or the cells of a table is not read, as `dot` ignores
     it, and it is an issue.
4. A syntax error wraps `okage.ErrHTMLSyntax` with the line and the column in
   `src`. An XML error also wraps the error of `internal/roxmltree`.
5. An attribute or a tag that `html` does not read, such as `href`, `title`,
   `id`, `class`, `gradientangle` or `<img>`, is an `Issue` with its line and
   its column. An invalid value and a port name that repeats are also issues.
   `html` never drops one without an issue. The caller decides if an issue
   is an error.
6. `(*Label).Layout(opts...)` returns a `*Layout` in points, with y down and
   the origin at the top left of the label. The layout holds the size, the
   boxes of the tables and the cells in the order that they draw, the rules,
   the runs of text, and the box of each port.
7. The layout follows `dot`:
   - A cell is as large as its content plus twice its border and its padding.
     `width` and `height` are minimums. With `fixedsize`, the cell has exactly
     that size.
   - Each cell takes the next free column of its row. A column is as wide as
     its widest cell of one column. A cell that spans columns adds the width
     that it still lacks to those columns in equal parts, in the order of the
     rows and the cells. Rows work the same way.
   - A table is as wide as its columns, the spacing between them and around
     them, and twice its border. A larger `width` makes the box larger and
     leaves the cells at the top left.
   - A table in a cell fills the content box of the cell, and its columns and
     rows share the extra space in equal parts. A table with `fixedsize`
     keeps its size, and its `align` and `valign` place it.
   - Text goes inside its cell by `align` and `valign`. Each line goes inside
     the text by the `align` of its `<br/>`, then the `balign` of the cell,
     then the center. With `align="text"`, the lines go inside the whole
     width of the cell.
   - A rule goes in the middle of the spacing. It is a segment for each cell,
     and it reaches the outer edge of the table at the ends, as `dot` draws
     it.
8. A box is the area of its table or cell. A caller draws its border inside
   the box: the path of the border is the box inset by half the width of the
   border. `dot` insets only a border wider than 1. A cell with `sides` has a
   box with no border, and a rule for each side, inset in the same way. Such
   a rule names its cell.
9. A run of text gives the left end of the run, the baseline, and the top and
   the height of its line. It gives the font, the color as the raw string, the
   flags of `u`, `o`, `s`, `sub` and `sup`, and the fill of the nearest box
   under it, so that the caller can choose a color with enough contrast. A run
   in `sub` or `sup` is 5/6 of the size of the font, and its baseline moves
   down by 1/5 or up by 1/3 of the size of the font around it.
10. The layout measures text with a function that `WithMeasure` gives. It
    returns the width, the height and the ascent of a run. The default
    measure calls `(*svg.Renderer).ShapeText`, so the fonts, the fallback and
    the shaping of `svg` apply, including the fallback for CJK text.
    `WithRenderer` gives that renderer. Without either option, `html` uses one
    renderer that it makes on the first call and then shares.
    `WithTextStyle` gives the font of text with no `<font>`. Its default is
    the default family of the renderer at 14 points, as in Graphviz.
11. A height of a line is the largest height of its runs, and its baseline is
    below its top by the largest ascent. An empty line has the height of a
    space. `dot` takes these metrics from Pango, so a line with more than one
    size of font can differ from `dot`.
12. The package is pure Go, holds no package-level configuration, and builds
    with TinyGo (D25). It imports `internal/roxmltree` and `svg`. It ports no
    upstream code, and it does not copy the code of Graphviz. The LICENSE of
    the root covers it.
13. The test module compares the layout with `dot -Tjson` (D22). The oracle
    measures each run with the width that `dot` gives, and with the height and
    the ascent of one line that `dot` gives for each size, so that it compares
    the geometry and not the fonts. A test skips when `dot` is not installed.

## Why

zuzu needs the labels to draw schema graphs, and iv gets them through zuzu.
okage has the two parts that the work needs, the XML parser and the layout of
text. A package of its own keeps the API of `svg` the same, and it leaves
room for other HTML.

`dot` is the oracle, so the layout follows `dot` where its documentation
says nothing. The request of zuzu asked for spaces that collapse and for a
table `width` that grows the columns. `dot` does neither, so `html` does
neither. The border inside the box follows the request, and not the inset of
`dot` for a border of width 1, so that every border draws the same way.
