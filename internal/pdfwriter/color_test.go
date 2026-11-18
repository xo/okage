// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"testing"
)

// src/color.rs of pdf-writer has no tests. The test module compares the
// writers of color.go with pdf-writer (D62). These tests cover the calls
// that the test module cannot make, and run without the oracle.

// TestAlternateDevice writes the alternate space of a separation and of a
// DeviceN as a device color space. DeviceColorSpace is not public in
// pdf-writer, so the oracle cannot call AlternateDevice.
func TestAlternateDevice(t *testing.T) {
	testObj(t, func(obj Obj) {
		s := Start[ColorSpace](obj).Separation(Name("Spot"))
		s.AlternateDevice(DeviceColorSpaceCMYK).TintRef(NewRef(3))
		s.Finish()
	}, "[/Separation /Spot /DeviceCMYK 3 0 R]")

	testObjNoPretty(t, func(obj Obj) {
		d := Start[ColorSpace](obj).DeviceN(Name("A"), Name("B"))
		d.AlternateDevice(DeviceColorSpaceGray).TintRef(NewRef(4))
		d.Attrs().Subtype(DeviceNSubtypeNChannel)
		d.Finish()
	}, "[/DeviceN[/A/B]/DeviceGray 4 0 R<</Subtype/NChannel>>]")

	testObj(t, func(obj Obj) {
		Start[ColorSpace](obj).Separation(Name("S")).AlternateDevice(DeviceColorSpaceRGB).TintExponential().N(1)
	}, "[/Separation /S /DeviceRGB <<", "  /FunctionType 2", "  /N 1", ">>]")

	mustPanic(t, "alternate space already specified", func() {
		testObj(t, func(obj Obj) {
			Start[ColorSpace](obj).Separation(Name("S")).AlternateDevice(DeviceColorSpaceRGB).AlternateDevice(DeviceColorSpaceRGB)
		})
	})
	mustPanic(t, "alternate space already specified", func() {
		testObj(t, func(obj Obj) {
			Start[ColorSpace](obj).DeviceN().AlternateColorSpaceRef(NewRef(1)).AlternateDevice(DeviceColorSpaceRGB)
		})
	})
}

// TestCalibratedColorSpaces writes the calibrated color spaces.
func TestCalibratedColorSpaces(t *testing.T) {
	testObjNoPretty(t, func(obj Obj) { Start[ColorSpace](obj).SRGB() },
		"[/CalRGB<</WhitePoint[0.9505 1 1.0888]/Gamma[2.2 2.2 2.2]"+
			"/Matrix[0.4124 0.2126 0.0193 0.3576 0.715 0.1192 0.1805 0.0722 0.9505]>>]")
	testObjNoPretty(t, func(obj Obj) { Start[ColorSpace](obj).D50Gray(nil) },
		"[/CalGray<</WhitePoint[0.9642 1 0.82489]>>]")
	testObj(t, func(obj Obj) { Start[ColorSpace](obj).DeviceCMYK() }, "/DeviceCMYK")
	testObj(t, func(obj Obj) { Start[ColorSpace](obj).Indexed(Name("DeviceRGB"), 1, []byte("\xff\x00\x00abc")) },
		"[/Indexed /DeviceRGB 1 <FF0000616263>]")
}

// TestPatterns writes a tiling pattern and a shading pattern.
func TestPatterns(t *testing.T) {
	w := NewChunkWithSettings(Settings{Pretty: false})
	w.TilingPattern(NewRef(1), []byte("q Q")).
		TilingType(TilingTypeConstantSpacing).
		PaintType(PaintTypeColored).
		BBox(NewRect(0, 0, 10, 20)).
		Matrix([6]float32{1, 0, 0, 1, 0, 0}).
		XStep(10).
		YStep(20)
	p := w.ShadingPattern(NewRef(2))
	p.FunctionShading().
		ShadingType(FunctionShadingTypeAxial).
		Coords(0, 0, 1, 0).
		Function(NewRef(3)).
		Extend([2]bool{true, true}).
		ColorSpace().DeviceRGB()
	p.Matrix([6]float32{2, 0, 0, 2, 0, 0})
	testBytes(t, w.AsBytes(),
		"1 0 obj",
		"<</Length 3/Type/Pattern/PatternType 1/TilingType 1/PaintType 1/BBox[0 0 10 20]/Matrix[1 0 0 1 0 0]/XStep 10/YStep 20>>",
		"stream",
		"q Q",
		"endstream",
		"endobj",
		"2 0 obj",
		"<</Type/Pattern/PatternType 2/Shading<</ShadingType 2/Coords[0 0 1 0]/Function 3 0 R/Extend[true true]/ColorSpace/DeviceRGB>>/Matrix[2 0 0 2 0 0]>>",
		"endobj",
		"")

	mustPanic(t, "x step must not be zero", func() { NewChunk().TilingPattern(NewRef(1), nil).XStep(0) })
	mustPanic(t, "n must be 1, 3, or 4, but is 2", func() { NewChunk().ICCProfile(NewRef(1), nil).N(2) })
}
