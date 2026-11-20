package okage

// This file ports no upstream file. It holds the tests of maskBounds, which
// leaves out of a rect the rows and the chunks whose mask is zero, and of
// the pool of IntersectPath.

import (
	"bytes"
	"math/rand/v2"
	"slices"
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
	data := m.DataMut()
	for range r.IntN(3) {
		x0, y0 := r.IntN(int(w)), r.IntN(int(h))
		x1, y1 := x0+1+r.IntN(int(w)-x0), y0+1+r.IntN(int(h)-y0)
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if r.IntN(4) == 0 {
					data[y*int(w)+x] = uint8(r.IntN(256))
				} else {
					data[y*int(w)+x] = 255
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
// new mask with the path, multiplied with the mask, as upstream makes it. It
// tests a mask that a caller can hold, a mask that holds only some rows,
// and a mask that shares its rows with a clone (D35).
func TestIntersectPathBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	for i := range 300 {
		w, h := uint32(1+r.IntN(90)), uint32(1+r.IntN(90))
		mask := sparseMask(t, r, w, h)
		if r.IntN(2) == 0 {
			for j := range mask.data {
				if r.IntN(3) == 0 {
					mask.data[j] = uint8(r.IntN(256))
				}
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

		// want is the work of upstream on masks that hold every row.
		before := bytes.Clone(mask.Data())
		want := bytes.Clone(before)
		sub, ok := NewMask(w, h)
		if !ok {
			t.Fatal("NewMask failed")
		}
		subData := sub.DataMut()
		sub.FillPath(p, fillRule, antiAlias, transform)
		for k := range want {
			want[k] = premultiplyU8(want[k], subData[k])
		}

		// A new mask that holds only the rows of the path.
		fill, ok := NewMask(w, h)
		if !ok {
			t.Fatal("NewMask failed")
		}
		fill.FillPath(p, fillRule, antiAlias, transform)
		if !bytes.Equal(fill.Data(), subData) {
			t.Fatalf("case %d: %dx%d: the fills differ", i, w, h)
		}

		alone := mask.Clone()
		rows := mask.Clone()
		shared := rows.Clone()
		other := shared.Clone()
		mask.IntersectPath(p, fillRule, antiAlias, transform)
		if !bytes.Equal(mask.data, want) {
			t.Fatalf("case %d: %dx%d: the masks differ", i, w, h)
		}
		alone.IntersectPath(p, fillRule, antiAlias, transform)
		if alone.isShared() || !bytes.Equal(alone.Data(), want) {
			t.Fatalf("case %d: %dx%d: the masks that share no rows differ", i, w, h)
		}
		rows.IntersectPath(p, fillRule, antiAlias, transform)
		if !bytes.Equal(rows.Data(), want) {
			t.Fatalf("case %d: %dx%d: the masks of some rows differ", i, w, h)
		}
		shared.IntersectPath(p, fillRule, antiAlias, transform)
		if !bytes.Equal(shared.Data(), want) {
			t.Fatalf("case %d: %dx%d: the shared masks differ", i, w, h)
		}
		if !bytes.Equal(other.Data(), before) {
			t.Fatalf("case %d: %dx%d: IntersectPath changed a clone", i, w, h)
		}
	}
}

// randomRuns returns the runs of one row of w pixels, with transparent,
// opaque and other values, as the scan converter gives them to blitAntiH.
func randomRuns(r *rand.Rand, w uint32) ([]alphaU8, []alphaRun) {
	var aa []alphaU8
	var runs []alphaRun
	for n := uint32(0); n < w; {
		run := min(uint32(1+r.IntN(20)), w-n)
		var alpha alphaU8
		switch r.IntN(3) {
		case 0:
			alpha = AlphaU8Opaque
		case 1:
			alpha = uint8(r.IntN(256))
		}
		aa = append(aa, alpha)
		runs = append(runs, alphaRun(run))
		for range run - 1 {
			aa = append(aa, 0)
			runs = append(runs, 0)
		}
		n += run
	}
	return aa, append(runs, 0)
}

// TestMaskBoundsAntiBytes tests that blitAntiH and blitMask, which leave out
// the chunks and the rows whose mask is zero, give the same bytes as the
// pipeline on each run and on each rect, as upstream runs it, for each blend
// mode, in lowp and in highp.
func TestMaskBoundsAntiBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	ar := alignedRand{r}
	for i := range 600 {
		w, h := uint32(2+r.IntN(70)), uint32(2+r.IntN(20))
		mask := sparseMask(t, r, w, h)
		dst, ok := NewPixmap(w, h)
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(dst.data)
		paint := DefaultPaint()
		paint.BlendMode = BlendMode(r.IntN(int(BlendModeLuminosity) + 1))
		paint.AntiAlias = true
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
		y := uint32(r.IntN(int(h)))
		aa, runs := randomRuns(r, w)
		// A column, as blitV draws it.
		x, vy := uint32(r.IntN(int(w))), uint32(r.IntN(int(h)))
		vh := uint32(1 + r.IntN(int(h-vy)))
		alpha := uint8(r.IntN(256))

		blit := func(pixmap *Pixmap, upstream bool) bool {
			m := pixmap.AsMut()
			subpix := m.asSubpixmap()
			b, ok := newRasterPipelineBlitter(&paint, asSubmask(mask), &subpix)
			if !ok {
				return false
			}
			bounds := screenIntRectFromXYWHSafe(x, vy, 1, vh)
			mb := maskBlitter{image: [2]uint8{alpha, alpha}, bounds: bounds}
			if !upstream {
				b.blitAntiH(0, y, slices.Clone(aa), slices.Clone(runs))
				b.blitMask(&mb, &bounds)
				return true
			}
			// The loops of upstream, without maskBounds.
			rx := uint32(0)
			for k := 0; runs[k] != 0; k += int(runs[k]) {
				width := uint32(runs[k])
				switch a := aa[k]; a {
				case AlphaU8Transparent:
				case AlphaU8Opaque:
					b.blitH(rx, y, width)
				default:
					b.blitAntiHRp.ctx.currentCoverage = float32(a) * (1.0 / 255.0)
					rect := screenIntRectFromXYWHSafe(rx, y, width, 1)
					b.blitAntiHRp.run(&rect, aaMaskCtx{}, b.maskCtx(), b.pixmapSrc, b.pixmap)
				}
				rx += width
			}
			amc := aaMaskCtx{pixels: mb.image, shift: uint(mb.bounds.Left())}
			b.blitMaskRp.run(&bounds, amc, b.maskCtx(), b.pixmapSrc, b.pixmap)
			return true
		}
		want := dst.Clone()
		if !blit(want, true) {
			continue
		}
		got := dst.Clone()
		blit(got, false)
		if !bytes.Equal(got.data, want.data) {
			t.Fatalf("case %d: %dx%d, blend mode %v: the bytes differ", i, w, h, paint.BlendMode)
		}
	}
}
