package lottie

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/lottie"

	"github.com/xo/okage"
	okagelottie "github.com/xo/okage/lottie"
)

// corpusDir returns the folder of the Noto animated emoji, from the variable
// OKAGE_LOTTIE_CORPUS or testdata/emoji, and skips the test when it is
// missing.
func corpusDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("OKAGE_LOTTIE_CORPUS")
	if dir == "" {
		dir = filepath.Join("testdata", "emoji")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) == 0 {
		t.Skipf("no Lottie files in %s; run fetch-corpus.sh", dir)
	}
	return dir
}

// result is the comparison of one frame.
type result struct {
	name    string
	frame   float32
	differ  int
	total   int
	err     string
	maxDiff uint8
}

// diffThreshold is the largest difference of a channel of two pixels that
// match.
const diffThreshold = 16

// pixelDiff returns the largest difference of a channel of the pixels at i
// and j, or 0 when both are transparent.
func pixelDiff(a []byte, i int, b []byte, j int) uint8 {
	if a[i+3] == 0 && b[j+3] == 0 {
		return 0
	}
	var d uint8
	for c := range 4 {
		x, y := a[i+c], b[j+c]
		if x > y {
			d = max(d, x-y)
		} else {
			d = max(d, y-x)
		}
	}
	return d
}

// matchesNear reports if the pixel (x, y) of a lies within the colors of
// the pixels of b at (x, y) and next to it: each channel is at least the
// least of those pixels and at most the most of them, give or take
// diffThreshold. So a value between two pixels of an edge matches, however
// the two rasterizers spread the antialiasing of the edge.
func matchesNear(a, b []byte, w, h, x, y int) bool {
	i := 4 * (y*w + x)
	for c := range 4 {
		lo, hi := 255, 0
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				xx, yy := x+dx, y+dy
				if xx < 0 || yy < 0 || xx >= w || yy >= h {
					continue
				}
				v := int(b[4*(yy*w+xx)+c])
				lo, hi = min(lo, v), max(hi, v)
			}
		}
		if v := int(a[i+c]); v < lo-diffThreshold || v > hi+diffThreshold {
			return false
		}
	}
	return true
}

// premultiply returns the premultiplied RGBA bytes of img.
func premultiply(img *image.NRGBA) []byte {
	out := make([]byte, len(img.Pix))
	for i := 0; i+3 < len(img.Pix); i += 4 {
		a := uint32(img.Pix[i+3])
		for c := range 3 {
			out[i+c] = uint8((uint32(img.Pix[i+c])*a + 127) / 255)
		}
		out[i+3] = uint8(a)
	}
	return out
}

// compareNRGBA counts the pixels that differ, with both images
// premultiplied, so that a pixel that is almost transparent cannot differ
// much. A pixel differs when it does not lie within the colors of the
// pixels of the other image at it and next to it, in either direction, as
// matchesNear says. So a difference of antialiasing or a shift of one pixel
// at an edge does not count, and a wrong color or a missing shape does.
func compareNRGBA(got []byte, wantImg *image.NRGBA, w, h int) (int, uint8) {
	want := &image.NRGBA{Pix: premultiply(wantImg)}
	differ := 0
	var maxDiff uint8
	for y := range h {
		for x := range w {
			i := 4 * (y*w + x)
			maxDiff = max(maxDiff, pixelDiff(got, i, want.Pix, i))
			if !matchesNear(got, want.Pix, w, h, x, y) || !matchesNear(want.Pix, got, w, h, x, y) {
				differ++
			}
		}
	}
	return differ, maxDiff
}

// TestThorVG compares the frames of each file of the corpus with ThorVG.
func TestThorVG(t *testing.T) {
	dir := corpusDir(t)
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if f := os.Getenv("OKAGE_LOTTIE_FILTER"); f != "" {
		files = slices.DeleteFunc(files, func(s string) bool { return !strings.Contains(s, f) })
	}
	ctx := context.Background()
	r, err := lottie.New(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(ctx) }()
	var results []result
	for _, file := range files {
		results = append(results, compareFile(ctx, r, file)...)
	}
	report(t, results)
}

// compareFile compares five frames of file.
func compareFile(ctx context.Context, r *lottie.Renderer, file string) []result {
	name := strings.TrimSuffix(filepath.Base(file), ".json")
	data, err := os.ReadFile(file)
	if err != nil {
		return []result{{name: name, err: err.Error()}}
	}
	c, err := okagelottie.Parse(data)
	if err != nil {
		return []result{{name: name, err: "okage: " + err.Error()}}
	}
	a, err := r.Load(ctx, data, lottie.Options{})
	if err != nil {
		return []result{{name: name, err: "thorvg: " + err.Error()}}
	}
	defer func() { _ = a.Close(ctx) }()
	total := a.TotalFrames()
	var results []result
	for _, f := range []float32{0, total / 4, total / 2, 3 * total / 4, total - 1} {
		f = float32(int(f))
		res := result{name: name, frame: f}
		want, err := a.Frame(ctx, f)
		if err != nil {
			res.err = "thorvg: " + err.Error()
			results = append(results, res)
			continue
		}
		pm, ok := okage.NewPixmap(uint32(c.Width), uint32(c.Height))
		if !ok {
			res.err = "okage: no pixmap"
			results = append(results, res)
			continue
		}
		m := pm.AsMut()
		if err := okagelottie.RenderFrame(c, c.Frames[0]+float64(f), okagelottie.IdentityAffine, &m); err != nil {
			res.err = "okage: " + err.Error()
			results = append(results, res)
			continue
		}
		res.total = int(c.Width * c.Height)
		res.differ, res.maxDiff = compareNRGBA(pm.Data(), want, int(c.Width), int(c.Height))
		if dump := os.Getenv("OKAGE_LOTTIE_DUMP"); dump != "" && res.differ > 0 {
			_ = os.MkdirAll(dump, 0o755)
			b, _ := lottie.EncodePNG(want)
			_ = os.WriteFile(filepath.Join(dump, fmt.Sprintf("%s-%03d-thorvg.png", name, int(f))), b, 0o644)
			pm2, _ := okage.NewPixmap(uint32(c.Width), uint32(c.Height))
			m2 := pm2.AsMut()
			_ = okagelottie.RenderFrame(c, c.Frames[0]+float64(f), okagelottie.IdentityAffine, &m2)
			_ = pm2.SavePNG(filepath.Join(dump, fmt.Sprintf("%s-%03d-okage.png", name, int(f))))
		}
		results = append(results, res)
	}
	return results
}

// report logs the results, worst first.
func report(t *testing.T, results []result) {
	t.Helper()
	slices.SortFunc(results, func(a, b result) int { return b.differ*1000/max(b.total, 1) - a.differ*1000/max(a.total, 1) })
	buckets := map[string]int{}
	var errs []string
	for _, r := range results {
		if r.err != "" {
			errs = append(errs, r.name+": "+r.err)
			continue
		}
		pct := float64(r.differ) * 100 / float64(r.total)
		switch {
		case pct == 0:
			buckets["0%"]++
		case pct <= 0.5:
			buckets["<=0.5%"]++
		case pct <= 1:
			buckets["<=1%"]++
		case pct <= 5:
			buckets["<=5%"]++
		default:
			buckets[">5%"]++
		}
	}
	t.Logf("%d frames: %v, %d errors", len(results), buckets, len(errs))
	for _, e := range errs[:min(len(errs), 20)] {
		t.Error(e)
	}
	// The gate of D43: every frame passes.
	failed := 0
	for _, r := range results {
		if r.err == "" && r.differ*100 > r.total {
			failed++
			if failed <= 20 {
				t.Errorf("%s frame %v: %.3f%% of the pixels differ from ThorVG", r.name, r.frame, float64(r.differ)*100/float64(r.total))
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d frames differ from ThorVG in more than 1%% of their pixels", failed)
	}
	if out := os.Getenv("OKAGE_LOTTIE_REPORT"); out != "" {
		var sb strings.Builder
		for _, r := range results {
			fmt.Fprintf(&sb, "%.3f\t%d\t%s\t%v\t%s\n", float64(r.differ)*100/float64(max(r.total, 1)), r.maxDiff, r.name, r.frame, r.err)
		}
		_ = os.WriteFile(out, []byte(sb.String()), 0o644)
	}
}
