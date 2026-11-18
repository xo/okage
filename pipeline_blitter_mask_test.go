package okage

// This file ports no upstream file. It holds the tests of maskBounds, which
// leaves out of a rect the rows and the chunks whose mask is zero, and of
// the pool of IntersectPath.

import (
	"bytes"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/path"
)

// sparseMask returns a mask of the given size that is zero except in a few
// rects, with values of 255 and other values.
func sparseMask(t *testing.T, r *rand.Rand, w, h uint32) *Mask {
	t.Helper()
	m, ok := NewMask(w, h)
	if !ok {
		t.Fatalf("NewMask(%d, %d) failed", w, h)
	}
	for range r.IntN(3) {
		x0, y0 := r.IntN(int(w)), r.IntN(int(h))
		x1, y1 := x0+1+r.IntN(int(w)-x0), y0+1+r.IntN(int(h)-y0)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if r.IntN(4) == 0 {
					m.data[y*int(w)+x] = uint8(r.IntN(256))
				} else {
					m.data[y*int(w)+x] = 255
				}
			}
		}
	}
	return m
}

// TestMaskBoundsBytes tests that a rect that maskBounds cuts gives the same
// bytes as the pipeline on the whole rect, for each blend mode, with a
// solid color, which runs in lowp, and with a pattern, which runs in highp.
func TestMaskBoundsBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	ar := alignedRand{r}
	for i := range 400 {
		w, h := uint32(1+r.IntN(70)), uint32(1+r.IntN(20))
		mask := sparseMask(t, r, w, h)
		dst, ok := NewPixmap(w, h)
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(dst.data)
		paint := DefaultPaint()
		paint.BlendMode = BlendMode(r.IntN(int(BlendModeLuminosity) + 1))
		if r.IntN(2) == 0 {
			paint.SetColorRGBA8(uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)))
		} else {
			src, ok := NewPixmap(5, 7)
			if !ok {
				t.Fatal("NewPixmap failed")
			}
			ar.pixels(src.data)
			paint.Shader = NewPattern(src.AsRef(), SpreadModeRepeat, FilterQualityBilinear, 0.8,
				path.TransformFromRow(1.3, 0.2, -0.1, 0.9, 0.5, 1.5))
		}

		want := dst.Clone()
		submask := asSubmask(mask)
		wantMut := want.AsMut()
		subpix := wantMut.asSubpixmap()
		blitter, ok := newRasterPipelineBlitter(&paint, submask, &subpix)
		if !ok {
			continue
		}
		rect := intSizeToScreenIntRect(want.Size(), 0, 0)
		if !blitter.hasMemset2dColor {
			blitter.blitRectRp.run(&rect, aaMaskCtx{}, blitter.maskCtx(), blitter.pixmapSrc, blitter.pixmap)
		} else {
			blitter.blitRect(&rect)
		}

		got := dst.Clone()
		got.FillRect(rect.ToRect(), &paint, IdentityTransform(), mask)
		if !bytes.Equal(got.data, want.data) {
			t.Fatalf("case %d: %dx%d, blend mode %v: the bytes differ", i, w, h, paint.BlendMode)
		}
	}
}

// TestIntersectPathBytes tests that IntersectPath gives the same mask as a
// new mask with the path, multiplied with the mask, as upstream makes it.
func TestIntersectPathBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for i := range 300 {
		w, h := uint32(1+r.IntN(90)), uint32(1+r.IntN(90))
		mask := sparseMask(t, r, w, h)
		for j := range mask.data {
			if r.IntN(3) == 0 {
				mask.data[j] = uint8(r.IntN(256))
			}
		}
		pb := path.NewPathBuilder()
		pb.MoveTo(float32(r.IntN(120)-15), float32(r.IntN(120)-15))
		for range 1 + r.IntN(4) {
			pb.QuadTo(float32(r.IntN(120)-15), float32(r.IntN(120)-15), float32(r.IntN(120)-15), float32(r.IntN(120)-15))
		}
		pb.Close()
		p, ok := pb.Finish()
		if !ok {
			continue
		}
		transform := IdentityTransform()
		if r.IntN(2) == 0 {
			transform = path.TransformFromRow(0.5+r.Float32(), r.Float32()-0.5, r.Float32()-0.5, 0.5+r.Float32(), float32(r.IntN(20)-10), float32(r.IntN(20)-10))
		}
		fillRule := FillRule(r.IntN(2))
		antiAlias := r.IntN(2) == 0

		want := mask.Clone()
		sub, ok := NewMask(w, h)
		if !ok {
			t.Fatal("NewMask failed")
		}
		sub.FillPath(p, fillRule, antiAlias, transform)
		for k := range want.data {
			want.data[k] = premultiplyU8(want.data[k], sub.data[k])
		}

		mask.IntersectPath(p, fillRule, antiAlias, transform)
		if !bytes.Equal(mask.data, want.data) {
			t.Fatalf("case %d: %dx%d: the masks differ", i, w, h)
		}
	}
}
