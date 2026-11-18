package svg_test

// This file ports no upstream test. It checks that a thin stroke draws on an
// image of 2 pixels or less, where resvg draws nothing (D54).

import (
	"fmt"
	"testing"

	"github.com/xo/okage/svg"
)

// TestThinStrokeOnANarrowImage checks that a horizontal or vertical line
// with a stroke of less than a pixel draws on an image that is 2 pixels
// high or wide.
func TestThinStrokeOnANarrowImage(t *testing.T) {
	for _, vertical := range []bool{false, true} {
		w, h, d := 20, 2, "M0 1 H20"
		if vertical {
			w, h, d = 2, 20, "M1 0 V20"
		}
		data := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`+
			`<path d="%s" fill="none" stroke="black" stroke-width="0.5"/></svg>`, w, h, d)
		img, err := svg.New().Render([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		ink := 0
		for i := 3; i < len(img.Pix); i += 4 {
			ink += int(img.Pix[i])
		}
		// The line covers half of 2 rows of 20 pixels, a quarter of each.
		if want := 20 * 2 * 64; ink < want*9/10 || ink > want*11/10 {
			t.Errorf("vertical=%v: expected about %d of alpha, got %d", vertical, want, ink)
		}
	}
}
