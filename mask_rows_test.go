package okage

// This file ports no upstream file. It tests that a mask that holds only
// some rows and columns draws the same bytes as the same mask with every
// pixel (D35).

import (
	"bytes"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/xo/okage/path"
)

// randomPath returns a closed path of quads with points from lo to hi, or
// nil.
func randomPath(r *rand.Rand, lo, hi int) *Path {
	pt := func() float32 { return float32(lo+r.IntN(hi-lo)) + r.Float32() }
	pb := path.NewPathBuilder()
	pb.MoveTo(pt(), pt())
	for range 1 + r.IntN(4) {
		if r.IntN(2) == 0 {
			pb.LineTo(pt(), pt())
		} else {
			pb.QuadTo(pt(), pt(), pt(), pt())
		}
	}
	pb.Close()
	p, ok := pb.Finish()
	if !ok {
		return nil
	}
	return p
}

// rowsMask returns a mask of w by h that holds only the rows and the
// columns of a few paths, made with FillPath, Clone and IntersectPath.
func rowsMask(t *testing.T, r *rand.Rand, w, h uint32) *Mask {
	t.Helper()
	m, ok := NewMask(w, h)
	if !ok {
		t.Fatalf("NewMask(%d, %d) failed", w, h)
	}
	// pathIn returns a path on the whole mask, or a small path.
	pathIn := func() *Path {
		lo, hi := -10, int(max(w, h))+10
		if r.IntN(2) == 0 {
			c := r.IntN(hi)
			lo, hi = c-20, c+20
		}
		return randomPath(r, lo, hi)
	}
	for range 1 + r.IntN(2) {
		if p := pathIn(); p != nil {
			m.FillPath(p, FillRule(r.IntN(2)), r.IntN(2) == 0, IdentityTransform())
		}
	}
	for range r.IntN(3) {
		if r.IntN(2) == 0 {
			m = m.Clone()
		}
		if p := pathIn(); p != nil {
			m.IntersectPath(p, FillRule(r.IntN(2)), r.IntN(2) == 0, IdentityTransform())
		}
	}
	return m
}

// TestMaskRowsBytes tests that each kind of draw through a mask that holds
// only some rows gives the same bytes as through the same mask with every
// row, for each blend mode, in lowp and in highp, with and without tiles.
func TestMaskRowsBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	ar := alignedRand{r}
	for i := range 600 {
		w, h := uint32(1+r.IntN(200)), uint32(1+r.IntN(80))
		if i%50 == 0 {
			w = drawTilerMaxDimensions + uint32(1+r.IntN(40))
		}
		rows := rowsMask(t, r, w, h)
		full := rows.Clone()
		full.DataMut()
		top, bottom := rows.rows()
		if rows.exposed || !full.exposed {
			t.Fatal("the masks do not differ in their rows")
		}

		dst, ok := NewPixmap(w, h)
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(dst.data)
		paint := DefaultPaint()
		paint.BlendMode = BlendMode(r.IntN(int(BlendModeLuminosity) + 1))
		paint.AntiAlias = r.IntN(4) != 0
		src, ok := NewPixmap(uint32(1+r.IntN(9)), uint32(1+r.IntN(9)))
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(src.data)
		if r.IntN(2) == 0 {
			paint.SetColorRGBA8(uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)))
		} else {
			paint.Shader = NewPattern(src.AsRef(), SpreadModeRepeat, FilterQuality(r.IntN(3)), 0.8,
				path.TransformFromRow(1.3, 0.2, -0.1, 0.9, 0.5, 1.5))
		}
		p := randomPath(r, -10, int(max(w, h))+10)
		stroke := DefaultStroke()
		if r.IntN(2) == 0 {
			stroke.Width = 0.5
		} else {
			stroke.Width = 3
		}
		pixmapPaint := DefaultPixmapPaint()
		pixmapPaint.BlendMode = paint.BlendMode
		if r.IntN(2) == 0 {
			pixmapPaint.BlendMode = BlendModeSourceOver
		}
		pixmapPaint.Quality = FilterQuality(r.IntN(3))
		pixmapTransform := path.TransformFromTranslate(float32(r.IntN(9)-4), float32(r.IntN(9)-4))
		if r.IntN(2) == 0 {
			pixmapTransform = path.TransformFromScale(1.5, 2.5)
		}
		x, y := int32(r.IntN(int(w))), int32(r.IntN(int(h)))
		rect, ok := path.RectFromXYWH(float32(r.IntN(int(w)))-0.5, float32(r.IntN(int(h)))+0.25, float32(1+r.IntN(int(w))), float32(1+r.IntN(int(h))))
		if !ok {
			t.Fatal("RectFromXYWH failed")
		}

		draws := []struct {
			name string
			draw func(pixmap *Pixmap, mask *Mask)
		}{
			{"FillPath", func(pixmap *Pixmap, mask *Mask) {
				if p != nil {
					pixmap.FillPath(p, &paint, FillRule(i%2), IdentityTransform(), mask)
				}
			}},
			{"FillRect", func(pixmap *Pixmap, mask *Mask) {
				pixmap.FillRect(rect, &paint, IdentityTransform(), mask)
			}},
			{"StrokePath", func(pixmap *Pixmap, mask *Mask) {
				if p != nil {
					pixmap.StrokePath(p, &paint, &stroke, IdentityTransform(), mask)
				}
			}},
			{"DrawPixmap", func(pixmap *Pixmap, mask *Mask) {
				pixmap.DrawPixmap(x, y, src.AsRef(), &pixmapPaint, pixmapTransform, mask)
			}},
			{"ApplyMask", func(pixmap *Pixmap, mask *Mask) {
				pixmap.ApplyMask(mask.Clone())
			}},
		}
		for _, d := range draws {
			want := dst.Clone()
			d.draw(want, full)
			got := dst.Clone()
			d.draw(got, rows)
			if !bytes.Equal(got.data, want.data) {
				t.Fatalf("case %d: %s, %dx%d, rows %d to %d, blend mode %v: the bytes differ",
					i, d.name, w, h, top, bottom, paint.BlendMode)
			}
		}
		if t2, b2 := rows.rows(); t2 != top || b2 != bottom {
			t.Fatalf("case %d: a draw changed the rows of the mask", i)
		}
	}
}

// TestMaskCloneConcurrent tests that goroutines can clone one mask, draw
// through it and change their clones at the same time, as in upstream, where
// Clone only reads the mask. Run it with -race.
func TestMaskCloneConcurrent(t *testing.T) {
	m, ok := NewMask(64, 64)
	if !ok {
		t.Fatal("NewMask failed")
	}
	circle, ok := path.PathFromCircle(32, 32, 20)
	if !ok {
		t.Fatal("PathFromCircle failed")
	}
	m.FillPath(circle, FillRuleWinding, true, IdentityTransform())
	want := bytes.Clone(m.Clone().Data())
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			pixmap, ok := NewPixmap(64, 64)
			if !ok {
				t.Error("NewPixmap failed")
				return
			}
			paint := DefaultPaint()
			rect, ok := path.RectFromXYWH(float32(i*4), 0, 8, 64)
			if !ok {
				t.Error("RectFromXYWH failed")
				return
			}
			pixmap.FillRect(rect, &paint, IdentityTransform(), m)
			c := m.Clone()
			c.IntersectPath(path.PathFromRect(rect), FillRuleWinding, true, IdentityTransform())
		})
	}
	wg.Wait()
	if !bytes.Equal(m.Clone().Data(), want) {
		t.Fatal("a clone changed the mask")
	}
}
