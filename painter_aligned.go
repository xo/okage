package okage

import "github.com/xo/okage/path"

// This file ports no upstream file. It holds the fast path of DrawPixmap for
// an image that is aligned to the pixels and drawn with SourceOver (D53).

// drawPixmapAligned draws pixmap as DrawPixmap does, without the raster
// pipeline, and reports whether it did. It draws when these are all true:
//
//   - The blend mode is SourceOver.
//   - The transform only moves by whole pixels, and the sum of the move and
//     of x and y is at most gatherAlignedMax on each axis.
//   - The pixmap of m needs no tiles.
//   - The mask, if there is one, has the size of the pixmap of m.
//
// It fills the same rectangle as the pipeline, and it pads the image as the
// pattern of the pipeline does. It gives the same bytes as the pipeline,
// which runs these stages for such a draw: gather, scale_1_float when the opacity is not 1, mask_u8 when
// there is a mask, and then source_over_rgba, or load_dst, source_over and
// store. It does the same float32 operations on each pixel, in groups of
// highpStageWidth pixels that start where the groups of the pipeline start,
// so that an image that shares its bytes with m gives the same bytes too.
//
// It skips the work for two kinds of pixels, which the tests of
// painter_aligned_test.go cover:
//
//   - A pixel whose mask is 0, or whose source is 0 in each channel, keeps
//     the destination. The source is then 0, and source_over gives
//     0 + d * (1 - 0), which stores the byte that load_dst read.
//   - A pixel whose source alpha is 255, with no opacity and a mask of 255
//     or no mask, takes the source bytes. The alpha is then 1, and
//     source_over gives s, which stores the byte that gather read.
func (m *PixmapMut) drawPixmapAligned(
	x, y int32,
	pixmap PixmapRef,
	paint *PixmapPaint,
	transform Transform,
	mask *Mask,
) bool {
	if paint.BlendMode != BlendModeSourceOver || drawTilerRequired(m.Width(), m.Height()) {
		return false
	}
	if mask != nil && mask.Size() != m.Size() {
		return false
	}
	tx, ty, ok := alignedOffset(transform)
	if !ok || x < -gatherAlignedMax || x > gatherAlignedMax || y < -gatherAlignedMax || y > gatherAlignedMax {
		return false
	}
	ox, oy := tx+int(x), ty+int(y)
	if ox < -gatherAlignedMax || ox > gatherAlignedMax || oy < -gatherAlignedMax || oy > gatherAlignedMax {
		return false
	}

	// The rectangle of the destination that the pipeline fills.
	dw, dh := int(m.Width()), int(m.Height())
	sw, sh := int(pixmap.Width()), int(pixmap.Height())
	var left, top, right, bottom int
	if transform.IsIdentity() {
		// FillRect rounds the rectangle with Rect.Round, which moves a
		// negative side by one pixel, and keeps a width and a height of at
		// least one. So an image of one column at x = -1 fills the column
		// x = 0.
		r, ok := pixmap.Size().ToIntRect(x, y).ToRect().Round()
		if !ok {
			return true
		}
		if r, ok = r.Intersect(intSizeToScreenIntRect(m.Size(), 0, 0).ToIntRect()); !ok {
			return true
		}
		left, top, right, bottom = int(r.Left()), int(r.Top()), int(r.Right()), int(r.Bottom())
	} else {
		// FillPath fills the pixels whose centers are in the rectangle.
		left, top = max(ox, 0), max(oy, 0)
		right, bottom = min(ox+sw, dw), min(oy+sh, dh)
		if left >= right || top >= bottom {
			return true
		}
	}

	var opacity float32 = 1
	scaled := false
	if o := path.NormalizedF32Clamped(paint.Opacity); o != path.NormalizedF32One {
		opacity, scaled = o.Get(), true
	}
	// Port: the rows that the mask does not hold are 0, so their pixels
	// keep the destination (D35).
	var maskData []byte
	if mask != nil {
		mt, mb := mask.rows()
		top, bottom = max(top, mt), min(bottom, mb)
		maskData = mask.data
	}

	for row := top; row < bottom; row++ {
		dstRow := m.data[row*dw*BytesPerPixel : (row+1)*dw*BytesPerPixel]
		// The pattern pads, so a pixel past the image takes the pixel at
		// its edge.
		sy := min(max(row-oy, 0), sh-1)
		srcRow := pixmap.data[sy*sw*BytesPerPixel : (sy+1)*sw*BytesPerPixel]
		var maskRow []byte
		if mask != nil {
			maskRow = maskData[(row-mask.top)*dw : (row-mask.top+1)*dw]
		}
		for start := left; start < right; start += highpStageWidth {
			end := min(start+highpStageWidth, right)
			// The pipeline reads the source of a group before it stores
			// the group.
			var src [highpStageWidth * BytesPerPixel]byte
			if start-ox >= 0 && end-ox <= sw {
				copy(src[:], srcRow[(start-ox)*BytesPerPixel:(end-ox)*BytesPerPixel])
			} else {
				for i := range end - start {
					j := min(max(start+i-ox, 0), sw-1) * BytesPerPixel
					copy(src[i*BytesPerPixel:(i+1)*BytesPerPixel], srcRow[j:j+BytesPerPixel])
				}
			}
			for i := range end - start {
				var cover uint8 = 255
				if maskRow != nil {
					cover = maskRow[start+i]
				}
				s := (*[BytesPerPixel]byte)(src[i*BytesPerPixel:])
				d := (*[BytesPerPixel]byte)(dstRow[(start+i)*BytesPerPixel:])
				switch {
				case cover == 0 || *s == [BytesPerPixel]byte{}:
				case s[3] == 255 && cover == 255 && !scaled:
					*d = *s
				default:
					sourceOverAligned(s, d, opacity, scaled, cover)
				}
			}
		}
	}
	return true
}

// sourceOverAligned draws the source pixel s onto the destination pixel d,
// with the float32 operations of the stages gather, scale_1_float, mask_u8,
// load_dst, source_over and store of highp (D53).
func sourceOverAligned(s, d *[BytesPerPixel]byte, opacity float32, scaled bool, cover uint8) {
	// The bytes become floats as load_8888 makes them, from byteUnit.
	var c [BytesPerPixel]float32
	for i := range c {
		c[i] = byteUnit[s[i]]
	}
	if scaled {
		for i := range c {
			c[i] = float32(c[i] * opacity)
		}
	}
	if cover != 255 {
		k := byteOver255[cover]
		for i := range c {
			c[i] = float32(c[i] * k)
		}
	}
	inv := invHighp(c[3])
	for i := range c {
		dc := byteUnit[d[i]]
		d[i] = uint8(unnormLane(madHighp(dc, inv, c[i])))
	}
}
