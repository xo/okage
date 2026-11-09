package okage

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage/path"
)

// The tests of this file port no upstream unit test. The tests of
// tiny-skia tests/integration/pixmap.rs are in pixmap_integration_test.go.

// TestCloneRect checks that CloneRect copies the right pixels.
func TestCloneRect(t *testing.T) {
	p, _ := NewPixmap(4, 3)
	pixels := p.PixelsMut()
	for i := range pixels.Len() {
		v := uint8(i)
		pixels.Set(i, premultipliedColorU8FromRGBAUnchecked(v, v, v, 255))
	}
	ir, _ := path.IntRectFromXYWH(1, 1, 5, 5)
	c, ok := p.CloneRect(ir)
	if !ok {
		t.Fatal("CloneRect failed")
	}
	if c.Width() != 3 || c.Height() != 2 {
		t.Fatalf("size %dx%d, want 3x2", c.Width(), c.Height())
	}
	for i, want := range []uint8{5, 6, 7, 9, 10, 11} {
		if got := c.Pixels().At(i).Red(); got != want {
			t.Errorf("pixel %d = %d, want %d", i, got, want)
		}
	}
}

// TestNewPixmapLimits checks the size checks of NewPixmap and the other
// constructors.
func TestNewPixmapLimits(t *testing.T) {
	if _, ok := NewPixmap(0, 10); ok {
		t.Error("NewPixmap(0, 10) is ok")
	}
	if _, ok := NewPixmap(math.MaxInt32/4+1, 1); ok {
		t.Error("NewPixmap with a width above i32::MAX/4 is ok")
	}
	size, _ := path.IntSizeFromWH(2, 2)
	if _, ok := PixmapFromVec(make([]byte, 15), size); ok {
		t.Error("PixmapFromVec with a short buffer is ok")
	}
	if _, ok := PixmapRefFromBytes(make([]byte, 15), 2, 2); ok {
		t.Error("PixmapRefFromBytes with a short buffer is ok")
	}
	if _, ok := PixmapMutFromBytes(make([]byte, 17), 2, 2); !ok {
		t.Error("PixmapMutFromBytes with a long buffer is not ok")
	}
}

// TestPixel checks Pixel at the edges of the pixmap.
func TestPixel(t *testing.T) {
	p, _ := NewPixmap(3, 2)
	if _, ok := p.Pixel(2, 1); !ok {
		t.Error("Pixel(2, 1) is not ok")
	}
	if _, ok := p.Pixel(0, 2); ok {
		t.Error("Pixel(0, 2) is ok")
	}
	if _, ok := p.Pixel(math.MaxUint32, math.MaxUint32); ok {
		t.Error("Pixel(MaxUint32, MaxUint32) is ok")
	}
}

// TestTakeDemultiplied checks TakeDemultiplied.
func TestTakeDemultiplied(t *testing.T) {
	p, _ := NewPixmap(1, 1)
	p.PixelsMut().Set(0, premultipliedColorU8FromRGBAUnchecked(153, 99, 54, 180))
	if got, want := p.TakeDemultiplied(), []byte{217, 140, 77, 180}; !bytes.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestPNGRoundTrip checks that EncodePNG and DecodePNG give back the same
// pixels, and SavePNG and LoadPNG too.
func TestPNGRoundTrip(t *testing.T) {
	p, _ := NewPixmap(5, 4)
	pixels := p.PixelsMut()
	for i := range pixels.Len() {
		pixels.Set(i, ColorU8FromRGBA(uint8(i*10), uint8(i*3), 200, uint8(255-i*12)).Premultiply())
	}
	data, err := p.EncodePNG()
	if err != nil {
		t.Fatal(err)
	}
	q, err := DecodePNG(data)
	if err != nil {
		t.Fatal(err)
	}
	// Demultiplying and premultiplying again can change a pixel, so the
	// test compares with a pixmap that went the same way.
	want, _ := PixmapFromVec(p.Clone().TakeDemultiplied(), p.Size())
	wantPixels := want.PixelsMut()
	for i := range wantPixels.Len() {
		c := wantPixels.At(i)
		wantPixels.Set(i, ColorU8(c).Premultiply())
	}
	if !bytes.Equal(q.Data(), want.Data()) {
		t.Errorf("got %v, want %v", q.Data(), want.Data())
	}

	name := filepath.Join(t.TempDir(), "a.png")
	if err := p.SavePNG(name); err != nil {
		t.Fatal(err)
	}
	r, err := LoadPNG(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.Data(), q.Data()) {
		t.Error("LoadPNG gave other pixels than DecodePNG")
	}
}

// TestDecodePNGFormats encodes images of several types with image/png and
// checks the pixels that DecodePNG gives.
func TestDecodePNGFormats(t *testing.T) {
	r := image.Rect(0, 0, 2, 1)
	gray := image.NewGray(r)
	gray.Pix = []uint8{0, 200}
	gray16 := image.NewGray16(r)
	gray16.SetGray16(1, 0, color.Gray16{Y: 0xABCD})
	nrgba := image.NewNRGBA(r)
	nrgba.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	nrgba64 := image.NewNRGBA64(r)
	nrgba64.SetNRGBA64(0, 0, color.NRGBA64{R: 0xC8FF, G: 0x6400, B: 0x3200, A: 0x8000})
	pal := image.NewPaletted(r, color.Palette{color.NRGBA{R: 200, G: 100, B: 50, A: 128}, color.RGBA{R: 1, G: 2, B: 3, A: 255}})
	pal.Pix = []uint8{0, 1}

	half := ColorU8FromRGBA(200, 100, 50, 128).Premultiply()
	for _, c := range []struct {
		name string
		img  image.Image
		want []PremultipliedColorU8
	}{
		{"gray", gray, []PremultipliedColorU8{
			premultipliedColorU8FromRGBAUnchecked(0, 0, 0, 255),
			premultipliedColorU8FromRGBAUnchecked(200, 200, 200, 255),
		}},
		{"gray16", gray16, []PremultipliedColorU8{
			premultipliedColorU8FromRGBAUnchecked(0, 0, 0, 255),
			premultipliedColorU8FromRGBAUnchecked(0xAB, 0xAB, 0xAB, 255),
		}},
		{"nrgba", nrgba, []PremultipliedColorU8{half, {}}},
		{"nrgba64", nrgba64, []PremultipliedColorU8{half, {}}},
		{"paletted", pal, []PremultipliedColorU8{half, premultipliedColorU8FromRGBAUnchecked(1, 2, 3, 255)}},
	} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, c.img); err != nil {
			t.Fatal(err)
		}
		p, err := DecodePNG(buf.Bytes())
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for i, want := range c.want {
			if got := p.Pixels().At(i); got != want {
				t.Errorf("%s: pixel %d = %v, want %v", c.name, i, got, want)
			}
		}
	}
}

// TestDecodePNGErrors checks the errors of DecodePNG and LoadPNG.
func TestDecodePNGErrors(t *testing.T) {
	if _, err := DecodePNG([]byte("not a png")); !errors.Is(err, ErrDecodingPNG) {
		t.Errorf("got %v, want ErrDecodingPNG", err)
	}
	_, err := LoadPNG(filepath.Join(t.TempDir(), "missing.png"))
	if !errors.Is(err, ErrDecodingPNG) || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("got %v, want ErrDecodingPNG and os.ErrNotExist", err)
	}
}
