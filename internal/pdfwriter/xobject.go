// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import "fmt"

// This file ports pdf-writer src/xobject.rs, with the methods of Chunk for
// XObjects of src/chunk.rs.

// ImageXObject is ImageXObject in pdf-writer src/xobject.rs.
//
// Writer for an image XObject stream.
//
// This struct is created by Chunk.ImageXObject.
type ImageXObject struct {
	*Stream
}

// startImageXObject is ImageXObject::start in pdf-writer src/xobject.rs.
//
// Create a new image stream writer.
func startImageXObject(stream *Stream) *ImageXObject {
	stream.Pair(Name("Type"), Name("XObject"))
	stream.Pair(Name("Subtype"), Name("Image"))
	return &ImageXObject{Stream: stream}
}

// Width is ImageXObject::width in pdf-writer src/xobject.rs.
//
// Write the /Width attribute.
func (x *ImageXObject) Width(width int32) *ImageXObject {
	x.Pair(Name("Width"), Int(width))
	return x
}

// Height is ImageXObject::height in pdf-writer src/xobject.rs.
//
// Write the /Height attribute.
func (x *ImageXObject) Height(height int32) *ImageXObject {
	x.Pair(Name("Height"), Int(height))
	return x
}

// ColorSpaceName is ImageXObject::color_space_name in pdf-writer
// src/xobject.rs.
//
// Write the /ColorSpace attribute as a name from the resource dictionary.
//
// Required for all images except if using the JPXDecode filter. If this is
// an image soft mask, the color space must be DeviceGray. Must not be
// Pattern.
func (x *ImageXObject) ColorSpaceName(name Name) *ImageXObject {
	x.Pair(Name("ColorSpace"), name)
	return x
}

// BitsPerComponent is ImageXObject::bits_per_component in pdf-writer
// src/xobject.rs.
//
// Write the /BitsPerComponent attribute. Required.
//
// Required for all images except if using the JPXDecode filter.
func (x *ImageXObject) BitsPerComponent(bits int32) *ImageXObject {
	x.Pair(Name("BitsPerComponent"), Int(bits))
	return x
}

// ImageMask is ImageXObject::image_mask in pdf-writer src/xobject.rs.
//
// Write the /ImageMask attribute to set whether this image is a clipping
// mask. If so, the /BitsPerComponent must be 1 and /Mask and /ColorSpace
// attributes shall be left undefined.
func (x *ImageXObject) ImageMask(mask bool) *ImageXObject {
	x.Pair(Name("ImageMask"), Bool(mask))
	return x
}

// ColorMask is ImageXObject::color_mask in pdf-writer src/xobject.rs.
//
// Write the /Mask attribute to set a color key mask. The iterable color
// argument must contain a range of colors (minimum and maximum) for each
// channel that shall be masked out. PDF 1.3+.
func (x *ImageXObject) ColorMask(colors ...int32) *ImageXObject {
	a := NewTypedArray[Int](x.Insert(Name("Mask")).Array())
	for _, c := range colors {
		a.Item(Int(c))
	}
	return x
}

// StencilMask is ImageXObject::stencil_mask in pdf-writer src/xobject.rs.
//
// Write the /Mask attribute to set another image as the stencil mask of this
// image.
func (x *ImageXObject) StencilMask(mask Ref) *ImageXObject {
	x.Pair(Name("Mask"), mask)
	return x
}

// Decode is ImageXObject::decode in pdf-writer src/xobject.rs.
//
// Write the /Decode attribute to set the decoding of the image sample colors
// to the specified color space. Must have twice the amount of elements as
// the color space.
func (x *ImageXObject) Decode(decode ...float32) *ImageXObject {
	itemsF32(x.Insert(Name("Decode")).Array(), decode)
	return x
}

// Interpolate is ImageXObject::interpolate in pdf-writer src/xobject.rs.
//
// Write the /Interpolate attribute.
//
// Must be false or unset for PDF/A files.
func (x *ImageXObject) Interpolate(interpolate bool) *ImageXObject {
	x.Pair(Name("Interpolate"), Bool(interpolate))
	return x
}

// Alternates is ImageXObject::alternates in pdf-writer src/xobject.rs.
//
// Write the /Alternates attribute. PDF 1.3+.
//
// Images that may replace this image. The order is not relevant.
//
// Note that this key is forbidden in PDF/A.
func (x *ImageXObject) Alternates(alternates ...Ref) *ImageXObject {
	NewTypedArray[Ref](x.Insert(Name("Alternates")).Array()).Items(alternates...)
	return x
}

// SMask is ImageXObject::s_mask in pdf-writer src/xobject.rs.
//
// Start writing the /SMask attribute. PDF 1.4+.
//
// Must not be used if this image already is an image soft mask.
//
// Note that this key is forbidden in PDF/A-1.
func (x *ImageXObject) SMask(xObject Ref) *ImageXObject {
	x.Pair(Name("SMask"), xObject)
	return x
}

// SMaskInData is ImageXObject::s_mask_in_data in pdf-writer src/xobject.rs.
//
// Write the /SMaskInData attribute. PDF 1.5+.
//
// May only be used for images that use the JPXDecode filter. If set to
// something other than Ignore, the SMask attribute must not be used.
func (x *ImageXObject) SMaskInData(mode SMaskInData) *ImageXObject {
	x.Pair(Name("SMaskInData"), Int(mode.toInt()))
	return x
}

// StructParent is ImageXObject::struct_parent in pdf-writer src/xobject.rs.
//
// Write the /StructParent attribute to indicate the structure tree element
// this image belongs to. PDF 1.3+.
func (x *ImageXObject) StructParent(key int32) *ImageXObject {
	x.Pair(Name("StructParent"), Int(key))
	return x
}

// Matte is ImageXObject::matte in pdf-writer src/xobject.rs.
//
// Write the /Matte attribute for image soft masks. PDF 1.4+.
//
// This shall be the matte color of the parent image encoded in its color
// space.
func (x *ImageXObject) Matte(color ...float32) *ImageXObject {
	itemsF32(x.Insert(Name("Matte")).Array(), color)
	return x
}

// Metadata is ImageXObject::metadata in pdf-writer src/xobject.rs.
//
// Write the /Metadata attribute to specify the image's metadata. PDF 1.4+.
//
// The reference shall point to a metadata stream.
func (x *ImageXObject) Metadata(id Ref) *ImageXObject {
	x.Pair(Name("Metadata"), id)
	return x
}

// AssociatedFiles is ImageXObject::associated_files in pdf-writer
// src/xobject.rs.
//
// Start writing the /AF array to specify the associated files of the image.
// PDF 2.0+ or PDF/A-3.
func (x *ImageXObject) AssociatedFiles() *WriterArray[FileSpec, *FileSpec] {
	return NewWriterArray[FileSpec](x.Insert(Name("AF")).Array())
}

// SMaskInData is SMaskInData in pdf-writer src/xobject.rs. Its zero value is
// the default of upstream, SMaskInDataIgnore.
//
// What to do with in-data mask information in JPXDecode images.
type SMaskInData uint8

// The values of SMaskInData.
const (
	// SMaskInDataIgnore discards the mask data.
	SMaskInDataIgnore SMaskInData = iota
	// SMaskInDataUse uses the mask data.
	SMaskInDataUse
	// SMaskInDataPreblended uses the mask data on the image whose backdrop
	// has been pre-blended with a matte color.
	SMaskInDataPreblended
)

// String returns the name of the mode.
func (m SMaskInData) String() string {
	switch m {
	case SMaskInDataIgnore:
		return "Ignore"
	case SMaskInDataUse:
		return "Use"
	case SMaskInDataPreblended:
		return "Preblended"
	}
	return fmt.Sprintf("SMaskInData(%d)", uint8(m))
}

// toInt is SMaskInData::to_int in pdf-writer src/xobject.rs.
func (m SMaskInData) toInt() int32 {
	switch m {
	case SMaskInDataIgnore:
		return 0
	case SMaskInDataUse:
		return 1
	case SMaskInDataPreblended:
		return 2
	}
	panic(fmt.Sprintf("pdfwriter: the mode %d of SMaskInData is not known", uint8(m)))
}

// FormXObject is FormXObject in pdf-writer src/xobject.rs.
//
// Writer for an form XObject stream. PDF 1.1+.
//
// This struct is created by Chunk.FormXObject.
//
// Note that these have nothing to do with forms that have fields to fill
// out. Rather, they are a way to encapsulate and reuse content across the
// file.
type FormXObject struct {
	*Stream
}

// startFormXObject is FormXObject::start in pdf-writer src/xobject.rs.
//
// Create a new form stream writer.
func startFormXObject(stream *Stream) *FormXObject {
	stream.Pair(Name("Type"), Name("XObject"))
	stream.Pair(Name("Subtype"), Name("Form"))
	return &FormXObject{Stream: stream}
}

// BBox is FormXObject::bbox in pdf-writer src/xobject.rs.
//
// Write the /BBox attribute. Required.
//
// This clips the form xobject to coordinates in its coordinate system.
func (x *FormXObject) BBox(bbox Rect) *FormXObject {
	x.Pair(Name("BBox"), bbox)
	return x
}

// Matrix is FormXObject::matrix in pdf-writer src/xobject.rs.
//
// Write the /Matrix attribute to map form space to user space.
func (x *FormXObject) Matrix(matrix [6]float32) *FormXObject {
	itemsF32(x.Insert(Name("Matrix")).Array(), matrix[:])
	return x
}

// Group is FormXObject::group in pdf-writer src/xobject.rs.
//
// Start writing the /Group dictionary to set up transparency model
// parameters and let this XObject be known as a group. PDF 1.4+.
func (x *FormXObject) Group() *Group {
	return Start[Group](x.Insert(Name("Group")))
}

// StructParent is FormXObject::struct_parent in pdf-writer src/xobject.rs.
//
// Write the /StructParent attribute to indicate the structure tree element
// this XObject belongs to. Mutually exclusive with StructParents. PDF 1.3+.
func (x *FormXObject) StructParent(key int32) *FormXObject {
	x.Pair(Name("StructParent"), Int(key))
	return x
}

// StructParents is FormXObject::struct_parents in pdf-writer src/xobject.rs.
//
// Write the /StructParents attribute to indicate the structure tree
// elements the contents of this XObject may belong to. Mutually exclusive
// with StructParent. PDF 1.3+.
func (x *FormXObject) StructParents(key int32) *FormXObject {
	x.Pair(Name("StructParents"), Int(key))
	return x
}

// Reference is FormXObject::reference in pdf-writer src/xobject.rs.
//
// Start writing the /Ref dictionary to identify the page from an external
// document that the XObject is a reference to. PDF 1.4+.
//
// Note that this key is forbidden in PDF/A.
func (x *FormXObject) Reference() *Reference {
	return Start[Reference](x.Insert(Name("Ref")))
}

// Metadata is FormXObject::metadata in pdf-writer src/xobject.rs.
//
// Write the /Metadata attribute to specify the XObject's metadata. PDF
// 1.4+.
//
// The reference shall point to a metadata stream.
func (x *FormXObject) Metadata(id Ref) *FormXObject {
	x.Pair(Name("Metadata"), id)
	return x
}

// LastModified is FormXObject::last_modified in pdf-writer src/xobject.rs.
//
// Write the /LastModified attribute. PDF 1.3+.
func (x *FormXObject) LastModified(lastModified Date) *FormXObject {
	x.Pair(Name("LastModified"), lastModified)
	return x
}

// AssociatedFiles is FormXObject::associated_files in pdf-writer
// src/xobject.rs.
//
// Start writing the /AF array to specify the associated files of the Form
// XObject. PDF 2.0+ or PDF/A-3.
func (x *FormXObject) AssociatedFiles() *WriterArray[FileSpec, *FileSpec] {
	return NewWriterArray[FileSpec](x.Insert(Name("AF")).Array())
}

// Group is Group in pdf-writer src/xobject.rs.
//
// Writer for a group XObject dictionary. PDF 1.4+.
//
// This struct is created by FormXObject.Group and Page.Group.
type Group struct {
	*Dict
}

// start is the macro writer! for Group in pdf-writer src/xobject.rs.
func (g *Group) start(obj Obj) {
	g.Dict = obj.Dict()
	g.Pair(Name("Type"), Name("Group"))
}

// Transparency is Group::transparency in pdf-writer src/xobject.rs.
//
// Set the /S attribute to /Transparency. Required to set the remaining
// transparency parameters.
func (g *Group) Transparency() *Group {
	g.Pair(Name("S"), Name("Transparency"))
	return g
}

// Isolated is Group::isolated in pdf-writer src/xobject.rs.
//
// Set the /I attribute to indicate whether the group is isolated.
//
// If it is true, the group will initially be composited against a clear
// backdrop. If it is false, the group will be composited against the
// backdrop of the parent group.
func (g *Group) Isolated(isolated bool) *Group {
	g.Pair(Name("I"), Bool(isolated))
	return g
}

// Knockout is Group::knockout in pdf-writer src/xobject.rs.
//
// Set the /K attribute to indicate whether the group is a knockout group.
//
// Within a knockout group, the group children are all composited separately
// against the backdrop instead of on top of each other.
func (g *Group) Knockout(knockout bool) *Group {
	g.Pair(Name("K"), Bool(knockout))
	return g
}

// Reference is Reference in pdf-writer src/xobject.rs.
//
// Writer for an external XObject reference dictionary. PDF 1.4+.
//
// This struct is created by FormXObject.Reference.
//
// Reference XObjects are forbidden in PDF/A.
type Reference struct {
	*Dict
}

// start is the macro writer! for Reference in pdf-writer src/xobject.rs.
func (r *Reference) start(obj Obj) {
	r.Dict = obj.Dict()
}

// FileSpec is Reference::file_spec in pdf-writer src/xobject.rs.
//
// Start writing the /F attribute to set a file specification dictionary.
// Required.
func (r *Reference) FileSpec() *FileSpec {
	return Start[FileSpec](r.Insert(Name("F")))
}

// PageNumber is Reference::page_number in pdf-writer src/xobject.rs.
//
// Write the /Page attribute to set the page number. Setting the attribute
// through either this function or PageLabel is required. Page indices start
// at 0.
func (r *Reference) PageNumber(page int32) *Reference {
	r.Pair(Name("Page"), Int(page))
	return r
}

// PageLabel is Reference::page_label in pdf-writer src/xobject.rs.
//
// Write the /Page attribute to set the page label. Setting the attribute
// through either this function or PageNumber is required.
func (r *Reference) PageLabel(label TextStrLike) *Reference {
	r.Pair(Name("Page"), label)
	return r
}

// ID is Reference::id in pdf-writer src/xobject.rs.
//
// Write the /ID attribute to set the file identifier.
func (r *Reference) ID(id [2]Str) *Reference {
	NewTypedArray[Str](r.Insert(Name("ID")).Array()).Items(id[:]...)
	return r
}

// Graphics and content.

// ImageXObject is Chunk::image_xobject in pdf-writer src/chunk.rs.
//
// Start writing an image XObject stream.
//
// The samples should be encoded according to the stream's filter, color
// space and bits per component.
func (c *Chunk) ImageXObject(id Ref, samples []byte) *ImageXObject {
	return startImageXObject(c.Stream(id, samples))
}

// FormXObject is Chunk::form_xobject in pdf-writer src/chunk.rs.
//
// Start writing a form XObject stream.
//
// These can be used as transparency groups.
//
// Note that these have nothing to do with forms that have fields to fill
// out. Rather, they are a way to encapsulate and reuse content across the
// file.
//
// You can create the content bytes using a Content builder.
func (c *Chunk) FormXObject(id Ref, content []byte) *FormXObject {
	return startFormXObject(c.Stream(id, content))
}

// ColorSpace is ImageXObject::color_space in pdf-writer src/xobject.rs.
//
// Start writing the /ColorSpace attribute.
func (x *ImageXObject) ColorSpace() *ColorSpace {
	return Start[ColorSpace](x.Insert(Name("ColorSpace")))
}

// Intent is ImageXObject::intent in pdf-writer src/xobject.rs.
//
// Write the /Intent attribute.
func (x *ImageXObject) Intent(intent RenderingIntent) *ImageXObject {
	x.Pair(Name("Intent"), intent.toName())
	return x
}

// Resources is FormXObject::resources in pdf-writer src/xobject.rs.
//
// Start writing the /Resources dictionary.
func (x *FormXObject) Resources() *Resources {
	return Start[Resources](x.Insert(Name("Resources")))
}

// ColorSpace is Group::color_space in pdf-writer src/xobject.rs.
//
// Start writing the /CS attribute.
func (g *Group) ColorSpace() *ColorSpace {
	return Start[ColorSpace](g.Insert(Name("CS")))
}
