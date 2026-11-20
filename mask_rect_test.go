package okage

// This file ports no upstream file. It tests that a mask of NewRectMask,
// which holds a rectangle and no pixels, draws the same bytes as a mask that
// holds the same pixels, and that each method of Mask makes its pixels
// (D86).

import (
	"bytes"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/xo/okage/path"
)

// randomIntRect returns a rectangle for a mask of w by h: the whole mask,
// one column, one row, a rectangle outside the mask, a rectangle larger
// than the mask, a rectangle that crosses the first tile edge, or any other
// rectangle.
func randomIntRect(t *testing.T, r *rand.Rand, w, h uint32) IntRect {
	t.Helper()
	iw, ih := int(w), int(h)
	var x, y, rw, rh int
	switch r.IntN(9) {
	case 0:
		x, y, rw, rh = 0, 0, iw, ih
	case 1:
		x, y, rw, rh = r.IntN(iw), r.IntN(ih+10)-5, 1, 1+r.IntN(ih+5)
	case 2:
		x, y, rw, rh = r.IntN(iw+10)-5, r.IntN(ih), 1+r.IntN(iw+5), 1
	case 3:
		// A rectangle with no pixel inside the mask.
		rw, rh = 1+r.IntN(20), 1+r.IntN(20)
		x, y = r.IntN(iw), r.IntN(ih)
		switch r.IntN(4) {
		case 0:
			x = iw + r.IntN(5)
		case 1:
			x = -rw - r.IntN(5)
		case 2:
			y = ih + r.IntN(5)
		default:
			y = -rh - r.IntN(5)
		}
	case 4:
		x, y = -r.IntN(20), -r.IntN(20)
		rw, rh = iw+40, ih+40
	case 5:
		if w > drawTilerMaxDimensions {
			x, y, rw, rh = int(drawTilerMaxDimensions)-r.IntN(30), 0, 1+r.IntN(60), ih
			break
		}
		fallthrough
	default:
		x, y = r.IntN(iw+25)-20, r.IntN(ih+25)-20
		rw, rh = 1+r.IntN(min(iw+30, 120)), 1+r.IntN(ih+30)
	}
	rect, ok := path.IntRectFromXYWH(int32(x), int32(y), uint32(rw), uint32(rh))
	if !ok {
		t.Fatalf("IntRectFromXYWH(%d, %d, %d, %d) failed", x, y, rw, rh)
	}
	return rect
}

// newRectMask returns NewRectMask(w, h, rect) or fails the test.
func newRectMask(t *testing.T, w, h uint32, rect IntRect) *Mask {
	t.Helper()
	m, ok := NewRectMask(w, h, rect)
	if !ok {
		t.Fatalf("NewRectMask(%d, %d, %v) failed", w, h, rect)
	}
	return m
}

// pixelMask returns a mask of w by h that holds the pixels of rect: 255
// inside rect and 0 outside. When dense is true, the mask holds every
// pixel. Else it holds only the rows and the columns of rect, as FillPath
// makes them (D35).
func pixelMask(t *testing.T, w, h uint32, rect IntRect, dense bool) *Mask {
	t.Helper()
	m, ok := NewMask(w, h)
	if !ok {
		t.Fatalf("NewMask(%d, %d) failed", w, h)
	}
	if !dense {
		m.FillPath(path.PathFromRect(rect.ToRect()), FillRuleWinding, false, IdentityTransform())
		return m
	}
	data := m.DataMut()
	for y := max(int(rect.Top()), 0); y < min(int(rect.Bottom()), int(h)); y++ {
		for x := max(int(rect.Left()), 0); x < min(int(rect.Right()), int(w)); x++ {
			data[y*int(w)+x] = 255
		}
	}
	return m
}

// TestNewRectMask tests the masks that NewRectMask makes.
func TestNewRectMask(t *testing.T) {
	rect, ok := path.IntRectFromXYWH(2, 3, 4, 5)
	if !ok {
		t.Fatal("IntRectFromXYWH failed")
	}
	if _, ok := NewRectMask(0, 10, rect); ok {
		t.Error("NewRectMask(0, 10) did not fail")
	}
	m := newRectMask(t, 40, 10, rect)
	if !m.rect || len(m.data) != 0 || len(m.edge) != 6+maskChunk-1 {
		t.Errorf("rect %v, data %d, edge %d: want a rectangle with no pixels and an edge of %d", m.rect, len(m.data), len(m.edge), 6+maskChunk-1)
	}
	if m.Width() != 40 || m.Height() != 10 || m.String() != "Mask { data: \"...\", width: 40, height: 10 }" {
		t.Errorf("size %dx%d, String %q", m.Width(), m.Height(), m.String())
	}
	// A rectangle with no pixel inside the mask gives the mask of NewMask.
	out, ok := path.IntRectFromXYWH(40, 0, 4, 5)
	if !ok {
		t.Fatal("IntRectFromXYWH failed")
	}
	e := newRectMask(t, 40, 10, out)
	if e.rect || len(e.data) != 0 {
		t.Errorf("a rectangle outside the mask gave rect %v, data %d", e.rect, len(e.data))
	}
	if !bytes.Equal(e.Data(), make([]byte, 400)) {
		t.Error("a rectangle outside the mask is not 0")
	}
}

// TestRectMaskBytes tests that each draw through a mask of NewRectMask
// gives the same bytes as through a mask of the same pixels, for each blend
// mode and space of color, in lowp and in highp, with and without tiles,
// and that no draw makes the pixels of the mask.
func TestRectMaskBytes(t *testing.T) {
	r := rand.New(rand.NewPCG(86, 1))
	ar := alignedRand{r}
	n := 1500
	if testing.Short() {
		n = 300
	}
	for i := range n {
		w, h := uint32(1+r.IntN(160)), uint32(1+r.IntN(60))
		if i%75 == 0 {
			w, h = drawTilerMaxDimensions+uint32(1+r.IntN(40)), uint32(1+r.IntN(6))
		}
		rect := randomIntRect(t, r, w, h)
		rm := newRectMask(t, w, h, rect)
		pm := pixelMask(t, w, h, rect, i%2 == 0)
		inside := rect.Left() < int32(w) && rect.Top() < int32(h) && rect.Right() > 0 && rect.Bottom() > 0
		if rm.rect != inside {
			t.Fatalf("case %d: %dx%d, rect %v: rect is %v", i, w, h, rect, rm.rect)
		}
		if !bytes.Equal(rm.Clone().Data(), pm.Clone().Data()) {
			t.Fatalf("case %d: %dx%d, rect %v: the pixels differ", i, w, h, rect)
		}

		dst, ok := NewPixmap(w, h)
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(dst.data)
		paint := DefaultPaint()
		paint.BlendMode = BlendMode(r.IntN(int(BlendModeLuminosity) + 1))
		paint.AntiAlias = r.IntN(4) != 0
		paint.ForceHQPipeline = r.IntN(3) == 0
		paint.Colorspace = ColorSpace(r.IntN(int(ColorSpaceFullSRGBGamma) + 1))
		paint.Coverage = Coverage(r.IntN(2))
		paint.StrokeAdjust = r.IntN(6) == 0
		src, ok := NewPixmap(uint32(1+r.IntN(9)), uint32(1+r.IntN(9)))
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(src.data)
		switch r.IntN(4) {
		case 0:
			paint.SetColorRGBA8(uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255)
		case 1:
			paint.SetColorRGBA8(uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)))
		case 2:
			paint.Shader = NewPattern(src.AsRef(), SpreadModeRepeat, FilterQuality(r.IntN(3)), 0.8,
				path.TransformFromRow(1.3, 0.2, -0.1, 0.9, 0.5, 1.5))
		default:
			stops := []GradientStop{
				NewGradientStop(0, ColorFromRGBA8(uint8(r.IntN(256)), 30, 200, uint8(r.IntN(256)))),
				NewGradientStop(1, ColorFromRGBA8(10, uint8(r.IntN(256)), 90, 255)),
			}
			if s, ok := NewLinearGradient(path.PointFromXY(0, 0), path.PointFromXY(float32(w), float32(h)+0.5), stops, SpreadModePad, IdentityTransform()); ok {
				paint.Shader = s
			}
		}
		p := randomPath(r, -10, int(max(w, h))+10)
		if r.IntN(3) == 0 {
			// A small path, which can be outside the rectangle.
			c := r.IntN(int(max(w, h)))
			p = randomPath(r, c-8, c+8)
		}
		transform := IdentityTransform()
		if r.IntN(3) == 0 {
			transform = path.TransformFromRow(0.9, 0.3, -0.2, 1.1, 1.5, -2.25)
		}
		stroke := DefaultStroke()
		switch r.IntN(4) {
		case 0:
			stroke.Width = 0
		case 1:
			stroke.Width = 0.5
		default:
			stroke.Width = 3
		}
		if r.IntN(4) == 0 {
			if dash, ok := path.NewStrokeDash([]float32{4, 2}, 1); ok {
				stroke.Dash = dash
			}
		}
		pixmapPaint := DefaultPixmapPaint()
		pixmapPaint.BlendMode = paint.BlendMode
		if r.IntN(2) == 0 {
			pixmapPaint.BlendMode = BlendModeSourceOver
		}
		pixmapPaint.Quality = FilterQuality(r.IntN(3))
		if r.IntN(3) == 0 {
			pixmapPaint.Opacity = 0.6
		}
		pixmapTransform := path.TransformFromTranslate(float32(r.IntN(9)-4), float32(r.IntN(9)-4))
		if r.IntN(3) == 0 {
			pixmapTransform = path.TransformFromScale(1.5, 2.5)
		}
		x, y := int32(r.IntN(int(w))-2), int32(r.IntN(int(h))-2)
		fillRect, ok := path.RectFromXYWH(float32(r.IntN(int(w)+4))-2.5, float32(r.IntN(int(h)+4))-1.75, float32(1+r.IntN(int(w))), float32(1+r.IntN(int(h))))
		if !ok {
			t.Fatal("RectFromXYWH failed")
		}
		if r.IntN(3) == 0 {
			// A rectangle on the pixel grid.
			fillRect = randomIntRect(t, r, w, h).ToRect()
		}
		element, ok := NewPixmap(w, h)
		if !ok {
			t.Fatal("NewPixmap failed")
		}
		ar.pixels(element.data)
		backdrop := PixmapRef{}
		if r.IntN(2) == 0 {
			b, ok := NewPixmap(w, h)
			if !ok {
				t.Fatal("NewPixmap failed")
			}
			ar.pixels(b.data)
			backdrop = b.AsRef()
		}

		draws := []struct {
			name string
			draw func(pixmap *Pixmap, mask *Mask)
		}{
			{"FillPath", func(pixmap *Pixmap, mask *Mask) {
				if p != nil {
					pixmap.FillPath(p, &paint, FillRule(i%2), transform, mask)
				}
			}},
			{"FillRect", func(pixmap *Pixmap, mask *Mask) {
				pixmap.FillRect(fillRect, &paint, transform, mask)
			}},
			{"StrokePath", func(pixmap *Pixmap, mask *Mask) {
				if p != nil {
					pixmap.StrokePath(p, &paint, &stroke, transform, mask)
				}
			}},
			{"DrawPixmap", func(pixmap *Pixmap, mask *Mask) {
				pixmap.DrawPixmap(x, y, src.AsRef(), &pixmapPaint, pixmapTransform, mask)
			}},
			{"ApplyMask", func(pixmap *Pixmap, mask *Mask) {
				pixmap.ApplyMask(mask)
			}},
			{"Knockout", func(pixmap *Pixmap, mask *Mask) {
				pixmap.Knockout(element.AsRef(), backdrop, mask)
			}},
		}
		for _, d := range draws {
			want := dst.Clone()
			d.draw(want, pm)
			got := dst.Clone()
			d.draw(got, rm)
			if inside && (!rm.rect || len(rm.data) != 0) {
				t.Fatalf("case %d: %s made the pixels of the mask", i, d.name)
			}
			if !bytes.Equal(got.data, want.data) {
				t.Fatalf("case %d: %s, %dx%d, rect %v, blend mode %v, hq %v, color space %v: the bytes differ",
					i, d.name, w, h, rect, paint.BlendMode, paint.ForceHQPipeline, paint.Colorspace)
			}
		}
	}
}

// TestRectMaskPixels tests that each method of Mask that reads or writes
// the pixels of a mask of NewRectMask makes its pixels, and gives the same
// result as on a mask of the same pixels.
func TestRectMaskPixels(t *testing.T) {
	const w, h = 70, 30
	rect, ok := path.IntRectFromXYWH(20, 5, 31, 12)
	if !ok {
		t.Fatal("IntRectFromXYWH failed")
	}
	circle, ok := path.PathFromCircle(30, 15, 12)
	if !ok {
		t.Fatal("PathFromCircle failed")
	}
	table := new([256]uint8)
	for i := range table {
		table[i] = uint8(255 - i/2)
	}
	dir := t.TempDir()
	methods := []struct {
		name string
		// do calls the method on m and returns the bytes that it gives,
		// or nil.
		do func(m *Mask) []byte
		// keeps is true when the mask keeps its rectangle.
		keeps bool
	}{
		{"Data", func(m *Mask) []byte { return m.Data() }, false},
		{"DataMut", func(m *Mask) []byte {
			data := m.DataMut()
			data[7] = 99
			return m.Data()
		}, false},
		{"Take", func(m *Mask) []byte { return m.Take() }, false},
		{"FillPath", func(m *Mask) []byte {
			m.FillPath(circle, FillRuleWinding, true, IdentityTransform())
			return nil
		}, false},
		{"IntersectPath", func(m *Mask) []byte {
			m.IntersectPath(circle, FillRuleWinding, true, IdentityTransform())
			return nil
		}, false},
		{"Invert", func(m *Mask) []byte {
			m.Invert()
			return nil
		}, false},
		{"ApplyTransfer", func(m *Mask) []byte {
			m.ApplyTransfer(table)
			return nil
		}, false},
		{"Clear", func(m *Mask) []byte {
			m.Clear()
			return nil
		}, false},
		{"EncodePNG", func(m *Mask) []byte {
			data, err := m.EncodePNG()
			if err != nil {
				t.Fatalf("EncodePNG: %v", err)
			}
			return data
		}, false},
		{"SavePNG", func(m *Mask) []byte {
			name := filepath.Join(dir, "mask.png")
			if err := m.SavePNG(name); err != nil {
				t.Fatalf("SavePNG: %v", err)
			}
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			return data
		}, false},
		{"Clone", func(m *Mask) []byte {
			c := m.Clone()
			if m.rect != c.rect {
				t.Fatal("Clone changed the form of the mask")
			}
			c.FillPath(circle, FillRuleWinding, true, IdentityTransform())
			if c.rect {
				t.Fatal("a write to a clone kept its rectangle")
			}
			return c.Data()
		}, true},
		{"Size", func(m *Mask) []byte {
			s := m.Size()
			return []byte{byte(s.Width()), byte(s.Height()), byte(m.Width()), byte(m.Height())}
		}, true},
	}
	for _, mt := range methods {
		for _, dense := range []bool{false, true} {
			rm := newRectMask(t, w, h, rect)
			pm := pixelMask(t, w, h, rect, dense)
			got, want := mt.do(rm), mt.do(pm)
			if !bytes.Equal(got, want) {
				t.Errorf("%s, dense %v: the method gives other bytes", mt.name, dense)
			}
			if rm.rect != mt.keeps {
				t.Errorf("%s: rect is %v after the method", mt.name, rm.rect)
			}
			if !bytes.Equal(rm.Clone().Data(), pm.Clone().Data()) {
				t.Errorf("%s, dense %v: the pixels differ after the method", mt.name, dense)
			}
			// A draw through the mask after the method gives the same bytes.
			paint := DefaultPaint()
			paint.SetColorRGBA8(10, 200, 30, 180)
			paint.AntiAlias = true
			gotPixmap, ok := NewPixmap(w, h)
			if !ok {
				t.Fatal("NewPixmap failed")
			}
			wantPixmap := gotPixmap.Clone()
			gotPixmap.FillPath(circle, &paint, FillRuleWinding, IdentityTransform(), rm)
			wantPixmap.FillPath(circle, &paint, FillRuleWinding, IdentityTransform(), pm)
			if !bytes.Equal(gotPixmap.data, wantPixmap.data) {
				t.Errorf("%s, dense %v: a draw after the method gives other bytes", mt.name, dense)
			}
		}
	}
}

// TestRectMaskConcurrent tests that goroutines can draw through one mask of
// NewRectMask and change their clones of it at the same time. Run it with
// -race.
func TestRectMaskConcurrent(t *testing.T) {
	rect, ok := path.IntRectFromXYWH(10, 12, 40, 30)
	if !ok {
		t.Fatal("IntRectFromXYWH failed")
	}
	m := newRectMask(t, 64, 64, rect)
	circle, ok := path.PathFromCircle(32, 32, 25)
	if !ok {
		t.Fatal("PathFromCircle failed")
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			pixmap, ok := NewPixmap(64, 64)
			if !ok {
				t.Error("NewPixmap failed")
				return
			}
			paint := DefaultPaint()
			paint.AntiAlias = true
			pixmap.FillPath(circle, &paint, FillRuleWinding, IdentityTransform(), m)
			c := m.Clone()
			c.IntersectPath(circle, FillRuleWinding, true, IdentityTransform())
		})
	}
	wg.Wait()
	if !m.rect {
		t.Fatal("a clone changed the mask")
	}
}
