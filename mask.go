// Copyright 2020 Yevhenii Reizner
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package okage

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"math"
	"os"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// MaskType is MaskType in tiny-skia src/mask.rs. It is a mask type.
type MaskType uint8

// The values of MaskType, in the order of the Rust enum.
const (
	// MaskTypeAlpha is MaskType::Alpha. It transfers only the Alpha channel
	// from Pixmap to Mask.
	MaskTypeAlpha MaskType = iota

	// MaskTypeLuminance is MaskType::Luminance. It transfers RGB channels as
	// luminance from Pixmap to Mask.
	//
	// Formula: Y = 0.2126 * R + 0.7152 * G + 0.0722 * B.
	MaskTypeLuminance
)

// String satisfies the fmt.Stringer interface. It returns the name of the
// Rust variant.
func (t MaskType) String() string {
	switch t {
	case MaskTypeAlpha:
		return "Alpha"
	case MaskTypeLuminance:
		return "Luminance"
	}
	return "MaskType(?)"
}

// Mask is Mask in tiny-skia src/mask.rs. It is a mask.
//
// During drawing over Pixmap, mask's black (0) "pixels" block rendering and
// white (255) allow it. Anything in between is used for gradual masking and
// anti-aliasing.
//
// Unlike Skia, we are using just a simple 8bit alpha mask. It is way slower,
// but easier to implement.
//
// Port: logger is the logger of D20, which FillPath logs to. The option
// WithLogger sets it. A nil logger logs nothing.
type Mask struct {
	data   []byte
	size   path.IntSize
	logger *slog.Logger
}

// NewMask is Mask::new in tiny-skia src/mask.rs. It creates a new mask,
// allocating a buffer of the given size.
//
// Port: opts configure the mask, as D20 says. Upstream has no options.
func NewMask(width, height uint32, opts ...Option) (*Mask, bool) {
	size, ok := path.IntSizeFromWH(width, height)
	if !ok {
		return nil, false
	}
	return &Mask{
		data:   make([]byte, int(width)*int(height)),
		size:   size,
		logger: newOptions(opts).logger,
	}, true
}

// MaskFromPixmap is Mask::from_pixmap in tiny-skia src/mask.rs. It creates a
// new mask from a PixmapRef.
//
// Port: opts configure the mask, as D20 says.
func MaskFromPixmap(pixmap PixmapRef, maskType MaskType, opts ...Option) *Mask {
	dataLen := int(pixmap.Width()) * int(pixmap.Height())
	mask := &Mask{
		data:   make([]byte, dataLen),
		size:   pixmap.Size(),
		logger: newOptions(opts).logger,
	}

	// TODO: optimize
	pixels := pixmap.Pixels()
	n := min(pixels.Len(), len(mask.data))
	switch maskType {
	case MaskTypeAlpha:
		for i := range n {
			mask.data[i] = pixels.At(i).Alpha()
		}
	case MaskTypeLuminance:
		for i := range n {
			p := pixels.At(i)
			// Normalize.
			r := float32(p.Red()) / 255.0
			g := float32(p.Green()) / 255.0
			b := float32(p.Blue()) / 255.0
			a := float32(p.Alpha()) / 255.0

			// Demultiply.
			if p.Alpha() != 0 {
				r /= a
				g /= a
				b /= a
			}

			luma := float32(r*0.2126) + float32(g*0.7152) + float32(b*0.0722)
			mask.data[i] = wide.TruncU8(ceilF32(clampF32(float32(luma*a)*255.0, 0.0, 255.0)))
		}
	}

	return mask
}

// clampF32 is f32::clamp. It ports no upstream function. NaN stays NaN.
func clampF32(x, lo, hi float32) float32 {
	if x < lo {
		x = lo
	}
	if x > hi {
		x = hi
	}
	return x
}

// MaskFromVec is Mask::from_vec in tiny-skia src/mask.rs. It creates a new
// mask by taking ownership over a mask buffer.
//
// The size needs to match the data provided.
//
// Port: Rust takes ownership of data. Go keeps data in the mask, so the
// caller does not use data after the call. opts configure the mask, as D20
// says.
func MaskFromVec(data []byte, size path.IntSize, opts ...Option) (*Mask, bool) {
	dataLen := int(size.Width()) * int(size.Height())
	if len(data) != dataLen {
		return nil, false
	}

	return &Mask{data: data, size: size, logger: newOptions(opts).logger}, true
}

// Width is Mask::width in tiny-skia src/mask.rs. It returns mask's width.
func (m *Mask) Width() uint32 {
	return m.size.Width()
}

// Height is Mask::height in tiny-skia src/mask.rs. It returns mask's height.
func (m *Mask) Height() uint32 {
	return m.size.Height()
}

// Size is Mask::size in tiny-skia src/mask.rs. It returns mask's size.
//
// Port: upstream makes it public in the crate only. Go exports it, as
// Pixmap.Size does.
func (m *Mask) Size() path.IntSize {
	return m.size
}

// Data is Mask::data in tiny-skia src/mask.rs. It returns the internal data.
func (m *Mask) Data() []byte {
	return m.data
}

// DataMut is Mask::data_mut in tiny-skia src/mask.rs. It returns the mutable
// internal data.
func (m *Mask) DataMut() []byte {
	return m.data
}

// Take is Mask::take in tiny-skia src/mask.rs. It consumes the mask and
// returns its owned internal data.
//
// Port: Rust consumes the mask. Go returns the data, and the caller does not
// use m after the call.
func (m *Mask) Take() []byte {
	return m.data
}

// Clone is the derived Clone impl of Mask in tiny-skia src/mask.rs. It
// returns a copy of the mask with its own data.
func (m *Mask) Clone() *Mask {
	return &Mask{
		data:   bytes.Clone(m.data),
		size:   m.size,
		logger: m.logger,
	}
}

// asSubmask is Mask::as_submask in tiny-skia src/mask.rs.
func (m *Mask) asSubmask() subMaskRef {
	return subMaskRef{
		size:      m.size,
		realWidth: m.size.Width(),
		data:      m.data,
	}
}

// submask is Mask::submask in tiny-skia src/mask.rs.
func (m *Mask) submask(rect IntRect) (subMaskRef, bool) {
	rect, ok := m.size.ToIntRect(0, 0).Intersect(rect)
	if !ok {
		return subMaskRef{}, false
	}
	rowBytes := int(m.Width())
	offset := int(rect.Top())*rowBytes + int(rect.Left())

	return subMaskRef{
		size:      rect.Size(),
		realWidth: m.size.Width(),
		data:      m.data[offset:],
	}, true
}

// asSubpixmap is Mask::as_subpixmap in tiny-skia src/mask.rs.
func (m *Mask) asSubpixmap() subPixmapMut {
	return subPixmapMut{
		size:      m.size,
		realWidth: int(m.size.Width()),
		data:      m.data,
		logger:    m.logger,
	}
}

// subpixmap is Mask::subpixmap in tiny-skia src/mask.rs.
func (m *Mask) subpixmap(rect IntRect) (subPixmapMut, bool) {
	rect, ok := m.size.ToIntRect(0, 0).Intersect(rect)
	if !ok {
		return subPixmapMut{}, false
	}
	rowBytes := int(m.Width())
	offset := int(rect.Top())*rowBytes + int(rect.Left())

	return subPixmapMut{
		size:      rect.Size(),
		realWidth: int(m.size.Width()),
		data:      m.data[offset:],
		logger:    m.logger,
	}, true
}

// DecodeMaskPNG is Mask::decode_png in tiny-skia src/mask.rs. It loads a PNG
// file into a Mask.
//
// Only grayscale images are supported.
//
// Port: upstream decodes with the crate png and asks it to expand a gray
// image with fewer than 8 bits and to strip 16 bits to 8 bits. The package
// image/png decodes the image here (D7). It gives an image.Gray for a gray
// image of 8 bits or fewer, with the same values, and an image.Gray16 for one
// of 16 bits, whose high byte is kept. opts configure the mask, as D20 says.
func DecodeMaskPNG(data []byte, opts ...Option) (*Mask, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodingPNG, err)
	}

	b := img.Bounds()
	imgData := make([]byte, 0, b.Dx()*b.Dy())
	switch m := img.(type) {
	case *image.Gray:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			i := m.PixOffset(b.Min.X, y)
			imgData = append(imgData, m.Pix[i:i+b.Dx()]...)
		}
	case *image.Gray16:
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				imgData = append(imgData, m.Pix[m.PixOffset(x, y)])
			}
		}
	default:
		// Upstream gives "only grayscale masks are supported".
		return nil, ErrUnsupportedPNG
	}

	if uint64(b.Dx()) > math.MaxUint32 || uint64(b.Dy()) > math.MaxUint32 {
		return nil, ErrInvalidImageSize
	}
	size, ok := path.IntSizeFromWH(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil, ErrInvalidImageSize
	}

	mask, ok := MaskFromVec(imgData, size, opts...)
	if !ok {
		return nil, ErrCreatingMask
	}
	return mask, nil
}

// LoadMaskPNG is Mask::load_png in tiny-skia src/mask.rs. It loads a PNG file
// into a Mask.
//
// Only grayscale images are supported.
//
// Port: opts configure the mask, as D20 says.
func LoadMaskPNG(name string, opts ...Option) (*Mask, error) {
	// `png::Decoder` is generic over input, which means that it instances
	// two copies: one for `&[]` and one for `File`. Which simply bloats the code.
	// Therefore we are using only one type for input.
	data, err := os.ReadFile(name) //nolint:gosec // the caller names the file, as in upstream
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodingPNG, err)
	}
	return DecodeMaskPNG(data, opts...)
}

// EncodePNG is Mask::encode_png in tiny-skia src/mask.rs. It encodes the mask
// into PNG data.
//
// Port: upstream encodes with the crate png, as 8-bit grayscale. The package
// image/png encodes here (D7), also as 8-bit grayscale. The bytes of the file
// can differ from upstream, and the decoded values are the same.
func (m *Mask) EncodePNG() ([]byte, error) {
	img := &image.Gray{
		Pix:    m.data,
		Stride: int(m.Width()),
		Rect:   image.Rect(0, 0, int(m.Width()), int(m.Height())),
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncodingPNG, err)
	}
	return buf.Bytes(), nil
}

// SavePNG is Mask::save_png in tiny-skia src/mask.rs. It saves the mask as a
// PNG file.
func (m *Mask) SavePNG(name string) error {
	data, err := m.EncodePNG()
	if err != nil {
		return err
	}
	if err := os.WriteFile(name, data, 0o666); err != nil { //nolint:gosec // the mode of a new image file, as in upstream
		return fmt.Errorf("%w: %w", ErrEncodingPNG, err)
	}
	return nil
}

// FillPath is Mask::fill_path in tiny-skia src/mask.rs. It draws a filled
// path onto the mask.
//
// It is almost a direct copy of PixmapMut.FillPath.
//
// In terms of RGB (no alpha) image, it draws a white path on top of black
// mask.
//
// It does not reset the existing mask content and draws the path on top of
// existing data.
//
// If the above behavior is undesired, call Mask.Clear first.
//
// This method is intended to be used for simple cases. For more complex
// masks prefer MaskFromPixmap.
//
// Port: upstream logs a warning with log::warn!. Go logs it to the logger of
// the mask, as D20 says.
func (m *Mask) FillPath(
	p *Path,
	fillRule FillRule,
	antiAlias bool,
	transform Transform,
) {
	if transform.IsIdentity() {
		// This is sort of similar to SkDraw::drawPath

		// Skip empty paths and horizontal/vertical lines.
		pathBounds := p.Bounds()
		if path.IsNearlyZero(pathBounds.Width()) || path.IsNearlyZero(pathBounds.Height()) {
			warn(m.logger, "empty paths and horizontal/vertical lines cannot be filled")
			return
		}

		if isTooBigForMath(p) {
			warn(m.logger, "path coordinates are too big")
			return
		}

		// TODO: ignore paths outside the pixmap

		if tiler, ok := newDrawTiler(m.Width(), m.Height()); ok {
			p = p.Clone() // TODO: avoid cloning

			for tile, ok := tiler.next(); ok; tile, ok = tiler.next() {
				ts := path.TransformFromTranslate(-float32(tile.X()), -float32(tile.Y()))
				var tok bool
				p, tok = p.Transform(ts)
				if !tok {
					warn(m.logger, "path transformation failed")
					return
				}

				clipRect := intSizeToScreenIntRect(tile.Size(), 0, 0)
				subpix, ok := m.subpixmap(tile.ToIntRect())
				if !ok {
					continue // technically unreachable
				}

				blitter, ok := newRasterPipelineBlitterMask(&subpix)
				if !ok {
					continue // nothing to do, all good
				}

				// We are ignoring "errors" here, because `fill_path` returns `None`
				// when rendering a tile that does not have a path on it.
				// Which is not an error in this case.
				if antiAlias {
					fillPathPathAA(p, fillRule, &clipRect, blitter, m.logger)
				} else {
					fillPathPath(p, fillRule, &clipRect, blitter, m.logger)
				}

				ts = path.TransformFromTranslate(float32(tile.X()), float32(tile.Y()))
				p, tok = p.Transform(ts)
				if !tok {
					return // technically unreachable
				}
			}
		} else {
			clipRect := intSizeToScreenIntRect(m.Size(), 0, 0)
			subpix := m.asSubpixmap()
			blitter, ok := newRasterPipelineBlitterMask(&subpix)
			if !ok {
				return // nothing to do, all good
			}

			if antiAlias {
				fillPathPathAA(p, fillRule, &clipRect, blitter, m.logger)
			} else {
				fillPathPath(p, fillRule, &clipRect, blitter, m.logger)
			}
		}
	} else {
		p, ok := p.Clone().Transform(transform)
		if !ok {
			warn(m.logger, "path transformation failed")
			return
		}

		m.FillPath(p, fillRule, antiAlias, path.IdentityTransform())
	}
}

// IntersectPath is Mask::intersect_path in tiny-skia src/mask.rs. It
// intersects the provided path with the current clipping path.
//
// A temporary mask with the same size as the current one is created.
func (m *Mask) IntersectPath(
	p *Path,
	fillRule FillRule,
	antiAlias bool,
	transform Transform,
) {
	submask, ok := NewMask(m.Width(), m.Height(), WithLogger(m.logger))
	if !ok {
		panic("called `Option::unwrap()` on a `None` value")
	}
	submask.FillPath(p, fillRule, antiAlias, transform)

	for i := range min(len(m.data), len(submask.data)) {
		m.data[i] = premultiplyU8(m.data[i], submask.data[i])
	}
}

// Invert is Mask::invert in tiny-skia src/mask.rs. It inverts the mask.
func (m *Mask) Invert() {
	for i := range m.data {
		m.data[i] = 255 - m.data[i]
	}
}

// Clear is Mask::clear in tiny-skia src/mask.rs. It clears the mask.
//
// It zero-fills the internal data buffer.
func (m *Mask) Clear() {
	clear(m.data)
}

// String is the Debug impl of Mask in tiny-skia src/mask.rs. It satisfies
// the fmt.Stringer interface.
func (m *Mask) String() string {
	return fmt.Sprintf("Mask { data: %q, width: %d, height: %d }", "...", m.size.Width(), m.size.Height())
}

// subMaskRef is SubMaskRef in tiny-skia src/mask.rs.
type subMaskRef struct {
	data      []byte
	size      path.IntSize
	realWidth uint32
}

// maskCtx is SubMaskRef::mask_ctx in tiny-skia src/mask.rs.
func (s *subMaskRef) maskCtx() maskCtx {
	return maskCtx{
		data:      s.data,
		realWidth: s.realWidth,
	}
}
