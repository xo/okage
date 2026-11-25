// Copyright 2023 The xmp-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmpwriter

// This file ports xmp-writer src/pdfa.rs, the module of the feature pdfa,
// which upstream enables by default. The port has no features, so the
// module is always there.
//
// PDF/A extension schema description.

// PDFAExtSchemaWriter is PdfAExtSchemaWriter in xmp-writer src/pdfa.rs.
//
// Write a extension schema description.
//
// Created by PDFAExtSchemasWriter.AddSchema.
type PDFAExtSchemaWriter struct {
	*Struct
}

// Namespace is PdfAExtSchemaWriter::namespace in xmp-writer src/pdfa.rs.
//
// Write the pdfaSchema:schema, pdfaSchema:namespaceURI, and
// pdfaSchema:prefix properties.
//
// The values are extracted from the given namespace.
func (s *PDFAExtSchemaWriter) Namespace(namespace Namespace) *PDFAExtSchemaWriter {
	s.schema(namespace.Name() + " schema")
	s.namespaceURI(namespace.URL())
	s.prefix(namespace.Prefix())
	return s
}

// schema is PdfAExtSchemaWriter::schema in xmp-writer src/pdfa.rs.
//
// Write the pdfaSchema:schema property.
//
// Description of the extension schema.
func (s *PDFAExtSchemaWriter) schema(schema string) {
	s.Element("schema", NamespacePdfASchema).Value(Text(schema))
}

// namespaceURI is PdfAExtSchemaWriter::namespace_uri in xmp-writer
// src/pdfa.rs.
//
// Write the pdfaSchema:namespaceURI property.
//
// The namespace URI of the extension schema.
func (s *PDFAExtSchemaWriter) namespaceURI(uri string) {
	s.Element("namespaceURI", NamespacePdfASchema).Value(Text(uri))
}

// prefix is PdfAExtSchemaWriter::prefix in xmp-writer src/pdfa.rs.
//
// Write the pdfaSchema:prefix property.
//
// The prefix of the extension schema.
func (s *PDFAExtSchemaWriter) prefix(prefix string) {
	s.Element("prefix", NamespacePdfASchema).Value(Text(prefix))
}

// Properties is PdfAExtSchemaWriter::properties in xmp-writer src/pdfa.rs.
//
// Start writing the pdfaSchema:property sequence.
//
// Describes the properties of the extension schema.
func (s *PDFAExtSchemaWriter) Properties() *PDFAExtPropertiesWriter {
	return &PDFAExtPropertiesWriter{s.Element("property", NamespacePdfASchema).Array(RdfCollectionSeq)}
}

// ValueTypes is PdfAExtSchemaWriter::value_types in xmp-writer src/pdfa.rs.
//
// Start writing the pdfaSchema:valueType sequence.
//
// Describes the value types of the extension schema.
func (s *PDFAExtSchemaWriter) ValueTypes() *PDFAExtTypesWriter {
	return &PDFAExtTypesWriter{s.Element("valueType", NamespacePdfASchema).Array(RdfCollectionSeq)}
}

// PDFAExtPropertyWriter is PdfAExtPropertyWriter in xmp-writer
// src/pdfa.rs.
//
// Write a property of an extension schema.
//
// Created by PDFAExtPropertiesWriter.AddProperty.
type PDFAExtPropertyWriter struct {
	*Struct
}

// Name is PdfAExtPropertyWriter::name in xmp-writer src/pdfa.rs.
//
// Write the pdfaProperty:name property.
//
// Name of the property.
func (p *PDFAExtPropertyWriter) Name(name string) *PDFAExtPropertyWriter {
	p.Element("name", NamespacePdfAProperty).Value(Text(name))
	return p
}

// ValueType is PdfAExtPropertyWriter::value_type in xmp-writer
// src/pdfa.rs.
//
// Write the pdfaProperty:valueType property.
//
// The value type of the property. Shall either be defined in the XMP
// specification or in the extension schema.
func (p *PDFAExtPropertyWriter) ValueType(valueType string) *PDFAExtPropertyWriter {
	p.Element("valueType", NamespacePdfAProperty).Value(Text(valueType))
	return p
}

// Category is PdfAExtPropertyWriter::category in xmp-writer src/pdfa.rs.
//
// Write the pdfaProperty:category property.
//
// Whether the property is generated through the document's contents
// (internal is true) or input by the user (internal is false).
func (p *PDFAExtPropertyWriter) Category(internal bool) *PDFAExtPropertyWriter {
	category := Text("external")
	if internal {
		category = "internal"
	}
	p.Element("category", NamespacePdfAProperty).Value(category)
	return p
}

// Description is PdfAExtPropertyWriter::description in xmp-writer
// src/pdfa.rs.
//
// Write the pdfaProperty:description property.
//
// Description of the property.
func (p *PDFAExtPropertyWriter) Description(description string) *PDFAExtPropertyWriter {
	p.Element("description", NamespacePdfAProperty).Value(Text(description))
	return p
}

// PDFAExtTypeWriter is PdfAExtTypeWriter in xmp-writer src/pdfa.rs.
//
// Write a value type of an extension schema.
//
// Created by PDFAExtTypesWriter.AddValueType.
type PDFAExtTypeWriter struct {
	*Struct
}

// Name is PdfAExtTypeWriter::name in xmp-writer src/pdfa.rs.
//
// Write the pdfaType:type property.
//
// The name of the value type.
func (t *PDFAExtTypeWriter) Name(name string) *PDFAExtTypeWriter {
	t.Element("type", NamespacePdfAType).Value(Text(name))
	return t
}

// Namespace is PdfAExtTypeWriter::namespace in xmp-writer src/pdfa.rs.
//
// Write the pdfaType:namespaceURI and pdfaType:prefix properties.
func (t *PDFAExtTypeWriter) Namespace(namespace Namespace) *PDFAExtTypeWriter {
	t.NamespaceURI(namespace.URL())
	t.Prefix(namespace.Prefix())
	return t
}

// NamespaceURI is PdfAExtTypeWriter::namespace_uri in xmp-writer
// src/pdfa.rs.
//
// Write the pdfaType:namespaceURI property.
//
// The namespace URI of the value type. Consider calling Namespace instead.
func (t *PDFAExtTypeWriter) NamespaceURI(uri string) *PDFAExtTypeWriter {
	t.Element("namespaceURI", NamespacePdfAType).Value(Text(uri))
	return t
}

// Prefix is PdfAExtTypeWriter::prefix in xmp-writer src/pdfa.rs.
//
// Write the pdfaType:prefix property.
//
// The prefix of the value type. Consider calling Namespace instead.
func (t *PDFAExtTypeWriter) Prefix(prefix string) *PDFAExtTypeWriter {
	t.Element("prefix", NamespacePdfAType).Value(Text(prefix))
	return t
}

// Description is PdfAExtTypeWriter::description in xmp-writer src/pdfa.rs.
//
// Write the pdfaType:description property.
//
// Human-readable description of the value type.
func (t *PDFAExtTypeWriter) Description(description string) *PDFAExtTypeWriter {
	t.Element("description", NamespacePdfAType).Value(Text(description))
	return t
}

// Fields is PdfAExtTypeWriter::fields in xmp-writer src/pdfa.rs.
//
// Start writing the pdfaType:field sequence.
//
// Describes the fields of the value type.
func (t *PDFAExtTypeWriter) Fields() *PDFAExtTypeFieldsWriter {
	return &PDFAExtTypeFieldsWriter{t.Element("field", NamespacePdfAType).Array(RdfCollectionSeq)}
}

// PDFAExtTypeFieldWriter is PdfAExtTypeFieldWriter in xmp-writer
// src/pdfa.rs.
//
// Write a field of an extension schema value type.
//
// Created by PDFAExtTypeFieldsWriter.AddField.
type PDFAExtTypeFieldWriter struct {
	*Struct
}

// Name is PdfAExtTypeFieldWriter::name in xmp-writer src/pdfa.rs.
//
// Write the pdfaField:name property.
//
// Name of the field in the type.
func (f *PDFAExtTypeFieldWriter) Name(name string) *PDFAExtTypeFieldWriter {
	f.Element("name", NamespacePdfAField).Value(Text(name))
	return f
}

// ValueType is PdfAExtTypeFieldWriter::value_type in xmp-writer
// src/pdfa.rs.
//
// Write the pdfaField:valueType property.
//
// The value type of the field. Shall either be defined in the XMP
// specification or in the extension schema.
func (f *PDFAExtTypeFieldWriter) ValueType(valueType string) *PDFAExtTypeFieldWriter {
	f.Element("valueType", NamespacePdfAField).Value(Text(valueType))
	return f
}

// Description is PdfAExtTypeFieldWriter::description in xmp-writer
// src/pdfa.rs.
//
// Write the pdfaField:category property.
//
// Human-readable description of the field in the type.
func (f *PDFAExtTypeFieldWriter) Description(description string) *PDFAExtTypeFieldWriter {
	f.Element("description", NamespacePdfAField).Value(Text(description))
	return f
}

// PDFAExtTypeFieldsWriter is PdfAExtTypeFieldsWriter in xmp-writer
// src/pdfa.rs.
//
// Write an array of extension schema value type fields.
//
// Created by PDFAExtTypeWriter.Fields.
type PDFAExtTypeFieldsWriter struct {
	*Array
}

// AddField is PdfAExtTypeFieldsWriter::add_field in xmp-writer
// src/pdfa.rs.
//
// Start writing a field.
func (f *PDFAExtTypeFieldsWriter) AddField() *PDFAExtTypeFieldWriter {
	return &PDFAExtTypeFieldWriter{f.Element().Obj()}
}

// PDFAExtPropertiesWriter is PdfAExtPropertiesWriter in xmp-writer
// src/pdfa.rs.
//
// Write an array of extension schema properties.
//
// Created by PDFAExtSchemaWriter.Properties.
type PDFAExtPropertiesWriter struct {
	*Array
}

// AddProperty is PdfAExtPropertiesWriter::add_property in xmp-writer
// src/pdfa.rs.
//
// Add a property.
func (p *PDFAExtPropertiesWriter) AddProperty() *PDFAExtPropertyWriter {
	return &PDFAExtPropertyWriter{p.Element().Obj()}
}

// PDFAExtTypesWriter is PdfAExtTypesWriter in xmp-writer src/pdfa.rs.
//
// Write an array of extension schema value types.
//
// Created by PDFAExtSchemaWriter.ValueTypes.
type PDFAExtTypesWriter struct {
	*Array
}

// AddValueType is PdfAExtTypesWriter::add_value_type in xmp-writer
// src/pdfa.rs.
//
// Start writing a value type.
func (t *PDFAExtTypesWriter) AddValueType() *PDFAExtTypeWriter {
	return &PDFAExtTypeWriter{t.Element().Obj()}
}

// PDFAExtSchemasWriter is PdfAExtSchemasWriter in xmp-writer src/pdfa.rs.
//
// Write a set of extension schema descriptions.
//
// Created by Writer.ExtensionSchemas. Check PDF/A-1 TechNote 0008 to learn
// which schemas and properties need to be described.
type PDFAExtSchemasWriter struct {
	*Array
}

// AddSchema is PdfAExtSchemasWriter::add_schema in xmp-writer src/pdfa.rs.
//
// Start writing a schema.
func (s *PDFAExtSchemasWriter) AddSchema() *PDFAExtSchemaWriter {
	return &PDFAExtSchemaWriter{s.Element().Obj()}
}

// PDFAID is PdfAExtSchemasWriter::pdfaid in xmp-writer src/pdfa.rs.
//
// Describe the pdfaid schema.
//
// If corrigendum is true, the pdfaid:corr property is added.
func (s *PDFAExtSchemasWriter) PDFAID(corrigendum bool) *PDFAExtSchemasWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespacePdfAID)
	properties := schema.Properties()

	p := properties.AddProperty().
		Category(true).
		Description("Part of PDF/A standard").
		Name("part").
		ValueType("Integer")
	p.Finish()

	p = properties.AddProperty().
		Category(true).
		Description("Amendment of PDF/A standard").
		Name("amd").
		ValueType("Text")
	p.Finish()

	if corrigendum {
		p = properties.AddProperty().
			Category(true).
			Description("Corrigendum of PDF/A standard").
			Name("corr").
			ValueType("Text")
		p.Finish()
	}

	p = properties.AddProperty().
		Category(true).
		Description("Conformance level of PDF/A standard").
		Name("conformance").
		ValueType("Text")
	p.Finish()

	properties.Finish()
	schema.Finish()
	return s
}

// PDFUAID is PdfAExtSchemasWriter::pdfua_id in xmp-writer src/pdfa.rs.
//
// Start describing the pdfuaid schema.
func (s *PDFAExtSchemasWriter) PDFUAID() *PDFUAIDSchemaWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespacePdfUAID)
	return &PDFUAIDSchemaWriter{schema}
}

// PDF is PdfAExtSchemasWriter::pdf in xmp-writer src/pdfa.rs.
//
// Start describing the pdf schema.
func (s *PDFAExtSchemasWriter) PDF() *AdobePDFDescsWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespaceAdobePdf)
	return &AdobePDFDescsWriter{schema}
}

// XMP is PdfAExtSchemasWriter::xmp in xmp-writer src/pdfa.rs.
//
// Start describing the xmp schema.
func (s *PDFAExtSchemasWriter) XMP() *XMPDescsWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespaceXmp)
	return &XMPDescsWriter{schema}
}

// XMPMediaManagement is PdfAExtSchemasWriter::xmp_media_management in
// xmp-writer src/pdfa.rs.
//
// Start describing the xmpMM schema.
func (s *PDFAExtSchemasWriter) XMPMediaManagement() *XMPMMDescsWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespaceXmpMedia)
	return &XMPMMDescsWriter{schema}
}

// PagedText is PdfAExtSchemasWriter::paged_text in xmp-writer src/pdfa.rs.
//
// Start describing the xmpTPg schema.
func (s *PDFAExtSchemasWriter) PagedText() *PagedTextDescsWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespaceXmpPaged)
	return &PagedTextDescsWriter{schema}
}

// ResourceEvent is PdfAExtSchemasWriter::resource_event in xmp-writer
// src/pdfa.rs.
//
// Start describing the stEvt auxiliary schema.
func (s *PDFAExtSchemasWriter) ResourceEvent() *ResourceEventDescsWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespaceXmpResourceEvent)
	return &ResourceEventDescsWriter{schema}
}

// Thumbnail is PdfAExtSchemasWriter::thumbnail in xmp-writer src/pdfa.rs.
//
// Start describing the xmpGImg schema.
func (s *PDFAExtSchemasWriter) Thumbnail() *ThumbnailSchemaWriter {
	schema := s.AddSchema()
	schema.Namespace(NamespaceXmpImage)
	return &ThumbnailSchemaWriter{schema}
}

// describe adds the description of one property, as each method describe_
// of upstream does. The property is closed at once, as its writer of
// upstream is dropped at the end of the statement.
func describe(p *PDFAExtPropertiesWriter, internal bool, description, name, valueType string) {
	p.AddProperty().
		Category(internal).
		Description(description).
		Name(name).
		ValueType(valueType).
		Finish()
}

// XMPPropertiesWriter is XmpPropertiesWriter in xmp-writer src/pdfa.rs.
//
// Writer for the property descriptions of the xmp schema.
//
// Only contains methods for properties that are defined in XMP 2005 or
// later. Created by XMPDescsWriter.Properties.
type XMPPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribeLabel is XmpPropertiesWriter::describe_label in xmp-writer
// src/pdfa.rs.
//
// Describe the xmp:Label property.
func (w *XMPPropertiesWriter) DescribeLabel() *XMPPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, false, "A user-defined label for the resource", "Label", "Text")
	return w
}

// DescribeRating is XmpPropertiesWriter::describe_rating in xmp-writer
// src/pdfa.rs.
//
// Describe the xmp:Rating property.
func (w *XMPPropertiesWriter) DescribeRating() *XMPPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, false, "A user-assigned rating of the resource", "Rating", "Integer")
	return w
}

// DescribeAll is XmpPropertiesWriter::describe_all in xmp-writer
// src/pdfa.rs. Upstream takes the writer by value and drops it, so the
// port finishes it.
//
// Describe all properties of the xmp schema.
func (w *XMPPropertiesWriter) DescribeAll() {
	w.DescribeLabel().DescribeRating()
	w.Finish()
}

// XMPDescsWriter is XmpDescsWriter in xmp-writer src/pdfa.rs.
//
// Writer for describing the XMP schema.
//
// Created by PDFAExtSchemasWriter.XMP.
type XMPDescsWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is XmpDescsWriter::properties in xmp-writer src/pdfa.rs.
//
// Start describing the properties of the xmp schema.
func (w *XMPDescsWriter) Properties() *XMPPropertiesWriter {
	return &XMPPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// XMPMMPropertiesWriter is XmpMMPropertiesWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the property descriptions of the xmpMM schema.
//
// Created by XMPMMDescsWriter.Properties.
type XMPMMPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribeInstanceID is XmpMMPropertiesWriter::describe_instance_id in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpMM:InstanceID property.
func (w *XMPMMPropertiesWriter) DescribeInstanceID() *XMPMMPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "UUID based identifier for specific incarnation of a document", "InstanceID", "Text")
	return w
}

// DescribeIngredients is XmpMMPropertiesWriter::describe_ingredients in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpMM:Ingredients property.
func (w *XMPMMPropertiesWriter) DescribeIngredients() *XMPMMPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "List of ingredients that were used to create a document", "Ingredients", "ResourceRef")
	return w
}

// DescribeOriginalDocID is XmpMMPropertiesWriter::describe_original_doc_id
// in xmp-writer src/pdfa.rs.
//
// Describe the xmpMM:OriginalDocumentID property.
func (w *XMPMMPropertiesWriter) DescribeOriginalDocID() *XMPMMPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "UUID based identifier for original document from which a document is derived", "OriginalDocumentID", "Text")
	return w
}

// DescribePantry is XmpMMPropertiesWriter::describe_pantry in xmp-writer
// src/pdfa.rs.
//
// Describe the xmpMM:Pantry property.
func (w *XMPMMPropertiesWriter) DescribePantry() *XMPMMPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "List of ingredients that were used to create a document", "Pantry", "ResourceRef")
	return w
}

// DescribeAll is XmpMMPropertiesWriter::describe_all in xmp-writer
// src/pdfa.rs. The port finishes the writer, as upstream drops it.
//
// Describe all properties of the xmpMM schema.
func (w *XMPMMPropertiesWriter) DescribeAll() {
	w.DescribeInstanceID().
		DescribeIngredients().
		DescribeOriginalDocID().
		DescribePantry()
	w.Finish()
}

// XMPMMDescsWriter is XmpMMDescsWriter in xmp-writer src/pdfa.rs.
//
// Writer for describing the XMP Media Management schema.
//
// Created by PDFAExtSchemasWriter.XMPMediaManagement.
type XMPMMDescsWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is XmpMMDescsWriter::properties in xmp-writer src/pdfa.rs.
//
// Start describing the properties of the xmpMM schema.
func (w *XMPMMDescsWriter) Properties() *XMPMMPropertiesWriter {
	return &XMPMMPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// AdobePDFPropertiesWriter is AdobePdfPropertiesWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the property descriptions of the pdf schema.
//
// Created by AdobePDFDescsWriter.Properties.
type AdobePDFPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribeKeywords is AdobePdfPropertiesWriter::describe_keywords in
// xmp-writer src/pdfa.rs.
//
// Describe the pdf:Keywords property.
//
// Optional even if present, see PDF/A-1 TechNote 0008.
func (w *AdobePDFPropertiesWriter) DescribeKeywords() *AdobePDFPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, false, "Keywords associated with the document", "Keywords", "Text")
	return w
}

// DescribePDFVersion is AdobePdfPropertiesWriter::describe_pdf_version in
// xmp-writer src/pdfa.rs.
//
// Describe the pdf:PDFVersion property.
//
// Optional even if present, see PDF/A-1 TechNote 0008.
func (w *AdobePDFPropertiesWriter) DescribePDFVersion() *AdobePDFPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Version of the PDF specification to which the document conforms", "PDFVersion", "Text")
	return w
}

// DescribeProducer is AdobePdfPropertiesWriter::describe_producer in
// xmp-writer src/pdfa.rs.
//
// Describe the pdf:Producer property.
//
// Optional even if present, see PDF/A-1 TechNote 0008.
func (w *AdobePDFPropertiesWriter) DescribeProducer() *AdobePDFPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Name of the application that created the PDF document", "Producer", "Text")
	return w
}

// DescribeTrapped is AdobePdfPropertiesWriter::describe_trapped in
// xmp-writer src/pdfa.rs.
//
// Describe the pdf:Trapped property.
func (w *AdobePDFPropertiesWriter) DescribeTrapped() *AdobePDFPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Whether the document has been trapped", "Trapped", "Text")
	return w
}

// DescribeAll is AdobePdfPropertiesWriter::describe_all in xmp-writer
// src/pdfa.rs. The port finishes the writer, as upstream drops it.
//
// Describe all properties of the pdf schema.
func (w *AdobePDFPropertiesWriter) DescribeAll() {
	w.DescribeKeywords().
		DescribePDFVersion().
		DescribeProducer().
		DescribeTrapped()
	w.Finish()
}

// AdobePDFDescsWriter is AdobePdfDescsWriter in xmp-writer src/pdfa.rs.
//
// Writer for describing the Adobe PDF extension schema.
//
// Created by PDFAExtSchemasWriter.PDF.
type AdobePDFDescsWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is AdobePdfDescsWriter::properties in xmp-writer src/pdfa.rs.
//
// Start describing the properties of the pdf schema.
func (w *AdobePDFDescsWriter) Properties() *AdobePDFPropertiesWriter {
	return &AdobePDFPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// PagedTextDescsWriter is PagedTextDescsWriter in xmp-writer src/pdfa.rs.
//
// Writer for describing the Paged Text extension schema.
//
// Created by PDFAExtSchemasWriter.PagedText.
type PagedTextDescsWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is PagedTextDescsWriter::properties in xmp-writer
// src/pdfa.rs.
//
// Start describing the properties of the xmpTPg schema.
func (w *PagedTextDescsWriter) Properties() *PagedTextPropertiesWriter {
	return &PagedTextPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// PagedTextPropertiesWriter is PagedTextPropertiesWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the property descriptions of the xmpTPg schema.
//
// Created by PagedTextDescsWriter.Properties.
type PagedTextPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribeKeywords is PagedTextPropertiesWriter::describe_keywords in
// xmp-writer src/pdfa.rs. It describes the colorants, as upstream does.
//
// Describe the xmpTPg:Colorants property.
func (w *PagedTextPropertiesWriter) DescribeKeywords() *PagedTextPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Colorants / swatches used in the document and its children", "Colorants", "Colorant")
	return w
}

// DescribeFonts is PagedTextPropertiesWriter::describe_fonts in xmp-writer
// src/pdfa.rs.
//
// Describe the xmpTPg:Fonts property.
func (w *PagedTextPropertiesWriter) DescribeFonts() *PagedTextPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Fonts used in the document and its children", "Fonts", "Font")
	return w
}

// DescribePlateNames is PagedTextPropertiesWriter::describe_plate_names in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpTPg:PlateNames property.
func (w *PagedTextPropertiesWriter) DescribePlateNames() *PagedTextPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Plate names used in the document and its children", "PlateNames", "Text")
	return w
}

// DescribeAll is PagedTextPropertiesWriter::describe_all in xmp-writer
// src/pdfa.rs. The port finishes the writer, as upstream drops it.
//
// Describe all properties of the xmpTPg schema.
func (w *PagedTextPropertiesWriter) DescribeAll() {
	w.DescribeKeywords().DescribeFonts().DescribePlateNames()
	w.Finish()
}

// ResourceEventDescsWriter is ResourceEventDescsWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the auxiliary ResourceEvent extension schema.
//
// Created by PDFAExtSchemasWriter.ResourceEvent.
type ResourceEventDescsWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is ResourceEventDescsWriter::properties in xmp-writer
// src/pdfa.rs.
//
// Start describing the properties of the stEvt schema.
func (w *ResourceEventDescsWriter) Properties() *ResourceEventPropertiesWriter {
	return &ResourceEventPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// ResourceEventPropertiesWriter is ResourceEventPropertiesWriter in
// xmp-writer src/pdfa.rs.
//
// Writer for the property descriptions of the stEvt schema.
//
// Created by ResourceEventDescsWriter.Properties.
type ResourceEventPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribeChanged is ResourceEventPropertiesWriter::describe_changed in
// xmp-writer src/pdfa.rs.
//
// Describe the stEvt:changed property.
func (w *ResourceEventPropertiesWriter) DescribeChanged() *ResourceEventPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, false, "semicolon-delimited list of the parts of the resource that were changed since the previous event history", "changed", "Text")
	return w
}

// ThumbnailSchemaWriter is ThumbnailSchemaWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the thumbnail extension schema.
//
// Created by PDFAExtSchemasWriter.Thumbnail.
type ThumbnailSchemaWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is ThumbnailSchemaWriter::properties in xmp-writer
// src/pdfa.rs.
//
// Start describing the properties of the xmpGImg schema.
func (w *ThumbnailSchemaWriter) Properties() *ThumbnailPropertiesWriter {
	return &ThumbnailPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// ThumbnailPropertiesWriter is ThumbnailPropertiesWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the property descriptions of the xmpGImg schema.
//
// Created by ThumbnailSchemaWriter.Properties.
type ThumbnailPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribeHeight is ThumbnailPropertiesWriter::describe_height in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpGImg:height property.
func (w *ThumbnailPropertiesWriter) DescribeHeight() *ThumbnailPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Height of the image in pixels", "height", "Integer")
	return w
}

// DescribeImage is ThumbnailPropertiesWriter::describe_image in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpGImg:image property.
func (w *ThumbnailPropertiesWriter) DescribeImage() *ThumbnailPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, false, "Thumbnail image data in base64 encoding", "image", "Text")
	return w
}

// DescribeWidth is ThumbnailPropertiesWriter::describe_width in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpGImg:width property.
func (w *ThumbnailPropertiesWriter) DescribeWidth() *ThumbnailPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Width of the image in pixels", "width", "Integer")
	return w
}

// DescribeFormat is ThumbnailPropertiesWriter::describe_format in
// xmp-writer src/pdfa.rs.
//
// Describe the xmpGImg:format property.
func (w *ThumbnailPropertiesWriter) DescribeFormat() *ThumbnailPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "The image encoding, i.e. JPEG", "format", "Closed Choice")
	return w
}

// DescribeAll is ThumbnailPropertiesWriter::describe_all in xmp-writer
// src/pdfa.rs. The port finishes the writer, as upstream drops it.
//
// Describe all properties of the xmpGImg schema.
func (w *ThumbnailPropertiesWriter) DescribeAll() {
	w.DescribeHeight().
		DescribeImage().
		DescribeWidth().
		DescribeFormat()
	w.Finish()
}

// PDFUAIDSchemaWriter is PdfUAIdSchemaWriter in xmp-writer src/pdfa.rs.
//
// Writer for the PDF/UA ID extension schema.
//
// Created by PDFAExtSchemasWriter.PDFUAID.
type PDFUAIDSchemaWriter struct {
	*PDFAExtSchemaWriter
}

// Properties is PdfUAIdSchemaWriter::properties in xmp-writer src/pdfa.rs.
//
// Start describing the properties of the pdfuaId schema.
func (w *PDFUAIDSchemaWriter) Properties() *PDFUAIDPropertiesWriter {
	return &PDFUAIDPropertiesWriter{w.PDFAExtSchemaWriter.Properties()}
}

// PDFUAIDPropertiesWriter is PdfUAIdPropertiesWriter in xmp-writer
// src/pdfa.rs.
//
// Writer for the property descriptions of the pdfuaid schema.
//
// Created by PDFUAIDSchemaWriter.Properties.
type PDFUAIDPropertiesWriter struct {
	*PDFAExtPropertiesWriter
}

// DescribePart is PdfUAIdPropertiesWriter::describe_part in xmp-writer
// src/pdfa.rs.
//
// Describe the pdfuaid:part property.
func (w *PDFUAIDPropertiesWriter) DescribePart() *PDFUAIDPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Part Number of ISO 14289 (PDF/UA)", "part", "Integer")
	return w
}

// DescribeAmd is PdfUAIdPropertiesWriter::describe_amd in xmp-writer
// src/pdfa.rs.
//
// Describe the pdfuaid:amd property.
func (w *PDFUAIDPropertiesWriter) DescribeAmd() *PDFUAIDPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Amendment Number, Colon, and four-digit year of the publication of ISO 14289 (PDF/UA)", "amd", "Text")
	return w
}

// DescribeCorr is PdfUAIdPropertiesWriter::describe_corr in xmp-writer
// src/pdfa.rs.
//
// Describe the pdfuaid:corr property.
func (w *PDFUAIDPropertiesWriter) DescribeCorr() *PDFUAIDPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Number of the technical corrigendum to ISO 14289 (PDF/UA)", "corr", "Text")
	return w
}

// DescribeRev is PdfUAIdPropertiesWriter::describe_rev in xmp-writer
// src/pdfa.rs.
//
// Describe the pdfuaid:rev property.
func (w *PDFUAIDPropertiesWriter) DescribeRev() *PDFUAIDPropertiesWriter {
	describe(w.PDFAExtPropertiesWriter, true, "Revision of ISO 14289 (PDF/UA) as a four-digit year", "rev", "Integer")
	return w
}

// DescribeAll is PdfUAIdPropertiesWriter::describe_all in xmp-writer
// src/pdfa.rs. The port finishes the writer, as upstream drops it.
//
// Describe the applicable properties of the pdfuaid schema.
func (w *PDFUAIDPropertiesWriter) DescribeAll(mode PDFUAIDDescriptionMode) {
	writer := w.DescribePart()

	switch mode {
	case PDFUAIDDescriptionModeUA1:
		writer.DescribeAmd()
	case PDFUAIDDescriptionModeUA2:
		writer.DescribeRev()
	case PDFUAIDDescriptionModeAllAlive:
		writer.DescribeAmd().DescribeRev()
	case PDFUAIDDescriptionModeAll:
		writer.DescribeAmd().DescribeCorr().DescribeRev()
	}
	w.Finish()
}

// PDFUAIDDescriptionMode is PdfUAIdDescriptionMode in xmp-writer
// src/pdfa.rs. The default of upstream is
// PDFUAIDDescriptionModeAllAlive, which is not the zero value.
//
// Which properties of the pdfuaid schema to describe.
type PDFUAIDDescriptionMode uint8

// The values of PDFUAIDDescriptionMode.
const (
	// PDFUAIDDescriptionModeUA1 only describes properties defined by
	// PDF/UA-1. This omits pdfuaid:corr since no technical corrigenda to
	// PDF/UA exist and ISO discontinued publishing any.
	PDFUAIDDescriptionModeUA1 PDFUAIDDescriptionMode = iota
	// PDFUAIDDescriptionModeUA2 only describes properties defined by
	// PDF/UA-2.
	PDFUAIDDescriptionModeUA2
	// PDFUAIDDescriptionModeAllAlive describes all properties of the
	// pdfuaid schema, except for pdfuaid:corr, since no technical corrigenda
	// to PDF/UA exist and ISO discontinued publishing any.
	PDFUAIDDescriptionModeAllAlive
	// PDFUAIDDescriptionModeAll describes all properties of the pdfuaid
	// schema, including pdfuaid:corr.
	PDFUAIDDescriptionModeAll
)
