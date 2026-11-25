// Copyright 2023 The xmp-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmpwriter

import (
	"slices"
)

// This file ports xmp-writer src/lib.rs. The macro deref! of upstream
// becomes an embedded field: each writer of a struct embeds *Struct, and
// each writer of an array embeds *Array, so that the methods of the struct
// or the array, such as Finish, are the methods of the writer.

// Writer is XmpWriter in xmp-writer src/lib.rs. Upstream keeps the
// namespaces in a BTreeSet. The port keeps them in a map, and Finish sorts
// them in the order of the derived Ord of upstream.
//
// The main writer struct.
//
// Use New to create a new instance and get the resulting XMP metadata by
// calling Finish.
type Writer struct {
	buf        []byte
	namespaces map[Namespace]struct{}
}

// New is XmpWriter::new in xmp-writer src/lib.rs.
//
// Create a new XMP writer.
func New() *Writer {
	return &Writer{namespaces: map[Namespace]struct{}{}}
}

// Element is XmpWriter::element in xmp-writer src/lib.rs.
//
// Add a custom element to the XMP metadata.
func (w *Writer) Element(name string, namespace Namespace) *Element {
	return startElement(w, name, namespace)
}

// Finish is XmpWriter::finish in xmp-writer src/lib.rs. An empty about is
// the None of upstream, which writes an empty attribute too.
//
// Finish the XMP metadata and return it as a string.
func (w *Writer) Finish(about string) string {
	buf := make([]byte, 0, 280+len(w.buf))
	buf = append(buf, "<?xpacket begin=\"\ufeff\" id=\"W5M0MpCehiHzreSzNTczkc9d\"?>"...)
	buf = append(buf, `<x:xmpmeta xmlns:x="adobe:ns:meta/" x:xmptk="xmp-writer"><rdf:RDF xmlns:rdf="`...)
	buf = append(buf, NamespaceRdf.URL()...)
	buf = append(buf, `"><rdf:Description rdf:about="`...)
	buf = append(buf, about...)
	buf = append(buf, '"')

	namespaces := make([]Namespace, 0, len(w.namespaces))
	for ns := range w.namespaces {
		if ns != NamespaceRdf {
			namespaces = append(namespaces, ns)
		}
	}
	slices.SortFunc(namespaces, Namespace.compare)
	for _, ns := range namespaces {
		buf = append(buf, " xmlns:"...)
		buf = append(buf, ns.Prefix()...)
		buf = append(buf, `="`...)
		buf = append(buf, ns.URL()...)
		buf = append(buf, `" `...)
	}

	buf = append(buf, '>')
	buf = append(buf, w.buf...)
	buf = append(buf, "</rdf:Description></rdf:RDF></x:xmpmeta><?xpacket end=\"r\"?>"...)
	return string(buf)
}

// XMP Dublin Core Schema.

// Contributor is XmpWriter::contributor in xmp-writer src/lib.rs.
//
// Write the dc:contributor property.
//
// All entities responsible for making contributions to the resource not
// listed in Writer.Creator.
func (w *Writer) Contributor(contributor ...string) *Writer {
	w.Element("contributor", NamespaceDublinCore).UnorderedArray(texts(contributor)...)
	return w
}

// Coverage is XmpWriter::coverage in xmp-writer src/lib.rs.
//
// Write the dc:coverage property.
//
// The scope of the resource.
func (w *Writer) Coverage(coverage string) *Writer {
	w.Element("coverage", NamespaceDublinCore).Value(Text(coverage))
	return w
}

// Creator is XmpWriter::creator in xmp-writer src/lib.rs.
//
// Write the dc:creator property.
//
// An entity primarily responsible for making the resource.
func (w *Writer) Creator(creator ...string) *Writer {
	w.Element("creator", NamespaceDublinCore).OrderedArray(texts(creator)...)
	return w
}

// Date is XmpWriter::date in xmp-writer src/lib.rs.
//
// Write the dc:date property.
//
// Date(s) that something happened to the resource.
func (w *Writer) Date(date ...DateTime) *Writer {
	items := make([]Type, len(date))
	for i, d := range date {
		items[i] = d
	}
	w.Element("date", NamespaceDublinCore).OrderedArray(items...)
	return w
}

// Description is XmpWriter::description in xmp-writer src/lib.rs.
//
// Write the dc:description property.
//
// An account of the resource, possibly in multiple languages.
func (w *Writer) Description(description ...LangAlt) *Writer {
	w.Element("description", NamespaceDublinCore).LanguageAlternative(description)
	return w
}

// Format is XmpWriter::format in xmp-writer src/lib.rs.
//
// Write the dc:format property.
//
// The mime type of the resource.
func (w *Writer) Format(mime string) *Writer {
	w.Element("format", NamespaceDublinCore).Value(Text(mime))
	return w
}

// Identifier is XmpWriter::identifier in xmp-writer src/lib.rs.
//
// Write the dc:identifier property.
//
// An unambiguous reference to the resource within a given context.
func (w *Writer) Identifier(id string) *Writer {
	w.Element("identifier", NamespaceDublinCore).Value(Text(id))
	return w
}

// Language is XmpWriter::language in xmp-writer src/lib.rs.
//
// Write the dc:language property.
//
// Languges used in the resource.
func (w *Writer) Language(lang ...LangID) *Writer {
	items := make([]Type, len(lang))
	for i, l := range lang {
		items[i] = l
	}
	w.Element("language", NamespaceDublinCore).UnorderedArray(items...)
	return w
}

// Publisher is XmpWriter::publisher in xmp-writer src/lib.rs.
//
// Write the dc:publisher property.
//
// Publishers of the resource.
func (w *Writer) Publisher(publisher ...string) *Writer {
	w.Element("publisher", NamespaceDublinCore).UnorderedArray(texts(publisher)...)
	return w
}

// Relation is XmpWriter::relation in xmp-writer src/lib.rs.
//
// Write the dc:relation property.
//
// List of related resources.
func (w *Writer) Relation(relation ...string) *Writer {
	w.Element("relation", NamespaceDublinCore).UnorderedArray(texts(relation)...)
	return w
}

// Rights is XmpWriter::rights in xmp-writer src/lib.rs.
//
// Write the dc:rights property.
//
// Informal rights statements, possibly in multiple languages.
func (w *Writer) Rights(rights ...LangAlt) *Writer {
	w.Element("rights", NamespaceDublinCore).LanguageAlternative(rights)
	return w
}

// Source is XmpWriter::source in xmp-writer src/lib.rs.
//
// Write the dc:source property.
//
// A related resource from which the described resource is derived.
func (w *Writer) Source(source string) *Writer {
	w.Element("source", NamespaceDublinCore).Value(Text(source))
	return w
}

// Subject is XmpWriter::subject in xmp-writer src/lib.rs.
//
// Write the dc:subject property.
//
// A list of phrases or keywords that specify the topic of the resource.
func (w *Writer) Subject(subject ...string) *Writer {
	w.Element("subject", NamespaceDublinCore).UnorderedArray(texts(subject)...)
	return w
}

// Title is XmpWriter::title in xmp-writer src/lib.rs.
//
// Write the dc:title property.
//
// A name given to the resource, possibly in multiple languages.
func (w *Writer) Title(title ...LangAlt) *Writer {
	w.Element("title", NamespaceDublinCore).LanguageAlternative(title)
	return w
}

// Type is XmpWriter::type_ in xmp-writer src/lib.rs.
//
// Write the dc:type property.
//
// The nature or genre of the resource. Please use Writer.Format to specify
// the mime type.
func (w *Writer) Type(kind ...string) *Writer {
	w.Element("type", NamespaceDublinCore).UnorderedArray(texts(kind)...)
	return w
}

// XMP Basic Schema.

// BaseURL is XmpWriter::base_url in xmp-writer src/lib.rs.
//
// Write the xmp:BaseURL property.
//
// The base URL for relative URLs in the document.
func (w *Writer) BaseURL(url string) *Writer {
	w.Element("BaseURL", NamespaceXmp).URI(url)
	return w
}

// CreateDate is XmpWriter::create_date in xmp-writer src/lib.rs.
//
// Write the xmp:CreateDate property.
//
// The date and time the resource was created.
func (w *Writer) CreateDate(date DateTime) *Writer {
	w.Element("CreateDate", NamespaceXmp).Value(date)
	return w
}

// CreatorTool is XmpWriter::creator_tool in xmp-writer src/lib.rs.
//
// Write the xmp:CreatorTool property.
//
// The name of the application used to create the resource.
func (w *Writer) CreatorTool(tool string) *Writer {
	w.Element("CreatorTool", NamespaceXmp).Value(Text(tool))
	return w
}

// XMPIdentifier is XmpWriter::xmp_identifier in xmp-writer src/lib.rs.
//
// Write the xmp:Identifier property.
//
// Unordered array of text strings that identify the resource. The
// Writer.IdqScheme method can be used to specify the scheme.
func (w *Writer) XMPIdentifier(id ...string) *Writer {
	w.Element("Identifier", NamespaceXmp).UnorderedArray(texts(id)...)
	return w
}

// Label is XmpWriter::label in xmp-writer src/lib.rs.
//
// Write the xmp:Label property.
//
// A user-defined label for the resource.
func (w *Writer) Label(label string) *Writer {
	w.Element("Label", NamespaceXmp).Value(Text(label))
	return w
}

// MetadataDate is XmpWriter::metadata_date in xmp-writer src/lib.rs.
//
// Write the xmp:MetadataDate property.
//
// The date and time the metadata for the resource was last changed.
func (w *Writer) MetadataDate(date DateTime) *Writer {
	w.Element("MetadataDate", NamespaceXmp).Value(date)
	return w
}

// ModifyDate is XmpWriter::modify_date in xmp-writer src/lib.rs.
//
// Write the xmp:ModifyDate property.
//
// The date and time the resource was last modified.
func (w *Writer) ModifyDate(date DateTime) *Writer {
	w.Element("ModifyDate", NamespaceXmp).Value(date)
	return w
}

// Nickname is XmpWriter::nickname in xmp-writer src/lib.rs.
//
// Write the xmp:Nickname property.
//
// A short informal name for the resource.
func (w *Writer) Nickname(nickname string) *Writer {
	w.Element("Nickname", NamespaceXmp).Value(Text(nickname))
	return w
}

// Rating is XmpWriter::rating in xmp-writer src/lib.rs.
//
// Write the xmp:Rating property.
//
// A user-assigned rating of the resource.
func (w *Writer) Rating(rating int64) *Writer {
	w.Element("Rating", NamespaceXmp).Value(Int64(rating))
	return w
}

// Thumbnails is XmpWriter::thumbnails in xmp-writer src/lib.rs.
//
// Start writing the xmp:Thumbnails property.
//
// A thumbnail image of the resource.
func (w *Writer) Thumbnails() *ThumbnailsWriter {
	return &ThumbnailsWriter{w.Element("Thumbnails", NamespaceXmp).Array(RdfCollectionAlt)}
}

// XMP Rights Management Schema.

// Certificate is XmpWriter::certificate in xmp-writer src/lib.rs.
//
// Write the xmpRights:Certificate property.
//
// A URL with a rights management certificate.
func (w *Writer) Certificate(cert string) *Writer {
	w.Element("Certificate", NamespaceXmpRights).Value(Text(cert))
	return w
}

// Marked is XmpWriter::marked in xmp-writer src/lib.rs.
//
// Write the xmpRights:Marked property.
//
// Whether the resource has been marked as rights managed. If false, the
// resource is in the public domain.
func (w *Writer) Marked(marked bool) *Writer {
	w.Element("Marked", NamespaceXmpRights).Value(Bool(marked))
	return w
}

// Owner is XmpWriter::owner in xmp-writer src/lib.rs.
//
// Write the xmpRights:Owner property.
//
// A list of people or organizations owning the resource.
func (w *Writer) Owner(owner ...string) *Writer {
	w.Element("Owner", NamespaceXmpRights).UnorderedArray(texts(owner)...)
	return w
}

// UsageTerms is XmpWriter::usage_terms in xmp-writer src/lib.rs.
//
// Write the xmpRights:UsageTerms property.
//
// Under what conditions the resource may be used.
func (w *Writer) UsageTerms(terms ...LangAlt) *Writer {
	w.Element("UsageTerms", NamespaceXmpRights).LanguageAlternative(terms)
	return w
}

// WebStatement is XmpWriter::web_statement in xmp-writer src/lib.rs.
//
// Write the xmpRights:WebStatement property.
//
// A URL with a rights management statement.
func (w *Writer) WebStatement(statement string) *Writer {
	w.Element("WebStatement", NamespaceXmpRights).Value(Text(statement))
	return w
}

// XMP Media Management Schema.

// DerivedFrom is XmpWriter::derived_from in xmp-writer src/lib.rs.
//
// Start writing the xmpMM:DerivedFrom property.
//
// The document from which this document is derived.
func (w *Writer) DerivedFrom() *ResourceRefWriter {
	return &ResourceRefWriter{w.Element("DerivedFrom", NamespaceXmpMedia).Obj()}
}

// DocumentID is XmpWriter::document_id in xmp-writer src/lib.rs.
//
// Write the xmpMM:DocumentID property.
//
// A common identifier for the document and all of its versions /
// renditions.
func (w *Writer) DocumentID(id string) *Writer {
	w.Element("DocumentID", NamespaceXmpMedia).Value(Text(id))
	return w
}

// History is XmpWriter::history in xmp-writer src/lib.rs.
//
// Start writing the xmpMM:History property.
//
// A list of actions taken on the document.
func (w *Writer) History() *ResourceEventsWriter {
	return &ResourceEventsWriter{w.Element("History", NamespaceXmpMedia).Array(RdfCollectionSeq)}
}

// Ingredients is XmpWriter::ingredients in xmp-writer src/lib.rs.
//
// Write the xmpMM:Ingredients property.
//
// A list of resources that were used to create the document.
func (w *Writer) Ingredients() *ResourceRefsWriter {
	return &ResourceRefsWriter{w.Element("Ingredients", NamespaceXmpMedia).Array(RdfCollectionBag)}
}

// InstanceID is XmpWriter::instance_id in xmp-writer src/lib.rs.
//
// Write the xmpMM:InstanceID property.
//
// A unique identifier for the rendition of the document, updated each time
// the document is saved.
func (w *Writer) InstanceID(id string) *Writer {
	w.Element("InstanceID", NamespaceXmpMedia).Value(Text(id))
	return w
}

// ManagedFrom is XmpWriter::managed_from in xmp-writer src/lib.rs.
//
// Start writing the xmpMM:ManagedFrom property.
//
// A reference to the document before it was managed.
func (w *Writer) ManagedFrom() *ResourceRefWriter {
	return &ResourceRefWriter{w.Element("ManagedFrom", NamespaceXmpMedia).Obj()}
}

// Manager is XmpWriter::manager in xmp-writer src/lib.rs.
//
// Write the xmpMM:Manager property.
//
// The name of the application that manages the document.
func (w *Writer) Manager(manager string) *Writer {
	w.Element("Manager", NamespaceXmpMedia).Value(Text(manager))
	return w
}

// ManageTo is XmpWriter::manage_to in xmp-writer src/lib.rs.
//
// Write the xmpMM:ManageTo property.
//
// The URI of the document in the management system.
func (w *Writer) ManageTo(uri string) *Writer {
	w.Element("ManageTo", NamespaceXmpMedia).URI(uri)
	return w
}

// ManageUI is XmpWriter::manage_ui in xmp-writer src/lib.rs.
//
// Write the xmpMM:ManageUI property.
//
// A web page that allows the user to manage the document.
func (w *Writer) ManageUI(uri string) *Writer {
	w.Element("ManageUI", NamespaceXmpMedia).URI(uri)
	return w
}

// ManagerVariant is XmpWriter::manager_variant in xmp-writer src/lib.rs.
//
// Write the xmpMM:ManagerVariant property.
//
// The name of the variant of the application that manages the document.
func (w *Writer) ManagerVariant(variant string) *Writer {
	w.Element("ManagerVariant", NamespaceXmpMedia).Value(Text(variant))
	return w
}

// OriginalDocID is XmpWriter::original_doc_id in xmp-writer src/lib.rs.
//
// Write the xmpMM:OriginalDocumentID property.
//
// The ID of the resource from which this document was derived.
func (w *Writer) OriginalDocID(id string) *Writer {
	w.Element("OriginalDocumentID", NamespaceXmpMedia).Value(Text(id))
	return w
}

// Pantry is XmpWriter::pantry in xmp-writer src/lib.rs.
//
// Start writing the xmpMM:Pantry property.
//
// An unordered array of structs with custom properties, each of which must
// have an xmpMM:InstanceID property.
func (w *Writer) Pantry() *PantryWriter {
	return &PantryWriter{w.Element("Pantry", NamespaceXmpMedia).Array(RdfCollectionBag)}
}

// RenditionClass is XmpWriter::rendition_class in xmp-writer src/lib.rs.
//
// Write the xmpMM:RenditionClass property.
//
// The type of the rendition. Shall be absent or RenditionClassDefault if
// this is not a derived document.
func (w *Writer) RenditionClass(class RenditionClass) *Writer {
	w.Element("RenditionClass", NamespaceXmpMedia).Value(class)
	return w
}

// RenditionParams is XmpWriter::rendition_params in xmp-writer src/lib.rs.
//
// Write the xmpMM:RenditionParams property.
//
// The parameters used to create the rendition.
func (w *Writer) RenditionParams(params string) *Writer {
	w.Element("RenditionParams", NamespaceXmpMedia).Value(Text(params))
	return w
}

// VersionID is XmpWriter::version_id in xmp-writer src/lib.rs.
//
// Write the xmpMM:VersionID property.
//
// A unique identifier for the version of the document.
func (w *Writer) VersionID(id string) *Writer {
	w.Element("VersionID", NamespaceXmpMedia).Value(Text(id))
	return w
}

// VersionRef is XmpWriter::version_ref in xmp-writer src/lib.rs.
//
// Start writing the xmpMM:Versions property.
//
// The list of versions of the document, starting with the oldest version.
func (w *Writer) VersionRef() *VersionsWriter {
	return &VersionsWriter{w.Element("Versions", NamespaceXmpMedia).Array(RdfCollectionSeq)}
}

// Basic Job Management.

// Jobs is XmpWriter::jobs in xmp-writer src/lib.rs.
//
// Start writing the xmpBJ:JobRef property.
//
// A reference to jobs in a system that involves this resource.
func (w *Writer) Jobs() *JobsWriter {
	return &JobsWriter{w.Element("JobRef", NamespaceXmpJobManagement).Array(RdfCollectionBag)}
}

// Paged-text.

// Colorants is XmpWriter::colorants in xmp-writer src/lib.rs.
//
// Start writing the xmpTPg:NPages property.
//
// Colorants used in the document.
func (w *Writer) Colorants() *ColorantsWriter {
	return &ColorantsWriter{w.Element("Colorants", NamespaceXmpPaged).Array(RdfCollectionSeq)}
}

// Fonts is XmpWriter::fonts in xmp-writer src/lib.rs.
//
// Start writing the xmpTPg:Fonts property.
//
// Fonts used in the document.
func (w *Writer) Fonts() *FontsWriter {
	return &FontsWriter{w.Element("Fonts", NamespaceXmpPaged).Array(RdfCollectionBag)}
}

// MaxPageSize is XmpWriter::max_page_size in xmp-writer src/lib.rs.
//
// Start writing the xmpTPg:MaxPageSize property.
//
// The maximum page size in the document.
func (w *Writer) MaxPageSize() *DimensionsWriter {
	return &DimensionsWriter{w.Element("MaxPageSize", NamespaceXmpPaged).Obj()}
}

// NumPages is XmpWriter::num_pages in xmp-writer src/lib.rs.
//
// Write the xmpTPg:NPages property.
//
// The number of pages in the document.
func (w *Writer) NumPages(num uint32) *Writer {
	w.Element("NPages", NamespaceXmpPaged).Value(Int64(num))
	return w
}

// PlateNames is XmpWriter::plate_names in xmp-writer src/lib.rs.
//
// Write the xmpTPg:PlateNames property.
//
// The names of the plates needed to print the document.
func (w *Writer) PlateNames(names ...string) *Writer {
	w.Element("PlateNames", NamespaceXmpPaged).OrderedArray(texts(names)...)
	return w
}

// XMPIDQ.

// IdqScheme is XmpWriter::idq_scheme in xmp-writer src/lib.rs.
//
// Write the xmpidq:GImg property.
//
// Identifies the scheme of the Writer.XMPIdentifier property.
func (w *Writer) IdqScheme(scheme string) *Writer {
	w.Element("Scheme", NamespaceXmpIdq).Value(Text(scheme))
	return w
}

// Adobe PDF.

// PDFKeywords is XmpWriter::pdf_keywords in xmp-writer src/lib.rs.
//
// Write the pdf:Keywords property.
//
// The document's keywords.
func (w *Writer) PDFKeywords(keywords string) *Writer {
	w.Element("Keywords", NamespaceAdobePdf).Value(Text(keywords))
	return w
}

// PDFVersion is XmpWriter::pdf_version in xmp-writer src/lib.rs.
//
// Write the pdf:PDFVersion property.
//
// The version of the PDF specification to which the document conforms
// (e.g. "1.0", "1.7").
func (w *Writer) PDFVersion(version string) *Writer {
	w.Element("PDFVersion", NamespaceAdobePdf).Value(Text(version))
	return w
}

// Producer is XmpWriter::producer in xmp-writer src/lib.rs.
//
// Write the pdf:Producer property.
//
// The name of the application that created the PDF document.
func (w *Writer) Producer(producer string) *Writer {
	w.Element("Producer", NamespaceAdobePdf).Value(Text(producer))
	return w
}

// Trapped is XmpWriter::trapped in xmp-writer src/lib.rs.
//
// Write the pdf:Trapped property.
//
// Whether the document has been trapped.
func (w *Writer) Trapped(trapped bool) *Writer {
	w.Element("Trapped", NamespaceAdobePdf).Value(Bool(trapped))
	return w
}

// PDF/A, PDF/UA and PDF/X.

// PDFAPart is XmpWriter::pdfa_part in xmp-writer src/lib.rs.
//
// Write the pdfaid:part property.
//
// The part of the PDF/A standard to which the document conforms (e.g. 1,
// 4).
func (w *Writer) PDFAPart(part int32) *Writer {
	w.Element("part", NamespacePdfAID).Value(Int32(part))
	return w
}

// PDFAAmd is XmpWriter::pdfa_amd in xmp-writer src/lib.rs.
//
// Write the pdfaid:amd property.
//
// The amendment specifier this file conforms to, if any.
func (w *Writer) PDFAAmd(amd string) *Writer {
	w.Element("amd", NamespacePdfAID).Value(Text(amd))
	return w
}

// PDFACorr is XmpWriter::pdfa_corr in xmp-writer src/lib.rs.
//
// Write the pdfaid:corr property.
//
// The corrigendum specifier this file conforms to, if any.
func (w *Writer) PDFACorr(corr string) *Writer {
	w.Element("corr", NamespacePdfAID).Value(Text(corr))
	return w
}

// ExtensionSchemas is XmpWriter::extension_schemas in xmp-writer
// src/lib.rs.
//
// Start writing the pdfaExtension:schemas property.
//
// Description of all extension schemas used in the document.
func (w *Writer) ExtensionSchemas() *PDFAExtSchemasWriter {
	return &PDFAExtSchemasWriter{w.Element("schemas", NamespacePdfAExtension).Array(RdfCollectionBag)}
}

// PDFAConformance is XmpWriter::pdfa_conformance in xmp-writer src/lib.rs.
//
// Write the pdfaid:conformance property.
//
// The conformance level of the PDF/A standard to which the document
// conforms (e.g. "A", "B").
func (w *Writer) PDFAConformance(conformance string) *Writer {
	w.Element("conformance", NamespacePdfAID).Value(Text(conformance))
	return w
}

// PDFARev is XmpWriter::pdfa_rev in xmp-writer src/lib.rs.
//
// Write the pdfaid:rev property.
//
// Four-digit year of the date of publication or revision.
func (w *Writer) PDFARev(rev int32) *Writer {
	w.Element("rev", NamespacePdfAID).Value(Int32(rev))
	return w
}

// PDFXVersion is XmpWriter::pdfx_version in xmp-writer src/lib.rs.
//
// Write the pdfxid:GTS_PDFXVersion property.
//
// The version of the PDF/X standard to which the document conforms (e.g.
// "PDF/X-3:2003").
func (w *Writer) PDFXVersion(version string) *Writer {
	w.Element("GTS_PDFXVersion", NamespacePdfXID).Value(Text(version))
	return w
}

// PDFUAPart is XmpWriter::pdfua_part in xmp-writer src/lib.rs.
//
// Write the pdfuaid:part property.
//
// The part of the PDF/UA standard to which the document conforms (e.g. 1).
func (w *Writer) PDFUAPart(part int32) *Writer {
	w.Element("part", NamespacePdfUAID).Value(Int32(part))
	return w
}

// PDFUAAmd is XmpWriter::pdfua_amd in xmp-writer src/lib.rs.
//
// Write the pdfuaid:amd property.
//
// The amendment specifier this file conforms to, if any. Specific to
// PDF/UA-1.
func (w *Writer) PDFUAAmd(amd string) *Writer {
	w.Element("amd", NamespacePdfUAID).Value(Text(amd))
	return w
}

// PDFUACorr is XmpWriter::pdfua_corr in xmp-writer src/lib.rs.
//
// Write the pdfuaid:corr property.
//
// The corrigendum specifier this file conforms to, if any. In practice,
// this should never appear since no corrigenda have been published for
// PDF/UA, and ISO does not allow for new corrigenda to be added to ISO
// 14289.
func (w *Writer) PDFUACorr(corr string) *Writer {
	w.Element("corr", NamespacePdfUAID).Value(Text(corr))
	return w
}

// PDFUARev is XmpWriter::pdfua_rev in xmp-writer src/lib.rs.
//
// Write the pdfuaid:rev property.
//
// Four-digit year of the date of publication or revision. Specific to
// PDF/UA-2.
func (w *Writer) PDFUARev(rev int32) *Writer {
	w.Element("rev", NamespacePdfUAID).Value(Int32(rev))
	return w
}

// ThumbnailWriter is ThumbnailWriter in xmp-writer src/lib.rs.
//
// A self-contained thumbnail image.
//
// Created by ThumbnailsWriter.AddThumbnail.
type ThumbnailWriter struct {
	*Struct
}

// Format is ThumbnailWriter::format in xmp-writer src/lib.rs.
//
// Write the xmpGImg:format property with a custom format of the thumbnail
// image. Must be "JPEG" for now.
func (t *ThumbnailWriter) Format(format string) *ThumbnailWriter {
	t.Element("format", NamespaceXmpImage).Value(Text(format))
	return t
}

// FormatJPEG is ThumbnailWriter::format_jpeg in xmp-writer src/lib.rs.
//
// Write the xmpGImg:format property with the value "JPEG".
func (t *ThumbnailWriter) FormatJPEG() *ThumbnailWriter {
	return t.Format("JPEG")
}

// Width is ThumbnailWriter::width in xmp-writer src/lib.rs. Upstream
// converts the width to an i64 with as, which the conversion of Go gives
// too.
//
// Write the xmpGImg:width property.
func (t *ThumbnailWriter) Width(width uint64) *ThumbnailWriter {
	t.Element("width", NamespaceXmpImage).Value(Int64(width))
	return t
}

// Height is ThumbnailWriter::height in xmp-writer src/lib.rs.
//
// Write the xmpGImg:height property.
func (t *ThumbnailWriter) Height(height uint64) *ThumbnailWriter {
	t.Element("height", NamespaceXmpImage).Value(Int64(height))
	return t
}

// Image is ThumbnailWriter::image in xmp-writer src/lib.rs.
//
// Write the xmpGImg:image property.
//
// The image must be a base64-encoded JPEG.
func (t *ThumbnailWriter) Image(image string) *ThumbnailWriter {
	t.Element("image", NamespaceXmpImage).Value(Text(image))
	return t
}

// ThumbnailsWriter is ThumbnailsWriter in xmp-writer src/lib.rs.
//
// Write a set of thumbnails.
//
// Created by Writer.Thumbnails.
type ThumbnailsWriter struct {
	*Array
}

// AddThumbnail is ThumbnailsWriter::add_thumbnail in xmp-writer
// src/lib.rs.
//
// Add a thumbnail.
func (t *ThumbnailsWriter) AddThumbnail() *ThumbnailWriter {
	return &ThumbnailWriter{t.Element().Obj()}
}

// ResourceRefWriter is ResourceRefWriter in xmp-writer src/lib.rs.
//
// Writer for a reference to a resource.
//
// Created by Writer.DerivedFrom, Writer.ManagedFrom, or
// ResourceRefsWriter.AddRef.
type ResourceRefWriter struct {
	*Struct
}

// AlternatePaths is ResourceRefWriter::alternate_paths in xmp-writer
// src/lib.rs.
//
// Write the stRef:alternatePaths property.
//
// Fallback paths to the resource.
func (r *ResourceRefWriter) AlternatePaths(paths ...string) *ResourceRefWriter {
	r.Element("alternatePaths", NamespaceXmpResourceRef).OrderedArray(texts(paths)...)
	return r
}

// DocumentID is ResourceRefWriter::document_id in xmp-writer src/lib.rs.
//
// Write the stRef:documentID property.
//
// The Writer.DocumentID of the referenced resource.
func (r *ResourceRefWriter) DocumentID(id string) *ResourceRefWriter {
	r.Element("documentID", NamespaceXmpResourceRef).Value(Text(id))
	return r
}

// FilePath is ResourceRefWriter::file_path in xmp-writer src/lib.rs.
//
// Write the stRef:filePath property.
//
// The path or URL to the resource.
func (r *ResourceRefWriter) FilePath(path string) *ResourceRefWriter {
	r.Element("filePath", NamespaceXmpResourceRef).URI(path)
	return r
}

// InstanceID is ResourceRefWriter::instance_id in xmp-writer src/lib.rs.
//
// Write the stRef:instanceID property.
//
// The Writer.InstanceID of the referenced resource.
func (r *ResourceRefWriter) InstanceID(id string) *ResourceRefWriter {
	r.Element("instanceID", NamespaceXmpResourceRef).Value(Text(id))
	return r
}

// LastModifyDate is ResourceRefWriter::last_modify_date in xmp-writer
// src/lib.rs.
//
// Write the stRef:lastModifyDate property.
//
// The last modification date of the resource. See ResourceEventWriter.When.
func (r *ResourceRefWriter) LastModifyDate(date DateTime) *ResourceRefWriter {
	r.Element("lastModifyDate", NamespaceXmpResourceRef).Value(date)
	return r
}

// Manager is ResourceRefWriter::manager in xmp-writer src/lib.rs.
//
// Write the stRef:manager property.
//
// The name of the application that manages the resource. See
// Writer.Manager.
func (r *ResourceRefWriter) Manager(manager string) *ResourceRefWriter {
	r.Element("manager", NamespaceXmpResourceRef).Value(Text(manager))
	return r
}

// ManagerVariant is ResourceRefWriter::manager_variant in xmp-writer
// src/lib.rs.
//
// Write the stRef:managerVariant property.
//
// The variant of the application that manages the resource. See
// Writer.ManagerVariant.
func (r *ResourceRefWriter) ManagerVariant(variant string) *ResourceRefWriter {
	r.Element("managerVariant", NamespaceXmpResourceRef).Value(Text(variant))
	return r
}

// ManageTo is ResourceRefWriter::manage_to in xmp-writer src/lib.rs.
//
// Write the stRef:manageTo property.
//
// The URI of the resource prior to being managed. See Writer.ManageTo.
func (r *ResourceRefWriter) ManageTo(uri string) *ResourceRefWriter {
	r.Element("manageTo", NamespaceXmpResourceRef).URI(uri)
	return r
}

// ManageUI is ResourceRefWriter::manage_ui in xmp-writer src/lib.rs.
//
// Write the stRef:manageUI property.
//
// An URI to the user interface of the application that manages the
// resource. See Writer.ManageUI.
func (r *ResourceRefWriter) ManageUI(uri string) *ResourceRefWriter {
	r.Element("manageUI", NamespaceXmpResourceRef).URI(uri)
	return r
}

// MaskMarkers is ResourceRefWriter::mask_markers in xmp-writer src/lib.rs.
//
// Write the stRef:maskMarkers property.
//
// Whether to process markers for resources in the Writer.Ingredients
// array.
func (r *ResourceRefWriter) MaskMarkers(markers MaskMarkers) *ResourceRefWriter {
	r.Element("maskMarkers", NamespaceXmpResourceRef).Value(markers)
	return r
}

// PartMapping is ResourceRefWriter::part_mapping in xmp-writer src/lib.rs.
//
// Write the stRef:partMapping property.
//
// The name or URI of a mapping function to map fromPart to toPart.
func (r *ResourceRefWriter) PartMapping(mapping string) *ResourceRefWriter {
	r.Element("partMapping", NamespaceXmpResourceRef).Value(Text(mapping))
	return r
}

// RenditionClass is ResourceRefWriter::rendition_class in xmp-writer
// src/lib.rs.
//
// Write the stRef:renditionClass property.
//
// The rendition class of the referenced resource. See
// Writer.RenditionClass.
func (r *ResourceRefWriter) RenditionClass(rendition RenditionClass) *ResourceRefWriter {
	r.Element("renditionClass", NamespaceXmpResourceRef).Value(rendition)
	return r
}

// RenditionParams is ResourceRefWriter::rendition_params in xmp-writer
// src/lib.rs.
//
// Write the stRef:renditionParams property.
//
// The rendition parameters of the referenced resource. See
// Writer.RenditionParams.
func (r *ResourceRefWriter) RenditionParams(params string) *ResourceRefWriter {
	r.Element("renditionParams", NamespaceXmpResourceRef).Value(Text(params))
	return r
}

// ToPart is ResourceRefWriter::to_part in xmp-writer src/lib.rs.
//
// Write the stRef:toPart property.
//
// For a resource in a Writer.Ingredients array, the part of the root
// resource that the ingredient corresponds to.
func (r *ResourceRefWriter) ToPart(part string) *ResourceRefWriter {
	r.Element("toPart", NamespaceXmpResourceRef).Value(Text(part))
	return r
}

// VersionID is ResourceRefWriter::version_id in xmp-writer src/lib.rs.
//
// Write the stRef:versionID property.
//
// The referenced resource's version ID. See Writer.VersionID.
func (r *ResourceRefWriter) VersionID(id string) *ResourceRefWriter {
	r.Element("versionID", NamespaceXmpResourceRef).Value(Text(id))
	return r
}

// ResourceRefsWriter is ResourceRefsWriter in xmp-writer src/lib.rs.
//
// Writer for a resource reference array.
//
// Created by Writer.Ingredients.
type ResourceRefsWriter struct {
	*Array
}

// AddRef is ResourceRefsWriter::add_ref in xmp-writer src/lib.rs.
//
// Add a reference to the array.
func (r *ResourceRefsWriter) AddRef() *ResourceRefWriter {
	return &ResourceRefWriter{r.Element().Obj()}
}

// ResourceEventWriter is ResourceEventWriter in xmp-writer src/lib.rs.
//
// Writer for an event that occurred to a resource.
//
// Created by VersionWriter.Event and ResourceEventsWriter.AddEvent.
type ResourceEventWriter struct {
	*Struct
}

// Action is ResourceEventWriter::action in xmp-writer src/lib.rs.
//
// Write the stEvt:action property.
//
// The action that occurred to the resource.
func (r *ResourceEventWriter) Action(action ResourceEventAction) *ResourceEventWriter {
	r.Element("action", NamespaceXmpResourceEvent).Value(action)
	return r
}

// Changed is ResourceEventWriter::changed in xmp-writer src/lib.rs.
//
// Write the stEvt:changed property.
//
// Semicolon-separated list of the parts of the resource that changed.
func (r *ResourceEventWriter) Changed(parts string) *ResourceEventWriter {
	r.Element("changed", NamespaceXmpResourceEvent).Value(Text(parts))
	return r
}

// InstanceID is ResourceEventWriter::instance_id in xmp-writer src/lib.rs.
//
// Write the stEvt:instanceID property.
//
// Value of the Writer.InstanceID property at the time of the action.
func (r *ResourceEventWriter) InstanceID(id string) *ResourceEventWriter {
	r.Element("instanceID", NamespaceXmpResourceEvent).Value(Text(id))
	return r
}

// Parameters is ResourceEventWriter::parameters in xmp-writer src/lib.rs.
//
// Write the stEvt:changed property.
//
// Additional parameters for the action.
func (r *ResourceEventWriter) Parameters(params string) *ResourceEventWriter {
	r.Element("parameters", NamespaceXmpResourceEvent).Value(Text(params))
	return r
}

// SoftwareAgent is ResourceEventWriter::software_agent in xmp-writer
// src/lib.rs.
//
// Write the stEvt:softwareAgent property.
//
// The name of the software agent that performed the action.
func (r *ResourceEventWriter) SoftwareAgent(agent string) *ResourceEventWriter {
	r.Element("softwareAgent", NamespaceXmpResourceEvent).Value(Text(agent))
	return r
}

// When is ResourceEventWriter::when in xmp-writer src/lib.rs.
//
// Write the stEvt:when property.
//
// The date and time the action occurred.
func (r *ResourceEventWriter) When(date DateTime) *ResourceEventWriter {
	r.Element("when", NamespaceXmpResourceEvent).Value(date)
	return r
}

// ResourceEventsWriter is ResourceEventsWriter in xmp-writer src/lib.rs.
//
// Writer for a resource event array.
//
// Created by Writer.History.
type ResourceEventsWriter struct {
	*Array
}

// AddEvent is ResourceEventsWriter::add_event in xmp-writer src/lib.rs.
//
// Add an event to the array.
func (r *ResourceEventsWriter) AddEvent() *ResourceEventWriter {
	return &ResourceEventWriter{r.Element().Obj()}
}

// PantryItemWriter is PantryItemWriter in xmp-writer src/lib.rs.
//
// Writer for an item in a Pantry array.
//
// Use the embedded Struct to add properties. Created by
// PantryWriter.AddItem.
type PantryItemWriter struct {
	*Struct
}

// InstanceID is PantryItemWriter::instance_id in xmp-writer src/lib.rs.
//
// Write the xmpMM:instanceID property. Required.
func (p *PantryItemWriter) InstanceID(id string) *PantryItemWriter {
	p.Element("instanceID", NamespaceXmpMedia).Value(Text(id))
	return p
}

// PantryWriter is PantryWriter in xmp-writer src/lib.rs.
//
// Writer for a Pantry array.
type PantryWriter struct {
	*Array
}

// AddItem is PantryWriter::add_item in xmp-writer src/lib.rs.
//
// Add an item to the array.
func (p *PantryWriter) AddItem() *PantryItemWriter {
	return &PantryItemWriter{p.Element().Obj()}
}

// VersionWriter is VersionWriter in xmp-writer src/lib.rs.
//
// Writer for a version struct.
//
// Created by VersionsWriter.AddVersion.
type VersionWriter struct {
	*Struct
}

// Comments is VersionWriter::comments in xmp-writer src/lib.rs.
//
// Write the stVer:comments property.
//
// Comments about the version.
func (v *VersionWriter) Comments(comments string) *VersionWriter {
	v.Element("comments", NamespaceXmpVersion).Value(Text(comments))
	return v
}

// Event is VersionWriter::event in xmp-writer src/lib.rs.
//
// Start writing the stVer:event property.
//
// The event that created the version.
func (v *VersionWriter) Event() *ResourceEventWriter {
	return &ResourceEventWriter{v.Element("event", NamespaceXmpVersion).Obj()}
}

// Modifier is VersionWriter::modifier in xmp-writer src/lib.rs.
//
// Write the stVer:modifier property.
//
// The person or organization that created the version.
func (v *VersionWriter) Modifier(modifier string) *VersionWriter {
	v.Element("modifier", NamespaceXmpVersion).Value(Text(modifier))
	return v
}

// ModifyDate is VersionWriter::modify_date in xmp-writer src/lib.rs.
//
// Write the stVer:modifyDate property.
//
// The date and time the version was created.
func (v *VersionWriter) ModifyDate(date DateTime) *VersionWriter {
	v.Element("modifyDate", NamespaceXmpVersion).Value(date)
	return v
}

// Version is VersionWriter::version in xmp-writer src/lib.rs.
//
// Write the stVer:version property.
//
// The new version number.
func (v *VersionWriter) Version(version string) *VersionWriter {
	v.Element("version", NamespaceXmpVersion).Value(Text(version))
	return v
}

// VersionsWriter is VersionsWriter in xmp-writer src/lib.rs.
//
// Writer for a versions array.
//
// Created by Writer.VersionRef.
type VersionsWriter struct {
	*Array
}

// AddVersion is VersionsWriter::add_version in xmp-writer src/lib.rs.
//
// Add a version to the array.
func (v *VersionsWriter) AddVersion() *VersionWriter {
	return &VersionWriter{v.Element().Obj()}
}

// JobWriter is JobWriter in xmp-writer src/lib.rs.
//
// Writer for a job struct.
//
// Created by JobsWriter.AddJob.
type JobWriter struct {
	*Struct
}

// ID is JobWriter::id in xmp-writer src/lib.rs.
//
// Write the stJob:id property.
//
// The unique identifier for the job.
func (j *JobWriter) ID(id string) *JobWriter {
	j.Element("id", NamespaceXmpJob).Value(Text(id))
	return j
}

// Name is JobWriter::name in xmp-writer src/lib.rs.
//
// Write the stJob:name property.
//
// The name of the job.
func (j *JobWriter) Name(name string) *JobWriter {
	j.Element("name", NamespaceXmpJob).Value(Text(name))
	return j
}

// URL is JobWriter::url in xmp-writer src/lib.rs.
//
// Write the stJob:url property.
//
// Reference an external job management file.
func (j *JobWriter) URL(url string) *JobWriter {
	j.Element("url", NamespaceXmpJob).URI(url)
	return j
}

// JobsWriter is JobsWriter in xmp-writer src/lib.rs.
//
// Writer for a job array.
//
// Created by Writer.Jobs.
type JobsWriter struct {
	*Array
}

// AddJob is JobsWriter::add_job in xmp-writer src/lib.rs.
//
// Add a job to the array.
func (j *JobsWriter) AddJob() *JobWriter {
	return &JobWriter{j.Element().Obj()}
}

// ColorantWriter is ColorantWriter in xmp-writer src/lib.rs.
//
// A writer for colorant structs.
//
// Created by ColorantsWriter.AddColorant.
type ColorantWriter struct {
	*Struct
}

// Type is ColorantWriter::type_ in xmp-writer src/lib.rs.
//
// Write the xmpG:type property.
//
// Whether this is a spot color or a process color.
func (c *ColorantWriter) Type(kind ColorantType) *ColorantWriter {
	c.Element("type", NamespaceXmpColorant).Value(kind)
	return c
}

// SwatchName is ColorantWriter::swatch_name in xmp-writer src/lib.rs.
//
// Write the xmpG:swatchName property.
//
// The name of the colorant.
func (c *ColorantWriter) SwatchName(name string) *ColorantWriter {
	c.Element("swatchName", NamespaceXmpColorant).Value(Text(name))
	return c
}

// ColorantMode is ColorantWriter::colorant_mode in xmp-writer src/lib.rs.
//
// Write the xmpG:colorantMode property.
//
// In which color space this colorant is defined.
func (c *ColorantWriter) ColorantMode(mode ColorantMode) *ColorantWriter {
	c.Element("colorantMode", NamespaceXmpColorant).Value(mode)
	return c
}

// L is ColorantWriter::l in xmp-writer src/lib.rs.
//
// Write the xmpG:colorantType property.
//
// The L value of a colorant with xmpG:colorantMode set to Lab.
func (c *ColorantWriter) L(l float64) *ColorantWriter {
	c.Element("L", NamespaceXmpColorant).Value(Float64(l))
	return c
}

// A is ColorantWriter::a in xmp-writer src/lib.rs.
//
// Write the xmpG:a property.
//
// The a value of a colorant with xmpG:colorantMode set to Lab.
func (c *ColorantWriter) A(a int32) *ColorantWriter {
	c.Element("A", NamespaceXmpColorant).Value(Int32(a))
	return c
}

// B is ColorantWriter::b in xmp-writer src/lib.rs.
//
// Write the xmpG:b property.
//
// The b value of a colorant with xmpG:colorantMode set to Lab.
func (c *ColorantWriter) B(b int32) *ColorantWriter {
	c.Element("B", NamespaceXmpColorant).Value(Int32(b))
	return c
}

// Black is ColorantWriter::black in xmp-writer src/lib.rs.
//
// Write the xmpG:black property.
//
// The K value of a colorant with xmpG:colorantMode set to CMYK.
func (c *ColorantWriter) Black(black float64) *ColorantWriter {
	c.Element("black", NamespaceXmpColorant).Value(Float64(black))
	return c
}

// Cyan is ColorantWriter::cyan in xmp-writer src/lib.rs.
//
// Write the xmpG:cyan property.
//
// The C value of a colorant with xmpG:colorantMode set to CMYK.
func (c *ColorantWriter) Cyan(cyan float64) *ColorantWriter {
	c.Element("cyan", NamespaceXmpColorant).Value(Float64(cyan))
	return c
}

// Magenta is ColorantWriter::magenta in xmp-writer src/lib.rs.
//
// Write the xmpG:magenta property.
//
// The M value of a colorant with xmpG:colorantMode set to CMYK.
func (c *ColorantWriter) Magenta(magenta float64) *ColorantWriter {
	c.Element("magenta", NamespaceXmpColorant).Value(Float64(magenta))
	return c
}

// Yellow is ColorantWriter::yellow in xmp-writer src/lib.rs.
//
// Write the xmpG:yellow property.
//
// The Y value of a colorant with xmpG:colorantMode set to CMYK.
func (c *ColorantWriter) Yellow(yellow float64) *ColorantWriter {
	c.Element("yellow", NamespaceXmpColorant).Value(Float64(yellow))
	return c
}

// Red is ColorantWriter::red in xmp-writer src/lib.rs.
//
// Write the xmpG:red property.
//
// The R value of a colorant with xmpG:colorantMode set to RGB.
func (c *ColorantWriter) Red(red int32) *ColorantWriter {
	c.Element("red", NamespaceXmpColorant).Value(Int32(red))
	return c
}

// Green is ColorantWriter::green in xmp-writer src/lib.rs.
//
// Write the xmpG:green property.
//
// The G value of a colorant with xmpG:colorantMode set to RGB.
func (c *ColorantWriter) Green(green int32) *ColorantWriter {
	c.Element("green", NamespaceXmpColorant).Value(Int32(green))
	return c
}

// Blue is ColorantWriter::blue in xmp-writer src/lib.rs.
//
// Write the xmpG:blue property.
//
// The B value of a colorant with xmpG:colorantMode set to RGB.
func (c *ColorantWriter) Blue(blue int32) *ColorantWriter {
	c.Element("blue", NamespaceXmpColorant).Value(Int32(blue))
	return c
}

// ColorantsWriter is ColorantsWriter in xmp-writer src/lib.rs.
//
// Writer for an array of colorants.
//
// Created by Writer.Colorants.
type ColorantsWriter struct {
	*Array
}

// AddColorant is ColorantsWriter::add_colorant in xmp-writer src/lib.rs.
//
// Add a new colorant to the array.
func (c *ColorantsWriter) AddColorant() *ColorantWriter {
	return &ColorantWriter{c.Element().Obj()}
}

// DimensionsWriter is DimensionsWriter in xmp-writer src/lib.rs.
//
// Writer for a dimensions struct.
//
// Created by Writer.MaxPageSize.
type DimensionsWriter struct {
	*Struct
}

// Width is DimensionsWriter::width in xmp-writer src/lib.rs.
//
// Write the stDim:w property.
//
// The width of the resource.
func (d *DimensionsWriter) Width(width float64) *DimensionsWriter {
	d.Element("w", NamespaceXmpDimensions).Value(Float64(width))
	return d
}

// Height is DimensionsWriter::height in xmp-writer src/lib.rs.
//
// Write the stDim:h property.
//
// The height of the resource.
func (d *DimensionsWriter) Height(height float64) *DimensionsWriter {
	d.Element("h", NamespaceXmpDimensions).Value(Float64(height))
	return d
}

// Unit is DimensionsWriter::unit in xmp-writer src/lib.rs.
//
// Write the stDim:unit property.
//
// The unit of the width and height properties.
func (d *DimensionsWriter) Unit(unit DimensionUnit) *DimensionsWriter {
	d.Element("unit", NamespaceXmpDimensions).Value(unit)
	return d
}

// FontWriter is FontWriter in xmp-writer src/lib.rs.
//
// Writer for a font struct.
//
// Created by Writer.Fonts.
type FontWriter struct {
	*Struct
}

// ChildFontFiles is FontWriter::child_font_files in xmp-writer src/lib.rs.
//
// Write the stFnt:childFontFiles property.
//
// An array of font files that make up this font.
func (f *FontWriter) ChildFontFiles(files ...string) *FontWriter {
	f.Element("childFontFiles", NamespaceXmpFont).OrderedArray(texts(files)...)
	return f
}

// Composite is FontWriter::composite in xmp-writer src/lib.rs.
//
// Write the stFnt:composite property.
//
// Whether the font is a composite font.
func (f *FontWriter) Composite(composite bool) *FontWriter {
	f.Element("composite", NamespaceXmpFont).Value(Bool(composite))
	return f
}

// FontFace is FontWriter::font_face in xmp-writer src/lib.rs.
//
// Write the stFnt:fontFace property.
//
// The font face name.
func (f *FontWriter) FontFace(face string) *FontWriter {
	f.Element("fontFace", NamespaceXmpFont).Value(Text(face))
	return f
}

// FontFamily is FontWriter::font_family in xmp-writer src/lib.rs.
//
// Write the stFnt:fontFamily property.
//
// The font family name.
func (f *FontWriter) FontFamily(family string) *FontWriter {
	f.Element("fontFamily", NamespaceXmpFont).Value(Text(family))
	return f
}

// FontFile is FontWriter::font_file in xmp-writer src/lib.rs.
//
// Write the stFnt:fontFile property.
//
// The font file name.
func (f *FontWriter) FontFile(fileName string) *FontWriter {
	f.Element("fontFileName", NamespaceXmpFont).Value(Text(fileName))
	return f
}

// FontName is FontWriter::font_name in xmp-writer src/lib.rs.
//
// Write the stFnt:fontName property.
//
// The PostScript name of the font.
func (f *FontWriter) FontName(name string) *FontWriter {
	f.Element("fontName", NamespaceXmpFont).Value(Text(name))
	return f
}

// FontType is FontWriter::font_type in xmp-writer src/lib.rs.
//
// Write the stFnt:fontType property.
//
// The font type.
func (f *FontWriter) FontType(fontType FontType) *FontWriter {
	f.Element("fontType", NamespaceXmpFont).Value(fontType)
	return f
}

// VersionString is FontWriter::version_string in xmp-writer src/lib.rs.
//
// Write the stFnt:versionString property.
//
// The version string of the font. Must be chosen depending on the font
// type:
//   - Type 1: /version
//   - TrueType / OpenType: nameId 5
//   - CID: /CIDFontVersion
//   - Bitmap Font: must be empty
func (f *FontWriter) VersionString(version string) *FontWriter {
	f.Element("versionString", NamespaceXmpFont).Value(Text(version))
	return f
}

// FontsWriter is FontsWriter in xmp-writer src/lib.rs.
//
// Writer for an array of fonts.
//
// Created by Writer.Fonts.
type FontsWriter struct {
	*Array
}

// AddFont is FontsWriter::add_font in xmp-writer src/lib.rs.
//
// Add a new font to the array.
func (f *FontsWriter) AddFont() *FontWriter {
	return &FontWriter{f.Element().Obj()}
}
