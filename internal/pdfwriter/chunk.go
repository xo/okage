// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"iter"
	"slices"
)

// This file ports pdf-writer src/chunk.rs. The methods of Chunk that start
// a typed writer, such as Chunk::pages and Chunk::image_xobject, are in the
// files of their writers, as the package comment says. Chunk::name_tree and
// Chunk::number_tree are here, as the functions ChunkNameTree and
// ChunkNumberTree.

// Settings is Settings in pdf-writer src/chunk.rs. Its zero value turns
// Pretty off. DefaultSettings gives the default of upstream.
//
// Settings that should be applied while writing a PDF file.
type Settings struct {
	// Whether to enable pretty-writing. In this case, pdf-writer will
	// serialize PDFs in such a way that they are easier to read by humans by
	// applying more padding and indentation, at the cost of larger file
	// sizes. If disabled, pdf-writer will serialize objects as compactly as
	// possible, leading to better file sizes but making it harder to inspect
	// the file manually.
	//
	// Default value: true.
	Pretty bool
}

// DefaultSettings is the impl Default for Settings in pdf-writer
// src/chunk.rs.
func DefaultSettings() Settings {
	return Settings{Pretty: true}
}

// refOffset is the tuple (Ref, usize) of the offsets of Chunk in pdf-writer
// src/chunk.rs: the reference of an object and the offset of its start.
type refOffset struct {
	id     Ref
	offset int
}

// Chunk is Chunk in pdf-writer src/chunk.rs.
//
// A builder for a collection of indirect PDF objects.
//
// This type holds written top-level indirect PDF objects. Typically, you
// won't create a colllection yourself, but use the primary chunk of the
// top-level Pdf, which embeds it.
//
// However, sometimes it's useful to be able to create a separate chunk to be
// able to write two things at the same time (which isn't possible with a
// single chunk because of the streaming nature: only one writer can borrow
// it at a time).
//
// Each method of Chunk first finishes the open writers of the chunk, as
// Rust drops them before it lends the chunk again. So the methods take a
// pointer, even where upstream takes &self.
type Chunk struct {
	buf      Buf
	offsets  []refOffset
	settings Settings
}

// NewChunk is Chunk::new in pdf-writer src/chunk.rs.
//
// Create a new chunk with the default settings and buffer capacity
// (currently 1 KB).
func NewChunk() *Chunk {
	return NewChunkWithSettings(DefaultSettings())
}

// NewChunkWithSettings is Chunk::with_settings in pdf-writer src/chunk.rs.
//
// Create a new chunk with the given settings and the default buffer capacity
// (currently 1 KB).
func NewChunkWithSettings(settings Settings) *Chunk {
	return NewChunkWithSettingsAndCapacity(settings, 1204)
}

// NewChunkWithCapacity is Chunk::with_capacity in pdf-writer src/chunk.rs.
//
// Create a new chunk with the default settings and the specified initial
// capacity.
func NewChunkWithCapacity(capacity int) *Chunk {
	return NewChunkWithSettingsAndCapacity(DefaultSettings(), capacity)
}

// NewChunkWithSettingsAndCapacity is Chunk::with_settings_and_capacity in
// pdf-writer src/chunk.rs.
//
// Create a new chunk with the given settings and the specified initial
// buffer capacity.
func NewChunkWithSettingsAndCapacity(settings Settings, capacity int) *Chunk {
	return &Chunk{
		buf:      newBufWithCapacity(capacity),
		offsets:  nil,
		settings: settings,
	}
}

// Len is Chunk::len in pdf-writer src/chunk.rs.
//
// The number of bytes that were written so far.
func (c *Chunk) Len() int {
	return c.buf.Len()
}

// Reserve is Chunk::reserve in pdf-writer src/chunk.rs.
//
// Reserve an additional number of bytes in the buffer.
func (c *Chunk) Reserve(additional int) {
	c.buf.finishFrom(0)
	c.buf.reserve(additional)
}

// AsBytes is Chunk::as_bytes in pdf-writer src/chunk.rs.
//
// The bytes already written so far.
func (c *Chunk) AsBytes() []byte {
	return c.buf.AsSlice()
}

// Extend is Chunk::extend in pdf-writer src/chunk.rs. It also finishes the
// open writers of other.
//
// Add all objects from another chunk to this one.
func (c *Chunk) Extend(other *Chunk) {
	base := c.Len()
	c.buf.extendBuf(&other.buf)
	for _, o := range other.offsets {
		c.offsets = append(c.offsets, refOffset{o.id, base + o.offset})
	}
}

// Refs is Chunk::refs in pdf-writer src/chunk.rs.
//
// An iterator over the references of the top-level objects of the chunk, in
// the order they appear in the chunk.
func (c *Chunk) Refs() iter.Seq[Ref] {
	c.buf.finishFrom(0)
	offsets := c.offsets
	return func(yield func(Ref) bool) {
		for _, o := range offsets {
			if !yield(o.id) {
				return
			}
		}
	}
}

// Limits is Chunk::limits in pdf-writer src/chunk.rs.
//
// Returns the limits of data written into the chunk.
func (c *Chunk) Limits() Limits {
	return c.buf.Limits()
}

// MergeLimits is Chunk::merge_limits in pdf-writer src/chunk.rs.
//
// Merges other limits into this chunk, taking the maximum of each field
// from the chunk's current Limits and other.
//
// This is, for instance, useful when adding a content stream (with limits
// of its own) to the chunk.
func (c *Chunk) MergeLimits(other Limits) {
	c.buf.finishFrom(0)
	c.buf.limits.Merge(other)
}

// Renumber is Chunk::renumber in pdf-writer src/chunk.rs.
//
// Renumbers the IDs of indirect objects and all indirect references in the
// chunk and returns the resulting chunk.
//
// The given closure is called for each object and indirect reference in the
// chunk. When an ID appears multiple times in the chunk (for object and/or
// reference), it will be called multiple times. When assigning new IDs, it
// is up to you to provide a well-defined mapping (it should most probably be
// a pure function so that a specific old ID is always mapped to the same new
// ID).
//
// A simple way to renumber a chunk is to map all old IDs to new consecutive
// IDs. This can be achieved by allocating a new ID for each unique ID we have
// seen and memoizing this mapping in a map.
//
// If a chunk references indirect objects that are not defined within it,
// the closure is still called with those references. Allocating new IDs for
// them will probably not make sense, so it's up to you to either not have
// dangling references or handle them in a way that makes sense for your use
// case.
func (c *Chunk) Renumber(mapping func(Ref) Ref) *Chunk {
	chunk := NewChunkWithCapacity(c.Len())
	c.RenumberInto(chunk, mapping)
	return chunk
}

// RenumberInto is Chunk::renumber_into in pdf-writer src/chunk.rs.
//
// Same as Renumber, but writes the results into an existing target chunk
// instead of creating a new chunk.
func (c *Chunk) RenumberInto(target *Chunk, mapping func(Ref) Ref) {
	target.Reserve(c.Len())
	renumber(c, target, mapping)
}

// Indirect objects and streams.

// Indirect is Chunk::indirect in pdf-writer src/chunk.rs.
//
// Start writing an indirectly referenceable object.
func (c *Chunk) Indirect(id Ref) Obj {
	c.buf.finishFrom(0)
	c.offsets = append(c.offsets, refOffset{id, len(c.buf.inner)})
	return indirectObj(&c.buf, id, c.settings)
}

// Stream is Chunk::stream in pdf-writer src/chunk.rs.
//
// Start writing an indirectly referenceable stream.
//
// The stream data and the /Length field are written automatically. You can
// add additional key-value pairs to the stream dictionary with the returned
// stream writer.
//
// You can use this function together with a Content stream builder to
// provide a page's contents.
//
// This package does not do any compression for you. If you want to compress
// a stream, you have to pass already compressed data into this function and
// specify the appropriate filter in the stream dictionary.
//
// For all the specialized stream functions, it works the same way: You can
// pass compressed data and specify a filter.
//
// Panics if the stream length exceeds math.MaxInt32.
func (c *Chunk) Stream(id Ref, data []byte) *Stream {
	return startStream(c.Indirect(id), data)
}

// Tree data structures.

// ChunkNameTree is Chunk::name_tree in pdf-writer src/chunk.rs. Go has no
// type parameters on methods, so it is a function.
//
// Start writing a name tree node.
func ChunkNameTree[T Primitive](c *Chunk, id Ref) *NameTree[T] {
	return Start[NameTree[T]](c.Indirect(id))
}

// ChunkNumberTree is Chunk::number_tree in pdf-writer src/chunk.rs. Go has
// no type parameters on methods, so it is a function.
//
// Start writing a number tree node.
func ChunkNumberTree[T Primitive](c *Chunk, id Ref) *NumberTree[T] {
	return Start[NumberTree[T]](c.Indirect(id))
}

// Clone is the derived impl Clone for Chunk in pdf-writer src/chunk.rs. It
// finishes the open writers first, and the copy has no open writer.
func (c *Chunk) Clone() *Chunk {
	c.buf.finishFrom(0)
	return &Chunk{
		buf: Buf{
			inner:  slices.Clone(c.buf.inner),
			limits: c.buf.limits,
		},
		offsets:  slices.Clone(c.offsets),
		settings: c.settings,
	}
}

// String is the impl Debug for Chunk in pdf-writer src/chunk.rs.
func (c *Chunk) String() string {
	return "Chunk(..)"
}
