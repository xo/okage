package krilla

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/pdf"
)

// The tests of this file compare the metadata, the outline, the
// destinations and the links of the package pdf with krilla, through the
// question "krilla" of the oracle (the gate of unit 6 of phase 16 of the PDF
// writer in docs/PLAN.md). The module interchange.rs of the oracle reads the
// same fields. doc.go documents them.

// interchangeSpec holds the fields of a scene for the metadata, the outline
// and the named destinations.
type interchangeSpec struct {
	Metadata *metadataSpec `json:"metadata,omitempty"`
	// Outline calls SetOutline when it is not nil, even when it is empty.
	Outline           *[]nodeSpec `json:"outline,omitempty"`
	NamedDestinations []namedSpec `json:"named_destinations,omitempty"`
	// EmbeddedFiles holds the files that the scene embeds after its pages,
	// before its metadata (D99).
	EmbeddedFiles []embedSpec `json:"embedded_files,omitempty"`
}

type metadataSpec struct {
	Title           *string     `json:"title,omitempty"`
	Description     *string     `json:"description,omitempty"`
	Creator         *string     `json:"creator,omitempty"`
	Producer        *string     `json:"producer,omitempty"`
	Keywords        []string    `json:"keywords,omitempty"`
	Authors         []string    `json:"authors,omitempty"`
	DocumentID      *string     `json:"document_id,omitempty"`
	VersionID       *string     `json:"version_id,omitempty"`
	Language        *string     `json:"language,omitempty"`
	CreationDate    *dateSpec   `json:"creation_date,omitempty"`
	TextDirection   string      `json:"text_direction,omitempty"`
	PageLayout      string      `json:"page_layout,omitempty"`
	DisplayDocTitle *bool       `json:"display_doc_title,omitempty"`
	Trapped         *bool       `json:"trapped,omitempty"`
	CustomFields    [][2]string `json:"custom_fields,omitempty"`
	// CustomXMPProperties and CustomXMPSchemas hold the custom XMP
	// metadata (D99).
	CustomXMPProperties []propertySpec `json:"custom_xmp_properties,omitempty"`
	CustomXMPSchemas    []schemaSpec   `json:"custom_xmp_schemas,omitempty"`
}

// dateSpec is a DateTime. A nil field is not set.
type dateSpec struct {
	Year            uint16 `json:"year"`
	Month           *uint8 `json:"month,omitempty"`
	Day             *uint8 `json:"day,omitempty"`
	Hour            *uint8 `json:"hour,omitempty"`
	Minute          *uint8 `json:"minute,omitempty"`
	Second          *uint8 `json:"second,omitempty"`
	UTCOffsetHour   *int8  `json:"utc_offset_hour,omitempty"`
	UTCOffsetMinute *uint8 `json:"utc_offset_minute,omitempty"`
}

type nodeSpec struct {
	Text     string     `json:"text"`
	Dest     xyzSpec    `json:"dest"`
	Open     bool       `json:"open,omitempty"`
	Children []nodeSpec `json:"children,omitempty"`
}

type xyzSpec struct {
	Page int     `json:"page"`
	X    float32 `json:"x"`
	Y    float32 `json:"y"`
}

type namedSpec struct {
	xyzSpec

	Name string `json:"name"`
}

// destSpec is a destination: one of the fields is set.
type destSpec struct {
	XYZ   *xyzSpec   `json:"xyz,omitempty"`
	Named *namedSpec `json:"named,omitempty"`
}

// targetSpec is the target of a link: one of the fields is set.
type targetSpec struct {
	XYZ   *xyzSpec   `json:"xyz,omitempty"`
	Named *namedSpec `json:"named,omitempty"`
	Link  *string    `json:"link,omitempty"`
	Goto  *destSpec  `json:"goto,omitempty"`
}

type borderSpec struct {
	Width float32        `json:"width"`
	Color map[string]any `json:"color"`
}

type annotationSpec struct {
	Rect *[4]float32 `json:"rect,omitempty"`
	// QuadPoints is a pointer, so that an empty list is written.
	QuadPoints *[][8]float32 `json:"quad_points,omitempty"`
	Target     targetSpec    `json:"target"`
	Border     *borderSpec   `json:"border,omitempty"`
	Alt        *string       `json:"alt,omitempty"`
}

func goXYZ(s xyzSpec) pdf.XYZDestination {
	return pdf.NewXYZDestination(s.Page, path.PointFromXY(s.X, s.Y))
}

func goNamed(s namedSpec) pdf.NamedDestination {
	return pdf.NewNamedDestination(s.Name, goXYZ(s.xyzSpec))
}

func goDest(s destSpec) (pdf.Destination, error) {
	switch {
	case s.XYZ != nil:
		return goXYZ(*s.XYZ), nil
	case s.Named != nil:
		return goNamed(*s.Named), nil
	}
	return nil, errors.New("a destination has no field")
}

func goTarget(s targetSpec) (pdf.Target, error) {
	switch {
	case s.XYZ != nil:
		return goXYZ(*s.XYZ), nil
	case s.Named != nil:
		return goNamed(*s.Named), nil
	case s.Link != nil:
		return pdf.NewLinkAction(*s.Link), nil
	case s.Goto != nil:
		d, err := goDest(*s.Goto)
		if err != nil {
			return nil, err
		}
		return pdf.NewGotoAction(d), nil
	}
	return nil, errors.New("a target has no field")
}

// goBorderColor makes a color of a scene: {"rgb": [r, g, b]}, {"luma": l} or
// {"cmyk": [c, m, y, k]}.
func goBorderColor(c map[string]any) (pdf.Color, error) {
	u8s := func(v any) []uint8 {
		var out []uint8
		switch v := v.(type) {
		case []int:
			for _, x := range v {
				out = append(out, uint8(x))
			}
		case int:
			out = append(out, uint8(v))
		}
		return out
	}
	for k, v := range c {
		b := u8s(v)
		switch {
		case k == "rgb" && len(b) == 3:
			return pdf.NewColorRGB(b[0], b[1], b[2]), nil
		case k == "luma" && len(b) == 1:
			return pdf.NewColorLuma(b[0]), nil
		case k == "cmyk" && len(b) == 4:
			return pdf.NewColorCMYK(b[0], b[1], b[2], b[3]), nil
		}
	}
	return nil, fmt.Errorf("the color %v is not known", c)
}

// goNamedDestinations registers the named destinations of a scene, as
// register_named_destinations of the oracle does.
func goNamedDestinations(d *pdf.Document, s interchangeSpec) {
	for _, n := range s.NamedDestinations {
		d.RegisterNamedDestination(goNamed(n))
	}
}

// goAnnotations adds the link annotations of a page, as add_annotations of
// the oracle does.
func goAnnotations(page *pdf.Page, annotations []annotationSpec) error {
	for _, a := range annotations {
		target, err := goTarget(a.Target)
		if err != nil {
			return err
		}
		var link pdf.LinkAnnotation
		switch {
		case a.Rect != nil && a.QuadPoints == nil:
			r, ok := path.RectFromXYWH(a.Rect[0], a.Rect[1], a.Rect[2], a.Rect[3])
			if !ok {
				return fmt.Errorf("the rectangle %v is not valid", *a.Rect)
			}
			link = pdf.NewLinkAnnotation(r, target)
		case a.Rect == nil && a.QuadPoints != nil:
			quads := make([]pdf.Quadrilateral, len(*a.QuadPoints))
			for i, v := range *a.QuadPoints {
				quads[i] = pdf.Quadrilateral{
					path.PointFromXY(v[0], v[1]), path.PointFromXY(v[2], v[3]),
					path.PointFromXY(v[4], v[5]), path.PointFromXY(v[6], v[7]),
				}
			}
			link = pdf.NewLinkAnnotationWithQuadPoints(quads, target)
		default:
			return errors.New("an annotation needs a rect or quad_points")
		}
		if b := a.Border; b != nil {
			c, err := goBorderColor(b.Color)
			if err != nil {
				return err
			}
			link = link.WithBorder(pdf.NewLinkBorder(b.Width, c))
		}
		page.AddAnnotation(pdf.NewAnnotationLink(link, a.Alt))
	}
	return nil
}

func goNode(s nodeSpec) pdf.OutlineNode {
	n := pdf.NewOutlineNode(s.Text, goXYZ(s.Dest)).WithOpen(s.Open)
	for _, c := range s.Children {
		n.PushChild(goNode(c))
	}
	return n
}

func goDate(s dateSpec) pdf.DateTime {
	d := pdf.NewDateTime(s.Year)
	if s.Month != nil {
		d = d.Month(*s.Month)
	}
	if s.Day != nil {
		d = d.Day(*s.Day)
	}
	if s.Hour != nil {
		d = d.Hour(*s.Hour)
	}
	if s.Minute != nil {
		d = d.Minute(*s.Minute)
	}
	if s.Second != nil {
		d = d.Second(*s.Second)
	}
	if s.UTCOffsetHour != nil {
		d = d.UTCOffsetHour(*s.UTCOffsetHour)
	}
	if s.UTCOffsetMinute != nil {
		d = d.UTCOffsetMinute(*s.UTCOffsetMinute)
	}
	return d
}

func goMetadata(s metadataSpec) (pdf.Metadata, error) {
	m := pdf.NewMetadata()
	for _, f := range []struct {
		v   *string
		set func(pdf.Metadata, string) pdf.Metadata
	}{
		{s.Title, pdf.Metadata.Title},
		{s.Description, pdf.Metadata.Description},
		{s.Creator, pdf.Metadata.Creator},
		{s.Producer, pdf.Metadata.Producer},
		{s.DocumentID, pdf.Metadata.DocumentID},
		{s.VersionID, pdf.Metadata.VersionID},
		{s.Language, pdf.Metadata.Language},
	} {
		if f.v != nil {
			m = f.set(m, *f.v)
		}
	}
	// The order of the calls does not change the metadata, except for the
	// fields that an empty value leaves as they are.
	if s.Keywords != nil {
		m = m.Keywords(s.Keywords)
	}
	if s.Authors != nil {
		m = m.Authors(s.Authors)
	}
	if s.CreationDate != nil {
		m = m.CreationDate(goDate(*s.CreationDate))
	}
	switch s.TextDirection {
	case "":
	case "ltr":
		m = m.TextDirection(pdf.TextDirectionMetadataLeftToRight)
	case "rtl":
		m = m.TextDirection(pdf.TextDirectionMetadataRightToLeft)
	default:
		return m, fmt.Errorf("the text direction %q is not known", s.TextDirection)
	}
	if s.PageLayout != "" {
		layouts := map[string]pdf.PageLayout{
			"SinglePage": pdf.PageLayoutSinglePage, "OneColumn": pdf.PageLayoutOneColumn,
			"TwoColumnLeft": pdf.PageLayoutTwoColumnLeft, "TwoColumnRight": pdf.PageLayoutTwoColumnRight,
			"TwoPageLeft": pdf.PageLayoutTwoPageLeft, "TwoPageRight": pdf.PageLayoutTwoPageRight,
		}
		l, ok := layouts[s.PageLayout]
		if !ok {
			return m, fmt.Errorf("the page layout %q is not known", s.PageLayout)
		}
		m = m.PageLayout(l)
	}
	if s.DisplayDocTitle != nil {
		m = m.DisplayDocTitle(*s.DisplayDocTitle)
	}
	if s.Trapped != nil {
		m = m.Trapped(*s.Trapped)
	}
	for _, f := range s.CustomFields {
		m, _ = m.CustomField(f[0], f[1])
	}
	return goCustomXMP(m, s)
}

// goFinishDocument embeds the files, then sets the metadata and the outline
// of a scene, as finish_document of the oracle does.
func goFinishDocument(d *pdf.Document, s interchangeSpec) error {
	if err := goEmbeddedFiles(d, s.EmbeddedFiles); err != nil {
		return err
	}
	if s.Metadata != nil {
		m, err := goMetadata(*s.Metadata)
		if err != nil {
			return err
		}
		d.SetMetadata(m)
	}
	if s.Outline != nil {
		outline := pdf.NewOutline()
		for _, n := range *s.Outline {
			outline.PushChild(goNode(n))
		}
		d.SetOutline(outline)
	}
	return nil
}

// Helpers that make the fields of the scenes of this file.
func xyz(page int, x, y float32) xyzSpec { return xyzSpec{Page: page, X: x, Y: y} }

func named(name string, page int, x, y float32) namedSpec {
	return namedSpec{Name: name, xyzSpec: xyz(page, x, y)}
}

func node(text string, dest xyzSpec, open bool, children ...nodeSpec) nodeSpec {
	return nodeSpec{Text: text, Dest: dest, Open: open, Children: children}
}

func outline(nodes ...nodeSpec) *[]nodeSpec {
	if nodes == nil {
		nodes = []nodeSpec{}
	}
	return &nodes
}

func linkRect(x, y, w, h float32, target targetSpec) annotationSpec {
	return annotationSpec{Rect: &[4]float32{x, y, w, h}, Target: target}
}

func toURI(uri string) targetSpec      { return targetSpec{Link: &uri} }
func toXYZ(s xyzSpec) targetSpec       { return targetSpec{XYZ: &s} }
func toNamed(s namedSpec) targetSpec   { return targetSpec{Named: &s} }
func gotoXYZ(s xyzSpec) targetSpec     { return targetSpec{Goto: &destSpec{XYZ: &s}} }
func gotoNamed(s namedSpec) targetSpec { return targetSpec{Goto: &destSpec{Named: &s}} }
func pageWith(w, h float32, a ...annotationSpec) pageSpec {
	p := page(w, h)
	p.Annotations = a
	return p
}

// fullDate is the date of the snapshot tests of krilla tests/src/metadata.rs.
func fullDate() *dateSpec {
	return &dateSpec{
		Year: 2024, Month: new(uint8(11)), Day: new(uint8(8)), Hour: new(uint8(22)), Minute: new(uint8(23)), Second: new(uint8(18)),
		UTCOffsetHour: new(int8(1)), UTCOffsetMinute: new(uint8(12)),
	}
}

// fullMetadata is metadata_impl of krilla tests/src/metadata.rs.
func fullMetadata() *metadataSpec {
	return &metadataSpec{
		CreationDate: fullDate(), Description: new("A very interesting subject"),
		Creator: new("krilla"), Producer: new("krilla"), Language: new("en"),
		Keywords: []string{"keyword1", "keyword2", "keyword3"}, Title: new("An awesome title"),
		Authors: []string{"John Doe", "Max Mustermann"}, TextDirection: "ltr",
		PageLayout: "TwoColumnRight", DisplayDocTitle: new(true), Trapped: new(true),
	}
}

// interchangeCases returns the scenes of the metadata, the outlines and the
// links.
func interchangeCases() []documentCase {
	var cases []documentCase
	pretty := func() map[string]any {
		return map[string]any{"xmp_metadata": false, "pretty": true, "ascii_compatible": true, "compress_content_streams": false}
	}
	withVersion := func(settings map[string]any, version string) map[string]any {
		settings["version"] = version
		return settings
	}
	add := func(name string, settings map[string]any, spec interchangeSpec, pages ...pageSpec) {
		if pages == nil {
			pages = []pageSpec{}
		}
		cases = append(cases, documentCase{name, scene{Settings: settings, Pages: pages, interchangeSpec: spec}})
	}

	// The metadata, in each version.
	for _, v := range []string{"1.4", "1.5", "1.7", "2.0"} {
		add("metadata full "+v, withVersion(pretty(), v), interchangeSpec{Metadata: fullMetadata()})
	}
	add("metadata compressed", map[string]any{"xmp_metadata": false}, interchangeSpec{Metadata: fullMetadata()})
	add("metadata empty", pretty(), interchangeSpec{Metadata: &metadataSpec{}})
	add("metadata document id", pretty(), interchangeSpec{Metadata: &metadataSpec{
		DocumentID: new("my id"), Title: new("t"), Authors: []string{"a"},
	}})
	add("metadata title and authors", pretty(), interchangeSpec{Metadata: &metadataSpec{
		Title: new("Ünïcödé (title) \\ 😀"), Authors: []string{"a", "b, c", ""},
	}})
	add("metadata title only", pretty(), interchangeSpec{Metadata: &metadataSpec{Title: new("t")}})
	add("metadata empty strings", pretty(), interchangeSpec{Metadata: &metadataSpec{
		Title: new(""), Description: new(""), Creator: new(""), Producer: new(""),
		Keywords: []string{}, Authors: []string{}, Language: new(""),
	}})
	add("metadata custom fields", pretty(), interchangeSpec{Metadata: &metadataSpec{
		Title: new("Standard title"), Trapped: new(false),
		CustomFields: [][2]string{
			{"CustomField", "old value"}, {"CustomField", "new value"}, {"Custom/Key", "Custom value"},
			{"", "empty"}, {"Title", "reserved"}, {"ModDate", "reserved"}, {"B", "b"}, {"A", "ä"},
			{"a b#(c)", "x"},
		},
	}})
	add("metadata custom fields on PDF 2.0", withVersion(pretty(), "2.0"), interchangeSpec{Metadata: &metadataSpec{
		Title: new("t"), CustomFields: [][2]string{{"Z", "z"}},
	}})
	add("metadata dates", pretty(), interchangeSpec{Metadata: &metadataSpec{
		CreationDate: &dateSpec{Year: 65535, Month: new(uint8(0)), Day: new(uint8(99)), Hour: new(uint8(99)), Minute: new(uint8(99)),
			Second: new(uint8(99)), UTCOffsetHour: new(int8(-128)), UTCOffsetMinute: new(uint8(255))},
	}})
	add("metadata short date", pretty(), interchangeSpec{Metadata: &metadataSpec{
		CreationDate: &dateSpec{Year: 1999, UTCOffsetMinute: new(uint8(30))},
	}})
	add("metadata negative offset", pretty(), interchangeSpec{Metadata: &metadataSpec{
		CreationDate: &dateSpec{Year: 2000, Month: new(uint8(2)), UTCOffsetHour: new(int8(-5)), UTCOffsetMinute: new(uint8(30))},
	}})
	for _, layout := range []string{"SinglePage", "OneColumn", "TwoColumnLeft", "TwoColumnRight", "TwoPageLeft", "TwoPageRight"} {
		for _, v := range []string{"1.4", "1.7"} {
			add("metadata layout "+layout+" "+v, withVersion(pretty(), v), interchangeSpec{Metadata: &metadataSpec{
				PageLayout: layout, TextDirection: "rtl",
			}})
		}
	}
	add("metadata version id only", pretty(), interchangeSpec{Metadata: &metadataSpec{VersionID: new("1.2.3")}})
	add("metadata display doc title false", pretty(), interchangeSpec{Metadata: &metadataSpec{DisplayDocTitle: new(false)}})
	for _, v := range []string{"1.7", "2.0"} {
		add("metadata trapped only "+v, withVersion(pretty(), v), interchangeSpec{Metadata: &metadataSpec{Trapped: new(false)}})
	}

	// The outlines.
	three := []pageSpec{page(200, 200), page(300, 400), page(100, 50)}
	add("outline simple", pretty(), interchangeSpec{Outline: outline(
		node("Heading 1", xyz(0, 0, 0), false, node("Heading 1.1", xyz(1, 50, 50), false)),
		node("Heading 2", xyz(2, 100, 100), false),
	)}, three...)
	add("outline mixed state", pretty(), interchangeSpec{Outline: outline(
		node("Heading 1", xyz(0, 0, 0), true,
			node("Heading 1.1", xyz(1, 50, 50), true, node("Heading 1.1.1", xyz(1, 60, 60), false))),
		node("Heading 2", xyz(2, 100, 100), false, node("Heading 2.1", xyz(2, 120, 120), false)),
	)}, three...)
	add("outline deep", pretty(), interchangeSpec{Outline: outline(
		node("1", xyz(0, 1.5, -2.25), true, node("2", xyz(1, 1e6, 3), true,
			node("3", xyz(2, 0, 0), false, node("4", xyz(2, 0, 0), true, node("5", xyz(0, 0, 0), true))),
			node("3b", xyz(2, 0, 0), true), node("3c", xyz(2, 7, 7), true, node("x", xyz(1, 0, 0), false)))),
	)}, three...)
	add("outline empty", pretty(), interchangeSpec{Outline: outline()})
	add("outline empty title", pretty(), interchangeSpec{Outline: outline(node("", xyz(0, 0, 0), false))})
	add("outline unicode", pretty(), interchangeSpec{Outline: outline(node("Überschrift (1) \\ Ωμέγα 😀", xyz(0, 3, 4), false))})
	var siblings []nodeSpec
	for i := range 40 {
		siblings = append(siblings, node(fmt.Sprint("n", i), xyz(i%3, float32(i), float32(i)), i%2 == 0,
			node("c", xyz(0, 0, 0), false)))
	}
	add("outline many siblings", pretty(), interchangeSpec{Outline: outline(siblings...)}, three...)
	add("outline compressed", map[string]any{"xmp_metadata": false}, interchangeSpec{Outline: outline(
		node("a", xyz(0, 0, 0), true, node("b", xyz(1, 0, 0), false)))}, three...)

	// The links and the destinations.
	add("link to uri", pretty(), interchangeSpec{},
		pageWith(200, 200, linkRect(50, 50, 100, 100, toURI("https://www.youtube.com"))))
	for _, border := range []map[string]any{rgb(255, 0, 0), luma(128), cmyk(10, 20, 30, 40)} {
		a := linkRect(50, 50, 100, 100, toURI("https://example.com/(x)"))
		a.Border = &borderSpec{Width: 1.5, Color: border}
		add(fmt.Sprint("link with border ", border), pretty(), interchangeSpec{}, pageWith(200, 200, a))
	}
	a := linkRect(50, 50, 100, 100, toURI("u"))
	a.Border = &borderSpec{Width: 0, Color: rgb(0, 0, 0)}
	add("link with zero border", pretty(), interchangeSpec{}, pageWith(200, 200, a))
	quad := annotationSpec{
		QuadPoints: &[][8]float32{{0, 50, 50, 50, 50, 0, 0, 0}, {50, 100, 100, 100, 100, 50, 50, 50}},
		Target:     toURI("https://www.youtube.com"),
	}
	for _, v := range []string{"1.5", "1.6", "1.7"} {
		add("link with quad points "+v, withVersion(pretty(), v), interchangeSpec{}, pageWith(200, 200, quad))
	}
	oneQuad := annotationSpec{QuadPoints: &[][8]float32{{1, 2, 3, 4, 5, 6, 7, 8}}, Target: toURI("x")}
	add("link with one quad", pretty(), interchangeSpec{}, pageWith(200, 200, oneQuad))
	add("links to destinations", pretty(), interchangeSpec{},
		pageWith(200, 200,
			linkRect(50, 0, 100, 100, toXYZ(xyz(1, 100, 100))),
			linkRect(0, 0, 10, 10, gotoXYZ(xyz(1, 100, 100))),
			linkRect(0, 0, 10, 10, gotoXYZ(xyz(0, 5, 5)))),
		pageWith(300, 400, linkRect(50, 100, 100, 100, toXYZ(xyz(0, 0, 0)))))
	add("named destinations", pretty(), interchangeSpec{},
		pageWith(595, 842,
			linkRect(0, 0, 100, 100, toNamed(named("hi", 0, 100, 100))),
			linkRect(100, 100, 100, 100, toNamed(named("by", 1, 0, 0)))),
		pageWith(595, 842, linkRect(0, 0, 100, 100, toNamed(named("hi", 0, 100, 100)))))
	// krilla sorts the names by their bytes in UTF-8, and writes them as byte
	// strings. qpdf reads a name as text, so it finds the names "Ä" and "é"
	// out of order. This fault of upstream is ported, and the case leaves
	// "Ä" out, so that qpdf accepts the file.
	var sorting []annotationSpec
	for _, n := range []string{"aaa", "bb", "a", "zzz", "y", "aa", "x", "ab", "é", "B"} {
		sorting = append(sorting, linkRect(0, 0, 100, 100, toNamed(named(n, 0, 0, 0))))
	}
	add("named destinations sorting", pretty(), interchangeSpec{}, pageWith(595, 842, sorting...))
	add("named destinations registered", pretty(), interchangeSpec{
		NamedDestinations: []namedSpec{named("z", 1, 1, 1), named("a", 0, 2, 2), named("z", 1, 1, 1), named("z", 0, 9, 9)},
	}, pageWith(100, 100, linkRect(0, 0, 10, 10, gotoNamed(named("g", 1, 3, 3)))), page(100, 100))
	alt := linkRect(0, 0, 10, 10, toURI("x"))
	alt.Alt = new("alt text (é)")
	emptyAlt := linkRect(0, 0, 10, 10, toURI("y"))
	emptyAlt.Alt = new("")
	add("links with alt text", pretty(), interchangeSpec{}, pageWith(100, 100, alt, emptyAlt))
	add("links on PDF 1.4", withVersion(pretty(), "1.4"), interchangeSpec{},
		pageWith(100, 100, linkRect(0, 0, 10, 10, toURI("x"))))
	add("links compressed", map[string]any{"xmp_metadata": false}, interchangeSpec{},
		pageWith(100, 100, linkRect(0, 0, 10, 10, toURI("x")), linkRect(0, 0, 10, 10, toXYZ(xyz(0, 1, 1)))))

	// Everything at once, with surfaces.
	full := interchangeSpec{
		Metadata:          fullMetadata(),
		Outline:           outline(node("a", xyz(1, 0, 0), true, node("b", xyz(0, 10, 10), false))),
		NamedDestinations: []namedSpec{named("start", 0, 0, 0)},
	}
	p1 := pageWith(200, 200, linkRect(10, 10, 50, 50, toNamed(named("start", 0, 0, 0))), linkRect(60, 60, 50, 50, toXYZ(xyz(0, 0, 0))))
	p1.Surface = nil
	p1.Ops = []op{{"op": "set_fill", "fill": map[string]any{"paint": rgb(255, 0, 0)}}, {"op": "draw_path", "path": rect(10, 10, 50, 50)}}
	p1.Label = label("r", "", 1)
	add("everything", pretty(), full, p1, pageWith(200, 200, linkRect(0, 0, 200, 200, gotoXYZ(xyz(0, 100, 100)))))
	return cases
}

// TestInterchangeGo compares the files of pdf with the files of krilla for
// the scenes of interchangeCases, as TestDocumentGo does.
func TestInterchangeGo(t *testing.T) {
	cases := interchangeCases()
	compareScenes(t, cases)
}

// compareScenes asks the oracle for the files of the scenes, and compares
// them with the files of pdf: the bytes when no stream is compressed, the
// objects after the reader decodes the streams, and qpdf.
func compareScenes(t *testing.T, cases []documentCase) {
	t.Helper()
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = sceneQuestion{Krilla: c.scene}
	}
	answers := ask(t, questions...)
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if answers[i].Err != "" {
				t.Fatalf("the oracle gives an error: %s", answers[i].Err)
			}
			got, err := goScene(c.scene)
			if err != nil {
				t.Fatal(err)
			}
			want := answers[i].Data
			if compress, ok := c.scene.Settings["compress_content_streams"].(bool); ok && !compress {
				if !bytes.Equal(got, want) {
					t.Errorf("the bytes differ: %s", firstDiff(got, want))
				}
			}
			gotFile, err := readPDF(got)
			if err != nil {
				t.Fatalf("reading the file of pdf: %v", err)
			}
			wantFile, err := readPDF(want)
			if err != nil {
				t.Fatalf("reading the file of krilla: %v", err)
			}
			if diffs := comparePDF(gotFile, wantFile); len(diffs) != 0 {
				t.Errorf("%d differences:\n%s", len(diffs), strings.Join(diffs[:min(len(diffs), 10)], "\n"))
			}
			qpdfCheck(t, writeTemp(t, "go.pdf", got))
		})
	}
}

// TestInterchangeGoErrors compares the errors: krilla returns the error of a
// named destination with two destinations, and panics where pdf returns
// ErrPDFInvalidDestination or ErrPDFInvalidAnnotation.
func TestInterchangeGoErrors(t *testing.T) {
	s := func(spec interchangeSpec, pages ...pageSpec) scene {
		if pages == nil {
			// The field pages of a scene is not optional.
			pages = []pageSpec{}
		}
		return scene{Settings: map[string]any{"xmp_metadata": false}, Pages: pages, interchangeSpec: spec}
	}
	cases := []struct {
		name   string
		scene  scene
		oracle string
		want   error
	}{
		{"duplicate named destination", s(interchangeSpec{NamedDestinations: []namedSpec{named("n", 0, 0, 0)}},
			pageWith(100, 100, linkRect(0, 0, 1, 1, toNamed(named("n", 0, 1, 0))))),
			"DuplicateNamedDestination", okage.ErrPDFDuplicateNamedDestination},
		{"duplicate named destination in a go-to", s(interchangeSpec{},
			pageWith(100, 100, linkRect(0, 0, 1, 1, toNamed(named("n", 0, 0, 0))),
				linkRect(0, 0, 1, 1, gotoNamed(named("n", 0, 0, 1))))),
			"DuplicateNamedDestination", okage.ErrPDFDuplicateNamedDestination},
		{"link to a page that does not exist", s(interchangeSpec{},
			pageWith(100, 100, linkRect(0, 0, 1, 1, toXYZ(xyz(1, 0, 0))))),
			"panic: attempted to link to page 2, but document only has 1 pages", okage.ErrPDFInvalidDestination},
		{"outline to a page that does not exist", s(interchangeSpec{Outline: outline(node("x", xyz(3, 0, 0), false))},
			page(100, 100)),
			"panic: attempted to link to page 4", okage.ErrPDFInvalidDestination},
		{"registered destination to a page that does not exist", s(interchangeSpec{NamedDestinations: []namedSpec{named("x", 9, 0, 0)}}),
			"panic: attempted to link to page 10", okage.ErrPDFInvalidDestination},
		{"no quadrilateral", s(interchangeSpec{},
			pageWith(100, 100, annotationSpec{QuadPoints: &[][8]float32{}, Target: toURI("x")})),
			"panic", okage.ErrPDFInvalidAnnotation},
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
			if _, err := goScene(c.scene); !errors.Is(err, c.want) {
				t.Errorf("pdf gives the error %v, want %v", err, c.want)
			}
		})
	}
}

// randomInterchange makes a random scene with metadata, an outline, named
// destinations and links, whose destinations are all on pages that exist.
func randomInterchange(r *rand.Rand) scene {
	versions := []string{"1.4", "1.5", "1.6", "1.7", "2.0"}
	settings := map[string]any{
		"xmp_metadata": r.IntN(2) == 0, "pretty": r.IntN(2) == 0, "ascii_compatible": r.IntN(2) == 0,
		"compress_content_streams": r.IntN(4) == 0, "version": versions[r.IntN(len(versions))],
	}
	nPages := 1 + r.IntN(4)
	coord := func() float32 {
		if r.IntN(3) == 0 {
			return float32(r.NormFloat64() * 1000)
		}
		return float32(r.IntN(400))
	}
	pt := func() xyzSpec { return xyz(r.IntN(nPages), coord(), coord()) }
	names := []string{"a", "b", "c", "ab", "é", "a b"}
	// Each name has one destination, so that no error stops the file.
	nameDest := map[string]xyzSpec{}
	nm := func() namedSpec {
		n := names[r.IntN(len(names))]
		d, ok := nameDest[n]
		if !ok {
			d = pt()
			nameDest[n] = d
		}
		return namedSpec{Name: n, xyzSpec: d}
	}
	text := func() string {
		words := []string{"", "x", "Heading", "Ü (é)", "\\", "Ωμέγα", "😀"}
		return words[r.IntN(len(words))]
	}
	var spec interchangeSpec
	for range r.IntN(3) {
		spec.NamedDestinations = append(spec.NamedDestinations, nm())
	}
	var nodes func(depth int) []nodeSpec
	nodes = func(depth int) []nodeSpec {
		if depth > 3 {
			return nil
		}
		var out []nodeSpec
		for range r.IntN(4) {
			out = append(out, nodeSpec{Text: text(), Dest: pt(), Open: r.IntN(2) == 0, Children: nodes(depth + 1)})
		}
		return out
	}
	if r.IntN(2) == 0 {
		o := nodes(0)
		spec.Outline = &o
	}
	if r.IntN(2) == 0 {
		m := &metadataSpec{}
		if r.IntN(2) == 0 {
			m.Title = new(text())
		}
		if r.IntN(2) == 0 {
			m.Authors = []string{text(), text()}
		}
		if r.IntN(3) == 0 {
			m.DocumentID = new(text())
		}
		if r.IntN(2) == 0 {
			m.Language = new("de")
		}
		if r.IntN(2) == 0 {
			m.CreationDate = &dateSpec{Year: uint16(r.IntN(3000)), Month: new(uint8(r.IntN(14))), UTCOffsetHour: new(int8(r.IntN(60) - 30))}
		}
		layouts := []string{"", "SinglePage", "TwoPageLeft", "TwoColumnRight"}
		m.PageLayout = layouts[r.IntN(len(layouts))]
		dirs := []string{"", "ltr", "rtl"}
		m.TextDirection = dirs[r.IntN(len(dirs))]
		m.DisplayDocTitle = new(r.IntN(2) == 0)
		if r.IntN(3) == 0 {
			m.Trapped = new(r.IntN(2) == 0)
		}
		for range r.IntN(3) {
			m.CustomFields = append(m.CustomFields, [2]string{names[r.IntN(len(names))], text()})
		}
		spec.Metadata = m
	}
	colors := []map[string]any{rgb(1, 2, 3), luma(9), cmyk(1, 2, 3, 4)}
	var pages []pageSpec
	for range nPages {
		p := page(float32(50+r.IntN(500)), float32(50+r.IntN(500)))
		for range r.IntN(4) {
			var target targetSpec
			switch r.IntN(5) {
			case 0:
				target = toURI(text())
			case 1:
				target = toXYZ(pt())
			case 2:
				target = toNamed(nm())
			case 3:
				target = gotoXYZ(pt())
			default:
				target = gotoNamed(nm())
			}
			var a annotationSpec
			if r.IntN(3) == 0 {
				var quads [][8]float32
				for range 1 + r.IntN(2) {
					var q [8]float32
					for j := range q {
						q[j] = coord()
					}
					quads = append(quads, q)
				}
				a.QuadPoints = &quads
			} else {
				a.Rect = &[4]float32{coord(), coord(), float32(1 + r.IntN(100)), float32(1 + r.IntN(100))}
			}
			a.Target = target
			if r.IntN(2) == 0 {
				a.Border = &borderSpec{Width: float32(r.IntN(4)) / 2, Color: colors[r.IntN(len(colors))]}
			}
			if r.IntN(3) == 0 {
				a.Alt = new(text())
			}
			p.Annotations = append(p.Annotations, a)
		}
		pages = append(pages, p)
	}
	return scene{Settings: settings, Pages: pages, interchangeSpec: spec}
}

// TestInterchangeGoRandom compares the files of pdf with the files of
// krilla for random scenes of metadata, outlines and links.
func TestInterchangeGoRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(16, 6))
	var cases []documentCase
	for i := range 300 {
		cases = append(cases, documentCase{fmt.Sprint("random ", i), randomInterchange(r)})
	}
	compareScenes(t, cases)
}
