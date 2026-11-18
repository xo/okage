// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package pdfwriter is a port of the part of the Rust crate pdf-writer 0.15.0
// that krilla calls, at the commit fdf09012 (D62). A step-by-step PDF writer.
//
// The entry point into the API is the Pdf struct, which constructs the
// document into one big internal buffer. The top-level writer has many
// methods to create specialized writers for specific PDF objects. These all
// follow the same general pattern: They borrow the main buffer mutably,
// expose a builder pattern for writing individual fields in a strongly typed
// fashion and finish up the object when dropped.
//
// There are a few more top-level structs with internal buffers, like the
// Content stream builder and the Chunk, but wherever possible buffers are
// borrowed from parent writers to minimize allocations.
//
// # Writers
//
// The writers contained is this crate fall roughly into two categories.
//
// Core writers enable you to write arbitrary PDF objects.
//
//   - The Obj writer allows to write most fundamental PDF objects (numbers,
//     strings, arrays, dictionaries, ...). It is exposed through
//     Chunk.Indirect to write top-level indirect objects and through
//     Array.Push and Dict.Insert to compose objects.
//   - Streams are exposed through a separate Chunk.Stream method since they
//     must be indirect objects.
//
// Specialized writers for things like a page or an image expose the core
// writer's capabilities in a strongly typed fashion.
//
//   - A Page writer, for example, is just a thin wrapper around a Dict and it
//     even derefs to a dictionary in case you need to write a field that is
//     not yet exposed by the typed API.
//   - Similarly, the ImageXObject derefs to a Stream, so that the Filter
//     function can be shared by all kinds of streams. The Stream in turn
//     derefs to a Dict so that you can add arbitrary fields to the stream
//     dictionary.
//
// When you bind a writer to a variable instead of just writing a chained
// builder pattern, you may need to manually drop it before starting a new
// object using Finish.
//
// # Note
//
// This crate is rather low-level. It does not allocate or validate indirect
// reference IDs for you and it does not check whether you write all required
// fields for an object. Refer to the PDF specification to make sure you
// create valid PDFs.
//
// # How the port finishes a writer
//
// Rust writes the end of an object, such as ">>", when it drops the writer,
// and the borrow checker makes sure that only the innermost writer writes.
// Go has neither, so the port does this:
//
//  1. A Buf holds a stack of its open writers: each Array, Dict and Stream
//     that is not finished.
//  2. A call on a writer first finishes each writer that is open inside it,
//     the innermost first. A call on a Chunk or a Pdf first finishes each
//     open writer of its buffer.
//  3. The method Finish of a writer finishes the writers inside it, and then
//     the writer itself. It is the trait Finish of upstream.
//  4. An Obj writes once. It panics if it writes after another write to its
//     buffer.
//  5. A call on a finished writer panics.
//
// So Go code makes the calls of the Rust code in the same order, and gets
// the same bytes. Call Finish where the Rust code calls finish or drop, or
// where a writer must end before the next call on its parent.
//
// # How a typed writer is built on the core
//
// Each later file of the port, such as content.go or structure.go, follows
// these rules, so that it adds its writers without a change to the files of
// the core:
//
//  1. A typed writer is a struct that embeds a pointer to the writer that it
//     derefs to in Rust: *Dict for the macro deref! to a Dict, *Stream for a
//     writer that derefs to a Stream. A writer that does not deref in Rust
//     holds the pointer in an unexported field, and has a method Finish that
//     calls Finish on it.
//  2. The macro writer! becomes the method start(obj Obj) on the pointer to
//     the struct. It starts the inner writer in place, and then writes the
//     fixed keys, as the macro does. Start[W](obj) then starts the writer, as
//     obj.start::<W>() does in Rust.
//  3. A writer of a stream has a function start<Writer>(stream *Stream) for
//     the method start of upstream, which takes a started stream.
//  4. A method of Chunk or of Pdf that starts a typed writer, such as
//     Chunk::pages of src/chunk.rs, goes in the file of the writer, after the
//     writer. Pdf::catalog and Pdf::document_info call the unexported
//     methods startCatalog and startDocumentInfo of lib.go.
//  5. A method that takes &mut self and returns &mut Self has a pointer
//     receiver and returns the pointer. A Rust i32 is an Int, an f32 a Real
//     and a bool a Bool, when it goes to Pair, Item or Primitive.
//  6. A TypedArray or a TypedDict of primitive values is TypedArray[T] or
//     TypedDict[T]. A TypedArray or a TypedDict of writers is
//     WriterArray[W, *W] or WriterDict[W, *W]. The trait Rewrite of upstream
//     has no port.
//  7. A Rust enum without data is a typed constant with a String method, as
//     Filter and Predictor are. Its method to_name or to_int is unexported.
//  8. A type that owns a Buf, such as Content, calls finishFrom(0) on the
//     Buf before it reads or returns the bytes.
//  9. A test of an upstream file that needs the writers of another file is
//     ported with the later of the two files. The tests test_content_limits
//     and test_chunk_limits of src/buf.rs wait for content.rs and
//     xobject.rs, and test_chunk of src/chunk.rs waits for font.rs.
//
// This is the port of the writer Pages of src/structure.rs, with
// Chunk::pages:
//
//	// Pages is Pages in pdf-writer src/structure.rs.
//	//
//	// Writer for a page tree.
//	type Pages struct {
//		*Dict
//	}
//
//	// start is the macro writer! for Pages in pdf-writer src/structure.rs.
//	func (p *Pages) start(obj Obj) {
//		p.Dict = obj.Dict()
//		p.Pair(Name("Type"), Name("Pages"))
//	}
//
//	// Kids is Pages::kids in pdf-writer src/structure.rs.
//	//
//	// Write the /Kids attribute, listing the immediate children of this
//	// node in the page tree.
//	func (p *Pages) Kids(kids ...Ref) *Pages {
//		NewTypedArray[Ref](p.Insert(Name("Kids")).Array()).Items(kids...)
//		return p
//	}
//
//	// Count is Pages::count in pdf-writer src/structure.rs.
//	//
//	// Write the /Count attribute, specifying how many descendants this
//	// node in the page tree has.
//	func (p *Pages) Count(count int32) *Pages {
//		p.Pair(Name("Count"), Int(count))
//		return p
//	}
//
//	// Pages is Chunk::pages in pdf-writer src/chunk.rs.
//	//
//	// Start writing a page tree.
//	func (c *Chunk) Pages(id Ref) *Pages {
//		return Start[Pages](c.Indirect(id))
//	}
//
// A Rust method that takes an iterator of values, such as kids, takes a
// variadic list in Go. The typed array writes the same bytes as the array
// of upstream. Then pdf.Pages(id).Kids(page).Count(1) writes the
// same bytes as pdf.pages(id).kids([page]).count(1) in Rust.
//
// The file macros.rs of upstream holds the macros writer! and deref!, which
// the rules above replace, and the macros of the tests, which are the
// helpers of macros_test.go.
package pdfwriter
