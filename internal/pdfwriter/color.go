// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import "fmt"

// This file ports pdf-writer src/color.rs, with the methods of Chunk for
// color spaces, shadings and patterns of src/chunk.rs.

// cieD65 is CIE_D65 in pdf-writer src/color.rs.
//
// CIE XYZ coordinates of the D65 noon daylight white.
var cieD65 = [3]float32{0.9505, 1.0, 1.0888}

// cieD50 is CIE_D50 in pdf-writer src/color.rs.
//
// CIE XYZ coordinates of the D50 horizon light white.
var cieD50 = [3]float32{0.9642, 1.0, 0.82489}

// cieE is CIE_E in pdf-writer src/color.rs.
//
// CIE XYZ coordinates of the E equal radiator white.
var cieE = [3]float32{1.000, 1.000, 1.000}

// cieC is CIE_C in pdf-writer src/color.rs.
//
// CIE XYZ coordinates of the C north sky daylight white.
var cieC = [3]float32{0.9807, 1.0000, 1.1822}

// colorSpaceType is ColorSpaceType in pdf-writer src/color.rs.
//
// The type of a color space.
type colorSpaceType uint8

// The values of colorSpaceType.
const (
	colorSpaceTypeCalGray colorSpaceType = iota
	colorSpaceTypeCalRGB
	colorSpaceTypeLab
	colorSpaceTypeICCBased
	colorSpaceTypeDeviceRGB
	colorSpaceTypeDeviceCMYK
	colorSpaceTypeDeviceGray
	colorSpaceTypeIndexed
	colorSpaceTypePattern
	colorSpaceTypeSeparation
	colorSpaceTypeDeviceN
)

// toName is ColorSpaceType::to_name in pdf-writer src/color.rs.
func (t colorSpaceType) toName() Name {
	switch t {
	case colorSpaceTypeCalRGB:
		return Name("CalRGB")
	case colorSpaceTypeCalGray:
		return Name("CalGray")
	case colorSpaceTypeLab:
		return Name("Lab")
	case colorSpaceTypeICCBased:
		return Name("ICCBased")
	case colorSpaceTypeDeviceRGB:
		return Name("DeviceRGB")
	case colorSpaceTypeDeviceCMYK:
		return Name("DeviceCMYK")
	case colorSpaceTypeDeviceGray:
		return Name("DeviceGray")
	case colorSpaceTypeSeparation:
		return Name("Separation")
	case colorSpaceTypeDeviceN:
		return Name("DeviceN")
	case colorSpaceTypeIndexed:
		return Name("Indexed")
	case colorSpaceTypePattern:
		return Name("Pattern")
	}
	panic(fmt.Sprintf("pdfwriter: the color space type %d is not known", uint8(t)))
}

// DeviceColorSpace is DeviceColorSpace in pdf-writer src/color.rs.
//
// The type of a device color space.
type DeviceColorSpace uint8

// The values of DeviceColorSpace.
const (
	// DeviceColorSpaceRGB is red, green and blue.
	DeviceColorSpaceRGB DeviceColorSpace = iota
	// DeviceColorSpaceCMYK is cyan, magenta, yellow and black.
	DeviceColorSpaceCMYK
	// DeviceColorSpaceGray is gray.
	DeviceColorSpaceGray
)

// String returns the PDF name of the color space, without the slash.
func (s DeviceColorSpace) String() string {
	return string(s.toName())
}

// toName is DeviceColorSpace::to_name in pdf-writer src/color.rs.
func (s DeviceColorSpace) toName() Name {
	switch s {
	case DeviceColorSpaceRGB:
		return Name("DeviceRGB")
	case DeviceColorSpaceCMYK:
		return Name("DeviceCMYK")
	case DeviceColorSpaceGray:
		return Name("DeviceGray")
	}
	panic(fmt.Sprintf("pdfwriter: the device color space %d is not known", uint8(s)))
}

// ColorSpace is ColorSpace in pdf-writer src/color.rs. Each method writes
// the color space and ends the writer, as the methods of upstream take self.
//
// Writer for a color space.
//
// This struct is created by Chunk.ColorSpace, ImageXObject.ColorSpace,
// Separation.AlternateColorSpace and Group.ColorSpace.
type ColorSpace struct {
	obj Obj
}

// start is the macro writer! for ColorSpace in pdf-writer src/color.rs.
func (cs *ColorSpace) start(obj Obj) {
	cs.obj = obj
}

// CIE-based color spaces.

// CalRGB is ColorSpace::cal_rgb in pdf-writer src/color.rs. A nil pointer
// is the None of upstream.
//
// Write a CalRGB color space.
func (cs *ColorSpace) CalRGB(whitePoint [3]float32, blackPoint, gamma *[3]float32, matrix *[9]float32) {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeCalRGB.toName())

	dict := array.Push().Dict()
	itemsF32(dict.Insert(Name("WhitePoint")).Array(), whitePoint[:])

	if blackPoint != nil {
		itemsF32(dict.Insert(Name("BlackPoint")).Array(), blackPoint[:])
	}

	if gamma != nil {
		itemsF32(dict.Insert(Name("Gamma")).Array(), gamma[:])
	}

	if matrix != nil {
		itemsF32(dict.Insert(Name("Matrix")).Array(), matrix[:])
	}
	array.Finish()
}

// CalGray is ColorSpace::cal_gray in pdf-writer src/color.rs. A nil pointer
// is the None of upstream.
//
// Write a CalGray color space.
func (cs *ColorSpace) CalGray(whitePoint [3]float32, blackPoint *[3]float32, gamma *float32) {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeCalGray.toName())

	dict := array.Push().Dict()
	itemsF32(dict.Insert(Name("WhitePoint")).Array(), whitePoint[:])

	if blackPoint != nil {
		itemsF32(dict.Insert(Name("BlackPoint")).Array(), blackPoint[:])
	}

	if gamma != nil {
		dict.Pair(Name("Gamma"), Real(*gamma))
	}
	array.Finish()
}

// Lab is ColorSpace::lab in pdf-writer src/color.rs. A nil pointer is the
// None of upstream.
//
// Write a Lab color space.
func (cs *ColorSpace) Lab(whitePoint [3]float32, blackPoint *[3]float32, rng *[4]float32) {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeLab.toName())

	dict := array.Push().Dict()
	itemsF32(dict.Insert(Name("WhitePoint")).Array(), whitePoint[:])

	if blackPoint != nil {
		itemsF32(dict.Insert(Name("BlackPoint")).Array(), blackPoint[:])
	}

	if rng != nil {
		itemsF32(dict.Insert(Name("Range")).Array(), rng[:])
	}
	array.Finish()
}

// ICCBased is ColorSpace::icc_based in pdf-writer src/color.rs.
//
// Write an ICCBased color space.
//
// The stream argument should be an indirect reference to an ICC profile
// stream.
func (cs *ColorSpace) ICCBased(stream Ref) {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeICCBased.toName())
	array.Item(stream)
	array.Finish()
}

// ICCProfile is IccProfile in pdf-writer src/color.rs.
//
// Writer for an ICC profile stream.
//
// This struct is created by Chunk.ICCProfile.
type ICCProfile struct {
	*Stream
}

// startICCProfile is IccProfile::start in pdf-writer src/color.rs.
//
// Create a new ICC profile stream writer.
func startICCProfile(stream *Stream) *ICCProfile {
	return &ICCProfile{Stream: stream}
}

// N is IccProfile::n in pdf-writer src/color.rs.
//
// Write the /N attribute. Required.
//
// The number of components in the color space. Shall be 1, 3, or 4.
func (p *ICCProfile) N(n int32) *ICCProfile {
	if n != 1 && n != 3 && n != 4 {
		panic(fmt.Sprintf("n must be 1, 3, or 4, but is %d", n))
	}
	p.Pair(Name("N"), Int(n))
	return p
}

// Alternate is IccProfile::alternate in pdf-writer src/color.rs.
//
// Write the /Alternate attribute with a color space.
//
// The alternate color space to use when the ICC profile is not supported.
// Must be a color space with the same number of components as the ICC
// profile. Pattern color spaces are not allowed.
func (p *ICCProfile) Alternate() *ColorSpace {
	return Start[ColorSpace](p.Insert(Name("Alternate")))
}

// AlternateName is IccProfile::alternate_name in pdf-writer src/color.rs.
//
// Write the /Alternate attribute with a name.
//
// The alternate color space referenced by name must be registered in the
// current resource dictionary.
func (p *ICCProfile) AlternateName(name Name) *ICCProfile {
	p.Pair(Name("Alternate"), name)
	return p
}

// Range is IccProfile::range in pdf-writer src/color.rs.
//
// Write the /Range attribute.
//
// Specifies the permissible range of values for each component. The array
// shall contain 2 × N numbers, where N is the number of components in the
// color space. The array is organized in pairs, where the first value shall
// be the minimum value and the second shall be the maximum value.
func (p *ICCProfile) Range(rng ...float32) *ICCProfile {
	itemsF32(p.Insert(Name("Range")).Array(), rng)
	return p
}

// Metadata is IccProfile::metadata in pdf-writer src/color.rs.
//
// Write the /Metadata attribute.
//
// A reference to a stream containing metadata for the ICC profile.
func (p *ICCProfile) Metadata(metadata Ref) *ICCProfile {
	p.Pair(Name("Metadata"), metadata)
	return p
}

// Common calibrated color spaces.

// SRGB is ColorSpace::srgb in pdf-writer src/color.rs.
//
// Write a CalRGB color space approximating sRGB.
//
// Use an ICC profile for more accurate results.
func (cs *ColorSpace) SRGB() {
	cs.CalRGB(
		cieD65,
		nil,
		&[3]float32{2.2, 2.2, 2.2},
		&[9]float32{0.4124, 0.2126, 0.0193, 0.3576, 0.715, 0.1192, 0.1805, 0.0722, 0.9505},
	)
}

// AdobeRGB is ColorSpace::adobe_rgb in pdf-writer src/color.rs.
//
// Write a CalRGB color space approximating Adobe RGB.
//
// Use an ICC profile for more accurate results.
func (cs *ColorSpace) AdobeRGB() {
	cs.CalRGB(
		cieD65,
		nil,
		&[3]float32{2.1992188, 2.1992188, 2.1992188},
		&[9]float32{
			0.57667, 0.29734, 0.02703, 0.18556, 0.62736, 0.07069, 0.18823, 0.07529,
			0.99134,
		},
	)
}

// DisplayP3 is ColorSpace::display_p3 in pdf-writer src/color.rs.
//
// Write a CalRGB color space approximating Display P3.
//
// Use an ICC profile for more accurate results.
func (cs *ColorSpace) DisplayP3() {
	cs.CalRGB(
		cieD65,
		nil,
		&[3]float32{2.6, 2.6, 2.6},
		&[9]float32{
			0.48657, 0.2297, 0.0, 0.26567, 0.69174, 0.04511, 0.19822, 0.07929,
			1.04394,
		},
	)
}

// ProPhoto is ColorSpace::pro_photo in pdf-writer src/color.rs.
//
// Write a CalRGB color space approximating ProPhoto.
//
// Use an ICC profile for more accurate results.
func (cs *ColorSpace) ProPhoto() {
	cs.CalRGB(
		cieD50,
		nil,
		&[3]float32{1.8, 1.8, 1.8},
		&[9]float32{
			0.7976749, 0.2880402, 0.0, 0.1351917, 0.7118741, 0.0, 0.0313534,
			0.0000857, 0.82521,
		},
	)
}

// ECIRGB is ColorSpace::eci_rgb in pdf-writer src/color.rs.
//
// Write a CalRGB color space for ECI RGB v1.
func (cs *ColorSpace) ECIRGB() {
	cs.CalRGB(
		cieD50,
		nil,
		&[3]float32{1.8, 1.8, 1.8},
		&[9]float32{
			0.6502043, 0.3202499, 0.0, 0.1780774, 0.6020711, 0.0678390, 0.1359384,
			0.0776791, 0.757371,
		},
	)
}

// NTSC is ColorSpace::ntsc in pdf-writer src/color.rs.
//
// Write a CalRGB color space for NTSC RGB.
func (cs *ColorSpace) NTSC() {
	cs.CalRGB(
		cieC,
		nil,
		&[3]float32{2.2, 2.2, 2.2},
		&[9]float32{
			0.6068909, 0.2989164, 0.0, 0.1735011, 0.586599, 0.0660957, 0.200348,
			0.1144845, 1.1162243,
		},
	)
}

// PAL is ColorSpace::pal in pdf-writer src/color.rs.
//
// Write a CalRGB color space for PAL/SECAM RGB.
func (cs *ColorSpace) PAL() {
	cs.CalRGB(
		cieD65,
		nil,
		&[3]float32{2.2, 2.2, 2.2},
		&[9]float32{
			0.430619, 0.2220379, 0.0201853, 0.3415419, 0.7066384, 0.1295504,
			0.1783091, 0.0713236, 0.9390944,
		},
	)
}

// D65Gray is ColorSpace::d65_gray in pdf-writer src/color.rs.
//
// Write a CalGray color space for CIE D65 at a 2.2 gamma, equivalent to
// sRGB, Adobe RGB, Display P3, PAL, ...
func (cs *ColorSpace) D65Gray() {
	gamma := float32(2.2)
	cs.CalGray(cieD65, nil, &gamma)
}

// D50Gray is ColorSpace::d50_gray in pdf-writer src/color.rs. A nil
// pointer is the None of upstream.
//
// Write a CalGray color space for CIE D50 (horizon light). Set a 1.8 gamma
// for ProPhoto or ECI RGB equivalency, 2.2 is another common value.
func (cs *ColorSpace) D50Gray(gamma *float32) {
	cs.CalGray(cieD50, nil, gamma)
}

// CGray is ColorSpace::c_gray in pdf-writer src/color.rs.
//
// Write a CalGray color space for CIE C (north sky daylight) at 2.2 gamma,
// equivalent to NTSC.
func (cs *ColorSpace) CGray() {
	gamma := float32(2.2)
	cs.CalGray(cieC, nil, &gamma)
}

// EGray is ColorSpace::e_gray in pdf-writer src/color.rs. A nil pointer is
// the None of upstream.
//
// Write a CalGray color space for CIE E (equal emission). Common gamma
// values include 1.8 or 2.2.
func (cs *ColorSpace) EGray(gamma *float32) {
	cs.CalGray(cieE, nil, gamma)
}

// Device color spaces.
//
// Please note that the use of the device color spaces is restricted by
// several PDF standards such as PDF/A, PDF/X, et cetera. Their appearance
// will be governed by any applicable output intent and default color spaces.

// DeviceRGB is ColorSpace::device_rgb in pdf-writer src/color.rs.
//
// Write a DeviceRGB color space.
func (cs *ColorSpace) DeviceRGB() {
	cs.obj.Primitive(colorSpaceTypeDeviceRGB.toName())
}

// DeviceCMYK is ColorSpace::device_cmyk in pdf-writer src/color.rs.
//
// Write a DeviceCMYK color space.
func (cs *ColorSpace) DeviceCMYK() {
	cs.obj.Primitive(colorSpaceTypeDeviceCMYK.toName())
}

// DeviceGray is ColorSpace::device_gray in pdf-writer src/color.rs.
//
// Write a DeviceGray color space.
func (cs *ColorSpace) DeviceGray() {
	cs.obj.Primitive(colorSpaceTypeDeviceGray.toName())
}

// Special color spaces.

// Separation is ColorSpace::separation in pdf-writer src/color.rs.
//
// Start writing a Separation color space. PDF 1.2+.
//
// The colorName argument is the name of the colorant that will be used by
// the printer.
func (cs *ColorSpace) Separation(colorName Name) *Separation {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeSeparation.toName())
	array.Item(colorName)
	return startSeparation(array)
}

// DeviceN is ColorSpace::device_n in pdf-writer src/color.rs.
//
// Write a DeviceN color space. PDF 1.3+.
//
// The names argument contains the N names of the colorants for the
// respective components.
func (cs *ColorSpace) DeviceN(names ...Name) *DeviceN {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeDeviceN.toName())
	NewTypedArray[Name](array.Push().Array()).Items(names...).Finish()
	return startDeviceN(array)
}

// Indexed is ColorSpace::indexed in pdf-writer src/color.rs.
//
// Write an Indexed color space. PDF 1.2+.
//
// The length of the lookup slice must be the product of the dimensions of
// the base color space and (hival + 1) and hival shall be at most 255.
func (cs *ColorSpace) Indexed(base Name, hival int32, lookup []byte) {
	array := cs.obj.Array()
	array.Item(colorSpaceTypeIndexed.toName())
	array.Item(base)
	array.Item(Int(hival))
	array.Item(Str(lookup))
	array.Finish()
}

// Pattern is ColorSpace::pattern in pdf-writer src/color.rs.
//
// Write a Pattern color space for uncolored patterns. PDF 1.2+.
//
// The base attribute is the color space in which the pattern's tint color is
// specified upon use.
func (cs *ColorSpace) Pattern(base Name) {
	array := cs.obj.Array()
	array.Item(colorSpaceTypePattern.toName())
	array.Item(base)
	array.Finish()
}

// patternType is PatternType in pdf-writer src/color.rs.
//
// Type of pattern.
type patternType uint8

// The values of patternType.
const (
	// patternTypeTiling is a tiling pattern.
	patternTypeTiling patternType = iota
	// patternTypeShading is a shading pattern.
	patternTypeShading
)

// toInt is PatternType::to_int in pdf-writer src/color.rs.
func (t patternType) toInt() int32 {
	switch t {
	case patternTypeTiling:
		return 1
	case patternTypeShading:
		return 2
	}
	panic(fmt.Sprintf("pdfwriter: the pattern type %d is not known", uint8(t)))
}

// Separation is Separation in pdf-writer src/color.rs.
//
// Writer for a separation dictionary. PDF 1.2+.
//
// First, one of the Alternate... methods must be called to specify the
// alternate color space. Then, one of the Tint... methods must be called to
// specify the tint transform method. If the tint transform method is called
// before the alternate color space, the function panics. If multiple
// alternate color space functions are called, the function panics.
//
// This struct is created by ColorSpace.Separation.
type Separation struct {
	array        *Array
	hasAlternate bool
}

// startSeparation is Separation::start in pdf-writer src/color.rs.
//
// Start the wrapper.
func startSeparation(array *Array) *Separation {
	return &Separation{array: array, hasAlternate: false}
}

// AlternateDevice is Separation::alternate_device in pdf-writer
// src/color.rs.
//
// Write the alternateSpace element as a device color space.
func (s *Separation) AlternateDevice(deviceSpace DeviceColorSpace) *Separation {
	if s.hasAlternate {
		panic("alternate space already specified")
	}
	s.array.Item(deviceSpace.toName())
	s.hasAlternate = true
	return s
}

// AlternateColorSpace is Separation::alternate_color_space in pdf-writer
// src/color.rs.
//
// Start writing the alternateSpace element as a color space array. The
// color space must not be another Pattern, Separation, or DeviceN color
// space.
func (s *Separation) AlternateColorSpace() *ColorSpace {
	if s.hasAlternate {
		panic("alternate space already specified")
	}
	s.hasAlternate = true
	return Start[ColorSpace](s.array.Push())
}

// AlternateColorSpaceRef is Separation::alternate_color_space_ref in
// pdf-writer src/color.rs.
//
// Write the alternateSpace element as an indirect reference. The color space
// must not be another Pattern, Separation, or DeviceN color space.
func (s *Separation) AlternateColorSpaceRef(id Ref) *Separation {
	if s.hasAlternate {
		panic("alternate space already specified")
	}
	s.array.Item(id)
	s.hasAlternate = true
	return s
}

// TintExponential is Separation::tint_exponential in pdf-writer
// src/color.rs.
//
// Start writing the tintTransform element as an exponential interpolation
// function.
func (s *Separation) TintExponential() *ExponentialFunction {
	if !s.hasAlternate {
		panic("alternate space must be specified before tint transform")
	}
	return Start[ExponentialFunction](s.array.Push())
}

// TintStitching is Separation::tint_stitching in pdf-writer src/color.rs.
//
// Start writing the tintTransform element as a stitching function.
func (s *Separation) TintStitching() *StitchingFunction {
	if !s.hasAlternate {
		panic("alternate space must be specified before tint transform")
	}
	return Start[StitchingFunction](s.array.Push())
}

// TintRef is Separation::tint_ref in pdf-writer src/color.rs.
//
// Write the tintTransform element as an indirect reference to a function.
// The function must take a single number as input and produce a color in
// the alternate color space as output. This must be used if a stream
// function like SampledFunction or PostScriptFunction is used.
func (s *Separation) TintRef(id Ref) *Separation {
	if !s.hasAlternate {
		panic("alternate space must be specified before tint transform")
	}
	s.array.Item(id)
	return s
}

// Finish ends the color space array, as Rust does when it drops the
// Separation.
func (s *Separation) Finish() {
	s.array.Finish()
}

// DeviceN is DeviceN in pdf-writer src/color.rs.
//
// Writer for a DeviceN color space array with attributes. PDF 1.6+.
//
// First, one of the Alternate... methods must be called to specify the
// alternate color space. Then, one of the Tint... methods must be called to
// specify the tint transform method. Finally, the Attrs function may
// optionally be called. If any function is called out of order, the
// function panics.
//
// This struct is created by ColorSpace.DeviceN.
type DeviceN struct {
	array        *Array
	hasAlternate bool
	hasTint      bool
}

// startDeviceN is DeviceN::start in pdf-writer src/color.rs.
//
// Start the wrapper.
func startDeviceN(array *Array) *DeviceN {
	return &DeviceN{array: array, hasAlternate: false, hasTint: false}
}

// AlternateDevice is DeviceN::alternate_device in pdf-writer src/color.rs.
//
// Write the alternateSpace element as a device color space.
func (d *DeviceN) AlternateDevice(deviceSpace DeviceColorSpace) *DeviceN {
	if d.hasAlternate {
		panic("alternate space already specified")
	}
	d.array.Item(deviceSpace.toName())
	d.hasAlternate = true
	return d
}

// AlternateColorSpace is DeviceN::alternate_color_space in pdf-writer
// src/color.rs.
//
// Start writing the alternateSpace element as a color space array. The
// color space must not be another Pattern, Separation, or DeviceN color
// space.
func (d *DeviceN) AlternateColorSpace() *ColorSpace {
	if d.hasAlternate {
		panic("alternate space already specified")
	}
	d.hasAlternate = true
	return Start[ColorSpace](d.array.Push())
}

// AlternateColorSpaceRef is DeviceN::alternate_color_space_ref in
// pdf-writer src/color.rs.
//
// Write the alternateSpace element as an indirect reference. The color space
// must not be another Pattern, Separation, or DeviceN color space.
func (d *DeviceN) AlternateColorSpaceRef(id Ref) *DeviceN {
	if d.hasAlternate {
		panic("alternate space already specified")
	}
	d.array.Item(id)
	d.hasAlternate = true
	return d
}

// TintExponential is DeviceN::tint_exponential in pdf-writer src/color.rs.
//
// Start writing the tintTransform element as an exponential interpolation
// function.
func (d *DeviceN) TintExponential() *ExponentialFunction {
	if !d.hasAlternate {
		panic("alternate space must be specified before tint transform")
	} else if d.hasTint {
		panic("tint transform already specified")
	}

	d.hasTint = true
	return Start[ExponentialFunction](d.array.Push())
}

// TintStitching is DeviceN::tint_stitching in pdf-writer src/color.rs.
//
// Start writing the tintTransform element as a stitching function.
func (d *DeviceN) TintStitching() *StitchingFunction {
	if !d.hasAlternate {
		panic("alternate space must be specified before tint transform")
	} else if d.hasTint {
		panic("tint transform already specified")
	}

	d.hasTint = true
	return Start[StitchingFunction](d.array.Push())
}

// TintRef is DeviceN::tint_ref in pdf-writer src/color.rs.
//
// Write the tintTransform element as an indirect reference to a function.
// The function must take n numbers as input and produce a color in the
// alternate color space as output. This must be used if a stream function
// like SampledFunction or PostScriptFunction is used.
func (d *DeviceN) TintRef(id Ref) *DeviceN {
	if !d.hasAlternate {
		panic("alternate space must be specified before tint transform")
	} else if d.hasTint {
		panic("tint transform already specified")
	}

	d.array.Item(id)
	d.hasTint = true
	return d
}

// Attrs is DeviceN::attrs in pdf-writer src/color.rs.
//
// Start writing the attrs dictionary. PDF 1.6+.
func (d *DeviceN) Attrs() *DeviceNAttrs {
	if !d.hasAlternate {
		panic(
			"alternate space and tint transform must be specified before attributes",
		)
	} else if !d.hasTint {
		panic("tint transform must be specified before attributes")
	}

	return Start[DeviceNAttrs](d.array.Push())
}

// Finish ends the color space array, as Rust does when it drops the
// DeviceN.
func (d *DeviceN) Finish() {
	d.array.Finish()
}

// DeviceNAttrs is DeviceNAttrs in pdf-writer src/color.rs.
//
// Writer for a DeviceN attributes dictionary. PDF 1.6+.
//
// This struct is created by DeviceN.Attrs.
type DeviceNAttrs struct {
	dict *Dict
}

// start is the macro writer! for DeviceNAttrs in pdf-writer src/color.rs.
func (a *DeviceNAttrs) start(obj Obj) {
	a.dict = obj.Dict()
}

// Subtype is DeviceNAttrs::subtype in pdf-writer src/color.rs.
//
// Write the /Subtype attribute.
func (a *DeviceNAttrs) Subtype(subtype DeviceNSubtype) *DeviceNAttrs {
	a.dict.Pair(Name("Subtype"), subtype.toName())
	return a
}

// Colorants is DeviceNAttrs::colorants in pdf-writer src/color.rs.
//
// Start writing the /Colorants dictionary. Its keys are the colorant names
// and its values are separation color space arrays.
//
// Required if the /Subtype attribute is NChannel. Required for spot colors
// in PDF/A-2, PDF/A-3, and PDF/A-4.
func (a *DeviceNAttrs) Colorants() *WriterDict[Dict, *Dict] {
	return NewWriterDict[Dict](a.dict.Insert(Name("Colorants")).Dict())
}

// Process is DeviceNAttrs::process in pdf-writer src/color.rs.
//
// Start writing the /Process dictionary.
//
// Required if the /Subtype attribute is Separation.
func (a *DeviceNAttrs) Process() *DeviceNProcess {
	return Start[DeviceNProcess](a.dict.Insert(Name("Process")))
}

// MixingHints is DeviceNAttrs::mixing_hints in pdf-writer src/color.rs.
//
// Start writing the /MixingHints dictionary.
func (a *DeviceNAttrs) MixingHints() *DeviceNMixingHints {
	return Start[DeviceNMixingHints](a.dict.Insert(Name("MixingHints")))
}

// Finish ends the dictionary, as Rust does when it drops the writer.
func (a *DeviceNAttrs) Finish() {
	a.dict.Finish()
}

// DeviceNProcess is DeviceNProcess in pdf-writer src/color.rs.
//
// Writer for a DeviceN process dictionary. PDF 1.6+.
//
// This struct is created by DeviceNAttrs.Process.
type DeviceNProcess struct {
	dict *Dict
}

// start is the macro writer! for DeviceNProcess in pdf-writer src/color.rs.
func (p *DeviceNProcess) start(obj Obj) {
	p.dict = obj.Dict()
}

// ColorSpace is DeviceNProcess::color_space in pdf-writer src/color.rs.
//
// Write the /ColorSpace attribute with a name. Required.
func (p *DeviceNProcess) ColorSpace(colorSpace Name) *DeviceNProcess {
	p.dict.Pair(Name("ColorSpace"), colorSpace)
	return p
}

// ColorSpaceArray is DeviceNProcess::color_space_array in pdf-writer
// src/color.rs.
//
// Write the /ColorSpace attribute with an array. Required.
func (p *DeviceNProcess) ColorSpaceArray() *ColorSpace {
	return Start[ColorSpace](p.dict.Insert(Name("ColorSpace")))
}

// Components is DeviceNProcess::components in pdf-writer src/color.rs.
//
// Write the /Components attribute. Required.
//
// Contains the names of the colorants in the order in which they appear in
// the color space array.
func (p *DeviceNProcess) Components(components ...Name) *DeviceNProcess {
	NewTypedArray[Name](p.dict.Insert(Name("Components")).Array()).Items(components...)
	return p
}

// Finish ends the dictionary, as Rust does when it drops the writer.
func (p *DeviceNProcess) Finish() {
	p.dict.Finish()
}

// DeviceNSubtype is DeviceNSubtype in pdf-writer src/color.rs.
//
// Type of n-dimensional color space.
type DeviceNSubtype uint8

// The values of DeviceNSubtype.
const (
	// DeviceNSubtypeDeviceN is a subtractive color space.
	DeviceNSubtypeDeviceN DeviceNSubtype = iota
	// DeviceNSubtypeNChannel is an additive color space.
	DeviceNSubtypeNChannel
)

// String returns the PDF name of the subtype, without the slash.
func (s DeviceNSubtype) String() string {
	return string(s.toName())
}

// toName is DeviceNSubtype::to_name in pdf-writer src/color.rs.
func (s DeviceNSubtype) toName() Name {
	switch s {
	case DeviceNSubtypeDeviceN:
		return Name("DeviceN")
	case DeviceNSubtypeNChannel:
		return Name("NChannel")
	}
	panic(fmt.Sprintf("pdfwriter: the DeviceN subtype %d is not known", uint8(s)))
}

// DeviceNMixingHints is DeviceNMixingHints in pdf-writer src/color.rs.
//
// Writer for a DeviceN mixing hints dictionary. PDF 1.6+.
//
// This struct is created by DeviceNAttrs.MixingHints.
type DeviceNMixingHints struct {
	dict *Dict
}

// start is the macro writer! for DeviceNMixingHints in pdf-writer
// src/color.rs.
func (h *DeviceNMixingHints) start(obj Obj) {
	h.dict = obj.Dict()
}

// Solidities is DeviceNMixingHints::solidities in pdf-writer src/color.rs.
//
// Start writing the /Solidities dictionary.
//
// Each key in the dictionary is a colorant name and each value is a number
// between 0 and 1 indicating the relative solidity of the colorant.
func (h *DeviceNMixingHints) Solidities() *TypedDict[Real] {
	return NewTypedDict[Real](h.dict.Insert(Name("Solidities")).Dict())
}

// PrintingOrder is DeviceNMixingHints::printing_order in pdf-writer
// src/color.rs.
//
// Write the /PrintingOrder attribute.
//
// Required if /Solidities is present. An array of colorant names in the
// order in which they should be printed.
func (h *DeviceNMixingHints) PrintingOrder(order ...Name) *DeviceNMixingHints {
	NewTypedArray[Name](h.dict.Insert(Name("PrintingOrder")).Array()).Items(order...)
	return h
}

// DotGain is DeviceNMixingHints::dot_gain in pdf-writer src/color.rs.
//
// Start writing the /DotGain dictionary.
//
// Each key in the dictionary is a colorant name and each value is a number
// between 0 and 1 indicating the dot gain of the colorant.
func (h *DeviceNMixingHints) DotGain() *TypedDict[Real] {
	return NewTypedDict[Real](h.dict.Insert(Name("DotGain")).Dict())
}

// Finish ends the dictionary, as Rust does when it drops the writer.
func (h *DeviceNMixingHints) Finish() {
	h.dict.Finish()
}

// TilingPattern is TilingPattern in pdf-writer src/color.rs.
//
// Writer for a tiling pattern stream.
//
// This struct is created by Chunk.TilingPattern.
type TilingPattern struct {
	*Stream
}

// startTilingPattern is TilingPattern::start_with_stream in pdf-writer
// src/color.rs.
func startTilingPattern(stream *Stream) *TilingPattern {
	stream.Pair(Name("Type"), Name("Pattern"))
	stream.Pair(Name("PatternType"), Int(patternTypeTiling.toInt()))
	return &TilingPattern{Stream: stream}
}

// PaintType is TilingPattern::paint_type in pdf-writer src/color.rs.
//
// Write the /PaintType attribute.
//
// Sets whether to use external or stream color. Required.
func (p *TilingPattern) PaintType(paintType PaintType) *TilingPattern {
	p.Pair(Name("PaintType"), Int(paintType.toInt()))
	return p
}

// TilingType is TilingPattern::tiling_type in pdf-writer src/color.rs.
//
// Write the /TilingType attribute.
//
// Sets how to stretch and space the pattern. Required.
func (p *TilingPattern) TilingType(tilingType TilingType) *TilingPattern {
	p.Pair(Name("TilingType"), Int(tilingType.toInt()))
	return p
}

// BBox is TilingPattern::bbox in pdf-writer src/color.rs.
//
// Write the /BBox attribute.
//
// Sets the bounding box of the pattern in the pattern's coordinate system.
// Required.
func (p *TilingPattern) BBox(bbox Rect) *TilingPattern {
	p.Pair(Name("BBox"), bbox)
	return p
}

// XStep is TilingPattern::x_step in pdf-writer src/color.rs.
//
// Write the /XStep attribute.
//
// Sets the horizontal spacing between pattern cells. Required.
//
// Panics if xStep is zero.
func (p *TilingPattern) XStep(xStep float32) *TilingPattern {
	if xStep == 0.0 {
		panic("x step must not be zero")
	}
	p.Pair(Name("XStep"), Real(xStep))
	return p
}

// YStep is TilingPattern::y_step in pdf-writer src/color.rs.
//
// Write the /YStep attribute.
//
// Sets the vertical spacing between pattern cells. Required.
//
// Panics if yStep is zero.
func (p *TilingPattern) YStep(yStep float32) *TilingPattern {
	if yStep == 0.0 {
		panic("y step must not be zero")
	}
	p.Pair(Name("YStep"), Real(yStep))
	return p
}

// Matrix is TilingPattern::matrix in pdf-writer src/color.rs.
//
// Write the /Matrix attribute.
//
// Maps the pattern coordinate system to the parent content stream
// coordinates. The default is the identity matrix.
func (p *TilingPattern) Matrix(matrix [6]float32) *TilingPattern {
	itemsF32(p.Insert(Name("Matrix")).Array(), matrix[:])
	return p
}

// PaintType is PaintType in pdf-writer src/color.rs.
//
// Type of paint for a tiling pattern.
type PaintType uint8

// The values of PaintType.
const (
	// PaintTypeColored paints the pattern with the colors specified in the
	// stream.
	PaintTypeColored PaintType = iota
	// PaintTypeUncolored paints the pattern with the colors active when the
	// pattern was painted.
	PaintTypeUncolored
)

// paintTypeNames holds the Rust name of each value of PaintType.
var paintTypeNames = [...]string{
	PaintTypeColored:   "Colored",
	PaintTypeUncolored: "Uncolored",
}

// String returns the Rust name of the paint type.
func (t PaintType) String() string {
	if int(t) < len(paintTypeNames) {
		return paintTypeNames[t]
	}
	return "PaintType(?)"
}

// toInt is PaintType::to_int in pdf-writer src/color.rs.
func (t PaintType) toInt() int32 {
	switch t {
	case PaintTypeColored:
		return 1
	case PaintTypeUncolored:
		return 2
	}
	panic(fmt.Sprintf("pdfwriter: the paint type %d is not known", uint8(t)))
}

// TilingType is TilingType in pdf-writer src/color.rs.
//
// How to adjust tile spacing.
type TilingType uint8

// The values of TilingType.
const (
	// TilingTypeConstantSpacing keeps a constant space between each tile,
	// tiles may be distorted by 1px.
	TilingTypeConstantSpacing TilingType = iota
	// TilingTypeNoDistortion keeps the tile size constant, spacing between
	// may vary by 1px.
	TilingTypeNoDistortion
	// TilingTypeFastConstantSpacing keeps a constant space between each tile
	// and draws faster, tiles may be distorted.
	TilingTypeFastConstantSpacing
)

// tilingTypeNames holds the Rust name of each value of TilingType.
var tilingTypeNames = [...]string{
	TilingTypeConstantSpacing:     "ConstantSpacing",
	TilingTypeNoDistortion:        "NoDistortion",
	TilingTypeFastConstantSpacing: "FastConstantSpacing",
}

// String returns the Rust name of the tiling type.
func (t TilingType) String() string {
	if int(t) < len(tilingTypeNames) {
		return tilingTypeNames[t]
	}
	return "TilingType(?)"
}

// toInt is TilingType::to_int in pdf-writer src/color.rs.
func (t TilingType) toInt() int32 {
	switch t {
	case TilingTypeConstantSpacing:
		return 1
	case TilingTypeNoDistortion:
		return 2
	case TilingTypeFastConstantSpacing:
		return 3
	}
	panic(fmt.Sprintf("pdfwriter: the tiling type %d is not known", uint8(t)))
}

// ShadingPattern is ShadingPattern in pdf-writer src/color.rs.
//
// Writer for a shading pattern dictionary. PDF 1.3+.
//
// This struct is created by Chunk.ShadingPattern.
type ShadingPattern struct {
	*Dict
}

// start is the macro writer! for ShadingPattern in pdf-writer src/color.rs.
func (p *ShadingPattern) start(obj Obj) {
	p.Dict = obj.Dict()
	p.Pair(Name("Type"), Name("Pattern"))
	p.Pair(Name("PatternType"), Int(patternTypeShading.toInt()))
}

// FunctionShading is ShadingPattern::function_shading in pdf-writer
// src/color.rs.
//
// Start writing the /Shading dictionary for a type 1, 2, or 3 shading.
func (p *ShadingPattern) FunctionShading() *FunctionShading {
	return Start[FunctionShading](p.Insert(Name("Shading")))
}

// ShadingRef is ShadingPattern::shading_ref in pdf-writer src/color.rs.
//
// Add an indirect reference to a /Shading dictionary or a shading stream.
func (p *ShadingPattern) ShadingRef(id Ref) *ShadingPattern {
	p.Pair(Name("Shading"), id)
	return p
}

// Matrix is ShadingPattern::matrix in pdf-writer src/color.rs.
//
// Write the /Matrix attribute.
//
// Sets the matrix to use for the pattern. Defaults to the identity matrix.
func (p *ShadingPattern) Matrix(matrix [6]float32) *ShadingPattern {
	itemsF32(p.Insert(Name("Matrix")).Array(), matrix[:])
	return p
}

// FunctionShading is FunctionShading in pdf-writer src/color.rs.
//
// Writer for a shading dictionary. PDF 1.3+.
//
// This struct is created by Chunk.FunctionShading and
// ShadingPattern.FunctionShading.
type FunctionShading struct {
	*Dict
}

// start is the macro writer! for FunctionShading in pdf-writer
// src/color.rs.
func (s *FunctionShading) start(obj Obj) {
	s.Dict = obj.Dict()
}

// ShadingType is FunctionShading::shading_type in pdf-writer src/color.rs.
//
// Write the /ShadingType attribute.
//
// Sets the type of shading. The available and required attributes change
// depending on this. Required.
func (s *FunctionShading) ShadingType(kind FunctionShadingType) *FunctionShading {
	s.Pair(Name("ShadingType"), Int(kind.toInt()))
	return s
}

// ColorSpace is FunctionShading::color_space in pdf-writer src/color.rs.
//
// Start writing the /ColorSpace attribute.
//
// Sets the color space of the shading function. May not be a Pattern space.
// Required.
func (s *FunctionShading) ColorSpace() *ColorSpace {
	return Start[ColorSpace](s.Insert(Name("ColorSpace")))
}

// Background is FunctionShading::background in pdf-writer src/color.rs.
//
// Write the /Background attribute.
//
// Sets the background color of the area to be shaded. The background
// iterator must contain exactly as many elements as the current color space
// has dimensions.
func (s *FunctionShading) Background(background ...float32) *FunctionShading {
	itemsF32(s.Insert(Name("Background")).Array(), background)
	return s
}

// BBox is FunctionShading::bbox in pdf-writer src/color.rs.
//
// Write the /BBox attribute.
//
// Sets the bounding box of the shading in the target coordinate system.
func (s *FunctionShading) BBox(bbox Rect) *FunctionShading {
	s.Pair(Name("BBox"), bbox)
	return s
}

// AntiAlias is FunctionShading::anti_alias in pdf-writer src/color.rs.
//
// Write the /AntiAlias attribute.
//
// Sets whether to anti-alias the shading.
func (s *FunctionShading) AntiAlias(antiAlias bool) *FunctionShading {
	s.Pair(Name("AntiAlias"), Bool(antiAlias))
	return s
}

// Domain is FunctionShading::domain in pdf-writer src/color.rs.
//
// Write the /Domain attribute.
//
// Sets the domain of the shading function in a rectangle. Can be used for
// function, axial, or radial shadings. Will otherwise default to
// [x_min = 0, x_max = 1, y_min = 0, y_max = 1].
func (s *FunctionShading) Domain(domain [4]float32) *FunctionShading {
	itemsF32(s.Insert(Name("Domain")).Array(), domain[:])
	return s
}

// Matrix is FunctionShading::matrix in pdf-writer src/color.rs.
//
// Write the /Matrix attribute.
//
// Maps the shading domain rectangle to the target coordinate system. Can be
// used for function shadings. Will otherwise default to the identity
// matrix.
func (s *FunctionShading) Matrix(matrix [6]float32) *FunctionShading {
	itemsF32(s.Insert(Name("Matrix")).Array(), matrix[:])
	return s
}

// Function is FunctionShading::function in pdf-writer src/color.rs.
//
// Write the /Function attribute.
//
// Sets the function to use for shading. Number of in- and outputs depends
// on shading type. Required for type 1, 2, and 3, optional otherwise.
func (s *FunctionShading) Function(function Ref) *FunctionShading {
	s.Pair(Name("Function"), function)
	return s
}

// Coords is FunctionShading::coords in pdf-writer src/color.rs.
//
// Write the /Coords attribute.
//
// Sets the coordinates of the start and end of the axis in terms of the
// target coordinate system. Required for axial (4 items) and radial (6
// items; centers and radii) shadings.
func (s *FunctionShading) Coords(coords ...float32) *FunctionShading {
	itemsF32(s.Insert(Name("Coords")).Array(), coords)
	return s
}

// Extend is FunctionShading::extend in pdf-writer src/color.rs.
//
// Write the /Extend attribute.
//
// Set whether the shading should extend beyond either side of the axis /
// circles. Can be used for axial and radial shadings.
func (s *FunctionShading) Extend(extend [2]bool) *FunctionShading {
	s.Insert(Name("Extend")).Array().Items(Bool(extend[0]), Bool(extend[1]))
	return s
}

// StreamShading is StreamShading in pdf-writer src/color.rs.
//
// Writer for a embedded file stream.
//
// This struct is created by Chunk.StreamShading.
type StreamShading struct {
	*Stream
}

// startStreamShading is StreamShading::start in pdf-writer src/color.rs.
//
// Create a new character map writer.
func startStreamShading(stream *Stream) *StreamShading {
	return &StreamShading{Stream: stream}
}

// ShadingType is StreamShading::shading_type in pdf-writer src/color.rs.
//
// Write the /ShadingType attribute.
//
// Sets the type of shading. The available and required attributes change
// depending on this. Required.
func (s *StreamShading) ShadingType(kind StreamShadingType) *StreamShading {
	s.Pair(Name("ShadingType"), Int(kind.toInt()))
	return s
}

// ColorSpace is StreamShading::color_space in pdf-writer src/color.rs.
//
// Start writing the /ColorSpace attribute.
//
// Sets the color space of the shading function. May not be a Pattern space.
// Required.
func (s *StreamShading) ColorSpace() *ColorSpace {
	return Start[ColorSpace](s.Insert(Name("ColorSpace")))
}

// Background is StreamShading::background in pdf-writer src/color.rs.
//
// Write the /Background attribute.
//
// Sets the background color of the area to be shaded. The background
// iterator must contain exactly as many elements as the current color space
// has dimensions.
func (s *StreamShading) Background(background ...float32) *StreamShading {
	itemsF32(s.Insert(Name("Background")).Array(), background)
	return s
}

// BBox is StreamShading::bbox in pdf-writer src/color.rs.
//
// Write the /BBox attribute.
//
// Sets the bounding box of the shading in the target coordinate system.
func (s *StreamShading) BBox(bbox Rect) *StreamShading {
	s.Pair(Name("BBox"), bbox)
	return s
}

// AntiAlias is StreamShading::anti_alias in pdf-writer src/color.rs.
//
// Write the /AntiAlias attribute.
//
// Sets whether to anti-alias the shading.
func (s *StreamShading) AntiAlias(antiAlias bool) *StreamShading {
	s.Pair(Name("AntiAlias"), Bool(antiAlias))
	return s
}

// Function is StreamShading::function in pdf-writer src/color.rs.
//
// Write the /Function attribute.
//
// Sets the function to use for shading. Number of in- and outputs depends
// on shading type. Optional.
func (s *StreamShading) Function(function Ref) *StreamShading {
	s.Pair(Name("Function"), function)
	return s
}

// BitsPerCoordinate is StreamShading::bits_per_coordinate in pdf-writer
// src/color.rs.
//
// Write the /BitsPerCoordinate attribute.
//
// Sets how many bits are used to represent each vertex coordinate. Can be
// any power of 2 between 1 and 32. Required.
func (s *StreamShading) BitsPerCoordinate(bits int32) *StreamShading {
	s.Pair(Name("BitsPerCoordinate"), Int(bits))
	return s
}

// BitsPerComponent is StreamShading::bits_per_component in pdf-writer
// src/color.rs.
//
// Write the /BitsPerComponent attribute.
//
// Sets how many bits are used to represent each color component. Can be
// any power of 2 between 1 and 16. Required.
func (s *StreamShading) BitsPerComponent(bits int32) *StreamShading {
	s.Pair(Name("BitsPerComponent"), Int(bits))
	return s
}

// BitsPerFlag is StreamShading::bits_per_flag in pdf-writer src/color.rs.
//
// Write the /BitsPerFlag attribute.
//
// Sets how many bits are used to represent the vertices' edge flags. Can be
// 0, 1, or 2. Required for type 4, 6, and 7.
func (s *StreamShading) BitsPerFlag(bits int32) *StreamShading {
	s.Pair(Name("BitsPerFlag"), Int(bits))
	return s
}

// Decode is StreamShading::decode in pdf-writer src/color.rs.
//
// Write the /Decode attribute.
//
// Sets the ranges of the vertices' coordinates. Required.
func (s *StreamShading) Decode(decode ...float32) *StreamShading {
	itemsF32(s.Insert(Name("Decode")).Array(), decode)
	return s
}

// VerticesPerRow is StreamShading::vertices_per_row in pdf-writer
// src/color.rs.
//
// Write the /VerticesPerRow attribute.
//
// Sets how many vertices are in each row of the lattice. Must be greater
// than 2. Required for type 5.
func (s *StreamShading) VerticesPerRow(vertices int32) *StreamShading {
	s.Pair(Name("VerticesPerRow"), Int(vertices))
	return s
}

// FunctionShadingType is FunctionShadingType in pdf-writer src/color.rs.
//
// What kind of shading to use for a function-based shading.
type FunctionShadingType uint8

// The values of FunctionShadingType.
const (
	// FunctionShadingTypeFunction is a shading where the function specifies
	// the color for each point in the domain.
	FunctionShadingTypeFunction FunctionShadingType = iota
	// FunctionShadingTypeAxial is a shading where the function specifies
	// the color for each point on a line.
	FunctionShadingTypeAxial
	// FunctionShadingTypeRadial is a shading where the function specifies
	// the color for each circle between two nested circles.
	FunctionShadingTypeRadial
)

// functionShadingTypeNames holds the Rust name of each value of
// FunctionShadingType.
var functionShadingTypeNames = [...]string{
	FunctionShadingTypeFunction: "Function",
	FunctionShadingTypeAxial:    "Axial",
	FunctionShadingTypeRadial:   "Radial",
}

// String returns the Rust name of the shading type.
func (t FunctionShadingType) String() string {
	if int(t) < len(functionShadingTypeNames) {
		return functionShadingTypeNames[t]
	}
	return "FunctionShadingType(?)"
}

// toInt is FunctionShadingType::to_int in pdf-writer src/color.rs.
func (t FunctionShadingType) toInt() int32 {
	switch t {
	case FunctionShadingTypeFunction:
		return 1
	case FunctionShadingTypeAxial:
		return 2
	case FunctionShadingTypeRadial:
		return 3
	}
	panic(fmt.Sprintf("pdfwriter: the function shading type %d is not known", uint8(t)))
}

// StreamShadingType is StreamShadingType in pdf-writer src/color.rs.
//
// What kind of shading to use.
type StreamShadingType uint8

// The values of StreamShadingType. In each one, the function specifies
// which vertex, or which corner of a cell, has which color. The function is
// optional.
const (
	StreamShadingTypeFreeformGouraud StreamShadingType = iota
	StreamShadingTypeLatticeGouraud
	StreamShadingTypeCoonsPatch
	StreamShadingTypeTensorProductPatch
)

// streamShadingTypeNames holds the Rust name of each value of
// StreamShadingType.
var streamShadingTypeNames = [...]string{
	StreamShadingTypeFreeformGouraud:    "FreeformGouraud",
	StreamShadingTypeLatticeGouraud:     "LatticeGouraud",
	StreamShadingTypeCoonsPatch:         "CoonsPatch",
	StreamShadingTypeTensorProductPatch: "TensorProductPatch",
}

// String returns the Rust name of the shading type.
func (t StreamShadingType) String() string {
	if int(t) < len(streamShadingTypeNames) {
		return streamShadingTypeNames[t]
	}
	return "StreamShadingType(?)"
}

// toInt is StreamShadingType::to_int in pdf-writer src/color.rs.
func (t StreamShadingType) toInt() int32 {
	switch t {
	case StreamShadingTypeFreeformGouraud:
		return 4
	case StreamShadingTypeLatticeGouraud:
		return 5
	case StreamShadingTypeCoonsPatch:
		return 6
	case StreamShadingTypeTensorProductPatch:
		return 7
	}
	panic(fmt.Sprintf("pdfwriter: the stream shading type %d is not known", uint8(t)))
}

// SeparationInfo is SeparationInfo in pdf-writer src/color.rs.
//
// Writer for the separation information dictionary. PDF 1.3+.
//
// This struct is created by Catalog.SeparationInfo.
type SeparationInfo struct {
	dict *Dict
}

// start is the macro writer! for SeparationInfo in pdf-writer src/color.rs.
func (i *SeparationInfo) start(obj Obj) {
	i.dict = obj.Dict()
}

// Pages is SeparationInfo::pages in pdf-writer src/color.rs.
//
// Write the /Pages attribute. Required.
//
// This indicates all page dictionaries in the document that represent
// separations of the same page and shall be rendered together.
func (i *SeparationInfo) Pages(pages ...Ref) *SeparationInfo {
	NewTypedArray[Ref](i.dict.Insert(Name("Pages")).Array()).Items(pages...)
	return i
}

// DeviceColorant is SeparationInfo::device_colorant in pdf-writer
// src/color.rs.
//
// Write the /DeviceColorant attribute as a name. Required.
//
// The name of the device colorant that corresponds to the separation.
func (i *SeparationInfo) DeviceColorant(colorant Name) *SeparationInfo {
	i.dict.Pair(Name("DeviceColorant"), colorant)
	return i
}

// DeviceColorantStr is SeparationInfo::device_colorant_str in pdf-writer
// src/color.rs.
//
// Write the /DeviceColorant attribute as a string. Required.
//
// The name of the device colorant that corresponds to the separation.
func (i *SeparationInfo) DeviceColorantStr(colorant string) *SeparationInfo {
	i.dict.Pair(Name("DeviceColorant"), TextStr(colorant))
	return i
}

// ColorSpace is SeparationInfo::color_space in pdf-writer src/color.rs.
//
// Start writing the /ColorSpace array.
//
// This shall be an Separation or DeviceN color space that further defines
// the separation color space.
func (i *SeparationInfo) ColorSpace() *ColorSpace {
	return Start[ColorSpace](i.dict.Insert(Name("ColorSpace")))
}

// Finish ends the dictionary, as Rust does when it drops the writer.
func (i *SeparationInfo) Finish() {
	i.dict.Finish()
}

// OutputIntent is OutputIntent in pdf-writer src/color.rs.
//
// Writer for an output intent dictionary. PDF 1.4+.
//
// This describes the output conditions under which the document may be
// rendered. Encouraged by PDF/A.
type OutputIntent struct {
	dict *Dict
}

// start is the macro writer! for OutputIntent in pdf-writer src/color.rs.
func (oi *OutputIntent) start(obj Obj) {
	oi.dict = obj.Dict()
	oi.dict.Pair(Name("Type"), Name("OutputIntent"))
}

// Subtype is OutputIntent::subtype in pdf-writer src/color.rs.
//
// Write the /S attribute. Required.
func (oi *OutputIntent) Subtype(subtype OutputIntentSubtype) *OutputIntent {
	oi.dict.Pair(Name("S"), subtype.toName())
	return oi
}

// OutputCondition is OutputIntent::output_condition in pdf-writer
// src/color.rs.
//
// Write the /OutputCondition attribute.
//
// A human-readable description of the output condition.
func (oi *OutputIntent) OutputCondition(condition TextStrLike) *OutputIntent {
	oi.dict.Pair(Name("OutputCondition"), condition)
	return oi
}

// OutputConditionIdentifier is OutputIntent::output_condition_identifier in
// pdf-writer src/color.rs.
//
// Write the /OutputConditionIdentifier attribute.
//
// A well-known identifier for the output condition.
func (oi *OutputIntent) OutputConditionIdentifier(identifier TextStrLike) *OutputIntent {
	oi.dict.Pair(Name("OutputConditionIdentifier"), identifier)
	return oi
}

// RegistryName is OutputIntent::registry_name in pdf-writer src/color.rs.
//
// Write the /RegistryName attribute.
//
// The URI of the registry that contains the output condition.
func (oi *OutputIntent) RegistryName(name TextStr) *OutputIntent {
	oi.dict.Pair(Name("RegistryName"), name)
	return oi
}

// Info is OutputIntent::info in pdf-writer src/color.rs.
//
// Write the /Info attribute.
//
// A human-readable string with additional info about the intended output
// device.
func (oi *OutputIntent) Info(info TextStrLike) *OutputIntent {
	oi.dict.Pair(Name("Info"), info)
	return oi
}

// DestOutputProfile is OutputIntent::dest_output_profile in pdf-writer
// src/color.rs.
//
// Write the /DestOutputProfile attribute.
//
// Required if /OutputConditionIdentifier does not contain a well-known
// identifier for the output condition. Must reference an ICC profile
// stream.
//
// Required for PDF/A. The profile must have the prtr or mntr tag.
func (oi *OutputIntent) DestOutputProfile(profile Ref) *OutputIntent {
	oi.dict.Pair(Name("DestOutputProfile"), profile)
	return oi
}

// Finish ends the dictionary, as Rust does when it drops the writer.
func (oi *OutputIntent) Finish() {
	oi.dict.Finish()
}

// OutputIntentSubtype is OutputIntentSubtype in pdf-writer src/color.rs. It
// is an enum with three cases without data and the case Custom, which
// OutputIntentSubtypeCustom makes (D27).
//
// The output intent subtype.
type OutputIntentSubtype struct {
	name Name
}

// The cases of OutputIntentSubtype without data.
var (
	// OutputIntentSubtypePDFX is GTS_PDFX.
	OutputIntentSubtypePDFX = OutputIntentSubtype{name: Name("GTS_PDFX")}
	// OutputIntentSubtypePDFA is GTS_PDFA1.
	//
	// This is the right value for PDF/A-1 through PDF/A-4.
	OutputIntentSubtypePDFA = OutputIntentSubtype{name: Name("GTS_PDFA1")}
	// OutputIntentSubtypePDFE is ISO_PDFE1.
	OutputIntentSubtypePDFE = OutputIntentSubtype{name: Name("ISO_PDFE1")}
)

// OutputIntentSubtypeCustom is OutputIntentSubtype::Custom in pdf-writer
// src/color.rs.
//
// Custom name defined in an ISO 32000 extension.
func OutputIntentSubtypeCustom(name Name) OutputIntentSubtype {
	return OutputIntentSubtype{name: name}
}

// toName is OutputIntentSubtype::to_name in pdf-writer src/color.rs.
func (s OutputIntentSubtype) toName() Name {
	return s.name
}

// Color spaces, shadings and patterns.

// ColorSpace is Chunk::color_space in pdf-writer src/chunk.rs.
//
// Start writing a color space.
func (c *Chunk) ColorSpace(id Ref) *ColorSpace {
	return Start[ColorSpace](c.Indirect(id))
}

// FunctionShading is Chunk::function_shading in pdf-writer src/chunk.rs.
//
// Start writing a function-based shading (type 1-3).
func (c *Chunk) FunctionShading(id Ref) *FunctionShading {
	return Start[FunctionShading](c.Indirect(id))
}

// StreamShading is Chunk::stream_shading in pdf-writer src/chunk.rs.
//
// Start writing a stream-based shading (type 4-7).
func (c *Chunk) StreamShading(id Ref, content []byte) *StreamShading {
	return startStreamShading(c.Stream(id, content))
}

// TilingPattern is Chunk::tiling_pattern in pdf-writer src/chunk.rs.
//
// Start writing a tiling pattern stream.
//
// You can create the content bytes using a Content builder.
func (c *Chunk) TilingPattern(id Ref, content []byte) *TilingPattern {
	return startTilingPattern(c.Stream(id, content))
}

// ShadingPattern is Chunk::shading_pattern in pdf-writer src/chunk.rs.
//
// Start writing a shading pattern.
func (c *Chunk) ShadingPattern(id Ref) *ShadingPattern {
	return Start[ShadingPattern](c.Indirect(id))
}

// ICCProfile is Chunk::icc_profile in pdf-writer src/chunk.rs.
//
// Start writing an ICC profile stream.
//
// The profile argument shall contain the ICC profile data conforming to
// ICC.1:2004-10 (PDF 1.7), ICC.1:2003-09 (PDF 1.6), ICC.1:2001-12 (PDF
// 1.5), ICC.1:1999-04 (PDF 1.4), or ICC 3.3 (PDF 1.3). Profile data is
// commonly compressed using the FlateDecode filter.
func (c *Chunk) ICCProfile(id Ref, profile []byte) *ICCProfile {
	return startICCProfile(c.Stream(id, profile))
}

// Resources is TilingPattern::resources in pdf-writer src/color.rs.
//
// Start writing the /Resources dictionary.
func (p *TilingPattern) Resources() *Resources {
	return Start[Resources](p.Insert(Name("Resources")))
}

// ExtGraphics is ShadingPattern::ext_graphics in pdf-writer src/color.rs.
//
// Start writing the /ExtGState attribute.
func (p *ShadingPattern) ExtGraphics() *ExtGraphicsState {
	return Start[ExtGraphicsState](p.Insert(Name("ExtGState")))
}
