package texmath_test

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/texmath"
	"github.com/xo/okage/texmath/stixtwomath"
	"github.com/xo/okage/typesetting/font"
)

// The gate of the height and the depth of phase 24 of D78: for each formula
// of the corpus that KaTeX and texmath both lay out, the height and the depth
// of texmath are close to those of KaTeX.
//
// KaTeX gives its metrics in em, and its em is 10pt. texmath counts one point
// as one unit of the size, as the source does, so the test lays out at the
// size 10 and divides by 10. The fonts of KaTeX are not STIX Two Math, so the
// gate is loose (D78 section 7, item 4): each of the height and the depth is
// within 15 percent of the vertical extent of KaTeX, its height plus its
// depth. For a formula of an extent under 0.1 em, the extent is 0.1 em.
//
// katex_layout.txt lists each formula outside the gate, with the reason. A
// formula outside the gate that the file does not list fails the test, and so
// does a listed formula that is inside the gate, so that the list stays
// true. To write the file again, run the test with the environment variable
// TEXMATH_KATEX_UPDATE=1. The rules of reasonOf then give the reasons, and a
// person reads them.

// gateTolerance is the part of the extent of KaTeX that the gate allows.
const gateTolerance = 0.15

// knownFile is the list of the formulas outside the gate.
const knownFile = "katex_layout.txt"

// gateMiss reports whether texmath misses the gate on a formula with the
// metrics h and d of texmath and kh and kd of KaTeX, all in em.
func gateMiss(h, d, kh, kd float64) bool {
	tol := gateTolerance * max(kh+kd, 0.1)
	return math.Abs(h-kh) > tol || math.Abs(d-kd) > tol
}

// The reasons of katex_layout.txt, in the order of the rules of reasonOf.
var reasonRules = []struct {
	reason string
	re     *regexp.Regexp
}{
	{"brace", regexp.MustCompile(`\\(over|under)brace`)},
	{"katex", regexp.MustCompile(`\\raisebox\{[^}]*\}\[|\\mskip[^ ]* *plus|\{18mu\}|\\char|[└┘┌┐]`)},
	{"unicode", regexp.MustCompile(`[∫∬∭∮∏∐∑⋀⋁⋂⋃⨀⨁⨂⨄⨆]`)},
	{"arrow", regexp.MustCompile(`\\(x[a-zA-Z]*arrow|overrightarrow|overleftarrow|overleftrightarrow|vv)\b`)},
	{"boxed", regexp.MustCompile(`\\boxed\b`)},
	{"stack", regexp.MustCompile(`\\(stackrel|overset|underset|buildrel)\b`)},
	{"wide", regexp.MustCompile(`\\(widehat|widetilde|widecheck)\b`)},
	{"layout", regexp.MustCompile(`\\(frac|tfrac|dfrac|over|atop|choose|sqrt|binom|dbinom|tbinom|genfrac|stackrel|overset|underset|buildrel|underline|overline|begin|widetilde|widecheck|widehat|hat|vec|left|big|Big|bigg|Bigg|raisebox|rule|sum|prod|int|lim|log|operatorname)\b|(^|[^\\])[\^_']`)},
}

// reasonOf returns the reason of a formula outside the gate.
func reasonOf(tex string) string {
	for _, r := range reasonRules {
		if r.re.MatchString(tex) {
			return r.reason
		}
	}
	return "glyph"
}

// reasons are the reasons of katex_layout.txt, in order, with their meaning.
var reasons = []struct{ name, text string }{
	{"brace", `KaTeX draws \overbrace and \underbrace with SVG images of its own, and texmath with the brace of STIX Two Math and its assembly`},
	{"katex", "KaTeX reads the formula in another way, or its fonts have no glyph for a character"},
	{"unicode", "KaTeX sets a large operator typed as a character as a large operator, and the source as a character"},
	{"arrow", "the source draws the shaft and the heads of an arrow with proportions of the em of its own, with heads 0.48 em high"},
	{"boxed", `the source pads \boxed by 0.15 em, and KaTeX by 3pt and a strut`},
	{"stack", `the source sets \overset, \underset and \stackrel with a gap of 0.1 em, and KaTeX sets them as limits`},
	{"wide", `KaTeX draws \widehat, \widetilde and \widecheck with SVG images of its own, and texmath with the variants of STIX Two Math`},
	{"layout", "the MATH constants and the glyph variants of STIX Two Math are not the parameters and the glyphs of the KaTeX fonts"},
	{"glyph", "the ink of a glyph of STIX Two Math is not the box of the glyph in the metrics of KaTeX"},
}

// readKnown reads katex_layout.txt: a reason and a quoted formula on each
// line.
func readKnown(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(knownFile)
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]bool{}
	for _, r := range reasons {
		valid[r.name] = true
	}
	known := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		reason, quoted, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s: %q has no tab", knownFile, line)
		}
		tex, err := strconv.Unquote(quoted)
		if err != nil {
			t.Fatalf("%s: %q: %v", knownFile, line, err)
		}
		if !valid[reason] {
			t.Fatalf("%s: %q has an unknown reason", knownFile, line)
		}
		known[tex] = reason
	}
	return known
}

// TestKaTeXLayoutGate is the gate of the height and the depth.
func TestKaTeXLayoutGate(t *testing.T) {
	t.Parallel()
	corpus := readCorpus(t)
	results := runKaTeX(t)
	if len(results) != len(corpus) {
		t.Fatalf("KaTeX gives %d results for %d formulas", len(results), len(corpus))
	}
	f, err := stixtwomath.Font()
	if err != nil {
		t.Fatal(err)
	}
	face := font.NewFace(f)
	update := os.Getenv("TEXMATH_KATEX_UPDATE") != ""
	known := map[string]string{}
	if !update {
		known = readKnown(t)
	}
	both, inside := 0, 0
	seen := map[string]bool{}
	counts := map[string]int{}
	var out strings.Builder
	for i, r := range results {
		tex := corpus[i]
		if !r.OK || seen[tex] {
			continue
		}
		seen[tex] = true
		b, err := texmath.LayoutTeX(tex, face, 10, texmath.Display)
		if err != nil {
			continue
		}
		both++
		h, d := b.Height/10, b.Depth/10
		if !gateMiss(h, d, r.Height, r.Depth) {
			inside++
			if reason, ok := known[tex]; ok {
				t.Errorf("%q is inside the gate, but %s lists it (%s): remove it", tex, knownFile, reason)
			}
			continue
		}
		reason := reasonOf(tex)
		counts[reason]++
		fmt.Fprintf(&out, "%s\t%s\n", reason, strconv.Quote(tex))
		if _, ok := known[tex]; !ok && !update {
			t.Errorf("%q: the height %.4f and the depth %.4f em are not within %v of the extent of KaTeX, from %.4f and %.4f em",
				tex, h, d, gateTolerance, r.Height, r.Depth)
		}
	}
	t.Logf("KaTeX and texmath lay out %d formulas of the corpus. %d are inside the gate (%.1f%%), and %d outside: %v",
		both, inside, 100*float64(inside)/float64(max(both, 1)), both-inside, counts)
	if !update {
		return
	}
	var head strings.Builder
	head.WriteString("# The formulas of corpus.txt outside the gate of the height and the depth in\n")
	head.WriteString("# layout_test.go, each with its reason, in the order of the corpus. The reasons:\n")
	for _, r := range reasons {
		fmt.Fprintf(&head, "#   %s: %s.\n", r.name, r.text)
	}
	if err := os.WriteFile(knownFile, []byte(head.String()+out.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}
