package oracle

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/svg"
)

type usvgQuestion struct {
	Usvg string `json:"usvg"`
}

// usvgErrors maps each sentinel error of okage for usvg::Error to the name of
// its case, as the oracle writes it.
var usvgErrors = []struct {
	err  error
	name string
}{
	{okage.ErrSVGNotAnUTF8Str, "NotAnUtf8Str"},
	{okage.ErrSVGMalformedGZip, "MalformedGZip"},
	{okage.ErrSVGElementsLimitReached, "ElementsLimitReached"},
	{okage.ErrSVGInvalidSize, "InvalidSize"},
	{okage.ErrSVGParsingFailed, "ParsingFailed"},
}

// usvgAnswer is the answer of okage to a usvg question: the tree of the file,
// written with the default options, or the name of the case of the error.
func usvgAnswer(file string) []byte {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	tree, err := svg.FromData(data, svg.WithResourcesDir(filepath.Dir(file)))
	if err != nil {
		for _, e := range usvgErrors {
			if errors.Is(err, e.err) {
				return []byte("error: " + e.name)
			}
		}
		return []byte("error: " + err.Error())
	}
	return []byte(tree.ToString())
}

// usvgKind sorts a test file by what can make its tree differ from usvg: text,
// which phase 9 ports, and raster images, which okage decodes with other code
// (D7). The gate of phase 7 holds for the files of kind "plain".
func usvgKind(file string) string {
	b, err := os.ReadFile(file)
	if err != nil {
		return "plain"
	}
	s := string(b)
	switch {
	case strings.Contains(s, "<text") || strings.Contains(s, "<tspan") || strings.Contains(s, "<textPath"):
		return "text"
	case strings.Contains(s, "<image") || strings.Contains(s, "<feImage"):
		return "image"
	}
	return "plain"
}

// TestUsvg parses each SVG file of the tests of resvg with okage and with usvg,
// built without its feature text, and compares the trees that the writer of
// usvg writes (phase 7, D23). Each file without text and without an image
// must give the same text. The files with text or an image are counted.
func TestUsvg(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	files := svgFiles(t)
	questions := make([]usvgQuestion, len(files))
	for i, f := range files {
		questions[i] = usvgQuestion{Usvg: f}
	}
	answers := ask(t, questions)
	total := map[string]int{}
	diffs := map[string]int{}
	shown := 0
	for i, f := range files {
		kind := usvgKind(f)
		total[kind]++
		got, err := safely(func() []byte { return usvgAnswer(f) })
		if err != nil {
			got = []byte("panic\n")
		}
		if string(got) == string(answers[i]) {
			continue
		}
		diffs[kind]++
		if kind == "plain" {
			shown++
			if shown <= 10 {
				t.Errorf("%s: the tree differs\nokage:\n%s\nusvg:\n%s", f, got, answers[i])
			}
		} else {
			t.Logf("%s: the tree differs (%s)", f, kind)
		}
	}
	for _, kind := range []string{"plain", "image", "text"} {
		t.Logf("%s: %d files, %d differ", kind, total[kind], diffs[kind])
	}
}

// TestUsvgMutated parses random mutations of the SVG files of the tests of
// resvg with okage and with usvg: a file cut short, a range of bytes deleted,
// or a byte of markup inserted, as TestXMLMutated makes them. The flag
// -oracle.n sets the number of mutations. Each mutation must give the same
// text, but a mutation of a file with text or an image is only counted. An
// image whose data a mutation breaks can differ, because imagesize reads the
// size from a header that image.DecodeConfig rejects (D7).
func TestUsvgMutated(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	files := svgFiles(t)
	r := rand.New(rand.NewPCG(*flagSeed, 8))
	dir := t.TempDir()
	const markup = "<>&;\"'=/!?[]-#x:"
	var paths, kinds []string
	for i := 0; len(paths) < *flagN && i < 10**flagN; i++ {
		src := files[r.IntN(len(files))]
		b, err := os.ReadFile(src)
		if err != nil || len(b) < 2 {
			continue
		}
		at := r.IntN(len(b))
		switch r.IntN(3) {
		case 0:
			b = b[:at]
		case 1:
			end := min(len(b), at+1+r.IntN(16))
			b = append(b[:at:at], b[end:]...)
		default:
			c := markup[r.IntN(len(markup))]
			b = append(b[:at:at], append([]byte{c}, b[at:]...)...)
		}
		// The copy is in a temporary folder, so a relative link to an image
		// does not resolve, for okage and for usvg alike.
		p := filepath.Join(dir, fmt.Sprintf("%d.svg", len(paths)))
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
		kinds = append(kinds, usvgKind(src))
	}
	questions := make([]usvgQuestion, len(paths))
	for i, p := range paths {
		questions[i] = usvgQuestion{Usvg: p}
	}
	answers := ask(t, questions)
	diffs, errs := 0, 0
	other := map[string]int{}
	for i, p := range paths {
		got, err := safely(func() []byte { return usvgAnswer(p) })
		if err != nil {
			got = []byte("panic\n")
		}
		if strings.HasPrefix(string(got), "error: ") {
			errs++
		}
		if string(got) == string(answers[i]) {
			continue
		}
		if kinds[i] != "plain" {
			other[kinds[i]]++
			continue
		}
		diffs++
		if diffs <= 10 {
			b, _ := os.ReadFile(p)
			t.Errorf("mutation %d differs\nfile:\n%s\nokage:\n%s\nusvg:\n%s", i, b, got, answers[i])
		}
	}
	t.Logf("%d mutations, %d errors, %d differ; of the mutations of files with an image %d differ, and with text %d",
		len(paths), errs, diffs, other["image"], other["text"])
}
