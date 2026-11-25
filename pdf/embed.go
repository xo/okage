// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"strings"

	"github.com/xo/okage/internal/pdfwriter"
)

// This file ports krilla src/interchange/embed.rs (D99). Document.EmbedFile
// is Document::embed_file of src/document.rs, and
// serializeContext.embedFile is SerializeContext::embed_file of
// src/serialize.rs.
//
// Embedding attachments to a PDF file.

// embedError is EmbedError in krilla src/interchange/embed.rs. Only the
// validation error EmbeddedFile carries it, so it is not public.
//
// An error while embedding the file.
type embedError uint8

// The cases of EmbedError.
const (
	// embedErrorExistence is EmbedError::Existence. The selected standard
	// does not support embedding files.
	embedErrorExistence embedError = iota
	// embedErrorMissingDate is EmbedError::MissingDate. The document doesn't
	// contain a modification date, which is required for embedded files in
	// some export modes.
	embedErrorMissingDate
	// embedErrorMissingDescription is EmbedError::MissingDescription. The
	// embedded file is missing a human-readable description.
	embedErrorMissingDescription
	// embedErrorMissingMIMEType is EmbedError::MissingMimeType. The mime type
	// of the embedded file is missing.
	embedErrorMissingMIMEType
)

// EmbeddedFile is EmbeddedFile in krilla src/interchange/embed.rs. A field
// of upstream that is an Option is a pointer here, and nil is None.
//
// An embedded file.
type EmbeddedFile struct {
	// Path is the name of the embedded file.
	Path string
	// MIMEType is the mime type of the embedded file.
	MIMEType *MIMEType
	// Description is a description of the embedded file.
	Description *string
	// AssociationKind is the association kind of the embedded file.
	AssociationKind AssociationKind
	// Data is the raw data of the embedded file.
	Data []byte
	// ModificationDate is the modification date of the embedded file.
	ModificationDate *DateTime
	// Compress says whether the embedded file should be compressed
	// (recommended to turn off if the original file already has
	// compression). If nil, krilla will use its own logic for determining
	// whether to compress the file or not.
	Compress *bool
	// Location is the location of the embedded file. Zero is None.
	Location Location
}

// typeID names EmbeddedFile in the hashes of the cache.
func (*EmbeddedFile) typeID() string { return "EmbeddedFile" }

// hash is the derived Hash of EmbeddedFile in krilla
// src/interchange/embed.rs. Each Option writes its case first, then its
// value.
func (f *EmbeddedFile) hash(h *hasher) {
	h.str(f.Path)
	h.bool(f.MIMEType != nil)
	if f.MIMEType != nil {
		h.str(f.MIMEType.s)
	}
	h.bool(f.Description != nil)
	if f.Description != nil {
		h.str(*f.Description)
	}
	h.usize(int(f.AssociationKind))
	h.bytes(f.Data)
	h.bool(f.ModificationDate != nil)
	if d := f.ModificationDate; d != nil {
		h.u16(d.year)
		for _, v := range []struct {
			v  uint8
			ok bool
		}{{d.month, d.hasMonth}, {d.day, d.hasDay}, {d.hour, d.hasHour}, {d.minute, d.hasMinute}, {d.second, d.hasSecond}} {
			h.bool(v.ok)
			h.u8(v.v)
		}
		h.bool(d.hasUTCOffsetHour)
		h.u8(uint8(d.utcOffsetHour))
		h.u8(d.utcOffsetMinute)
	}
	h.bool(f.Compress != nil)
	if f.Compress != nil {
		h.bool(*f.Compress)
	}
	h.u64(uint64(f.Location))
}

// serialize is Cacheable::serialize for EmbeddedFile in krilla
// src/interchange/embed.rs.
func (f *EmbeddedFile) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	sc.registerValidationError(validationError{
		kind:     validationEmbeddedFile,
		embed:    embedErrorExistence,
		location: f.Location,
	})

	chunk := cc.nonStream.embeddedFiles
	streamChunk := sc.newChunk()
	streamRef := sc.newRef()

	var builder *filterStreamBuilder
	switch {
	case f.Compress != nil && *f.Compress:
		builder = newFilterStreamFromBinaryData(f.Data)
	case f.Compress != nil:
		builder = newFilterStreamFromUncompressed(f.Data)
	default:
		builder = newFilterStreamAutoCompressed(f.Data)
	}
	fileStream := builder.finish(sc.serializeSettings())

	embeddedFileStream := streamChunk.EmbeddedFile(streamRef, fileStream.encodedData())
	fileStream.writeFilters(embeddedFileStream.Dict)

	if f.MIMEType != nil {
		embeddedFileStream.Subtype(pdfwriter.Name(f.MIMEType.s))
	} else {
		sc.registerValidationError(validationError{
			kind:     validationEmbeddedFile,
			embed:    embedErrorMissingMIMEType,
			location: f.Location,
		})
	}

	params := embeddedFileStream.Params()
	params.Size(int32(len(f.Data)))

	if f.ModificationDate != nil {
		params.ModificationDate(pdfDate(*f.ModificationDate))
	} else {
		sc.registerValidationError(validationError{
			kind:     validationEmbeddedFile,
			embed:    embedErrorMissingDate,
			location: f.Location,
		})
	}

	params.Finish()
	embeddedFileStream.Finish()

	fileSpec := chunk.FileSpec(rootRef)
	fileSpec.Path(pdfwriter.Str(f.Path))

	if sc.serializeSettings().pdfVersion() >= PDF17 {
		fileSpec.UnicFile(pdfwriter.TextStr(f.Path))
	}

	ef := fileSpec.Insert(pdfwriter.Name("EF")).Dict()
	ef.Pair(pdfwriter.Name("F"), streamRef)

	if sc.serializeSettings().pdfVersion() >= PDF17 {
		ef.Pair(pdfwriter.Name("UF"), streamRef)
	}

	ef.Finish()

	if sc.serializeSettings().supportsAssociatedFiles() {
		fileSpec.AssociationKind(f.AssociationKind.toPDF(sc.serializeSettings().pdfVersion()))
	}

	if f.Description != nil {
		fileSpec.Description(pdfwriter.TextStr(*f.Description))
	} else {
		sc.registerValidationError(validationError{
			kind:     validationEmbeddedFile,
			embed:    embedErrorMissingDescription,
			location: f.Location,
		})
	}

	fileSpec.Finish()
	cc.streams.embeddedFiles = append(cc.streams.embeddedFiles, streamChunk)
}

// AssociationKind is AssociationKind in krilla src/interchange/embed.rs.
//
// How an embedded file relates to the PDF document it is embedded in.
type AssociationKind uint8

// The values of AssociationKind.
const (
	// AssociationKindSource means that the PDF document was created from
	// this source file.
	AssociationKindSource AssociationKind = iota
	// AssociationKindData means that this file was used to derive a visual
	// presentation in the PDF.
	AssociationKindData
	// AssociationKindAlternative is an alternative representation of this
	// document.
	AssociationKindAlternative
	// AssociationKindSupplement is additional resources for this document.
	AssociationKindSupplement
	// AssociationKindSchema is a machine-readable schema. PDF 2.0+, written
	// as AssociationKindUnspecified in earlier versions.
	AssociationKindSchema
	// AssociationKindUnspecified means that there is no clear relationship
	// or it is not known.
	AssociationKindUnspecified
)

// toPDF is AssociationKind::to_pdf in krilla src/interchange/embed.rs. A
// value that is not one of the constants is unspecified.
func (k AssociationKind) toPDF(version PDFVersion) pdfwriter.AssociationKind {
	switch k {
	case AssociationKindSchema:
		if version < PDF20 {
			return pdfwriter.AssociationKindUnspecified
		}
		return pdfwriter.AssociationKindSchema
	case AssociationKindSource:
		return pdfwriter.AssociationKindSource
	case AssociationKindData:
		return pdfwriter.AssociationKindData
	case AssociationKindAlternative:
		return pdfwriter.AssociationKindAlternative
	case AssociationKindSupplement:
		return pdfwriter.AssociationKindSupplement
	}
	return pdfwriter.AssociationKindUnspecified
}

// MIMEType is MimeType in krilla src/interchange/embed.rs.
//
// A mime type.
type MIMEType struct {
	s string
}

// NewMIMEType is MimeType::new in krilla src/interchange/embed.rs. It
// returns false where upstream returns None.
//
// Create a new mime type.
//
// Returns false if the mime type is invalid.
func NewMIMEType(mimeType string) (*MIMEType, bool) {
	if !validMIMEType(mimeType) {
		return nil, false
	}
	return &MIMEType{s: mimeType}, true
}

// String returns the mime type.
func (m *MIMEType) String() string {
	return m.s
}

// validMIMEType is valid_mime_type in krilla src/interchange/embed.rs.
func validMIMEType(mimeType string) bool {
	parts := strings.Split(mimeType, "/")
	if len(parts) != 2 {
		return false
	}
	return validMIMEPart(parts[0]) && validMIMEPart(parts[1])
}

// validMIMEPart is valid_mime_part in krilla src/interchange/embed.rs.
func validMIMEPart(part string) bool {
	if part == "" {
		return false
	}
	for _, c := range part {
		alnum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !alnum && c != '-' && c != '+' && c != '.' {
			return false
		}
	}
	return true
}

// EmbedFile is Document::embed_file in krilla src/document.rs. It returns
// false where upstream returns None.
//
// Embed a new file in the PDF document.
//
// Returns false if the file couldn't be embedded because a file with the
// same name has already been embedded.
func (d *Document) EmbedFile(file EmbeddedFile) bool {
	return d.sc.embedFile(d.cc, &file)
}

// embedFile is SerializeContext::embed_file in krilla src/serialize.rs.
func (sc *serializeContext) embedFile(cc *chunkContainer, file *EmbeddedFile) bool {
	name := file.Path
	ref := sc.registerCacheable(cc, file)
	if sc.globalObjects.embeddedFiles == nil {
		sc.globalObjects.embeddedFiles = map[string]pdfwriter.Ref{}
	}
	if _, ok := sc.globalObjects.embeddedFiles[name]; ok {
		sc.globalObjects.embeddedFiles[name] = ref
		return false
	}
	sc.globalObjects.embeddedFiles[name] = ref
	return true
}
