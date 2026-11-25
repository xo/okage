# D80. pdf draws a font as paths when its subset fails

Status: Decided, amends D62.

krilla makes the subset of each font in `Document::finish`. When the subset
of one font fails, krilla and `pdf` return an error, and the whole file is
not written. ivy then makes the subset of each font in a file of its own,
and draws each page again without the font that fails. A range of glyphs that
is not valid also makes `Document.Finish` fail, and ivy checks each range
itself before the call ("The PDF writer" in `docs/BACKLOG.md`). The
maintainers want `pdf` to handle both.

## The decision

1. When the subset of a font fails in `Document.Finish`, `pdf` draws each
   glyph of that font as a path, filled or stroked as the text was, in the
   place of the text. The text stays in the file as an invisible text run
   with the same font name, so that a reader can still copy and search it.
   `Finish` returns no error for it, and logs the font and the error of the
   subset to the logger of the options, if one is set (D20).
2. A new option turns item 1 off, so that a program that wants the error of
   krilla gets it.
3. `pdf` exports the check of the ranges of `DrawGlyphs` as a function, so
   that a caller can check a run of glyphs before it calls `DrawGlyphs`. The
   check is the one that `Finish` runs, and it returns the same error.
4. Each change has a test with a font that the subsetter refuses, and the
   oracle of krilla still gives the same file for each font that does not
   fail.
5. These are deliberate differences from krilla, and `docs/DIFFERENCES.md`
   lists them.

## When upstream changes

If krilla adds a fallback for a font that does not subset, or exports the
check of the ranges, port its commit, and remove the part of this decision
that it replaces. If the result is not the same, a maintainer decides which
one okage keeps.

## Why

One broken font must not stop a document of many pages. Paths keep the look
of the text, and the invisible text keeps the text that a reader copies.
