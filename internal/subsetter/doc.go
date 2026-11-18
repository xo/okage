// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package subsetter is a port of the Rust crate subsetter 0.2.6, at the
// commit c3f23c67 (D62). It makes the subsets of OpenType fonts with
// TrueType or CFF outlines for embedding in PDF files. The PDF writer of
// okage embeds the subset of each font of a document, as krilla does.
//
// # Scope
//
// The subsets are for PDF files only. The subsetter removes much of the
// information of a font that a PDF file does not need, and that other uses
// of a font need. This is on purpose.
//
// A user of the subsetter must know these facts:
//
//   - A font goes into a PDF file as a CID font. The subsetter removes the
//     cmap table, so the PDF file gives its own map of the characters.
//   - The subsetter keeps the copyright information of the font.
//   - A CID font addresses each glyph with a CID. The subsetter converts a
//     CFF font with SIDs into a font with CIDs, and it maps each new glyph ID
//     to the same CID in every font. So the new glyph ID of a glyph is its
//     CID.
//
// # Example
//
// This example keeps the glyphs 68, 69 and 70 of Noto Sans, the letters a, b
// and c:
//
//	data, err := os.ReadFile("NotoSans-Regular.ttf")
//	if err != nil {
//		return err
//	}
//	// The remapper gives the glyphs new IDs that follow each other: the
//	// glyph IDs of a font start at 0 and have no gap. The glyph 0 is always
//	// in the subset.
//	remapper := subsetter.NewGlyphRemapper()
//	for _, glyph := range []uint16{68, 69, 70} {
//		remapper.Remap(glyph)
//	}
//	sub, err := subsetter.Subset(data, 0, remapper)
//	if err != nil {
//		return err
//	}
//	for _, glyph := range []uint16{68, 69, 70} {
//		id, _ := remapper.Get(glyph)
//		fmt.Printf("Glyph %d has the ID %d in the new font\n", glyph, id)
//	}
//
// The font has 556 KB, and the subset has 1 KB.
//
// # The files of the port
//
// Each file of src of subsetter is a Go file of the same name, as
// docs/UPSTREAM.md says, such as glyf.go for src/glyf.rs. A function subset
// of a module, such as glyf::subset, is subsetGlyf. A type of a module whose
// name collides, such as Table in src/glyf.rs and src/name.rs, takes the
// name of its module as a suffix, such as tableGlyf (D10).
//
// read.go and write.go hold the reader and the writer of binary data. The
// trait Readable of upstream becomes a method of reader for each integer
// type, such as readU16, and a function for each other type, such as
// readTableRecord. The trait Writeable becomes a method of writer for each
// integer type, such as writeU16, and a method write of each other type. A
// Rust Option of a read is a second result of the type bool. Each case of
// the enum Error of upstream is a sentinel error of okage, such as
// okage.ErrSubsetMalformedFont, which lib.go names errMalformedFont (D21).
//
// subsetter reads some tables of a font with skrifa and read-fonts. The port
// reads them with typesetting (D52, D62). Each table that subsetter reads
// with its own code, the port reads with the port of that code.
//
// # The seams of the units of phase 13
//
// Phase 13 ports subsetter in four units. Units 1 and 2 are the reader, the
// writer, the remapper, the tables head, hhea, hmtx, maxp, name, post, glyf
// and loca, and lib.rs. Unit 3 ports src/cff, and unit 4 ports src/cff2.rs,
// the rest of src/interjector.rs, and the code of write-fonts that writes a
// simple glyph. lib.go calls each part of units 3 and 4 through a package
// variable of a function, a seam. The file of the unit sets its seam in a
// function init, so that the unit adds files and changes no file of units 1
// and 2. A seam that is nil gives an error that wraps
// okage.ErrSubsetUnimplemented, with the text "is not ported yet". These
// are the seams:
//
//   - subsetCFFTable is cff::subset of src/cff/mod.rs. cff.go sets it.
//     Until then, a font with a CFF table gives the error.
//   - subsetCFF2Table is cff2::subset of src/cff2.rs. cff2.go sets it. Until
//     then, SubsetWithVariations of a CFF2 font gives the error.
//   - interjectGlyph is SkrifaInterjector::interject of src/interjector.rs.
//     The file of unit 4 sets it. Until then, SubsetWithVariations of a
//     TrueType font at a location other than the default gives the error,
//     and so does a CFF2 font when cff2.go is there.
//
// For example, unit 3 writes this in cff.go:
//
//	func init() {
//		subsetCFFTable = subsetCFF
//	}
//
//	// subsetCFF is subset in subsetter src/cff/mod.rs.
//	func subsetCFF(ctx *context) error {
//		...
//	}
//
// The code of units 3 and 4 reads the font with ctx.expectTable and the
// reader of read.go, writes with the writer of write.go, and adds a table
// with ctx.push. cff2::subset calls subsetWith of glyf.go, and sets
// ctx.customMaxpData and ctx.customHmtxData, as upstream does. interject
// gets the skrifaInterjector of interjector.go, which holds the font as
// read-fonts finds it and the normalized coordinates of the location. When a
// unit is ported, its seam is no longer nil, and the tests that skip or
// expect the error for it run the test of upstream. The tests of that unit
// add their fonts to testdata, with their licenses (D63), and the lines of
// testdata/subsets.tests for those fonts then run.
//
// # Differences from upstream
//
// The subsets are the subsets of upstream, byte for byte. The test module
// compares them with the oracle in test/krilla. The port differs in these
// ways:
//
//  1. Where upstream panics on a font, the port returns an error. The
//     closure of glyf.go returns okage.ErrSubsetOverflow when it adds a
//     glyph to a remapper that holds 65535 glyphs. GlyphRemapper.Remap still
//     panics as upstream does, because the caller gives the glyphs.
//  2. The closure of glyf.go reads each glyph once, and removes from its
//     stack the entries that upstream skips. It gives the same glyphs in the
//     same order, and its memory and time do not grow with the square of the
//     number of components, as the closure of upstream can.
//  3. skrifa computes the normalized coordinates of a location in the fixed
//     point numbers of OpenType, and reads the avar table of version 2. The
//     port computes them as typesetting does, in float32, and maps them with
//     the avar table of version 1 only. Only a coordinate that is very near
//     the default can give another choice between the subset of the default
//     location and an instance.
//  4. GlyphRemapper.Hash writes the bytes of the glyphs in little-endian
//     order on each machine, as internal/siphasher reads them. Upstream
//     writes them in the order of the machine.
package subsetter
