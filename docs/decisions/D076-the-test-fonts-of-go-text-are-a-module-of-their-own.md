# D76. The test fonts of go-text are a module of their own

Status: Decided, amends D63.

D63 puts back the tests of go-text that read the fonts and the data of
`github.com/go-text/typesetting-utils`, with copies of the files that they
read. Rule 2 of D63 leaves out a font whose license does not let okage copy
it. On 2026-11-20 the copies were about 120 MB, and 22 tests skipped all or
part of their work for fonts that state no license or that are proprietary.

## The decision

1. okage copies each font and each file that the restored tests read, also a
   font that is proprietary or that states no license. The maintainers judge
   that a copy for tests is a fair use. Rule 2 of D63 no longer applies.
2. A copied font keeps the license file of its source when one exists. The
   file `readme.md` of the folder lists the fonts that have no license, and
   says that they are there for tests only.
3. The copies are in `typesetting/testdata/`, and that folder holds a file
   `go.mod` of its own. The Go tool leaves a folder with its own `go.mod`
   out of the module `github.com/xo/okage`. So a program that requires okage
   does not download the copies, and no binary holds them. The folder holds
   no Go package.
4. The tests read the copies from the disk, not with `go:embed`. When the
   folder is not there, such as in the module cache of a program that
   requires okage, a test that needs it skips.

## Why

The tests of `harfbuzz`, `font` and `font/opentype/tables` check the shaper
and the parser against the expectations of HarfBuzz and go-text. Each font
that stays out removes a part of that check. A module of its own keeps the
size of the copies out of the programs that use okage.
