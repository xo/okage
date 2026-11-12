package oracle

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/internal/lottiecompat"
	"github.com/xo/okage/lottie"
)

// lottieQuestion is a question of velato (D41).
type lottieQuestion struct {
	Lottie string    `json:"lottie"`
	Frames []float64 `json:"frames"`
}

// recorder is a lottie.Sink that writes each call as a line of text, as the
// Recorder of the oracle does.
type recorder struct {
	strings.Builder
}

func (r *recorder) num(v float64) {
	_, _ = fmt.Fprintf(r, " %.6f", v)
}

func (r *recorder) affine(t lottie.Affine) {
	for _, c := range t {
		r.num(c)
	}
}

func (r *recorder) shape(s []lottie.PathEl) {
	for _, el := range s {
		switch el.Kind {
		case lottie.PathElMoveTo:
			r.WriteString(" M")
		case lottie.PathElLineTo:
			r.WriteString(" L")
		case lottie.PathElQuadTo:
			r.WriteString(" Q")
		case lottie.PathElCurveTo:
			r.WriteString(" C")
		case lottie.PathElClosePath:
			r.WriteString(" Z")
			continue
		}
		n := map[lottie.PathElKind]int{lottie.PathElMoveTo: 1, lottie.PathElLineTo: 1, lottie.PathElQuadTo: 2, lottie.PathElCurveTo: 3}[el.Kind]
		for _, p := range el.P[:n] {
			r.num(p.X)
			r.num(p.Y)
		}
	}
}

func (r *recorder) color(c lottie.Color) {
	for _, v := range c {
		r.num(float64(v))
	}
}

func (r *recorder) PushLayer(b lottie.BlendMode, alpha float32, t lottie.Affine, s []lottie.PathEl) {
	_, _ = fmt.Fprintf(r, "push mix %d compose %d", b.Mix, b.Compose)
	r.num(float64(alpha))
	r.affine(t)
	r.shape(s)
	r.WriteString("\n")
}

func (r *recorder) PushClipLayer(t lottie.Affine, s []lottie.PathEl) {
	r.WriteString("clip")
	r.affine(t)
	r.shape(s)
	r.WriteString("\n")
}

func (r *recorder) PopLayer() { r.WriteString("pop\n") }

func (r *recorder) PushFilter(lottie.Affine, *lottie.FilterEffect) lottie.FilterLayerResult {
	r.WriteString("filter\n")
	return lottie.FilterUnsupported
}

func (r *recorder) Draw(st *lottie.Stroke, t lottie.Affine, b *lottie.Brush, s []lottie.PathEl) {
	r.WriteString("draw")
	if st != nil {
		r.WriteString(" stroke")
		r.num(st.Width)
		_, _ = fmt.Fprintf(r, " %d %d", st.Join, st.StartCap)
		r.num(st.MiterLimit)
	} else {
		r.WriteString(" fill")
	}
	r.affine(t)
	switch b.Kind {
	case lottie.BrushSolid:
		r.WriteString(" solid")
		r.color(b.Color)
	case lottie.BrushGradient:
		g := &b.Gradient
		if g.Kind == lottie.GradientLinear {
			r.WriteString(" linear")
			r.num(g.Start.X)
			r.num(g.Start.Y)
			r.num(g.End.X)
			r.num(g.End.Y)
		} else {
			r.WriteString(" radial")
			r.num(g.Start.X)
			r.num(g.Start.Y)
			r.num(float64(g.Radius))
		}
		r.WriteString(" stops")
		for _, st := range g.Stops {
			r.num(float64(st.Offset))
			r.color(st.Color)
		}
	}
	r.shape(s)
	r.WriteString("\n")
}

func (r *recorder) DrawImage(*lottie.ImageAsset, lottie.Affine, float64) { r.WriteString("image\n") }
func (r *recorder) BeginLayerGroup(string, int)                          {}
func (r *recorder) EndLayerGroup()                                       {}

// lottieCorpus returns the Lottie files of OKAGE_LOTTIE_CORPUS, or of the
// folder of the corpus in the test module.
func lottieCorpus(t *testing.T) []string {
	t.Helper()
	dir := os.Getenv("OKAGE_LOTTIE_CORPUS")
	if dir == "" {
		dir = filepath.Join("..", "lottie", "testdata", "emoji")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) == 0 {
		t.Skipf("no Lottie files in %s", dir)
	}
	return files
}

// sameLine reports if two lines of the recorders are the same, with each
// number within 1e-3.
func sameLine(a, b string) bool {
	fa, fb := strings.Fields(a), strings.Fields(b)
	if len(fa) != len(fb) {
		return false
	}
	for i := range fa {
		if fa[i] == fb[i] {
			continue
		}
		x, err1 := strconv.ParseFloat(fa[i], 64)
		y, err2 := strconv.ParseFloat(fb[i], 64)
		if err1 != nil || err2 != nil || math.Abs(x-y) > 1e-3 {
			return false
		}
	}
	return true
}

// TestLottieVelato renders five frames of each file of the corpus with
// okage and with velato, and compares the calls of the sinks (D41). It turns
// off the differences of D42 and D43 with internal/lottiecompat, so that it
// compares the port itself.
func TestLottieVelato(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	lottiecompat.Velato = true
	defer func() { lottiecompat.Velato = false }()
	var files []string
	var questions []lottieQuestion
	var comps []*lottie.Composition
	for _, f := range lottieCorpus(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		c, err := lottie.Parse(data)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		n := c.Frames[1] - c.Frames[0]
		var frames []float64
		for _, k := range []float64{0, n / 4, n / 2, 3 * n / 4, n - 1} {
			frames = append(frames, c.Frames[0]+math.Trunc(k))
		}
		files = append(files, f)
		comps = append(comps, c)
		questions = append(questions, lottieQuestion{Lottie: f, Frames: frames})
	}
	answers := ask(t, questions)
	failed := 0
	for i, q := range questions {
		var rec recorder
		for _, frame := range q.Frames {
			_, _ = fmt.Fprintf(&rec, "frame %v\n", frame)
			if err := lottie.NewRenderer().Append(comps[i], frame, lottie.IdentityAffine, 1.0, &rec); err != nil {
				rec.WriteString("evaluation error\n")
			}
		}
		got := strings.Split(strings.TrimSuffix(rec.String(), "\n"), "\n")
		want := strings.Split(strings.TrimSuffix(string(answers[i]), "\n"), "\n")
		bad := len(got) != len(want)
		line := 0
		for ; line < min(len(got), len(want)); line++ {
			if !sameLine(got[line], want[line]) {
				bad = true
				break
			}
		}
		if bad && os.Getenv("OKAGE_LOTTIE_DIFF") != "" {
			_ = os.WriteFile(filepath.Join(os.Getenv("OKAGE_LOTTIE_DIFF"), filepath.Base(files[i])+".okage"), []byte(rec.String()), 0o644)
			_ = os.WriteFile(filepath.Join(os.Getenv("OKAGE_LOTTIE_DIFF"), filepath.Base(files[i])+".velato"), answers[i], 0o644)
		}
		if bad {
			failed++
			if failed <= 5 {
				g, w := "", ""
				if line < len(got) {
					g = got[line]
				}
				if line < len(want) {
					w = want[line]
				}
				t.Errorf("%s: line %d of %d/%d differs\nokage:  %.300s\nvelato: %.300s", filepath.Base(files[i]), line, len(got), len(want), g, w)
			}
		}
	}
	t.Logf("%d files compared, %d differ", len(questions), failed)
}
