package svg

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// TestLogo renders the logo of okage, docs/okage.svg, and compares some of
// its pixels with the render of the logo as its author drew it, before the
// file was made smaller. The smaller file holds the blurred glow at a quarter
// of its size, so a pixel can differ by 2 at most. The test module compares
// the whole logo with resvg.
func TestLogo(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "docs", "okage.svg"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := New(testFonts()...).Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != (image.Point{X: 1600, Y: 893}) {
		t.Fatalf("expected 1600x893, got %v", got)
	}
	for _, test := range []struct {
		name    string
		x, y    int
		r, g, b uint8
	}{
		{"background, top left", 0, 0, 29, 32, 37},
		{"background, bottom right", 1599, 892, 29, 32, 37},
		{"inside the o", 400, 460, 30, 35, 38},
		{"stem of the k", 540, 300, 150, 235, 185},
		{"glow between the letters", 800, 446, 40, 41, 39},
		{"tail of the g", 1000, 620, 103, 115, 64},
		{"inside the e", 1250, 420, 130, 75, 34},
	} {
		c := img.RGBAAt(test.x, test.y)
		if c.A != 255 || far(c.R, test.r) || far(c.G, test.g) || far(c.B, test.b) {
			t.Errorf("%s: expected %d, %d, %d, got %v", test.name, test.r, test.g, test.b, c)
		}
	}
}

// far reports whether a and b differ by more than 2.
func far(a, b uint8) bool {
	return max(a, b)-min(a, b) > 2
}
