# Backlog

This document holds the work that is known and not done, with where each item
came from. When an item is done, delete it, and record anything that was
decided in [`decisions/`](decisions/README.md).

Work on the port goes in units: one upstream file with its tests. Claim a unit
under "Units in progress" before you start, with the name of your session.
When the change is committed, delete the claim.

## Units in progress

## typesetting

### The generator of the USE table

From D101. The maintainers asked for a Go generator of the USE table of
`typesetting/harfbuzz`, as a port of `gen-use-table.py` of HarfBuzz 15.2.0
or of `writer_use.go` of typesetting-utils. The regenerated table replaces
the mapping of the space separators to the category SPACE that D97 added by
hand in `useCategory`. The generator reads the files of the Unicode
Character Database, such as `IndicSyllabicCategory.txt` and
`IndicPositionalCategory.txt`. No copy of them was on the disk, so the
maintainers must first choose a source for these files.
