# D61. The font loader limits tables, and the matcher groups selectors by name

Status: Decided.

ivy reads fonts from strangers, and it lays out large HTML documents with
`typesetting`, `svg` and `cssselect2`. On 2026-11-18 a maintainer asked okage
for three changes, for ivy. This decision records the commits of go-text
that okage applies out of order, each difference from go-text and from
cssselect2, and the new API. Each change of `typesetting` has a comment that
starts with `okage:` and names this decision (D52).

## The length of a table

The table directory of a font gives the offset and the length of each table.
The loader of `font/opentype` made a buffer of that length before it read
the table. So a font whose directory claims a table of 4 GB asked for 4 GB,
and broke the memory limits of ivy (ivy D5).

1. okage applies seven commits of go-text out of order: `0d88d2a5`, then
   `23022c67` to `be47f20d`. They add `opentype.ResourceSlicer` and
   `opentype.NewSlicedResource`. A loader whose resource is a
   `ResourceSlicer` returns each table that is not compressed as a part of
   the resource, without a copy. `Slice` refuses a range that ends after the
   end of the resource. `0d88d2a5` changes the lines that the other commits
   change, so okage applies it first.
2. Their rows in `docs/upstream/typesetting.tsv` are `ported`. The column
   `matched` does not move, because older commits are still pending (D12).
   The tests of these commits read `typesetting-utils`, so the copy leaves
   them out (D52).
3. A loader whose resource is not a `ResourceSlicer` still copies each
   table. Before it makes the buffer, it reads the last byte of the table.
   If the resource has no such byte, `RawTable` returns the error of
   `ReadAt`, as go-text does after it makes the buffer.
4. A compressed table of WOFF gives its length after decompression, which
   the size of the resource does not limit. If the buffer of the caller is
   too small, the loader grows a buffer as the data comes, and does not make
   a buffer of that length. A stream that ends early gives
   `io.ErrUnexpectedEOF`, as `io.ReadFull` does.
5. The tests in `typesetting/font/opentype/okage_test.go` build fonts whose
   directory claims a table of almost 4 GB. Each read returns an error, and
   allocates less than 1 MB.

## A region of variation with no coordinate

`tables.VariationRegion.Evaluate` gives the factor of a region of an item
variation store, such as the store of MVAR, HVAR or CFF2. go-text reads only
the axes that it has a coordinate for. A face without variations has no
coordinates, so go-text gave the factor 1 to each region, and applied every
delta of the font in full.

1. The copy reads an axis without a coordinate at its default, 0, as
   HarfBuzz and skrifa do. A face without variations now gets the values of
   the tables.
2. Upstream commit `c6965dda`, which is pending, changes the same function.
   When okage applies it, keep the change of okage.

## The metrics of the line through

1. `svg.FontMetrics` has the new field `LineThroughThickness`. It is
   `yStrikeoutSize` of OS/2, with the delta `strs` of MVAR at the location of
   the face, rounded as `UnderlineThickness` is. A font without OS/2, or with
   a thickness that is not positive, gets `UnderlineThickness`.
2. The layout of text of `svg` still draws the line through with the
   thickness of the underline, as usvg does.
3. ivy found that `FaceMetrics` differed from go-text for Adwaita Sans. For
   example, the position of the underline was -348 against -229. The values
   of `FaceMetrics` are the values of the tables at the default location.
   go-text gave other values only for a face without variations, because of
   the fault of `Evaluate` above. With the change, both give the same values
   at each location.

## A faster Matcher

`cssselect2.Matcher` keeps its selectors in groups: by ID, by class, by the
name of the element, by namespace, by the attribute `lang`, and the others.
`Match` tests each selector of the groups of an element. A selector whose
last compound selector has no ID, class or type selector is in the group of
the others, and `Match` tests it on every element. The user-agent style
sheet of WeasyPrint has 24 such selectors, such as
`[dir=ltr i] :is(dir, menu, ol, ul)`.

1. A selector of the group of the others whose subject can only have some
   names goes into the group of each of these names. The names come from the
   type selectors of the last compound selector, and from the selectors in
   `:is()` and `:where()` of that compound selector, at any depth. An element
   has one name, so `Match` finds such a selector once at most.
2. A selector can only match an element whose name, in ASCII lower case, is
   in its list. So the groups change which selectors `Match` tests, and not
   the result.
3. `Match` collects the matches in an array on the stack, and returns a copy
   of the length of the result. Upstream grows its list for each match.
4. `Match` sorts the matches by insertion when there are 32 or fewer. Each
   match has its own order, so any sort gives the same result.
5. `Matcher.AppendMatches(dst, element)` is new. It appends the result of
   `Match` to `dst`. A program that reuses `dst` for each element does not
   allocate.
6. `TestMatchesEachSelector` tests each selector of the tests of cssselect2,
   and selectors with `:is()` and `:where()`, on each element of three
   documents. `Match` must return the selectors whose test is true, in order.

On a table of 1,000 rows and 10 columns, with the user-agent style sheet of
WeasyPrint, `Match` used 1.9 kB in 5 allocations for each element. It now
takes about a third of the time, with 570 bytes in one allocation.
`AppendMatches` takes about a fifth of the time, and does not allocate.
`BenchmarkMatchTable` and `BenchmarkAppendMatchesTable` measure the same work
with a sheet of the same kind.
