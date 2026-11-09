// Copyright 2006 The Android Open Source Project
// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"math"
	"os"

	"github.com/xo/okage/path"
)

// BytesPerPixel is BYTES_PER_PIXEL in tiny-skia src/pixmap.rs. It is the
// number of bytes per pixel.
const BytesPerPixel = 4

// PixelSlice is the Go form of the Rust slices &[PremultipliedColorU8] and
// &mut [PremultipliedColorU8], which tiny-skia src/pixmap.rs makes from the
// bytes of a pixmap with bytemuck. It ports no upstream item.
//
// Go cannot view a []byte as a slice of colors without the package unsafe
// (D25), so a PixelSlice holds the bytes and reads or writes one pixel at a
// time. It shares the bytes of the pixmap. Its length is a multiple of
// BytesPerPixel.
type PixelSlice []byte

// Len returns the number of pixels.
func (s PixelSlice) Len() int {
	return len(s) / BytesPerPixel
}

// At returns the pixel at index i. It panics if i is out of range.
func (s PixelSlice) At(i int) PremultipliedColorU8 {
	return PremultipliedColorU8{rgba: [4]uint8(s[i*BytesPerPixel : i*BytesPerPixel+BytesPerPixel])}
}

// Set sets the pixel at index i to c. It panics if i is out of range.
func (s PixelSlice) Set(i int, c PremultipliedColorU8) {
	*(*[4]uint8)(s[i*BytesPerPixel : i*BytesPerPixel+BytesPerPixel]) = c.rgba
}

// get is the method get of a Rust slice. It returns false if i is out of
// range.
func (s PixelSlice) get(i int) (PremultipliedColorU8, bool) {
	if i < 0 || i >= s.Len() {
		return PremultipliedColorU8{}, false
	}
	return s.At(i), true
}

// Pixmap is Pixmap in tiny-skia src/pixmap.rs. It is a container that owns
// premultiplied RGBA pixels.
//
// The data is not aligned, therefore width == stride.
//
// Port: logger is the logger of D20, which the painter logs to. The option
// WithLogger sets it. A nil logger logs nothing.
type Pixmap struct {
	data   []byte
	size   path.IntSize
	logger *slog.Logger
}

// NewPixmap is Pixmap::new in tiny-skia src/pixmap.rs. It allocates a new
// pixmap.
//
// A pixmap is filled with transparent black by default, aka (0, 0, 0, 0).
//
// A zero size is an error, and NewPixmap returns false.
//
// The width of a pixmap is limited by i32::MAX/4.
//
// Port: opts configure the pixmap, as D20 says. Upstream has no options.
func NewPixmap(width, height uint32, opts ...Option) (*Pixmap, bool) {
	size, ok := path.IntSizeFromWH(width, height)
	if !ok {
		return nil, false
	}
	dataLen, ok := dataLenForSize(size)
	if !ok {
		return nil, false
	}

	// We cannot check that allocation was successful yet.
	// We have to wait for https://github.com/rust-lang/rust/issues/48043

	return &Pixmap{
		data:   make([]byte, dataLen),
		size:   size,
		logger: newOptions(opts).logger,
	}, true
}

// PixmapFromVec is Pixmap::from_vec in tiny-skia src/pixmap.rs. It creates a
// new pixmap by taking ownership over an image buffer (premultiplied RGBA
// pixels).
//
// The size needs to match the data provided.
//
// The width of a pixmap is limited by i32::MAX/4.
//
// Port: Rust takes ownership of data. Go keeps data in the pixmap, so the
// caller does not use data after the call. opts configure the pixmap, as D20
// says.
func PixmapFromVec(data []byte, size path.IntSize, opts ...Option) (*Pixmap, bool) {
	dataLen, ok := dataLenForSize(size)
	if !ok {
		return nil, false
	}
	if len(data) != dataLen {
		return nil, false
	}

	return &Pixmap{data: data, size: size, logger: newOptions(opts).logger}, true
}

// DecodePNG is Pixmap::decode_png in tiny-skia src/pixmap.rs. It decodes PNG
// data into a Pixmap.
//
// Only 8-bit images are supported. Index PNGs are not supported.
//
// Port: upstream decodes with the crate png, and asks it to expand a palette,
// a gray image with fewer than 8 bits and the tRNS chunk, and to strip 16
// bits to 8 bits. The package image/png decodes the image here (D7). The
// conversion keeps the high byte of a 16-bit value and expands the other
// formats as the crate png does, so that the same pixels come out. Before it
// decodes the pixels, it reads the size of the image and makes the size
// checks of upstream, so that it does not allocate a huge image that
// upstream rejects later. opts configure the pixmap, as D20 says.
func DecodePNG(data []byte, opts ...Option) (*Pixmap, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodingPNG, err)
	}
	if cfg.Width < 0 || cfg.Height < 0 || uint64(cfg.Width) > math.MaxUint32 || uint64(cfg.Height) > math.MaxUint32 {
		return nil, ErrInvalidImageSize
	}
	if size, ok := path.IntSizeFromWH(uint32(cfg.Width), uint32(cfg.Height)); ok {
		if _, ok := dataLenForSize(size); !ok {
			return nil, ErrImageTooBig
		}
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodingPNG, err)
	}

	b := img.Bounds()
	size, ok := path.IntSizeFromWH(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil, ErrInvalidImageSize
	}
	dataLen, ok := dataLenForSize(size)
	if !ok {
		return nil, ErrImageTooBig
	}

	imgData, err := pngToRGBA(img, dataLen)
	if err != nil {
		return nil, err
	}

	// Premultiply alpha.
	//
	// We cannon use RasterPipeline here, which is faster,
	// because it produces slightly different results.
	// Seems like Skia does the same.
	//
	// Also, in our tests unsafe version (no bound checking)
	// had roughly the same performance. So we keep the safe one.
	for i := 0; i+BytesPerPixel <= len(imgData); i += BytesPerPixel {
		pixel := imgData[i : i+BytesPerPixel]
		a := pixel[3]
		pixel[0] = premultiplyU8(pixel[0], a)
		pixel[1] = premultiplyU8(pixel[1], a)
		pixel[2] = premultiplyU8(pixel[2], a)
	}

	p, ok := PixmapFromVec(imgData, size, opts...)
	if !ok {
		return nil, ErrCreatingPixmap
	}
	return p, nil
}

// pngToRGBA is the match on info.color_type in Pixmap::decode_png in
// tiny-skia src/pixmap.rs. It returns the pixels of img as RGBA bytes that are
// not premultiplied.
//
// Port: upstream matches the color type that the crate png gives after its
// transformations: Rgb, Rgba, Grayscale and GrayscaleAlpha. The package
// image/png gives an image type for each PNG format, and each case below
// gives the bytes that the crate png gives for that format.
func pngToRGBA(img image.Image, dataLen int) ([]byte, error) {
	b := img.Bounds()
	rgba := make([]byte, 0, dataLen)
	switch m := img.(type) {
	case *image.Gray:
		// png::ColorType::Grayscale
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				gray := m.Pix[m.PixOffset(x, y)]
				rgba = append(rgba, gray, gray, gray, AlphaU8Opaque)
			}
		}
	case *image.Gray16:
		// png::ColorType::Grayscale, stripped to 8 bits
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				gray := m.Pix[m.PixOffset(x, y)]
				rgba = append(rgba, gray, gray, gray, AlphaU8Opaque)
			}
		}
	case *image.RGBA:
		// png::ColorType::Rgb. image/png gives an RGBA image only for an
		// RGB image without a tRNS chunk, so each alpha is 255.
		for y := b.Min.Y; y < b.Max.Y; y++ {
			i := m.PixOffset(b.Min.X, y)
			rgba = append(rgba, m.Pix[i:i+b.Dx()*4]...)
		}
	case *image.RGBA64:
		// png::ColorType::Rgb, stripped to 8 bits
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				i := m.PixOffset(x, y)
				rgba = append(rgba, m.Pix[i], m.Pix[i+2], m.Pix[i+4], AlphaU8Opaque)
			}
		}
	case *image.NRGBA:
		// png::ColorType::Rgba and png::ColorType::GrayscaleAlpha, and the
		// formats that the tRNS chunk expands to them
		for y := b.Min.Y; y < b.Max.Y; y++ {
			i := m.PixOffset(b.Min.X, y)
			rgba = append(rgba, m.Pix[i:i+b.Dx()*4]...)
		}
	case *image.NRGBA64:
		// png::ColorType::Rgba and png::ColorType::GrayscaleAlpha, stripped
		// to 8 bits
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				i := m.PixOffset(x, y)
				rgba = append(rgba, m.Pix[i], m.Pix[i+2], m.Pix[i+4], m.Pix[i+6])
			}
		}
	case *image.Paletted:
		// png::ColorType::Indexed, which the crate png expands to Rgb or
		// Rgba
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				switch c := m.Palette[m.Pix[m.PixOffset(x, y)]].(type) {
				case color.RGBA:
					rgba = append(rgba, c.R, c.G, c.B, c.A)
				case color.NRGBA:
					rgba = append(rgba, c.R, c.G, c.B, c.A)
				default:
					return nil, ErrUnsupportedPNG
				}
			}
		}
	default:
		return nil, ErrUnsupportedPNG
	}
	return rgba, nil
}

// LoadPNG is Pixmap::load_png in tiny-skia src/pixmap.rs. It loads a PNG file
// into a Pixmap.
//
// Only 8-bit images are supported. Index PNGs are not supported.
//
// Port: opts configure the pixmap, as D20 says.
func LoadPNG(name string, opts ...Option) (*Pixmap, error) {
	// `png::Decoder` is generic over input, which means that it will instance
	// two copies: one for `&[]` and one for `File`. Which will simply bloat the code.
	// Therefore we are using only one type for input.
	data, err := os.ReadFile(name) //nolint:gosec // the caller names the file, as in upstream
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodingPNG, err)
	}
	return DecodePNG(data, opts...)
}

// EncodePNG is Pixmap::encode_png in tiny-skia src/pixmap.rs. It encodes the
// pixmap into PNG data.
func (p *Pixmap) EncodePNG() ([]byte, error) {
	return p.AsRef().EncodePNG()
}

// SavePNG is Pixmap::save_png in tiny-skia src/pixmap.rs. It saves the pixmap
// as a PNG file.
func (p *Pixmap) SavePNG(name string) error {
	return p.AsRef().SavePNG(name)
}

// AsRef is Pixmap::as_ref in tiny-skia src/pixmap.rs. It returns a container
// that references the data of the pixmap.
func (p *Pixmap) AsRef() PixmapRef {
	return PixmapRef{
		data:   p.data,
		size:   p.size,
		logger: p.logger,
	}
}

// AsMut is Pixmap::as_mut in tiny-skia src/pixmap.rs. It returns a container
// that references the data of the pixmap.
func (p *Pixmap) AsMut() PixmapMut {
	return PixmapMut{
		data:   p.data,
		size:   p.size,
		logger: p.logger,
	}
}

// Width is Pixmap::width in tiny-skia src/pixmap.rs. It returns the width of
// the pixmap.
func (p *Pixmap) Width() uint32 {
	return p.size.Width()
}

// Height is Pixmap::height in tiny-skia src/pixmap.rs. It returns the height
// of the pixmap.
func (p *Pixmap) Height() uint32 {
	return p.size.Height()
}

// Size is Pixmap::size in tiny-skia src/pixmap.rs. It returns the size of the
// pixmap.
//
// Port: upstream makes it public in the crate only. Go exports it, because
// the method of PixmapRef has the same name.
func (p *Pixmap) Size() path.IntSize {
	return p.size
}

// Fill is Pixmap::fill in tiny-skia src/pixmap.rs. It fills the entire pixmap
// with the given color.
func (p *Pixmap) Fill(color Color) {
	c := color.Premultiply().ToColorU8()
	pm := p.AsMut()
	pixels := pm.PixelsMut()
	for i := range pixels.Len() {
		pixels.Set(i, c)
	}
}

// Data is Pixmap::data in tiny-skia src/pixmap.rs. It returns the internal
// data.
//
// Byte order: RGBA.
func (p *Pixmap) Data() []byte {
	return p.data
}

// DataMut is Pixmap::data_mut in tiny-skia src/pixmap.rs. It returns the
// mutable internal data.
//
// Byte order: RGBA.
func (p *Pixmap) DataMut() []byte {
	return p.data
}

// Pixel is Pixmap::pixel in tiny-skia src/pixmap.rs. It returns the color of
// a pixel.
//
// It returns false when the position is out of bounds.
func (p *Pixmap) Pixel(x, y uint32) (PremultipliedColorU8, bool) {
	return p.AsRef().Pixel(x, y)
}

// PixelsMut is Pixmap::pixels_mut in tiny-skia src/pixmap.rs. It returns a
// mutable slice of pixels.
func (p *Pixmap) PixelsMut() PixelSlice {
	return PixelSlice(p.DataMut())
}

// Pixels is Pixmap::pixels in tiny-skia src/pixmap.rs. It returns a slice of
// pixels.
func (p *Pixmap) Pixels() PixelSlice {
	return PixelSlice(p.Data())
}

// Take is Pixmap::take in tiny-skia src/pixmap.rs. It consumes the internal
// data.
//
// Byte order: RGBA.
//
// Port: Rust consumes the pixmap. Go returns the data, and the caller does
// not use p after the call.
func (p *Pixmap) Take() []byte {
	return p.data
}

// TakeDemultiplied is Pixmap::take_demultiplied in tiny-skia src/pixmap.rs.
// It consumes the pixmap and returns the internal data as demultiplied RGBA
// bytes.
//
// Byte order: RGBA.
//
// Port: Rust consumes the pixmap. Go changes the data in place and returns
// it, and the caller does not use p after the call.
func (p *Pixmap) TakeDemultiplied() []byte {
	// Demultiply alpha.
	//
	// RasterPipeline is 15% faster here, but produces slightly different results
	// due to rounding. So we stick with this method for now.
	pixels := p.PixelsMut()
	for i := range pixels.Len() {
		c := pixels.At(i).Demultiply()
		pixels.Set(i, premultipliedColorU8FromRGBAUnchecked(c.Red(), c.Green(), c.Blue(), c.Alpha()))
	}
	return p.data
}

// CloneRect is Pixmap::clone_rect in tiny-skia src/pixmap.rs. It returns a
// copy of the part of the pixmap that intersects rect.
//
// It returns false when the rect of the pixmap does not contain rect.
func (p *Pixmap) CloneRect(rect IntRect) (*Pixmap, bool) {
	return p.AsRef().CloneRect(rect)
}

// Clone is the derived Clone impl of Pixmap in tiny-skia src/pixmap.rs. It
// returns a copy of the pixmap with its own data.
func (p *Pixmap) Clone() *Pixmap {
	return &Pixmap{
		data:   bytes.Clone(p.data),
		size:   p.size,
		logger: p.logger,
	}
}

// PixmapRef is PixmapRef in tiny-skia src/pixmap.rs. It is a container that
// references premultiplied RGBA pixels.
//
// It can be created from a Pixmap or from data that the user gives.
//
// The data is not aligned, therefore width == stride.
//
// Port: logger is the logger of D20. A Pixmap that ToOwned or CloneRect makes
// takes it.
type PixmapRef struct {
	data   []byte
	size   path.IntSize
	logger *slog.Logger
}

// PixmapRefFromBytes is PixmapRef::from_bytes in tiny-skia src/pixmap.rs. It
// creates a new PixmapRef from bytes.
//
// The size must be at least width * height * BytesPerPixel. A zero size is an
// error. The width is limited by i32::MAX/4.
//
// The data is assumed to have premultiplied RGBA pixels (byte order: RGBA).
//
// Port: opts configure the PixmapRef, as D20 says.
func PixmapRefFromBytes(data []byte, width, height uint32, opts ...Option) (PixmapRef, bool) {
	size, ok := path.IntSizeFromWH(width, height)
	if !ok {
		return PixmapRef{}, false
	}
	dataLen, ok := dataLenForSize(size)
	if !ok {
		return PixmapRef{}, false
	}
	if len(data) < dataLen {
		return PixmapRef{}, false
	}

	return PixmapRef{data: data, size: size, logger: newOptions(opts).logger}, true
}

// ToOwned is PixmapRef::to_owned in tiny-skia src/pixmap.rs. It creates a new
// Pixmap from the current data.
//
// It clones the underlying data.
func (r PixmapRef) ToOwned() *Pixmap {
	return &Pixmap{
		data:   bytes.Clone(r.data),
		size:   r.size,
		logger: r.logger,
	}
}

// Width is PixmapRef::width in tiny-skia src/pixmap.rs. It returns the width
// of the pixmap.
func (r PixmapRef) Width() uint32 {
	return r.size.Width()
}

// Height is PixmapRef::height in tiny-skia src/pixmap.rs. It returns the
// height of the pixmap.
func (r PixmapRef) Height() uint32 {
	return r.size.Height()
}

// Size is PixmapRef::size in tiny-skia src/pixmap.rs. It returns the size of
// the pixmap.
//
// Port: upstream makes it public in the crate only. Go exports it, so that a
// later port, such as the pipeline, can call it.
func (r PixmapRef) Size() path.IntSize {
	return r.size
}

// rect is PixmapRef::rect in tiny-skia src/pixmap.rs. It returns the rect of
// the pixmap.
func (r PixmapRef) rect() screenIntRect {
	return intSizeToScreenIntRect(r.size, 0, 0)
}

// Data is PixmapRef::data in tiny-skia src/pixmap.rs. It returns the internal
// data.
//
// Byte order: RGBA.
func (r PixmapRef) Data() []byte {
	return r.data
}

// Pixel is PixmapRef::pixel in tiny-skia src/pixmap.rs. It returns the color
// of a pixel.
//
// It returns false when the position is out of bounds.
func (r PixmapRef) Pixel(x, y uint32) (PremultipliedColorU8, bool) {
	idx, ok := checkedMulU32(r.Width(), y)
	if !ok {
		return PremultipliedColorU8{}, false
	}
	idx, ok = checkedAddU32(idx, x)
	if !ok {
		return PremultipliedColorU8{}, false
	}
	return r.Pixels().get(int(idx))
}

// Pixels is PixmapRef::pixels in tiny-skia src/pixmap.rs. It returns a slice
// of pixels.
func (r PixmapRef) Pixels() PixelSlice {
	return PixelSlice(r.Data())
}

// TODO: add rows() iterator

// CloneRect is PixmapRef::clone_rect in tiny-skia src/pixmap.rs. It returns a
// copy of the part of the pixmap that intersects rect.
//
// It returns false when the rect of the pixmap does not contain rect.
func (r PixmapRef) CloneRect(rect IntRect) (*Pixmap, bool) {
	// TODO: to ScreenIntRect?

	rect, ok := r.rect().ToIntRect().Intersect(rect)
	if !ok {
		return nil, false
	}
	n, ok := NewPixmap(rect.Width(), rect.Height(), WithLogger(r.logger))
	if !ok {
		return nil, false
	}
	{
		oldPixels := r.Pixels()
		newMut := n.AsMut()
		newPixels := newMut.PixelsMut()

		// TODO: optimize
		for y := range rect.Height() {
			for x := range rect.Width() {
				oldIdx := (y+uint32(rect.Y()))*r.Width() + (x + uint32(rect.X()))
				newIdx := y*rect.Width() + x
				newPixels.Set(int(newIdx), oldPixels.At(int(oldIdx)))
			}
		}
	}

	return n, true
}

// EncodePNG is PixmapRef::encode_png in tiny-skia src/pixmap.rs. It encodes
// the pixmap into PNG data.
//
// Port: upstream encodes with the crate png, as 8-bit RGBA. The package
// image/png encodes here (D7). It writes 8-bit RGB when every pixel is
// opaque. The bytes of the file differ from upstream, and the decoded pixels
// are the same.
func (r PixmapRef) EncodePNG() ([]byte, error) {
	// Skia uses skcms here, which is somewhat similar to RasterPipeline.

	// Sadly, we have to copy the pixmap here, because of demultiplication.
	// Not sure how to avoid this.
	// TODO: remove allocation
	demultipliedData := r.ToOwned().TakeDemultiplied()

	img := &image.NRGBA{
		Pix:    demultipliedData,
		Stride: int(r.Width()) * BytesPerPixel,
		Rect:   image.Rect(0, 0, int(r.Width()), int(r.Height())),
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodingPNG, err)
	}

	return buf.Bytes(), nil
}

// SavePNG is PixmapRef::save_png in tiny-skia src/pixmap.rs. It saves the
// pixmap as a PNG file.
func (r PixmapRef) SavePNG(name string) error {
	data, err := r.EncodePNG()
	if err != nil {
		return err
	}
	if err := os.WriteFile(name, data, 0o666); err != nil { //nolint:gosec // the mode of a new image file, as in upstream
		return fmt.Errorf("%w: %w", ErrEncodingPNG, err)
	}
	return nil
}

// PixmapMut is PixmapMut in tiny-skia src/pixmap.rs. It is a container that
// references mutable premultiplied RGBA pixels.
//
// It can be created from a Pixmap or from data that the user gives.
//
// The data is not aligned, therefore width == stride.
//
// Port: logger is the logger of D20, which the painter logs to.
type PixmapMut struct {
	data   []byte
	size   path.IntSize
	logger *slog.Logger
}

// PixmapMutFromBytes is PixmapMut::from_bytes in tiny-skia src/pixmap.rs. It
// creates a new PixmapMut from bytes.
//
// The size must be at least width * height * BytesPerPixel. A zero size is an
// error. The width is limited by i32::MAX/4.
//
// The data is assumed to have premultiplied RGBA pixels (byte order: RGBA).
//
// Port: opts configure the PixmapMut, as D20 says.
func PixmapMutFromBytes(data []byte, width, height uint32, opts ...Option) (PixmapMut, bool) {
	size, ok := path.IntSizeFromWH(width, height)
	if !ok {
		return PixmapMut{}, false
	}
	dataLen, ok := dataLenForSize(size)
	if !ok {
		return PixmapMut{}, false
	}
	if len(data) < dataLen {
		return PixmapMut{}, false
	}

	return PixmapMut{data: data, size: size, logger: newOptions(opts).logger}, true
}

// ToOwned is PixmapMut::to_owned in tiny-skia src/pixmap.rs. It creates a new
// Pixmap from the current data.
//
// It clones the underlying data.
func (m *PixmapMut) ToOwned() *Pixmap {
	return &Pixmap{
		data:   bytes.Clone(m.data),
		size:   m.size,
		logger: m.logger,
	}
}

// AsRef is PixmapMut::as_ref in tiny-skia src/pixmap.rs. It returns a
// container that references the data of the pixmap.
func (m *PixmapMut) AsRef() PixmapRef {
	return PixmapRef{
		data:   m.data,
		size:   m.size,
		logger: m.logger,
	}
}

// Width is PixmapMut::width in tiny-skia src/pixmap.rs. It returns the width
// of the pixmap.
func (m *PixmapMut) Width() uint32 {
	return m.size.Width()
}

// Height is PixmapMut::height in tiny-skia src/pixmap.rs. It returns the
// height of the pixmap.
func (m *PixmapMut) Height() uint32 {
	return m.size.Height()
}

// Size is PixmapMut::size in tiny-skia src/pixmap.rs. It returns the size of
// the pixmap.
//
// Port: upstream makes it public in the crate only. Go exports it, so that a
// later port, such as the painter, can call it.
func (m *PixmapMut) Size() path.IntSize {
	return m.size
}

// Fill is PixmapMut::fill in tiny-skia src/pixmap.rs. It fills the entire
// pixmap with the given color.
func (m *PixmapMut) Fill(color Color) {
	c := color.Premultiply().ToColorU8()
	pixels := m.PixelsMut()
	for i := range pixels.Len() {
		pixels.Set(i, c)
	}
}

// DataMut is PixmapMut::data_mut in tiny-skia src/pixmap.rs. It returns the
// mutable internal data.
//
// Byte order: RGBA.
func (m *PixmapMut) DataMut() []byte {
	return m.data
}

// PixelsMut is PixmapMut::pixels_mut in tiny-skia src/pixmap.rs. It returns a
// mutable slice of pixels.
func (m *PixmapMut) PixelsMut() PixelSlice {
	return PixelSlice(m.DataMut())
}

// asSubpixmap is PixmapMut::as_subpixmap in tiny-skia src/pixmap.rs. It
// creates a subPixmapMut that contains the whole PixmapMut.
func (m *PixmapMut) asSubpixmap() subPixmapMut {
	return subPixmapMut{
		size:      m.Size(),
		realWidth: int(m.Width()),
		data:      m.data,
		logger:    m.logger,
	}
}

// subpixmap is PixmapMut::subpixmap in tiny-skia src/pixmap.rs. It returns a
// mutable reference to the region of the pixmap that intersects rect.
//
// It returns false when the rect of the pixmap does not contain rect.
func (m *PixmapMut) subpixmap(rect IntRect) (subPixmapMut, bool) {
	rect, ok := m.size.ToIntRect(0, 0).Intersect(rect)
	if !ok {
		return subPixmapMut{}, false
	}
	rowBytes := int(m.Width()) * BytesPerPixel
	offset := int(rect.Top())*rowBytes + int(rect.Left())*BytesPerPixel

	return subPixmapMut{
		size:      rect.Size(),
		realWidth: int(m.Width()),
		data:      m.data[offset:],
		logger:    m.logger,
	}, true
}

// subPixmapMut is SubPixmapMut in tiny-skia src/pixmap.rs. It is a region of
// a PixmapMut.
//
// Unlike PixmapMut, it contains realWidth, which references the width of the
// parent PixmapMut. This way we can operate on a region of a PixmapMut
// without reallocations. It is primarily required because of DrawTiler.
//
// We cannot implement it in PixmapMut directly, because it will break fill,
// data_mut, pixels_mut and other similar methods. This is because
// SubPixmapMut.data references more "data" than it is actually allowed to
// access. On the other hand, PixmapMut.data can access all its data and it is
// stored linearly.
//
// Port: logger is the logger of D20, which the pipeline blitter and the
// shaders log to.
type subPixmapMut struct {
	data      []byte
	size      path.IntSize
	realWidth int
	logger    *slog.Logger
}

// pixelsMut is SubPixmapMut::pixels_mut in tiny-skia src/pixmap.rs. It returns
// a mutable slice of pixels.
//
// Port: bytemuck casts only whole pixels, so the slice ends at the last whole
// pixel of data.
func (s *subPixmapMut) pixelsMut() PixelSlice {
	n := len(s.data) / BytesPerPixel * BytesPerPixel
	return PixelSlice(s.data[:n])
}

// minRowBytes is min_row_bytes in tiny-skia src/pixmap.rs. It returns the
// minimum bytes per row.
//
// The maximum value for row bytes of a pixmap must fit in 31 bits.
func minRowBytes(size path.IntSize) (int, bool) {
	if size.Width() > math.MaxInt32 {
		return 0, false
	}
	w := int64(size.Width()) * BytesPerPixel
	if w > math.MaxInt32 {
		return 0, false
	}
	if w == 0 {
		return 0, false
	}
	return int(w), true
}

// computeDataLen is compute_data_len in tiny-skia src/pixmap.rs. It returns
// the storage size that the pixel array needs.
//
// Port: upstream computes in usize, which has 64 bits on the platforms that
// okage tests. Go computes in uint64 and makes sure that the result fits in an
// int.
func computeDataLen(size path.IntSize, rowBytes int) (int, bool) {
	h, ok := checkedSubU32(size.Height(), 1)
	if !ok {
		return 0, false
	}
	hb, overflow := mulU64(uint64(h), uint64(rowBytes))
	if overflow {
		return 0, false
	}

	w := uint64(size.Width()) * BytesPerPixel

	sum := hb + w
	if sum < hb || sum > math.MaxInt {
		return 0, false
	}
	return int(sum), true
}

// mulU64 is u64::overflowing_mul. It ports no upstream function.
func mulU64(a, b uint64) (uint64, bool) {
	if a == 0 || b == 0 {
		return 0, false
	}
	p := a * b
	return p, p/b != a
}

// dataLenForSize is data_len_for_size in tiny-skia src/pixmap.rs.
func dataLenForSize(size path.IntSize) (int, bool) {
	rowBytes, ok := minRowBytes(size)
	if !ok {
		return 0, false
	}
	return computeDataLen(size, rowBytes)
}
