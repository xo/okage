// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
)

// render renders the frame of the Lottie file data onto a new pixmap of the
// size of the composition.
func render(t *testing.T, data []byte, frame float64) *okage.Pixmap {
	t.Helper()
	return renderWith(t, NewRenderer(), data, frame)
}

// renderWith renders as render does, with r.
func renderWith(t *testing.T, r *Renderer, data []byte, frame float64) *okage.Pixmap {
	t.Helper()
	c, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	pm, ok := okage.NewPixmap(uint32(c.Width), uint32(c.Height))
	if !ok {
		t.Fatal("no pixmap")
	}
	m := pm.AsMut()
	if err := r.Append(c, frame, IdentityAffine, 1.0, NewPixmapSink(&m)); err != nil {
		t.Fatal(err)
	}
	return pm
}

// pixel returns the demultiplied color of the pixel (x, y).
func pixel(t *testing.T, pm *okage.Pixmap, x, y uint32) [4]uint8 {
	t.Helper()
	p, ok := pm.Pixel(x, y)
	if !ok {
		t.Fatalf("no pixel (%d, %d)", x, y)
	}
	c := p.Demultiply()
	return [4]uint8{c.Red(), c.Green(), c.Blue(), c.Alpha()}
}

// TestRenderTestdata renders three frames of each Noto animated emoji in
// testdata, and checks that each draws something.
func TestRenderTestdata(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "*.json"))
	if len(files) == 0 {
		t.Fatal("no test files")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			c, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			for _, frame := range []float64{c.Frames[0], (c.Frames[0] + c.Frames[1]) / 2, c.Frames[1] - 1} {
				pm := render(t, data, frame)
				if p := pixel(t, pm, pm.Width()/2, pm.Height()/2); p[3] == 0 {
					t.Errorf("frame %v: the center is transparent", frame)
				}
			}
		})
	}
}

// TestGradientOpacity checks that a gradient fill takes its opacity (D43).
// At frame 0 the imp is yellow: its pink gradient has the opacity 0.
func TestGradientOpacity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "1f608-imp-smile.json"))
	if err != nil {
		t.Fatal(err)
	}
	pm := render(t, data, 0)
	if p := pixel(t, pm, 512, 300); p[0] < 200 || p[1] < 160 || p[2] > 120 {
		t.Errorf("the face is %v, want yellow", p)
	}
}

// lottieFile returns a Lottie file of 100 by 100 pixels with one shape layer
// that holds the shape items items.
func lottieFile(items string) []byte {
	return []byte(fmt.Sprintf(`{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "layers": [
		{"ty": 4, "ind": 1, "ip": 0, "op": 10, "st": 0, "ks": {"p": {"a": 0, "k": [0, 0]}}, "shapes": [%s]}
	]}`, items))
}

// rect returns a rectangle shape item centered at (x, y), of size w by h.
func rect(x, y, w, h float64) string {
	return fmt.Sprintf(`{"ty": "rc", "p": {"a": 0, "k": [%v, %v]}, "s": {"a": 0, "k": [%v, %v]}, "r": {"a": 0, "k": 0}}`, x, y, w, h)
}

// red is a red fill item.
const red = `{"ty": "fl", "o": {"a": 0, "k": 100}, "c": {"a": 0, "k": [1, 0, 0, 1]}}`

// TestMergePaths checks the merge paths of modes 2 to 5 with
// Renderer.MergePaths (D42). The first rectangle covers x from 10 to 60, and
// the second covers x from 40 to 90. Without MergePaths, each mode draws both
// rectangles, as ThorVG does (D43).
func TestMergePaths(t *testing.T) {
	for _, c := range []struct {
		mode                int
		left, middle, right bool
	}{
		{1, true, true, true},
		{2, true, true, true},
		{3, true, false, false},
		{4, false, true, false},
		{5, true, false, true},
	} {
		t.Run(fmt.Sprintf("mode %d", c.mode), func(t *testing.T) {
			items := fmt.Sprintf(`{"ty": "gr", "it": [%s, %s, {"ty": "mm", "mm": %d}, %s,
				{"ty": "tr", "p": {"a": 0, "k": [0, 0]}}]}`, rect(35, 50, 50, 50), rect(65, 50, 50, 50), c.mode, red)
			if pm := render(t, lottieFile(items), 0); pixel(t, pm, 50, 50)[3] != 255 || pixel(t, pm, 80, 50)[3] != 255 {
				t.Error("the default renderer applied the merge path")
			}
			pm := renderWith(t, &Renderer{MergePaths: true}, lottieFile(items), 0)
			for _, s := range []struct {
				x    uint32
				want bool
			}{{20, c.left}, {50, c.middle}, {80, c.right}} {
				if got := pixel(t, pm, s.x, 50)[3] == 255; got != s.want {
					t.Errorf("x %d: painted is %v, want %v", s.x, got, s.want)
				}
			}
		})
	}
}

// TestSolidLayer checks that a solid layer fills its rectangle with its
// color (D43).
func TestSolidLayer(t *testing.T) {
	data := []byte(`{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "layers": [
		{"ty": 1, "ind": 1, "ip": 0, "op": 10, "st": 0, "sw": 50, "sh": 40, "sc": "#00ff00", "ks": {"p": {"a": 0, "k": [10, 20]}}}
	]}`)
	pm := render(t, data, 0)
	if p := pixel(t, pm, 30, 40); p != [4]uint8{0, 255, 0, 255} {
		t.Errorf("inside is %v", p)
	}
	if p := pixel(t, pm, 5, 5); p[3] != 0 {
		t.Errorf("outside is %v", p)
	}
}

// TestGradientStops checks that the opacity stops of a gradient give the
// alpha at each offset (D43): opaque at the start, transparent at the end.
func TestGradientStops(t *testing.T) {
	gradient := `{"ty": "gf", "o": {"a": 0, "k": 100}, "t": 1,
		"s": {"a": 0, "k": [0, 50]}, "e": {"a": 0, "k": [100, 50]},
		"g": {"p": 2, "k": {"a": 0, "k": [0, 1, 0, 0, 1, 1, 0, 0, 0, 1, 1, 0]}}}`
	pm := render(t, lottieFile(rect(50, 50, 100, 100)+", "+gradient), 0)
	left, right := pixel(t, pm, 2, 50), pixel(t, pm, 97, 50)
	if left[0] < 240 || left[3] < 240 || right[3] > 16 {
		t.Errorf("left is %v and right is %v", left, right)
	}
}

// TestFillEffect checks the Fill effect of the PixmapSink.
func TestFillEffect(t *testing.T) {
	data := []byte(`{"v": "5.7.0", "fr": 30, "ip": 0, "op": 10, "w": 100, "h": 100, "layers": [
		{"ty": 4, "ind": 1, "ip": 0, "op": 10, "st": 0, "ks": {"p": {"a": 0, "k": [0, 0]}},
		 "ef": [{"ty": 21, "ef": [{}, {}, {"v": {"a": 0, "k": [0, 0, 1, 1]}}, {}, {}, {}, {"v": {"a": 0, "k": 1}}]}],
		 "shapes": [` + rect(50, 50, 40, 40) + `, ` + red + `]}
	]}`)
	pm := render(t, data, 0)
	if p := pixel(t, pm, 50, 50); p != [4]uint8{0, 0, 255, 255} {
		t.Errorf("the fill is %v, want blue", p)
	}
}
