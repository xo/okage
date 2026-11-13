package lottie

import (
	"errors"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xo/okage"
)

// loadTestdata returns the composition of a file of testdata.
func loadTestdata(t *testing.T, name string) *Composition {
	t.Helper()
	buf, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load(buf)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestFrameCountAndDuration(t *testing.T) {
	c := loadTestdata(t, "1f525-fire.json")
	if got := c.FrameCount(); got != 65 {
		t.Errorf("expected 65 frames, got %d", got)
	}
	if got, exp := c.FrameDuration(), time.Second/60; got != exp {
		t.Errorf("expected %v, got %v", exp, got)
	}
	if got, exp := c.Duration(), 65*(time.Second/60); got != exp {
		t.Errorf("expected %v, got %v", exp, got)
	}
	var empty Composition
	if empty.FrameCount() != 0 || empty.FrameDuration() != 0 || empty.Duration() != 0 {
		t.Errorf("expected 0 for an empty composition")
	}
}

func TestImage(t *testing.T) {
	c := loadTestdata(t, "1f525-fire.json")
	img, err := c.Image(32, 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != (image.Point{X: 200, Y: 200}) {
		t.Errorf("expected 200x200, got %v", got)
	}
	drawn := false
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] != 0 {
			drawn = true
			break
		}
	}
	if !drawn {
		t.Error("expected the frame to have something drawn on it")
	}
	// Image draws as RenderFrame does with the same scale.
	pm, _ := okage.NewPixmap(200, 200)
	m := pm.AsMut()
	if err := RenderFrame(c, c.Frames[0]+32, AffineScale(200.0/1024), &m); err != nil {
		t.Fatal(err)
	}
	if string(pm.Data()) != string(img.Pix) {
		t.Error("expected the same pixels as RenderFrame")
	}
	for _, test := range []struct {
		name    string
		n, w, h int
	}{
		{"negative frame", -1, 10, 10},
		{"frame past the end", 65, 10, 10},
		{"no width", 0, 0, 10},
		{"no height", 0, 10, 0},
		{"too wide", 0, 1 << 40, 10},
	} {
		if _, err := c.Image(test.n, test.w, test.h); !errors.Is(err, okage.ErrLottieEmpty) {
			t.Errorf("%s: expected ErrLottieEmpty, got %v", test.name, err)
		}
	}
}
