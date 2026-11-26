# D102. `pdf` gives a glyph a second CID for a second text

Status: Decided, amends D62.

## The problem

A CID font of krilla gives each glyph one CID, the glyph ID of the subset,
with the CIDToGIDMap `Identity`. The ToUnicode map then gives each glyph one
text. When a later use of the glyph has another text, the text spanner of
krilla marks that use with an ActualText span. Noto Naskh Arabic draws ى and
ي with one dotless glyph, and draws the dots of ي as a separate glyph, so a
text in Arabic takes many such spans.

ivy asked for a change on 2026-11-26, as item 5 of "Work for okage" in its
backlog. The maintainers asked for it on the same day.

## The rule

1. A glyph of a TrueType CID font whose text differs from the text of its
   first use takes a second CID, which has its own entry in the ToUnicode
   map. The same glyph with the same text takes the same second CID again.
   The first CID keeps the text of the first use, as in krilla.
2. A second CID starts at the number of glyphs of the font, which no CID of
   the subset reaches. A CIDToGIDMap stream maps each second CID to its
   glyph. The CIDs of the subset keep the glyph IDs of the subset, as with
   `Identity`. The second CID has the width of its glyph in /W, and the
   CIDSet holds it.
3. A font with no second CID keeps the CIDToGIDMap `Identity`, so other PDF
   files do not change.
4. A CFF font keeps the ActualText of krilla. Its CIDs are the CIDs of the
   CFF program, and a CIDFontType0 has no CIDToGIDMap. A font with 65,535
   CIDs in use keeps the ActualText too, because no CID is left.
5. A glyph in the same cluster as the glyph before it keeps the ActualText of
   krilla. A second CID cannot give a cluster of many glyphs its text.
6. A glyph without text takes an empty ActualText span. An example is a
   mark that the caller puts after the text of its cluster, with an empty
   text range. The ToUnicode map of krilla has no entry for it.

## Why

A comment of krilla says that only Acrobat and Chrome seem to read
ActualText. pdftotext of Poppler 26.11 and MuPDF 1.28 read it too. A second
CID puts the text of each use of the glyph into the ToUnicode map, which
each reader reads.

MuPDF gives U+FFFD for a glyph that has no entry in the ToUnicode map. It
also gives U+FFFD for an entry with an empty destination string, such as
`<0001> <>`. The PDF format does not say that such an entry is valid. An
empty ActualText removes the glyph from the text of MuPDF. Poppler gives
no text for a glyph without an entry, with or without the span.

On the file of `poi/docx/rtl.docx` of the corpus of ivy, MuPDF missed 104
of 114 words in Arabic before the change, and misses none after it.
pdftotext with `-raw` missed 75 words before the change and misses 75
after it. Poppler starts a new word at a glyph whose baseline moves by more
than half a point, even for a glyph without text. The marks that ivy draws
above or below the line then break the words. A test that moved the marks
onto the line gave each word whole.

## The code

`cidFont.aliasCID` of `pdf/text_cid.go` gives the second CIDs, and
`cidFont.serialize` writes the map, the widths, the CIDSet and the
ToUnicode entries. `spannerGlyph` of `pdf/text_group.go` chooses the second
CID and the empty ActualText. The span carries the second CIDs to the
content stream. The fallback of D80 draws the glyph of a second CID.

The switch `pdfcompat.Krilla` of `internal/pdfcompat` turns this rule off.
Only the snapshot tests of the text of krilla in `pdf` and the oracle in
`test/krilla` set it, so that they compare the port itself.
