// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// The tests of pdf-writer src/content.rs, and the test test_content_limits
// of src/buf.rs, which needs the writers of src/content.rs.

// TestContentEncoding is test_content_encoding in pdf-writer
// src/content.rs.
func TestContentEncoding(t *testing.T) {
	content := NewContent()
	content.
		SaveState().
		Rect(1.0, 2.0, 3.0, 4.0).
		FillNonzero().
		SetDashPattern([]float32{7.0, 2.0}, 4.0).
		XObject(Name("MyImage")).
		SetFillPattern([]float32{2.0, 3.5}, Name("MyPattern")).
		RestoreState()

	testBytes(t, content.Finish().IntoVec(),
		"q\n1 2 3 4 re\nf\n[7 2] 4 d\n/MyImage Do\n2 3.5 /MyPattern scn\nQ")
}

// TestContentText is test_content_text in pdf-writer src/content.rs.
func TestContentText(t *testing.T) {
	content := NewContent()

	content.SetFont(Name("F1"), 12.0)
	content.BeginText()
	content.ShowPositioned().Items()
	content.
		ShowPositioned().
		Items().
		Show(Str("AB")).
		Adjust(2.0).
		Show(Str("CD"))
	content.EndText()

	testBytes(t, content.Finish().IntoVec(),
		"/F1 12 Tf\nBT\n[] TJ\n[(AB) 2 (CD)] TJ\nET")
}

// TestContentArrayNoPretty is test_content_array_no_pretty in pdf-writer
// src/content.rs.
func TestContentArrayNoPretty(t *testing.T) {
	content := NewContentWithSettings(Settings{Pretty: false})

	content.SetFont(Name("F1"), 12.0)
	content.SetFont(Name("F2"), 15.0)
	content.BeginText()
	content.ShowPositioned().Items()
	content.
		ShowPositioned().
		Items().
		Show(Str("AB")).
		Adjust(2.0).
		Show(Str("CD")).
		Adjust(4.0).
		Show(Str("EF"))
	content.EndText()

	testBytes(t, content.Finish().IntoVec(),
		"/F1 12 Tf/F2 15 Tf\nBT[]TJ[(AB)2(CD)4(EF)]TJ\nET")
}

// TestContentDictNoPretty is test_content_dict_no_pretty in pdf-writer
// src/content.rs.
func TestContentDictNoPretty(t *testing.T) {
	content := NewContentWithSettings(Settings{Pretty: false})

	mc := content.BeginMarkedContentWithProperties(Name("Test"))
	properties := mc.Properties()
	properties.ActualText(TextStr("Actual")).Identify(1)
	properties.Artifact().Kind(ArtifactTypeBackground)
	mc.Finish()

	testBytes(t, content.Finish().IntoVec(),
		"/Test<</ActualText(Actual)/MCID 1/Type/Background>>BDC")
}

// TestContentLimits is test_content_limits in pdf-writer src/buf.rs.
func TestContentLimits(t *testing.T) {
	limits := NewLimits()

	content := NewContent()
	content.CubicTo(14.3, 16.2, 22.6, 30.9, 50.1, 40.0)
	content.Show(Str("Some text"))
	content.SetFont(Name("NotoSans"), 10.0)
	limits.Merge(content.Finish().Limits())

	content = NewContent()
	content.LineTo(55.0, -75.3)
	content.SetFont(Name("Noto"), 10.0)
	content.
		ShowPositioned().
		Items().
		Show(Str("A")).
		Show(Str("B")).
		Adjust(32.0)
	content.
		MarkedContentPointWithProperties(Name("Hi")).
		Properties().
		ActualText(TextStr("text"))
	limits.Merge(content.Finish().Limits())

	want := Limits{int: 55, real: 75.3, nameLen: 10, strLen: 9, arrayLen: 3, dictEntries: 1}
	if limits != want {
		t.Errorf("got %+v, want %+v", limits, want)
	}
}

// The tests below port no test of pdf-writer.

// TestContentWriters writes each writer of content.rs that the tests of
// upstream do not reach: the graphics state, the color operands, the
// operations that finish themselves, and the dictionaries.
func TestContentWriters(t *testing.T) {
	content := NewContent()
	content.
		SetLineWidth(0).
		SetLineCap(LineCapStyleProjectingSquareCap).
		SetLineJoin(LineJoinStyleRoundJoin).
		SetRenderingIntent(RenderingIntentPerceptual).
		SetFlatness(100).
		SetStrokeColorSpace(ColorSpaceOperand{Kind: ColorSpaceOperandDeviceCMYK}).
		SetFillColorSpace(ColorSpaceOperandFromName(Name("Cs1"))).
		SetFillColor().
		SetTextRenderingMode(TextRenderingModeClip).
		Transform([6]float32{1, 0, 0, -1, 0.5, 10}).
		NextLineShowAndSetWordAndCharSpacing(1, 2, Str("x"))
	content.MarkedContentPointWithProperties(Name("Tag")).PropertiesNamed(Name("P0"))
	content.BeginMarkedContentWithProperties(Name("Artifact")).Properties().Artifact().
		Subtype(ArtifactSubtype{Kind: ArtifactSubtypeCustom, Name: Name("Mine")}).
		BoundingBox(NewRect(0, 0, 1, 2)).
		Attached(ArtifactAttachmentTop, ArtifactAttachmentLeft)
	content.SaveState().SaveState().RestoreState().RestoreState().RestoreState()
	if got := content.StateNestingDepth(); got != 0 {
		t.Errorf("the depth is %d, want 0", got)
	}
	content.SaveState()
	if got := content.StateNestingDepth(); got != 1 {
		t.Errorf("the depth is %d, want 1", got)
	}

	testBytes(t, content.Finish().IntoVec(),
		"0 w",
		"2 J",
		"1 j",
		"/Perceptual ri",
		"100 i",
		"/DeviceCMYK CS",
		"/Cs1 cs",
		"scn",
		"7 Tr",
		"1 0 0 -1 0.5 10 cm",
		"1 2 (x) \"",
		"/Tag /P0 DP",
		"/Artifact <<",
		"  /Subtype /Mine",
		"  /BBox [0 0 1 2]",
		"  /Attached [/Top /Left]",
		">> BDC",
		"q",
		"q",
		"Q",
		"Q",
		"Q",
		"q")

	c := NewChunk()
	c.ExtGraphics(NewRef(1)).
		DashPattern([]float32{1, 2}, 0.5).
		Font(Name("F1"), 9).
		BlendMode(BlendModeColorDodge).
		OverprintMode(OverprintModeIgnoreZeroChannel).
		SoftMask().
		Subtype(MaskTypeLuminosity).
		Backdrop(0, 1)
	r := Start[Resources](c.Indirect(NewRef(2)))
	r.ProcSetsAll()
	r.Properties().Insert(Name("P0")).Identify(3)
	testBytes(t, c.AsBytes(),
		"1 0 obj",
		"<<",
		"  /Type /ExtGState",
		"  /D [[1 2] 0.5]",
		"  /Font [/F1 9]",
		"  /BM /ColorDodge",
		"  /OPM 1",
		"  /SMask <<",
		"    /Type /Mask",
		"    /S /Luminosity",
		"    /BC [0 1]",
		"  >>",
		">>",
		"endobj",
		"",
		"2 0 obj",
		"<<",
		"  /ProcSet [/PDF /Text /ImageB /ImageC /ImageI]",
		"  /Properties <<",
		"    /P0 <<",
		"      /MCID 3",
		"    >>",
		"  >>",
		">>",
		"endobj",
		"",
		"")
}

// TestContentPanics makes sure that the checks of the operands panic with
// the messages of upstream.
func TestContentPanics(t *testing.T) {
	for _, c := range []struct {
		name string
		f    func(c *Content)
		want string
	}{
		{"negative width", func(c *Content) { c.SetLineWidth(-1) }, "line width must be positive"},
		{"negative flatness", func(c *Content) { c.SetFlatness(-1) }, "flatness tolerance must be between 0 and 100"},
		{"large flatness", func(c *Content) { c.SetFlatness(101) }, "flatness tolerance must be between 0 and 100"},
		{"finished operation", func(c *Content) {
			op := c.Op("x")
			c.Op("y")
			op.Operand(Int(1))
		}, "pdfwriter: the Operation is finished"},
	} {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != c.want {
					t.Errorf("got the panic %v, want %q", r, c.want)
				}
			}()
			c.f(NewContent())
		})
	}
}
