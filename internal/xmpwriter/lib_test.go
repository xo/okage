// Copyright 2023 The xmp-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmpwriter

import (
	"os"
	"strings"
	"testing"
)

// The crate xmp-writer has no tests. TestReference compares the port with
// testdata/reference.txt, which the Rust program testdata/reference.rs
// wrote with xmp-writer at the commit 9498ae2b, one packet on each line. The
// test makes the same calls in the same order.
func TestReference(t *testing.T) {
	data, err := os.ReadFile("testdata/reference.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	got := []string{example(), everyProperty()}
	if len(got) != len(want) {
		t.Fatalf("got %d packets, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			n := 0
			for n < len(got[i]) && n < len(want[i]) && got[i][n] == want[i][n] {
				n++
			}
			lo := max(0, n-40)
			t.Errorf("packet %d differs at byte %d:\ngot  %q\nwant %q", i, n,
				got[i][lo:min(len(got[i]), n+40)], want[i][lo:min(len(want[i]), n+40)])
		}
	}
}

// example is the example of the documentation of the crate.
func example() string {
	w := New()
	w.Creator("Martin Haug")
	de := LangID("de")
	w.Title(LangAlt{Lang: &de, Value: "Titel"}, LangAlt{Value: "Title"})
	w.NumPages(3)
	w.PDFKeywords("Keyword1, Keyword2")
	w.Description(LangAlt{Value: "Description"})
	w.Date(Date(2021, 11, 6))
	colors := w.Colorants()
	colors.AddColorant().SwatchName("Red").Finish()
	colors.AddColorant().SwatchName("Green").Finish()
	colors.Finish()
	w.CreatorTool("xmp-writer 0.3.3")
	return w.Finish("")
}

// everyProperty writes each property of the crate.
func everyProperty() string {
	w := New()
	w.Contributor("a", "b<&>'\"").Coverage("c").
		Date(Year(2020), LocalTime(2021, 1, 2, 3, 4, 5),
			NewDateTime(2022, 12, 31, 23, 59, 58, Timezone{Local: true, Hour: -5, Minute: 30}),
			NewDateTime(2023, 1, 1, 0, 0, 0, Timezone{})).
		Format("application/pdf").Identifier("id").Language("en", "de").Publisher("p").Relation("r").
		Rights(LangAlt{Value: "r"}).Source("s").Subject("x", "y").Type("t")
	w.BaseURL("http://a?b=1&c=2").CreateDate(Date(2000, 1, 1)).XMPIdentifier("i").Label("l").
		MetadataDate(Year(1999)).ModifyDate(Year(1998)).Nickname("n").Rating(-1)

	th := w.Thumbnails()
	th.AddThumbnail().FormatJPEG().Width(10).Height(20).Image("abc").Finish()
	th.Finish()

	fr := LangID("fr")
	w.Certificate("c").Marked(true).Owner("o").UsageTerms(LangAlt{Lang: &fr, Value: "u"}).WebStatement("ws")

	format, cs := "JPEG", "RGB"
	d := w.DerivedFrom()
	d.AlternatePaths("p1").DocumentID("d").FilePath("f").InstanceID("i").LastModifyDate(Year(2001)).
		Manager("m").ManagerVariant("mv").ManageTo("mt").ManageUI("mu").MaskMarkers(MaskMarkersAll).
		PartMapping("pm").RenditionClass(RenditionClassThumbnail(&format, &[2]uint32{4, 5}, &cs)).
		RenditionParams("rp").ToPart("tp").VersionID("v")
	d.Finish()
	w.DocumentID("doc")

	h := w.History()
	h.AddEvent().Action(ResourceEventActionSaved).Changed("c").InstanceID("i").Parameters("p").
		SoftwareAgent("s").When(Year(2002)).Finish()
	h.AddEvent().Action("x").Finish()
	h.Finish()

	in := w.Ingredients()
	in.AddRef().MaskMarkers(MaskMarkersNone).Finish()
	in.Finish()

	w.InstanceID("inst")
	w.ManagedFrom().DocumentID("x").Finish()
	w.Manager("m").ManageTo("mt").ManageUI("mu").ManagerVariant("mv").OriginalDocID("o")

	p := w.Pantry()
	p.AddItem().InstanceID("pi").Finish()
	p.Finish()

	w.RenditionClass(RenditionClassLowResolution).RenditionParams("rp").VersionID("vi")

	v := w.VersionRef()
	ver := v.AddVersion()
	ver.Comments("c").Modifier("m").ModifyDate(Year(2003)).Version("1")
	ver.Event().Action(ResourceEventActionConverted).Finish()
	ver.Finish()
	v.Finish()

	j := w.Jobs()
	j.AddJob().ID("1").Name("n").URL("u").Finish()
	j.Finish()

	c := w.Colorants()
	c.AddColorant().Type(ColorantTypeSpot).ColorantMode(ColorantModeLab).L(1.5).A(-2).B(3).
		Black(0.25).Cyan(1.0).Magenta(1e21).Yellow(0.1).Red(1).Green(2).Blue(3).Finish()
	c.Finish()

	f := w.Fonts()
	f.AddFont().ChildFontFiles("a").Composite(false).FontFace("f").FontFamily("fa").FontFile("ff").
		FontName("fn").FontType(FontTypeOpenType).VersionString("1").Finish()
	f.Finish()

	w.MaxPageSize().Width(1.25).Height(2.0).Unit(DimensionUnitMm).Finish()
	w.NumPages(7).PlateNames("p").IdqScheme("s").PDFKeywords("k").PDFVersion("1.7").Producer("p").Trapped(false)
	w.PDFAPart(4).PDFAAmd("a").PDFACorr("c").PDFAConformance("F").PDFARev(2020).PDFXVersion("x").
		PDFUAPart(1).PDFUAAmd("a").PDFUACorr("c").PDFUARev(2024)

	s := w.ExtensionSchemas()
	s.PDFAID(true)
	ua := s.PDFUAID()
	ua.Properties().DescribeAll(PDFUAIDDescriptionModeAll)
	ua.Finish()
	pdf := s.PDF()
	pdf.Properties().DescribeAll()
	pdf.Finish()
	xmp := s.XMP()
	xmp.Properties().DescribeAll()
	xmp.Finish()
	mm := s.XMPMediaManagement()
	mm.Properties().DescribeAll()
	mm.Finish()
	pg := s.PagedText()
	pg.Properties().DescribeAll()
	pg.Finish()
	re := s.ResourceEvent()
	rp := re.Properties()
	rp.DescribeChanged()
	rp.Finish()
	re.Finish()
	tn := s.Thumbnail()
	tn.Properties().DescribeAll()
	tn.Finish()
	sch := s.AddSchema()
	sch.Namespace(NewCustom(NewCustomNamespace("Name", "zz", "http://z/")))
	vt := sch.ValueTypes()
	ty := vt.AddValueType()
	ty.Name("T").Namespace(NamespaceXmpJob).Description("d")
	fields := ty.Fields()
	fields.AddField().Name("f").ValueType("Text").Description("fd").Finish()
	fields.Finish()
	ty.Finish()
	vt.Finish()
	sch.Finish()
	s.Finish()

	w.Element("custom", NewCustom(NewCustomNamespace("A", "aa", "http://a/"))).Value(Float32(1.5))
	w.Element("custom2", NewCustom(NewCustomNamespace("A", "ab", "http://a/"))).AlternativeArray(Int64(1), Int64(2))
	return w.Finish("about")
}

// TestRating is the conversion of Rating::from_stars and Rating::to_xmp.
func TestRating(t *testing.T) {
	five := uint32(5)
	if r := RatingFromStars(&five); r.ToXMP() != 5 {
		t.Errorf("five stars give %v", r.ToXMP())
	}
	if r := RatingFromStars(nil); r.ToXMP() != 0 {
		t.Errorf("no stars give %v", r.ToXMP())
	}
	if RatingRejected.ToXMP() != -1 {
		t.Errorf("rejected gives %v", RatingRejected.ToXMP())
	}
	defer func() {
		if recover() == nil {
			t.Error("six stars do not panic")
		}
	}()
	six := uint32(6)
	RatingFromStars(&six)
}
