// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// This file ports krilla src/interchange/metadata/xmp.rs (D99). The module
// is public in upstream, as krilla::metadata::xmp. Its types are in the
// package pdf, with their Rust names, as no name of pdf collides with them
// (D10). A Rust enum with data becomes a struct with a constructor for each
// case, and a type that upstream converts with Into takes the interface
// IntoValueType or a constructor of Value.
//
// Custom XMP metadata.
//
// In addition to the standard properties exposed on Metadata, krilla
// supports writing arbitrary XMP metadata under user-defined namespaces.
// This is useful for embedding things like e-invoice descriptors (e.g.
// ZUGFeRD/Factur-X) or RDF license information into a PDF.
//
// Create a Namespace, build Property values in it with Namespace.Property,
// and attach them via Metadata.CustomXMPProperty.
//
// A Schema describes the properties of a namespace: their value types and
// what they mean. Attach it via Metadata.CustomXMPSchema. Properties are
// checked against the schema of their namespace, if there is one.
//
// PDF/A-1 to PDF/A-3 require every property outside the predefined XMP
// schemas to be described in a PDF/A extension schema, which krilla writes
// from the attached schemas. For PDF/A-4f and PDF/A-4e, krilla embeds a
// RELAX NG schema describing the metadata instead. For other export
// targets, schemas are optional.
//
// If a namespace URI matches a schema known to xmp-writer, properties are
// written under that schema's canonical prefix, and the namespace's prefix
// is ignored. Such a namespace can't be used to redefine properties of the
// predefined schema.
//
// Custom namespaces must not reuse the prefix of a predefined schema or bind
// one prefix to two different URIs, and each URI must always be used with
// the same prefix. Otherwise Document.Finish returns an XMPError.

// Namespace is Namespace in krilla src/interchange/metadata/xmp.rs.
//
// An XMP namespace, identified by its URI.
//
// The prefix is the short name used in the serialized XML.
type Namespace struct {
	prefix     string
	uri        string
	escapedURI string
}

// NewNamespace is Namespace::new in krilla
// src/interchange/metadata/xmp.rs.
//
// Create a new namespace with the given prefix (e.g. "fx") and URI (e.g.
// "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#").
func NewNamespace(prefix, uri string) Namespace {
	escapedURI := uri
	if strings.ContainsAny(uri, `&<"`) {
		escapedURI = strings.ReplaceAll(uri, "&", "&amp;")
		escapedURI = strings.ReplaceAll(escapedURI, "<", "&lt;")
		escapedURI = strings.ReplaceAll(escapedURI, `"`, "&quot;")
	}
	return Namespace{prefix: prefix, uri: uri, escapedURI: escapedURI}
}

// Prefix is Namespace::prefix in krilla src/interchange/metadata/xmp.rs.
//
// The prefix of the namespace.
func (n Namespace) Prefix() string {
	return n.prefix
}

// URI is Namespace::uri in krilla src/interchange/metadata/xmp.rs.
//
// The URI of the namespace.
func (n Namespace) URI() string {
	return n.uri
}

// Property is Namespace::property in krilla
// src/interchange/metadata/xmp.rs.
//
// Create a property in this namespace.
func (n Namespace) Property(name string, value Value) Property {
	return NewProperty(n, name, value)
}

// Schema is Schema in krilla src/interchange/metadata/xmp.rs. A method
// returns a copy of the schema, as the builder of upstream does, and the
// copy shares no slice with the schema it came from.
//
// A description of the properties in a Namespace.
type Schema struct {
	namespace   Namespace
	name        *string
	description *string
	properties  []PropertyDescription
}

// NewSchema is Schema::new in krilla src/interchange/metadata/xmp.rs.
//
// Create a new schema without any properties.
func NewSchema(namespace Namespace) Schema {
	return Schema{namespace: namespace}
}

// Name is Schema::name in krilla src/interchange/metadata/xmp.rs.
//
// The human-readable name of the schema. Defaults to "<prefix> schema".
func (s Schema) Name(name string) Schema {
	s.name = &name
	return s
}

// Description is Schema::description in krilla
// src/interchange/metadata/xmp.rs.
//
// A human-readable description of the schema.
func (s Schema) Description(description string) Schema {
	s.description = &description
	return s
}

// Property is Schema::property in krilla src/interchange/metadata/xmp.rs.
//
// Describe a property of the schema.
func (s Schema) Property(property PropertyDescription) Schema {
	s.properties = append(slices.Clip(s.properties), property)
	return s
}

// get is Schema::get in krilla src/interchange/metadata/xmp.rs.
func (s *Schema) get(name string) (*PropertyDescription, bool) {
	for i := range s.properties {
		if s.properties[i].name == name {
			return &s.properties[i], true
		}
	}
	return nil, false
}

// PropertyDescription is PropertyDescription in krilla
// src/interchange/metadata/xmp.rs.
//
// Description of a single XMP property, or of a field of a StructType.
type PropertyDescription struct {
	name        string
	valueType   ValueType
	category    Category
	required    bool
	description string
}

// NewPropertyDescription is PropertyDescription::new in krilla
// src/interchange/metadata/xmp.rs.
//
// Create a new description of an optional, external property.
func NewPropertyDescription(name string, valueType IntoValueType, description string) PropertyDescription {
	return PropertyDescription{
		name:        name,
		valueType:   valueType.intoValueType(),
		category:    CategoryExternal,
		description: description,
	}
}

// Internal is PropertyDescription::internal in krilla
// src/interchange/metadata/xmp.rs.
//
// Mark the property as being generated internally by the producer.
func (d PropertyDescription) Internal() PropertyDescription {
	d.category = CategoryInternal
	return d
}

// Required is PropertyDescription::required in krilla
// src/interchange/metadata/xmp.rs.
//
// Mark the property as required.
func (d PropertyDescription) Required() PropertyDescription {
	d.required = true
	return d
}

// equal is the derived PartialEq of PropertyDescription in krilla
// src/interchange/metadata/xmp.rs.
func (d *PropertyDescription) equal(o *PropertyDescription) bool {
	return d.name == o.name &&
		d.valueType.equal(&o.valueType) &&
		d.category == o.category &&
		d.required == o.required &&
		d.description == o.description
}

// Category is Category in krilla src/interchange/metadata/xmp.rs.
//
// Whether a property is generated internally or supplied externally.
type Category uint8

// The values of Category.
const (
	// CategoryInternal means that the property is computed by the producer
	// (e.g. page count).
	CategoryInternal Category = iota
	// CategoryExternal means that the property is supplied by the user.
	CategoryExternal
)

// StructType is StructType in krilla src/interchange/metadata/xmp.rs.
// Upstream shares a structure type with an Arc. The port copies it into the
// value type that uses it. A method returns a copy of the type that shares
// no slice with the type it came from.
//
// A user-defined structure type.
//
// The fields of a structure are properties in the namespace of the type.
type StructType struct {
	namespace   Namespace
	name        string
	description string
	fields      []PropertyDescription
}

// NewStructType is StructType::new in krilla
// src/interchange/metadata/xmp.rs.
//
// Create a new structure type without any fields.
func NewStructType(namespace Namespace, name, description string) StructType {
	return StructType{namespace: namespace, name: name, description: description}
}

// Field is StructType::field in krilla src/interchange/metadata/xmp.rs.
//
// Describe a field of the structure.
func (t StructType) Field(field PropertyDescription) StructType {
	t.fields = append(slices.Clip(t.fields), field)
	return t
}

// equal is the derived PartialEq of StructType in krilla
// src/interchange/metadata/xmp.rs.
func (t *StructType) equal(o *StructType) bool {
	if t == o {
		return true
	}
	return t.namespace == o.namespace &&
		t.name == o.name &&
		t.description == o.description &&
		slices.EqualFunc(t.fields, o.fields, func(a, b PropertyDescription) bool { return a.equal(&b) })
}

// intoValueType is From<StructType> for ValueType in krilla
// src/interchange/metadata/xmp.rs.
func (t StructType) intoValueType() ValueType {
	return ValueType{kind: valueTypeStruct, structType: &t}
}

// valueTypeKind is the case of the enum ValueType in krilla
// src/interchange/metadata/xmp.rs.
type valueTypeKind uint8

// The cases of ValueType.
const (
	valueTypeSimple valueTypeKind = iota
	valueTypeOpenChoice
	valueTypeClosedChoice
	valueTypeOrderedArray
	valueTypeUnorderedArray
	valueTypeAlternativeArray
	valueTypeLanguageAlternative
	valueTypePredefinedStruct
	valueTypeStruct
)

// IntoValueType is the trait Into<ValueType> of krilla
// src/interchange/metadata/xmp.rs. ValueType, SimpleType, PredefinedStruct
// and StructType satisfy it.
type IntoValueType interface {
	intoValueType() ValueType
}

// ValueType is ValueType in krilla src/interchange/metadata/xmp.rs. The
// zero value is a simple text. A function gives each case with data, and
// LanguageAlternativeType gives the language alternative.
//
// The type of an XMP value.
//
// This mirrors the XMP type system: simple types, choices, the three array
// kinds, language alternatives and structures.
type ValueType struct {
	kind valueTypeKind
	// simple is the SimpleType of Simple, OpenChoice and ClosedChoice.
	simple SimpleType
	// choices holds the values of OpenChoice and ClosedChoice.
	choices []string
	// item is the item type of the three arrays.
	item *ValueType
	// predefined is the PredefinedStruct of PredefinedStruct.
	predefined PredefinedStruct
	// structType is the StructType of Struct.
	structType *StructType
}

// intoValueType returns the type itself.
func (t ValueType) intoValueType() ValueType {
	return t
}

// equal is the derived PartialEq of ValueType in krilla
// src/interchange/metadata/xmp.rs.
func (t *ValueType) equal(o *ValueType) bool {
	if t.kind != o.kind {
		return false
	}
	switch t.kind {
	case valueTypeSimple:
		return t.simple == o.simple
	case valueTypeOpenChoice, valueTypeClosedChoice:
		return t.simple == o.simple && slices.Equal(t.choices, o.choices)
	case valueTypeOrderedArray, valueTypeUnorderedArray, valueTypeAlternativeArray:
		return t.item.equal(o.item)
	case valueTypePredefinedStruct:
		return t.predefined == o.predefined
	case valueTypeStruct:
		return t.structType.equal(o.structType)
	}
	return true
}

// arrayItem returns the item type of an array type, or nil for any other
// type.
func (t *ValueType) arrayItem() *ValueType {
	switch t.kind {
	case valueTypeOrderedArray, valueTypeUnorderedArray, valueTypeAlternativeArray:
		return t.item
	}
	return nil
}

// OrderedArrayType is ValueType::ordered_array in krilla
// src/interchange/metadata/xmp.rs.
//
// An ordered array (rdf:Seq) of the given type.
func OrderedArrayType(item IntoValueType) ValueType {
	it := item.intoValueType()
	return ValueType{kind: valueTypeOrderedArray, item: &it}
}

// UnorderedArrayType is ValueType::unordered_array in krilla
// src/interchange/metadata/xmp.rs.
//
// An unordered array (rdf:Bag) of the given type.
func UnorderedArrayType(item IntoValueType) ValueType {
	it := item.intoValueType()
	return ValueType{kind: valueTypeUnorderedArray, item: &it}
}

// AlternativeArrayType is ValueType::alternative_array in krilla
// src/interchange/metadata/xmp.rs.
//
// An alternative array (rdf:Alt) of the given type.
func AlternativeArrayType(item IntoValueType) ValueType {
	it := item.intoValueType()
	return ValueType{kind: valueTypeAlternativeArray, item: &it}
}

// OpenChoiceType is ValueType::open_choice in krilla
// src/interchange/metadata/xmp.rs.
//
// A value of the given type, typically one of values.
func OpenChoiceType(ty SimpleType, values ...string) ValueType {
	return ValueType{kind: valueTypeOpenChoice, simple: ty, choices: slices.Clone(values)}
}

// ClosedChoiceType is ValueType::closed_choice in krilla
// src/interchange/metadata/xmp.rs.
//
// One of values.
func ClosedChoiceType(ty SimpleType, values ...string) ValueType {
	return ValueType{kind: valueTypeClosedChoice, simple: ty, choices: slices.Clone(values)}
}

// LanguageAlternativeType is ValueType::LanguageAlternative in krilla
// src/interchange/metadata/xmp.rs.
//
// A language alternative: an rdf:Alt of xml:lang-tagged text.
func LanguageAlternativeType() ValueType {
	return ValueType{kind: valueTypeLanguageAlternative}
}

// SimpleType is SimpleType in krilla src/interchange/metadata/xmp.rs.
//
// A simple XMP value type.
//
// Text, Boolean, Integer, Real and Date are the core simple types. The rest
// are derived from them and are all stored as text, but carry additional
// meaning.
type SimpleType uint8

// The values of SimpleType.
const (
	// SimpleTypeText is unconstrained text.
	SimpleTypeText SimpleType = iota
	// SimpleTypeBoolean is a boolean, written as True or False.
	SimpleTypeBoolean
	// SimpleTypeInteger is an integer.
	SimpleTypeInteger
	// SimpleTypeReal is a floating-point number.
	SimpleTypeReal
	// SimpleTypeDate is a date and time.
	SimpleTypeDate
	// SimpleTypeAgentName is the name of an agent, i.e. a piece of
	// software.
	SimpleTypeAgentName
	// SimpleTypeGUID is a globally unique identifier.
	SimpleTypeGUID
	// SimpleTypeLocale is a language tag.
	SimpleTypeLocale
	// SimpleTypeMIMEType is a MIME type.
	SimpleTypeMIMEType
	// SimpleTypeProperName is the name of a person or organization.
	SimpleTypeProperName
	// SimpleTypeRenditionClass is the name of a rendition of a document.
	SimpleTypeRenditionClass
	// SimpleTypeURI is a URI.
	SimpleTypeURI
	// SimpleTypeURL is a URL.
	SimpleTypeURL
	// SimpleTypeRational is a rational number, written as
	// numerator/denominator.
	SimpleTypeRational
	// SimpleTypeFrameRate is a frame rate, written as f<frames> or
	// f<frames>s<basis>.
	SimpleTypeFrameRate
	// SimpleTypeFrameCount is a number of frames at a given frame rate.
	SimpleTypeFrameCount
	// SimpleTypePart is a path identifying a portion of a resource, e.g.
	// /content/audio.
	SimpleTypePart
	// SimpleTypeXPath is an XPath expression.
	SimpleTypeXPath
)

// simpleTypeNames holds the name of each SimpleType.
var simpleTypeNames = [...]string{
	SimpleTypeText:           "Text",
	SimpleTypeBoolean:        "Boolean",
	SimpleTypeInteger:        "Integer",
	SimpleTypeReal:           "Real",
	SimpleTypeDate:           "Date",
	SimpleTypeAgentName:      "AgentName",
	SimpleTypeGUID:           "GUID",
	SimpleTypeLocale:         "Locale",
	SimpleTypeMIMEType:       "MIMEType",
	SimpleTypeProperName:     "ProperName",
	SimpleTypeRenditionClass: "RenditionClass",
	SimpleTypeURI:            "URI",
	SimpleTypeURL:            "URL",
	SimpleTypeRational:       "Rational",
	SimpleTypeFrameRate:      "FrameRate",
	SimpleTypeFrameCount:     "FrameCount",
	SimpleTypePart:           "Part",
	SimpleTypeXPath:          "XPath",
}

// String is SimpleType::as_str in krilla src/interchange/metadata/xmp.rs.
// A value that is not one of the constants gives SimpleType(n).
//
// The name of the type in a PDF/A extension schema description.
func (t SimpleType) String() string {
	if int(t) < len(simpleTypeNames) {
		return simpleTypeNames[t]
	}
	return "SimpleType(" + strconv.Itoa(int(t)) + ")"
}

// intoValueType is From<SimpleType> for ValueType in krilla
// src/interchange/metadata/xmp.rs.
func (t SimpleType) intoValueType() ValueType {
	return ValueType{kind: valueTypeSimple, simple: t}
}

// PredefinedStruct is PredefinedStruct in krilla
// src/interchange/metadata/xmp.rs.
//
// A structure type defined by the XMP specification.
type PredefinedStruct uint8

// The values of PredefinedStruct.
const (
	// PredefinedStructDimensions is the dimensions of a document (stDim).
	PredefinedStructDimensions PredefinedStruct = iota
	// PredefinedStructThumbnail is a thumbnail image (xmpGImg).
	PredefinedStructThumbnail
	// PredefinedStructResourceEvent is a high-level event in the history of
	// a resource (stEvt).
	PredefinedStructResourceEvent
	// PredefinedStructResourceRef is a reference to another resource
	// (stRef).
	PredefinedStructResourceRef
	// PredefinedStructVersion is a version of a document (stVer).
	PredefinedStructVersion
	// PredefinedStructJob is a job (stJob).
	PredefinedStructJob
	// PredefinedStructColorant is a colorant (xmpG).
	PredefinedStructColorant
	// PredefinedStructFont is a font (stFnt).
	PredefinedStructFont
)

// predefinedStructNames holds the name of each PredefinedStruct.
var predefinedStructNames = [...]string{
	PredefinedStructDimensions:    "Dimensions",
	PredefinedStructThumbnail:     "Thumbnail",
	PredefinedStructResourceEvent: "ResourceEvent",
	PredefinedStructResourceRef:   "ResourceRef",
	PredefinedStructVersion:       "Version",
	PredefinedStructJob:           "Job",
	PredefinedStructColorant:      "Colorant",
	PredefinedStructFont:          "Font",
}

// String is PredefinedStruct::as_str in krilla
// src/interchange/metadata/xmp.rs. A value that is not one of the
// constants gives PredefinedStruct(n).
func (t PredefinedStruct) String() string {
	if int(t) < len(predefinedStructNames) {
		return predefinedStructNames[t]
	}
	return "PredefinedStruct(" + strconv.Itoa(int(t)) + ")"
}

// intoValueType is From<PredefinedStruct> for ValueType in krilla
// src/interchange/metadata/xmp.rs.
func (t PredefinedStruct) intoValueType() ValueType {
	return ValueType{kind: valueTypePredefinedStruct, predefined: t}
}

// Property is Property in krilla src/interchange/metadata/xmp.rs.
//
// A single XMP property in a Namespace.
//
// Also used for the fields of a struct value, which are named values in a
// namespace just like top-level properties.
type Property struct {
	// Namespace is the namespace this property belongs to.
	Namespace Namespace
	// Name is the property name within the namespace.
	Name string
	// Value is the property value.
	Value Value
}

// NewProperty is Property::new in krilla src/interchange/metadata/xmp.rs.
//
// Create a new XMP property.
func NewProperty(namespace Namespace, name string, value Value) Property {
	return Property{Namespace: namespace, Name: name, Value: value}
}

// valueKind is the case of the enum Value in krilla
// src/interchange/metadata/xmp.rs.
type valueKind uint8

// The cases of Value.
const (
	valueText valueKind = iota
	valueURI
	valueBool
	valueInteger
	valueReal
	valueRational
	valueDate
	valueOrderedArray
	valueUnorderedArray
	valueAlternativeArray
	valueLanguageAlternative
	valueStruct
)

// Value is Value in krilla src/interchange/metadata/xmp.rs. A function
// gives each case, such as TextValue for Value::Text. The zero value is an
// empty text.
//
// An XMP property value.
//
// The cases mirror the value shapes supported by XMP/RDF: simple values, the
// three array kinds (rdf:Seq, rdf:Bag, rdf:Alt), language alternatives, and
// structs. URIs and URLs are written as URIValue, other derived types like
// agent names as TextValue.
type Value struct {
	kind valueKind
	// text is the String of Text and Uri.
	text string
	// b is the bool of Bool.
	b bool
	// i is the i64 of Integer, and the numerator of Rational.
	i int64
	// den is the denominator of Rational.
	den uint64
	// r is the f64 of Real.
	r float64
	// date is the DateTime of Date.
	date DateTime
	// items holds the values of the three arrays.
	items []Value
	// langs holds the entries of LanguageAlternative.
	langs []LangText
	// fields holds the fields of Struct.
	fields []Property
}

// LangText is an entry of Value::LanguageAlternative in krilla
// src/interchange/metadata/xmp.rs: an optional RFC 3066 language tag and
// its text. A nil language is None, which krilla writes as x-default.
type LangText struct {
	Lang *string
	Text string
}

// TextValue is Value::Text and Value::text in krilla
// src/interchange/metadata/xmp.rs.
//
// A text value.
func TextValue(value string) Value {
	return Value{kind: valueText, text: value}
}

// URIValue is Value::Uri and Value::uri in krilla
// src/interchange/metadata/xmp.rs.
//
// A URI or URL, written as an rdf:resource attribute.
func URIValue(value string) Value {
	return Value{kind: valueURI, text: value}
}

// BoolValue is Value::Bool in krilla src/interchange/metadata/xmp.rs.
//
// A boolean value.
func BoolValue(value bool) Value {
	return Value{kind: valueBool, b: value}
}

// IntegerValue is Value::Integer in krilla
// src/interchange/metadata/xmp.rs.
//
// An integer value.
func IntegerValue(value int64) Value {
	return Value{kind: valueInteger, i: value}
}

// RealValue is Value::Real in krilla src/interchange/metadata/xmp.rs.
//
// A floating-point value. Must be finite.
func RealValue(value float64) Value {
	return Value{kind: valueReal, r: value}
}

// RationalValue is Value::Rational in krilla
// src/interchange/metadata/xmp.rs.
//
// A rational number, given as numerator and denominator. The denominator
// must not be zero.
func RationalValue(numerator int64, denominator uint64) Value {
	return Value{kind: valueRational, i: numerator, den: denominator}
}

// DateValue is Value::Date in krilla src/interchange/metadata/xmp.rs.
//
// A date value.
func DateValue(value DateTime) Value {
	return Value{kind: valueDate, date: value}
}

// OrderedArrayValue is Value::OrderedArray in krilla
// src/interchange/metadata/xmp.rs.
//
// An ordered array (rdf:Seq).
func OrderedArrayValue(items ...Value) Value {
	return Value{kind: valueOrderedArray, items: slices.Clone(items)}
}

// UnorderedArrayValue is Value::UnorderedArray in krilla
// src/interchange/metadata/xmp.rs.
//
// An unordered array (rdf:Bag).
func UnorderedArrayValue(items ...Value) Value {
	return Value{kind: valueUnorderedArray, items: slices.Clone(items)}
}

// AlternativeArrayValue is Value::AlternativeArray in krilla
// src/interchange/metadata/xmp.rs.
//
// An alternative array (rdf:Alt).
func AlternativeArrayValue(items ...Value) Value {
	return Value{kind: valueAlternativeArray, items: slices.Clone(items)}
}

// LanguageAlternativeValue is Value::LanguageAlternative in krilla
// src/interchange/metadata/xmp.rs.
//
// A language-alternative array (rdf:Alt of xml:lang-tagged text).
//
// Each entry pairs an optional RFC 3066 language tag (nil means
// x-default) with its text value.
func LanguageAlternativeValue(items ...LangText) Value {
	return Value{kind: valueLanguageAlternative, langs: slices.Clone(items)}
}

// StructValue is Value::Struct in krilla src/interchange/metadata/xmp.rs.
//
// A struct value, given as its fields.
func StructValue(fields ...Property) Value {
	return Value{kind: valueStruct, fields: slices.Clone(fields)}
}

// XMPErrorKind is the case of the enum XmpError in krilla
// src/interchange/metadata/xmp.rs.
type XMPErrorKind uint8

// The cases of XmpError.
const (
	// XMPErrorConflictingNamespace means that one namespace URI was used
	// with two different prefixes. The name is the namespace URI.
	XMPErrorConflictingNamespace XMPErrorKind = iota
	// XMPErrorConflictingPrefix means that one prefix was bound to two
	// different namespace URIs. The name is the prefix.
	XMPErrorConflictingPrefix
	// XMPErrorReservedPrefix means that a custom namespace used a prefix
	// reserved by a predefined XMP schema (e.g. dc, xmp, pdf) or by XML
	// (xml, xmlns). The name is the prefix.
	XMPErrorReservedPrefix
	// XMPErrorInvalidName means that a prefix, or the name of a property,
	// structure field or structure type, isn't a valid XML name without a
	// colon (an NCName). The name is that name.
	XMPErrorInvalidName
	// XMPErrorDuplicateSchema means that more than one schema was attached
	// for the same namespace. The name is the namespace URI.
	XMPErrorDuplicateSchema
	// XMPErrorInvalidNumber means that a real value was NaN or infinite, or
	// a rational value had a zero denominator. The name is the name of the
	// containing property.
	XMPErrorInvalidNumber
	// XMPErrorInvalidValue means that a value didn't match the type its
	// schema declared for it. The name is the name of the containing
	// property.
	XMPErrorInvalidValue
	// XMPErrorMissingProperty means that a property or structure field that
	// its schema marked as required was missing. The name is the name of the
	// property or field.
	XMPErrorMissingProperty
	// XMPErrorManagedProperty means that a property that krilla writes
	// itself was set. Such properties must be set via Metadata, if at all.
	// The name is the qualified name of the property.
	XMPErrorManagedProperty
	// XMPErrorDuplicateProperty means that a property or structure field was
	// set more than once. The name is its qualified name.
	XMPErrorDuplicateProperty
)

// XMPError is XmpError in krilla src/interchange/metadata/xmp.rs, and
// KrillaError::Xmp of krilla src/error.rs, which wraps it. It wraps
// okage.ErrPDFXMP (D21, D99), and its text is the Display text of
// KrillaError::Xmp.
//
// An invalid set of custom XMP properties or schemas.
type XMPError struct {
	Kind XMPErrorKind
	Name string
}

// Error gives the Display text of KrillaError::Xmp in krilla
// src/error.rs, after the text of okage.ErrPDFXMP.
func (e *XMPError) Error() string {
	var text string
	switch e.Kind {
	case XMPErrorConflictingNamespace:
		text = fmt.Sprintf("the namespace %s was declared with multiple different prefixes", e.Name)
	case XMPErrorConflictingPrefix:
		text = fmt.Sprintf("the prefix %s was bound to two different namespaces", e.Name)
	case XMPErrorReservedPrefix:
		text = fmt.Sprintf("the prefix %s is reserved by a predefined XMP schema", e.Name)
	case XMPErrorInvalidName:
		text = strconv.Quote(e.Name) + " is not a valid XML name"
	case XMPErrorDuplicateSchema:
		text = fmt.Sprintf("the namespace %s was described by multiple schemas", e.Name)
	case XMPErrorInvalidNumber:
		text = fmt.Sprintf("the property %s contained an invalid number", e.Name)
	case XMPErrorInvalidValue:
		text = fmt.Sprintf("the property %s doesn't match its schema", e.Name)
	case XMPErrorMissingProperty:
		text = fmt.Sprintf("the required property %s is missing", e.Name)
	case XMPErrorManagedProperty:
		text = fmt.Sprintf("the property %s is managed by krilla", e.Name)
	default:
		text = fmt.Sprintf("the property %s was set more than once", e.Name)
	}
	return okage.ErrPDFXMP.Error() + ": " + text
}

// Unwrap returns okage.ErrPDFXMP.
func (e *XMPError) Unwrap() error {
	return okage.ErrPDFXMP
}

// xmpError returns an XMPError of the kind and the name.
func xmpError(kind XMPErrorKind, name string) error {
	return &XMPError{Kind: kind, Name: name}
}
