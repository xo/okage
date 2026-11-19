// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"math/bits"
	"slices"

	"golang.org/x/image/webp"
)

// This file is not a port. It holds the code that takes the place of the
// crates png, zune-jpeg, gif, image-webp and imagesize in
// graphics_image.go (D7, D62). The decoders of Go do not give everything
// that krilla reads from these crates, such as an ICC profile or the color
// space of a JPEG file. So this file reads the headers of each format as
// the crate of upstream does, and gives the samples in the layout that the
// crate gives.

// maxImageBytes is the largest number of bytes of the samples of a decoded
// image. Above it, pdf returns an error, so that a small file with a large
// size in its header cannot allocate all the memory. The samples are the
// samples of the output buffer of the crate of upstream. 256 MiB hold an RGBA
// image of 8 bits with 64 million pixels.
const maxImageBytes = 1 << 28

// maxICCBytes is the largest size of an ICC profile that a PNG file holds
// compressed. It is the default limit of the crate png, 64 MiB.
const maxICCBytes = 64 << 20

// checkImageBytes returns an error when an image of the size, with the
// bytes for each pixel, needs more than maxImageBytes bytes.
func checkImageBytes(width, height int, pixelBytes uint64) error {
	if width < 0 || height < 0 {
		return errors.New("image has a negative size")
	}
	hi, n := bits.Mul64(uint64(width), uint64(height))
	hi2, n := bits.Mul64(n, pixelBytes)
	if hi != 0 || hi2 != 0 || n > maxImageBytes {
		return fmt.Errorf("image is too large: %d by %d pixels need more than %d bytes", width, height, maxImageBytes)
	}
	return nil
}

// pngInfo is the information of the header of a PNG file, as read_info of
// the crate png gives it with the transformation EXPAND. The output color
// type and bit depth are the layout of the samples that the crate gives.
type pngInfo struct {
	width           uint32
	height          uint32
	outputColorType pngColorType
	outputBitDepth  uint8
	// colorType, bitDepth and interlaced are the fields of the chunk IHDR.
	colorType  pngColorType
	bitDepth   uint8
	interlaced bool
	// iccProfile is the decompressed data of the chunk iCCP, or nil.
	iccProfile []byte
}

// readPNGInfo reads the header of a PNG file, as read_info of the crate png
// does. png.DecodeConfig checks the signature and the chunk IHDR. Then it
// reads the chunks up to the start of the first chunk IDAT, for the chunks
// tRNS and iCCP. A chunk iCCP that cannot be read is ignored, and a second one is an
// error, as the crate png does.
func readPNGInfo(data []byte) (pngInfo, error) {
	if _, err := png.DecodeConfig(bytes.NewReader(data)); err != nil {
		return pngInfo{}, err
	}
	r := pngReader{data: data[len(pngMagic):]}
	var info pngInfo
	var colorType pngColorType
	var bitDepth uint8
	hasTRNS, hasICCP, hasIHDR := false, false, false
	for {
		length, ok1 := r.readU32()
		kind, ok2 := r.read(4)
		if !ok1 || !ok2 {
			return pngInfo{}, errors.New("unexpected end of file before image data")
		}
		k := pngChunkType(kind)
		// The crate png stops at the start of the first chunk IDAT.
		if k == pngChunkIDAT {
			break
		}
		data, ok1 := r.read(int(length))
		_, ok2 = r.readU32()
		if !ok1 || !ok2 {
			return pngInfo{}, errPNGChunkTooShort
		}
		switch k {
		case pngChunkIHDR:
			if hasIHDR || len(data) != 13 {
				return pngInfo{}, errors.New("invalid chunk IHDR")
			}
			hasIHDR = true
			info.width = binary.BigEndian.Uint32(data[0:4])
			info.height = binary.BigEndian.Uint32(data[4:8])
			bitDepth = data[8]
			colorType = pngColorType(data[9])
			info.interlaced = data[12] == 1
		case pngChunkTRNS:
			hasTRNS = true
		case pngChunkICCP:
			if hasICCP {
				return pngInfo{}, errors.New("duplicate chunk iCCP")
			}
			hasICCP = true
			info.iccProfile = readICCP(data)
		}
	}

	info.colorType, info.bitDepth = colorType, bitDepth
	info.outputBitDepth = 8
	if bitDepth == 16 {
		info.outputBitDepth = 16
	}
	info.outputColorType = colorType
	switch colorType {
	case pngIndexed:
		info.outputColorType = pngRGB
		if hasTRNS {
			info.outputColorType = pngRGBA
		}
	case pngGrayscale:
		if hasTRNS {
			info.outputColorType = pngGrayscaleAlpha
		}
	case pngRGB:
		if hasTRNS {
			info.outputColorType = pngRGBA
		}
	}
	return info, nil
}

// readICCP reads the profile of a chunk iCCP: a name of 1 to 79 bytes, the
// byte 0, the compression method 0 and the data in zlib. It returns nil
// when it cannot read the profile.
func readICCP(data []byte) []byte {
	n := bytes.IndexByte(data, 0)
	if n < 1 || n > 79 || n+1 >= len(data) || data[n+1] != 0 {
		return nil
	}
	r, err := zlib.NewReader(bytes.NewReader(data[n+2:]))
	if err != nil {
		return nil
	}
	profile, err := io.ReadAll(io.LimitReader(r, maxICCBytes+1))
	if err != nil || len(profile) > maxICCBytes {
		return nil
	}
	return profile
}

// decodePNGSamples decodes a PNG file with image/png, and gives the samples
// of the output buffer of the crate png with the transformation EXPAND:
// each pixel holds the channels of the output color type, with 8 or 16
// bits each, in big-endian order.
func decodePNGSamples(data []byte, info pngInfo) ([]byte, error) {
	channels := int(info.outputColorType.samples())
	sixteen := info.outputBitDepth == 16
	pixelBytes := uint64(channels)
	if sixteen {
		pixelBytes *= 2
	}
	// image/png holds a gray image with an alpha in an RGBA image, so the
	// bound counts four channels.
	if err := checkImageBytes(int(info.width), int(info.height), 4*pixelBytes/uint64(channels)); err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(pngThroughIDAT(data)))
	if err != nil {
		// The crate png ignores the checksum of the zlib stream and the data
		// after the samples. image/png does not, so it decodes a repaired
		// copy.
		repaired, ok := repairPNG(data, info)
		if !ok {
			return nil, err
		}
		if img, err = png.Decode(bytes.NewReader(repaired)); err != nil {
			return nil, err
		}
	}
	return interleave(img, channels, sixteen), nil
}

// pngRawSize is the number of bytes of the filtered rows of a PNG image,
// with the passes of Adam7 when it is interlaced.
func pngRawSize(info pngInfo) uint64 {
	bitsPerPixel := uint64(info.colorType.samples()) * uint64(info.bitDepth)
	rows := func(w, h uint64) uint64 {
		if w == 0 || h == 0 {
			return 0
		}
		return h * (1 + (w*bitsPerPixel+7)/8)
	}
	w, h := uint64(info.width), uint64(info.height)
	if !info.interlaced {
		return rows(w, h)
	}
	var n uint64
	for _, p := range [7][4]uint64{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}} {
		var pw, ph uint64
		if w > p[0] {
			pw = (w - p[0] + p[2] - 1) / p[2]
		}
		if h > p[1] {
			ph = (h - p[1] + p[3] - 1) / p[3]
		}
		n += rows(pw, ph)
	}
	return n
}

// repairPNG returns a copy of a PNG file whose image data holds only the
// filtered rows, in a zlib stream with a valid checksum. It reads the
// chunks IDAT as the crate png does: it ignores the checksum of their zlib
// stream, and the data after the rows. It returns false when the stream
// does not hold all the rows.
func repairPNG(data []byte, info pngInfo) ([]byte, bool) {
	r := pngReader{data: data[min(len(pngMagic), len(data)):]}
	var head, idat []byte
	for {
		start := len(data) - len(r.data)
		length, ok1 := r.readU32()
		kind, ok2 := r.read(4)
		if !ok1 || !ok2 {
			return nil, false
		}
		k := pngChunkType(kind)
		if head != nil && k != pngChunkIDAT {
			break
		}
		chunk, ok := r.read(int(length))
		if !ok {
			return nil, false
		}
		if _, ok := r.readU32(); !ok {
			return nil, false
		}
		if k == pngChunkIDAT {
			if head == nil {
				head = data[:start:start]
			}
			idat = append(idat, chunk...)
		}
	}
	// The header of the zlib stream: the method deflate and a valid check.
	if len(idat) < 2 || idat[0]&0x0f != 8 || (uint16(idat[0])<<8|uint16(idat[1]))%31 != 0 || idat[1]&0x20 != 0 {
		return nil, false
	}
	size := pngRawSize(info)
	if size > 2*maxImageBytes {
		return nil, false
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(flate.NewReader(bytes.NewReader(idat[2:])), raw); err != nil {
		return nil, false
	}
	var z bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&z, zlib.BestSpeed)
	// A write to a bytes.Buffer cannot fail.
	_, _ = zw.Write(raw)
	_ = zw.Close()
	out := append([]byte{}, head...)
	out = binary.BigEndian.AppendUint32(out, uint32(z.Len()))
	chunkStart := len(out)
	out = append(out, "IDAT"...)
	out = append(out, z.Bytes()...)
	out = binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[chunkStart:]))
	return append(out, pngIEND...), true
}

// pngIEND is a chunk IEND with its CRC.
const pngIEND = "\x00\x00\x00\x00IEND\xae\x42\x60\x82"

// pngThroughIDAT returns the chunks of a PNG file up to the end of its
// chunks IDAT, and a chunk IEND. The crate png stops after the image data,
// and image/png reads the chunks up to IEND. So a fault after the image
// data, such as an IEND that is cut off, stops image/png and not the crate
// png. When the chunks IDAT are not complete, or the file ends without the
// header of a chunk after them, it returns the data as it is, and both
// decoders fail.
func pngThroughIDAT(data []byte) []byte {
	r := pngReader{data: data[min(len(pngMagic), len(data)):]}
	seen := false
	for {
		start := len(data) - len(r.data)
		length, ok1 := r.readU32()
		kind, ok2 := r.read(4)
		if ok1 && ok2 && seen && pngChunkType(kind) != pngChunkIDAT {
			return append(data[:start:start], pngIEND...)
		}
		// The crate png needs the header of the chunk after the image data.
		if !ok1 || !ok2 {
			return data
		}
		if _, ok := r.read(int(length)); !ok {
			return data
		}
		if _, ok := r.readU32(); !ok {
			return data
		}
		if pngChunkType(kind) == pngChunkIDAT {
			seen = true
		}
	}
}

// interleave gives the samples of an image with 1, 2, 3 or 4 channels:
// gray, gray and alpha, RGB or RGBA, not premultiplied, with 8 or 16 bits
// each in big-endian order. The gray of a color is its red channel, which
// is exact for the images of image/png that come from a gray image.
func interleave(img image.Image, channels int, sixteen bool) []byte {
	b := img.Bounds()
	n := b.Dx() * b.Dy() * channels
	if sixteen {
		n *= 2
	}
	out := make([]byte, 0, n)
	if !sixteen {
		if g, ok := img.(*image.Gray); ok && channels == 1 {
			return rowsOf(g.Pix, g.Stride, b.Dx(), b.Dy())
		}
		at := nrgbaFunc(img)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := at(x, y)
				switch channels {
				case 1:
					out = append(out, c.R)
				case 2:
					out = append(out, c.R, c.A)
				case 3:
					out = append(out, c.R, c.G, c.B)
				default:
					out = append(out, c.R, c.G, c.B, c.A)
				}
			}
		}
		return out
	}
	if g, ok := img.(*image.Gray16); ok && channels == 1 {
		return rowsOf(g.Pix, g.Stride, 2*b.Dx(), b.Dy())
	}
	at := nrgba64Func(img)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := at(x, y)
			switch channels {
			case 1:
				out = append(out, uint8(c.R>>8), uint8(c.R))
			case 2:
				out = append(out, uint8(c.R>>8), uint8(c.R), uint8(c.A>>8), uint8(c.A))
			case 3:
				out = append(out, uint8(c.R>>8), uint8(c.R), uint8(c.G>>8), uint8(c.G), uint8(c.B>>8), uint8(c.B))
			default:
				out = append(out, uint8(c.R>>8), uint8(c.R), uint8(c.G>>8), uint8(c.G), uint8(c.B>>8), uint8(c.B),
					uint8(c.A>>8), uint8(c.A))
			}
		}
	}
	return out
}

// rowsOf copies the rows of the pixels of an image, n bytes each.
func rowsOf(pix []byte, stride, n, rows int) []byte {
	out := make([]byte, 0, n*rows)
	for y := range rows {
		out = append(out, pix[y*stride:y*stride+n]...)
	}
	return out
}

// splitChannels splits interleaved samples into the color samples and the
// alpha samples.
func splitChannels(data []byte, colorBytes, alphaBytes int) (colorChannel, alphaChannel []byte) {
	pixelBytes := colorBytes + alphaBytes
	n := len(data) / pixelBytes
	colorChannel = make([]byte, 0, n*colorBytes)
	alphaChannel = make([]byte, 0, n*alphaBytes)
	for p := 0; p+pixelBytes <= len(data); p += pixelBytes {
		colorChannel = append(colorChannel, data[p:p+colorBytes]...)
		alphaChannel = append(alphaChannel, data[p+colorBytes:p+pixelBytes]...)
	}
	return colorChannel, alphaChannel
}

// nrgbaFunc returns a function that gives the color of a pixel of the image
// with 8 bits for each channel, not premultiplied. A model of image/color
// gives a color of its own type, so its type assertions do not fail.
func nrgbaFunc(img image.Image) func(x, y int) color.NRGBA {
	switch m := img.(type) {
	case *image.NRGBA:
		return m.NRGBAAt
	case *image.Gray:
		return func(x, y int) color.NRGBA {
			g := m.GrayAt(x, y).Y
			return color.NRGBA{g, g, g, 0xff}
		}
	case *image.RGBA:
		return func(x, y int) color.NRGBA {
			c := m.RGBAAt(x, y)
			if c.A == 0xff {
				return color.NRGBA(c)
			}
			n, _ := color.NRGBAModel.Convert(c).(color.NRGBA)
			return n
		}
	case *image.Paletted:
		var pal [256]color.NRGBA
		for i := range pal {
			pal[i] = color.NRGBA{0, 0, 0, 0xff}
		}
		for i, c := range m.Palette {
			if i < len(pal) {
				pal[i], _ = color.NRGBAModel.Convert(c).(color.NRGBA)
			}
		}
		return func(x, y int) color.NRGBA {
			return pal[m.ColorIndexAt(x, y)]
		}
	}
	return func(x, y int) color.NRGBA {
		n, _ := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
		return n
	}
}

// nrgba64Func returns a function that gives the color of a pixel of the
// image with 16 bits for each channel, not premultiplied.
func nrgba64Func(img image.Image) func(x, y int) color.NRGBA64 {
	switch m := img.(type) {
	case *image.NRGBA64:
		return m.NRGBA64At
	case *image.Gray16:
		return func(x, y int) color.NRGBA64 {
			g := m.Gray16At(x, y).Y
			return color.NRGBA64{g, g, g, 0xffff}
		}
	case *image.RGBA64:
		return func(x, y int) color.NRGBA64 {
			c := m.RGBA64At(x, y)
			if c.A == 0xffff {
				return color.NRGBA64(c)
			}
			n, _ := color.NRGBA64Model.Convert(c).(color.NRGBA64)
			return n
		}
	}
	return func(x, y int) color.NRGBA64 {
		n, _ := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
		return n
	}
}

// gray16Func returns a function that gives the gray of a pixel of the
// image with 16 bits.
func gray16Func(img image.Image) func(x, y int) uint16 {
	return func(x, y int) uint16 {
		g, _ := color.Gray16Model.Convert(img.At(x, y)).(color.Gray16)
		return g.Y
	}
}

// isOpaque reports whether each pixel of the image has the alpha 1.
func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

// jpegMaxDimension is the largest width and height of a JPEG image, the
// default of the options of zune-jpeg.
const jpegMaxDimension = 1 << 14

// jpegICCChunk is ICCChunk of zune-jpeg: one segment APP2 of an ICC
// profile.
type jpegICCChunk struct {
	seqNo      uint8
	numMarkers uint8
	data       []byte
}

// jpegHeaders is what decode_headers of zune-jpeg gives to krilla: the
// size, the color space of the input and the segments of the ICC profile.
type jpegHeaders struct {
	width      uint16
	height     uint16
	colorSpace zuneColorSpace
	icc        []jpegICCChunk
}

// jpegComponent is a component of the segment SOF of a JPEG file.
type jpegComponent struct {
	id uint8
}

// jpegReader reads the bytes of a JPEG file.
type jpegReader struct {
	data []byte
}

var errJPEGEnd = errors.New("no more bytes")

func (r *jpegReader) u8() (uint8, error) {
	if len(r.data) == 0 {
		return 0, errJPEGEnd
	}
	v := r.data[0]
	r.data = r.data[1:]
	return v, nil
}

func (r *jpegReader) u16() (uint16, error) {
	if len(r.data) < 2 {
		return 0, errJPEGEnd
	}
	v := binary.BigEndian.Uint16(r.data)
	r.data = r.data[2:]
	return v, nil
}

func (r *jpegReader) bytes(n int) ([]byte, error) {
	if n < 0 || n > len(r.data) {
		return nil, errJPEGEnd
	}
	b := r.data[:n]
	r.data = r.data[n:]
	return b, nil
}

// readJPEGHeaders reads the headers of a JPEG file up to the segment SOS,
// as decode_headers of zune-jpeg 0.5 does for krilla. It reads the segments
// SOF0, SOF1 and SOF2, APP2 with an ICC profile, APP14 of Adobe, DRI and
// SOS, and skips the other segments. Unlike zune-jpeg, it does not read the
// tables of the segments DQT and DHT, because the PDF file holds the data of
// the file as it is.
func readJPEGHeaders(data []byte) (jpegHeaders, error) {
	r := &jpegReader{data: data}
	h := jpegHeaders{colorSpace: zuneYCbCr}
	if magic, err := r.u16(); err != nil || magic != 0xffd8 {
		return jpegHeaders{}, errors.New("illegal magic bytes")
	}
	var components []jpegComponent
	seenSOF := false
	var lastByte uint8
	for {
		m, err := r.u8()
		if err != nil {
			return jpegHeaders{}, err
		}
		// Some images have fill bytes or byte stuffing before a marker.
		if (m == 0xff || m == 0) && lastByte == 0xff {
			for m == 0xff || m == 0 {
				lastByte = m
				if m, err = r.u8(); err != nil {
					return jpegHeaders{}, err
				}
			}
		}
		if lastByte == 0xff {
			switch m {
			case 0xc0, 0xc1, 0xc2:
				if seenSOF {
					return jpegHeaders{}, errors.New("two start of frame markers")
				}
				components, err = h.readSOF(r)
				if err != nil {
					return jpegHeaders{}, err
				}
				seenSOF = true
			case 0xe2:
				if err := h.readAPP2(r); err != nil {
					return jpegHeaders{}, err
				}
			case 0xee:
				if err := h.readAPP14(r); err != nil {
					return jpegHeaders{}, err
				}
			case 0xcc, 0xdc:
				return jpegHeaders{}, fmt.Errorf("parsing of the marker 0xFF%X is not supported", m)
			case 0xd9:
				return jpegHeaders{}, errors.New("premature end of image")
			case 0xdd:
				if n, err := r.u16(); err != nil || n != 4 {
					return jpegHeaders{}, errors.New("bad DRI length, corrupt JPEG")
				}
				if _, err := r.u16(); err != nil {
					return jpegHeaders{}, err
				}
			case 0xda:
				if err := readSOS(r, components); err != nil {
					return jpegHeaders{}, err
				}
				// An image whose components have the IDs R, G and B is RGB.
				if len(components) == 3 && components[0].id == 'R' && components[1].id == 'G' && components[2].id == 'B' {
					h.colorSpace = zuneRGB
				}
				return h, nil
			default:
				n, err := r.u16()
				if err != nil {
					return jpegHeaders{}, err
				}
				if n < 2 {
					return jpegHeaders{}, fmt.Errorf("found a marker with invalid length: %d", n)
				}
				if _, err := r.bytes(int(n) - 2); err != nil {
					return jpegHeaders{}, err
				}
			}
		}
		lastByte = m
	}
}

// readSOF reads a segment SOF0, SOF1 or SOF2, as parse_start_of_frame of
// zune-jpeg does.
func (h *jpegHeaders) readSOF(r *jpegReader) ([]jpegComponent, error) {
	length, err := r.u16()
	if err != nil {
		return nil, err
	}
	precision, err := r.u8()
	if err != nil {
		return nil, err
	}
	if precision != 8 {
		return nil, fmt.Errorf("the library can only parse 8-bit images, the image has %d bits of precision", precision)
	}
	if h.height, err = r.u16(); err != nil {
		return nil, err
	}
	if h.width, err = r.u16(); err != nil {
		return nil, err
	}
	if h.width > jpegMaxDimension || h.height > jpegMaxDimension {
		return nil, fmt.Errorf("image size %d by %d greater than the limit %d", h.width, h.height, jpegMaxDimension)
	}
	if h.width == 0 || h.height == 0 {
		return nil, errors.New("image width or height is zero")
	}
	n, err := r.u8()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, errors.New("number of components cannot be zero")
	}
	if want := 8 + 3*uint16(n); length != want {
		return nil, fmt.Errorf("length of start of frame differs from expected %d, value is %d", want, length)
	}
	if n == 1 {
		h.colorSpace = zuneLuma
	}
	if n == 4 && h.colorSpace == zuneYCbCr {
		h.colorSpace = zuneCMYK
	}
	components := make([]jpegComponent, 0, n)
	for pos := range n {
		c, err := r.bytes(3)
		if err != nil {
			return nil, err
		}
		if pos > 3 {
			return nil, fmt.Errorf("unknown component id found, %d, expected value between 1 and 4", pos)
		}
		hs, vs := c[1]>>4, c[1]&0x0f
		if hs > 4 || vs > 4 {
			return nil, errors.New("bogus sampling factor")
		}
		if c[2] >= 4 {
			return nil, fmt.Errorf("too large quantization number: %d", c[2])
		}
		if hs == 0 || hs&(hs-1) != 0 || vs == 0 {
			return nil, errors.New("bogus sampling factor")
		}
		components = append(components, jpegComponent{id: c[0]})
	}
	return components, nil
}

// readAPP2 reads a segment APP2, as parse_app2 of zune-jpeg does for an ICC
// profile.
func (h *jpegHeaders) readAPP2(r *jpegReader) error {
	length, err := r.u16()
	if err != nil {
		return err
	}
	if length < 2 {
		return errors.New("too small app2 segment")
	}
	seg, err := r.bytes(int(length) - 2)
	if err != nil {
		return err
	}
	if len(seg) > 14 && string(seg[:12]) == "ICC_PROFILE\x00" {
		h.icc = append(h.icc, jpegICCChunk{seqNo: seg[12], numMarkers: seg[13], data: seg[14:]})
	}
	return nil
}

// readAPP14 reads a segment APP14, as parse_app14 of zune-jpeg does for the
// color transform of Adobe.
func (h *jpegHeaders) readAPP14(r *jpegReader) error {
	length, err := r.u16()
	if err != nil {
		return err
	}
	if length < 2 {
		return errors.New("too small APP14 length")
	}
	if len(r.data) < 5 {
		return errJPEGEnd
	}
	if string(r.data[:5]) != "Adobe" {
		_, err := r.bytes(int(length) - 2)
		return err
	}
	if length < 14 {
		return errors.New("too short of a length for App14 segment")
	}
	seg, err := r.bytes(12)
	if err != nil {
		return err
	}
	switch transform := seg[11]; transform {
	case 0:
		h.colorSpace = zuneCMYK
	case 1:
		h.colorSpace = zuneYCbCr
	case 2:
		h.colorSpace = zuneYCCK
	default:
		return fmt.Errorf("unknown Adobe colorspace %d", transform)
	}
	_, err = r.bytes(int(length) - 14)
	return err
}

// readSOS reads the start of a segment SOS, as parse_sos of zune-jpeg does:
// the length and the components of the scan.
func readSOS(r *jpegReader, components []jpegComponent) error {
	ls, err := r.u16()
	if err != nil {
		return err
	}
	ns, err := r.u8()
	if err != nil {
		return err
	}
	if int(ls) != 6+2*int(ns) {
		return fmt.Errorf("bad SOS length %d, corrupt jpeg", ls)
	}
	if ns < 1 || ns > 4 {
		return fmt.Errorf("invalid number of components in start of scan %d, expected in range 1..5", ns)
	}
	if len(components) == 0 {
		return errors.New("error decoding SOF marker, number of components cannot be zero")
	}
	var seen []uint8
	for range ns {
		c, err := r.bytes(2)
		if err != nil {
			return err
		}
		if slices.Contains(seen, c[0]) {
			return fmt.Errorf("duplicate ID %d seen twice in the same component", c[0])
		}
		seen = append(seen, c[0])
		if !slices.ContainsFunc(components, func(comp jpegComponent) bool { return comp.id == c[0] }) {
			return fmt.Errorf("invalid component id %d", c[0])
		}
	}
	return nil
}

// iccProfile is JpegDecoder::icc_profile of zune-jpeg. It joins the
// segments of the profile in the order of their numbers, and returns nil
// when a segment is missing or repeats.
func (h *jpegHeaders) iccProfile() []byte {
	var present [256]*jpegICCChunk
	n := len(h.icc)
	if n == 0 || n >= 255 {
		return nil
	}
	for i := range h.icc {
		c := &h.icc[i]
		if int(c.numMarkers) != n || c.seqNo == 0 || present[c.seqNo] != nil {
			return nil
		}
		present[c.seqNo] = c
	}
	var data []byte
	for _, c := range present[1 : n+1] {
		if c == nil {
			return nil
		}
		data = append(data, c.data...)
	}
	return data
}

// gifSize reads the size of the logical screen of a GIF file, which
// blob_size of the crate imagesize gives. image/gif reads it.
func gifSize(data []byte) (width, height uint32, _ error) {
	cfg, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return uint32(cfg.Width), uint32(cfg.Height), nil
}

// decodeGIFFrame decodes the first frame of a GIF file with image/gif, and
// gives its samples in RGBA, as the crate gif gives them with
// ColorOutput::RGBA. The frame has its own size, which can be smaller than
// the logical screen. image/gif gives a transparent pixel the color 0, 0, 0,
// where the crate gif gives it the color of its palette entry, with the
// alpha 0.
func decodeGIFFrame(data []byte) ([]byte, error) {
	cfg, err := gif.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := checkImageBytes(cfg.Width, cfg.Height, 4); err != nil {
		return nil, err
	}
	img, err := gif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return interleave(img, 4, false), nil
}

// webpInfo is what the decoder of image-webp gives to krilla: the size,
// whether the image has an alpha channel, and the ICC profile.
type webpInfo struct {
	width      uint32
	height     uint32
	hasAlpha   bool
	iccProfile []byte
}

// readWebPInfo reads the header of a WebP file, as WebPDecoder::new of
// image-webp does. A lossy image has no alpha. A lossless image has the
// alpha of the bit alpha_is_used of its header. An extended image has the
// alpha of its flags, and the profile of its chunk ICCP.
// golang.org/x/image/webp gives the size and checks the header.
func readWebPInfo(data []byte) (webpInfo, error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return webpInfo{}, err
	}
	info := webpInfo{width: uint32(cfg.Width), height: uint32(cfg.Height)}
	if len(data) < 20 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		return webpInfo{}, errors.New("invalid WebP header")
	}
	riffEnd := min(uint64(binary.LittleEndian.Uint32(data[4:8]))+8, uint64(len(data)))
	pos := uint64(12)
	first := true
	for pos+8 <= riffEnd {
		kind := string(data[pos : pos+4])
		size := uint64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		start := pos + 8
		end := min(start+size, riffEnd)
		chunk := data[start:end]
		switch {
		case first && kind == "VP8L":
			if len(chunk) >= 5 {
				info.hasAlpha = (binary.LittleEndian.Uint32(chunk[1:5])>>28)&1 != 0
			}
			return info, nil
		case first && kind == "VP8X":
			if len(chunk) >= 1 {
				info.hasAlpha = chunk[0]&0x10 != 0
			}
		case first:
			return info, nil
		case kind == "ICCP" && info.iccProfile == nil:
			info.iccProfile = append([]byte{}, chunk...)
		}
		first = false
		pos = start + size + size&1
	}
	return info, nil
}

// decodeWebPSamples decodes a WebP file with golang.org/x/image/webp, and
// gives its samples in RGBA when it has an alpha channel, and in RGB when
// not, as read_image of image-webp does. A lossy image converts its planes
// to RGB as image-webp does, with webpFancyRGB.
func decodeWebPSamples(data []byte, hasAlpha bool) ([]byte, error) {
	cfg, err := webp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// The decoder holds an alpha plane and the planes of YCbCr, or an RGBA
	// image.
	if err := checkImageBytes(cfg.Width, cfg.Height, 4); err != nil {
		return nil, err
	}
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	channels := 3
	if hasAlpha {
		channels = 4
	}
	switch m := img.(type) {
	case *image.YCbCr:
		if m.SubsampleRatio == image.YCbCrSubsampleRatio420 {
			return webpFancyRGB(m, nil, 0, channels), nil
		}
	case *image.NYCbCrA:
		if m.SubsampleRatio == image.YCbCrSubsampleRatio420 {
			return webpFancyRGB(&m.YCbCr, m.A, m.AStride, channels), nil
		}
	}
	return interleave(img, channels, false), nil
}

// webpFancyRGB converts the planes of a lossy WebP image to RGB or RGBA, as
// fill_rgb_buffer_fancy of image-webp src/yuv.rs does. It interpolates the
// chroma between the 4 nearest samples, and converts YUV to RGB with the
// integer formulas of libwebp. A nil alpha is 255. The image of
// golang.org/x/image/webp holds the planes that the decoder of image-webp
// holds, so the samples are the samples of image-webp.
func webpFancyRGB(m *image.YCbCr, alpha []byte, alphaStride, channels int) []byte {
	w, h := m.Rect.Dx(), m.Rect.Dy()
	out := make([]byte, w*h*channels)
	cw := (w + 1) / 2
	yRow := func(y int) []byte {
		o := m.YOffset(m.Rect.Min.X, m.Rect.Min.Y+y)
		return m.Y[o : o+w]
	}
	cRow := func(plane []byte, cy int) []byte {
		o := m.COffset(m.Rect.Min.X, m.Rect.Min.Y+2*cy)
		return plane[o : o+cw]
	}
	set := func(row []byte, x int, y, u, v uint8) {
		px := row[x*channels:]
		px[0] = webpYUVToR(y, v)
		px[1] = webpYUVToG(y, u, v)
		px[2] = webpYUVToB(y, u)
	}
	// fillRow1 is fill_row_fancy_with_1_uv_row of image-webp.
	fillRow1 := func(row, y, u, v []byte) {
		set(row, 0, y[0], u[0], v[0])
		k := 0
		for ; 2+2*k < w; k++ {
			set(row, 1+2*k, y[1+2*k], webpFancy(u[k], u[k+1], u[k], u[k+1]), webpFancy(v[k], v[k+1], v[k], v[k+1]))
			set(row, 2+2*k, y[2+2*k], webpFancy(u[k+1], u[k], u[k+1], u[k]), webpFancy(v[k+1], v[k], v[k+1], v[k]))
		}
		if x := 1 + 2*k; x < w {
			set(row, x, y[x], u[len(u)-1], v[len(v)-1])
		}
	}
	// fillRow2 is fill_row_fancy_with_2_uv_rows of image-webp. The rows u1
	// and v1 are the nearest.
	fillRow2 := func(row, y, u1, u2, v1, v2 []byte) {
		set(row, 0, y[0], webpFancy(u1[0], u1[0], u2[0], u2[0]), webpFancy(v1[0], v1[0], v2[0], v2[0]))
		k := 0
		for ; 2+2*k < w; k++ {
			set(row, 1+2*k, y[1+2*k], webpFancy(u1[k], u1[k+1], u2[k], u2[k+1]), webpFancy(v1[k], v1[k+1], v2[k], v2[k+1]))
			set(row, 2+2*k, y[2+2*k], webpFancy(u1[k+1], u1[k], u2[k+1], u2[k]), webpFancy(v1[k+1], v1[k], v2[k+1], v2[k]))
		}
		if x := 1 + 2*k; x < w {
			lu1, lu2, lv1, lv2 := u1[len(u1)-1], u2[len(u2)-1], v1[len(v1)-1], v2[len(v2)-1]
			set(row, x, y[x], webpFancy(lu1, lu1, lu2, lu2), webpFancy(lv1, lv1, lv2, lv2))
		}
	}
	rowOf := func(y int) []byte { return out[y*w*channels : (y+1)*w*channels] }

	fillRow1(rowOf(0), yRow(0), cRow(m.Cb, 0), cRow(m.Cr, 0))
	k := 0
	for ; 2+2*k < h; k++ {
		u1, u2 := cRow(m.Cb, k), cRow(m.Cb, k+1)
		v1, v2 := cRow(m.Cr, k), cRow(m.Cr, k+1)
		fillRow2(rowOf(1+2*k), yRow(1+2*k), u1, u2, v1, v2)
		fillRow2(rowOf(2+2*k), yRow(2+2*k), u2, u1, v2, v1)
	}
	if y := 1 + 2*k; y < h {
		last := (h+1)/2 - 1
		fillRow1(rowOf(y), yRow(y), cRow(m.Cb, last), cRow(m.Cr, last))
	}

	if channels == 4 {
		for y := range h {
			row := rowOf(y)
			for x := range w {
				a := uint8(255)
				if alpha != nil {
					a = alpha[y*alphaStride+x]
				}
				row[4*x+3] = a
			}
		}
	}
	return out
}

// webpFancy is get_fancy_chroma_value of image-webp src/yuv.rs.
func webpFancy(main, secondary1, secondary2, tertiary uint8) uint8 {
	return uint8((9*uint16(main) + 3*uint16(secondary1) + 3*uint16(secondary2) + uint16(tertiary) + 8) / 16)
}

// webpMulHi is mulhi of image-webp src/yuv.rs.
func webpMulHi(v uint8, coeff uint16) int32 {
	return int32((uint32(v) * uint32(coeff)) >> 8)
}

// webpClip is clip of image-webp src/yuv.rs.
func webpClip(v int32) uint8 {
	return uint8(min(max(v>>6, 0), 255))
}

// webpYUVToR is yuv_to_r of image-webp src/yuv.rs.
func webpYUVToR(y, v uint8) uint8 {
	return webpClip(webpMulHi(y, 19077) + webpMulHi(v, 26149) - 14234)
}

// webpYUVToG is yuv_to_g of image-webp src/yuv.rs.
func webpYUVToG(y, u, v uint8) uint8 {
	return webpClip(webpMulHi(y, 19077) - webpMulHi(u, 6419) - webpMulHi(v, 13320) + 8708)
}

// webpYUVToB is yuv_to_b of image-webp src/yuv.rs.
func webpYUVToB(y, u uint8) uint8 {
	return webpClip(webpMulHi(y, 19077) + webpMulHi(u, 33050) - 17685)
}
