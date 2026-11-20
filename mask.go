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
	"sync"
	"sync/atomic"

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
//
// Port: data holds only a rectangle of the mask, and the other pixels are 0
// (D35). The rectangle has the rows from top and the columns from left to
// left+stride. The byte of the pixel (x, y) is at stride*(y-top) + x, so the
// first left bytes of data are not used, and the stages find a pixel with
// the offset of upstream, with stride in place of the width. A clip then
// takes the memory and the time of the pixels that its path covers, not of
// the whole pixmap. The rectangle has a margin of maskChunk-1 columns on
// each side of the pixels that are not 0, or ends at the side of the mask,
// so that each chunk that maskBounds keeps is inside it (D60). Clone shares
// data, and the first write to a mask that shares its data copies it. A
// caller can hold data after Data, DataMut, Take or MaskFromVec. Then data
// holds every pixel, and the mask never shares it again. A caller sees no
// difference, except in the memory that a mask takes.
//
// Port: a mask of NewRectMask holds a rectangle and no pixels (D86). The
// first read or write of its pixels makes the pixels of the rectangle, with
// the layout above.
type Mask struct {
	data   []byte
	size   path.IntSize
	logger *slog.Logger

	// top is the first row and left the first column of data, and stride
	// is the number of its columns. stride is 0 when data is empty.
	top, left, stride int
	// exposed is true when a caller can hold data. data then holds every
	// row.
	exposed bool
	// cloned belongs to data, and all the masks that share data share it.
	// Clone sets it, so that a write copies data first. It is nil when the
	// mask cannot share data.
	cloned *atomic.Bool

	// rect is true for a mask of NewRectMask that holds no pixels (D86).
	// The mask is then 255 on the columns from rectLeft to rectRight of the
	// rows from rectTop to rectBottom, without rectRight and rectBottom,
	// and 0 on the other pixels. data is empty. edge holds the row of the
	// mask from the column 0 to maskChunk-1 columns past rectRight, or to
	// the side of the mask. No mask writes edge, so its clones share it.
	rect                                     bool
	rectLeft, rectTop, rectRight, rectBottom int
	edge                                     []byte
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
	// Port: the mask holds no row until a path covers one (D35).
	return &Mask{
		size:   size,
		logger: newOptions(opts).logger,
	}, true
}

// NewRectMask makes a mask of the size width by height that is 255 on the
// pixels of rect inside the mask and 0 on the other pixels. It returns false
// where NewMask returns false. It ports no upstream function (D86).
//
// The mask holds the rectangle and no pixels. A method of Pixmap or
// PixmapMut that draws through the mask gives the same bytes as through a
// mask that holds those pixels, and runs the stage mask_u8 only on the
// chunks of pixels at the sides of the rectangle. The first method that
// reads or writes the pixels of the mask, such as Data, DataMut, FillPath,
// IntersectPath or Invert, makes its pixels. A rect with no pixel inside
// the mask gives the mask of NewMask.
//
// opts configure the mask, as D20 says.
func NewRectMask(width, height uint32, rect IntRect, opts ...Option) (*Mask, bool) {
	m, ok := NewMask(width, height, opts...)
	if !ok {
		return nil, false
	}
	r, ok := m.size.ToIntRect(0, 0).Intersect(rect)
	if !ok {
		return m, true
	}
	m.rect = true
	m.rectLeft, m.rectTop = int(r.Left()), int(r.Top())
	m.rectRight, m.rectBottom = int(r.Right()), int(r.Bottom())
	m.edge = make([]byte, min(int(width), m.rectRight+maskChunk-1))
	fill255(m.edge[m.rectLeft:m.rectRight])
	return m, true
}

// fill255 sets each byte of b to 255.
func fill255(b []byte) {
	for i := range b {
		b[i] = 255
	}
}

// unrect makes a mask of NewRectMask hold the pixels of its rectangle, with
// the margin of padColumns, as a mask of pixels (D35, D86).
func (m *Mask) unrect() {
	if !m.rect {
		return
	}
	x0, y0, x1, y1 := m.rectLeft, m.rectTop, m.rectRight, m.rectBottom
	m.dropRect()
	left, right := m.padColumns(x0, x1)
	stride := right - left
	data := make([]byte, left+stride*(y1-y0))
	for y := y0; y < y1; y++ {
		start := stride*(y-y0) + x0
		fill255(data[start : start+x1-x0])
	}
	m.setData(data, y0, left, stride)
}

// dropRect makes a mask of NewRectMask a mask that holds no pixel (D86).
func (m *Mask) dropRect() {
	m.rect, m.edge = false, nil
	m.rectLeft, m.rectTop, m.rectRight, m.rectBottom = 0, 0, 0, 0
}

// rectAt returns the pixel (x, y) of a mask of NewRectMask (D86).
func (m *Mask) rectAt(x, y int) byte {
	if x >= m.rectLeft && x < m.rectRight && y >= m.rectTop && y < m.rectBottom {
		return 255
	}
	return 0
}

// rectSubmask returns the submask of a mask of NewRectMask inside r, a
// rectangle inside the mask (D86).
func (m *Mask) rectSubmask(r IntRect) subMaskRef {
	x, y := int(r.Left()), int(r.Top())
	s := subMaskRef{
		size:   r.Size(),
		rect:   true,
		top:    max(m.rectTop-y, 0),
		bottom: min(m.rectBottom-y, int(r.Height())),
		left:   max(m.rectLeft-x, 0),
		right:  min(m.rectRight-x, int(r.Width())),
	}
	if s.top >= s.bottom || s.left >= s.right {
		// No pixel of r is in the rectangle, so the blitter draws nothing.
		return subMaskRef{size: r.Size(), rect: true}
	}
	// x is left of rectRight, so edge holds it.
	s.data = m.edge[x:]
	return s
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
		stride: int(pixmap.Width()),
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

	return &Mask{
		data:    data,
		size:    size,
		logger:  newOptions(opts).logger,
		stride:  int(size.Width()),
		exposed: true,
	}, true
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
//
// Port: the first call makes data hold every pixel (D35).
func (m *Mask) Data() []byte {
	m.expose()
	return m.data
}

// DataMut is Mask::data_mut in tiny-skia src/mask.rs. It returns the mutable
// internal data.
//
// Port: the first call makes data hold every pixel (D35).
func (m *Mask) DataMut() []byte {
	m.expose()
	return m.data
}

// Take is Mask::take in tiny-skia src/mask.rs. It consumes the mask and
// returns its owned internal data.
//
// Port: Rust consumes the mask. Go returns the data, and the caller does not
// use m after the call.
func (m *Mask) Take() []byte {
	m.expose()
	return m.data
}

// Clone is the derived Clone impl of Mask in tiny-skia src/mask.rs. It
// returns a copy of the mask with its own data.
//
// Port: the copy shares data with m, and the first write to either mask
// copies it. Clone only sets an atomic flag, so that two goroutines can
// clone a mask at the same time, as they can in upstream. When m cannot
// share its data, as when a caller can hold it, the copy takes only the rows
// of m from its first row that is not 0 to its last (D35). The copy of a
// mask of NewRectMask holds the same rectangle and no pixels (D86).
func (m *Mask) Clone() *Mask {
	c := &Mask{size: m.size, logger: m.logger}
	if m.rect {
		c.rect, c.edge = true, m.edge
		c.rectLeft, c.rectTop, c.rectRight, c.rectBottom = m.rectLeft, m.rectTop, m.rectRight, m.rectBottom
		return c
	}
	if !m.exposed && m.cloned != nil {
		m.cloned.Store(true)
		c.data, c.top, c.left, c.stride, c.cloned = m.data, m.top, m.left, m.stride, m.cloned
		return c
	}
	top, bottom := m.rows()
	for top < bottom && allZero(m.row(top)) {
		top++
	}
	for bottom > top && allZero(m.row(bottom-1)) {
		bottom--
	}
	if top < bottom {
		// The rows keep their columns, so the copy starts left bytes before
		// the first column of the row top.
		start := m.stride * (top - m.top)
		end := m.stride*(bottom-m.top) + m.left
		c.setData(bytes.Clone(m.data[start:end]), top, m.left, m.stride)
	}
	return c
}

// setData makes data, which no other mask holds, the data of the mask, with
// the rows from top and the columns from left to left+stride (D35).
func (m *Mask) setData(data []byte, top, left, stride int) {
	m.data, m.top, m.left, m.stride, m.cloned = data, top, left, stride, new(atomic.Bool)
}

// drop makes the mask hold no pixel (D35).
func (m *Mask) drop() {
	m.data, m.top, m.left, m.stride, m.cloned = nil, 0, 0, 0, nil
}

// isShared reports whether another mask can hold data (D35).
func (m *Mask) isShared() bool {
	return m.cloned != nil && m.cloned.Load()
}

// rows returns the rows that data holds: from top to bottom, without
// bottom (D35).
func (m *Mask) rows() (top, bottom int) {
	if len(m.data) == 0 {
		return m.top, m.top
	}
	return m.top, m.top + (len(m.data)-m.left)/m.stride
}

// columns returns the columns that data holds: from left to right, without
// right (D35).
func (m *Mask) columns() (left, right int) {
	return m.left, m.left + m.stride
}

// row returns the columns that data holds of the row y, which data holds
// (D35).
func (m *Mask) row(y int) []byte {
	start := m.stride*(y-m.top) + m.left
	return m.data[start : start+m.stride]
}

// padColumns returns the columns from x0 to x1 with a margin of
// maskChunk-1 columns on each side, inside the mask (D35, D60).
func (m *Mask) padColumns(x0, x1 int) (left, right int) {
	return max(x0-(maskChunk-1), 0), min(x1+maskChunk-1, int(m.Width()))
}

// dense makes data hold every pixel (D35).
func (m *Mask) dense() {
	m.unrect()
	w, h := int(m.Width()), int(m.Height())
	if m.top == 0 && m.left == 0 && m.stride == w && len(m.data) == w*h {
		return
	}
	data := make([]byte, w*h)
	top, bottom := m.rows()
	for y := top; y < bottom; y++ {
		copy(data[y*w+m.left:], m.row(y))
	}
	m.setData(data, 0, 0, w)
}

// own copies data when another mask can hold it, before a write (D35).
func (m *Mask) own() {
	if m.isShared() {
		m.setData(bytes.Clone(m.data), m.top, m.left, m.stride)
	}
}

// expose makes data hold every pixel, in a buffer that no other mask holds,
// for a caller that can hold it (D35).
func (m *Mask) expose() {
	m.dense()
	m.own()
	m.exposed = true
}

// cover makes data hold the pixels from the column x0 to x1 and from the
// row y0 to y1, without x1 and y1, with the margin of padColumns and the
// pixels that it held before, in a buffer that no other mask shares. A mask
// that a caller can hold already holds every pixel (D35). A mask of
// NewRectMask first makes its pixels (D86).
func (m *Mask) cover(x0, y0, x1, y1 int) {
	m.unrect()
	if m.exposed {
		return
	}
	oldTop, oldBottom := m.rows()
	oldLeft, oldRight := m.columns()
	if x0 < x1 && y0 < y1 {
		x0, x1 = m.padColumns(x0, x1)
		if oldTop < oldBottom {
			x0, x1 = min(x0, oldLeft), max(x1, oldRight)
			y0, y1 = min(y0, oldTop), max(y1, oldBottom)
		}
	} else {
		x0, y0, x1, y1 = oldLeft, oldTop, oldRight, oldBottom
	}
	if y0 >= y1 || (x0 == oldLeft && x1 == oldRight && y0 == oldTop && y1 == oldBottom && !m.isShared()) {
		return
	}
	stride := x1 - x0
	data := make([]byte, x0+stride*(y1-y0))
	for y := oldTop; y < oldBottom; y++ {
		copy(data[stride*(y-y0)+oldLeft:], m.row(y))
	}
	m.setData(data, y0, x0, stride)
}

// asSubmask is Mask::as_submask in tiny-skia src/mask.rs.
//
// Port: the submask holds the pixels of data (D35), or the rectangle of a
// mask of NewRectMask (D86).
func (m *Mask) asSubmask() subMaskRef {
	if m.rect {
		return m.rectSubmask(m.size.ToIntRect(0, 0))
	}
	top, bottom := m.rows()
	left, right := m.columns()
	return subMaskRef{
		size:      m.size,
		realWidth: uint32(m.stride),
		data:      m.data,
		top:       top,
		bottom:    bottom,
		left:      left,
		right:     right,
	}
}

// submask is Mask::submask in tiny-skia src/mask.rs.
//
// Port: the submask holds the pixels of data inside rect (D35), or the
// part of the rectangle of a mask of NewRectMask inside rect (D86).
func (m *Mask) submask(rect IntRect) (subMaskRef, bool) {
	rect, ok := m.size.ToIntRect(0, 0).Intersect(rect)
	if !ok {
		return subMaskRef{}, false
	}
	if m.rect {
		return m.rectSubmask(rect), true
	}
	s := m.subrect(rect)
	return subMaskRef{
		size:      rect.Size(),
		realWidth: uint32(m.stride),
		data:      s.data,
		top:       s.top,
		bottom:    s.bottom,
		left:      s.left,
		right:     s.right,
	}, true
}

// maskRect is the part of the data of a mask inside a rectangle of the mask,
// which subrect returns (D35).
type maskRect struct {
	// data starts at the row top of the rectangle and its column left.
	data []byte
	// top, bottom, left and right are the rows and the columns of the
	// rectangle that data holds, without bottom and right.
	top, bottom, left, right int
}

// subrect returns the part of data inside rect (D35).
func (m *Mask) subrect(rect IntRect) maskRect {
	mt, mb := m.rows()
	ml, mr := m.columns()
	rt, rl := int(rect.Top()), int(rect.Left())
	r := maskRect{
		top:    max(mt-rt, 0),
		bottom: min(mb-rt, int(rect.Height())),
		left:   max(ml-rl, 0),
		right:  min(mr-rl, int(rect.Width())),
	}
	if r.top >= r.bottom || r.left >= r.right {
		return maskRect{}
	}
	r.data = m.data[m.stride*(rt+r.top-mt)+rl:]
	return r
}

// asSubpixmap is Mask::as_subpixmap in tiny-skia src/mask.rs.
//
// Port: the subpixmap holds the pixels of data (D35).
func (m *Mask) asSubpixmap() subPixmapMut {
	top, bottom := m.rows()
	left, right := m.columns()
	return subPixmapMut{
		size:      m.size,
		realWidth: m.stride,
		data:      m.data,
		logger:    m.logger,
		top:       top,
		bottom:    bottom,
		left:      left,
		right:     right,
	}
}

// subpixmap is Mask::subpixmap in tiny-skia src/mask.rs.
//
// Port: the subpixmap holds the pixels of data inside rect (D35).
func (m *Mask) subpixmap(rect IntRect) (subPixmapMut, bool) {
	rect, ok := m.size.ToIntRect(0, 0).Intersect(rect)
	if !ok {
		return subPixmapMut{}, false
	}
	s := m.subrect(rect)
	return subPixmapMut{
		size:      rect.Size(),
		realWidth: m.stride,
		data:      s.data,
		logger:    m.logger,
		top:       s.top,
		bottom:    s.bottom,
		left:      s.left,
		right:     s.right,
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
	m.dense()
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
	// Port: data first holds the pixels that the path can cover (D35).
	w, h := int(m.Width()), int(m.Height())
	m.cover(pathPixelBounds(p, transform, w, h))
	m.fillPath(p, fillRule, antiAlias, transform)
}

// fillPath is the body of Mask::fill_path in tiny-skia src/mask.rs. It
// draws only on the pixels that data holds (D35).
func (m *Mask) fillPath(
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
					fillPathPathAA(p, fillRule, &clipRect, blitter, shiftPathAA, m.logger)
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
				fillPathPathAA(p, fillRule, &clipRect, blitter, shiftPathAA, m.logger)
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

		m.fillPath(p, fillRule, antiAlias, path.IdentityTransform())
	}
}

// IntersectPath is Mask::intersect_path in tiny-skia src/mask.rs. It
// intersects the provided path with the current clipping path.
//
// A temporary mask with the same size as the current one is created.
//
// Port: the temporary mask comes from a pool, and only the pixels that the
// path can cover are cleared and multiplied. The other pixels of the mask
// are 0, as the multiplication by 0 of upstream gives. A clip of a page
// then takes a time that depends on the size of the path, not on the size
// of the page. The bytes are the same as upstream (D60). Then data holds
// only the pixels that the path and the mask both cover, with the margin of
// padColumns (D35). A mask of NewRectMask first makes its pixels (D86).
func (m *Mask) IntersectPath(
	p *Path,
	fillRule FillRule,
	antiAlias bool,
	transform Transform,
) {
	m.unrect()
	w, h := int(m.Width()), int(m.Height())
	x0, y0, x1, y1 := pathPixelBounds(p, transform, w, h)
	oldTop, _ := m.rows()
	oldLeft, oldRight := m.columns()
	if !m.exposed {
		_, oldBottom := m.rows()
		y0, y1 = max(y0, oldTop), min(y1, oldBottom)
		x0, x1 = max(x0, oldLeft), min(x1, oldRight)
	}
	if x0 >= x1 || y0 >= y1 {
		if m.exposed {
			clear(m.data)
		} else {
			m.drop()
		}
		return
	}
	buf, ok := maskPool.Get().(*[]byte)
	if !ok {
		buf = new([]byte)
	}
	defer maskPool.Put(buf)
	// The temporary mask holds the pixels from x0 to x1 and from y0 to y1.
	// The blitter of fillPath draws only there.
	stride := x1 - x0
	n := x0 + stride*(y1-y0)
	if cap(*buf) < n {
		*buf = make([]byte, n)
	}
	data := (*buf)[:n]
	clear(data[x0:])
	submask := &Mask{data: data, size: m.size, logger: m.logger, top: y0, left: x0, stride: stride}
	submask.fillPath(p, fillRule, antiAlias, transform)

	// out holds the result, with the rows from outTop and the columns from
	// outLeft to outLeft+outStride.
	outTop, outLeft, outStride := 0, 0, w
	var out []byte
	switch {
	case m.exposed:
		out = m.data
		clear(out[:y0*w])
		clear(out[y1*w:])
	default:
		outTop = y0
		left, right := m.padColumns(x0, x1)
		outLeft = max(left, oldLeft)
		outStride = min(right, oldRight) - outLeft
		outLen := outLeft + outStride*(y1-y0)
		if m.isShared() {
			out = make([]byte, outLen)
		} else {
			// The result is a part of data with fewer or the same rows and
			// columns. A byte of the result is at the same index as its
			// byte in data, or at a lower index, so the loop below reads
			// each byte of data before it writes over it.
			out = m.data[:outLen]
		}
	}
	for y := y0; y < y1; y++ {
		row := m.row(y)
		sub := submask.row(y)
		start := outStride*(y-outTop) + outLeft
		dst := out[start : start+outStride]
		clear(dst[:x0-outLeft])
		for x := x0; x < x1; x++ {
			dst[x-outLeft] = premultiplyU8(row[x-oldLeft], sub[x-x0])
		}
		clear(dst[x1-outLeft:])
	}
	if !m.exposed {
		if m.isShared() {
			m.setData(out, outTop, outLeft, outStride)
		} else {
			m.data, m.top, m.left, m.stride = out, outTop, outLeft, outStride
		}
	}
}

// maskPool holds the temporary masks of IntersectPath.
var maskPool = sync.Pool{New: func() any { return new([]byte) }}

// pathPixelBounds returns the pixels of a mask of size w by h that the path
// p with transform can cover: the columns x0 to x1 and the rows y0 to y1,
// without x1 and y1. The bounds of the path grow by 2 pixels, for the
// anti-aliasing and the rounding of the transform. A path whose bounds are
// not finite covers the whole mask.
func pathPixelBounds(p *Path, transform Transform, w, h int) (x0, y0, x1, y1 int) {
	r, ok := p.Bounds().Transform(transform)
	if !ok {
		return 0, 0, w, h
	}
	clamp := func(v float32, limit int) int {
		switch {
		case !(v > 0):
			return 0
		case v >= float32(limit):
			return limit
		}
		return int(v)
	}
	const margin = 2
	x0 = clamp(float32(math.Floor(float64(r.Left())))-margin, w)
	y0 = clamp(float32(math.Floor(float64(r.Top())))-margin, h)
	x1 = clamp(float32(math.Ceil(float64(r.Right())))+margin, w)
	y1 = clamp(float32(math.Ceil(float64(r.Bottom())))+margin, h)
	return x0, y0, x1, y1
}

// Invert is Mask::invert in tiny-skia src/mask.rs. It inverts the mask.
//
// Port: data first holds every pixel (D35).
func (m *Mask) Invert() {
	m.dense()
	m.own()
	for i := range m.data {
		m.data[i] = 255 - m.data[i]
	}
}

// Clear is Mask::clear in tiny-skia src/mask.rs. It clears the mask.
//
// It zero-fills the internal data buffer.
//
// Port: a mask that no caller can hold drops its data (D35). A mask of
// NewRectMask drops its rectangle (D86).
func (m *Mask) Clear() {
	m.dropRect()
	if m.exposed {
		clear(m.data)
		return
	}
	m.drop()
}

// String is the Debug impl of Mask in tiny-skia src/mask.rs. It satisfies
// the fmt.Stringer interface.
func (m *Mask) String() string {
	return fmt.Sprintf("Mask { data: %q, width: %d, height: %d }", "...", m.size.Width(), m.size.Height())
}

// subMaskRef is SubMaskRef in tiny-skia src/mask.rs.
//
// Port: data holds the rows from top to bottom and the columns from left to
// right, without bottom and right, and realWidth is its number of columns.
// The byte of the pixel (x, y) is at realWidth*(y-top) + x. The other
// pixels are 0 (D35).
//
// Port: rect is true for a submask of a mask of NewRectMask (D86). The
// submask is then 255 on the columns from left to right of the rows from
// top to bottom, and 0 on the other pixels. data holds one row of the mask
// from the column 0 of the submask, and realWidth is 0, so that the stage
// mask_u8 reads that row for each row.
type subMaskRef struct {
	data                     []byte
	size                     path.IntSize
	realWidth                uint32
	top, bottom, left, right int
	rect                     bool
}

// maskCtx is SubMaskRef::mask_ctx in tiny-skia src/mask.rs.
func (s *subMaskRef) maskCtx() maskCtx {
	return maskCtx{
		data:      s.data,
		realWidth: s.realWidth,
		top:       int32(s.top),
	}
}
