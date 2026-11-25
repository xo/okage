// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// The graphics/ folder of krilla has no unit tests. The tests of this file
// check the functions of the port that the snapshots and the scenes do not
// reach, against the code of upstream.

// TestColors checks the colors of krilla src/graphics/color.rs.
func TestColors(t *testing.T) {
	c := NewColorRGB(1, 2, 3)
	if c.Red() != 1 || c.Green() != 2 || c.Blue() != 3 || NewColorRGBLinear(1, 2, 3) != c {
		t.Errorf("the components are wrong: %+v", c)
	}
	if ColorRGBBlack() != (ColorRGB{}) || ColorRGBWhite() != NewColorRGB(255, 255, 255) {
		t.Error("black or white is wrong")
	}
	if ColorLumaBlack() != (ColorLuma{}) || ColorLumaWhite() != NewColorLuma(255) {
		t.Error("the luma black or white is wrong")
	}
	if DefaultColorCMYK() != NewColorCMYK(0, 0, 0, 255) {
		t.Error("the default CMYK color is not black")
	}
	if got := NewColorCMYK(0, 51, 102, 255).toPDFColor(); len(got) != 4 || got[1] != 0.2 || got[3] != 1 {
		t.Errorf("the CMYK components are %v", got)
	}

	tests := []struct {
		color       Color
		rgb         ColorRGB
		hasRGB      bool
		subtractive bool
	}{
		{NewColorRGB(9, 8, 7), NewColorRGB(9, 8, 7), true, false},
		{NewColorLuma(5), NewColorRGB(5, 5, 5), true, false},
		{NewColorCMYK(1, 2, 3, 4), ColorRGB{}, false, true},
	}
	for _, tt := range tests {
		rgb, ok := tt.color.asRGB()
		if rgb != tt.rgb || ok != tt.hasRGB || tt.color.isSubtractive() != tt.subtractive {
			t.Errorf("%T gives %v, %v and %v", tt.color, rgb, ok, tt.color.isSubtractive())
		}
		rgb, ok = paintAsRGB(tt.color)
		if rgb != tt.rgb || ok != tt.hasRGB {
			t.Errorf("paintAsRGB of %T gives %v and %v", tt.color, rgb, ok)
		}
	}
	if _, ok := paintAsRGB(LinearGradient{}); ok {
		t.Error("a gradient has an RGB color")
	}

	// The three colors of the same values do not share a hash.
	hashes := map[[2]uint64]bool{}
	for _, c := range []Paint{NewColorRGB(0, 0, 0), NewColorLuma(0), NewColorCMYK(0, 0, 0, 0)} {
		h := newHasher()
		c.hashPaint(h)
		hi, lo := h.finish128().AsU128()
		hashes[[2]uint64{hi, lo}] = true
	}
	if len(hashes) != 3 {
		t.Error("two colors share a hash")
	}
}

// TestColorSpaces checks the color spaces of the colors in krilla
// src/graphics/color.rs, with and without device color spaces.
func TestColorSpaces(t *testing.T) {
	d := NewDocument()
	for _, tt := range []struct {
		color Color
		kind  maybeDeviceKind
	}{
		{NewColorRGB(0, 0, 0), maybeDeviceRGB},
		{NewColorLuma(0), maybeDeviceGray},
		{NewColorCMYK(0, 0, 0, 0), maybeDeviceCMYK},
	} {
		if got := d.sc.registerColorSpace(d.cc, tt.color.colorSpace(d.sc)); got.kind != tt.kind {
			t.Errorf("%T gives the color space %d", tt.color, got.kind)
		}
	}

	// Without a CMYK profile, a device-independent CMYK color falls back to
	// the device CMYK color space.
	d = NewDocument(WithNoDeviceCS(true))
	if cs := NewColorCMYK(0, 0, 0, 0).colorSpace(d.sc); cs.kind != colorSpaceDeviceCMYK {
		t.Errorf("the CMYK color without a profile has the color space %d", cs.kind)
	}
	rgb := d.sc.registerColorSpace(d.cc, NewColorRGB(0, 0, 0).colorSpace(d.sc))
	luma := d.sc.registerColorSpace(d.cc, NewColorLuma(0).colorSpace(d.sc))
	again := d.sc.registerColorSpace(d.cc, NewColorRGB(1, 2, 3).colorSpace(d.sc))
	if rgb.kind != maybeDeviceResource || luma.kind != maybeDeviceResource || rgb == luma || rgb != again {
		t.Errorf("the ICC-based color spaces are %+v, %+v and %+v", rgb, luma, again)
	}
	if len(d.cc.streams.iccProfiles) != 2 {
		t.Errorf("the document holds %d ICC profiles", len(d.cc.streams.iccProfiles))
	}

	// maybeDeviceColorSpace.write writes a name or a reference.
	chunk := d.sc.newChunk()
	array := chunk.Indirect(pdfwriter.NewRef(1)).Array()
	for _, m := range []maybeDeviceColorSpace{
		{kind: maybeDeviceRGB}, {kind: maybeDeviceGray}, {kind: maybeDeviceCMYK},
		{kind: maybeDeviceResource, cs: colorSpaceResource{pdfwriter.NewRef(7)}},
	} {
		m.write(array.Push())
	}
	array.Finish()
	if got := string(chunk.AsBytes()); !strings.Contains(got, "[/DeviceRGB/DeviceGray/DeviceCMYK 7 0 R]") {
		t.Errorf("the color spaces are written as %q", got)
	}
}

// TestColorSpacePreferDevice checks color_space_prefer_device in krilla
// src/graphics/color.rs, and ContentBuilder::get_color_space in
// src/content.rs, which calls it for variable text.
func TestColorSpacePreferDevice(t *testing.T) {
	d := NewDocument(WithNoDeviceCS(true))
	for _, tt := range []struct {
		color  Color
		device colorSpaceKind
		other  colorSpaceKind
	}{
		{NewColorRGB(1, 2, 3), colorSpaceDeviceRGB, colorSpaceCIESRGB},
		{NewColorLuma(4), colorSpaceDeviceGray, colorSpaceCIELuma},
		// Without a CMYK profile, both fall back to the device CMYK color
		// space.
		{NewColorCMYK(1, 2, 3, 4), colorSpaceDeviceCMYK, colorSpaceDeviceCMYK},
	} {
		if cs := tt.color.colorSpacePreferDevice(d.sc); cs.kind != tt.device {
			t.Errorf("%T prefers the color space %d, want %d", tt.color, cs.kind, tt.device)
		}
		if cs := tt.color.colorSpace(d.sc); cs.kind != tt.other {
			t.Errorf("%T has the color space %d, want %d", tt.color, cs.kind, tt.other)
		}
		b := newContentBuilder(path.IdentityTransform(), false, d.sc)
		if cs := b.getColorSpace(d.sc, tt.color); cs.kind != tt.other {
			t.Errorf("%T outside variable text has the color space %d, want %d", tt.color, cs.kind, tt.other)
		}
		b.activeVariableText = true
		if cs := b.getColorSpace(d.sc, tt.color); cs.kind != tt.device {
			t.Errorf("%T in variable text has the color space %d, want %d", tt.color, cs.kind, tt.device)
		}
	}

	// A CMYK color with a profile keeps its profile in variable text.
	profile, ok := NewICCProfile(testCMYKProfile(), 4)
	if !ok {
		t.Fatal("the CMYK profile is not valid")
	}
	d = NewDocument(WithNoDeviceCS(true), WithCMYKProfile(profile))
	if cs := NewColorCMYK(0, 0, 0, 0).colorSpacePreferDevice(d.sc); cs.kind != colorSpaceCIECMYK {
		t.Errorf("the CMYK color with a profile prefers the color space %d", cs.kind)
	}
}

// testCMYKProfile is the header of an ICC profile of version 2.1 of the
// color space CMYK. The port reads only the header.
func testCMYKProfile() []byte {
	data := make([]byte, 128)
	data[8], data[9] = 2, 0x10
	copy(data[16:], "CMYK")
	return data
}

// TestICCProfile checks ICCProfile and ICCMetadata in krilla
// src/graphics/icc.rs, and the CMYK profile of the settings.
func TestICCProfile(t *testing.T) {
	if _, ok := NewICCProfile(make([]byte, 19), 4); ok {
		t.Error("a short profile is valid")
	}
	if _, ok := NewICCProfile(make([]byte, 128), 4); ok {
		t.Error("a profile of an unknown color space is valid")
	}
	if _, ok := NewICCProfile(testCMYKProfile(), 3); ok {
		t.Error("a CMYK profile is valid with 3 components")
	}
	p, ok := NewICCProfile(testCMYKProfile(), 4)
	if !ok {
		t.Fatal("the CMYK profile is not valid")
	}
	if m := p.metadata(); m.major != 2 || m.minor != 1 || m.colorSpace != iccColorSpaceCMYK {
		t.Errorf("the metadata is %+v", m)
	}

	signatures := map[string]uint8{
		"XYZ ": 3, "Lab ": 3, "Luv ": 3, "YCbr": 3, "Yxy ": 3, "LMS ": 3, "RGB ": 3, "GRAY": 1,
		"HSV ": 3, "HLS ": 3, "CMYK": 4, "CMY ": 3, "1CLR": 1, "3CLR": 3, "4CLR": 4,
	}
	for sig, n := range signatures {
		cs, ok := iccColorSpaceFromU32(uint32(sig[0])<<24 | uint32(sig[1])<<16 | uint32(sig[2])<<8 | uint32(sig[3]))
		if !ok || cs.numComponents() != n {
			t.Errorf("the signature %q gives %d and %v", sig, cs, ok)
		}
	}

	// The versions of PDF before 1.7 take the profiles of ICC v2.
	for _, tt := range []struct {
		version     PDFVersion
		rgb, grey   uint8
		rgbN, greyN uint8
	}{
		{PDF14, 2, 2, 3, 1},
		{PDF16, 2, 2, 3, 1},
		{PDF17, 4, 4, 3, 1},
		{PDF20, 4, 4, 3, 1},
	} {
		rgb, grey := tt.version.rgbICC(), tt.version.greyICC()
		if rgb.metadata().major != tt.rgb || grey.metadata().major != tt.grey ||
			rgb.components != tt.rgbN || grey.components != tt.greyN {
			t.Errorf("%s takes the profiles %+v and %+v", tt.version, rgb.metadata(), grey.metadata())
		}
	}

	// WithCMYKProfile takes a CMYK profile, and nil in place of another.
	d := NewDocument(WithNoDeviceCS(true), WithCMYKProfile(p), WithPretty(false))
	cs := d.sc.registerColorSpace(d.cc, NewColorCMYK(0, 0, 0, 0).colorSpace(d.sc))
	if cs.kind != maybeDeviceResource {
		t.Fatalf("the CMYK color space is %+v", cs)
	}
	if got := string(d.cc.streams.iccProfiles[0].AsBytes()); !strings.Contains(got, "/N 4/Range[0 1 0 1 0 1 0 1]/Filter/FlateDecode") {
		t.Errorf("the CMYK profile is written as %q", got)
	}
	grey := d.sc.settings.pdfVersion().greyICC()
	d = NewDocument(WithCMYKProfile(grey))
	if d.sc.settings.cmykProfile != nil {
		t.Error("WithCMYKProfile takes a grey profile")
	}
}

// TestPaints checks the paints of krilla src/graphics/paint.rs.
func TestPaints(t *testing.T) {
	f := DefaultFill()
	if f.Paint != ColorLumaBlack() || f.Opacity != 1 || f.Rule != FillRuleNonZero {
		t.Errorf("the default fill is %+v", f)
	}
	s := DefaultStroke()
	if s.Paint != ColorLumaBlack() || s.Width != 1 || s.MiterLimit != 10 || s.Opacity != 1 || s.Dash != nil {
		t.Errorf("the default stroke is %+v", s)
	}

	for _, tt := range []struct {
		lineCap LineCap
		pdf     pdfwriter.LineCapStyle
		path    path.LineCap
	}{
		{LineCapButt, pdfwriter.LineCapStyleButtCap, path.LineCapButt},
		{LineCapRound, pdfwriter.LineCapStyleRoundCap, path.LineCapRound},
		{LineCapSquare, pdfwriter.LineCapStyleProjectingSquareCap, path.LineCapSquare},
	} {
		s := Stroke{LineCap: tt.lineCap}
		if tt.lineCap.toPDFLineCap() != tt.pdf || s.intoTinySkia().LineCap != tt.path {
			t.Errorf("the line cap %d is wrong", tt.lineCap)
		}
	}
	for _, tt := range []struct {
		lineJoin LineJoin
		pdf      pdfwriter.LineJoinStyle
		path     path.LineJoin
	}{
		{LineJoinMiter, pdfwriter.LineJoinStyleMiterJoin, path.LineJoinMiter},
		{LineJoinRound, pdfwriter.LineJoinStyleRoundJoin, path.LineJoinRound},
		{LineJoinBevel, pdfwriter.LineJoinStyleBevelJoin, path.LineJoinBevel},
	} {
		s := Stroke{LineJoin: tt.lineJoin}
		if tt.lineJoin.toPDFLineJoin() != tt.pdf || s.intoTinySkia().LineJoin != tt.path {
			t.Errorf("the line join %d is wrong", tt.lineJoin)
		}
	}

	s.Width, s.MiterLimit = 3, 5
	s.Dash = &StrokeDash{Array: []float32{2, 1}, Offset: 1}
	ts := s.intoTinySkia()
	if ts.Width != 3 || ts.MiterLimit != 5 || ts.Dash == nil {
		t.Errorf("the stroke of path is %+v", ts)
	}
	// A dash that path does not take gives no dash.
	if bad := (Stroke{Dash: &StrokeDash{Array: []float32{-1, 2}}}).intoTinySkia(); bad.Dash != nil {
		t.Error("a negative dash gives a dash")
	}
}

// TestBlendModes checks BlendMode::to_pdf in krilla src/graphics/blend.rs:
// each mode maps to the mode of pdf-writer of the same name.
func TestBlendModes(t *testing.T) {
	names := []string{
		"Normal", "Multiply", "Screen", "Overlay", "Darken", "Lighten", "ColorDodge", "ColorBurn",
		"HardLight", "SoftLight", "Difference", "Exclusion", "Hue", "Saturation", "Color", "Luminosity",
	}
	for i, name := range names {
		if got := BlendMode(i).toPDF().String(); got != name {
			t.Errorf("the blend mode %d is %s, want %s", i, got, name)
		}
	}
}

// TestGraphicsStates checks ExtGState, GraphicsState and GraphicsStates in
// krilla src/graphics/graphics_state.rs.
func TestGraphicsStates(t *testing.T) {
	if !newExtGState().empty() {
		t.Error("a new state is not empty")
	}
	a := newExtGState().strokingAlpha(0.5).blendMode(pdfwriter.BlendModeScreen)
	b := newExtGState().nonStrokingAlpha(0.25).blendMode(pdfwriter.BlendModeHue)
	if a.empty() {
		t.Error("a state with an alpha is empty")
	}
	a.combine(b)
	if a.r.strokingAlpha != 0.5 || a.r.nonStrokingAlpha != 0.25 || a.r.blendMode != pdfwriter.BlendModeHue {
		t.Errorf("the combined state is %+v", a.r)
	}
	if b.r.hasStrokingAlpha {
		t.Error("combine changes the other state")
	}

	gs := newGraphicsStates()
	gs.saveState()
	gs.transform(path.TransformFromTranslate(10, 20))
	gs.combine(b)
	gs.transform(path.TransformFromScale(2, 2))
	r, _ := path.RectFromXYWH(0, 0, 1, 1)
	got, ok := gs.transformBBox(r)
	want, _ := path.RectFromXYWH(10, 20, 2, 2)
	if !ok || got != want || gs.cur().extGState() != b {
		t.Errorf("the box is %v and the state %+v", got, gs.cur().extGState())
	}
	gs.restoreState()
	if gs.cur().transform() != path.IdentityTransform() || !gs.cur().extGState().empty() {
		t.Error("restoreState does not restore the state")
	}
	gs.transform(path.TransformFromRow(float32(math.Inf(1)), 0, 0, 1, 0, 0))
	if _, ok := gs.transformBBox(r); ok {
		t.Error("a transform that is not finite gives a box")
	}

	// A state with a mask writes the mask once, and each state records the
	// error Transparency, which no validator of the base port prohibits.
	d := NewDocument(WithPretty(false))
	mask := NewMask(emptyStream(), MaskTypeLuminosity)
	m1 := newExtGState().mask(mask, d.sc, d.cc)
	m2 := newExtGState().mask(NewMask(emptyStream(), MaskTypeLuminosity), d.sc, d.cc)
	if m1 != m2 || m1.empty() {
		t.Error("the same mask gives two references")
	}
	registerResourceable(d.sc, d.cc, m1.blendMode(pdfwriter.BlendModeNormal).strokingAlpha(1))
	if got := string(d.cc.nonStream.extGStates.AsBytes()); !strings.Contains(got, "/CA 1/BM/Normal/SMask") {
		t.Errorf("the state is written as %q", got)
	}
	if got := string(d.cc.nonStream.masks.AsBytes()); !strings.Contains(got, "/S/Luminosity") {
		t.Errorf("the mask is written as %q", got)
	}
}

// TestXObject checks XObject in krilla src/graphics/xobject.rs.
func TestXObject(t *testing.T) {
	s := emptyStream()
	x := newXObject(s, false, groupColorSpaceNone, nil)
	if !x.isEmpty() || x.bbox() != s.bbox {
		t.Error("the XObject of an empty stream is wrong")
	}
	r, _ := path.RectFromXYWH(1, 2, 3, 4)
	x = newXObject(newStream([]byte("q Q"), s.bbox, nil, s.resourceDictionary, true), false, groupColorSpaceNone, &r)
	r, _ = path.RectFromXYWH(5, 6, 7, 8)
	if x.isEmpty() || x.bbox().Left() != 1 || x.p.value.transparencyGroupColorSpace != groupColorSpaceRGB {
		t.Errorf("the XObject is %+v", x.p.value)
	}

	// A transparency group of device-independent colors takes the sRGB
	// color space.
	d := NewDocument(WithNoDeviceCS(true), WithPretty(false))
	registerResourceable(d.sc, d.cc, newXObject(emptyStream(), true, groupColorSpaceNone, nil))
	if got := string(d.cc.streams.xObjects[0].AsBytes()); !strings.Contains(got, "/Group<</Type/Group/S/Transparency/I true/CS 3 0 R>>") {
		t.Errorf("the group is written as %q", got)
	}
}

// TestGradientProperties checks get_expanded_bbox and get_point_ts in
// krilla src/graphics/shading_function.rs, and the cases in which krilla
// panics.
func TestGradientProperties(t *testing.T) {
	ts, min, max := getPointTs(path.PointFromXY(1, 1), path.PointFromXY(4, 5))
	if min != 1 || max != 6 {
		t.Errorf("the range is %v to %v", min, max)
	}
	p := path.PointFromXY(6, 1)
	ts.MapPoint(&p)
	if math.Abs(float64(p.X-4)) > 1e-5 || math.Abs(float64(p.Y-5)) > 1e-5 {
		t.Errorf("the rotation maps the end to %v", p)
	}

	bbox, _ := path.RectFromLTRB(0, 0, 10, 10)
	got, ok := getExpandedBBox(bbox, path.TransformFromScale(2, 2))
	if !ok || got != bbox {
		t.Errorf("the expanded box of a scale is %v", got)
	}
	got, ok = getExpandedBBox(bbox, path.TransformFromTranslate(5, -5))
	if want, _ := path.RectFromLTRB(-5, 0, 10, 15); !ok || got != want {
		t.Errorf("the expanded box of a translation is %v", got)
	}
	if _, ok := getExpandedBBox(bbox, path.TransformFromScale(0, 0)); ok {
		t.Error("a transform without an inverse expands the box")
	}

	singular := path.TransformFromScale(0, 1)
	stops := []Stop{{Color: NewColorRGB(0, 0, 0), Opacity: 1}, {Offset: 1, Color: NewColorRGB(1, 1, 1), Opacity: 1}}
	if _, _, ok := (LinearGradient{Transform: singular, SpreadMethod: SpreadMethodRepeat, Stops: stops}).gradientProperties(bbox); ok {
		t.Error("a linear gradient without an inverse has properties")
	}
	if _, _, ok := (SweepGradient{Transform: singular, Stops: stops}).gradientProperties(bbox); ok {
		t.Error("a sweep gradient without an inverse has properties")
	}
	gp, tr, ok := LinearGradient{X1: 1, Y1: 2, X2: 3, Y2: 4, Transform: singular, Stops: stops}.gradientProperties(bbox)
	if rag, isRAG := gp.(radialAxialGradient); !ok || !isRAG || tr != singular ||
		len(rag.coords) != 4 || rag.coords[3] != 4 || rag.shadingType != pdfwriter.FunctionShadingTypeAxial {
		t.Errorf("a linear gradient with the spread method pad gives %+v", gp)
	}

	// krilla panics for stops of two color spaces. pdf records
	// ErrPDFMixedColorSpaces, and the shading of the opacities has no such
	// error.
	mixed := []Stop{{Color: NewColorRGB(0, 0, 0), Opacity: 1}, {Offset: 1, Color: NewColorLuma(1), Opacity: 1}}
	gp, _, _ = RadialGradient{Transform: path.IdentityTransform(), Stops: mixed}.gradientProperties(bbox)
	d := NewDocument()
	d.sc.registerCacheable(d.cc, newShadingFunction(gp, true))
	if d.sc.err != nil {
		t.Errorf("the shading of the opacities gives the error %v", d.sc.err)
	}
	d.sc.registerCacheable(d.cc, newShadingFunction(gp, false))
	if err := d.Finish(io.Discard); !errors.Is(err, okage.ErrPDFMixedColorSpaces) {
		t.Errorf("stops of two color spaces give the error %v", err)
	}
}

// TestTrimStops checks trim_stops in krilla src/graphics/shading_function.rs.
func TestTrimStops(t *testing.T) {
	for _, n := range []int{98, 120, 200, 1000} {
		stops := make([]Stop, n)
		for i := range stops {
			stops[i].Offset = NormalizedF32(float32(i) / float32(n))
		}
		trimmed := trimStops(stops)
		if len(trimmed) > maxPostScriptStops+1 || len(trimmed) < maxPostScriptStops-1 || trimmed[0].Offset != 0 {
			t.Errorf("%d stops give %d stops", n, len(trimmed))
		}
	}
}

// TestSeams checks that newMaskFromShading gives no mask for opaque stops,
// and draws the shading of the opacities on a surface for the other stops,
// and that newTilingPattern draws the stream of a pattern with an opacity on
// a surface.
func TestSeams(t *testing.T) {
	d := NewDocument()
	bbox, _ := path.RectFromLTRB(0, 0, 10, 10)
	opaque := radialAxialGradient{stops: []Stop{{Opacity: 1}, {Opacity: 1}}}
	if newMaskFromShading(opaque, path.IdentityTransform(), bbox, d.sc, d.cc) != nil {
		t.Error("opaque stops give a mask")
	}
	stops := []Stop{{Color: NewColorLuma(0), Opacity: 0.5}, {Offset: 1, Color: NewColorLuma(255), Opacity: 1}}
	gp := radialAxialGradient{coords: []float32{0, 0, 1, 0, 0, 5}, shadingType: 3, stops: stops}
	m := newMaskFromShading(gp, path.IdentityTransform(), bbox, d.sc, d.cc)
	if m == nil || m.customBBox == nil || *m.customBBox != bbox || !strings.Contains(string(m.stream.content), " sh\n") {
		t.Errorf("the mask of the shading is %+v", m)
	}
	stream := emptyStream()
	if p := newTilingPattern(stream, path.IdentityTransform(), 1, 2, 3, d.sc, d.cc); p.width != 2 || p.stream != stream {
		t.Errorf("the tiling pattern is %+v", p)
	}
	if p := newTilingPattern(stream, path.IdentityTransform(), 0.5, 1, 1, d.sc, d.cc); p.stream == stream || !strings.Contains(string(p.stream.content), " Do\n") {
		t.Errorf("the tiling pattern with an opacity is %+v", p)
	}
}

// TestMaskType checks MaskType::to_name in krilla src/graphics/mask.rs.
func TestMaskType(t *testing.T) {
	if MaskTypeAlpha.toName() != "Alpha" || MaskTypeLuminosity.toName() != "Luminosity" {
		t.Error("the names of the mask types are wrong")
	}
}
