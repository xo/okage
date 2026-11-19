// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/internal/siphasher"
)

// This file ports krilla src/graphics/image/mod.rs, the bitmap images.
//
// okage ports no crate that decodes an image (D7, D62). image/png,
// image/jpeg, image/gif and golang.org/x/image/webp decode the images in
// place of the crates png, zune-jpeg, gif and image-webp. The readers of
// graphics_image_decode.go read what these packages do not give, such as an
// ICC profile, from the headers of the files.
//
// The port has these differences from upstream:
//
//   - Deferred of upstream computes a value at once without the feature
//     rayon, so a function of this file that makes an Image decodes the
//     image at once. A decode error is kept, and Document.Finish returns it,
//     as krilla returns KrillaError::Image.
//   - A decoded image holds at most maxImageBytes bytes. Above that, a
//     function that makes an Image returns ErrPDFImage. Upstream has no such
//     limit, except the limits of its decoders.
//   - The trait CustomImage of upstream is an image.Image of Go, which
//     ImageFromImage takes. It cannot give an ICC profile.
//   - An error is an error that wraps ErrPDFImage, where upstream returns a
//     String.

// bitsPerComponent is BitsPerComponent in krilla
// src/graphics/image/mod.rs.
//
// The number of bits per color component.
type bitsPerComponent uint8

// The cases of BitsPerComponent.
const (
	// bitsPerComponentEight is BitsPerComponent::Eight, eight bits per
	// component.
	bitsPerComponentEight bitsPerComponent = iota
	// bitsPerComponentSixteen is BitsPerComponent::Sixteen, sixteen bits per
	// component.
	bitsPerComponentSixteen
)

// asU8 is BitsPerComponent::as_u8 in krilla src/graphics/image/mod.rs.
func (b bitsPerComponent) asU8() uint8 {
	if b == bitsPerComponentSixteen {
		return 16
	}
	return 8
}

// imageColorSpace is ImageColorspace in krilla src/graphics/image/mod.rs.
//
// The color space of the image.
type imageColorSpace uint8

// The cases of ImageColorspace.
const (
	// imageColorSpaceRGB is ImageColorspace::Rgb, the RGB color space.
	imageColorSpaceRGB imageColorSpace = iota
	// imageColorSpaceLuma is ImageColorspace::Luma, the luma color space.
	imageColorSpaceLuma
	// imageColorSpaceCMYK is ImageColorspace::Cmyk, the CMYK color space.
	imageColorSpaceCMYK
)

// numComponents is ImageColorspace::num_components in krilla
// src/graphics/image/mod.rs.
func (cs imageColorSpace) numComponents() uint8 {
	switch cs {
	case imageColorSpaceLuma:
		return 1
	case imageColorSpaceCMYK:
		return 4
	default:
		return 3
	}
}

// matchesICCProfile is ImageColorspace::matches_icc_profile in krilla
// src/graphics/image/mod.rs.
func (cs imageColorSpace) matchesICCProfile(profile genericICCProfile) bool {
	return cs.numComponents() == profile.profile.components
}

// imageColorSpaceFromZune is the impl TryFrom<ColorSpace> for
// ImageColorspace in krilla src/graphics/image/mod.rs.
func imageColorSpaceFromZune(value zuneColorSpace) (imageColorSpace, bool) {
	switch value {
	case zuneRGB, zuneRGBA, zuneYCbCr:
		return imageColorSpaceRGB, true
	case zuneLuma, zuneLumaA:
		return imageColorSpaceLuma, true
	case zuneYCCK, zuneCMYK:
		return imageColorSpaceCMYK, true
	}
	return 0, false
}

// zuneColorSpace is ColorSpace of the crate zune-core, which upstream uses
// to name the layout of the samples of a decoded image and the color space
// of a JPEG file.
type zuneColorSpace uint8

// The cases of ColorSpace of zune-core that krilla uses.
const (
	zuneRGB zuneColorSpace = iota
	zuneRGBA
	zuneYCbCr
	zuneLuma
	zuneLumaA
	zuneYCCK
	zuneCMYK
)

// imageRepr is the enum Repr in krilla src/graphics/image/mod.rs. The cases
// are *sampledRepr, *jpegRepr and *pngRepr.
type imageRepr interface {
	// bits is Repr::bits_per_component. The fields of two cases have the
	// name bitsPerComponent, so the method takes another name.
	bits() uint8
}

// sampledRepr is SampledRepr in krilla src/graphics/image/mod.rs: the
// deflated samples of the colors and of the alpha. A nil alphaChannel is
// None.
type sampledRepr struct {
	colorChannel     []byte
	alphaChannel     []byte
	bitsPerComponent bitsPerComponent
}

// jpegRepr is JpegRepr in krilla src/graphics/image/mod.rs: the data of a
// JPEG file, which the PDF file holds as it is.
type jpegRepr struct {
	data             []byte
	bitsPerComponent bitsPerComponent
	invertCMYK       bool
}

// pngRepr is PngRepr in krilla src/graphics/image/mod.rs: the data of the
// chunks IDAT of a PNG file, which the PDF file holds as it is. A palette is
// nil when hasPalette is false, the None of upstream.
type pngRepr struct {
	data       []byte
	bitDepth   uint8
	colorType  pngColorType
	hasPalette bool
	palette    []byte
}

// pngReprFromData is the impl From<png::PngData> for PngRepr in krilla
// src/graphics/image/mod.rs.
func pngReprFromData(data pngData) *pngRepr {
	return &pngRepr{
		data:       data.idat,
		bitDepth:   data.bitDepth,
		colorType:  data.colorType,
		hasPalette: data.hasPalette,
		palette:    data.palette,
	}
}

// bits is Repr::bits_per_component in krilla src/graphics/image/mod.rs.
func (s *sampledRepr) bits() uint8 { return s.bitsPerComponent.asU8() }

// bits is Repr::bits_per_component in krilla src/graphics/image/mod.rs.
func (j *jpegRepr) bits() uint8 { return j.bitsPerComponent.asU8() }

// bits is Repr::bits_per_component in krilla src/graphics/image/mod.rs.
func (p *pngRepr) bits() uint8 { return p.bitDepth }

// imageMetadata is ImageMetadata in krilla src/graphics/image/mod.rs. A
// profile with no data is None.
type imageMetadata struct {
	width            uint32
	height           uint32
	colorSpace       imageColorSpace
	hasAlpha         bool
	bitsPerComponent bitsPerComponent
	icc              genericICCProfile
}

// Image is Image in krilla src/graphics/image/mod.rs, with the fields of
// ImageRepr. A *Image is cheap to share, as the Arc of upstream. The Image
// keeps the data that made it, so the caller must not change the data.
//
// A bitmap image.
type Image struct {
	// inner is the Deferred<Result<Repr, String>> of upstream, which the
	// port computes at once. err is its error.
	inner       imageRepr
	err         error
	metadata    imageMetadata
	sip         siphasher.Hash128
	interpolate bool
}

// imageBytes hashes the data of an image, as krilla hashes a [u8] with
// sip_hash.
type imageBytes []byte

// typeID names the type for sipHash.
func (imageBytes) typeID() string { return "imageBytes" }

// hash is the Hash of a [u8].
func (b imageBytes) hash(h *hasher) { h.bytes(b) }

// typeID names the type for sipHash.
func (*Image) typeID() string { return "Image" }

// hash is the derived Hash of Image in krilla src/graphics/image/mod.rs. It
// writes the hash of the data, as the impl Hash for ImageRepr does.
func (i *Image) hash(h *hasher) {
	h.u128(i.sip)
}

// imageError wraps ErrPDFImage with the message of a decoder.
func imageError(err error) error {
	return fmt.Errorf("%w: %w", okage.ErrPDFImage, err)
}

// ImageFromPNG is Image::from_png in krilla src/graphics/image/mod.rs.
// interpolate sets the key /Interpolate of the image.
//
// Create a new bitmap image from a .png file.
//
// If possible, the PNG image will be embedded directly into the PDF.
// Otherwise, krilla falls back to decoding and then re-compressing the
// image.
func ImageFromPNG(data []byte, interpolate bool) (*Image, error) {
	hash := sipHash(imageBytes(data))
	metadata, info, err := pngMetadata(data)
	if err != nil {
		return nil, imageError(err)
	}

	inner, err := decodePNG(data, info)
	return &Image{
		inner:       inner,
		err:         err,
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}, nil
}

// errPNGNeedsDecode is the error of ImageFromPNGNative for a PNG file that
// cannot go into a PDF file without a decode.
var errPNGNeedsDecode = errors.New("PNG cannot be embedded without decoding")

// ImageFromPNGNative is Image::from_png_native in krilla
// src/graphics/image/mod.rs.
//
// Create a new bitmap image from a .png file without decoding its pixels.
//
// If possible, the PNG image will be embedded directly into the PDF.
// Otherwise, unlike ImageFromPNG, this function returns an error. This is
// useful if you do not want the image to be decoded without notice.
func ImageFromPNGNative(data []byte, interpolate bool) (*Image, error) {
	hash := sipHash(imageBytes(data))
	metadata, _, err := pngMetadata(data)
	if err != nil {
		return nil, imageError(err)
	}
	pd, ok := newPNGData(data)
	if !ok {
		return nil, imageError(errPNGNeedsDecode)
	}

	return &Image{
		inner:       pngReprFromData(pd),
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}, nil
}

// ImageFromJPEG is Image::from_jpeg in krilla src/graphics/image/mod.rs.
// The PDF file holds the data of the file as it is.
//
// Create a new bitmap image from a .jpg file.
func ImageFromJPEG(data []byte, interpolate bool) (*Image, error) {
	hash := sipHash(imageBytes(data))
	metadata, err := jpegMetadata(data)
	if err != nil {
		return nil, imageError(err)
	}

	inner, err := decodeJPEG(data)
	return &Image{
		inner:       inner,
		err:         err,
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}, nil
}

// imageFromJPEGWithICC is Image::from_jpeg_with_icc in krilla
// src/graphics/image/mod.rs, which upstream hides from its documentation.
// A nil profile is None.
//
// Create a new bitmap image from a .jpg file with custom ICC profile.
func imageFromJPEGWithICC(data, iccProfile []byte, interpolate bool) (*Image, error) {
	hash := sipHash(imageBytes(data))
	metadata, err := jpegMetadata(data)
	if err != nil {
		return nil, imageError(err)
	}
	metadata.icc = genericICCProfile{}
	if iccProfile != nil {
		metadata.icc = getICCProfileType(iccProfile, metadata.colorSpace)
	}

	inner, err := decodeJPEG(data)
	return &Image{
		inner:       inner,
		err:         err,
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}, nil
}

// ImageFromGIF is Image::from_gif in krilla src/graphics/image/mod.rs. It
// takes the first frame of the file.
//
// Create a new bitmap image from a .gif file.
func ImageFromGIF(data []byte, interpolate bool) (*Image, error) {
	hash := sipHash(imageBytes(data))
	metadata, err := gifMetadata(data)
	if err != nil {
		return nil, imageError(err)
	}

	inner, err := decodeGIF(data)
	return &Image{
		inner:       inner,
		err:         err,
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}, nil
}

// ImageFromWebP is Image::from_webp in krilla src/graphics/image/mod.rs. It
// takes the first frame of the file.
//
// Create a new bitmap image from a .webp file.
func ImageFromWebP(data []byte, interpolate bool) (*Image, error) {
	hash := sipHash(imageBytes(data))
	metadata, err := webpMetadata(data)
	if err != nil {
		return nil, imageError(err)
	}

	inner, err := decodeWebP(data, metadata.hasAlpha)
	return &Image{
		inner:       inner,
		err:         err,
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}, nil
}

// customImage is the data that the trait CustomImage of krilla
// src/graphics/image/mod.rs gives. ImageFromImage makes it from an
// image.Image. A nil alphaChannel or iccProfile is None.
type customImage struct {
	colorChannel     []byte
	alphaChannel     []byte
	bitsPerComponent bitsPerComponent
	width            uint32
	height           uint32
	iccProfile       []byte
	colorSpace       imageColorSpace
}

// customImageKey is the tuple (image, interpolate) that Image::from_custom
// hashes.
type customImageKey struct {
	image       *customImage
	interpolate bool
}

// typeID names the type for sipHash.
func (customImageKey) typeID() string { return "customImageKey" }

// hash writes the fields of the custom image, then interpolate.
func (k customImageKey) hash(h *hasher) {
	c := k.image
	h.bytes(c.colorChannel)
	h.bool(c.alphaChannel != nil)
	h.bytes(c.alphaChannel)
	h.u8(uint8(c.bitsPerComponent))
	h.u32(c.width)
	h.u32(c.height)
	h.bool(c.iccProfile != nil)
	h.bytes(c.iccProfile)
	h.u8(uint8(c.colorSpace))
	h.bool(k.interpolate)
}

// errCustomImageSize is the error of a custom image whose channels do not
// have the length of its size.
var errCustomImageSize = errors.New("the length of a channel does not match the size of the image")

// imageFromCustom is Image::from_custom in krilla
// src/graphics/image/mod.rs. upstream panics when the length of a channel
// does not match the size of the image. The port then keeps an error, which
// Document.Finish returns.
//
// Create a new image from a custom image.
func imageFromCustom(c *customImage, interpolate bool) *Image {
	hash := sipHash(customImageKey{image: c, interpolate: interpolate})
	metadata := imageMetadata{
		width:            c.width,
		height:           c.height,
		colorSpace:       c.colorSpace,
		hasAlpha:         c.alphaChannel != nil,
		bitsPerComponent: c.bitsPerComponent,
	}
	if c.iccProfile != nil {
		metadata.icc = getICCProfileType(c.iccProfile, c.colorSpace)
	}

	img := &Image{
		metadata:    metadata,
		sip:         hash,
		interpolate: interpolate,
	}
	bytesPerComponent := uint64(c.bitsPerComponent.asU8() / 8)
	pixels := uint64(c.width) * uint64(c.height)
	colorChannelLen := bytesPerComponent * uint64(c.colorSpace.numComponents()) * pixels
	alphaChannelLen := bytesPerComponent * pixels
	if uint64(len(c.colorChannel)) != colorChannelLen ||
		(c.alphaChannel != nil && uint64(len(c.alphaChannel)) != alphaChannelLen) {
		img.err = errCustomImageSize
		return img
	}

	s := &sampledRepr{
		colorChannel:     deflateEncode(c.colorChannel),
		bitsPerComponent: c.bitsPerComponent,
	}
	if c.alphaChannel != nil {
		s.alphaChannel = deflateEncode(c.alphaChannel)
	}
	img.inner = s
	return img
}

// ImageFromImage is Image::from_custom in krilla src/graphics/image/mod.rs,
// for an image.Image of Go in place of the trait CustomImage. A gray image
// becomes an image in the luma color space, a CMYK image an image in the
// CMYK color space, and any other image an RGB image. An image with 16 bits
// for each channel, such as an *image.NRGBA64, keeps its 16 bits. The image
// has an alpha channel when it is not opaque. It returns an error that wraps
// ErrPDFImage when the samples of the image, with an alpha channel, need
// more than maxImageBytes bytes.
//
// Create a new image from a custom image.
func ImageFromImage(img image.Image, interpolate bool) (*Image, error) {
	c, err := customFromImage(img)
	if err != nil {
		return nil, imageError(err)
	}
	return imageFromCustom(c, interpolate), nil
}

// customFromImage converts an image.Image into the data of a custom image.
func customFromImage(img image.Image) (*customImage, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	c := &customImage{width: uint32(w), height: uint32(h)}
	sixteen := false
	switch img.ColorModel() {
	case color.Gray16Model, color.RGBA64Model, color.NRGBA64Model, color.Alpha16Model:
		sixteen = true
		c.bitsPerComponent = bitsPerComponentSixteen
	}
	switch img.ColorModel() {
	case color.GrayModel, color.Gray16Model:
		c.colorSpace = imageColorSpaceLuma
	case color.CMYKModel:
		c.colorSpace = imageColorSpaceCMYK
	default:
		c.colorSpace = imageColorSpaceRGB
	}
	// The bound counts an alpha channel, so that isOpaque does not read the
	// pixels of an image that is too large.
	pixelBytes := uint64(c.colorSpace.numComponents()) + 1
	if sixteen {
		pixelBytes *= 2
	}
	if err := checkImageBytes(w, h, pixelBytes); err != nil {
		return nil, err
	}
	opaque := isOpaque(img)

	switch m := img.(type) {
	case *image.Gray:
		c.colorChannel = rowsOf(m.Pix, m.Stride, w, h)
		return c, nil
	case *image.Gray16:
		c.colorChannel = rowsOf(m.Pix, m.Stride, 2*w, h)
		return c, nil
	case *image.CMYK:
		c.colorChannel = rowsOf(m.Pix, m.Stride, 4*w, h)
		return c, nil
	}

	switch c.colorSpace {
	case imageColorSpaceLuma:
		at := gray16Func(img)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				v := at(x, y)
				if sixteen {
					c.colorChannel = append(c.colorChannel, uint8(v>>8), uint8(v))
				} else {
					c.colorChannel = append(c.colorChannel, uint8(v>>8))
				}
			}
		}
	case imageColorSpaceCMYK:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				// The model gives a color.CMYK.
				v, _ := color.CMYKModel.Convert(img.At(x, y)).(color.CMYK)
				c.colorChannel = append(c.colorChannel, v.C, v.M, v.Y, v.K)
			}
		}
	default:
		channels := 3
		if !opaque {
			channels = 4
		}
		data := interleave(img, channels, sixteen)
		bps := 1
		if sixteen {
			bps = 2
		}
		if opaque {
			c.colorChannel = data
			return c, nil
		}
		c.colorChannel, c.alphaChannel = splitChannels(data, 3*bps, bps)
	}
	return c, nil
}

// ImageFromRGBA8 is Image::from_rgba8 in krilla src/graphics/image/mod.rs.
// data holds 4 bytes for each pixel, which are not premultiplied by the
// alpha. Like upstream, it does not check the length of data.
//
// Create a new RGB image from raw RGBA pixels.
func ImageFromRGBA8(data []byte, width, height uint32) *Image {
	hash := sipHash(imageBytes(data))
	metadata := imageMetadata{
		hasAlpha:         true,
		width:            width,
		height:           height,
		bitsPerComponent: bitsPerComponentEight,
		colorSpace:       imageColorSpaceRGB,
	}

	colorChannel, alphaChannel, bpc := handleImage(data, zuneRGBA, bitsPerComponentEight)
	return &Image{
		inner: &sampledRepr{
			colorChannel:     colorChannel,
			alphaChannel:     alphaChannel,
			bitsPerComponent: bpc,
		},
		metadata: metadata,
		sip:      hash,
	}
}

// Size is Image::size in krilla src/graphics/image/mod.rs.
//
// Return the size of the image.
func (i *Image) Size() (width, height uint32) {
	return i.metadata.width, i.metadata.height
}

// icc is Image::icc in krilla src/graphics/image/mod.rs.
func (i *Image) icc() genericICCProfile {
	return i.metadata.icc
}

// colorSpace is Image::color_space in krilla src/graphics/image/mod.rs.
func (i *Image) colorSpace() imageColorSpace {
	return i.metadata.colorSpace
}

// supportsICC is PdfVersion::supports_icc in krilla
// src/configure/version.rs.
func (v PDFVersion) supportsICC(metadata iccMetadata) bool {
	var major, minor uint8
	switch v {
	case PDF14:
		major, minor = 2, 3
	case PDF15:
		major, minor = 4, 0
	case PDF16:
		major, minor = 4, 1
	case PDF17:
		major, minor = 4, 2
	default:
		major, minor = 4, 3
	}
	return metadata.major < major || (metadata.major == major && metadata.minor <= minor)
}

// supportsBitDepth is PdfVersion::supports_bit_depth in krilla
// src/configure/version.rs.
func (v PDFVersion) supportsBitDepth(bpc bitsPerComponent) bool {
	if bpc == bitsPerComponentSixteen {
		return v >= PDF15
	}
	return true
}

// registerImage is SerializeContext::register_image in krilla
// src/serialize.rs.
func (sc *serializeContext) registerImage(cc *chunkContainer, img *Image) pdfwriter.Ref {
	return sc.registerCached(img, func(sc *serializeContext, rootRef pdfwriter.Ref) {
		img.serialize(sc, cc, rootRef)
	})
}

// serialize is Image::serialize in krilla src/graphics/image/mod.rs. It
// adds the chunk of the image, or its error, to cc.streams.images.
func (i *Image) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	var softMaskID pdfwriter.Ref
	if i.metadata.hasAlpha {
		sc.registerValidationError(validationError{kind: validationTransparency, location: sc.location})
		softMaskID = sc.newRef()
	}

	var iccRef pdfwriter.Ref
	hasICCRef := false
	if ic := i.icc(); ic.profile != nil {
		if sc.serializeSettings().pdfVersion().supportsICC(ic.metadata()) &&
			i.colorSpace().matchesICCProfile(ic) {
			iccRef = sc.registerCacheable(cc, iccBasedColorSpace(ic))
			hasICCRef = true
		}
		// Don't embed ICC profiles from images if the current PDF version
		// does not support it.
	}

	if i.interpolate {
		sc.registerValidationError(validationError{kind: validationImageInterpolation, location: sc.location})
	}

	settings := sc.serializeSettings()

	var cs maybeDeviceColorSpace
	{
		var c colorSpace
		switch i.colorSpace() {
		case imageColorSpaceRGB:
			c = colorSpaceRGB(settings.noDeviceCS)
		case imageColorSpaceLuma:
			c = colorSpaceLuma(settings.noDeviceCS)
		default:
			var ok bool
			c, ok = colorSpaceCMYK(settings)
			if !ok {
				sc.registerValidationError(validationError{kind: validationMissingCMYKProfile})
				c = colorSpace{kind: colorSpaceDeviceCMYK}
			}
		}
		cs = sc.registerColorSpace(cc, c)
	}

	supportsBitDepth := settings.pdfVersion().supportsBitDepth(i.metadata.bitsPerComponent)
	location := sc.location
	chunk := sc.newChunk()

	err := i.writeChunk(chunk, settings, rootRef, softMaskID, iccRef, hasICCRef, cs, supportsBitDepth, location)
	cc.streams.images = append(cc.streams.images, chunkResult{chunk: chunk, err: err})
}

// writeChunk is the deferred chunk of Image::serialize in krilla
// src/graphics/image/mod.rs, which the port computes at once.
func (i *Image) writeChunk(
	chunk *pdfwriter.Chunk,
	settings *serializeSettings,
	rootRef, softMaskID, iccRef pdfwriter.Ref,
	hasICCRef bool,
	cs maybeDeviceColorSpace,
	supportsBitDepth bool,
	location Location,
) error {
	if !supportsBitDepth {
		return fmt.Errorf("%w%s", okage.ErrPDFSixteenBitImage, locationText(location))
	}

	if i.err != nil {
		return fmt.Errorf("%w%s: %w", okage.ErrPDFImage, locationText(location), i.err)
	}
	repr := i.inner

	width, height := i.Size()

	alphaMask := false
	if s, ok := repr.(*sampledRepr); ok && s.alphaChannel != nil {
		maskStream := newFilterStreamFromDeflated(s.alphaChannel).finish(settings)
		sMask := chunk.ImageXObject(softMaskID, maskStream.encodedData())
		maskStream.writeFilters(sMask.Dict)
		sMask.Width(int32(width))
		sMask.Height(int32(height))
		// Mask color space must be device gray -- see Table 145.
		sMask.Pair(pdfwriter.Name("ColorSpace"), pdfwriter.Name(deviceGray))

		if i.interpolate {
			sMask.Interpolate(true)
		}

		sMask.BitsPerComponent(int32(repr.bits()))
		sMask.Finish()
		alphaMask = true
	}

	var filterStream filterStream
	switch r := repr.(type) {
	case *sampledRepr:
		filterStream = newFilterStreamFromDeflated(r.colorChannel).finish(settings)
	case *jpegRepr:
		filterStream = newFilterStreamFromJPEGData(r.data).finish(settings)
	case *pngRepr:
		filterStream = newFilterStreamFromPNGData(
			r.data,
			r.colorType.samples(),
			int32(r.bitDepth),
			int32(width),
		).finish(settings)
	}

	imageXObject := chunk.ImageXObject(rootRef, filterStream.encodedData())
	filterStream.writeFilters(imageXObject.Dict)
	imageXObject.Width(int32(width))
	imageXObject.Height(int32(height))

	if p, ok := repr.(*pngRepr); ok && p.colorType == pngIndexed && p.hasPalette {
		setIndexedColorSpace(iccRef, hasICCRef, cs, p.palette, imageXObject.Dict)
	} else if hasICCRef {
		imageXObject.Pair(pdfwriter.Name("ColorSpace"), iccRef)
	} else {
		cs.write(imageXObject.Insert(pdfwriter.Name("ColorSpace")))
	}

	if i.interpolate {
		imageXObject.Interpolate(true)
	}

	// Photoshop CMYK images need to be inverted, see
	// https://github.com/sile-typesetter/libtexpdf/blob/1891bee5e0b73165e4a259f910d3ea3fe1df0b42/jpegimage.c#L25-L51
	// I'm not sure if this applies to all JPEG CMYK images out there, but for
	// now we just always do it. In libtexpdf, they only seem to do it if they
	// can find the Adobe APP marker.
	if j, ok := repr.(*jpegRepr); ok && j.invertCMYK {
		n := int(i.colorSpace().numComponents())
		decode := make([]float32, 0, 2*n)
		for range n {
			decode = append(decode, 1, 0)
		}
		imageXObject.Decode(decode...)
	}

	imageXObject.BitsPerComponent(int32(repr.bits()))
	if alphaMask {
		imageXObject.SMask(softMaskID)
	}
	imageXObject.Finish()

	return nil
}

// pngMetadata is png_metadata in krilla src/graphics/image/mod.rs. It also
// returns the information of the header, for decodePNG.
func pngMetadata(data []byte) (imageMetadata, pngInfo, error) {
	info, err := readPNGInfo(data)
	if err != nil {
		return imageMetadata{}, pngInfo{}, err
	}

	bpc := bitsPerComponentEight
	// We will normalize to 8 when decoding.
	if info.outputBitDepth == 16 {
		bpc = bitsPerComponentSixteen
	}

	var cs imageColorSpace
	var hasAlpha bool
	switch info.outputColorType {
	case pngGrayscale:
		cs, hasAlpha = imageColorSpaceLuma, false
	case pngGrayscaleAlpha:
		cs, hasAlpha = imageColorSpaceLuma, true
	case pngRGB:
		cs, hasAlpha = imageColorSpaceRGB, false
	case pngRGBA:
		cs, hasAlpha = imageColorSpaceRGB, true
	default:
		return imageMetadata{}, pngInfo{}, errors.New("image uses an indexed color space, which is unsupported")
	}
	var icc genericICCProfile
	if info.iccProfile != nil {
		icc = getICCProfileType(info.iccProfile, cs)
	}

	return imageMetadata{
		hasAlpha:         hasAlpha,
		width:            info.width,
		height:           info.height,
		bitsPerComponent: bpc,
		colorSpace:       cs,
		icc:              icc,
	}, info, nil
}

// decodePNG is decode_png in krilla src/graphics/image/mod.rs. image/png
// decodes the file in place of the crate png, and pngSamples gives the
// samples that the transformation EXPAND of the crate png gives.
func decodePNG(data []byte, info pngInfo) (imageRepr, error) {
	if pd, ok := newPNGData(data); ok {
		return pngReprFromData(pd), nil
	}

	imgData, err := decodePNGSamples(data, info)
	if err != nil {
		return nil, err
	}

	var cs zuneColorSpace
	switch info.outputColorType {
	case pngRGB:
		cs = zuneRGB
	case pngRGBA:
		cs = zuneRGBA
	case pngGrayscale:
		cs = zuneLuma
	default:
		cs = zuneLumaA
	}

	bpc := bitsPerComponentEight
	if info.outputBitDepth == 16 {
		bpc = bitsPerComponentSixteen
	}
	colorChannel, alphaChannel, bpc := handleImage(imgData, cs, bpc)

	return &sampledRepr{
		colorChannel:     colorChannel,
		alphaChannel:     alphaChannel,
		bitsPerComponent: bpc,
	}, nil
}

// setIndexedColorSpace is set_indexed_colorspace in krilla
// src/graphics/image/mod.rs.
func setIndexedColorSpace(iccRef pdfwriter.Ref, hasICCRef bool, base maybeDeviceColorSpace, palette []byte, target *pdfwriter.Dict) {
	colorSpace := target.Insert(pdfwriter.Name("ColorSpace")).Array()
	colorSpace.Item(pdfwriter.Name("Indexed"))

	if hasICCRef {
		colorSpace.Item(iccRef)
	} else {
		base.write(colorSpace.Push())
	}

	colorSpace.Item(pdfwriter.Int(int32(len(palette)/3 - 1)))
	colorSpace.Item(pdfwriter.Str(palette))
	colorSpace.Finish()
}

// jpegMetadata is jpeg_metadata in krilla src/graphics/image/mod.rs.
func jpegMetadata(data []byte) (imageMetadata, error) {
	headers, err := readJPEGHeaders(data)
	if err != nil {
		return imageMetadata{}, err
	}

	cs, ok := imageColorSpaceFromZune(headers.colorSpace)
	if !ok {
		return imageMetadata{}, errors.New("failed to read image colorspace")
	}

	var icc genericICCProfile
	if p := headers.iccProfile(); p != nil {
		icc = getICCProfileType(p, cs)
	}

	return imageMetadata{
		hasAlpha:         false,
		width:            uint32(headers.width),
		height:           uint32(headers.height),
		bitsPerComponent: bitsPerComponentEight,
		colorSpace:       cs,
		icc:              icc,
	}, nil
}

// decodeJPEG is decode_jpeg in krilla src/graphics/image/mod.rs. It reads
// the headers again, as upstream does, and keeps the data of the file.
func decodeJPEG(data []byte) (imageRepr, error) {
	headers, err := readJPEGHeaders(data)
	if err != nil {
		return nil, err
	}

	switch headers.colorSpace {
	case zuneLuma, zuneYCbCr, zuneRGB, zuneCMYK, zuneYCCK:
		return &jpegRepr{
			data:             data,
			bitsPerComponent: bitsPerComponentEight,
			invertCMYK:       headers.colorSpace == zuneYCCK || headers.colorSpace == zuneCMYK,
		}, nil
	}
	// JPEGs shouldn't be able to have a different color space?
	return nil, errors.New("image has an unknown color space")
}

// decodeGIF is decode_gif in krilla src/graphics/image/mod.rs. image/gif
// decodes the first frame in place of the crate gif.
func decodeGIF(data []byte) (imageRepr, error) {
	firstFrame, err := decodeGIFFrame(data)
	if err != nil {
		return nil, err
	}

	colorChannel, alphaChannel, bpc := handleImage(firstFrame, zuneRGBA, bitsPerComponentEight)

	return &sampledRepr{
		colorChannel:     colorChannel,
		alphaChannel:     alphaChannel,
		bitsPerComponent: bpc,
	}, nil
}

// gifMetadata is gif_metadata in krilla src/graphics/image/mod.rs. The size
// is the size of the logical screen of the file, which upstream reads with
// the crate imagesize.
func gifMetadata(data []byte) (imageMetadata, error) {
	width, height, err := gifSize(data)
	if err != nil {
		return imageMetadata{}, err
	}

	return imageMetadata{
		// We always decode GIFs using RGBA, see decodeGIF.
		hasAlpha:         true,
		bitsPerComponent: bitsPerComponentEight,
		width:            width,
		height:           height,
		colorSpace:       imageColorSpaceRGB,
	}, nil
}

// webpMetadata is webp_metadata in krilla src/graphics/image/mod.rs.
func webpMetadata(data []byte) (imageMetadata, error) {
	info, err := readWebPInfo(data)
	if err != nil {
		return imageMetadata{}, err
	}
	cs := imageColorSpaceRGB
	var icc genericICCProfile
	if info.iccProfile != nil {
		icc = getICCProfileType(info.iccProfile, cs)
	}

	return imageMetadata{
		hasAlpha:         info.hasAlpha,
		bitsPerComponent: bitsPerComponentEight,
		width:            info.width,
		height:           info.height,
		colorSpace:       cs,
		icc:              icc,
	}, nil
}

// decodeWebP is decode_webp in krilla src/graphics/image/mod.rs.
// golang.org/x/image/webp decodes the image in place of the crate
// image-webp. hasAlpha is has_alpha of the decoder of upstream, which the
// metadata holds.
func decodeWebP(data []byte, hasAlpha bool) (imageRepr, error) {
	firstFrame, err := decodeWebPSamples(data, hasAlpha)
	if err != nil {
		return nil, err
	}

	cs := zuneRGB
	if hasAlpha {
		cs = zuneRGBA
	}

	colorChannel, alphaChannel, bpc := handleImage(firstFrame, cs, bitsPerComponentEight)

	return &sampledRepr{
		colorChannel:     colorChannel,
		alphaChannel:     alphaChannel,
		bitsPerComponent: bpc,
	}, nil
}

// handleImage is handle_image in krilla src/graphics/image/mod.rs. It
// deflates the samples of the colors and, when an alpha is not 255, of the
// alpha. upstream panics for another color space, which no caller gives.
func handleImage(data []byte, cs zuneColorSpace, bpc bitsPerComponent) (colorChannel, alphaChannel []byte, _ bitsPerComponent) {
	switch {
	case cs == zuneRGB || cs == zuneLuma:
		colorChannel = deflateEncode(data)
	case cs == zuneRGBA && bpc == bitsPerComponentEight:
		colorChannel, alphaChannel = deflateImageChannels(data, 3, 1)
	case cs == zuneRGBA:
		colorChannel, alphaChannel = deflateImageChannels(data, 6, 2)
	case cs == zuneLumaA && bpc == bitsPerComponentEight:
		colorChannel, alphaChannel = deflateImageChannels(data, 1, 1)
	default:
		colorChannel, alphaChannel = deflateImageChannels(data, 2, 2)
	}

	return colorChannel, alphaChannel, bpc
}

// imageChannelEncoder is ImageChannelEncoder in krilla
// src/graphics/image/mod.rs.
type imageChannelEncoder struct {
	out     bytes.Buffer
	encoder interface {
		Write(p []byte) (int, error)
		Close() error
	}
	buffer []byte
}

// newImageChannelEncoder is ImageChannelEncoder::new in krilla
// src/graphics/image/mod.rs.
func newImageChannelEncoder(capacity int) *imageChannelEncoder {
	e := &imageChannelEncoder{buffer: make([]byte, 0, capacity)}
	e.encoder = deflateEncoder(&e.out)
	return e
}

// encodeBatch is ImageChannelEncoder::encode_batch in krilla
// src/graphics/image/mod.rs.
func (e *imageChannelEncoder) encodeBatch() {
	// A write to a bytes.Buffer cannot fail.
	_, _ = e.encoder.Write(e.buffer)
	e.buffer = e.buffer[:0]
}

// finish is ImageChannelEncoder::finish in krilla
// src/graphics/image/mod.rs.
func (e *imageChannelEncoder) finish() []byte {
	_ = e.encoder.Close()
	return e.out.Bytes()
}

// deflateImageChannels is deflate_image_channels in krilla
// src/graphics/image/mod.rs, with the constants COLOR_BYTES and
// ALPHA_BYTES of upstream as arguments. It splits the pixels into the color
// samples and the alpha samples, and deflates each. The alpha is nil when
// each byte of it is 255. Like upstream, it leaves out the bytes of an
// incomplete pixel at the end.
func deflateImageChannels(data []byte, colorBytes, alphaBytes int) (colorChannel, alphaChannel []byte) {
	pixelBytes := colorBytes + alphaBytes

	hasMask := false
	for p := 0; p+pixelBytes <= len(data) && !hasMask; p += pixelBytes {
		for _, v := range data[p+colorBytes : p+pixelBytes] {
			if v != 255 {
				hasMask = true
				break
			}
		}
	}

	// batchBytes is a multiple of each pixel size, 2, 4 and 8.
	const batchBytes = 16 * 1024

	colors := newImageChannelEncoder(batchBytes / pixelBytes * colorBytes)
	var alphas *imageChannelEncoder
	if hasMask {
		alphas = newImageChannelEncoder(batchBytes / pixelBytes * alphaBytes)
	}

	for start := 0; start < len(data); start += batchBytes {
		batch := data[start:min(start+batchBytes, len(data))]
		for p := 0; p+pixelBytes <= len(batch); p += pixelBytes {
			colors.buffer = append(colors.buffer, batch[p:p+colorBytes]...)
			if alphas != nil {
				alphas.buffer = append(alphas.buffer, batch[p+colorBytes:p+pixelBytes]...)
			}
		}

		colors.encodeBatch()
		if alphas != nil {
			alphas.encodeBatch()
		}
	}

	colorChannel = colors.finish()
	if alphas != nil {
		alphaChannel = alphas.finish()
	}
	return colorChannel, alphaChannel
}

// getICCProfileType is get_icc_profile_type in krilla
// src/graphics/image/mod.rs. The profile is the zero value, None, when the
// data is not a profile of the color space.
func getICCProfileType(data []byte, cs imageColorSpace) genericICCProfile {
	p, ok := NewICCProfile(data, cs.numComponents())
	if !ok {
		return genericICCProfile{}
	}
	return genericICCProfile{profile: p}
}
