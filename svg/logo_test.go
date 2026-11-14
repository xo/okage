package svg

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// TestLogo renders the logo of okage, docs/okage.svg, and compares some of
// its pixels with the render of the logo as its author drew it.
//
// The author drew the logo at 1600 by 893 on a gray background, with the glow
// behind the letters as a blurred image. The file is cut to the letters and
// their glow, with no background, and it draws the glow as a blur of the
// letters. That blur differs from the image by 25 at most and by 1.2 on
// average. The test puts the render on the gray of the author, and at the
// pixels of the test it differs by 4 at most. The test module compares the
// whole logo with resvg.
func TestLogo(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "docs", "okage.svg"))
	if err != nil {
		t.Fatal(err)
	}
	img, err := New(testFonts()...).Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Size(); got != (image.Point{X: 1142, Y: 508}) {
		t.Fatalf("expected 1142x508, got %v", got)
	}
	for _, p := range []image.Point{{0, 0}, {1141, 0}, {0, 507}, {1141, 507}} {
		if c := img.RGBAAt(p.X, p.Y); c.A != 0 {
			t.Errorf("corner %v: expected no color, got %v", p, c)
		}
	}
	// The gray of the author, and the place of the cut in the logo of the
	// author.
	const bgR, bgG, bgB = 29, 32, 37
	const left, top = 228, 208
	for _, test := range []struct {
		name    string
		x, y    int
		r, g, b uint8
	}{
		{"inside the o", 400, 460, 30, 35, 38},
		{"stem of the k", 540, 300, 150, 235, 185},
		{"glow between the letters", 800, 446, 40, 41, 39},
		{"tail of the g", 1000, 620, 103, 115, 64},
		{"inside the e", 1250, 420, 130, 75, 34},
	} {
		// The pixels are premultiplied, so the gray adds its share of what
		// the logo leaves uncovered.
		c := img.RGBAAt(test.x-left, test.y-top)
		over := func(v, bg uint8) uint8 {
			return uint8(int(v) + (int(bg)*(255-int(c.A))+127)/255)
		}
		r, g, b := over(c.R, bgR), over(c.G, bgG), over(c.B, bgB)
		if far(r, test.r) || far(g, test.g) || far(b, test.b) {
			t.Errorf("%s: expected %d, %d, %d, got %d, %d, %d", test.name, test.r, test.g, test.b, r, g, b)
		}
	}
}

// far reports whether a and b differ by more than 4.
func far(a, b uint8) bool {
	return max(a, b)-min(a, b) > 4
}
