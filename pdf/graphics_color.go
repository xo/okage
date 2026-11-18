// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// The colors and the color spaces of krilla src/graphics/color.rs.
//
// krilla supports four color models: RGB, luma, CMYK and separation. The
// base port leaves out the separation colors, with
// graphics/separation.rs (D62). Each color model has its own color type:
// ColorRGB, ColorLuma and ColorCMYK. Upstream names each of them Color in
// the modules rgb, luma and cmyk, so each Go name takes the name of its
// module as a suffix (D10).
//
// A color can be device-dependent or device-independent. A device-dependent
// color gives its values to the device as they are, and each screen or
// printer shows it in its own way. A device-independent color is in a
// well-specified color space, which each device converts into its own. krilla
// writes device-dependent colors by default. With the option WithNoDeviceCS,
// it embeds an ICC profile for the sGrey and sRGB color spaces, for luma and
// RGB colors. A CMYK color then uses the profile of WithCMYKProfile, or the
// device CMYK color space without it.

// The PDF names of the device color spaces in krilla
// src/graphics/color.rs.
const (
	// deviceRGB is DEVICE_RGB, the PDF name for the device RGB color space.
	deviceRGB = "DeviceRGB"
	// deviceGray is DEVICE_GRAY, the PDF name for the device gray color
	// space.
	deviceGray = "DeviceGray"
	// deviceCMYK is DEVICE_CMYK, the PDF name for the device CMYK color
	// space.
	deviceCMYK = "DeviceCMYK"
)

// Color is Color in krilla src/graphics/color.rs. Upstream holds a regular
// color, which is a RegularColor, or a special color, which is a
// SpecialColor. The only special color is the separation color, which the
// base port leaves out (D62). So Color is RegularColor too, and its types
// are ColorRGB, ColorLuma and ColorCMYK. Color::to_regular gives the color
// itself, so it has no port.
//
// A Color is a Paint, as the impl From<Color> for Paint of upstream makes
// it.
//
// A wrapper for storing colors from different color spaces.
type Color interface {
	Paint
	// toPDFColor is Color::to_pdf_color: the components of the color, from
	// 0 to 1.
	toPDFColor() []float32
	// colorSpace is Color::color_space and RegularColor::color_space.
	colorSpace(sc *serializeContext) colorSpace
	// asRGB is RegularColor::as_rgb. It returns the color as RGB, for the
	// colored glyphs of SVG and COLR, and false for a CMYK color.
	asRGB() (ColorRGB, bool)
	// isSubtractive is RegularColor::is_subtractive. It returns true for a
	// CMYK color, which is subtractive.
	isSubtractive() bool
	// hash is the derived Hash of Color.
	hash(h *hasher)
}

// The cases of RegularColor in krilla src/graphics/color.rs, which the
// derived Hash writes.
const (
	regularColorRGB = iota
	regularColorLuma
	regularColorCMYK
)

// hashRegularColor writes the derived Hash of Color::Regular in krilla
// src/graphics/color.rs: the case of Color, then the case of RegularColor.
func hashRegularColor(h *hasher, regularCase int) {
	// The case Color::Regular.
	h.usize(0)
	h.usize(regularCase)
}

// ColorLuma is luma::Color in krilla src/graphics/color.rs. The zero value is
// black, the Default of upstream.
//
// A luma color.
type ColorLuma struct {
	lightness uint8
}

// NewColorLuma is luma::Color::new in krilla src/graphics/color.rs.
//
// Create a new luma color.
func NewColorLuma(lightness uint8) ColorLuma {
	return ColorLuma{lightness: lightness}
}

// ColorLumaBlack is luma::Color::black in krilla src/graphics/color.rs.
//
// Create a black luma color.
func ColorLumaBlack() ColorLuma {
	return NewColorLuma(0)
}

// ColorLumaWhite is luma::Color::white in krilla src/graphics/color.rs.
//
// Create a white luma color.
func ColorLumaWhite() ColorLuma {
	return NewColorLuma(255)
}

// toPDFColor is luma::Color::to_pdf_color in krilla src/graphics/color.rs,
// in a slice of one component.
func (c ColorLuma) toPDFColor() []float32 {
	return []float32{float32(c.lightness) / 255.0}
}

// colorSpace is the case Luma of RegularColor::color_space in krilla
// src/graphics/color.rs.
func (c ColorLuma) colorSpace(sc *serializeContext) colorSpace {
	return colorSpaceLuma(sc.serializeSettings().noDeviceCS)
}

// asRGB is the case Luma of RegularColor::as_rgb in krilla
// src/graphics/color.rs.
func (c ColorLuma) asRGB() (ColorRGB, bool) {
	return NewColorRGB(c.lightness, c.lightness, c.lightness), true
}

// isSubtractive is the case Luma of RegularColor::is_subtractive in krilla
// src/graphics/color.rs.
func (c ColorLuma) isSubtractive() bool {
	return false
}

// hash is the derived Hash of Color for a luma color in krilla
// src/graphics/color.rs.
func (c ColorLuma) hash(h *hasher) {
	hashRegularColor(h, regularColorLuma)
	h.u8(c.lightness)
}

// hashPaint is the derived Hash of Paint for a luma color.
func (c ColorLuma) hashPaint(h *hasher) {
	h.usize(innerPaintColor)
	c.hash(h)
}

// colorSpaceLuma is luma::color_space in krilla src/graphics/color.rs.
func colorSpaceLuma(noDeviceCS bool) colorSpace {
	if noDeviceCS {
		return colorSpace{kind: colorSpaceCIELuma}
	}
	return colorSpace{kind: colorSpaceDeviceGray}
}

// ColorCMYK is cmyk::Color in krilla src/graphics/color.rs. The Default of
// upstream is DefaultColorCMYK, which is black.
//
// A CMYK color.
type ColorCMYK struct {
	cyan, magenta, yellow, black uint8
}

// NewColorCMYK is cmyk::Color::new in krilla src/graphics/color.rs.
//
// Create a new CMYK color.
func NewColorCMYK(cyan, magenta, yellow, black uint8) ColorCMYK {
	return ColorCMYK{cyan: cyan, magenta: magenta, yellow: yellow, black: black}
}

// DefaultColorCMYK is the Default of cmyk::Color in krilla
// src/graphics/color.rs: the black of the component black alone.
func DefaultColorCMYK() ColorCMYK {
	return NewColorCMYK(0, 0, 0, 255)
}

// toPDFColor is cmyk::Color::to_pdf_color in krilla src/graphics/color.rs.
func (c ColorCMYK) toPDFColor() []float32 {
	return []float32{
		float32(c.cyan) / 255.0,
		float32(c.magenta) / 255.0,
		float32(c.yellow) / 255.0,
		float32(c.black) / 255.0,
	}
}

// colorSpace is the case Cmyk of RegularColor::color_space in krilla
// src/graphics/color.rs. Without a CMYK profile, when the colors must be
// device-independent, it records the error MissingCMYKProfile and gives the
// device CMYK color space.
func (c ColorCMYK) colorSpace(sc *serializeContext) colorSpace {
	cs, ok := colorSpaceCMYK(sc.serializeSettings())
	if !ok {
		sc.registerValidationError(validationError{kind: validationMissingCMYKProfile})
		return colorSpace{kind: colorSpaceDeviceCMYK}
	}
	return cs
}

// asRGB is the case Cmyk of RegularColor::as_rgb in krilla
// src/graphics/color.rs. A CMYK color has no RGB form.
func (c ColorCMYK) asRGB() (ColorRGB, bool) {
	return ColorRGB{}, false
}

// isSubtractive is the case Cmyk of RegularColor::is_subtractive in krilla
// src/graphics/color.rs.
func (c ColorCMYK) isSubtractive() bool {
	return true
}

// hash is the derived Hash of Color for a CMYK color in krilla
// src/graphics/color.rs.
func (c ColorCMYK) hash(h *hasher) {
	hashRegularColor(h, regularColorCMYK)
	h.u8(c.cyan)
	h.u8(c.magenta)
	h.u8(c.yellow)
	h.u8(c.black)
}

// hashPaint is the derived Hash of Paint for a CMYK color.
func (c ColorCMYK) hashPaint(h *hasher) {
	h.usize(innerPaintColor)
	c.hash(h)
}

// colorSpaceCMYK is cmyk::color_space in krilla src/graphics/color.rs. It
// returns false when the colors must be device-independent and the settings
// have no CMYK profile.
func colorSpaceCMYK(ss *serializeSettings) (colorSpace, bool) {
	if ss.noDeviceCS {
		if ss.cmykProfile == nil {
			return colorSpace{}, false
		}
		return colorSpace{kind: colorSpaceCIECMYK, cmyk: ss.cmykProfile}, true
	}
	return colorSpace{kind: colorSpaceDeviceCMYK}, true
}

// ColorRGB is rgb::Color in krilla src/graphics/color.rs. The zero value is
// black, the Default of upstream.
//
// An RGB color.
type ColorRGB struct {
	red, green, blue uint8
}

// NewColorRGB is rgb::Color::new in krilla src/graphics/color.rs.
//
// Create a new RGB color.
func NewColorRGB(red, green, blue uint8) ColorRGB {
	return ColorRGB{red: red, green: green, blue: blue}
}

// NewColorRGBLinear is rgb::Color::new_linear in krilla
// src/graphics/color.rs. It gives the same color as NewColorRGB, as
// upstream does.
//
// Create a new linear RGB color.
func NewColorRGBLinear(red, green, blue uint8) ColorRGB {
	return ColorRGB{red: red, green: green, blue: blue}
}

// ColorRGBBlack is rgb::Color::black in krilla src/graphics/color.rs.
//
// Create a black RGB color.
func ColorRGBBlack() ColorRGB {
	return NewColorRGB(0, 0, 0)
}

// ColorRGBWhite is rgb::Color::white in krilla src/graphics/color.rs.
//
// Create a white RGB color.
func ColorRGBWhite() ColorRGB {
	return NewColorRGB(255, 255, 255)
}

// Red is rgb::Color::red in krilla src/graphics/color.rs.
//
// The red component of the color.
func (c ColorRGB) Red() uint8 {
	return c.red
}

// Green is rgb::Color::green in krilla src/graphics/color.rs.
//
// The green component of the color.
func (c ColorRGB) Green() uint8 {
	return c.green
}

// Blue is rgb::Color::blue in krilla src/graphics/color.rs.
//
// The blue component of the color.
func (c ColorRGB) Blue() uint8 {
	return c.blue
}

// toPDFColor is rgb::Color::to_pdf_color in krilla src/graphics/color.rs.
func (c ColorRGB) toPDFColor() []float32 {
	return []float32{
		float32(c.red) / 255.0,
		float32(c.green) / 255.0,
		float32(c.blue) / 255.0,
	}
}

// colorSpace is rgb::Color::color_space in krilla src/graphics/color.rs.
func (c ColorRGB) colorSpace(sc *serializeContext) colorSpace {
	return colorSpaceRGB(sc.serializeSettings().noDeviceCS)
}

// asRGB is the case Rgb of RegularColor::as_rgb in krilla
// src/graphics/color.rs.
func (c ColorRGB) asRGB() (ColorRGB, bool) {
	return c, true
}

// isSubtractive is the case Rgb of RegularColor::is_subtractive in krilla
// src/graphics/color.rs.
func (c ColorRGB) isSubtractive() bool {
	return false
}

// hash is the derived Hash of Color for an RGB color in krilla
// src/graphics/color.rs.
func (c ColorRGB) hash(h *hasher) {
	hashRegularColor(h, regularColorRGB)
	h.u8(c.red)
	h.u8(c.green)
	h.u8(c.blue)
}

// hashPaint is the derived Hash of Paint for an RGB color.
func (c ColorRGB) hashPaint(h *hasher) {
	h.usize(innerPaintColor)
	c.hash(h)
}

// colorSpaceRGB is rgb::color_space in krilla src/graphics/color.rs.
func colorSpaceRGB(noDeviceCS bool) colorSpace {
	if noDeviceCS {
		return colorSpace{kind: colorSpaceCIESRGB}
	}
	return colorSpace{kind: colorSpaceDeviceRGB}
}

// colorSpaceKind is the case of a colorSpace.
type colorSpaceKind uint8

// The cases of ColorSpace in krilla src/graphics/color.rs. The device cases
// are DeviceColorSpace, and the CIE-based cases are CieBasedColorSpace. The
// case Special is for the separation colors, which the base port leaves out
// (D62).
const (
	// colorSpaceDeviceRGB is DeviceColorSpace::Rgb.
	colorSpaceDeviceRGB colorSpaceKind = iota
	// colorSpaceDeviceGray is DeviceColorSpace::Gray.
	colorSpaceDeviceGray
	// colorSpaceDeviceCMYK is DeviceColorSpace::Cmyk.
	colorSpaceDeviceCMYK
	// colorSpaceCIESRGB is CieBasedColorSpace::Srgb.
	colorSpaceCIESRGB
	// colorSpaceCIELuma is CieBasedColorSpace::Luma.
	colorSpaceCIELuma
	// colorSpaceCIECMYK is CieBasedColorSpace::Cmyk, with an ICC profile of
	// four components.
	colorSpaceCIECMYK
)

// colorSpace is ColorSpace in krilla src/graphics/color.rs, and also
// RegularColorSpace, DeviceColorSpace and CieBasedColorSpace, which are its
// parts. Without the case Special, a ColorSpace and a RegularColorSpace are
// the same, and the conversions of upstream between them have no port.
type colorSpace struct {
	kind colorSpaceKind
	// cmyk is the profile of colorSpaceCIECMYK, the ICCBasedColorSpace<4>
	// of upstream.
	cmyk *ICCProfile
}

// maybeDeviceKind is the case of a maybeDeviceColorSpace.
type maybeDeviceKind uint8

// The cases of MaybeDeviceColorSpace in krilla src/serialize.rs.
const (
	maybeDeviceRGB maybeDeviceKind = iota
	maybeDeviceGray
	maybeDeviceCMYK
	maybeDeviceResource
)

// maybeDeviceColorSpace is MaybeDeviceColorSpace in krilla src/serialize.rs:
// a device color space, which a name gives, or a color space resource.
type maybeDeviceColorSpace struct {
	kind maybeDeviceKind
	// cs is the resource of the case ColorSpace.
	cs colorSpaceResource
}

// write is MaybeDeviceColorSpace::write in krilla src/serialize.rs.
func (m maybeDeviceColorSpace) write(target pdfwriter.Obj) {
	switch m.kind {
	case maybeDeviceRGB:
		target.Primitive(pdfwriter.Name(deviceRGB))
	case maybeDeviceGray:
		target.Primitive(pdfwriter.Name(deviceGray))
	case maybeDeviceCMYK:
		target.Primitive(pdfwriter.Name(deviceCMYK))
	default:
		target.Primitive(m.cs.getRef())
	}
}

// registerColorSpace is SerializeContext::register_colorspace in krilla
// src/serialize.rs. It writes the ICC-based color space of a
// device-independent color space once, and gives its resource. A device
// color space needs no object. The case of the separation colors is not in
// the base port (D62).
func (sc *serializeContext) registerColorSpace(cc *chunkContainer, cs colorSpace) maybeDeviceColorSpace {
	switch cs.kind {
	case colorSpaceCIESRGB:
		return maybeDeviceColorSpace{
			kind: maybeDeviceResource,
			cs:   registerResourceable(sc, cc, iccBasedColorSpace{profile: sc.settings.pdfVersion().rgbICC()}),
		}
	case colorSpaceCIELuma:
		return maybeDeviceColorSpace{
			kind: maybeDeviceResource,
			cs:   registerResourceable(sc, cc, iccBasedColorSpace{profile: sc.settings.pdfVersion().greyICC()}),
		}
	case colorSpaceCIECMYK:
		return maybeDeviceColorSpace{
			kind: maybeDeviceResource,
			cs:   registerResourceable(sc, cc, iccBasedColorSpace{profile: cs.cmyk}),
		}
	case colorSpaceDeviceGray:
		return maybeDeviceColorSpace{kind: maybeDeviceGray}
	case colorSpaceDeviceRGB:
		return maybeDeviceColorSpace{kind: maybeDeviceRGB}
	default:
		return maybeDeviceColorSpace{kind: maybeDeviceCMYK}
	}
}
