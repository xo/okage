package oracle

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/xo/okage/internal/roxmltree"
)

// svgDir is the folder of the SVG files of the tests of resvg and usvg, as the
// package svg holds them, so that the tests run without a checkout of resvg.
var svgDir = filepath.Join(root, "svg", "testdata")

// svgFiles returns the absolute paths of the SVG files of the tests of
// resvg and usvg. The test skips when the folder is missing.
func svgFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(svgDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".svg") {
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			files = append(files, abs)
		}
		return nil
	})
	switch {
	case errors.Is(err, fs.ErrNotExist):
		t.Skip("the test files of svg are missing")
	case err != nil:
		t.Fatal(err)
	}
	return files
}

type xmlQuestion struct {
	XML string `json:"xml"`
}

// xmlAnswer is the answer of okage to an XML question: the tree of the file,
// as roxmltree tests/ast.rs writes it, or the error.
func xmlAnswer(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(b) {
		return nil
	}
	doc, err := roxmltree.ParseWithOptions(string(b), xmlOptions())
	if err != nil {
		if e, ok := errors.AsType[*roxmltree.Error](err); ok {
			return []byte(`error: "` + e.Message() + `"`)
		}
		return []byte("error: " + err.Error())
	}
	return []byte(roxmltree.ToYAML(doc))
}

// xmlOptions are the options of usvg: the default ones, with a DTD allowed.
func xmlOptions() roxmltree.ParsingOptions {
	opt := roxmltree.DefaultParsingOptions()
	opt.AllowDTD = true
	return opt
}

// TestXML parses each SVG file of the tests of resvg with okage and with
// roxmltree, and compares the trees (phase 6).
func TestXML(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	files := svgFiles(t)
	questions := make([]xmlQuestion, len(files))
	for i, f := range files {
		questions[i] = xmlQuestion{XML: f}
	}
	answers := ask(t, questions)
	diffs := 0
	for i, f := range files {
		got := xmlAnswer(f)
		if string(got) != string(answers[i]) {
			diffs++
			if diffs <= 10 {
				t.Errorf("%s: the tree differs\nokage:\n%s\nroxmltree:\n%s", f, got, answers[i])
			}
		}
	}
	t.Logf("%d files, %d differ", len(files), diffs)
}

// TestXMLMutated parses random mutations of the SVG files of the tests of
// resvg: a file cut short, a range of bytes deleted, or a byte of markup
// inserted. It reaches the errors of the parser. The flag -oracle.n sets the
// number of mutations.
func TestXMLMutated(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	files := svgFiles(t)
	r := rand.New(rand.NewPCG(*flagSeed, 6))
	dir := t.TempDir()
	const markup = "<>&;\"'=/!?[]-#x:"
	var paths []string
	for i := 0; len(paths) < *flagN && i < 10**flagN; i++ {
		b, err := os.ReadFile(files[r.IntN(len(files))])
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
		if !utf8.Valid(b) {
			continue
		}
		p := filepath.Join(dir, fmt.Sprintf("%d.svg", len(paths)))
		if err := os.WriteFile(p, b, 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	questions := make([]xmlQuestion, len(paths))
	for i, p := range paths {
		questions[i] = xmlQuestion{XML: p}
	}
	answers := ask(t, questions)
	diffs, errs := 0, 0
	for i, p := range paths {
		got := xmlAnswer(p)
		if strings.HasPrefix(string(got), "error: ") {
			errs++
		}
		if string(got) != string(answers[i]) {
			diffs++
			if diffs <= 10 {
				t.Errorf("mutation %d differs\nokage:\n%s\nroxmltree:\n%s", i, got, answers[i])
			}
		}
	}
	t.Logf("%d mutations, %d errors, %d differ", len(paths), errs, diffs)
}
