package texmath_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// katexVersion is the version of KaTeX that Mermaid 12.3.0 bundles. Its
// source map names node_modules/.pnpm/katex@0.18.11.
const katexVersion = "0.18.11"

// katexResult is the answer of KaTeX for one formula.
type katexResult struct {
	TeX    string  `json:"tex"`
	OK     bool    `json:"ok"`
	Crash  bool    `json:"crash"`
	Error  string  `json:"error"`
	Height float64 `json:"height"`
	Depth  float64 `json:"depth"`
	HTML   string  `json:"html"` // with the flag --html only
}

// readCorpus returns the formulas of corpus.txt, in order.
func readCorpus(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("corpus.txt")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// runKaTeX runs katex/katex.mjs on corpus.txt with the flags flags, and
// returns the result of each formula, in the order of the corpus. It skips
// the test when node or the packages of npm are missing.
func runKaTeX(t *testing.T, flags ...string) []katexResult {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is missing")
	}
	if _, err := os.Stat(filepath.Join("katex", "node_modules", "katex", "package.json")); err != nil {
		t.Skip("KaTeX is not installed. Run npm ci in test/texmath/katex")
	}
	var stderr bytes.Buffer
	args := append([]string{filepath.Join("katex", "katex.mjs"), "corpus.txt"}, flags...)
	cmd := exec.CommandContext(t.Context(), node, args...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running katex.mjs: %v\n%s", err, stderr.Bytes())
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(nil, 16<<20)
	if !sc.Scan() {
		t.Fatal("katex.mjs wrote nothing")
	}
	var head struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(sc.Bytes(), &head); err != nil {
		t.Fatalf("reading the version of KaTeX: %v", err)
	}
	if head.Version != katexVersion {
		t.Fatalf("KaTeX is at %s, want %s. Run npm ci in test/texmath/katex", head.Version, katexVersion)
	}
	var results []katexResult
	for sc.Scan() {
		var r katexResult
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			t.Fatalf("reading %q: %v", sc.Bytes(), err)
		}
		results = append(results, r)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return results
}

// TestKaTeXOracle runs KaTeX on the corpus. It gives one result for each
// formula, and the height and the depth of the formula of the gate of
// phase 20 (D78).
func TestKaTeXOracle(t *testing.T) {
	t.Parallel()
	corpus := readCorpus(t)
	results := runKaTeX(t)
	if len(results) != len(corpus) {
		t.Fatalf("KaTeX gives %d results for %d formulas", len(results), len(corpus))
	}
	parsed := 0
	byTeX := map[string]katexResult{}
	for i, r := range results {
		if r.TeX != corpus[i] {
			t.Fatalf("the result %d is for %q, want %q", i, r.TeX, corpus[i])
		}
		byTeX[r.TeX] = r
		switch {
		case r.OK:
			parsed++
			if math.IsNaN(r.Height) || math.IsNaN(r.Depth) {
				t.Errorf("%q: the height is %v and the depth %v", r.TeX, r.Height, r.Depth)
			}
		case r.Error == "":
			t.Errorf("%q: KaTeX fails without a message", r.TeX)
		}
	}
	t.Logf("KaTeX %s parses %d of the %d formulas of the corpus", katexVersion, parsed, len(corpus))

	// The values in em of the formula of the gate, as KaTeX 0.18.11 gives
	// them in display style.
	const gate = `\frac{x^2+1}{\alpha-\beta}`
	r, ok := byTeX[gate]
	switch {
	case !ok:
		t.Errorf("the corpus has no %s", gate)
	case !r.OK:
		t.Errorf("KaTeX does not parse %s: %s", gate, r.Error)
	case math.Abs(r.Height-1.491108) > 1e-9 || math.Abs(r.Depth-0.88044) > 1e-9:
		t.Errorf("KaTeX gives %s the height %v and the depth %v, want 1.491108 and 0.88044", gate, r.Height, r.Depth)
	}
}
