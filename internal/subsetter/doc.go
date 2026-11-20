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
// interjector_skrifa.go holds the rest of src/interjector.rs, which draws
// the glyphs of an instance of a variable font. writefonts_simple.go holds
// the code of write-fonts 0.48.0 that writes a simple glyph, which
// interjector.rs calls. It has the license of write-fonts, which LICENSE
// holds.
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
//   - subsetCFFTable is cff::subset of src/cff/mod.rs. cff.go sets it,
//     and a font with a CFF table gives its subset.
//   - subsetCFF2Table is cff2::subset of src/cff2.rs. cff2.go sets it.
//   - interjectGlyph is SkrifaInterjector::interject of src/interjector.rs.
//     interjector_skrifa.go sets it.
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
//     panics as upstream does, because the caller gives the glyphs. A CFF
//     font gives an error that wraps okage.ErrSubsetMalformedFont when the
//     subset must hold a real number that is not finite, and an error that
//     wraps okage.ErrSubsetOverflow when its glyphs use 256 font DICTs.
//  2. The closure of glyf.go reads each glyph once, and removes from its
//     stack the entries that upstream skips. It gives the same glyphs in the
//     same order, and its memory and time do not grow with the square of the
//     number of components, as the closure of upstream can.
//  3. skrifa reads the avar table of version 2. The port computes the
//     normalized coordinates of a location in the 16.16 fixed-point numbers
//     of OpenType, as skrifa does, but maps them with the avar table of
//     version 1 only (D84).
//  4. GlyphRemapper.Hash writes the bytes of the glyphs in little-endian
//     order on each machine, as internal/siphasher reads them. Upstream
//     writes them in the order of the machine.
//  5. The decompiler of the charstrings of cff_charstring.go runs at most
//     1,000,000 operators for one glyph, through all the calls of
//     subroutines, and makes a program of at most 1,000,000 instructions.
//     Past either limit, it returns an error that wraps
//     okage.ErrSubsetMalformedFont. Upstream has no limit other than a
//     depth of 64 calls, so a glyph whose 63 nested subroutines each call
//     the next one twice runs 2^63 subroutines. The maintainers chose the
//     limits on 2026-11-18. A glyph of a font has some hundreds of
//     operators.
//  6. Upstream decompiles the charstring of each glyph into a program, and
//     compiles all the programs at the end. The port compiles each program
//     when it is decompiled. The bytes are the same, and the memory holds
//     one program only.
//  7. The CFF code of upstream holds the offsets and the ranges of the
//     DICTs in a usize, which has 64 bits on the machines that the oracle
//     runs on. The port holds them in a uint64, so that a machine of 32
//     bits gives the same result.
//  8. skrifa computes the deltas of the variations in the 16.16
//     fixed-point numbers of OpenType, and rounds each step. typesetting
//     computes them in the same way (D84), so an instance of a variable
//     font is the same as the instance of upstream. The delta of HVAR comes
//     from typesetting in float32, which holds it exactly when it is less
//     than 256 units. test/krilla requires the instances of upstream, byte
//     for byte.
//  9. skrifa reads a CFF2 table with the FontMatrix of its dictionaries, and
//     typesetting does not read it. A CFF2 font with a FontMatrix gives
//     other outlines.
//  10. The port of write-fonts takes one path, where write-fonts takes the
//     paths of many instances, so that they can be interpolated.
//  11. The glyph data of a TrueType subset holds at most 8 times the size of
//     the font data plus 1 MiB. Past that size, the subset returns
//     okage.ErrSubsetTooLarge before it copies the glyphs. Upstream copies a
//     glyph once for each glyph ID that points at it, so a hostile font can
//     make a subset of some GB from a font of some hundred KB (D74).
package subsetter
