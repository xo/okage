package harfbuzz_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The tests of D93 compare the flag unsafe-to-concat of HarfBuzz, which
// the buffer flag ProduceUnsafeToConcat turns on.

// compareConcat shapes each text in the font data with harfbuzz and with
// HarfBuzz, with the buffer flag ProduceUnsafeToConcat, and fails the test
// for each difference in the glyphs, their positions or their flags
// unsafe-to-break and unsafe-to-concat.
func compareConcat(t *testing.T, data []byte, texts []string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	want := shapeHarfBuzz(t, path, texts, false, "--concat")
	got := shapeOkageConcat(t, data, texts, false, true, 0)
	reportDiffs(t, texts, got, want)
}

// setTable builds a subtable of format 1 whose coverage holds the glyph
// first, with one set of the items, each from the function item: a
// ligature set of a ligature subtable, or a rule set of a context or
// chained context subtable.
func setTable[T any](first int, items []T, item func(T) []byte) []byte {
	set := u16s(len(items))
	var body []byte
	for _, it := range items {
		set = append(set, u16s(2+2*len(items)+len(body))...)
		body = append(body, item(it)...)
	}
	set = append(set, body...)
	coverage := coverage1(first)
	out := u16s(1, 8, 1, 8+len(coverage))
	out = append(out, coverage...)
	return append(out, set...)
}

// chainRule is a rule of a chained context subtable, with no lookup
// records: its backtrack, its input after the first glyph, and its
// lookahead.
type chainRule struct{ backtrack, input, lookahead []int }

// chainRuleSet builds a chained context subtable of format 1 with one
// rule set for the glyph first.
func chainRuleSet(first int, rules ...chainRule) []byte {
	return setTable(first, rules, func(r chainRule) []byte {
		out := u16s(len(r.backtrack))
		out = append(out, u16s(r.backtrack...)...)
		out = append(out, u16s(len(r.input)+1)...)
		out = append(out, u16s(r.input...)...)
		out = append(out, u16s(len(r.lookahead))...)
		out = append(out, u16s(r.lookahead...)...)
		return append(out, u16s(0)...)
	})
}

// ruleSet builds a context subtable of format 1 with one rule set for the
// glyph first. Each rule is its input after the first glyph, with no
// lookup records.
func ruleSet(first int, rules ...[]int) []byte {
	return setTable(first, rules, func(input []int) []byte {
		return append(u16s(len(input)+1, 0), u16s(input...)...)
	})
}

// ligatureSet builds a ligature subtable with one ligature set for the
// glyph first. Each ligature is its components after the first glyph, and
// gives the glyph z.
func ligatureSet(first int, ligatures ...[]int) []byte {
	return setTable(first, ligatures, func(components []int) []byte {
		return append(u16s(robotoZ, len(components)+1), u16s(components...)...)
	})
}

// TestRuleSetUnsafeToConcat compares the flags with HarfBuzz for lookups
// built by hand, whose rules and ligatures do not match the text in each
// way. HarfBuzz 15.1.0 tries a ligature set and a rule set of more than 4
// rules on a fast path, which tests the first glyphs after the current
// glyph before it tries a rule. Each path marks other glyphs unsafe to
// concat. A chained context rule whose input does not match marks no
// glyph.
func TestRuleSetUnsafeToConcat(t *testing.T) {
	b, c, x, y := robotoB, robotoC, robotoX, robotoY
	five := []chainRule{{nil, []int{b}, nil}, {nil, []int{b, b}, nil}, {nil, []int{x}, nil}, {nil, []int{y}, nil}, {nil, []int{robotoZ}, nil}}
	texts := []string{"a", "xa", "ab", "ac", "abc", "abb", "acb", "aa", "abx", "ax", "acx", "bac", "cab", "xab", "cac"}
	for _, lookup := range [][]byte{
		lookupTable(t, 6, 0, chainRuleSet(robotoA, chainRule{nil, []int{b}, nil})),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, five[:2]...)),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, five...)),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, append([]chainRule{{nil, nil, []int{x}}}, five...)...)),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, append(slices.Clone(five), chainRule{})...)),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, chainRule{[]int{x}, []int{b}, nil})),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, chainRule{nil, []int{b}, []int{x}})),
		lookupTable(t, 6, 0, chainRuleSet(robotoA, chainRule{[]int{b, b}, []int{b}, nil}, chainRule{[]int{b}, []int{b}, nil})),
		lookupTable(t, 5, 0, ruleSet(robotoA, []int{b, c})),
		lookupTable(t, 5, 0, ruleSet(robotoA, []int{b, c}, []int{x})),
		lookupTable(t, 5, 0, ruleSet(robotoA, []int{b, c}, []int{x}, []int{y}, []int{b, x}, []int{c, c})),
		lookupTable(t, 5, 0, ruleSet(robotoA, []int{b, c}, []int{x}, []int{y}, []int{b, x}, []int{c, c}, nil)),
		lookupTable(t, 4, 0, ligatureSet(robotoA, []int{b, c})),
		lookupTable(t, 4, 0, ligatureSet(robotoA, []int{b, c}, []int{x})),
		lookupTable(t, 4, 0, ligatureSet(robotoA, []int{x}, []int{b, c})),
		lookupTable(t, 4, 0, ligatureSet(robotoA, []int{b, c}, []int{x}, []int{y}, []int{b, x}, []int{c, c})),
		lookupTable(t, 4, 0, ligatureSet(robotoA, []int{x}, []int{y}, []int{b, x}, []int{c, c}, nil)),
	} {
		gsub := layoutTable(t, []string{"DFLT", "latn"}, []layoutFeature{{"liga", []int{0}}}, [][]byte{lookup})
		compareConcat(t, robotoWith(t, map[string][]byte{"GSUB": gsub}), texts)
	}
}

// TestUnsafeToConcat compares the flags unsafe-to-break and
// unsafe-to-concat of harfbuzz with HarfBuzz, on texts in Roboto and Amiri
// of typesetting/font/testdata, and on the texts of perf_reference in each
// of its fonts, a line at a time. By default it shapes the first 200 lines
// of each text, and with -harfbuzz.all each line.
func TestUnsafeToConcat(t *testing.T) {
	for _, c := range []struct {
		font  string
		texts []string
	}{
		{"Roboto-Regular.ttf", []string{"AVATAR Wave, To. office", "fi ffi Type faces", "Tv ´a"}},
		{"Amiri-Regular.ttf", []string{"بسم الله الرحمن الرحيم", "لَكَ ـــ أُم", "١٢٣ ـــــ ـ"}},
	} {
		data, err := os.ReadFile(filepath.Join("../../typesetting/font/testdata", c.font))
		if err != nil {
			t.Fatal(err)
		}
		compareConcat(t, data, c.texts)
	}

	fonts, _ := filepath.Glob(filepath.Join(perfReference, "fonts", "*"))
	texts, _ := filepath.Glob(filepath.Join(perfReference, "texts", "*"))
	if len(fonts) == 0 || len(texts) == 0 {
		t.Skip("the folder typesetting/testdata is missing")
	}
	for _, fontPath := range fonts {
		data, err := os.ReadFile(fontPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, textPath := range texts {
			t.Run(filepath.Base(fontPath)+"/"+filepath.Base(textPath), func(t *testing.T) {
				raw, err := os.ReadFile(textPath)
				if err != nil {
					t.Fatal(err)
				}
				var lines []string
				for line := range strings.SplitSeq(string(raw), "\n") {
					if strings.TrimSpace(line) != "" {
						lines = append(lines, line)
					}
				}
				if !*all && len(lines) > 200 {
					lines = lines[:200]
				}
				compareConcat(t, data, lines)
			})
		}
	}
}
