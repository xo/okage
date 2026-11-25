// Copyright 2023 The xmp-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmpwriter

import (
	"cmp"
	"fmt"
	"math"
	"strconv"
)

// This file ports xmp-writer src/types.rs.

// nsKind is the case of the enum Namespace in xmp-writer src/types.rs, in
// the order of upstream. The order is the order of the derived Ord, which
// sorts the declarations of the namespaces in Writer.Finish.
type nsKind uint8

// The cases of Namespace.
const (
	nsRdf nsKind = iota
	nsDublinCore
	nsXmp
	nsXmpRights
	nsXmpResourceRef
	nsXmpResourceEvent
	nsXmpVersion
	nsXmpJob
	nsXmpJobManagement
	nsXmpColorant
	nsXmpFont
	nsXmpDimensions
	nsXmpMedia
	nsXmpPaged
	nsXmpDynamicMedia
	nsXmpImage
	nsXmpIdq
	nsAdobePdf
	nsPdfAID
	nsPdfUAID
	nsPdfXID
	nsPdfAExtension
	nsPdfASchema
	nsPdfAProperty
	nsPdfAType
	nsPdfAField
	nsCustom
)

// Namespace is Namespace in xmp-writer src/types.rs. A Rust enum with a
// custom case becomes a struct that holds the case and, for the custom case,
// the custom namespace. The value is comparable, as the derived Eq of
// upstream makes it. The variables below hold the cases without data.
//
// XML Namespaces for the XMP properties.
type Namespace struct {
	kind   nsKind
	custom CustomNamespace
}

// The cases of Namespace without data. Do not change them.
var (
	NamespaceRdf              = Namespace{kind: nsRdf}
	NamespaceDublinCore       = Namespace{kind: nsDublinCore}
	NamespaceXmp              = Namespace{kind: nsXmp}
	NamespaceXmpRights        = Namespace{kind: nsXmpRights}
	NamespaceXmpResourceRef   = Namespace{kind: nsXmpResourceRef}
	NamespaceXmpResourceEvent = Namespace{kind: nsXmpResourceEvent}
	NamespaceXmpVersion       = Namespace{kind: nsXmpVersion}
	NamespaceXmpJob           = Namespace{kind: nsXmpJob}
	NamespaceXmpJobManagement = Namespace{kind: nsXmpJobManagement}
	NamespaceXmpColorant      = Namespace{kind: nsXmpColorant}
	NamespaceXmpFont          = Namespace{kind: nsXmpFont}
	NamespaceXmpDimensions    = Namespace{kind: nsXmpDimensions}
	NamespaceXmpMedia         = Namespace{kind: nsXmpMedia}
	NamespaceXmpPaged         = Namespace{kind: nsXmpPaged}
	NamespaceXmpDynamicMedia  = Namespace{kind: nsXmpDynamicMedia}
	NamespaceXmpImage         = Namespace{kind: nsXmpImage}
	NamespaceXmpIdq           = Namespace{kind: nsXmpIdq}
	NamespaceAdobePdf         = Namespace{kind: nsAdobePdf}
	NamespacePdfAID           = Namespace{kind: nsPdfAID}
	NamespacePdfUAID          = Namespace{kind: nsPdfUAID}
	NamespacePdfXID           = Namespace{kind: nsPdfXID}
	NamespacePdfAExtension    = Namespace{kind: nsPdfAExtension}
	NamespacePdfASchema       = Namespace{kind: nsPdfASchema}
	NamespacePdfAProperty     = Namespace{kind: nsPdfAProperty}
	NamespacePdfAType         = Namespace{kind: nsPdfAType}
	NamespacePdfAField        = Namespace{kind: nsPdfAField}
)

// NewCustom is Namespace::Custom in xmp-writer src/types.rs.
func NewCustom(custom CustomNamespace) Namespace {
	return Namespace{kind: nsCustom, custom: custom}
}

// CustomNamespace is CustomNamespace in xmp-writer src/types.rs.
//
// A custom XML namespace.
type CustomNamespace struct {
	name      string
	namespace string
	url       string
}

// NewCustomNamespace is CustomNamespace::new in xmp-writer src/types.rs.
//
// Creates a new custom namespace.
func NewCustomNamespace(name, namespace, url string) CustomNamespace {
	return CustomNamespace{name: name, namespace: namespace, url: url}
}

// Predefined is Namespace::PREDEFINED in xmp-writer src/types.rs. Do not
// change it.
//
// All namespaces this crate knows natively.
var Predefined = []Namespace{
	NamespaceRdf,
	NamespaceDublinCore,
	NamespaceXmp,
	NamespaceXmpRights,
	NamespaceXmpResourceRef,
	NamespaceXmpResourceEvent,
	NamespaceXmpVersion,
	NamespaceXmpJob,
	NamespaceXmpJobManagement,
	NamespaceXmpColorant,
	NamespaceXmpFont,
	NamespaceXmpDimensions,
	NamespaceXmpMedia,
	NamespaceXmpPaged,
	NamespaceXmpDynamicMedia,
	NamespaceXmpImage,
	NamespaceXmpIdq,
	NamespaceAdobePdf,
	NamespacePdfAID,
	NamespacePdfUAID,
	NamespacePdfXID,
	NamespacePdfAExtension,
	NamespacePdfASchema,
	NamespacePdfAProperty,
	NamespacePdfAType,
	NamespacePdfAField,
}

// namespaceInfo holds the name, the URL and the prefix of a namespace
// without data, in the order of nsKind.
var namespaceInfo = [...]struct{ name, url, prefix string }{
	nsRdf:              {"RDF", "http://www.w3.org/1999/02/22-rdf-syntax-ns#", "rdf"},
	nsDublinCore:       {"Dublin Core", "http://purl.org/dc/elements/1.1/", "dc"},
	nsXmp:              {"XMP", "http://ns.adobe.com/xap/1.0/", "xmp"},
	nsXmpRights:        {"XMP Rights", "http://ns.adobe.com/xap/1.0/rights/", "xmpRights"},
	nsXmpResourceRef:   {"XMP Resource Reference", "http://ns.adobe.com/xap/1.0/sType/ResourceRef#", "stRef"},
	nsXmpResourceEvent: {"XMP Resource Event", "http://ns.adobe.com/xap/1.0/sType/ResourceEvent#", "stEvt"},
	nsXmpVersion:       {"XMP Version", "http://ns.adobe.com/xap/1.0/sType/Version#", "stVer"},
	nsXmpJob:           {"XMP Job", "http://ns.adobe.com/xap/1.0/sType/Job#", "stJob"},
	nsXmpJobManagement: {"XMP Job Management", "http://ns.adobe.com/xap/1.0/bj/", "xmpBJ"},
	nsXmpColorant:      {"XMP Colorant", "http://ns.adobe.com/xap/1.0/g/", "xmpG"},
	nsXmpFont:          {"XMP Font", "http://ns.adobe.com/xap/1.0/sType/Font#", "stFnt"},
	nsXmpDimensions:    {"XMP Dimensions", "http://ns.adobe.com/xap/1.0/sType/Dimensions#", "stDim"},
	nsXmpMedia:         {"XMP Media Management", "http://ns.adobe.com/xap/1.0/mm/", "xmpMM"},
	nsXmpPaged:         {"XMP Paged Text", "http://ns.adobe.com/xap/1.0/t/pg/", "xmpTPg"},
	nsXmpDynamicMedia:  {"XMP Dynamic Media", "http://ns.adobe.com/xmp/1.0/DynamicMedia/", "xmpDM"},
	nsXmpImage:         {"XMP Image", "http://ns.adobe.com/xap/1.0/g/img/", "xmpGImg"},
	nsXmpIdq:           {"XMP Identifier Qualifier", "http://ns.adobe.com/xmp/Identifier/qual/1.0/", "xmpidq"},
	nsAdobePdf:         {"Adobe PDF", "http://ns.adobe.com/pdf/1.3/", "pdf"},
	nsPdfAID:           {"PDF/A Identification", "http://www.aiim.org/pdfa/ns/id/", "pdfaid"},
	nsPdfUAID:          {"PDF/UA Identification", "http://www.aiim.org/pdfua/ns/id/", "pdfuaid"},
	nsPdfXID:           {"PDF/X Identification", "http://www.npes.org/pdfx/ns/id/", "pdfxid"},
	nsPdfAExtension:    {"PDF/A Extension schema container", "http://www.aiim.org/pdfa/ns/extension/", "pdfaExtension"},
	nsPdfASchema:       {"PDF/A Schema container", "http://www.aiim.org/pdfa/ns/schema#", "pdfaSchema"},
	nsPdfAProperty:     {"PDF/A Property", "http://www.aiim.org/pdfa/ns/property#", "pdfaProperty"},
	nsPdfAType:         {"PDF/A Type", "http://www.aiim.org/pdfa/ns/type#", "pdfaType"},
	nsPdfAField:        {"PDF/A Field", "http://www.aiim.org/pdfa/ns/field#", "pdfaField"},
}

// Name is Namespace::name in xmp-writer src/types.rs.
//
// Returns a human-readable name for the namespace.
func (n Namespace) Name() string {
	if n.kind == nsCustom {
		return n.custom.name
	}
	return namespaceInfo[n.kind].name
}

// URL is Namespace::url in xmp-writer src/types.rs.
//
// Returns the URL for the namespace.
func (n Namespace) URL() string {
	if n.kind == nsCustom {
		return n.custom.url
	}
	return namespaceInfo[n.kind].url
}

// Prefix is Namespace::prefix in xmp-writer src/types.rs.
//
// Returns the prefix for the namespace.
func (n Namespace) Prefix() string {
	if n.kind == nsCustom {
		return n.custom.namespace
	}
	return namespaceInfo[n.kind].prefix
}

// compare is the derived Ord of Namespace in xmp-writer src/types.rs: the
// order of the cases, then the fields of a custom namespace in their order.
func (n Namespace) compare(o Namespace) int {
	if c := cmp.Compare(n.kind, o.kind); c != 0 {
		return c
	}
	if c := cmp.Compare(n.custom.name, o.custom.name); c != 0 {
		return c
	}
	if c := cmp.Compare(n.custom.namespace, o.custom.namespace); c != 0 {
		return c
	}
	return cmp.Compare(n.custom.url, o.custom.url)
}

// Element is Element in xmp-writer src/types.rs. Each method of an element
// ends it, as each method of upstream takes the element by value. Do not
// use an element after one of its methods.
//
// A XMP property.
//
// Created by Writer.Element, Array.Element, Array.ElementWithAttrs,
// Struct.Element, Struct.ElementWithAttrs.
type Element struct {
	writer    *Writer
	name      string
	namespace Namespace
}

// Attr is the (&str, &str) of an attribute of xmp-writer src/types.rs: a
// key and a value, which the writer writes as they are.
type Attr struct {
	Key, Value string
}

// startElement is Element::start in xmp-writer src/types.rs.
func startElement(writer *Writer, name string, namespace Namespace) *Element {
	return elementWithAttrs(writer, name, namespace, nil)
}

// elementWithAttrs is Element::with_attrs in xmp-writer src/types.rs.
func elementWithAttrs(writer *Writer, name string, namespace Namespace, attrs []Attr) *Element {
	writer.buf = append(writer.buf, '<')
	writer.buf = append(writer.buf, namespace.Prefix()...)
	writer.buf = append(writer.buf, ':')
	writer.buf = append(writer.buf, name...)

	for _, a := range attrs {
		writer.buf = append(writer.buf, ' ')
		writer.buf = append(writer.buf, a.Key...)
		writer.buf = append(writer.buf, `="`...)
		writer.buf = append(writer.buf, a.Value...)
		writer.buf = append(writer.buf, '"')
	}

	writer.namespaces[namespace] = struct{}{}
	return &Element{writer: writer, name: name, namespace: namespace}
}

// Value is Element::value in xmp-writer src/types.rs.
//
// Sets the property to a primitive value.
func (e *Element) Value(val Type) {
	e.writer.buf = append(e.writer.buf, '>')
	e.writer.buf = val.Append(e.writer.buf)
	e.close()
}

// URI is Element::uri in xmp-writer src/types.rs.
//
// Sets the property to a URI or URL.
func (e *Element) URI(uri string) {
	e.writer.namespaces[NamespaceRdf] = struct{}{}
	e.writer.buf = append(e.writer.buf, ` rdf:resource="`...)
	e.writer.buf = Text(uri).Append(e.writer.buf)
	e.writer.buf = append(e.writer.buf, `"/>`...)
}

// Obj is Element::obj in xmp-writer src/types.rs.
//
// Start writing a struct as the property value.
func (e *Element) Obj() *Struct {
	e.writer.namespaces[NamespaceRdf] = struct{}{}
	e.writer.buf = append(e.writer.buf, ` rdf:parseType="Resource">`...)
	return startStruct(e.writer, e.name, e.namespace)
}

// Array is Element::array in xmp-writer src/types.rs.
//
// Start writing an array as the property value.
func (e *Element) Array(kind RdfCollectionType) *Array {
	e.writer.buf = append(e.writer.buf, '>')
	return startArray(e.writer, kind, e.name, e.namespace)
}

// close is Element::close in xmp-writer src/types.rs.
func (e *Element) close() {
	e.writer.buf = append(e.writer.buf, "</"...)
	e.writer.buf = append(e.writer.buf, e.namespace.Prefix()...)
	e.writer.buf = append(e.writer.buf, ':')
	e.writer.buf = append(e.writer.buf, e.name...)
	e.writer.buf = append(e.writer.buf, '>')
}

// LangAlt is an item of a language alternative of xmp-writer src/types.rs:
// a language and a text. A nil language is None, which the writer writes
// as x-default.
type LangAlt struct {
	Lang  *LangID
	Value string
}

// LanguageAlternative is Element::language_alternative in xmp-writer
// src/types.rs.
//
// Set a language alternative of primitive values as the property value.
func (e *Element) LanguageAlternative(items []LangAlt) {
	array := e.Array(RdfCollectionAlt)
	for _, item := range items {
		lang := DefaultLangID
		if item.Lang != nil {
			lang = *item.Lang
		}
		array.ElementWithAttrs([]Attr{{Key: "xml:lang", Value: string(lang)}}).Value(Text(item.Value))
	}
	array.Finish()
}

// UnorderedArray is Element::unordered_array in xmp-writer src/types.rs.
//
// Start writing an unordered array (rdf:Bag) as the property value.
func (e *Element) UnorderedArray(items ...Type) {
	e.collection(RdfCollectionBag, items)
}

// OrderedArray is Element::ordered_array in xmp-writer src/types.rs.
//
// Start writing an ordered array (rdf:Seq) as the property value.
func (e *Element) OrderedArray(items ...Type) {
	e.collection(RdfCollectionSeq, items)
}

// AlternativeArray is Element::alternative_array in xmp-writer
// src/types.rs.
//
// Start writing an alternative array (rdf:Alt) as the property value.
func (e *Element) AlternativeArray(items ...Type) {
	e.collection(RdfCollectionAlt, items)
}

// collection writes an array of the kind with one element for each item,
// as the three methods of arrays of upstream do.
func (e *Element) collection(kind RdfCollectionType, items []Type) {
	array := e.Array(kind)
	for _, item := range items {
		array.Element().Value(item)
	}
	array.Finish()
}

// Array is Array in xmp-writer src/types.rs. Finish runs the impl Drop of
// upstream, which closes the array. Call it once, after the last element.
//
// An XMP array value.
//
// Created by Element.Array.
type Array struct {
	writer    *Writer
	kind      RdfCollectionType
	name      string
	namespace Namespace
}

// startArray is Array::start in xmp-writer src/types.rs.
func startArray(writer *Writer, kind RdfCollectionType, name string, namespace Namespace) *Array {
	writer.namespaces[NamespaceRdf] = struct{}{}
	writer.buf = append(writer.buf, "<rdf:"...)
	writer.buf = append(writer.buf, kind.RdfType()...)
	writer.buf = append(writer.buf, '>')
	return &Array{writer: writer, kind: kind, name: name, namespace: namespace}
}

// Element is Array::element in xmp-writer src/types.rs.
//
// Start writing an element in the array.
func (a *Array) Element() *Element {
	return a.ElementWithAttrs(nil)
}

// ElementWithAttrs is Array::element_with_attrs in xmp-writer
// src/types.rs.
//
// Start writing an element with attributes in the array.
func (a *Array) ElementWithAttrs(attrs []Attr) *Element {
	return elementWithAttrs(a.writer, "li", NamespaceRdf, attrs)
}

// Finish is the impl Drop for Array in xmp-writer src/types.rs.
func (a *Array) Finish() {
	w := a.writer
	w.buf = append(w.buf, "</rdf:"...)
	w.buf = append(w.buf, a.kind.RdfType()...)
	w.buf = append(w.buf, "></"...)
	w.buf = append(w.buf, a.namespace.Prefix()...)
	w.buf = append(w.buf, ':')
	w.buf = append(w.buf, a.name...)
	w.buf = append(w.buf, '>')
}

// Struct is Struct in xmp-writer src/types.rs. Finish runs the impl Drop of
// upstream, which closes the struct. Call it once, after the last property.
//
// An XMP struct value.
//
// Created by Element.Obj.
type Struct struct {
	writer    *Writer
	name      string
	namespace Namespace
}

// startStruct is Struct::start in xmp-writer src/types.rs.
func startStruct(writer *Writer, name string, namespace Namespace) *Struct {
	return &Struct{writer: writer, name: name, namespace: namespace}
}

// Element is Struct::element in xmp-writer src/types.rs.
//
// Start writing a property in the struct.
func (s *Struct) Element(name string, namespace Namespace) *Element {
	return s.ElementWithAttrs(name, namespace, nil)
}

// ElementWithAttrs is Struct::element_with_attrs in xmp-writer
// src/types.rs.
//
// Start writing a property with attributes in the struct.
func (s *Struct) ElementWithAttrs(name string, namespace Namespace, attrs []Attr) *Element {
	return elementWithAttrs(s.writer, name, namespace, attrs)
}

// Finish is the impl Drop for Struct in xmp-writer src/types.rs.
func (s *Struct) Finish() {
	w := s.writer
	w.buf = append(w.buf, "</"...)
	w.buf = append(w.buf, s.namespace.Prefix()...)
	w.buf = append(w.buf, ':')
	w.buf = append(w.buf, s.name...)
	w.buf = append(w.buf, '>')
}

// Type is the trait XmpType in xmp-writer src/types.rs. Append is the
// method write of upstream: it appends the value to buf and returns the
// result.
//
// Primitive XMP types.
type Type interface {
	Append(buf []byte) []byte
}

// Bool is the impl XmpType for bool in xmp-writer src/types.rs.
type Bool bool

// Append writes True or False.
func (b Bool) Append(buf []byte) []byte {
	if b {
		return append(buf, "True"...)
	}
	return append(buf, "False"...)
}

// Int32 is the impl XmpType for i32 in xmp-writer src/types.rs.
type Int32 int32

// Append writes the integer in decimal.
func (i Int32) Append(buf []byte) []byte {
	return strconv.AppendInt(buf, int64(i), 10)
}

// Int64 is the impl XmpType for i64 in xmp-writer src/types.rs.
type Int64 int64

// Append writes the integer in decimal.
func (i Int64) Append(buf []byte) []byte {
	return strconv.AppendInt(buf, int64(i), 10)
}

// Float32 is the impl XmpType for f32 in xmp-writer src/types.rs.
type Float32 float32

// Append writes the float as the Display of Rust writes it.
func (f Float32) Append(buf []byte) []byte {
	return appendRustFloat(buf, float64(f), 32)
}

// Float64 is the impl XmpType for f64 in xmp-writer src/types.rs.
type Float64 float64

// Append writes the float as the Display of Rust writes it.
func (f Float64) Append(buf []byte) []byte {
	return appendRustFloat(buf, float64(f), 64)
}

// appendRustFloat appends a float as the Display of a Rust float writes it:
// the shortest decimal that gives back the float, with no exponent, and
// inf, -inf and NaN for the values that are not finite.
func appendRustFloat(buf []byte, f float64, bitSize int) []byte {
	switch {
	case math.IsInf(f, 1):
		return append(buf, "inf"...)
	case math.IsInf(f, -1):
		return append(buf, "-inf"...)
	case math.IsNaN(f):
		return append(buf, "NaN"...)
	}
	return strconv.AppendFloat(buf, f, 'f', -1, bitSize)
}

// Text is the impl XmpType for &str in xmp-writer src/types.rs.
type Text string

// Append writes the text with the characters <, >, &, ' and " escaped.
func (t Text) Append(buf []byte) []byte {
	for _, c := range []byte(t) {
		switch c {
		case '<':
			buf = append(buf, "&lt;"...)
		case '>':
			buf = append(buf, "&gt;"...)
		case '&':
			buf = append(buf, "&amp;"...)
		case '\'':
			buf = append(buf, "&apos;"...)
		case '"':
			buf = append(buf, "&quot;"...)
		default:
			buf = append(buf, c)
		}
	}
	return buf
}

// texts converts strings to texts, for the arrays of text of upstream.
func texts(items []string) []Type {
	out := make([]Type, len(items))
	for i, s := range items {
		out[i] = Text(s)
	}
	return out
}

// RdfCollectionType is RdfCollectionType in xmp-writer src/types.rs.
//
// Types of RDF collections.
type RdfCollectionType uint8

// The values of RdfCollectionType.
const (
	// RdfCollectionSeq is an ordered array / sequence.
	RdfCollectionSeq RdfCollectionType = iota
	// RdfCollectionBag is an unordered array / bag.
	RdfCollectionBag
	// RdfCollectionAlt is an alternative array.
	RdfCollectionAlt
)

// RdfType is RdfCollectionType::rdf_type in xmp-writer src/types.rs.
//
// The RDF type name for this collection type.
func (k RdfCollectionType) RdfType() string {
	switch k {
	case RdfCollectionSeq:
		return "Seq"
	case RdfCollectionBag:
		return "Bag"
	default:
		return "Alt"
	}
}

// LangID is LangId in xmp-writer src/types.rs.
//
// A language specifier as defined in RFC 3066. Can also be x-default if the
// language is not known.
type LangID string

// DefaultLangID is the Default of LangId in xmp-writer src/types.rs.
const DefaultLangID LangID = "x-default"

// Append writes the language as it is.
func (l LangID) Append(buf []byte) []byte {
	return append(buf, l...)
}

// DateTime is DateTime in xmp-writer src/types.rs. A nil field is None.
//
// A date and time.
type DateTime struct {
	Year   uint16
	Month  *uint8
	Day    *uint8
	Hour   *uint8
	Minute *uint8
	Second *uint8
	// Timezone is the timezone of this date and time. No assumptions about
	// the timezone or locale should be made if this is nil.
	Timezone *Timezone
}

// Timezone is Timezone in xmp-writer src/types.rs. A zero value is
// Timezone::Utc, and Local is Timezone::Local.
//
// A timezone.
type Timezone struct {
	// Local is false for UTC time. Use Local for British time.
	Local bool
	// Hour is the timezone offset in hours of a local timezone.
	Hour int8
	// Minute is the timezone offset in minutes of a local timezone.
	Minute int8
}

// NewDateTime is DateTime::new in xmp-writer src/types.rs.
//
// Create a new date and time with all fields.
func NewDateTime(year uint16, month, day, hour, minute, second uint8, timezone Timezone) DateTime {
	return DateTime{
		Year:     year,
		Month:    &month,
		Day:      &day,
		Hour:     &hour,
		Minute:   &minute,
		Second:   &second,
		Timezone: &timezone,
	}
}

// Year is DateTime::year in xmp-writer src/types.rs.
//
// Create a new date with a year only.
func Year(year uint16) DateTime {
	return DateTime{Year: year}
}

// Date is DateTime::date in xmp-writer src/types.rs.
//
// Create a new date and time without a timezone.
func Date(year uint16, month, day uint8) DateTime {
	return DateTime{Year: year, Month: &month, Day: &day}
}

// LocalTime is DateTime::local_time in xmp-writer src/types.rs.
//
// Create a new date and time without a timezone.
func LocalTime(year uint16, month, day, hour, minute, second uint8) DateTime {
	return DateTime{
		Year:   year,
		Month:  &month,
		Day:    &day,
		Hour:   &hour,
		Minute: &minute,
		Second: &second,
	}
}

// Append writes the date and its fields up to the first field that is
// nil, as the closure of upstream does.
func (d DateTime) Append(buf []byte) []byte {
	buf = fmt.Appendf(buf, "%04d", d.Year)
	if d.Month == nil {
		return buf
	}
	buf = fmt.Appendf(buf, "-%02d", *d.Month)
	if d.Day == nil {
		return buf
	}
	buf = fmt.Appendf(buf, "-%02d", *d.Day)
	if d.Hour == nil || d.Minute == nil {
		return buf
	}
	buf = fmt.Appendf(buf, "T%02d:%02d", *d.Hour, *d.Minute)
	if d.Second != nil {
		buf = fmt.Appendf(buf, ":%02d", *d.Second)
	}
	if d.Timezone == nil {
		return buf
	}
	if !d.Timezone.Local {
		return append(buf, 'Z')
	}
	return fmt.Appendf(buf, "%+03d:%02d", d.Timezone.Hour, d.Timezone.Minute)
}

// RenditionClass is RenditionClass in xmp-writer src/types.rs. Each case
// writes a text, so the type is the text. RenditionClassThumbnail gives the
// case Thumbnail, and a conversion of a string gives the case Custom.
//
// The intended use of the resource.
type RenditionClass string

// The cases of RenditionClass without data.
const (
	// RenditionClassDefault is the master resource.
	RenditionClassDefault RenditionClass = "default"
	// RenditionClassDraft is a review copy.
	RenditionClassDraft RenditionClass = "draft"
	// RenditionClassLowResolution is a low-resolution stand-in.
	RenditionClassLowResolution RenditionClass = "low-res"
	// RenditionClassProof is a proof copy.
	RenditionClassProof RenditionClass = "proof"
	// RenditionClassScreen is a copy at screen resolution.
	RenditionClassScreen RenditionClass = "screen"
)

// RenditionClassThumbnail is RenditionClass::Thumbnail in xmp-writer
// src/types.rs. A nil argument is None.
//
// A thumbnail, with the format, the size and the color space of the
// thumbnail.
func RenditionClassThumbnail(format *string, size *[2]uint32, colorSpace *string) RenditionClass {
	buf := []byte("thumbnail")
	if format != nil {
		buf = append(buf, ':')
		buf = append(buf, *format...)
	}
	if size != nil {
		buf = append(buf, ':')
		buf = strconv.AppendUint(buf, uint64(size[0]), 10)
		buf = append(buf, 'x')
		buf = strconv.AppendUint(buf, uint64(size[1]), 10)
	}
	if colorSpace != nil {
		buf = append(buf, ':')
		buf = append(buf, *colorSpace...)
	}
	return RenditionClass(buf)
}

// Append writes the text of the class.
func (r RenditionClass) Append(buf []byte) []byte {
	return append(buf, r...)
}

// Rating is Rating in xmp-writer src/types.rs.
//
// A user-assigned rating.
type Rating uint8

// The values of Rating.
const (
	// RatingRejected means that the resource has been rejected.
	RatingRejected Rating = iota
	// RatingUnknown means that the resource has not been rated.
	RatingUnknown
	// RatingOneStar means that the resource has been rated 1 star.
	RatingOneStar
	// RatingTwoStars means that the resource has been rated 2 stars.
	RatingTwoStars
	// RatingThreeStars means that the resource has been rated 3 stars.
	RatingThreeStars
	// RatingFourStars means that the resource has been rated 4 stars.
	RatingFourStars
	// RatingFiveStars means that the resource has been rated 5 stars.
	RatingFiveStars
)

// RatingFromStars is Rating::from_stars in xmp-writer src/types.rs. A nil
// number is None. It panics for a number above 5, as upstream does.
//
// Creates a new Rating from the number of stars.
func RatingFromStars(stars *uint32) Rating {
	if stars == nil {
		return RatingUnknown
	}
	switch *stars {
	case 0:
		return RatingUnknown
	case 1:
		return RatingOneStar
	case 2:
		return RatingTwoStars
	case 3:
		return RatingThreeStars
	case 4:
		return RatingFourStars
	case 5:
		return RatingFiveStars
	}
	panic(fmt.Sprintf("Invalid number of stars: %d (must be between 0 and 5)", *stars))
}

// ToXMP is Rating::to_xmp in xmp-writer src/types.rs.
//
// Convert the rating to an XMP primitive.
func (r Rating) ToXMP() float32 {
	if r == RatingRejected {
		return -1
	}
	return float32(r - RatingUnknown)
}

// MaskMarkers is MaskMarkers in xmp-writer src/types.rs.
//
// Whether to ignore the markers of an ingredient.
type MaskMarkers uint8

// The values of MaskMarkers.
const (
	// MaskMarkersAll ignores all markers and those of the children.
	MaskMarkersAll MaskMarkers = iota
	// MaskMarkersNone processes all markers.
	MaskMarkersNone
)

// Append writes All or None.
func (m MaskMarkers) Append(buf []byte) []byte {
	if m == MaskMarkersAll {
		return append(buf, "All"...)
	}
	return append(buf, "None"...)
}

// ResourceEventAction is ResourceEventAction in xmp-writer src/types.rs.
// Each case writes a text, so the type is the text, and a conversion of a
// string gives the case Custom.
//
// The type of a resource event.
type ResourceEventAction string

// The cases of ResourceEventAction without data.
const (
	ResourceEventActionConverted      ResourceEventAction = "converted"
	ResourceEventActionCopied         ResourceEventAction = "copied"
	ResourceEventActionCreated        ResourceEventAction = "created"
	ResourceEventActionCropped        ResourceEventAction = "cropped"
	ResourceEventActionEdited         ResourceEventAction = "edited"
	ResourceEventActionFiltered       ResourceEventAction = "filtered"
	ResourceEventActionFormatted      ResourceEventAction = "formatted"
	ResourceEventActionVersionUpdated ResourceEventAction = "version_updated"
	ResourceEventActionPrinted        ResourceEventAction = "printed"
	ResourceEventActionPublished      ResourceEventAction = "published"
	ResourceEventActionManaged        ResourceEventAction = "managed"
	ResourceEventActionProduced       ResourceEventAction = "produced"
	ResourceEventActionResized        ResourceEventAction = "resized"
	ResourceEventActionSaved          ResourceEventAction = "saved"
)

// Append writes the text of the action.
func (a ResourceEventAction) Append(buf []byte) []byte {
	return append(buf, a...)
}

// ColorantMode is ColorantMode in xmp-writer src/types.rs.
//
// The color space in which a colorant is defined.
type ColorantMode uint8

// The values of ColorantMode.
const (
	ColorantModeCMYK ColorantMode = iota
	ColorantModeRGB
	ColorantModeLab
)

// Append writes CMYK, RGB or Lab.
func (m ColorantMode) Append(buf []byte) []byte {
	switch m {
	case ColorantModeCMYK:
		return append(buf, "CMYK"...)
	case ColorantModeRGB:
		return append(buf, "RGB"...)
	default:
		return append(buf, "Lab"...)
	}
}

// ColorantType is ColorantType in xmp-writer src/types.rs.
//
// The type of a colorant.
type ColorantType uint8

// The values of ColorantType.
const (
	// ColorantTypeProcess is a color inherent to the printing process.
	ColorantTypeProcess ColorantType = iota
	// ColorantTypeSpot is a special color.
	ColorantTypeSpot
)

// Append writes PROCESS or SPOT.
func (t ColorantType) Append(buf []byte) []byte {
	if t == ColorantTypeProcess {
		return append(buf, "PROCESS"...)
	}
	return append(buf, "SPOT"...)
}

// DimensionUnit is DimensionUnit in xmp-writer src/types.rs. Each case
// writes a text, so the type is the text, and a conversion of a string
// gives the case Custom.
//
// The unit of a physical dimension.
type DimensionUnit string

// The cases of DimensionUnit without data.
const (
	DimensionUnitInch  DimensionUnit = "inch"
	DimensionUnitMm    DimensionUnit = "mm"
	DimensionUnitPixel DimensionUnit = "pixel"
	DimensionUnitPica  DimensionUnit = "pica"
	DimensionUnitPoint DimensionUnit = "point"
)

// Append writes the text of the unit.
func (u DimensionUnit) Append(buf []byte) []byte {
	return append(buf, u...)
}

// FontType is FontType in xmp-writer src/types.rs. Each case writes a
// text, so the type is the text, and a conversion of a string gives the
// case Custom.
//
// The font file type.
type FontType string

// The cases of FontType without data.
const (
	FontTypeTrueType FontType = "TrueType"
	FontTypeOpenType FontType = "OpenType"
	FontTypeType1    FontType = "Type1"
	FontTypeBitmap   FontType = "Bitmap"
)

// Append writes the text of the font type.
func (t FontType) Append(buf []byte) []byte {
	return append(buf, t...)
}
