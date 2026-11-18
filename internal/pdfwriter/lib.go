// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"math/bits"
	"slices"
	"strconv"
)

// This file ports pdf-writer src/lib.rs, without the package comment, which
// is in doc.go. Pdf::catalog and Pdf::document_info return the writers of
// src/structure.rs, so they are in the file of that port. They call
// startCatalog and startDocumentInfo, which hold the code of lib.rs.

// Pdf is Pdf in pdf-writer src/lib.rs. It embeds its Chunk, as the impl
// Deref of upstream gives the methods of the chunk.
//
// A builder for a PDF file.
//
// This type constructs a PDF file in-memory. Aside from a few specific
// structures, a PDF file mostly consists of indirect objects. For more
// flexibility, you can write these objects either directly into a Pdf or
// into a Chunk, which you can add to the Pdf (or another chunk) later.
// Therefore, most writing methods are exposed on the chunk type, which this
// type embeds.
type Pdf struct {
	Chunk

	trailerData trailerData
}

// NewPdf is Pdf::new in pdf-writer src/lib.rs.
//
// Create a new PDF with the default settings and buffer capacity (currently
// 8 KB).
func NewPdf() *Pdf {
	return NewPdfWithSettings(DefaultSettings())
}

// NewPdfWithSettings is Pdf::with_settings in pdf-writer src/lib.rs.
//
// Create a new PDF with the given settings and the default buffer capacity
// (currently 8 KB).
func NewPdfWithSettings(settings Settings) *Pdf {
	return NewPdfWithSettingsAndCapacity(settings, 8*1024)
}

// NewPdfWithCapacity is Pdf::with_capacity in pdf-writer src/lib.rs.
//
// Create a new PDF with the default settings and the specified initial
// buffer capacity.
func NewPdfWithCapacity(capacity int) *Pdf {
	return NewPdfWithSettingsAndCapacity(DefaultSettings(), capacity)
}

// NewPdfWithSettingsAndCapacity is Pdf::with_settings_and_capacity in
// pdf-writer src/lib.rs.
//
// Create a new PDF with the given settings and the specified initial buffer
// capacity.
func NewPdfWithSettingsAndCapacity(settings Settings, capacity int) *Pdf {
	chunk := NewChunkWithSettingsAndCapacity(settings, capacity)
	chunk.buf.extendString("%PDF-1.7\n%\x80\x80\x80\x80\n\n")
	return &Pdf{Chunk: *chunk}
}

// SetBinaryMarker is Pdf::set_binary_marker in pdf-writer src/lib.rs.
//
// Set the binary marker in the header of the PDF.
//
// This can be useful if you want to ensure that your PDF consists of only
// ASCII characters, as this is not the case by default.
//
// Default value: \x80\x80\x80\x80.
func (p *Pdf) SetBinaryMarker(marker [4]byte) {
	p.buf.finishFrom(0)
	copy(p.buf.inner[10:14], marker[:])
}

// SetVersion is Pdf::set_version in pdf-writer src/lib.rs.
//
// Set the PDF version.
//
// The version is not semantically important to the crate, but must be
// present in the output document.
//
// Default value: 1.7.
func (p *Pdf) SetVersion(major, minor uint8) {
	p.buf.finishFrom(0)
	if major < 10 {
		p.buf.inner[5] = '0' + major
	}
	if minor < 10 {
		p.buf.inner[7] = '0' + minor
	}
}

// SetFileID is Pdf::set_file_id in pdf-writer src/lib.rs. The two strings of
// the identifier are the tuple of upstream.
//
// Set the file identifier for the document.
//
// The file identifier is a pair of two byte strings that shall be used to
// uniquely identify a particular file. The first string should always stay
// the same for a document, the second should change for each revision. It
// is optional, but recommended. In PDF/A, this is required. PDF 1.1+.
func (p *Pdf) SetFileID(first, second []byte) {
	p.buf.finishFrom(0)
	p.trailerData.fileID = &[2][]byte{first, second}
}

// startCatalog is the code of Pdf::catalog in pdf-writer src/lib.rs. It
// returns the object that the writer Catalog starts on.
//
// Start writing the document catalog. Required.
//
// This will also register the document catalog with the file trailer,
// meaning that you don't need to provide the given id anywhere else.
func (p *Pdf) startCatalog(id Ref) Obj {
	p.buf.finishFrom(0)
	p.trailerData.catalogID = &id
	return p.Indirect(id)
}

// startDocumentInfo is the code of Pdf::document_info in pdf-writer
// src/lib.rs. It returns the object that the writer DocumentInfo starts on.
//
// Start writing the document information.
//
// This will also register the document information dictionary with the
// file trailer, meaning that you don't need to provide the given id
// anywhere else.
func (p *Pdf) startDocumentInfo(id Ref) Obj {
	p.buf.finishFrom(0)
	p.trailerData.infoID = &id
	return p.Indirect(id)
}

// Finish is Pdf::finish in pdf-writer src/lib.rs. Upstream takes the Pdf by
// value. Do not use the Pdf after this call.
//
// Write the cross-reference table and file trailer and return the
// underlying buffer.
//
// Panics if any indirect reference id was used twice.
func (p *Pdf) Finish() []byte {
	p.buf.finishFrom(0)
	buf, offsets, settings := &p.buf, p.offsets, p.settings
	td := p.trailerData
	xrefOffset := len(buf.inner)

	writer := &plainXRefWriter{buf: buf}
	xrefLen := writeOffsets(offsets, writer)

	// Write the trailer dictionary.
	buf.extendString("trailer\n")
	trailer := directObj(buf, 0, settings, false).Dict()
	td.writeIntoDict(trailer, xrefLen)
	trailer.Finish()

	return finishTrailer(buf, xrefOffset, []byte("\n"))
}

// FinishWithXRefStream is Pdf::finish_with_xref_stream in pdf-writer
// src/lib.rs. Do not use the Pdf after this call.
//
// Write the cross-reference stream and file trailer and return the
// underlying buffer. This method is functionally the same as Finish, the
// difference being that the cross-reference information is written as a
// cross-reference stream instead of a cross-reference table.
// Cross-reference streams usually allow for smaller file sizes since they
// can also be compressed (see FinishWithXRefStreamAndFilter), but are only
// available from PDF 1.5 onwards. It is also necessary to call this method
// instead of Finish in case object stream are used anywhere in the document
// since normal xref tables do not support object streams.
//
// xrefID will be the object identifier used for the cross-reference stream.
// As in other cases, the identifier needs to be unique throughout the whole
// document.
//
// Panics if any indirect reference id was used twice.
func (p *Pdf) FinishWithXRefStream(xrefID Ref) []byte {
	return p.finishWithXRefStreamInner(xrefID, func(buf []byte) ([]byte, *XRefFilter) {
		return buf, nil
	})
}

// FinishWithXRefStreamAndFilter is
// Pdf::finish_with_xref_stream_and_filter in pdf-writer src/lib.rs. Do not
// use the Pdf after this call.
//
// Write the cross-reference stream and file trailer and return the
// underlying buffer.
//
// This method is equivalent to FinishWithXRefStream, except that it allows
// you to apply one or multiple filters to the xref stream via the filter
// closure. The input of the closure will be the raw content of the xref
// stream, and the output should be the filtered data as well as a single
// filter or a list of filters that need to be applied to unfilter the data
// in the correct order.
func (p *Pdf) FinishWithXRefStreamAndFilter(xrefID Ref, filter func([]byte) ([]byte, XRefFilter)) []byte {
	return p.finishWithXRefStreamInner(xrefID, func(buf []byte) ([]byte, *XRefFilter) {
		xrefData, f := filter(buf)
		return xrefData, &f
	})
}

// finishWithXRefStreamInner is Pdf::finish_with_xref_stream_inner in
// pdf-writer src/lib.rs.
func (p *Pdf) finishWithXRefStreamInner(xrefID Ref, filter func([]byte) ([]byte, *XRefFilter)) []byte {
	p.buf.finishFrom(0)
	buf, offsets, settings := &p.buf, p.offsets, p.settings
	td := p.trailerData

	// Include the reference of the xref stream in the offsets as well!
	xrefOffset := len(buf.inner)
	offsets = append(offsets, refOffset{xrefID, xrefOffset})
	fieldWidth := determineFieldWidth(uint(xrefOffset))

	writer := newXRefStreamWriter(fieldWidth)
	xrefLen := writeOffsets(offsets, writer)

	xrefData, f := filter(writer.buf)

	stream := startStream(indirectObj(buf, xrefID, settings), xrefData)

	stream.Pair(Name("Type"), Name("XRef"))

	if f != nil {
		if !f.multiple {
			stream.Filter(f.filters[0])
		} else {
			arr := stream.Insert(Name("Filter")).Array()

			for _, filter := range f.filters {
				arr.Item(filter.toName())
			}
		}
	}

	td.writeIntoDict(stream.Dict, xrefLen)

	stream.Insert(Name("W")).
		Array().
		Item(Int(1)).
		Item(Int(fieldWidth)).
		Item(Int(2))

	stream.Finish()

	return finishTrailer(buf, xrefOffset, nil)
}

// XRefFilter is XRefFilter in pdf-writer src/lib.rs. It is an enum with the
// cases Single and Multiple, which XRefFilterSingle and XRefFilterMultiple
// make (D27).
//
// The filters used for the xref stream.
type XRefFilter struct {
	filters  []Filter
	multiple bool
}

// XRefFilterSingle is XRefFilter::Single in pdf-writer src/lib.rs.
//
// A single filter.
func XRefFilterSingle(filter Filter) XRefFilter {
	return XRefFilter{filters: []Filter{filter}}
}

// XRefFilterMultiple is XRefFilter::Multiple in pdf-writer src/lib.rs.
//
// An array of filters.
func XRefFilterMultiple(filters ...Filter) XRefFilter {
	return XRefFilter{filters: filters, multiple: true}
}

// finishTrailer is finish_trailer in pdf-writer src/lib.rs.
func finishTrailer(buf *Buf, xrefOffset int, pad []byte) []byte {
	buf.extend(pad)
	// Write startxref pointing to the xref stream
	buf.extendString("startxref\n")
	buf.inner = strconv.AppendInt(buf.inner, int64(xrefOffset), 10)

	// Write EOF marker
	buf.extendString("\n%%EOF")
	return buf.IntoVec()
}

// writeOffsets is write_offsets in pdf-writer src/lib.rs.
func writeOffsets(offsets []refOffset, writer xrefWriter) int32 {
	slices.SortFunc(offsets, func(a, b refOffset) int {
		if c := cmp.Compare(a.id.Get(), b.id.Get()); c != 0 {
			return c
		}
		return cmp.Compare(a.offset, b.offset)
	})

	xrefLen := int32(1)
	if len(offsets) > 0 {
		xrefLen += offsets[len(offsets)-1].id.Get()
	}
	writer.prologue(xrefLen)

	if len(offsets) == 0 {
		writer.writeFreeEntry(0, 65535)
	}

	var written int32
	for i, o := range offsets {
		objectID, offset := o.id, o.offset
		if written > objectID.Get() {
			panic(fmt.Sprintf("duplicate indirect reference id: %d", objectID.Get()))
		}

		// Fill in free list.
		start := written
		for freeID := start; freeID < objectID.Get(); freeID++ {
			next := freeID + 1
			if next == objectID.Get() {
				// Find next free id.
				for _, used := range offsets[i:] {
					if next < used.id.Get() {
						break
					}
					next = used.id.Get() + 1
				}
			}

			gen := uint16(0)
			if freeID == 0 {
				gen = 65535
			}
			writer.writeFreeEntry(uint(next%xrefLen), gen)
			written++
		}

		writer.writeOccupiedEntry(uint(offset), 0)
		written++
	}

	return xrefLen
}

// String is the impl Debug for Pdf in pdf-writer src/lib.rs.
func (p *Pdf) String() string {
	return "Pdf(..)"
}

// trailerData is TrailerData in pdf-writer src/lib.rs. A nil field is None.
type trailerData struct {
	catalogID *Ref
	infoID    *Ref
	fileID    *[2][]byte
}

// writeIntoDict is TrailerData::write_into_dict in pdf-writer src/lib.rs.
func (td trailerData) writeIntoDict(dict *Dict, xrefLen int32) {
	dict.Pair(Name("Size"), Int(xrefLen))

	if td.catalogID != nil {
		dict.Pair(Name("Root"), *td.catalogID)
	}

	if td.infoID != nil {
		dict.Pair(Name("Info"), *td.infoID)
	}

	if td.fileID != nil {
		ids := dict.Insert(Name("ID")).Array()
		ids.Item(Str(td.fileID[0]))
		ids.Item(Str(td.fileID[1]))
	}
}

// xrefWriter is the trait XRefWriter in pdf-writer src/lib.rs.
type xrefWriter interface {
	prologue(xrefLen int32)
	writeFreeEntry(offset uint, genNumber uint16)
	writeOccupiedEntry(offset uint, genNumber uint16)
}

// xrefStreamWriter is XRefStreamWriter in pdf-writer src/lib.rs.
type xrefStreamWriter struct {
	buf        []byte
	fieldWidth uint32
}

// newXRefStreamWriter is XRefStreamWriter::new in pdf-writer src/lib.rs.
func newXRefStreamWriter(fieldWidth uint32) *xrefStreamWriter {
	return &xrefStreamWriter{buf: nil, fieldWidth: fieldWidth}
}

// write is XRefStreamWriter::write in pdf-writer src/lib.rs.
func (w *xrefStreamWriter) write(entryType uint8, offset uint, genNumber uint16) {
	var offsetBytes [8]byte
	binary.BigEndian.PutUint64(offsetBytes[:], uint64(offset))

	w.buf = append(w.buf, entryType)
	w.buf = append(w.buf, offsetBytes[len(offsetBytes)-int(w.fieldWidth):]...)
	w.buf = binary.BigEndian.AppendUint16(w.buf, genNumber)
}

func (w *xrefStreamWriter) prologue(int32) {}

func (w *xrefStreamWriter) writeFreeEntry(offset uint, genNumber uint16) {
	w.write(0, offset, genNumber)
}

func (w *xrefStreamWriter) writeOccupiedEntry(offset uint, genNumber uint16) {
	w.write(1, offset, genNumber)
}

// plainXRefWriter is PlainXRefWriter in pdf-writer src/lib.rs.
type plainXRefWriter struct {
	buf *Buf
}

func (w *plainXRefWriter) prologue(xrefLen int32) {
	w.buf.extendString("xref\n0 ")
	w.buf.pushInt(xrefLen)
	w.buf.push('\n')
}

func (w *plainXRefWriter) writeFreeEntry(offset uint, genNumber uint16) {
	w.buf.inner = fmt.Appendf(w.buf.inner, "%010d %05d f\r\n", offset, genNumber)
}

func (w *plainXRefWriter) writeOccupiedEntry(offset uint, genNumber uint16) {
	w.buf.inner = fmt.Appendf(w.buf.inner, "%010d %05d n\r\n", offset, genNumber)
}

// determineFieldWidth is determine_field_width in pdf-writer src/lib.rs.
func determineFieldWidth(offset uint) uint32 {
	return (uint32(bits.Len(offset)) + 7) / 8
}
