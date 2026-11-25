package krilla

import (
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/pdf"
)

// The tests of this file compare the XMP metadata, the custom XMP metadata,
// the embedded files and the validators of PDF/A of the package pdf with
// krilla (D99), through the question "krilla" of the oracle. doc.go
// documents the fields of a scene for them: the setting "archival", the
// fields "custom_xmp_properties" and "custom_xmp_schemas" of the metadata,
// and the field "embedded_files" of a scene.

// propertySpec is a custom XMP property. Value is a value of xmpValue.
type propertySpec struct {
	NS    [2]string      `json:"ns"`
	Name  string         `json:"name"`
	Value map[string]any `json:"value"`
}

// descSpec is the description of a property or of a field. Type is a value
// type of xmpType.
type descSpec struct {
	Name        string `json:"name"`
	Type        any    `json:"type"`
	Description string `json:"description,omitempty"`
	Internal    bool   `json:"internal,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// structSpec is a user-defined structure type.
type structSpec struct {
	NS          [2]string  `json:"ns"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Fields      []descSpec `json:"fields,omitempty"`
}

// schemaSpec is a schema of a custom namespace.
type schemaSpec struct {
	NS          [2]string  `json:"ns"`
	Name        *string    `json:"name,omitempty"`
	Description *string    `json:"description,omitempty"`
	Properties  []descSpec `json:"properties,omitempty"`
}

// embedSpec is an embedded file. Data holds the bytes in base64.
type embedSpec struct {
	Path        string    `json:"path"`
	Mime        *string   `json:"mime,omitempty"`
	Description *string   `json:"description,omitempty"`
	Kind        string    `json:"kind"`
	Data        string    `json:"data"`
	Date        *dateSpec `json:"date,omitempty"`
	Compress    *bool     `json:"compress,omitempty"`
}

// The values of a custom XMP property, as the oracle reads them.
func vText(s string) map[string]any              { return map[string]any{"text": s} }
func vURI(s string) map[string]any               { return map[string]any{"uri": s} }
func vBool(b bool) map[string]any                { return map[string]any{"bool": b} }
func vInt(i int64) map[string]any                { return map[string]any{"int": i} }
func vReal(r float64) map[string]any             { return map[string]any{"real": r} }
func vRational(n int64, d uint64) map[string]any { return map[string]any{"rational": []any{n, d}} }
func vDate(d dateSpec) map[string]any            { return map[string]any{"date": d} }
func vSeq(items ...map[string]any) map[string]any {
	return map[string]any{"seq": items}
}
func vBag(items ...map[string]any) map[string]any { return map[string]any{"bag": items} }
func vAlt(items ...map[string]any) map[string]any { return map[string]any{"alt": items} }

// vLang is a language alternative. Each item is a language, or nil for
// x-default, and a text.
func vLang(items ...[2]*string) map[string]any { return map[string]any{"lang": items} }
func vStruct(fields ...propertySpec) map[string]any {
	return map[string]any{"struct": fields}
}

// The value types of a description, as the oracle reads them.
func tSimple(name string) any { return map[string]any{"simple": name} }
func tClosed(name string, values ...string) any {
	return map[string]any{"closed": []any{name, values}}
}
func tOpen(name string, values ...string) any { return map[string]any{"open": []any{name, values}} }
func tSeq(item any) any                       { return map[string]any{"seq": item} }
func tBag(item any) any                       { return map[string]any{"bag": item} }
func tAlt(item any) any                       { return map[string]any{"alt": item} }
func tPredefined(name string) any             { return map[string]any{"predefined": name} }
func tStruct(s structSpec) any                { return map[string]any{"struct": s} }

const tLang = "lang"

func prop(ns [2]string, name string, value map[string]any) propertySpec {
	return propertySpec{NS: ns, Name: name, Value: value}
}

func desc(name string, ty any, description string) descSpec {
	return descSpec{Name: name, Type: ty, Description: description}
}

// simpleTypes maps the names of the simple types to pdf.SimpleType.
var simpleTypes = map[string]pdf.SimpleType{
	"Text": pdf.SimpleTypeText, "Boolean": pdf.SimpleTypeBoolean, "Integer": pdf.SimpleTypeInteger,
	"Real": pdf.SimpleTypeReal, "Date": pdf.SimpleTypeDate, "AgentName": pdf.SimpleTypeAgentName,
	"GUID": pdf.SimpleTypeGUID, "Locale": pdf.SimpleTypeLocale, "MIMEType": pdf.SimpleTypeMIMEType,
	"ProperName": pdf.SimpleTypeProperName, "RenditionClass": pdf.SimpleTypeRenditionClass,
	"URI": pdf.SimpleTypeURI, "URL": pdf.SimpleTypeURL, "Rational": pdf.SimpleTypeRational,
	"FrameRate": pdf.SimpleTypeFrameRate, "FrameCount": pdf.SimpleTypeFrameCount,
	"Part": pdf.SimpleTypePart, "XPath": pdf.SimpleTypeXPath,
}

// predefinedStructs maps the names of the predefined structure types to
// pdf.PredefinedStruct.
var predefinedStructs = map[string]pdf.PredefinedStruct{
	"Dimensions": pdf.PredefinedStructDimensions, "Thumbnail": pdf.PredefinedStructThumbnail,
	"ResourceEvent": pdf.PredefinedStructResourceEvent, "ResourceRef": pdf.PredefinedStructResourceRef,
	"Version": pdf.PredefinedStructVersion, "Job": pdf.PredefinedStructJob,
	"Colorant": pdf.PredefinedStructColorant, "Font": pdf.PredefinedStructFont,
}

func goNS(ns [2]string) pdf.Namespace { return pdf.NewNamespace(ns[0], ns[1]) }

func goProperty(p propertySpec) (pdf.Property, error) {
	v, err := goXMPValue(p.Value)
	if err != nil {
		return pdf.Property{}, err
	}
	return pdf.NewProperty(goNS(p.NS), p.Name, v), nil
}

// goXMPValue makes a value of the helpers vText to vStruct. The helpers set
// the type of each value, so the assertions do not fail.
//
//nolint:forcetypeassert // the helpers of this file make the values
func goXMPValue(v map[string]any) (pdf.Value, error) {
	list := func(items []map[string]any) ([]pdf.Value, error) {
		out := make([]pdf.Value, len(items))
		for i, item := range items {
			var err error
			if out[i], err = goXMPValue(item); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	for k, x := range v {
		switch k {
		case "text":
			return pdf.TextValue(x.(string)), nil
		case "uri":
			return pdf.URIValue(x.(string)), nil
		case "bool":
			return pdf.BoolValue(x.(bool)), nil
		case "int":
			return pdf.IntegerValue(x.(int64)), nil
		case "real":
			return pdf.RealValue(x.(float64)), nil
		case "rational":
			r := x.([]any)
			return pdf.RationalValue(r[0].(int64), r[1].(uint64)), nil
		case "date":
			return pdf.DateValue(goDate(x.(dateSpec))), nil
		case "seq", "bag", "alt":
			items, err := list(x.([]map[string]any))
			if err != nil {
				return pdf.Value{}, err
			}
			switch k {
			case "seq":
				return pdf.OrderedArrayValue(items...), nil
			case "bag":
				return pdf.UnorderedArrayValue(items...), nil
			}
			return pdf.AlternativeArrayValue(items...), nil
		case "lang":
			var items []pdf.LangText
			for _, l := range x.([][2]*string) {
				items = append(items, pdf.LangText{Lang: l[0], Text: *l[1]})
			}
			return pdf.LanguageAlternativeValue(items...), nil
		case "struct":
			var fields []pdf.Property
			for _, f := range x.([]propertySpec) {
				p, err := goProperty(f)
				if err != nil {
					return pdf.Value{}, err
				}
				fields = append(fields, p)
			}
			return pdf.StructValue(fields...), nil
		}
	}
	return pdf.Value{}, fmt.Errorf("the value %v is not known", v)
}

// goXMPType makes a value type of the helpers tSimple to tStruct.
//
//nolint:forcetypeassert // the helpers of this file make the types
func goXMPType(t any) (pdf.IntoValueType, error) {
	if t == tLang {
		return pdf.LanguageAlternativeType(), nil
	}
	m, ok := t.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the type %v is not known", t)
	}
	for k, x := range m {
		switch k {
		case "simple":
			return simpleTypes[x.(string)], nil
		case "open", "closed":
			a := x.([]any)
			if k == "open" {
				return pdf.OpenChoiceType(simpleTypes[a[0].(string)], a[1].([]string)...), nil
			}
			return pdf.ClosedChoiceType(simpleTypes[a[0].(string)], a[1].([]string)...), nil
		case "seq", "bag", "alt":
			item, err := goXMPType(x)
			if err != nil {
				return nil, err
			}
			switch k {
			case "seq":
				return pdf.OrderedArrayType(item), nil
			case "bag":
				return pdf.UnorderedArrayType(item), nil
			}
			return pdf.AlternativeArrayType(item), nil
		case "predefined":
			return predefinedStructs[x.(string)], nil
		case "struct":
			s := x.(structSpec)
			ty := pdf.NewStructType(goNS(s.NS), s.Name, s.Description)
			for _, f := range s.Fields {
				d, err := goDescription(f)
				if err != nil {
					return nil, err
				}
				ty = ty.Field(d)
			}
			return ty, nil
		}
	}
	return nil, fmt.Errorf("the type %v is not known", t)
}

func goDescription(d descSpec) (pdf.PropertyDescription, error) {
	ty, err := goXMPType(d.Type)
	if err != nil {
		return pdf.PropertyDescription{}, err
	}
	desc := pdf.NewPropertyDescription(d.Name, ty, d.Description)
	if d.Internal {
		desc = desc.Internal()
	}
	if d.Required {
		desc = desc.Required()
	}
	return desc, nil
}

// goCustomXMP adds the custom XMP metadata of a scene, as metadata of the
// oracle does.
func goCustomXMP(m pdf.Metadata, s metadataSpec) (pdf.Metadata, error) {
	for _, p := range s.CustomXMPProperties {
		prop, err := goProperty(p)
		if err != nil {
			return m, err
		}
		m = m.CustomXMPProperty(prop)
	}
	for _, sc := range s.CustomXMPSchemas {
		schema := pdf.NewSchema(goNS(sc.NS))
		if sc.Name != nil {
			schema = schema.Name(*sc.Name)
		}
		if sc.Description != nil {
			schema = schema.Description(*sc.Description)
		}
		for _, d := range sc.Properties {
			desc, err := goDescription(d)
			if err != nil {
				return m, err
			}
			schema = schema.Property(desc)
		}
		m = m.CustomXMPSchema(schema)
	}
	return m, nil
}

// goEmbeddedFiles embeds the files of a scene, as finish_document of the
// oracle does.
func goEmbeddedFiles(d *pdf.Document, files []embedSpec) error {
	kinds := map[string]pdf.AssociationKind{
		"Source": pdf.AssociationKindSource, "Data": pdf.AssociationKindData,
		"Alternative": pdf.AssociationKindAlternative, "Supplement": pdf.AssociationKindSupplement,
		"Schema": pdf.AssociationKindSchema, "Unspecified": pdf.AssociationKindUnspecified,
	}
	for _, e := range files {
		var f pdf.EmbeddedFile
		f.Path = e.Path
		if e.Mime != nil {
			m, ok := pdf.NewMIMEType(*e.Mime)
			if !ok {
				return fmt.Errorf("the mime type %q is not valid", *e.Mime)
			}
			f.MIMEType = m
		}
		f.Description = e.Description
		kind, ok := kinds[e.Kind]
		if !ok {
			return fmt.Errorf("the association kind %q is not known", e.Kind)
		}
		f.AssociationKind = kind
		data, err := base64.StdEncoding.DecodeString(e.Data)
		if err != nil {
			return err
		}
		f.Data = data
		if e.Date != nil {
			date := goDate(*e.Date)
			f.ModificationDate = &date
		}
		f.Compress = e.Compress
		d.EmbedFile(f)
	}
	return nil
}

// xmpCases returns the scenes of the XMP metadata, the embedded files and
// the validators of PDF/A.
func xmpCases() []documentCase {
	var cases []documentCase
	add := func(name string, settings map[string]any, spec interchangeSpec) {
		cases = append(cases, documentCase{name, scene{Settings: settings, Pages: []pageSpec{page(200, 200)}, interchangeSpec: spec}})
	}
	pretty := func(extra map[string]any) map[string]any {
		s := map[string]any{"pretty": true, "ascii_compatible": true, "compress_content_streams": false}
		maps.Copy(s, extra)
		return s
	}

	// The XMP metadata of the metadata of metadataSpec.
	for _, v := range []string{"1.4", "1.7", "2.0"} {
		add("xmp full "+v, pretty(map[string]any{"version": v}), interchangeSpec{Metadata: fullMetadata()})
	}
	add("xmp default settings", map[string]any{}, interchangeSpec{Metadata: fullMetadata()})
	add("xmp without metadata", pretty(nil), interchangeSpec{})
	add("xmp version id", pretty(nil), interchangeSpec{Metadata: &metadataSpec{VersionID: new("1.2.3"), DocumentID: new("d")}})
	add("xmp escapes", pretty(nil), interchangeSpec{Metadata: &metadataSpec{
		Title: new(`<a & "b">`), Authors: []string{"x'y"}, Keywords: []string{"<", ">"}, Creator: new("&"),
		CreationDate: &dateSpec{Year: 1999, UTCOffsetHour: new(int8(-3)), UTCOffsetMinute: new(uint8(30))},
	}})

	// Each validator of PDF/A, with the metadata that it needs. The ICC
	// profile of the output intent is compressed, so the bytes differ.
	needed := func() *metadataSpec {
		return &metadataSpec{Language: new("en"), Title: new("t"), CreationDate: fullDate(), Creator: new("c")}
	}
	for _, a := range []string{"A1_B", "A2_B", "A2_U", "A3_B", "A3_U", "A4", "A4F", "A4E"} {
		add("archival "+a, map[string]any{"archival": a, "pretty": true}, interchangeSpec{Metadata: needed()})
	}
	add("archival A2_B on PDF 1.4", map[string]any{"archival": "A2_B", "version": "1.4"}, interchangeSpec{Metadata: needed()})
	custom := needed()
	custom.CustomFields = [][2]string{{"Custom", "value"}}
	for _, a := range []string{"A1_B", "A2_B", "A3_U"} {
		add("archival custom fields "+a, map[string]any{"archival": a}, interchangeSpec{Metadata: custom})
	}

	// The custom XMP metadata.
	cc := [2]string{"cc", "http://creativecommons.org/ns#"}
	mm := [2]string{"xmpMM", "http://ns.adobe.com/xap/1.0/mm/"}
	basic := fullMetadata()
	basic.CustomXMPProperties = []propertySpec{
		prop(cc, "license", vURI("https://creativecommons.org/licenses/by/4.0/")),
		prop(cc, "attributionName", vLang([2]*string{nil, new("Krilla")}, [2]*string{new("de"), new("Krilla (DE)")})),
		prop(cc, "tags", vBag(vText("a"), vText("b"))),
		prop(cc, "ordering", vSeq(vInt(1), vInt(-2))),
		prop(cc, "rating", vAlt(vReal(4.5), vReal(5), vReal(1e21), vReal(-0.001))),
		prop(cc, "reviewed", vBool(true)),
		prop(cc, "ratio", vRational(-16, 9)),
		prop(cc, "when", vDate(dateSpec{Year: 2024, Month: new(uint8(5)), Day: new(uint8(3)), Hour: new(uint8(10)), UTCOffsetHour: new(int8(-3))})),
		prop(cc, "derivedFrom", vStruct(
			prop(mm, "DocumentID", vText("uuid:source-doc")),
			prop(mm, "VersionID", vText("1.0")),
		)),
	}
	add("custom xmp basic", pretty(nil), interchangeSpec{Metadata: basic})
	add("custom xmp basic compressed", map[string]any{}, interchangeSpec{Metadata: basic})

	fx := [2]string{"fx", "urn:factur-x:pdfa:CrossIndustryDocument:invoice:1p0#"}
	facturX := needed()
	facturX.CustomXMPSchemas = []schemaSpec{{NS: fx, Name: new("Factur-X PDFA Extension Schema"), Properties: []descSpec{
		desc("DocumentType", tSimple("Text"), "Type of the embedded XML"),
		desc("DocumentFileName", tSimple("Text"), "File name of the embedded XML"),
		desc("Version", tSimple("Text"), "Version of the Factur-X profile"),
		desc("ConformanceLevel", tSimple("Text"), "Conformance level of the invoice"),
	}}}
	facturX.CustomXMPProperties = []propertySpec{
		prop(fx, "DocumentType", vText("INVOICE")),
		prop(fx, "DocumentFileName", vText("factur-x.xml")),
		prop(fx, "Version", vText("1.0")),
		prop(fx, "ConformanceLevel", vText("MINIMUM")),
	}
	invoice := embedSpec{
		Path: "factur-x.xml", Mime: new("text/xml"), Description: new("Factur-X invoice data"),
		Kind: "Alternative", Data: base64.StdEncoding.EncodeToString([]byte("<invoice/>")), Date: fullDate(),
		Compress: new(false),
	}
	for _, a := range []string{"A1_B", "A2_B", "A3_B", "A4F"} {
		add("custom xmp factur-x "+a, map[string]any{"archival": a}, interchangeSpec{Metadata: facturX})
	}
	add("custom xmp factur-x embedded", map[string]any{"archival": "A3_B", "pretty": true},
		interchangeSpec{Metadata: facturX, EmbeddedFiles: []embedSpec{invoice}})
	add("custom xmp factur-x without validator", pretty(nil), interchangeSpec{Metadata: facturX, EmbeddedFiles: []embedSpec{invoice}})

	ex := [2]string{"ex", "http://example.com/xmp/address#"}
	address := structSpec{NS: ex, Name: "Address", Description: "A postal address", Fields: []descSpec{
		{Name: "street", Type: tSimple("Text"), Description: "The street and house number", Required: true},
		desc("city", tSimple("Text"), "The city"),
		desc("size", tPredefined("Dimensions"), "The size"),
	}}
	book := needed()
	book.CustomXMPSchemas = []schemaSpec{{NS: ex, Name: new("Address book"), Description: new("Addresses & contacts"), Properties: []descSpec{
		desc("addresses", tSeq(tStruct(address)), "Known addresses"),
		{Name: "kind", Type: tClosed("Text", "private", "business"), Description: "The kind of address book", Required: true, Internal: true},
		desc("ratio", tSimple("Rational"), "Aspect ratio"),
		desc("tags", tBag(tOpen("Text", "a")), "Tags"),
		desc("alts", tAlt(tSimple("URI")), "Links"),
		desc("titles", tLang, "Titles"),
		desc("event", tPredefined("ResourceEvent"), "An event"),
		desc("font", tPredefined("Font"), "A font"),
		desc("home", tClosed("URI", "http://a", "http://b"), "Home"),
	}}}
	book.CustomXMPProperties = []propertySpec{
		prop(ex, "kind", vText("private")),
		prop(ex, "addresses", vSeq(vStruct(
			prop(ex, "street", vText("Main Street 1")),
			prop(ex, "city", vText("Berlin")),
			prop(ex, "size", vStruct(prop([2]string{"stDim", "http://ns.adobe.com/xap/1.0/sType/Dimensions#"}, "w", vReal(1)))),
		))),
		prop(ex, "ratio", vRational(16, 9)),
		prop(ex, "titles", vLang([2]*string{nil, new("x")})),
		prop([2]string{"dc", "http://purl.org/dc/elements/1.1/"}, "rights", vLang([2]*string{nil, new("CC-BY")})),
		prop([2]string{"tdm", "http://www.w3.org/ns/tdmrep/"}, "reservation", vInt(1)),
	}
	// PDF/A-2 and PDF/A-3 need a description of tdm:reservation, so their
	// book leaves it out.
	described := *book
	described.CustomXMPProperties = book.CustomXMPProperties[:len(book.CustomXMPProperties)-1]
	for _, a := range []string{"A2_B", "A3_U"} {
		add("custom xmp address book "+a, map[string]any{"archival": a}, interchangeSpec{Metadata: &described})
	}
	for _, a := range []string{"A4", "A4F", "A4E"} {
		add("custom xmp address book "+a, map[string]any{"archival": a}, interchangeSpec{Metadata: book})
	}
	add("custom xmp address book without validator", pretty(nil), interchangeSpec{Metadata: book})

	// The embedded files.
	data := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("embedded text ", 50)))
	file := func(path string, compress *bool) embedSpec {
		return embedSpec{Path: path, Mime: new("text/plain"), Description: new("d " + path), Kind: "Supplement",
			Data: data, Date: &dateSpec{Year: 2001}, Compress: compress}
	}
	files := []embedSpec{file("b.txt", new(false)), file("a.txt", new(true)), file("c.txt", nil),
		{Path: "bare", Kind: "Schema", Data: ""}, file("b.txt", new(false))}
	for _, v := range []string{"1.4", "1.7", "2.0"} {
		add("embedded files "+v, map[string]any{"version": v, "pretty": true}, interchangeSpec{EmbeddedFiles: files})
	}
	add("embedded files A3_B", map[string]any{"archival": "A3_B"}, interchangeSpec{Metadata: needed(), EmbeddedFiles: files[:3]})
	add("embedded files A4F", map[string]any{"archival": "A4F"}, interchangeSpec{Metadata: needed(), EmbeddedFiles: files[:3]})
	add("embedded files empty A4F", map[string]any{"archival": "A4F"}, interchangeSpec{Metadata: needed()})
	return cases
}

// TestXMPGo compares the files of pdf with the files of krilla for the
// scenes of xmpCases, as TestInterchangeGo does.
func TestXMPGo(t *testing.T) {
	compareScenes(t, xmpCases())
}

// TestXMPGoErrors compares the errors of the custom XMP metadata and of the
// validators: krilla returns KrillaError::Xmp or KrillaError::Validation,
// and pdf returns an error that wraps ErrPDFXMP or ErrPDFValidation.
func TestXMPGoErrors(t *testing.T) {
	s := func(settings map[string]any, m *metadataSpec, files ...embedSpec) scene {
		return scene{Settings: settings, Pages: []pageSpec{}, interchangeSpec: interchangeSpec{Metadata: m, EmbeddedFiles: files}} //nolint:modernize // the field names the spec
	}
	cc := [2]string{"cc", "http://creativecommons.org/ns#"}
	fx := [2]string{"fx", "urn:factur-x:test#"}
	cases := []struct {
		name   string
		scene  scene
		oracle string
		want   error
		kind   pdf.XMPErrorKind
	}{
		{"conflicting namespace", s(nil, &metadataSpec{CustomXMPProperties: []propertySpec{
			prop(cc, "a", vText("x")), prop([2]string{"cc2", cc[1]}, "b", vText("y")),
		}}), `Xmp(ConflictingNamespace("http://creativecommons.org/ns#"))`, okage.ErrPDFXMP, pdf.XMPErrorConflictingNamespace},
		{"conflicting prefix", s(nil, &metadataSpec{CustomXMPProperties: []propertySpec{
			prop(cc, "a", vText("x")), prop([2]string{"cc", "urn:other#"}, "b", vText("y")),
		}}), `Xmp(ConflictingPrefix("cc"))`, okage.ErrPDFXMP, pdf.XMPErrorConflictingPrefix},
		{"managed property", s(nil, &metadataSpec{CustomXMPProperties: []propertySpec{
			prop([2]string{"x", "http://purl.org/dc/elements/1.1/"}, "title", vText("x")),
		}}), `Xmp(ManagedProperty("dc:title"))`, okage.ErrPDFXMP, pdf.XMPErrorManagedProperty},
		{"invalid value", s(nil, &metadataSpec{
			CustomXMPSchemas:    []schemaSpec{{NS: cc, Properties: []descSpec{desc("n", tSimple("Integer"), "")}}},
			CustomXMPProperties: []propertySpec{prop(cc, "n", vText("x"))},
		}), `Xmp(InvalidValue("n"))`, okage.ErrPDFXMP, pdf.XMPErrorInvalidValue},
		{"missing description", s(map[string]any{"archival": "A3_B"}, &metadataSpec{
			Language: new("en"), CreationDate: fullDate(),
			CustomXMPProperties: []propertySpec{prop(fx, "DocumentType", vText("INVOICE"))},
		}), "MissingXmpPropertyDescription", okage.ErrPDFValidation, 0},
		{"missing date and language", s(map[string]any{"archival": "A2_U"}, &metadataSpec{}),
			"MissingDocumentDate", okage.ErrPDFValidation, 0},
		{"embedded file in PDF/A-2", s(map[string]any{"archival": "A2_B"}, &metadataSpec{CreationDate: fullDate()},
			embedSpec{Path: "x", Kind: "Data", Data: ""}),
			"EmbeddedFile(Existence", okage.ErrPDFValidation, 0},
		{"tags of PDF/A-3a", s(map[string]any{"archival": "A3_A"}, &metadataSpec{CreationDate: fullDate(), Language: new("en")}),
			"MissingTagging", okage.ErrPDFValidation, 0},
	}
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(answers[i].Err, c.oracle) {
				t.Errorf("the oracle gives %q, want an error with %q", answers[i].Err, c.oracle)
			}
			_, err := goScene(c.scene)
			if !errors.Is(err, c.want) {
				t.Fatalf("pdf gives the error %v, want %v", err, c.want)
			}
			var xe *pdf.XMPError
			if errors.As(err, &xe) && xe.Kind != c.kind {
				t.Errorf("pdf gives the kind %d, want %d", xe.Kind, c.kind)
			}
		})
	}
}
