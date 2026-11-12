// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// randomPremultiplied returns n premultiplied pixels with random colors.
func randomPremultiplied(r *rand.Rand, n int) []byte {
	data := make([]byte, n*4)
	for i := range n {
		a := r.IntN(256)
		if r.IntN(4) == 0 {
			a = []int{0, 255}[r.IntN(2)]
		}
		for c := range 3 {
			data[i*4+c] = uint8(r.IntN(a + 1))
		}
		data[i*4+3] = uint8(a)
	}
	return data
}

// TestComposite checks composite against DrawPixmap of tiny-skia for each
// Porter-Duff mode, with an opacity and a mask, within 2 in each channel.
func TestComposite(t *testing.T) {
	const w, h = 32, 32
	r := rand.New(rand.NewPCG(1, 2))
	modes := map[Compose]okage.BlendMode{
		ComposeSrcOver: okage.BlendModeSourceOver, ComposeSrcIn: okage.BlendModeSourceIn,
		ComposeSrcOut: okage.BlendModeSourceOut, ComposeDestIn: okage.BlendModeDestinationIn,
		ComposeDestOut: okage.BlendModeDestinationOut, ComposeXor: okage.BlendModeXor,
		ComposeDestOver: okage.BlendModeDestinationOver, ComposeSrcAtop: okage.BlendModeSourceAtop,
		ComposeDestAtop: okage.BlendModeDestinationAtop, ComposePlus: okage.BlendModePlus,
	}
	for compose, mode := range modes {
		for _, alpha := range []float32{1, 0.6} {
			src := randomPremultiplied(r, w*h)
			dst := randomPremultiplied(r, w*h)
			mask, _ := okage.NewMask(w, h)
			for i := range mask.DataMut() {
				mask.DataMut()[i] = uint8(r.IntN(256))
			}
			want, _ := okage.NewPixmap(w, h)
			copy(want.DataMut(), dst)
			srcPm, _ := okage.NewPixmap(w, h)
			copy(srcPm.DataMut(), src)
			paint := okage.DefaultPixmapPaint()
			paint.BlendMode = mode
			paint.Opacity = alpha
			wm := want.AsMut()
			wm.DrawPixmap(0, 0, srcPm.AsRef(), &paint, path.IdentityTransform(), mask)

			got := append([]byte(nil), dst...)
			composite(got, src, mask.Data(), w, pixRect{0, 0, w, h}, compose, alpha)
			for i := range got {
				d := int(got[i]) - int(want.Data()[i])
				if d < -2 || d > 2 {
					t.Fatalf("mode %v alpha %v: byte %d is %d, want %d", compose, alpha, i, got[i], want.Data()[i])
				}
			}
		}
	}
}

// BenchmarkRenderFrame renders a frame of each test file at its size.
func BenchmarkRenderFrame(b *testing.B) {
	files, _ := filepath.Glob(filepath.Join("testdata", "*.json"))
	for _, f := range files {
		b.Run(filepath.Base(f), func(b *testing.B) {
			data, err := os.ReadFile(f)
			if err != nil {
				b.Fatal(err)
			}
			c, err := Parse(data)
			if err != nil {
				b.Fatal(err)
			}
			pm, _ := okage.NewPixmap(uint32(c.Width), uint32(c.Height))
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				m := pm.AsMut()
				m.Fill(okage.Color{})
				if err := RenderFrame(c, c.Frames[0]+float64(i%int(c.Frames[1]-c.Frames[0])), IdentityAffine, &m); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
