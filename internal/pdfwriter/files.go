// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import "fmt"

// This file ports the part of pdf-writer src/files.rs that the port needs:
// FileSpec and AssociationKind, which xobject.go uses, with
// Chunk::file_spec. The writers EmbeddedFile and EmbeddingParams, and
// Chunk::embedded_file, write the embedded files, which D62 leaves out of
// the base port of krilla. They have no port.

// FileSpec is FileSpec in pdf-writer src/files.rs.
//
// Writer for a file specification dictionary.
//
// This struct is created by Annotation.FileSpec, Reference.FileSpec,
// MediaClip.Data, and Action.FileSpec.
type FileSpec struct {
	*Dict
}

// start is the macro writer! for FileSpec in pdf-writer src/files.rs.
func (f *FileSpec) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("Type"), Name("Filespec"))
}

// FileSystem is FileSpec::file_system in pdf-writer src/files.rs.
//
// Write the /FS attribute to set the file system this entry relates to. If
// you set the system argument to Name("URL"), this becomes an URL
// specification.
func (f *FileSpec) FileSystem(system Name) *FileSpec {
	f.Pair(Name("FS"), system)
	return f
}

// Path is FileSpec::path in pdf-writer src/files.rs.
//
// Write the /F attribute to set the file path. Directories are indicated by
// /, independent of the platform.
func (f *FileSpec) Path(path Str) *FileSpec {
	f.Pair(Name("F"), path)
	return f
}

// UnicFile is FileSpec::unic_file in pdf-writer src/files.rs.
//
// Write the /UF attribute to set a Unicode-compatible path. Directories are
// indicated by /, independent of the platform. PDF 1.7+.
func (f *FileSpec) UnicFile(path TextStr) *FileSpec {
	f.Pair(Name("UF"), path)
	return f
}

// Volatile is FileSpec::volatile in pdf-writer src/files.rs.
//
// Write the /V attribute to indicate whether not to cache the file.
func (f *FileSpec) Volatile(dontCache bool) *FileSpec {
	f.Pair(Name("V"), Bool(dontCache))
	return f
}

// Description is FileSpec::description in pdf-writer src/files.rs.
//
// Write the /Desc attribute to set a file description. PDF 1.6+.
func (f *FileSpec) Description(desc TextStrLike) *FileSpec {
	f.Pair(Name("Desc"), desc)
	return f
}

// EmbeddedFile is FileSpec::embedded_file in pdf-writer src/files.rs.
//
// Write the /EF attribute to reference an embedded file. PDF 1.3+.
//
// This only sets an embedded file for the F attribute corresponding to the
// Path method. If you want to set the same embedded file for the UF
// attribute, also call EmbeddedFileWithUnicode instead.
//
// Note that this key is forbidden in PDF/A-1 and restricted in PDF/A-2 and
// PDF/A-4.
func (f *FileSpec) EmbeddedFile(id Ref) *FileSpec {
	f.Insert(Name("EF")).Dict().Pair(Name("F"), id)
	return f
}

// EmbeddedFileWithUnicode is FileSpec::embedded_file_with_unicode in
// pdf-writer src/files.rs.
//
// Write the /EF attribute to reference an embedded file for the legacy and
// Unicode-compatible file path. PDF 1.7+.
//
// Note that this key is forbidden in PDF/A-1 and restricted in PDF/A-2 an
// PDF/A-4.
func (f *FileSpec) EmbeddedFileWithUnicode(id Ref) *FileSpec {
	f.Insert(Name("EF")).
		Dict().
		Pair(Name("F"), id).
		Pair(Name("UF"), id)
	return f
}

// AssociationKind is FileSpec::association_kind in pdf-writer src/files.rs.
//
// How this file relates to the PDF document it is embedded in. PDF/A-3 and
// PDF/A-4f.
func (f *FileSpec) AssociationKind(kind AssociationKind) *FileSpec {
	f.Pair(Name("AFRelationship"), kind.toName())
	return f
}

// AssociationKind is AssociationKind in pdf-writer src/files.rs. The default
// of upstream is AssociationKindUnspecified, which is not the zero value.
//
// How an embedded file relates to the PDF document it is embedded in. PDF
// 1.7 with PDF/A-3, PDF 2.0+ (including PDF/A-4f).
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
	// AssociationKindEncryptedPayload is an encrypted file. PDF 2.0+.
	AssociationKindEncryptedPayload
	// AssociationKindFormData is data associated with the AcroForm. PDF
	// 2.0+.
	AssociationKindFormData
	// AssociationKindSchema is a machine-readable schema. PDF 2.0+.
	AssociationKindSchema
	// AssociationKindUnspecified means that there is no clear relationship
	// or it is not known.
	AssociationKindUnspecified
)

// associationKindNames holds the PDF names of the values of
// AssociationKind, in order.
var associationKindNames = [...]Name{
	"Source",
	"Data",
	"Alternative",
	"Supplement",
	"EncryptedPayload",
	"FormData",
	"Schema",
	"Unspecified",
}

// String returns the PDF name of the kind, without the slash.
func (k AssociationKind) String() string {
	return string(k.toName())
}

// toName is AssociationKind::to_name in pdf-writer src/files.rs.
func (k AssociationKind) toName() Name {
	if int(k) < len(associationKindNames) {
		return associationKindNames[k]
	}
	panic(fmt.Sprintf("pdfwriter: the association kind %d is not known", uint8(k)))
}

// FileSpec is Chunk::file_spec in pdf-writer src/chunk.rs.
//
// Start writing a file specification dictionary.
func (c *Chunk) FileSpec(id Ref) *FileSpec {
	return Start[FileSpec](c.Indirect(id))
}
