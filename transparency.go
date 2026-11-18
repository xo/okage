package okage

import (
	"github.com/xo/okage/internal/wide"
)

// This file ports no upstream file. It holds the parts of the transparency
// model of PDF that tiny-skia does not have: a mask of luminance over a
// backdrop color, a transfer function for a mask, and the compositing of a
// group that is not isolated or that is a knockout group (D56). Each
// function works on premultiplied RGBA8 pixels, and each one gives the same
// bytes in every build.

// MaskFromLuminance makes a mask from the luminance of pixmap over an opaque
// backdrop of the color backdrop. It is the soft mask of luminosity of PDF
// with the backdrop color BC (ISO 32000-2, 11.6.5.2). The value of each
// pixel is the luminance of the pixel of pixmap, composited with the normal
// blend mode over the backdrop.
//
// The function ignores the alpha of backdrop, because the backdrop of PDF is
// opaque. With ColorBlack, or with any color of a luminance of zero, the
// mask has the same bytes as MaskFromPixmap with MaskTypeLuminance.
//
// opts configure the mask, as D20 says.
func MaskFromLuminance(pixmap PixmapRef, backdrop Color, opts ...Option) *Mask {
	dataLen := int(pixmap.Width()) * int(pixmap.Height())
	mask := &Mask{
		data:   make([]byte, dataLen),
		size:   pixmap.Size(),
		logger: newOptions(opts).logger,
	}
	lumaBC := float32(backdrop.Red()*0.2126) + float32(backdrop.Green()*0.7152) + float32(backdrop.Blue()*0.0722)
	pixels := pixmap.Pixels()
	n := min(pixels.Len(), len(mask.data))
	for i := range n {
		p := pixels.At(i)
		// The same steps as MaskFromPixmap, so that a black backdrop gives
		// the same bits.
		r := float32(p.Red()) / 255.0
		g := float32(p.Green()) / 255.0
		b := float32(p.Blue()) / 255.0
		a := float32(p.Alpha()) / 255.0
		if p.Alpha() != 0 {
			r /= a
			g /= a
			b /= a
		}
		luma := float32(r*0.2126) + float32(g*0.7152) + float32(b*0.0722)
		// The luminance is linear, so the luminance of the pixel over the
		// backdrop is luma*a + lumaBC*(1-a). A luminance of zero adds zero.
		v := float32(luma*a) + float32(lumaBC*(1.0-a))
		mask.data[i] = wide.TruncU8(ceilF32(clampF32(float32(v*255.0), 0.0, 255.0)))
	}
	return mask
}

// ApplyTransfer changes each value v of the mask to table[v]. It applies
// the transfer function TR of a soft mask of PDF (ISO 32000-2, 11.6.5.2),
// with the function sampled at the 256 values of a byte. table[i] is the
// value of the function at i/255, times 255.
func (m *Mask) ApplyTransfer(table *[256]uint8) {
	for i, v := range m.data {
		m.data[i] = table[v]
	}
}

// RemoveBackdrop calls PixmapMut.RemoveBackdrop on the pixels of p. See
// PixmapMut.RemoveBackdrop for details.
func (p *Pixmap) RemoveBackdrop(backdrop, group PixmapRef) {
	m := p.AsMut()
	m.RemoveBackdrop(backdrop, group)
}

// Knockout calls PixmapMut.Knockout on the pixels of p. See
// PixmapMut.Knockout for details.
func (p *Pixmap) Knockout(element, backdrop PixmapRef, shape *Mask) {
	m := p.AsMut()
	m.Knockout(element, backdrop, shape)
}

// RemoveBackdrop turns the result of a group that is not isolated into the
// result of the group alone, with the contribution of the backdrop removed
// (ISO 32000-2, 11.6.3). A caller then draws the pixmap onto the backdrop
// with DrawPixmap, with the blend mode, the opacity and the mask of the
// group.
//
// A group that is not isolated starts as a copy of its backdrop. Its
// elements draw onto that copy, so that their blend modes see the backdrop.
// m holds that result, and backdrop holds the backdrop of the group. The
// alpha channel of group holds the alpha of the group alone. A caller gets
// it when it draws each element of the group a second time, onto a
// transparent pixmap. The colors of group are not read.
//
// PDF gives the color of the result as C = Cn + (Cn - C0) * (a0/ag - a0),
// with the alpha ag. Cn is the color of m, C0 the color of the backdrop, a0
// the alpha of the backdrop, and ag the alpha of group. With premultiplied
// colors this is c = cn - c0*(1-ag). The function rounds c0*(1-ag) to the
// nearest integer, and clamps c between 0 and ag.
//
// When the blend mode of the group is SourceOver, its opacity is 1 and it
// has no mask, DrawPixmap draws the result onto the backdrop and gives the
// colors of m again, where no clamp changed them. Over an opaque backdrop,
// it gives the same bytes as m. Over a backdrop that is not opaque, the
// alpha can differ by 1, because the alpha of m is the union a0 + ag -
// a0*ag, and the pipeline rounds it in other steps.
//
// m, backdrop and group must have the same size. Else the function logs a
// warning to the logger of m and does nothing.
func (m *PixmapMut) RemoveBackdrop(backdrop, group PixmapRef) {
	if m.Size() != backdrop.Size() || m.Size() != group.Size() {
		warn(m.logger, "RemoveBackdrop: the pixmaps must have the same size")
		return
	}
	dst := m.data
	bd := backdrop.data
	gd := group.data
	n := min(len(dst), len(bd), len(gd))
	for i := 0; i+BytesPerPixel <= n; i += BytesPerPixel {
		ag := int32(gd[i+3])
		inv := 255 - ag
		for k := range 3 {
			c := int32(dst[i+k]) - divRound255(int32(bd[i+k])*inv)
			dst[i+k] = uint8(clampTo(c, ag))
		}
		dst[i+3] = uint8(ag)
	}
}

// Knockout draws one element of a knockout group (ISO 32000-2, 11.4.8 and
// 11.6.3). In a knockout group, each element composites with the backdrop
// of the group, not with the earlier elements. Where the element has shape,
// it replaces what the earlier elements drew.
//
// m holds the result of the earlier elements of the group. element holds the
// element drawn onto a copy of backdrop, the initial backdrop of the group.
// The zero PixmapRef is a transparent backdrop, as in a group that is
// isolated, and element then holds the element drawn onto a transparent
// pixmap. shape holds the shape of the element: its coverage, without its
// opacity. A nil shape covers the whole pixmap.
//
// For each pixel, with the shape f from 0 to 1, the result is
// element + (m - backdrop)*(1-f), on premultiplied colors. It is the result
// of the earlier elements where f is 0, and element where f is 1. The
// function rounds the product to the nearest integer, clamps the alpha
// between 0 and 255, and clamps each color between 0 and the alpha.
//
// A knockout group that is not isolated also needs the alpha of the group
// alone for RemoveBackdrop. A caller gets it with a second call of
// Knockout, on a pixmap that holds that alpha, with the element drawn onto a
// transparent pixmap and the zero PixmapRef as the backdrop.
//
// m, element, a backdrop that is not the zero PixmapRef, and a shape that
// is not nil must have the same size. Else the function logs a warning to
// the logger of m and does nothing.
func (m *PixmapMut) Knockout(element, backdrop PixmapRef, shape *Mask) {
	size := m.Size()
	if element.Size() != size ||
		(backdrop.data != nil && backdrop.Size() != size) ||
		(shape != nil && shape.Size() != size) {
		warn(m.logger, "Knockout: the pixmaps and the mask must have the same size")
		return
	}
	dst := m.data
	ed := element.data
	bd := backdrop.data
	n := min(len(dst), len(ed))
	for i := 0; i+BytesPerPixel <= n; i += BytesPerPixel {
		inv := int32(0)
		if shape != nil {
			inv = 255 - int32(shape.data[i/BytesPerPixel])
		}
		var px [BytesPerPixel]int32
		for k := range BytesPerPixel {
			d := int32(dst[i+k])
			if bd != nil {
				d -= int32(bd[i+k])
			}
			px[k] = int32(ed[i+k]) + divRound255(d*inv)
		}
		a := clampTo(px[3], 255)
		dst[i+0] = uint8(clampTo(px[0], a))
		dst[i+1] = uint8(clampTo(px[1], a))
		dst[i+2] = uint8(clampTo(px[2], a))
		dst[i+3] = uint8(a)
	}
}

// divRound255 returns v/255 rounded to the nearest integer, with the sign
// of v. No integer v is half way between two integers after the division,
// so the rounding has no ties.
func divRound255(v int32) int32 {
	if v < 0 {
		return -((-v + 127) / 255)
	}
	return (v + 127) / 255
}

// clampTo returns v clamped between 0 and hi.
func clampTo(v, hi int32) int32 {
	return max(0, min(v, hi))
}
