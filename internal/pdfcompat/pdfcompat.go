// Package pdfcompat holds the switch that makes the package pdf write the
// text of a glyph as krilla does, without the second CIDs and the empty
// ActualText of D102. Only the tests that compare pdf with krilla set it:
// the snapshot tests of krilla in pdf and the oracle in test/krilla. A
// program never sets it.
package pdfcompat

// Krilla is true while a test against krilla runs. pdf reads it when it
// gives a glyph its text.
var Krilla bool
