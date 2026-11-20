package texmath_test

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/texmath"
)

// The parse gate of D78 section 7, item 4: each formula of the corpus that
// KaTeX parses, texmath.Parse parses too, except a formula of
// katex_parse.txt. Each line of that file holds a reason, the missing command
// or the message of the error of Parse, and the quoted formula.
//
// A formula that KaTeX parses and Parse does not, and that the file does not
// list, fails the test. So does a listed formula that Parse parses, that
// KaTeX does not parse, or whose error changed, so that the list stays true.
// To write the file again, run the test with the environment variable
// TEXMATH_KATEX_UPDATE=1. The rules of parseReasonOf then give the reasons,
// and a person reads them.

// parseKnownFile is the list of the formulas outside the parse gate.
const parseKnownFile = "katex_parse.txt"

// parseReasons are the reasons of katex_parse.txt, in order, with their
// meaning.
var parseReasons = []struct{ name, text string }{
	{"missing", "the command is in the list of the missing commands of D78 section 10, phase 26 did not add it, and it waits for a request"},
	{"text", "the command sets text, such as an accent of text or a letter of Latin-1, and the source has no text mode for it"},
	{"html", "the command is an extension of KaTeX for HTML, such as a link or an image, which a box cannot hold"},
	{"syntax", "the source reads the formula in another way than KaTeX, and gives an error with no command"},
	{"command", "the source has no such symbol, operator, macro or environment, and KaTeX has one"},
}

// The rules of parseReasonOf, for the reasons with a command.
var (
	missingRe = regexp.MustCompile(`^\\(underrightarrow|sideset|begin\{(align|equation|gather)\})$`)
	textRe    = regexp.MustCompile("^\\\\([\"'.=^`~]|H|r|u|v|AA|AE|aa|ae|i|j|o|O|oe|OE|ss|text[a-z]*|copyright|pounds|yen|dag|ddag|sect|degree|mathsterling|maltese|circledR|lq|rq|KaTeX|LaTeX|TeX)$")
	htmlRe    = regexp.MustCompile(`^\\(href|url|html[A-Za-z]*|includegraphics)$`)
)

// parseReasonOf returns the reason of a formula outside the parse gate, from
// the error of Parse.
func parseReasonOf(se *texmath.SyntaxError) string {
	switch c := se.Command; {
	case c == "":
		return "syntax"
	case missingRe.MatchString(c):
		return "missing"
	case textRe.MatchString(c):
		return "text"
	case htmlRe.MatchString(c):
		return "html"
	}
	return "command"
}

// parseMiss is one line of katex_parse.txt.
type parseMiss struct {
	reason string
	what   string // the missing command, or the message of the error
}

// whatOf returns the missing command of an error of Parse, or its message
// when it names no command.
func whatOf(se *texmath.SyntaxError) string {
	if se.Command != "" {
		return se.Command
	}
	return se.Msg
}

// readParseKnown reads katex_parse.txt.
func readParseKnown(t *testing.T) map[string]parseMiss {
	t.Helper()
	data, err := os.ReadFile(parseKnownFile)
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]bool{}
	for _, r := range parseReasons {
		valid[r.name] = true
	}
	known := map[string]parseMiss{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("%s: %q does not have three fields", parseKnownFile, line)
		}
		if !valid[fields[0]] {
			t.Fatalf("%s: %q has an unknown reason", parseKnownFile, line)
		}
		what, err := strconv.Unquote(fields[1])
		if err != nil {
			t.Fatalf("%s: %q: %v", parseKnownFile, line, err)
		}
		tex, err := strconv.Unquote(fields[2])
		if err != nil {
			t.Fatalf("%s: %q: %v", parseKnownFile, line, err)
		}
		known[tex] = parseMiss{reason: fields[0], what: what}
	}
	return known
}

// TestKaTeXParseGate is the parse gate.
func TestKaTeXParseGate(t *testing.T) {
	t.Parallel()
	corpus := readCorpus(t)
	results := runKaTeX(t)
	if len(results) != len(corpus) {
		t.Fatalf("KaTeX gives %d results for %d formulas", len(results), len(corpus))
	}
	update := os.Getenv("TEXMATH_KATEX_UPDATE") != ""
	known := map[string]parseMiss{}
	if !update {
		known = readParseKnown(t)
	}
	parsed, both := 0, 0
	counts := map[string]int{}
	commands := map[string]bool{}
	var out strings.Builder
	for i, r := range results {
		tex := corpus[i]
		m, listed := known[tex]
		if !r.OK {
			if listed {
				t.Errorf("KaTeX does not parse %q, but %s lists it: remove it", tex, parseKnownFile)
			}
			continue
		}
		parsed++
		_, err := texmath.Parse(tex)
		if err == nil {
			both++
			if listed {
				t.Errorf("Parse parses %q, but %s lists it (%s): remove it", tex, parseKnownFile, m.reason)
			}
			continue
		}
		var se *texmath.SyntaxError
		if !errors.As(err, &se) {
			t.Fatalf("Parse(%q): %v is not a *SyntaxError", tex, err)
		}
		reason, what := parseReasonOf(se), whatOf(se)
		counts[reason]++
		if se.Command != "" {
			commands[se.Command] = true
		}
		fmt.Fprintf(&out, "%s\t%s\t%s\n", reason, strconv.Quote(what), strconv.Quote(tex))
		switch {
		case update:
		case !listed:
			t.Errorf("KaTeX parses %q, and Parse gives %v", tex, err)
		case m.what != what:
			t.Errorf("%q: Parse misses %q, and %s says %q", tex, what, parseKnownFile, m.what)
		}
	}
	t.Logf("KaTeX parses %d formulas of the corpus, and Parse parses %d of them. %d are outside the gate, with %d missing commands: %v",
		parsed, both, parsed-both, len(commands), counts)
	if !update {
		return
	}
	var head strings.Builder
	head.WriteString("# The formulas of corpus.txt that KaTeX parses and texmath.Parse does not, for\n")
	head.WriteString("# the parse gate of katex_parse_test.go, in the order of the corpus. Each line\n")
	head.WriteString("# holds the reason, the missing command or the message of the error, and the\n")
	head.WriteString("# formula. The reasons:\n")
	for _, r := range parseReasons {
		fmt.Fprintf(&head, "#   %s: %s.\n", r.name, r.text)
	}
	if err := os.WriteFile(parseKnownFile, []byte(head.String()+out.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}
