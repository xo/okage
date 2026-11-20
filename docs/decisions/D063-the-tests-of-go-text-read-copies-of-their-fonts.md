# D63. The tests of go-text read copies of their fonts

Status: Decided, amends D52, amended by D76.

D52 copies the packages of go-text into `typesetting`. 33 of the 54 test
files of these packages read the fonts and the test data of the module
`github.com/go-text/typesetting-utils`, which holds about 165 MB. That module
is an outside package (D6), so the copy left these tests out, and most tests
of `harfbuzz`, `font` and `font/opentype/tables` did not run. This was
question 19.

On 2026-11-18 the maintainers decided:

1. okage copies into its tests only the fonts and the files of
   `typesetting-utils` that the tests of `typesetting` read, each with its
   license, and the tests stay in their packages.
2. A font whose license does not let okage copy it stays out, with the test
   that reads it. The ledger of `typesetting` says which tests are left out
   and why.
3. The tests read the copies from the `testdata/` of their packages, or
   from one folder that the packages share, so that each font is copied
   once.

The other answers were to require `typesetting-utils` in the test module,
and to compare the shaping with HarfBuzz as an oracle instead of the tests.
