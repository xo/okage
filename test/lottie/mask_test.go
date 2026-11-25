package lottie

import (
	"fmt"
	"strings"
	"testing"
)

// maskCircles are the shapes of the masks of TestThorVGMaskDifference, as
// Lottie shape values: four circles of radius 70 that overlap each other
// near the middle of the canvas.
var maskCircles = []string{
	maskCircle(160, 120),
	maskCircle(240, 120),
	maskCircle(200, 180),
	maskCircle(200, 110),
}

// maskCircle returns a Lottie shape value of a circle of radius 70 at (x,
// y), with four cubic curves.
func maskCircle(x, y float64) string {
	const r, k = 70.0, 70.0 * 0.5523
	return fmt.Sprintf(`{"c": true, "v": [[%[1]g, %[3]g], [%[5]g, %[2]g], [%[1]g, %[4]g], [%[6]g, %[2]g]],
		"i": [[%[8]g, 0], [0, %[8]g], [%[7]g, 0], [0, %[7]g]],
		"o": [[%[7]g, 0], [0, %[7]g], [%[8]g, 0], [0, %[8]g]]}`,
		x, y, y-r, y+r, x+r, x-r, k, -k)
}

// maskedLayer returns a shape layer that fills the canvas, with one mask for
// each item of modes, with the shape maskCircles[i]. An item is the letter
// of the mode, then the opacity, then "x" and the expansion, such as "f50"
// or "fx-10". The opacity is 100 and the expansion 0 when the item leaves
// them out.
func maskedLayer(modes ...string) string {
	var masks []string
	for i, m := range modes {
		opacity, expand, _ := strings.Cut(m[1:], "x")
		if opacity == "" {
			opacity = "100"
		}
		if expand == "" {
			expand = "0"
		}
		masks = append(masks, fmt.Sprintf(`{"mode": %q, "inv": false, "pt": {"a": 0, "k": %s}, "o": {"a": 0, "k": %s}, "x": {"a": 0, "k": %s}}`, m[:1], maskCircles[i], opacity, expand))
	}
	return fmt.Sprintf(`{
		"ty": 4, "ind": 1, "ip": 0, "op": 60, "st": 0, "hasMask": true, "masksProperties": [%s],
		"ks": {"a": {"a": 0, "k": [0, 0]}, "p": {"a": 0, "k": [0, 0]}, "s": {"a": 0, "k": [100, 100]}, "r": {"a": 0, "k": 0}, "o": {"a": 0, "k": 100}},
		"shapes": [{"ty": "rc", "p": {"a": 0, "k": [200, 150]}, "s": {"a": 0, "k": [400, 300]}, "r": {"a": 0, "k": 0}}, %s]
	}`, strings.Join(masks, ", "), mergeFill)
}

// TestThorVGMaskDifference compares the masks of a layer with ThorVG (D43):
// two, three and four masks of the mode difference, alone and after a mask
// of another mode, with another opacity or with an expansion, and the masks
// of other modes around them. ThorVG joins the masks of the mode subtract
// or difference that follow each other with the same opacity into one
// shape, and combines each shape with the result of the shapes after it.
func TestThorVGMaskDifference(t *testing.T) {
	var cases []compatCase
	for _, modes := range [][]string{
		{"f", "f"},
		{"f", "f", "f"},
		{"f", "f", "f", "f"},
		{"a", "f"},
		{"a", "f", "f"},
		{"a", "f", "f", "f"},
		{"f"},
		{"f", "a"},
		{"f", "n", "f"},
		{"a", "f", "a", "f"},
		{"a", "f", "s", "f"},
		{"a", "f", "f50"},
		{"a", "f50", "f"},
		{"a50", "f", "f"},
		{"s", "f", "f"},
		{"s", "a"},
		{"s", "i"},
		{"i", "f", "f"},
		{"a", "s50", "s"},
		{"a", "s", "s50"},
		{"a", "i50", "i"},
		{"a", "l", "f"},
		{"a", "d", "f"},
		{"a", "fx10", "f"},
		{"a", "f", "fx-10"},
	} {
		cases = append(cases, compatCase{
			name: "mask-" + strings.Join(modes, "-"),
			doc:  mergeDocument(maskedLayer(modes...)),
		})
	}
	compareCases(t, cases)
}
