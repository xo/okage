// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"fmt"
	"unicode/utf16"
)

// This file ports pdf-writer src/font.rs, with the methods of Chunk for
// fonts of src/chunk.rs.

// Type1Font is Type1Font in pdf-writer src/font.rs.
//
// Writer for a Type-1 font dictionary.
//
// This struct is created by Chunk.Type1Font.
type Type1Font struct {
	*Dict
}

// start is the macro writer! for Type1Font in pdf-writer src/font.rs.
func (f *Type1Font) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("Type"), Name("Font"))
	f.Pair(Name("Subtype"), Name("Type1"))
}

// Name is Type1Font::name in pdf-writer src/font.rs.
//
// Write the /Name attribute, which is the name of the font in the current
// resource dictionary. Required in PDF 1.0, discouraged in PDF 1.1+.
func (f *Type1Font) Name(name Name) *Type1Font {
	f.Pair(Name("Name"), name)
	return f
}

// BaseFont is Type1Font::base_font in pdf-writer src/font.rs.
//
// Write the /BaseFont attribute. This is the PostScript name of the font.
// Required.
//
// In PDF/A files, the standard 14 fonts are unavailable, so you must embed
// the font data.
func (f *Type1Font) BaseFont(name Name) *Type1Font {
	f.Pair(Name("BaseFont"), name)
	return f
}

// FirstChar is Type1Font::first_char in pdf-writer src/font.rs.
//
// Write the FirstChar attribute, defining the first character code in the
// font's widths array. Required (except for standard 14 fonts before PDF
// 1.5).
func (f *Type1Font) FirstChar(first uint8) *Type1Font {
	f.Pair(Name("FirstChar"), Int(first))
	return f
}

// LastChar is Type1Font::last_char in pdf-writer src/font.rs.
//
// Write the LastChar attribute, defining the last character code in the
// font's widths array. Required (except for standard 14 fonts before PDF
// 1.5).
func (f *Type1Font) LastChar(last uint8) *Type1Font {
	f.Pair(Name("LastChar"), Int(last))
	return f
}

// Widths is Type1Font::widths in pdf-writer src/font.rs.
//
// Write the /Widths array. Should be of length last - first + 1. Required
// (except for standard 14 fonts before PDF 1.5).
func (f *Type1Font) Widths(widths ...float32) *Type1Font {
	itemsF32(f.Insert(Name("Widths")).Array(), widths)
	return f
}

// FontDescriptor is Type1Font::font_descriptor in pdf-writer src/font.rs.
//
// Write the /FontDescriptor attribute. Required (except for standard 14
// fonts before PDF 1.5).
func (f *Type1Font) FontDescriptor(id Ref) *Type1Font {
	f.Pair(Name("FontDescriptor"), id)
	return f
}

// EncodingPredefined is Type1Font::encoding_predefined in pdf-writer
// src/font.rs.
//
// Write the /Encoding attribute as a predefined encoding. Either this or
// EncodingCustom is required.
func (f *Type1Font) EncodingPredefined(encoding Name) *Type1Font {
	f.Pair(Name("Encoding"), encoding)
	return f
}

// EncodingCustom is Type1Font::encoding_custom in pdf-writer src/font.rs.
//
// Start writing an /Encoding dictionary. Either this or EncodingPredefined
// is required.
func (f *Type1Font) EncodingCustom() *Encoding {
	return Start[Encoding](f.Insert(Name("Encoding")))
}

// ToUnicode is Type1Font::to_unicode in pdf-writer src/font.rs.
//
// Write the /ToUnicode attribute. PDF 1.2+.
//
// A suitable character map can be built with UnicodeCmap.
func (f *Type1Font) ToUnicode(id Ref) *Type1Font {
	f.Pair(Name("ToUnicode"), id)
	return f
}

// Type3Font is Type3Font in pdf-writer src/font.rs.
//
// Writer for a Type-3 font dictionary.
//
// This struct is created by Chunk.Type3Font.
type Type3Font struct {
	*Dict
}

// start is the macro writer! for Type3Font in pdf-writer src/font.rs.
func (f *Type3Font) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("Type"), Name("Font"))
	f.Pair(Name("Subtype"), Name("Type3"))
}

// Name is Type3Font::name in pdf-writer src/font.rs.
//
// Write the /Name attribute, which is the name of the font in the current
// resource dictionary. Always required in PDF 1.0, required if FontName is
// set in child font descriptor.
func (f *Type3Font) Name(name Name) *Type3Font {
	f.Pair(Name("Name"), name)
	return f
}

// BBox is Type3Font::bbox in pdf-writer src/font.rs.
//
// Write the /FontBBox attribute. Required.
func (f *Type3Font) BBox(bbox Rect) *Type3Font {
	f.Pair(Name("FontBBox"), bbox)
	return f
}

// Matrix is Type3Font::matrix in pdf-writer src/font.rs.
//
// Write the /FontMatrix attribute, which defines the mapping from glyph
// space to text space. Required.
func (f *Type3Font) Matrix(matrix [6]float32) *Type3Font {
	itemsF32(f.Insert(Name("FontMatrix")).Array(), matrix[:])
	return f
}

// CharProcs is Type3Font::char_procs in pdf-writer src/font.rs.
//
// Start writing the /CharProcs dictionary, which maps glyph names to glyph
// content streams. Required.
//
// Each glyph's content stream must start with either the d0
// (Content.StartColorGlyph) or d1 (Content.StartShapeGlyph) operator.
func (f *Type3Font) CharProcs() *TypedDict[Ref] {
	return NewTypedDict[Ref](f.Insert(Name("CharProcs")).Dict())
}

// EncodingPredefined is Type3Font::encoding_predefined in pdf-writer
// src/font.rs.
//
// Write the /Encoding attribute as a predefined encoding. Either this or
// EncodingCustom is required.
func (f *Type3Font) EncodingPredefined(encoding Name) *Type3Font {
	f.Pair(Name("Encoding"), encoding)
	return f
}

// EncodingCustom is Type3Font::encoding_custom in pdf-writer src/font.rs.
//
// Start writing an /Encoding dictionary. Either this or EncodingPredefined
// is required.
func (f *Type3Font) EncodingCustom() *Encoding {
	return Start[Encoding](f.Insert(Name("Encoding")))
}

// FirstChar is Type3Font::first_char in pdf-writer src/font.rs.
//
// Write the FirstChar attribute, defining the first character code in the
// font's widths array. Required.
func (f *Type3Font) FirstChar(first uint8) *Type3Font {
	f.Pair(Name("FirstChar"), Int(first))
	return f
}

// LastChar is Type3Font::last_char in pdf-writer src/font.rs.
//
// Write the LastChar attribute, defining the last character code in the
// font's widths array. Required.
func (f *Type3Font) LastChar(last uint8) *Type3Font {
	f.Pair(Name("LastChar"), Int(last))
	return f
}

// Widths is Type3Font::widths in pdf-writer src/font.rs.
//
// Write the /Widths array. Should be of length last - first + 1. Required.
func (f *Type3Font) Widths(widths ...float32) *Type3Font {
	itemsF32(f.Insert(Name("Widths")).Array(), widths)
	return f
}

// FontDescriptor is Type3Font::font_descriptor in pdf-writer src/font.rs.
//
// Write the /FontDescriptor attribute. Required in Tagged PDFs.
func (f *Type3Font) FontDescriptor(id Ref) *Type3Font {
	f.Pair(Name("FontDescriptor"), id)
	return f
}

// ToUnicode is Type3Font::to_unicode in pdf-writer src/font.rs.
//
// Write the /ToUnicode attribute. PDF 1.2+.
//
// A suitable character map can be built with UnicodeCmap.
//
// This attribute is required in some profiles of PDF/A-2, PDF/A-3, and
// PDF/A-4 for some fonts. When present, these standards require that no
// character may be mapped to 0, U+FEFF, or U+FFFE.
func (f *Type3Font) ToUnicode(id Ref) *Type3Font {
	f.Pair(Name("ToUnicode"), id)
	return f
}

// Encoding is Encoding in pdf-writer src/font.rs.
//
// Writer for a simple font encoding dictionary.
//
// This struct is created by Type1Font.EncodingCustom and
// Type3Font.EncodingCustom.
type Encoding struct {
	*Dict
}

// start is the macro writer! for Encoding in pdf-writer src/font.rs.
func (e *Encoding) start(obj Obj) {
	e.Dict = obj.Dict()
	e.Pair(Name("Type"), Name("Encoding"))
}

// BaseEncoding is Encoding::base_encoding in pdf-writer src/font.rs.
//
// Write the BaseEncoding attribute, from which this encoding is described
// through differences.
func (e *Encoding) BaseEncoding(name Name) *Encoding {
	e.Pair(Name("BaseEncoding"), name)
	return e
}

// Differences is Encoding::differences in pdf-writer src/font.rs.
//
// Start writing the /Differences array.
func (e *Encoding) Differences() *Differences {
	return Start[Differences](e.Insert(Name("Differences")))
}

// Differences is Differences in pdf-writer src/font.rs.
//
// Writer for an encoding differences array.
//
// This struct is created by Encoding.Differences.
type Differences struct {
	*Array
}

// start is the macro writer! for Differences in pdf-writer src/font.rs.
func (d *Differences) start(obj Obj) {
	d.Array = obj.Array()
}

// Consecutive is Differences::consecutive in pdf-writer src/font.rs.
//
// Maps consecutive character codes starting at start to the given glyph
// names.
func (d *Differences) Consecutive(start uint8, names ...Name) *Differences {
	d.Item(Int(start))
	for _, name := range names {
		d.Item(name)
	}
	return d
}

// Type0Font is Type0Font in pdf-writer src/font.rs.
//
// Writer for a Type-0 (composite) font dictionary. PDF 1.2+.
//
// This struct is created by Chunk.Type0Font.
type Type0Font struct {
	*Dict
}

// start is the macro writer! for Type0Font in pdf-writer src/font.rs.
func (f *Type0Font) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("Type"), Name("Font"))
	f.Pair(Name("Subtype"), Name("Type0"))
}

// BaseFont is Type0Font::base_font in pdf-writer src/font.rs.
//
// Write the /BaseFont attribute. This is the PostScript name of the font.
// Required.
func (f *Type0Font) BaseFont(name Name) *Type0Font {
	f.Pair(Name("BaseFont"), name)
	return f
}

// EncodingPredefined is Type0Font::encoding_predefined in pdf-writer
// src/font.rs.
//
// Write the /Encoding attribute as a predefined encoding. Either this or
// EncodingCmap is required.
func (f *Type0Font) EncodingPredefined(encoding Name) *Type0Font {
	f.Pair(Name("Encoding"), encoding)
	return f
}

// EncodingCmap is Type0Font::encoding_cmap in pdf-writer src/font.rs.
//
// Write the /Encoding attribute as a reference to a character map. Either
// this or EncodingPredefined is required.
func (f *Type0Font) EncodingCmap(cmap Ref) *Type0Font {
	f.Pair(Name("Encoding"), cmap)
	return f
}

// DescendantFont is Type0Font::descendant_font in pdf-writer src/font.rs.
//
// Write the /DescendantFonts attribute as a one-element array containing a
// reference to a CIDFont. Required.
func (f *Type0Font) DescendantFont(cidFont Ref) *Type0Font {
	f.Insert(Name("DescendantFonts")).Array().Item(cidFont)
	return f
}

// ToUnicode is Type0Font::to_unicode in pdf-writer src/font.rs.
//
// Write the /ToUnicode attribute.
//
// A suitable character map can be built with UnicodeCmap.
func (f *Type0Font) ToUnicode(cmap Ref) *Type0Font {
	f.Pair(Name("ToUnicode"), cmap)
	return f
}

// CIDFont is CidFont in pdf-writer src/font.rs.
//
// Writer for a CID font dictionary. PDF 1.2+.
//
// This struct is created by Chunk.CIDFont.
type CIDFont struct {
	*Dict
}

// start is the macro writer! for CidFont in pdf-writer src/font.rs.
func (f *CIDFont) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("Type"), Name("Font"))
}

// Subtype is CidFont::subtype in pdf-writer src/font.rs.
//
// Write the /Subtype attribute. Required.
func (f *CIDFont) Subtype(subtype CIDFontType) *CIDFont {
	f.Pair(Name("Subtype"), subtype.toName())
	return f
}

// BaseFont is CidFont::base_font in pdf-writer src/font.rs.
//
// Write the /BaseFont attribute. Required.
func (f *CIDFont) BaseFont(name Name) *CIDFont {
	f.Pair(Name("BaseFont"), name)
	return f
}

// SystemInfo is CidFont::system_info in pdf-writer src/font.rs.
//
// Write the /CIDSystemInfo dictionary. Required.
func (f *CIDFont) SystemInfo(info SystemInfo) *CIDFont {
	info.write(f.Insert(Name("CIDSystemInfo")))
	return f
}

// FontDescriptor is CidFont::font_descriptor in pdf-writer src/font.rs.
//
// Write the /FontDescriptor attribute. Required.
func (f *CIDFont) FontDescriptor(id Ref) *CIDFont {
	f.Pair(Name("FontDescriptor"), id)
	return f
}

// DefaultWidth is CidFont::default_width in pdf-writer src/font.rs.
//
// Write the /DW attribute, specifying the default glyph width.
func (f *CIDFont) DefaultWidth(width float32) *CIDFont {
	f.Pair(Name("DW"), Real(width))
	return f
}

// Widths is CidFont::widths in pdf-writer src/font.rs.
//
// Start writing the /W (widths) array.
func (f *CIDFont) Widths() *Widths {
	return Start[Widths](f.Insert(Name("W")))
}

// CIDToGIDMapPredefined is CidFont::cid_to_gid_map_predefined in pdf-writer
// src/font.rs.
//
// Write the /CIDToGIDMap attribute as a predefined name.
//
// The attribute must be present for PDF/A. The only permissible predefined
// name is Identity, otherwise CIDToGIDMapStream must be used.
func (f *CIDFont) CIDToGIDMapPredefined(name Name) *CIDFont {
	f.Pair(Name("CIDToGIDMap"), name)
	return f
}

// CIDToGIDMapStream is CidFont::cid_to_gid_map_stream in pdf-writer
// src/font.rs.
//
// Write the /CIDToGIDMap attribute as a reference to a stream, whose bytes
// directly map from CIDs to glyph indices.
//
// The attribute must be present for PDF/A.
func (f *CIDFont) CIDToGIDMapStream(stream Ref) *CIDFont {
	f.Pair(Name("CIDToGIDMap"), stream)
	return f
}

// CIDFontType is CidFontType in pdf-writer src/font.rs.
//
// The subtype of a CID font.
type CIDFontType uint8

// The values of CIDFontType.
const (
	// CIDFontType0 is a CID font containing CFF glyph descriptions.
	CIDFontType0 CIDFontType = iota
	// CIDFontType2 is a CID font containing TrueType glyph descriptions.
	CIDFontType2
)

// String returns the PDF name of the subtype, without the slash.
func (t CIDFontType) String() string {
	return string(t.toName())
}

// toName is CidFontType::to_name in pdf-writer src/font.rs.
func (t CIDFontType) toName() Name {
	switch t {
	case CIDFontType0:
		return Name("CIDFontType0")
	case CIDFontType2:
		return Name("CIDFontType2")
	}
	panic(fmt.Sprintf("pdfwriter: the CID font type %d is not known", uint8(t)))
}

// Widths is Widths in pdf-writer src/font.rs.
//
// Writer for a CID font widths array.
//
// This struct is created by CIDFont.Widths.
type Widths struct {
	*Array
}

// start is the macro writer! for Widths in pdf-writer src/font.rs.
func (w *Widths) start(obj Obj) {
	w.Array = obj.Array()
}

// Consecutive is Widths::consecutive in pdf-writer src/font.rs.
//
// Specifies individual widths for a range of consecutive CIDs starting at
// start.
func (w *Widths) Consecutive(start uint16, widths ...float32) *Widths {
	w.Item(Int(start))
	itemsF32(w.Push().Array(), widths)
	return w
}

// Same is Widths::same in pdf-writer src/font.rs.
//
// Specifies the same width for all CIDs between first and last.
func (w *Widths) Same(first, last uint16, width float32) *Widths {
	w.Item(Int(first))
	w.Item(Int(last))
	w.Item(Real(width))
	return w
}

// FontDescriptor is FontDescriptor in pdf-writer src/font.rs.
//
// Writer for a font descriptor dictionary.
//
// This struct is created by Chunk.FontDescriptor.
type FontDescriptor struct {
	*Dict
}

// start is the macro writer! for FontDescriptor in pdf-writer src/font.rs.
func (d *FontDescriptor) start(obj Obj) {
	d.Dict = obj.Dict()
	d.Pair(Name("Type"), Name("FontDescriptor"))
}

// Name is FontDescriptor::name in pdf-writer src/font.rs.
//
// Write the /FontName attribute. Required, except for Type 3 fonts.
func (d *FontDescriptor) Name(name Name) *FontDescriptor {
	d.Pair(Name("FontName"), name)
	return d
}

// Family is FontDescriptor::family in pdf-writer src/font.rs.
//
// Write the /FontFamily attribute. Recommended for Type 3 fonts in Tagged
// PDFs. PDF 1.5+.
func (d *FontDescriptor) Family(family Str) *FontDescriptor {
	d.Pair(Name("FontFamily"), family)
	return d
}

// Stretch is FontDescriptor::stretch in pdf-writer src/font.rs.
//
// Write the /FontStretch attribute. Recommended for Type 3 fonts in Tagged
// PDFs. PDF 1.5+.
func (d *FontDescriptor) Stretch(stretch FontStretch) *FontDescriptor {
	d.Pair(Name("FontStretch"), stretch.toName())
	return d
}

// Weight is FontDescriptor::weight in pdf-writer src/font.rs.
//
// Write the /FontWeight attribute. Should be between 100 (lightest) and 900
// (heaviest), 400 is normal weight, 700 is bold. Recommended for Type 3
// fonts in Tagged PDFs. PDF 1.5+.
func (d *FontDescriptor) Weight(weight uint16) *FontDescriptor {
	d.Pair(Name("FontWeight"), Int(weight))
	return d
}

// Flags is FontDescriptor::flags in pdf-writer src/font.rs.
//
// Write the /Flags attribute. Required.
func (d *FontDescriptor) Flags(flags FontFlags) *FontDescriptor {
	d.Pair(Name("Flags"), Int(int32(flags)))
	return d
}

// BBox is FontDescriptor::bbox in pdf-writer src/font.rs.
//
// Write the /FontBBox attribute. Required, except for Type 3 fonts.
func (d *FontDescriptor) BBox(bbox Rect) *FontDescriptor {
	d.Pair(Name("FontBBox"), bbox)
	return d
}

// ItalicAngle is FontDescriptor::italic_angle in pdf-writer src/font.rs.
//
// Write the /ItalicAngle attribute. Required.
func (d *FontDescriptor) ItalicAngle(angle float32) *FontDescriptor {
	d.Pair(Name("ItalicAngle"), Real(angle))
	return d
}

// Ascent is FontDescriptor::ascent in pdf-writer src/font.rs.
//
// Write the /Ascent attribute. Required.
func (d *FontDescriptor) Ascent(ascent float32) *FontDescriptor {
	d.Pair(Name("Ascent"), Real(ascent))
	return d
}

// Descent is FontDescriptor::descent in pdf-writer src/font.rs.
//
// Write the /Descent attribute. Required.
func (d *FontDescriptor) Descent(descent float32) *FontDescriptor {
	d.Pair(Name("Descent"), Real(descent))
	return d
}

// Leading is FontDescriptor::leading in pdf-writer src/font.rs.
//
// Write the /Leading attribute.
func (d *FontDescriptor) Leading(leading float32) *FontDescriptor {
	d.Pair(Name("Leading"), Real(leading))
	return d
}

// CapHeight is FontDescriptor::cap_height in pdf-writer src/font.rs.
//
// Write the /CapHeight attribute. Required for fonts with Latin characters,
// except for Type 3 fonts.
func (d *FontDescriptor) CapHeight(capHeight float32) *FontDescriptor {
	d.Pair(Name("CapHeight"), Real(capHeight))
	return d
}

// XHeight is FontDescriptor::x_height in pdf-writer src/font.rs.
//
// Write the /XHeight attribute.
func (d *FontDescriptor) XHeight(xHeight float32) *FontDescriptor {
	d.Pair(Name("XHeight"), Real(xHeight))
	return d
}

// StemV is FontDescriptor::stem_v in pdf-writer src/font.rs.
//
// Write the /StemV attribute. Required, except for Type 3 fonts.
func (d *FontDescriptor) StemV(stemV float32) *FontDescriptor {
	d.Pair(Name("StemV"), Real(stemV))
	return d
}

// StemH is FontDescriptor::stem_h in pdf-writer src/font.rs.
//
// Write the /StemH attribute.
func (d *FontDescriptor) StemH(stemH float32) *FontDescriptor {
	d.Pair(Name("StemH"), Real(stemH))
	return d
}

// AvgWidth is FontDescriptor::avg_width in pdf-writer src/font.rs.
//
// Write the /AvgWidth attribute.
func (d *FontDescriptor) AvgWidth(avgWidth float32) *FontDescriptor {
	d.Pair(Name("AvgWidth"), Real(avgWidth))
	return d
}

// MaxWidth is FontDescriptor::max_width in pdf-writer src/font.rs.
//
// Write the /MaxWidth attribute.
func (d *FontDescriptor) MaxWidth(maxWidth float32) *FontDescriptor {
	d.Pair(Name("MaxWidth"), Real(maxWidth))
	return d
}

// MissingWidth is FontDescriptor::missing_width in pdf-writer src/font.rs.
//
// Write the /MissingWidth attribute.
func (d *FontDescriptor) MissingWidth(missingWidth float32) *FontDescriptor {
	d.Pair(Name("MissingWidth"), Real(missingWidth))
	return d
}

// FontFile is FontDescriptor::font_file in pdf-writer src/font.rs.
//
// Write the /FontFile attribute, referecing Type 1 font data.
func (d *FontDescriptor) FontFile(id Ref) *FontDescriptor {
	d.Pair(Name("FontFile"), id)
	return d
}

// FontFile2 is FontDescriptor::font_file2 in pdf-writer src/font.rs.
//
// Write the /FontFile2 attribute, referencing TrueType font data. PDF 1.1+.
func (d *FontDescriptor) FontFile2(id Ref) *FontDescriptor {
	d.Pair(Name("FontFile2"), id)
	return d
}

// FontFile3 is FontDescriptor::font_file3 in pdf-writer src/font.rs.
//
// Write the /FontFile3 attribute, referencing CFF font data. PDF 1.2+ or
// PDF 1.3+ for CID-keyed fonts.
func (d *FontDescriptor) FontFile3(id Ref) *FontDescriptor {
	d.Pair(Name("FontFile3"), id)
	return d
}

// CharSet is FontDescriptor::char_set in pdf-writer src/font.rs.
//
// Write the /CharSet attribute, encoding the character names of a font
// subset as a string. This is only relevant for Type 1 fonts. PDF 1.1+.
//
// If present in PDF/A, this must include all characters in the subset, even
// if they are not used in the document.
func (d *FontDescriptor) CharSet(names Str) *FontDescriptor {
	d.Pair(Name("CharSet"), names)
	return d
}

// Additional FontDescriptor attributes for CIDFonts.

// Style is FontDescriptor::style in pdf-writer src/font.rs.
//
// Write the /Style attribute. Optional.
//
// The class and subclass values should be extracted from the sFamilyClass
// field of the OS/2 table. The panose array should be extracted from the
// panose field of the OS/2 table.
func (d *FontDescriptor) Style(class, subclass uint8, panose [10]uint8) *FontDescriptor {
	var bytes [12]byte
	bytes[0] = class
	bytes[1] = subclass
	copy(bytes[2:], panose[:])
	d.Insert(Name("Style")).Dict().Pair(Name("Panose"), Str(bytes[:]))
	return d
}

// DescriptorOverride is FontDescriptor::descriptor_override in pdf-writer
// src/font.rs.
//
// Start writing the /FD attribute. Optional.
//
// Overrides the global font descriptor for specific glyphs.
func (d *FontDescriptor) DescriptorOverride() *FontDescriptorOverride {
	return Start[FontDescriptorOverride](d.Insert(Name("FD")))
}

// CIDSet is FontDescriptor::cid_set in pdf-writer src/font.rs.
//
// Write the /CIDSet attribute.
//
// If present in PDF/A, this must include all characters in the subset, even
// if they are not used in the document.
func (d *FontDescriptor) CIDSet(id Ref) *FontDescriptor {
	d.Pair(Name("CIDSet"), id)
	return d
}

// FontDescriptorOverride is FontDescriptorOverride in pdf-writer
// src/font.rs.
//
// Writer for a font descriptor override dictionary.
//
// This struct is created by FontDescriptor.DescriptorOverride.
//
// The font descriptor dictionaries within can only contain metrics data.
type FontDescriptorOverride struct {
	*Dict
}

// start is the macro writer! for FontDescriptorOverride in pdf-writer
// src/font.rs.
func (o *FontDescriptorOverride) start(obj Obj) {
	o.Dict = obj.Dict()
}

// CustomClass is FontDescriptorOverride::custom_class in pdf-writer
// src/font.rs.
//
// Start writing a FontDescriptor dictionary for a custom character class.
func (o *FontDescriptorOverride) CustomClass(class Name) *FontDescriptor {
	return Start[FontDescriptor](o.Insert(class))
}

// CustomClassRef is FontDescriptorOverride::custom_class_ref in pdf-writer
// src/font.rs.
//
// Write a class override referencing a FontDescriptor dictionary
// indirectly.
func (o *FontDescriptorOverride) CustomClassRef(class Name, id Ref) *FontDescriptorOverride {
	o.Pair(class, id)
	return o
}

// CJKClass is FontDescriptorOverride::cjk_class in pdf-writer src/font.rs.
//
// Start writing a FontDescriptor dictionary for a predefined CJK class.
func (o *FontDescriptorOverride) CJKClass(class CJKClass) *FontDescriptor {
	return Start[FontDescriptor](o.Insert(class.toName()))
}

// CJKClassRef is FontDescriptorOverride::cjk_class_ref in pdf-writer
// src/font.rs.
//
// Write a class override for a predefined CJK class referencing a
// FontDescriptor dictionary indirectly.
func (o *FontDescriptorOverride) CJKClassRef(class CJKClass, id Ref) *FontDescriptorOverride {
	o.Pair(class.toName(), id)
	return o
}

// CJKClass is CjkClass in pdf-writer src/font.rs.
//
// Glyph classes for CJK fonts as defined in ISO 32000-1:2008.
type CJKClass uint8

// The values of CJKClass.
const (
	CJKClassAlphabetic CJKClass = iota
	CJKClassAlphaNum
	CJKClassDingbats
	CJKClassDingbatsRot
	CJKClassGeneric
	CJKClassGenericRot
	CJKClassHangul
	CJKClassHanja
	CJKClassHanzi
	CJKClassHRoman
	CJKClassHRomanRot
	CJKClassHKana
	CJKClassHKanaRot
	CJKClassHojoKanji
	CJKClassKana
	CJKClassKanji
	CJKClassProportional
	CJKClassProportionalRot
	CJKClassRuby
)

// cjkClassNames holds the PDF names of the values of CJKClass, in order.
var cjkClassNames = [...]Name{
	"Alphabetic",
	"AlphaNum",
	"Dingbats",
	"DingbatsRot",
	"Generic",
	"GenericRot",
	"Hangul",
	"Hanja",
	"Hanzi",
	"HRoman",
	"HRomanRot",
	"HKana",
	"HKanaRot",
	"HojoKanji",
	"Kana",
	"Kanji",
	"Proportional",
	"ProportionalRot",
	"Ruby",
}

// String returns the PDF name of the class, without the slash.
func (c CJKClass) String() string {
	return string(c.toName())
}

// toName is CjkClass::to_name in pdf-writer src/font.rs.
func (c CJKClass) toName() Name {
	if int(c) < len(cjkClassNames) {
		return cjkClassNames[c]
	}
	panic(fmt.Sprintf("pdfwriter: the CJK class %d is not known", uint8(c)))
}

// FontStretch is FontStretch in pdf-writer src/font.rs.
//
// The width of a font's glyphs.
type FontStretch uint8

// The values of FontStretch.
const (
	FontStretchUltraCondensed FontStretch = iota
	FontStretchExtraCondensed
	FontStretchCondensed
	FontStretchSemiCondensed
	FontStretchNormal
	FontStretchSemiExpanded
	FontStretchExpanded
	FontStretchExtraExpanded
	FontStretchUltraExpanded
)

// fontStretchNames holds the PDF names of the values of FontStretch, in
// order.
var fontStretchNames = [...]Name{
	"UltraCondensed",
	"ExtraCondensed",
	"Condensed",
	"SemiCondensed",
	"Normal",
	"SemiExpanded",
	"Expanded",
	"ExtraExpanded",
	"UltraExpanded",
}

// String returns the PDF name of the stretch, without the slash.
func (s FontStretch) String() string {
	return string(s.toName())
}

// toName is FontStretch::to_name in pdf-writer src/font.rs.
func (s FontStretch) toName() Name {
	if int(s) < len(fontStretchNames) {
		return fontStretchNames[s]
	}
	panic(fmt.Sprintf("pdfwriter: the font stretch %d is not known", uint8(s)))
}

// FontFlags is FontFlags in pdf-writer src/font.rs, the bitflags of
// upstream. A value can hold bits that have no constant, as
// FontFlags::from_bits_retain gives them.
//
// Bitflags describing various characteristics of fonts.
type FontFlags uint32

// The flags of FontFlags.
const (
	// FontFlagsFixedPitch means that all glyphs have the same width.
	FontFlagsFixedPitch FontFlags = 1 << 0
	// FontFlagsSerif means that glyphs have short strokes at their stems.
	FontFlagsSerif FontFlags = 1 << 1
	// FontFlagsSymbolic means that the font contains glyphs not in the Adobe
	// standard Latin character set.
	FontFlagsSymbolic FontFlags = 1 << 2
	// FontFlagsScript means that the glyphs resemeble cursive handwritiwng.
	FontFlagsScript FontFlags = 1 << 3
	// FontFlagsNonSymbolic means that the font only uses glyphs in the Adobe
	// standard Latin character set.
	FontFlagsNonSymbolic FontFlags = 1 << 5
	// FontFlagsItalic means that the glyphs are slanted to the right.
	FontFlagsItalic FontFlags = 1 << 6
	// FontFlagsAllCap means that the font does not contain lowercase
	// letters.
	FontFlagsAllCap FontFlags = 1 << 16
	// FontFlagsSmallCap means that the font's lowercase letters are similar
	// to the uppercase ones, but smaller.
	FontFlagsSmallCap FontFlags = 1 << 17
	// FontFlagsForceBold ensures that bold glyphs are painted with more
	// pixels than normal glyphs even at very small sizes.
	FontFlagsForceBold FontFlags = 1 << 18
)

// Cmap is Cmap in pdf-writer src/font.rs.
//
// Writer for a character map stream.
//
// This struct is created by Chunk.Cmap.
type Cmap struct {
	*Stream
}

// startCmap is Cmap::start in pdf-writer src/font.rs.
//
// Create a new character map writer.
func startCmap(stream *Stream) *Cmap {
	stream.Pair(Name("Type"), Name("CMap"))
	return &Cmap{Stream: stream}
}

// Name is Cmap::name in pdf-writer src/font.rs.
//
// Write the /CMapName attribute. Required.
func (c *Cmap) Name(name Name) *Cmap {
	c.Pair(Name("CMapName"), name)
	return c
}

// SystemInfo is Cmap::system_info in pdf-writer src/font.rs.
//
// Write the /CIDSystemInfo attribute. Required.
func (c *Cmap) SystemInfo(info SystemInfo) *Cmap {
	info.write(c.Insert(Name("CIDSystemInfo")))
	return c
}

// WritingMode is Cmap::writing_mode in pdf-writer src/font.rs.
//
// Write the /WMode attribute. Optional.
//
// This describes whether the CMap applies to a font with horizontal or
// vertical writing mode. The default is whatever is specified in the CMap.
//
// This is required in PDF/A and must match the writing mode of the embedded
// CMap.
func (c *Cmap) WritingMode(mode WMode) *Cmap {
	c.Pair(Name("WMode"), Int(mode.toInt()))
	return c
}

// UseCmapStream is Cmap::use_cmap_stream in pdf-writer src/font.rs.
//
// Write the /UseCMap attribute using a stream reference. Optional.
//
// This allows specifying a base CMap to extend.
//
// Note that this attribute is restricted in PDF/A and may only be used with
// the well-known CMap names from the PDF standard. Use UseCmapPredefined to
// specify a predefined name.
func (c *Cmap) UseCmapStream(cmap Ref) *Cmap {
	c.Pair(Name("UseCMap"), cmap)
	return c
}

// UseCmapPredefined is Cmap::use_cmap_predefined in pdf-writer src/font.rs.
//
// Write the /UseCMap attribute using a predefined name. Optional.
//
// This allows specifying a base CMap to extend.
//
// Note that this attribute is restricted in PDF/A.
func (c *Cmap) UseCmapPredefined(name Name) *Cmap {
	c.Pair(Name("UseCMap"), name)
	return c
}

// WMode is WMode in pdf-writer src/font.rs. Its zero value is the default
// of upstream, WModeHorizontal.
//
// The writing mode of a character map.
type WMode uint8

// The values of WMode.
const (
	WModeHorizontal WMode = iota
	WModeVertical
)

// String returns the name of the writing mode.
func (m WMode) String() string {
	switch m {
	case WModeHorizontal:
		return "Horizontal"
	case WModeVertical:
		return "Vertical"
	}
	return fmt.Sprintf("WMode(%d)", uint8(m))
}

// toInt is WMode::to_int in pdf-writer src/font.rs.
func (m WMode) toInt() int32 {
	switch m {
	case WModeHorizontal:
		return 0
	case WModeVertical:
		return 1
	}
	panic(fmt.Sprintf("pdfwriter: the writing mode %d is not known", uint8(m)))
}

// UnicodeCmap is UnicodeCmap in pdf-writer src/font.rs. Go has no default
// type arguments, so a caller names G, where upstream defaults to u16.
//
// A builder for a /ToUnicode character map stream.
type UnicodeCmap[G GlyphID] struct {
	buf      Buf
	mappings Buf
	count    int32
}

// NewUnicodeCmap is UnicodeCmap::new in pdf-writer src/font.rs.
//
// Create a new, empty unicode character map for a horizontal writing mode
// font.
func NewUnicodeCmap[G GlyphID](name Name, info SystemInfo) *UnicodeCmap[G] {
	return NewUnicodeCmapWithWritingMode[G](name, info, WModeHorizontal)
}

// NewUnicodeCmapWithWritingMode is UnicodeCmap::with_writing_mode in
// pdf-writer src/font.rs.
//
// Create a new, empty unicode character map while specifying the writing
// mode.
func NewUnicodeCmapWithWritingMode[G GlyphID](name Name, info SystemInfo, mode WMode) *UnicodeCmap[G] {
	// https://www.adobe.com/content/dam/acom/en/devnet/font/pdfs/5014.CIDFont_Spec.pdf

	buf := newBuf()

	// Static header.
	buf.extendString("%!PS-Adobe-3.0 Resource-CMap\n")
	buf.extendString("%%DocumentNeededResources: procset CIDInit\n")
	buf.extendString("%%IncludeResource: procset CIDInit\n")

	// Dynamic header.
	buf.extendString("%%BeginResource: CMap ")
	buf.extendString(string(name))
	buf.push('\n')
	buf.extendString("%%Title: (")
	buf.extendString(string(name))
	buf.push(' ')
	buf.extendString(string(info.Registry))
	buf.push(' ')
	buf.extendString(string(info.Ordering))
	buf.push(' ')
	buf.pushInt(info.Supplement)
	buf.extendString(")\n")
	buf.extendString("%%Version: 1\n")
	buf.extendString("%%EndComments\n")

	// General body.
	buf.extendString("/CIDInit /ProcSet findresource begin\n")
	buf.extendString("12 dict begin\n")
	buf.extendString("begincmap\n")
	buf.extendString("/CIDSystemInfo 3 dict dup begin\n")
	buf.extendString("    /Registry ")
	buf.pushVal(info.Registry)
	buf.extendString(" def\n")
	buf.extendString("    /Ordering ")
	buf.pushVal(info.Ordering)
	buf.extendString(" def\n")
	buf.extendString("    /Supplement ")
	buf.pushVal(Int(info.Supplement))
	buf.extendString(" def\n")
	buf.extendString("end def\n")
	buf.extendString("/CMapName ")
	buf.pushVal(name)
	buf.extendString(" def\n")
	buf.extendString("/CMapVersion 1 def\n")
	buf.extendString("/CMapType 0 def\n")
	buf.extendString("/WMode ")
	buf.pushInt(mode.toInt())
	buf.extendString(" def\n")

	// We just cover the whole unicode codespace.
	buf.extendString("1 begincodespacerange\n")
	buf.push('<')
	pushGlyphID(&buf, G(0))
	buf.extendString("> <")
	pushGlyphID(&buf, ^G(0))
	buf.extendString(">\n")
	buf.extendString("endcodespacerange\n")

	return &UnicodeCmap[G]{
		buf:      buf,
		mappings: newBuf(),
		count:    0,
	}
}

// Pair is UnicodeCmap::pair in pdf-writer src/font.rs.
//
// Add a mapping from a glyph ID to a codepoint.
func (u *UnicodeCmap[G]) Pair(glyph G, codepoint rune) {
	u.PairWithMultiple(glyph, codepoint)
}

// PairWithMultiple is UnicodeCmap::pair_with_multiple in pdf-writer
// src/font.rs. A Rust char is always a Unicode scalar value. A rune that is
// not one, such as a surrogate, is written as U+FFFD, as utf16.AppendRune
// writes it.
//
// Add a mapping from a glyph ID to multiple codepoints.
func (u *UnicodeCmap[G]) PairWithMultiple(glyph G, codepoints ...rune) {
	u.mappings.push('<')
	pushGlyphID(&u.mappings, glyph)
	u.mappings.extendString("> <")

	var parts [2]uint16
	for _, c := range codepoints {
		for _, part := range utf16.AppendRune(parts[:0], c) {
			u.mappings.pushHexU16(part)
		}
	}

	u.mappings.extendString(">\n")
	u.count++

	// At most 100 lines per range.
	if u.count >= 100 {
		u.flushRange()
	}
}

// Finish is UnicodeCmap::finish in pdf-writer src/font.rs. Upstream takes
// the map by value. Do not use the map after this call.
//
// Finish building the character map.
func (u *UnicodeCmap[G]) Finish() Buf {
	// Flush the in-progress range.
	u.flushRange()

	// End of body.
	u.buf.extendString("endcmap\n")
	u.buf.extendString("CMapName currentdict /CMap defineresource pop\n")
	u.buf.extendString("end\n")
	u.buf.extendString("end\n")
	u.buf.extendString("%%EndResource\n")
	u.buf.extendString("%%EOF")

	return u.buf
}

// flushRange is UnicodeCmap::flush_range in pdf-writer src/font.rs.
func (u *UnicodeCmap[G]) flushRange() {
	if u.count > 0 {
		u.buf.pushInt(u.count)
		u.buf.extendString(" beginbfchar\n")
		u.buf.extend(u.mappings.AsSlice())
		u.buf.extendString("endbfchar\n")
	}

	u.count = 0
	u.mappings.inner = u.mappings.inner[:0]
}

// GlyphID is the traits GlyphId and Sealed in pdf-writer src/font.rs. Its
// type set is closed, as the sealed trait of upstream is.
//
// Type3 fonts require (in Acrobat at least) IDs in CMaps to be encoded with
// one byte only, whereas other font types use two bytes.
//
// This trait provides an abstraction to support both.
type GlyphID interface {
	uint8 | uint16
}

// pushGlyphID is Sealed::push in pdf-writer src/font.rs. MIN of Sealed is
// G(0), and MAX is ^G(0).
func pushGlyphID[G GlyphID](buf *Buf, glyph G) {
	switch g := any(glyph).(type) {
	case uint8:
		buf.pushHex(g)
	case uint16:
		buf.pushHexU16(g)
	}
}

// SystemInfo is SystemInfo in pdf-writer src/font.rs.
//
// Specifics about a character collection.
type SystemInfo struct {
	// The issuer of the collection.
	Registry Str
	// A unique name of the collection within the registry.
	Ordering Str
	// The supplement number (i.e. the version).
	Supplement int32
}

// write is SystemInfo::write in pdf-writer src/font.rs.
func (info SystemInfo) write(obj Obj) {
	obj.Dict().
		Pair(Name("Registry"), info.Registry).
		Pair(Name("Ordering"), info.Ordering).
		Pair(Name("Supplement"), Int(info.Supplement))
}

// Fonts.

// Type1Font is Chunk::type1_font in pdf-writer src/chunk.rs.
//
// Start writing a Type-1 font.
func (c *Chunk) Type1Font(id Ref) *Type1Font {
	return Start[Type1Font](c.Indirect(id))
}

// Type3Font is Chunk::type3_font in pdf-writer src/chunk.rs.
//
// Start writing a Type-3 font.
func (c *Chunk) Type3Font(id Ref) *Type3Font {
	return Start[Type3Font](c.Indirect(id))
}

// Type0Font is Chunk::type0_font in pdf-writer src/chunk.rs.
//
// Start writing a Type-0 font.
func (c *Chunk) Type0Font(id Ref) *Type0Font {
	return Start[Type0Font](c.Indirect(id))
}

// CIDFont is Chunk::cid_font in pdf-writer src/chunk.rs.
//
// Start writing a CID font.
func (c *Chunk) CIDFont(id Ref) *CIDFont {
	return Start[CIDFont](c.Indirect(id))
}

// FontDescriptor is Chunk::font_descriptor in pdf-writer src/chunk.rs.
//
// Start writing a font descriptor.
func (c *Chunk) FontDescriptor(id Ref) *FontDescriptor {
	return Start[FontDescriptor](c.Indirect(id))
}

// Cmap is Chunk::cmap in pdf-writer src/chunk.rs.
//
// Start writing a character map stream.
//
// If you want to use this for a /ToUnicode CMap, you can create the bytes
// using a UnicodeCmap builder.
func (c *Chunk) Cmap(id Ref, cmap []byte) *Cmap {
	return startCmap(c.Stream(id, cmap))
}

// Resources is Type3Font::resources in pdf-writer src/font.rs.
//
// Start writing the /Resources dictionary.
func (f *Type3Font) Resources() *Resources {
	return Start[Resources](f.Insert(Name("Resources")))
}
