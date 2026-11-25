// Copyright 2023 The xmp-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package xmpwriter is a port of the Rust crate xmp-writer 0.3.3, at the
// commit 9498ae2b that krilla uses (D99). It writes XMP metadata, step by
// step. The package pdf writes the XMP metadata of a document with it.
//
// XMP is a metadata format developed by Adobe. It is either embedded into
// files (e.g. PDF, JPEG, TIFF) or stored in a separate "side-car" file.
//
// Start by creating a new Writer with New, then add entries to it. Finally,
// call Writer.Finish to get the XMP metadata as a string. Some properties
// contain a complex data type like a struct or an array. In this case, the
// writer returns a new writer for the struct or the array. Rust closes the
// struct or the array when it drops that writer. Go has no drop, so the
// caller calls its method Finish before it uses the main writer again:
//
//	w := xmpwriter.New()
//	w.Creator("Martin Haug")
//	de := xmpwriter.LangID("de")
//	w.Title(xmpwriter.LangAlt{Lang: &de, Value: "Titel"}, xmpwriter.LangAlt{Value: "Title"})
//	w.NumPages(3)
//	w.PDFKeywords("Keyword1, Keyword2")
//	w.Description(xmpwriter.LangAlt{Value: "Description"})
//	w.Date(xmpwriter.Date(2021, 11, 6))
//
//	colors := w.Colorants()
//	colors.AddColorant().SwatchName("Red").Finish()
//	colors.AddColorant().SwatchName("Green").Finish()
//	colors.Finish()
//
//	w.CreatorTool("xmp-writer 0.3.3")
//	fmt.Println(w.Finish(""))
//
// The files of the package follow the files of the crate: lib.rs is lib.go,
// types.rs is types.go, and pdfa.rs, the module of the feature pdfa that the
// crate enables by default, is pdfa.go. A name of the crate keeps its Rust
// name in Go form, so XmpWriter is Writer, Namespace::PdfAId is
// NamespacePdfAID and PdfAExtSchemaWriter is PDFAExtSchemaWriter. A method
// of upstream that takes an iterator takes a variadic list of items.
//
// See also:
//   - XMP Specification, Part 1: Basics
//   - XMP Specification, Part 2: Additional Properties
//   - XMP Specification, Part 3: File Embedding and Interchange
package xmpwriter
