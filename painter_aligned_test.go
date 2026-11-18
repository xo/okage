package okage

// This file ports no upstream file. It holds the tests of drawPixmapAligned
// and of stageGatherAligned (D53).

import (
	"bytes"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// TestDrawPixmapAlignedBytes tests the two facts that the shortcuts of
// drawPixmapAligned need: a byte that load_8888 loads and store_8888 stores
// stays the same, and an alpha of 255 loads as exactly 1.
func TestDrawPixmapAlignedBytes(t *testing.T) {
	const factor float32 = 1.0 / 255.0
	for c := range 256 {
		if got := unnormLane(float32(uint8(c)) * factor); got != int32(c) {
			t.Errorf("byte %d stores as %d", c, got)
		}
	}
	if a := float32(uint8(255)) * factor; a != 1 {
		t.Errorf("alpha 255 loads as %v, want 1", a)
	}
}

// alignedRand gives the random pixmaps, masks and paints of the tests of
// drawPixmapAligned.
type alignedRand struct {
	r *rand.Rand
}

// pixels fills data with pixels of each kind that drawPixmapAligned treats
// apart: transparent, opaque, premultiplied, and bytes that are not
// premultiplied.
func (ar alignedRand) pixels(data []byte) {
	r := ar.r
	for i := 0; i < len(data); i += BytesPerPixel {
		px := data[i : i+BytesPerPixel]
		switch r.IntN(5) {
		case 0:
			px[0], px[1], px[2], px[3] = 0, 0, 0, 0
		case 1:
			px[0], px[1], px[2], px[3] = uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255
		case 2:
			a := r.IntN(256)
			px[0], px[1], px[2], px[3] = uint8(r.IntN(a+1)), uint8(r.IntN(a+1)), uint8(r.IntN(a+1)), uint8(a)
		default:
			for j := range px {
				px[j] = uint8(r.IntN(256))
			}
		}
	}
}

// mask returns nil, or a mask of the given size with 0, 255 and other
// values.
func (ar alignedRand) mask(t *testing.T, w, h uint32) *Mask {
	t.Helper()
	r := ar.r
	if r.IntN(3) == 0 {
		return nil
	}
	m, ok := NewMask(w, h)
	if !ok {
		t.Fatalf("NewMask(%d, %d) failed", w, h)
	}
	for i := range m.data {
		switch r.IntN(4) {
		case 0:
		case 1:
			m.data[i] = 255
		default:
			m.data[i] = uint8(r.IntN(256))
		}
	}
	return m
}

// paint returns a PixmapPaint with SourceOver, a random quality, and an
// opacity that is 1, in 0..1, or out of that range.
func (ar alignedRand) paint() PixmapPaint {
	r := ar.r
	p := DefaultPixmapPaint()
	p.Quality = FilterQuality(r.IntN(3))
	opacities := []float32{1, 1, 0, 0.5, 0.25, 1.5, -1, float32(math.NaN())}
	p.Opacity = opacities[r.IntN(len(opacities))]
	if r.IntN(4) == 0 {
		p.Opacity = r.Float32()
	}
	return p
}

// newPixmapT returns a new pixmap of the given size.
func newPixmapT(t *testing.T, w, h uint32) *Pixmap {
	t.Helper()
	p, ok := NewPixmap(w, h)
	if !ok {
		t.Fatalf("NewPixmap(%d, %d) failed", w, h)
	}
	return p
}

// TestDrawPixmapAlignedSameAsPipeline draws random images that are aligned
// to the pixels with drawPixmapAligned and with the pipeline, and compares
// the bytes.
func TestDrawPixmapAlignedSameAsPipeline(t *testing.T) {
	ar := alignedRand{rand.New(rand.NewPCG(53, 1))}
	r := ar.r
	rounds := 3000
	if testing.Short() {
		rounds = 300
	}
	for round := range rounds {
		dw, dh := uint32(1+r.IntN(70)), uint32(1+r.IntN(40))
		sw, sh := uint32(1+r.IntN(80)), uint32(1+r.IntN(50))
		dst := newPixmapT(t, dw, dh)
		ar.pixels(dst.data)
		src := newPixmapT(t, sw, sh)
		ar.pixels(src.data)
		mask := ar.mask(t, dw, dh)
		paint := ar.paint()
		x, y := int32(r.IntN(181)-90), int32(r.IntN(121)-60)
		ts := path.IdentityTransform()
		if r.IntN(2) == 0 {
			ts = path.TransformFromTranslate(float32(r.IntN(101)-50), float32(r.IntN(61)-30))
		}
		// An image that shares its bytes with the destination.
		self := r.IntN(8) == 0

		fast := dst.Clone()
		slow := dst.Clone()
		fastSrc, slowSrc := src.AsRef(), src.AsRef()
		if self {
			fastSrc, slowSrc = fast.AsRef(), slow.AsRef()
		}
		fm, sm := fast.AsMut(), slow.AsMut()
		if !fm.drawPixmapAligned(x, y, fastSrc, &paint, ts, mask) {
			t.Fatalf("round %d: drawPixmapAligned did not draw", round)
		}
		sm.drawPixmapPipeline(x, y, slowSrc, &paint, ts, mask)
		if !bytes.Equal(fast.data, slow.data) {
			t.Fatalf("round %d: the bytes differ: dst %dx%d, src %dx%d, at %d, %d, %v, opacity %v, mask %v, self %v",
				round, dw, dh, sw, sh, x, y, ts, paint.Opacity, mask != nil, self)
		}
	}
}

// TestDrawPixmapAlignedExhaustive draws an image with each pair of a color
// byte and an alpha byte onto destinations of each byte, with and without
// opacity and mask, and compares drawPixmapAligned with the pipeline.
func TestDrawPixmapAlignedExhaustive(t *testing.T) {
	src := newPixmapT(t, 256, 256)
	for y := range 256 {
		for x := range 256 {
			i := (y*256 + x) * BytesPerPixel
			src.data[i], src.data[i+1], src.data[i+2], src.data[i+3] = uint8(x), uint8(255-x), uint8(x^y), uint8(y)
		}
	}
	half, ok := NewMask(256, 256)
	if !ok {
		t.Fatal("NewMask failed")
	}
	for i := range half.data {
		half.data[i] = uint8(i * 7)
	}
	step := 1
	if testing.Short() {
		step = 17
	}
	for _, opacity := range []float32{1, 0.5} {
		for _, mask := range []*Mask{nil, half} {
			paint := DefaultPixmapPaint()
			paint.Opacity = opacity
			for d := 0; d < 256; d += step {
				fast := newPixmapT(t, 256, 256)
				for i := range fast.data {
					fast.data[i] = uint8(d)
				}
				slow := fast.Clone()
				fm, sm := fast.AsMut(), slow.AsMut()
				if !fm.drawPixmapAligned(0, 0, src.AsRef(), &paint, path.IdentityTransform(), mask) {
					t.Fatal("drawPixmapAligned did not draw")
				}
				sm.drawPixmapPipeline(0, 0, src.AsRef(), &paint, path.IdentityTransform(), mask)
				if !bytes.Equal(fast.data, slow.data) {
					t.Fatalf("destination %d, opacity %v, mask %v: the bytes differ", d, opacity, mask != nil)
				}
			}
		}
	}
}

// TestDrawPixmapAlignedFallback makes sure that drawPixmapAligned leaves the
// draws that it cannot do to the pipeline.
func TestDrawPixmapAlignedFallback(t *testing.T) {
	src := newPixmapT(t, 4, 4)
	small := newPixmapT(t, 8, 8)
	other, ok := NewMask(4, 4)
	if !ok {
		t.Fatal("NewMask failed")
	}
	multiply := DefaultPixmapPaint()
	multiply.BlendMode = BlendModeMultiply
	paint := DefaultPixmapPaint()
	tests := []struct {
		name   string
		x      int32
		paint  *PixmapPaint
		ts     Transform
		mask   *Mask
		pixmap *Pixmap
	}{
		{"half pixel", 0, &paint, path.TransformFromTranslate(0.5, 0), nil, small},
		{"scale", 0, &paint, path.TransformFromScale(2, 2), nil, small},
		{"blend mode", 0, &multiply, path.IdentityTransform(), nil, small},
		{"mask size", 0, &paint, path.IdentityTransform(), other, small},
		{"far", 0, &paint, path.TransformFromTranslate(1<<22, 0), nil, small},
		{"far x", 1 << 22, &paint, path.IdentityTransform(), nil, small},
		{"tiles", 0, &paint, path.IdentityTransform(), nil, newPixmapT(t, drawTilerMaxDimensions+1, 1)},
	}
	for _, tt := range tests {
		m := tt.pixmap.AsMut()
		if m.drawPixmapAligned(tt.x, 0, src.AsRef(), tt.paint, tt.ts, tt.mask) {
			t.Errorf("%s: drawPixmapAligned drew", tt.name)
		}
	}
}

// TestGatherAligned runs stageGatherAligned and the stages that it replaces,
// seed_shader, transform and gather, on random states, and compares the
// channels bit for bit. The stages after them then give the same bytes.
func TestGatherAligned(t *testing.T) {
	r := rand.New(rand.NewPCG(53, 2))
	for round := range 20000 {
		w, h := 1+r.IntN(40), 1+r.IntN(20)
		data := make([]byte, w*h*BytesPerPixel)
		for i := range data {
			data[i] = uint8(r.IntN(256))
		}
		src, ok := PixmapRefFromBytes(data, uint32(w), uint32(h))
		if !ok {
			t.Fatal("PixmapRefFromBytes failed")
		}
		tx, ty := r.IntN(121)-60, r.IntN(61)-30
		if r.IntN(20) == 0 {
			tx, ty = r.IntN(2*gatherAlignedMax+1)-gatherAlignedMax, r.IntN(2*gatherAlignedMax+1)-gatherAlignedMax
		}
		ts := path.TransformFromTranslate(float32(tx), float32(ty))
		dx, dy := r.IntN(100), r.IntN(100)
		if r.IntN(10) == 0 {
			// the path for a pixel past gatherAlignedMax
			dx, dy = gatherAlignedMax+r.IntN(1000), r.IntN(gatherAlignedMax*2)
		}
		tail := highpStageWidth
		if r.IntN(2) == 0 {
			tail = 1 + r.IntN(highpStageWidth)
		}

		run := func(stages ...func(*highpPipeline)) *highpPipeline {
			ctx := newContext()
			ctx.transform = ts
			ctx.gatherAligned = gatherAlignedCtx{x: tx, y: ty, transform: ts}
			p := &highpPipeline{
				functions: append(stages, (*highpPipeline).justReturn),
				pixmapSrc: src,
				ctx:       &ctx,
				tail:      tail,
				dx:        dx,
				dy:        dy,
			}
			// garbage in the destination registers, which seed_shader clears
			for i := range p.dr {
				p.dr[i], p.da[i] = float32(i), -1
			}
			p.nextStage()
			return p
		}
		ref := []func(*highpPipeline){(*highpPipeline).seedShader}
		if !ts.IsIdentity() {
			ref = append(ref, (*highpPipeline).transform)
		}
		ref = append(ref, (*highpPipeline).gather)
		want := run(ref...)
		got := run((*highpPipeline).gatherAligned)
		for _, c := range []struct {
			name      string
			got, want *wide.F32x8
		}{
			{"r", &got.r, &want.r}, {"g", &got.g, &want.g}, {"b", &got.b, &want.b}, {"a", &got.a, &want.a},
			{"dr", &got.dr, &want.dr}, {"dg", &got.dg, &want.dg}, {"db", &got.db, &want.db}, {"da", &got.da, &want.da},
		} {
			if i, ok := sameF32x8(c.got, c.want); !ok {
				t.Fatalf("round %d: %dx%d, move %d, %d, at %d, %d: lane %d of %s is %v, want %v",
					round, w, h, tx, ty, dx, dy, i, c.name, c.got[i], c.want[i])
			}
		}
	}
}

// TestDrawPixmapAlignedEdges draws small images at each position near the
// edges of a small pixmap, with the identity and with moves, and compares
// drawPixmapAligned with the pipeline. Upstream rounds the rectangle of an
// image with the identity transform, so that an image of one column at x =
// -1 still draws the column x = 0.
func TestDrawPixmapAlignedEdges(t *testing.T) {
	ar := alignedRand{rand.New(rand.NewPCG(53, 3))}
	for _, move := range [][2]float32{{0, 0}, {1, 0}, {0, -1}, {-2, 3}, {-1, -1}} {
		ts := path.TransformFromTranslate(move[0], move[1])
		for sw := uint32(1); sw <= 3; sw++ {
			for sh := uint32(1); sh <= 3; sh++ {
				src := newPixmapT(t, sw, sh)
				ar.pixels(src.data)
				for x := int32(-5); x <= 6; x++ {
					for y := int32(-5); y <= 6; y++ {
						dst := newPixmapT(t, 5, 4)
						ar.pixels(dst.data)
						paint := ar.paint()
						mask := ar.mask(t, 5, 4)
						fast, slow := dst.Clone(), dst.Clone()
						fm, sm := fast.AsMut(), slow.AsMut()
						if !fm.drawPixmapAligned(x, y, src.AsRef(), &paint, ts, mask) {
							t.Fatal("drawPixmapAligned did not draw")
						}
						sm.drawPixmapPipeline(x, y, src.AsRef(), &paint, ts, mask)
						if !bytes.Equal(fast.data, slow.data) {
							t.Fatalf("src %dx%d at %d, %d, move %v: the bytes differ", sw, sh, x, y, move)
						}
					}
				}
			}
		}
	}
}
