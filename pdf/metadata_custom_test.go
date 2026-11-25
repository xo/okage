// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/xmpwriter"
)

// The tests of this file port the tests of krilla tests/src/metadata.rs
// with custom XMP metadata, and the tests of the modules of
// src/interchange/metadata (D99). A file with a validator of PDF/A holds
// the compressed ICC profile of its output intent, so its bytes differ from
// the bytes of krilla. checkSnapshotXMP compares it as checkSnapshotDecoded
// does, with the instance ID, which hashes the compressed bytes, replaced.

// instanceIDPattern matches the instance ID of the XMP metadata.
var instanceIDPattern = regexp.MustCompile(`<xmpMM:InstanceID>([^<]*)</xmpMM:InstanceID>`)

// withoutInstanceID returns the decoded snapshot without each instance ID.
func withoutInstanceID(t *testing.T, data []byte) string {
	t.Helper()
	s := normalizeSnapshot(t, data)
	m := instanceIDPattern.FindStringSubmatch(s)
	if m == nil || m[1] == "" {
		return s
	}
	return strings.ReplaceAll(s, m[1], "INSTANCE-ID")
}

// checkSnapshotXMP compares the file with the snapshot of krilla after
// normalizeSnapshot, with the instance ID replaced.
func checkSnapshotXMP(t *testing.T, name string, d *Document) {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshots", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	got, wantS := withoutInstanceID(t, buf.Bytes()), withoutInstanceID(t, want)
	if got == wantS {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(wantS, "\n")
	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			n := 0
			for n < len(g) && n < len(w) && g[n] == w[n] {
				n++
			}
			lo := max(0, n-60)
			t.Fatalf("%s differs from the decoded snapshot at the line %d, byte %d:\ngot  %q\nwant %q",
				name, i+1, n, g[lo:min(len(g), n+100)], w[lo:min(len(w), n+100)])
		}
	}
}

// textDescription is text_description of krilla tests/src/metadata.rs.
func textDescription(name, description string) PropertyDescription {
	return NewPropertyDescription(name, SimpleTypeText, description)
}

// ccNamespace is cc_namespace of krilla tests/src/metadata.rs.
func ccNamespace() Namespace {
	return NewNamespace("cc", "http://creativecommons.org/ns#")
}

// facturXNamespace is factur_x_namespace of krilla tests/src/metadata.rs.
func facturXNamespace() Namespace {
	return NewNamespace("fx", "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#")
}

// facturXSchema is factur_x_schema of krilla tests/src/metadata.rs.
func facturXSchema() Schema {
	return NewSchema(facturXNamespace()).
		Name("Factur-X PDFA Extension Schema").
		Property(textDescription("DocumentType", "Type of the embedded XML")).
		Property(textDescription("DocumentFileName", "File name of the embedded XML")).
		Property(textDescription("Version", "Version of the Factur-X profile")).
		Property(textDescription("ConformanceLevel", "Conformance level of the invoice"))
}

// facturXMetadata is factur_x_metadata of krilla tests/src/metadata.rs.
func facturXMetadata(conformanceLevel string) Metadata {
	fx := facturXNamespace()
	return NewMetadata().
		CreationDate(datetime()).
		Language("en").
		CustomXMPSchema(facturXSchema()).
		CustomXMPProperty(fx.Property("DocumentType", TextValue("INVOICE"))).
		CustomXMPProperty(fx.Property("DocumentFileName", TextValue("factur-x.xml"))).
		CustomXMPProperty(fx.Property("Version", TextValue("1.0"))).
		CustomXMPProperty(fx.Property("ConformanceLevel", TextValue(conformanceLevel)))
}

// TestMetadataCustomXMPBasic is the snapshot test metadata_custom_xmp_basic
// of krilla tests/src/metadata.rs, with settings_5.
func TestMetadataCustomXMPBasic(t *testing.T) {
	d := NewDocument(settings5()...)
	metadataImpl(d)

	cc := ccNamespace()
	mm := NewNamespace("xmpMM", "http://ns.adobe.com/xap/1.0/mm/")

	metadata := NewMetadata().
		CreationDate(datetime()).
		CustomXMPProperty(cc.Property("license", URIValue("https://creativecommons.org/licenses/by/4.0/"))).
		CustomXMPProperty(cc.Property("attributionName", LanguageAlternativeValue(
			LangText{Text: "Krilla"},
			LangText{Lang: new("de"), Text: "Krilla (DE)"},
		))).
		CustomXMPProperty(cc.Property("tags", UnorderedArrayValue(TextValue("a"), TextValue("b")))).
		CustomXMPProperty(cc.Property("ordering", OrderedArrayValue(IntegerValue(1), IntegerValue(2)))).
		CustomXMPProperty(cc.Property("rating", AlternativeArrayValue(RealValue(4.5), RealValue(5.0)))).
		CustomXMPProperty(cc.Property("reviewed", BoolValue(true))).
		CustomXMPProperty(cc.Property("derivedFrom", StructValue(
			mm.Property("DocumentID", TextValue("uuid:source-doc")),
			mm.Property("VersionID", TextValue("1.0")),
		)))
	d.SetMetadata(metadata)
	checkSnapshot(t, "metadata_custom_xmp_basic", d)
}

// TestMetadataCustomXMPFacturX is the snapshot test
// metadata_custom_xmp_factur_x of krilla tests/src/metadata.rs, with
// settings_10.
func TestMetadataCustomXMPFacturX(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA3B)...)
	d.SetMetadata(facturXMetadata("BASIC"))
	checkSnapshotXMP(t, "metadata_custom_xmp_factur_x", d)
}

// TestMetadataCustomXMPStructType is the snapshot test
// metadata_custom_xmp_struct_type of krilla tests/src/metadata.rs, with
// settings_10.
func TestMetadataCustomXMPStructType(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA3B)...)
	ex := NewNamespace("ex", "http://example.com/xmp/address#")
	address := NewStructType(ex, "Address", "A postal address").
		Field(textDescription("street", "The street and house number")).
		Field(textDescription("city", "The city"))
	schema := NewSchema(ex).
		Name("Address book").
		Property(NewPropertyDescription("addresses", OrderedArrayType(address), "Known addresses")).
		Property(NewPropertyDescription("ratio", SimpleTypeRational, "Aspect ratio"))

	metadata := NewMetadata().
		CreationDate(datetime()).
		Language("en").
		CustomXMPSchema(schema).
		CustomXMPProperty(ex.Property("addresses", OrderedArrayValue(StructValue(
			ex.Property("street", TextValue("Main Street 1")),
			ex.Property("city", TextValue("Berlin")),
		)))).
		CustomXMPProperty(ex.Property("ratio", RationalValue(16, 9)))
	d.SetMetadata(metadata)
	checkSnapshotXMP(t, "metadata_custom_xmp_struct_type", d)
}

// TestValidatePDFA1CustomXMPBuiltinNamespace is the snapshot test
// validate_pdf_a1_custom_xmp_builtin_namespace of krilla
// tests/src/metadata.rs, with settings_19. xmpDM is a namespace xmp-writer
// serializes natively, but it is only a predefined PDF/A schema from XMP
// 2005 on.
func TestValidatePDFA1CustomXMPBuiltinNamespace(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA1B)...)
	dm := NewNamespace("xmpDM", "http://ns.adobe.com/xmp/1.0/DynamicMedia/")
	schema := NewSchema(dm).
		Name("XMP Dynamic Media").
		Property(textDescription("scene", "The name of the scene"))
	metadata := NewMetadata().
		CreationDate(datetime()).
		Language("en").
		CustomXMPSchema(schema).
		CustomXMPProperty(dm.Property("scene", TextValue("intro")))
	d.SetMetadata(metadata)
	checkSnapshotXMP(t, "validate_pdf_a1_custom_xmp_builtin_namespace", d)
}

// facturXMinimumXML is FACTUR_X_MINIMUM_XML of krilla
// tests/src/metadata.rs, a Factur-X invoice in the MINIMUM profile.
const facturXMinimumXML = `<?xml version="1.0" encoding="UTF-8"?>
<rsm:CrossIndustryInvoice
    xmlns:rsm="urn:un:unece:uncefact:data:standard:CrossIndustryInvoice:100"
    xmlns:ram="urn:un:unece:uncefact:data:standard:ReusableAggregateBusinessInformationEntity:100"
    xmlns:udt="urn:un:unece:uncefact:data:standard:UnqualifiedDataType:100">
  <rsm:ExchangedDocumentContext>
    <ram:BusinessProcessSpecifiedDocumentContextParameter>
      <ram:ID>A1</ram:ID>
    </ram:BusinessProcessSpecifiedDocumentContextParameter>
    <ram:GuidelineSpecifiedDocumentContextParameter>
      <ram:ID>urn:factur-x.eu:1p0:minimum</ram:ID>
    </ram:GuidelineSpecifiedDocumentContextParameter>
  </rsm:ExchangedDocumentContext>
  <rsm:ExchangedDocument>
    <ram:ID>2026-001</ram:ID>
    <ram:TypeCode>380</ram:TypeCode>
    <ram:IssueDateTime>
      <udt:DateTimeString format="102">20260615</udt:DateTimeString>
    </ram:IssueDateTime>
  </rsm:ExchangedDocument>
  <rsm:SupplyChainTradeTransaction>
    <ram:ApplicableHeaderTradeAgreement>
      <ram:BuyerReference>SERVEXEC</ram:BuyerReference>
      <ram:SellerTradeParty>
        <ram:Name>Krilla Seller GmbH</ram:Name>
        <ram:PostalTradeAddress>
          <ram:CountryID>DE</ram:CountryID>
        </ram:PostalTradeAddress>
        <ram:SpecifiedTaxRegistration>
          <ram:ID schemeID="VA">DE123456789</ram:ID>
        </ram:SpecifiedTaxRegistration>
      </ram:SellerTradeParty>
      <ram:BuyerTradeParty>
        <ram:Name>Buyer SARL</ram:Name>
      </ram:BuyerTradeParty>
    </ram:ApplicableHeaderTradeAgreement>
    <ram:ApplicableHeaderTradeDelivery/>
    <ram:ApplicableHeaderTradeSettlement>
      <ram:InvoiceCurrencyCode>EUR</ram:InvoiceCurrencyCode>
      <ram:SpecifiedTradeSettlementHeaderMonetarySummation>
        <ram:TaxBasisTotalAmount>100.00</ram:TaxBasisTotalAmount>
        <ram:TaxTotalAmount currencyID="EUR">19.00</ram:TaxTotalAmount>
        <ram:GrandTotalAmount>119.00</ram:GrandTotalAmount>
        <ram:DuePayableAmount>119.00</ram:DuePayableAmount>
      </ram:SpecifiedTradeSettlementHeaderMonetarySummation>
    </ram:ApplicableHeaderTradeSettlement>
  </rsm:SupplyChainTradeTransaction>
</rsm:CrossIndustryInvoice>
`

// TestMetadataCustomXMPFacturXEmbedded is the snapshot test
// metadata_custom_xmp_factur_x_embedded of krilla tests/src/metadata.rs,
// with settings_10. A Factur-X (ZUGFeRD) invoice: a PDF/A-3 carrying the
// Factur-X XMP extension schema and the structured XML embedded.
func TestMetadataCustomXMPFacturXEmbedded(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA3B)...)
	d.SetMetadata(facturXMetadata("MINIMUM"))

	mimeType, _ := NewMIMEType("text/xml")
	description := "Factur-X invoice data"
	date := datetime()
	compress := false
	d.EmbedFile(EmbeddedFile{
		Path:             "factur-x.xml",
		MIMEType:         mimeType,
		Description:      &description,
		AssociationKind:  AssociationKindAlternative,
		Data:             []byte(facturXMinimumXML),
		ModificationDate: &date,
		Compress:         &compress,
	})
	checkSnapshotXMP(t, "metadata_custom_xmp_factur_x_embedded", d)
}

// pdfAMetadata is pdf_a_metadata of krilla tests/src/metadata.rs.
func pdfAMetadata(property Property) Metadata {
	return NewMetadata().
		CreationDate(datetime()).
		Language("en").
		CustomXMPProperty(property)
}

// finishResult finishes a document with a validator of PDF/A and the
// metadata, and returns the error of Finish.
func finishResult(a Archival, metadata Metadata) error {
	var opts []Option
	if a != 0 {
		opts = append(opts, WithArchivalValidator(a))
	}
	d := NewDocument(opts...)
	d.SetMetadata(metadata)
	return d.Finish(&bytes.Buffer{})
}

// validationErrorsOf is validation_errors of krilla tests/src/main.rs. It
// returns the validation errors of the error of Finish.
func validationErrorsOf(t *testing.T, err error) []validationError {
	t.Helper()
	var ve *validationFailedError
	if !errors.As(err, &ve) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	out := make([]validationError, len(ve.entries))
	for i, e := range ve.entries {
		out[i] = e.err
	}
	return out
}

// assertMissingDescription is assert_missing_description of krilla
// tests/src/metadata.rs.
func assertMissingDescription(t *testing.T, err error, uri, name string) {
	t.Helper()
	errs := validationErrorsOf(t, err)
	if !slices.ContainsFunc(errs, func(e validationError) bool {
		return e.kind == validationMissingXMPPropertyDescription && e.propertyName == name && e.namespaceURI == uri
	}) {
		t.Errorf("expected MissingXmpPropertyDescription for %s %s, got %+v", uri, name, errs)
	}
}

// TestMetadataCustomXMPPDFAMissingDescription is the test
// metadata_custom_xmp_pdf_a_missing_description of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPPDFAMissingDescription(t *testing.T) {
	undeclared := NewNamespace("fx", "urn:factur-x:test#")
	err := finishResult(ArchivalA3B, pdfAMetadata(undeclared.Property("DocumentType", TextValue("INVOICE"))))
	assertMissingDescription(t, err, "urn:factur-x:test#", "DocumentType")
}

// TestMetadataCustomXMPNoSchemaOutsidePDFA is the test
// metadata_custom_xmp_no_schema_outside_pdf_a of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPNoSchemaOutsidePDFA(t *testing.T) {
	tdm := NewNamespace("tdm", "http://www.w3.org/ns/tdmrep/")
	metadata := NewMetadata().
		CustomXMPProperty(tdm.Property("reservation", IntegerValue(1))).
		CustomXMPProperty(tdm.Property("policy", URIValue("https://example.com/policy.json")))
	if err := finishResult(0, metadata); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPPredefinedNamespacePDFA is the test
// metadata_custom_xmp_predefined_namespace_pdf_a of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPPredefinedNamespacePDFA(t *testing.T) {
	// Properties in predefined schemas don't need an extension schema, so
	// no property descriptions are required even in PDF/A.
	xmpRights := NewNamespace("xmpRights", "http://ns.adobe.com/xap/1.0/rights/")
	if err := finishResult(ArchivalA3B, pdfAMetadata(xmpRights.Property("Marked", BoolValue(true)))); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPExifPredefinedPDFA is the test
// metadata_custom_xmp_exif_predefined_pdf_a of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPExifPredefinedPDFA(t *testing.T) {
	// EXIF is a predefined PDF/A schema even though xmp-writer has no
	// native variant for it, so it needs no extension schema and no
	// descriptions.
	exif := NewNamespace("exif", "http://ns.adobe.com/exif/1.0/")
	if err := finishResult(ArchivalA3B, pdfAMetadata(exif.Property("ColorSpace", IntegerValue(1)))); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPCameraRawVersionSpecific is the test
// metadata_custom_xmp_camera_raw_version_specific of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPCameraRawVersionSpecific(t *testing.T) {
	// Camera Raw is predefined only from XMP 2005 on. It is exempt from an
	// extension schema in PDF/A-2/-3 but not in PDF/A-1.
	crs := NewNamespace("crs", "http://ns.adobe.com/camera-raw-settings/1.0/")
	metadata := pdfAMetadata(crs.Property("WhiteBalance", TextValue("Auto")))
	if err := finishResult(ArchivalA3B, metadata); err != nil {
		t.Error(err)
	}
	assertMissingDescription(t, finishResult(ArchivalA1B, metadata), crs.URI(), "WhiteBalance")
}

// assertXMPError is assert_xmp_error of krilla tests/src/metadata.rs.
func assertXMPError(t *testing.T, metadata Metadata, kind XMPErrorKind, name string) {
	t.Helper()
	err := finishResult(0, metadata)
	var xe *XMPError
	if !errors.As(err, &xe) || !errors.Is(err, okage.ErrPDFXMP) {
		t.Fatalf("got %v, want an XMPError", err)
	}
	if xe.Kind != kind || xe.Name != name {
		t.Errorf("got %+v, want %d %q", *xe, kind, name)
	}
}

// TestMetadataCustomXMPConflictingNamespace is the test
// metadata_custom_xmp_conflicting_namespace of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPConflictingNamespace(t *testing.T) {
	assertXMPError(t, NewMetadata().
		CustomXMPProperty(ccNamespace().Property("license", TextValue("CC-BY"))).
		CustomXMPProperty(NewNamespace("cc2", "http://creativecommons.org/ns#").
			Property("attributionName", TextValue("Krilla"))),
		XMPErrorConflictingNamespace, "http://creativecommons.org/ns#")
}

// TestMetadataCustomXMPConflictingPrefix is the test
// metadata_custom_xmp_conflicting_prefix of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPConflictingPrefix(t *testing.T) {
	assertXMPError(t, NewMetadata().
		CustomXMPProperty(facturXNamespace().Property("DocumentType", TextValue("INVOICE"))).
		CustomXMPProperty(NewNamespace("fx", "urn:factur-x:other#").Property("Version", TextValue("1.0"))),
		XMPErrorConflictingPrefix, "fx")
}

// TestMetadataCustomXMPReservedPrefix is the test
// metadata_custom_xmp_reserved_prefix of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPReservedPrefix(t *testing.T) {
	assertXMPError(t, NewMetadata().CustomXMPProperty(
		NewNamespace("dc", "http://example.com/custom#").Property("something", TextValue("value"))),
		XMPErrorReservedPrefix, "dc")
}

// TestMetadataCustomXMPDuplicateSchema is the test
// metadata_custom_xmp_duplicate_schema of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPDuplicateSchema(t *testing.T) {
	assertXMPError(t, NewMetadata().
		CustomXMPSchema(facturXSchema()).
		CustomXMPSchema(NewSchema(facturXNamespace())),
		XMPErrorDuplicateSchema, facturXNamespace().URI())
}

// TestMetadataCustomXMPInvalidNumber is the test
// metadata_custom_xmp_invalid_number of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPInvalidNumber(t *testing.T) {
	nan := 0.0
	nan /= nan
	assertXMPError(t, NewMetadata().CustomXMPProperty(ccNamespace().Property("rating",
		OrderedArrayValue(RealValue(4.5), RealValue(nan)))),
		XMPErrorInvalidNumber, "rating")
	assertXMPError(t, NewMetadata().CustomXMPProperty(ccNamespace().Property("ratio", RationalValue(1, 0))),
		XMPErrorInvalidNumber, "ratio")
}

// tdmSchema is tdm_schema of krilla tests/src/metadata.rs.
func tdmSchema() (Namespace, Schema) {
	tdm := NewNamespace("tdm", "http://www.w3.org/ns/tdmrep/")
	schema := NewSchema(tdm).
		Property(NewPropertyDescription(
			"reservation",
			ClosedChoiceType(SimpleTypeInteger, "0", "1"),
			"Whether TDM rights are reserved",
		).Required()).
		Property(NewPropertyDescription("policy", SimpleTypeURI, "A machine-readable TDM policy"))
	return tdm, schema
}

// TestMetadataCustomXMPValidAgainstSchema is the test
// metadata_custom_xmp_valid_against_schema of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPValidAgainstSchema(t *testing.T) {
	tdm, schema := tdmSchema()
	metadata := NewMetadata().
		CustomXMPSchema(schema).
		CustomXMPProperty(tdm.Property("reservation", IntegerValue(1))).
		CustomXMPProperty(tdm.Property("policy", URIValue("https://example.com/policy.json")))
	if err := finishResult(0, metadata); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPInvalidLanguageTag is the test
// metadata_custom_xmp_invalid_language_tag of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPInvalidLanguageTag(t *testing.T) {
	ex := NewNamespace("ex", "http://example.com/xmp#")
	title := func(l string) Metadata {
		return NewMetadata().CustomXMPProperty(ex.Property("title", LanguageAlternativeValue(
			LangText{Text: "Title"},
			LangText{Lang: new(l), Text: "Title"},
		)))
	}

	for _, l := range []string{"", "en_US", `en" bad="1`, "en<"} {
		assertXMPError(t, title(l), XMPErrorInvalidValue, "title")
	}

	if err := finishResult(0, title("de-AT")); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPLocaleAndMIMEType is the test
// metadata_custom_xmp_locale_and_mime_type of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPLocaleAndMIMEType(t *testing.T) {
	ex := NewNamespace("ex", "http://example.com/xmp#")
	schema := NewSchema(ex).
		Property(NewPropertyDescription("locale", SimpleTypeLocale, "Locale")).
		Property(NewPropertyDescription("format", SimpleTypeMIMEType, "Format"))
	metadata := func(name, value string) Metadata {
		return NewMetadata().
			CustomXMPSchema(schema).
			CustomXMPProperty(ex.Property(name, TextValue(value)))
	}

	for _, c := range [][2]string{
		{"locale", "en_US"},
		{"locale", "en-"},
		{"format", "font/woff"},
		{"format", "text"},
		{"format", "text/x..y"},
	} {
		assertXMPError(t, metadata(c[0], c[1]), XMPErrorInvalidValue, c[0])
	}

	for _, c := range [][2]string{
		{"locale", "en"},
		{"locale", "de-AT-1996"},
		{"format", "text/xml"},
		{"format", "application/vnd.ms-excel"},
		{"format", "image/svg+xml"},
	} {
		if err := finishResult(0, metadata(c[0], c[1])); err != nil {
			t.Errorf("%s: %s: %v", c[0], c[1], err)
		}
	}
}

// TestMetadataCustomXMPInvalidValue is the test
// metadata_custom_xmp_invalid_value of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPInvalidValue(t *testing.T) {
	tdm, schema := tdmSchema()
	assertXMPError(t, NewMetadata().
		CustomXMPSchema(schema).
		CustomXMPProperty(tdm.Property("reservation", IntegerValue(2))),
		XMPErrorInvalidValue, "reservation")
	assertXMPError(t, NewMetadata().
		CustomXMPSchema(schema).
		CustomXMPProperty(tdm.Property("reservation", IntegerValue(1))).
		CustomXMPProperty(tdm.Property("policy", BoolValue(true))),
		XMPErrorInvalidValue, "policy")
	assertXMPError(t, NewMetadata().
		CustomXMPSchema(schema).
		CustomXMPProperty(tdm.Property("reservation", IntegerValue(1))).
		CustomXMPProperty(tdm.Property("policy", TextValue("https://example.com/policy.json"))),
		XMPErrorInvalidValue, "policy")
}

// TestMetadataCustomXMPMissingRequiredProperty is the test
// metadata_custom_xmp_missing_required_property of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPMissingRequiredProperty(t *testing.T) {
	tdm, schema := tdmSchema()
	assertXMPError(t, NewMetadata().
		CustomXMPSchema(schema).
		CustomXMPProperty(tdm.Property("policy", URIValue("https://example.com/policy.json"))),
		XMPErrorMissingProperty, "reservation")
}

// TestMetadataCustomXMPInvalidStruct is the test
// metadata_custom_xmp_invalid_struct of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPInvalidStruct(t *testing.T) {
	ex := NewNamespace("ex", "http://example.com/xmp/address#")
	address := NewStructType(ex, "Address", "A postal address").
		Field(textDescription("street", "The street").Required()).
		Field(textDescription("city", "The city"))
	metadata := func(fields ...Property) Metadata {
		return NewMetadata().
			CustomXMPSchema(NewSchema(ex).Property(NewPropertyDescription("address", address, "An address"))).
			CustomXMPProperty(ex.Property("address", StructValue(fields...)))
	}

	assertXMPError(t, metadata(
		ex.Property("street", TextValue("Main Street 1")),
		ex.Property("country", TextValue("DE")),
	), XMPErrorInvalidValue, "address")
	assertXMPError(t, metadata(ex.Property("city", TextValue("Berlin"))), XMPErrorMissingProperty, "street")
}

// TestMetadataCustomXMPDuplicateProperty is the test
// metadata_custom_xmp_duplicate_property of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPDuplicateProperty(t *testing.T) {
	assertXMPError(t, NewMetadata().
		CustomXMPProperty(ccNamespace().Property("license", TextValue("CC-BY"))).
		CustomXMPProperty(ccNamespace().Property("license", TextValue("CC0"))),
		XMPErrorDuplicateProperty, "cc:license")

	ex := NewNamespace("ex", "http://example.com/xmp/address#")
	assertXMPError(t, NewMetadata().CustomXMPProperty(ex.Property("address", StructValue(
		ex.Property("street", TextValue("Main Street 1")),
		ex.Property("street", TextValue("Main Street 2")),
	))), XMPErrorDuplicateProperty, "ex:street")
}

// TestMetadataCustomXMPInvalidName is the test
// metadata_custom_xmp_invalid_name of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPInvalidName(t *testing.T) {
	assertXMPError(t, NewMetadata().CustomXMPProperty(ccNamespace().Property("has space", TextValue("value"))),
		XMPErrorInvalidName, "has space")
	assertXMPError(t, NewMetadata().CustomXMPProperty(
		NewNamespace("1ex", "http://example.com/xmp#").Property("name", TextValue("value"))),
		XMPErrorInvalidName, "1ex")
	assertXMPError(t, NewMetadata().CustomXMPProperty(
		NewNamespace("xml", "http://example.com/xmp#").Property("name", TextValue("value"))),
		XMPErrorReservedPrefix, "xml")

	ex := NewNamespace("ex", "http://example.com/xmp/address#")
	withStruct := func(ty StructType) Metadata {
		return NewMetadata().CustomXMPSchema(
			NewSchema(ex).Property(NewPropertyDescription("address", ty, "An address")))
	}
	assertXMPError(t, withStruct(NewStructType(ex, "Postal Address", "")),
		XMPErrorInvalidName, "Postal Address")
	assertXMPError(t, withStruct(NewStructType(ex, "Address", "").Field(textDescription("zip:code", ""))),
		XMPErrorInvalidName, "zip:code")
	assertXMPError(t, NewMetadata().CustomXMPProperty(ex.Property("address", StructValue(
		ex.Property("-street", TextValue("Main Street 1")),
	))), XMPErrorInvalidName, "-street")
}

// TestMetadataCustomXMPEscapedNamespaceURI is the test
// metadata_custom_xmp_escaped_namespace_uri of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPEscapedNamespaceURI(t *testing.T) {
	d := NewDocument()
	d.SetMetadata(NewMetadata().CustomXMPProperty(
		NewNamespace("ex", `http://example.com/xmp?a=1&b="2"#`).Property("name", TextValue("value"))))
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	expected := []byte(`xmlns:ex="http://example.com/xmp?a=1&amp;b=&quot;2&quot;#"`)
	if !bytes.Contains(buf.Bytes(), expected) {
		t.Errorf("the file has no %s", expected)
	}
}

// TestMetadataCustomXMPInvalidPredefinedStruct is the test
// metadata_custom_xmp_invalid_predefined_struct of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPInvalidPredefinedStruct(t *testing.T) {
	ex := NewNamespace("ex", "http://example.com/xmp/print#")
	type field struct {
		name  string
		value Value
	}
	metadata := func(dim Namespace, fields []field) Metadata {
		props := make([]Property, len(fields))
		for i, f := range fields {
			props[i] = dim.Property(f.name, f.value)
		}
		return NewMetadata().
			CustomXMPSchema(NewSchema(ex).Property(
				NewPropertyDescription("pageSize", PredefinedStructDimensions, "The page size"))).
			CustomXMPProperty(ex.Property("pageSize", StructValue(props...)))
	}

	stDim := NewNamespace("stDim", "http://ns.adobe.com/xap/1.0/sType/Dimensions#")
	assertXMPError(t, metadata(stDim, []field{{"depth", RealValue(4.0)}}), XMPErrorInvalidValue, "pageSize")
	assertXMPError(t, metadata(stDim, []field{{"w", TextValue("wide")}}), XMPErrorInvalidValue, "pageSize")

	// Fields of a predefined type are matched by URI, not prefix.
	dim := NewNamespace("dim", "http://ns.adobe.com/xap/1.0/sType/Dimensions#")
	err := finishResult(0, metadata(dim, []field{
		{"w", RealValue(210.0)},
		{"h", RealValue(297.0)},
		{"unit", TextValue("mm")},
	}))
	if err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPManagedProperty is the test
// metadata_custom_xmp_managed_property of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPManagedProperty(t *testing.T) {
	dc := NewNamespace("dc", "http://purl.org/dc/elements/1.1/")
	assertXMPError(t, NewMetadata().CustomXMPProperty(dc.Property("title", TextValue("Title"))),
		XMPErrorManagedProperty, "dc:title")

	pdfNS := NewNamespace("pdf", "http://ns.adobe.com/pdf/1.3/")
	assertXMPError(t, NewMetadata().CustomXMPProperty(pdfNS.Property("Trapped", TextValue("True"))),
		XMPErrorManagedProperty, "pdf:Trapped")

	pdfaExtension := NewNamespace("pdfaExtension", "http://www.aiim.org/pdfa/ns/extension/")
	assertXMPError(t, NewMetadata().CustomXMPProperty(pdfaExtension.Property("schemas", TextValue("Schemas"))),
		XMPErrorManagedProperty, "pdfaExtension:schemas")
	pdfaField := NewNamespace("pdfaField", "http://www.aiim.org/pdfa/ns/field#")
	assertXMPError(t, NewMetadata().CustomXMPProperty(pdfaField.Property("anything", TextValue("Value"))),
		XMPErrorManagedProperty, "pdfaField:anything")

	if err := finishResult(0, NewMetadata().CustomXMPProperty(dc.Property("rights", TextValue("CC-BY")))); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPPDFA1PredefinedProperties is the test
// metadata_custom_xmp_pdf_a1_predefined_properties of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPPDFA1PredefinedProperties(t *testing.T) {
	dc := NewNamespace("dc", "http://purl.org/dc/elements/1.1/")
	exif := NewNamespace("exif", "http://ns.adobe.com/exif/1.0/")

	for _, m := range []Metadata{
		pdfAMetadata(dc.Property("rights", TextValue("CC-BY"))),
		pdfAMetadata(exif.Property("ColorSpace", IntegerValue(1))),
		pdfAMetadata(dc.Property("rights", TextValue("CC-BY"))).Trapped(false),
	} {
		if err := finishResult(ArchivalA1B, m); err != nil {
			t.Error(err)
		}
	}

	assertMissingDescription(t, finishResult(ArchivalA1B, pdfAMetadata(dc.Property("extra", TextValue("value")))),
		dc.URI(), "extra")
	assertMissingDescription(t, finishResult(ArchivalA1B, pdfAMetadata(exif.Property("Extra", TextValue("value")))),
		exif.URI(), "Extra")
}

// TestMetadataCustomXMPPDFA1FalseFriend is the test
// metadata_custom_xmp_pdf_a1_false_friend of krilla tests/src/metadata.rs.
func TestMetadataCustomXMPPDFA1FalseFriend(t *testing.T) {
	xmp := NewNamespace("xmp", "http://ns.adobe.com/xap/1.0/")
	assertMissingDescription(t, finishResult(ArchivalA1B, pdfAMetadata(xmp.Property("Label", TextValue("Draft")))),
		xmp.URI(), "Label")

	described := pdfAMetadata(xmp.Property("Label", TextValue("Draft"))).
		CustomXMPSchema(NewSchema(xmp).Property(textDescription("Label", "A label")))
	if err := finishResult(ArchivalA1B, described); err != nil {
		t.Error(err)
	}

	// XMP 2005 added xmp:Label, so it is predefined in PDF/A-3.
	if err := finishResult(ArchivalA3B, pdfAMetadata(xmp.Property("Label", TextValue("Draft")))); err != nil {
		t.Error(err)
	}
}

// TestValidatePDFA1CustomXMPPredefinedSchemaExtension is the snapshot test
// validate_pdf_a1_custom_xmp_predefined_schema_extension of krilla
// tests/src/metadata.rs, with settings_19. A property added to a
// predefined schema needs an extension schema that only describes the new
// property.
func TestValidatePDFA1CustomXMPPredefinedSchemaExtension(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA1B)...)
	dc := NewNamespace("dc", "http://purl.org/dc/elements/1.1/")
	schema := NewSchema(dc).
		Property(NewPropertyDescription("rights", LanguageAlternativeType(), "Informal rights statement")).
		Property(textDescription("extra", "An additional property"))
	metadata := pdfAMetadata(dc.Property("extra", TextValue("value"))).
		CustomXMPSchema(schema).
		CustomXMPProperty(dc.Property("rights", LanguageAlternativeValue(LangText{Text: "CC-BY"})))
	d.SetMetadata(metadata)
	checkSnapshotXMP(t, "validate_pdf_a1_custom_xmp_predefined_schema_extension", d)
}

// TestValidatePDFA4FCustomXMPRelaxNG is the snapshot test
// validate_pdf_a4f_custom_xmp_relax_ng of krilla tests/src/metadata.rs,
// with settings_27.
func TestValidatePDFA4FCustomXMPRelaxNG(t *testing.T) {
	d := NewDocument(settingsArchival(ArchivalA4F)...)
	ex := NewNamespace("ex", "http://example.com/xmp/address#")
	address := NewStructType(ex, "Address", "A postal address").
		Field(textDescription("street", "The street and house number").Required()).
		Field(textDescription("city", "The city"))
	schema := NewSchema(ex).
		Name("Address book").
		Description("Addresses & contacts").
		Property(NewPropertyDescription("addresses", OrderedArrayType(address), "Known addresses")).
		Property(NewPropertyDescription(
			"kind",
			ClosedChoiceType(SimpleTypeText, "private", "business"),
			"The kind of address book",
		).Required()).
		Property(NewPropertyDescription("ratio", SimpleTypeRational, "Aspect ratio"))

	dc := NewNamespace("dc", "http://purl.org/dc/elements/1.1/")
	tdm := NewNamespace("tdm", "http://www.w3.org/ns/tdmrep/")
	metadata := NewMetadata().
		CreationDate(datetime()).
		Language("en").
		Title("Address book").
		CustomXMPSchema(schema).
		CustomXMPProperty(ex.Property("kind", TextValue("private"))).
		CustomXMPProperty(ex.Property("addresses", OrderedArrayValue(StructValue(
			ex.Property("street", TextValue("Main Street 1")),
			ex.Property("city", TextValue("Berlin")),
		)))).
		CustomXMPProperty(ex.Property("ratio", RationalValue(16, 9))).
		CustomXMPProperty(dc.Property("rights", LanguageAlternativeValue(LangText{Text: "CC-BY"}))).
		CustomXMPProperty(tdm.Property("reservation", IntegerValue(1)))
	d.SetMetadata(metadata)
	checkSnapshotXMP(t, "validate_pdf_a4f_custom_xmp_relax_ng", d)
}

// TestMetadataCustomXMPRelaxNGOnlyWithEmbeddedFiles is the test
// metadata_custom_xmp_relax_ng_only_with_embedded_files of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPRelaxNGOnlyWithEmbeddedFiles(t *testing.T) {
	tdm := NewNamespace("tdm", "http://www.w3.org/ns/tdmrep/")
	containsSchema := func(a Archival) bool {
		d := NewDocument(WithArchivalValidator(a))
		d.SetMetadata(pdfAMetadata(tdm.Property("reservation", IntegerValue(1))))
		var buf bytes.Buffer
		if err := d.Finish(&buf); err != nil {
			t.Fatal(err)
		}
		return bytes.Contains(buf.Bytes(), []byte("xmp-metadata.rng"))
	}

	if !containsSchema(ArchivalA4F) {
		t.Error("PDF/A-4f has no RELAX NG schema")
	}
	// Plain PDF/A-4 doesn't allow embedded files.
	if containsSchema(ArchivalA4) {
		t.Error("PDF/A-4 has a RELAX NG schema")
	}
}

// TestMetadataCustomXMPPDFAUnsupportedValueType is the test
// metadata_custom_xmp_pdf_a_unsupported_value_type of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPPDFAUnsupportedValueType(t *testing.T) {
	ex := NewNamespace("ex", "http://example.com/xmp/matrix#")
	row := NewStructType(ex, "Row", "A row").Field(NewPropertyDescription(
		"titles",
		OrderedArrayType(LanguageAlternativeType()),
		"Titles",
	))
	metadata := pdfAMetadata(ex.Property("matrix", OrderedArrayValue(OrderedArrayValue(IntegerValue(1))))).
		CustomXMPSchema(NewSchema(ex).
			Property(NewPropertyDescription(
				"matrix",
				OrderedArrayType(OrderedArrayType(SimpleTypeInteger)),
				"A matrix",
			)).
			Property(NewPropertyDescription("row", row, "A row")))

	errs := validationErrorsOf(t, finishResult(ArchivalA3B, metadata))
	for _, name := range []string{"matrix", "titles"} {
		if !slices.ContainsFunc(errs, func(e validationError) bool {
			return e.kind == validationUnsupportedXMPValueType && e.namespaceURI == ex.URI() && e.propertyName == name
		}) {
			t.Errorf("expected UnsupportedXmpValueType for %s, got %+v", name, errs)
		}
	}

	// Outside of PDF/A-1 to PDF/A-3, no extension schemas are written.
	if err := finishResult(0, metadata); err != nil {
		t.Error(err)
	}
}

// TestMetadataCustomXMPPDFA3PredefinedProperties is the test
// metadata_custom_xmp_pdf_a3_predefined_properties of krilla
// tests/src/metadata.rs.
func TestMetadataCustomXMPPDFA3PredefinedProperties(t *testing.T) {
	dc := NewNamespace("dc", "http://purl.org/dc/elements/1.1/")
	exif := NewNamespace("exif", "http://ns.adobe.com/exif/1.0/")
	dm := NewNamespace("xmpDM", "http://ns.adobe.com/xmp/1.0/DynamicMedia/")

	if err := finishResult(ArchivalA3B, pdfAMetadata(dc.Property("rights", TextValue("CC-BY")))); err != nil {
		t.Error(err)
	}
	if err := finishResult(ArchivalA3B, pdfAMetadata(dm.Property("album", TextValue("Greatest Hits")))); err != nil {
		t.Error(err)
	}
	// Removed from the EXIF schema in XMP 2005.
	if err := finishResult(ArchivalA1B, pdfAMetadata(exif.Property("MakerNote", TextValue("Note")))); err != nil {
		t.Error(err)
	}
	assertMissingDescription(t, finishResult(ArchivalA3B, pdfAMetadata(exif.Property("MakerNote", TextValue("Note")))),
		exif.URI(), "MakerNote")

	assertMissingDescription(t, finishResult(ArchivalA3B, pdfAMetadata(dc.Property("extra", TextValue("value")))),
		dc.URI(), "extra")
}

// TestRelaxNGPrefixesDontCollideWithCustomPrefixes is the test
// relax_ng_prefixes_dont_collide_with_custom_prefixes of krilla
// src/interchange/metadata/mod.rs.
func TestRelaxNGPrefixesDontCollideWithCustomPrefixes(t *testing.T) {
	m := NewMetadata().
		CustomXMPProperty(NewNamespace("ui", "http://example.com/ui#").Property("label", TextValue("Label"))).
		CustomXMPProperty(NewNamespace("rng", "http://example.com/rng#").Property("grammar", TextValue("Grammar")))
	rng := m.relaxNGSchema()

	for _, want := range []string{
		`<grammar xmlns="http://relaxng.org/ns/structure/1.0"`,
		`xmlns:ui="http://example.com/ui#"`,
		`xmlns:rng="http://example.com/rng#"`,
		`xmlns:ui1="http://ns.iso.org/iso-16684-2/xmp-schema-ui-info/1.0"`,
		"<ui1:label",
	} {
		if !strings.Contains(rng, want) {
			t.Errorf("the schema has no %s", want)
		}
	}
	if strings.Contains(rng, "xmpmeta") {
		t.Error("the schema has xmpmeta")
	}
}

// TestRelaxNGDescribesTrapped is the test relax_ng_describes_trapped of
// krilla src/interchange/metadata/mod.rs.
func TestRelaxNGDescribesTrapped(t *testing.T) {
	m := NewMetadata().
		Trapped(true).
		CustomXMPProperty(NewNamespace("ex", "http://example.com/ex#").Property("kind", TextValue("private")))
	if rng := m.relaxNGSchema(); !strings.Contains(rng, `<element name="pdf:Trapped">`) {
		t.Error("the schema does not describe pdf:Trapped")
	}
}

// TestRelaxNGExpectsURIsAsResources is the test
// relax_ng_expects_uris_as_resources of krilla
// src/interchange/metadata/mod.rs.
func TestRelaxNGExpectsURIsAsResources(t *testing.T) {
	ns := NewNamespace("ex", "http://example.com/ex#")
	m := NewMetadata().
		CustomXMPSchema(NewSchema(ns).Property(NewPropertyDescription("policy", SimpleTypeURI, "Policy"))).
		CustomXMPProperty(ns.Property("policy", URIValue("https://example.com/")))
	want := `<define name="ISO16684-1.Types.Base.URI" combine="choice"><attribute name="rdf:resource"><text/></attribute></define>`
	if rng := m.relaxNGSchema(); !strings.Contains(rng, want) {
		t.Errorf("the schema has no %s", want)
	}
}

// TestCustomDatesKeepTheirPrecision is the test
// custom_dates_keep_their_precision of krilla
// src/interchange/metadata/custom.rs.
func TestCustomDatesKeepTheirPrecision(t *testing.T) {
	date := NewDateTime(2024)
	day := date.Month(5).Day(3)
	for _, c := range []struct {
		d    DateTime
		want string
	}{
		{date, "2024"},
		{date.Day(3), "2024"},
		{date.Month(5), "2024-05"},
		{date.Month(5).Hour(10), "2024-05"},
		{day, "2024-05-03"},
		{day.UTCOffsetHour(2), "2024-05-03"},
		{day.Hour(10), "2024-05-03T10:00"},
		{day.Hour(10).Minute(30).Second(15), "2024-05-03T10:30:15"},
		{day.Hour(10).Minute(30).UTCOffsetHour(2), "2024-05-03T10:30+02:00"},
		{day.Hour(10).UTCOffsetHour(-3).UTCOffsetMinute(30), "2024-05-03T10:00-03:30"},
		{day.Hour(10).UTCOffsetHour(0), "2024-05-03T10:00+00:00"},
	} {
		if got := customXMPDate(c.d); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

// TestExtensionSchemasOfSameNamespaceAreMerged is the test
// extension_schemas_of_same_namespace_are_merged of krilla
// src/interchange/metadata/extension_schema.rs.
func TestExtensionSchemasOfSameNamespaceAreMerged(t *testing.T) {
	ns := NewNamespace("pdfuaid", "http://www.aiim.org/pdfua/ns/id/")
	builtin := NewSchema(ns).Property(NewPropertyDescription("part", SimpleTypeInteger, "Part"))
	user := NewSchema(ns).
		Property(NewPropertyDescription("part", SimpleTypeInteger, "Part")).
		Property(NewPropertyDescription("custom", SimpleTypeText, "Custom"))

	xmp := xmpwriter.New()
	w := xmp.ExtensionSchemas()
	writeExtensionSchemas(w, []Schema{builtin}, []Schema{user})
	w.Finish()
	out := xmp.Finish("")

	for want, n := range map[string]int{
		"<pdfaSchema:namespaceURI>":  1,
		"<pdfaProperty:name>part<":   1,
		"<pdfaProperty:name>custom<": 1,
	} {
		if got := strings.Count(out, want); got != n {
			t.Errorf("%s appears %d times, want %d", want, got, n)
		}
	}
}

// TestPredefinedPropertyListsAreSorted is the test
// predefined_property_lists_are_sorted of krilla
// src/interchange/metadata/predefined.rs.
func TestPredefinedPropertyListsAreSorted(t *testing.T) {
	for _, list := range [][]predefinedProperties{
		pdfA1PredefinedProperties,
		pdfA2A3AdditionalProperties,
		pdfA1OnlyProperties,
	} {
		if !slices.IsSortedFunc(list, func(a, b predefinedProperties) int { return strings.Compare(a.uri, b.uri) }) {
			t.Error("a list is not sorted by its URIs")
		}
		for _, p := range list {
			if !slices.IsSorted(p.names) {
				t.Errorf("the names of %s are not sorted", p.uri)
			}
		}
	}
}
