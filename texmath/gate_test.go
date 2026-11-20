package texmath

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage"
)

// This file holds the gate of phase 23 of D78 and the fuzz target FuzzParse.
//
// testdata/gate.txt holds each string literal of the tests of go-tex/math at
// 6a6d2729, and \ooalign before each argument that a test of ooalign_test.go
// gives to parseOoalign. Most literals are formulas. Some are names of cases,
// messages or parts of a formula that a test joins. Each line says whether
// the literal parses, or the message of its error, which is the message of
// the source. The list was made with Parse and checked by hand against the
// tests of the source: each literal that a test of the source parses, parses
// here, and each literal that a test of the source expects to fail, fails
// here with the message of the source.

type gateInput struct {
	tex string
	err string // the message of the error, or empty
}

// gateInputs reads testdata/gate.txt.
func gateInputs(tb testing.TB) []gateInput {
	tb.Helper()
	f, err := os.Open("testdata/gate.txt")
	if err != nil {
		tb.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	var out []gateInput
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		tex, err := strconv.Unquote(fields[1])
		if err != nil {
			tb.Fatalf("%s: %v", line, err)
		}
		in := gateInput{tex: tex}
		if fields[0] == "err" {
			if in.err, err = strconv.Unquote(fields[2]); err != nil {
				tb.Fatalf("%s: %v", line, err)
			}
		}
		out = append(out, in)
	}
	if err := sc.Err(); err != nil {
		tb.Fatal(err)
	}
	return out
}

func TestGate(t *testing.T) {
	inputs := gateInputs(t)
	if len(inputs) < 1000 {
		t.Fatalf("the gate has %d inputs", len(inputs))
	}
	for _, in := range inputs {
		_, err := Parse(in.tex)
		switch {
		case in.err == "" && err != nil:
			t.Errorf("Parse(%q): %v, want no error", in.tex, err)
		case in.err != "" && err == nil:
			t.Errorf("Parse(%q): no error, want %q", in.tex, in.err)
		case in.err != "":
			var se *SyntaxError
			if !errors.As(err, &se) || se.Msg != in.err {
				t.Errorf("Parse(%q): %v, want %q", in.tex, err, in.err)
			}
		}
	}
}

// FuzzParse gives random text to Parse. Parse must not panic or hang, and an
// error must be a *SyntaxError that wraps a sentinel error of okage. The
// seeds are the inputs of the gate and the formulas of the snapshots, which
// hold the corpus of test/texmath (D78 section 5).
func FuzzParse(f *testing.F) {
	for _, tex := range fuzzSeeds(f) {
		f.Add(tex)
	}
	f.Fuzz(func(t *testing.T, tex string) {
		l, err := Parse(tex)
		if err != nil {
			var se *SyntaxError
			if !errors.As(err, &se) {
				t.Fatalf("Parse(%q): %T %v, want a *SyntaxError", tex, err, err)
			}
			if !errors.Is(err, okage.ErrTeXMathSyntax) && !errors.Is(err, okage.ErrMathLimit) {
				t.Fatalf("Parse(%q): %v wraps no sentinel error", tex, err)
			}
			if se.Pos < 0 || se.Pos > len(tex) {
				t.Fatalf("Parse(%q): position %d", tex, se.Pos)
			}
			return
		}
		if n := countNodes(l); n > defaultMaxNodes {
			t.Fatalf("Parse(%q): %d nodes", tex, n)
		}
	})
}
