// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

// Package usetable is a port of src/gen-use-table.py of HarfBuzz 15.2.0. It
// reads the files of the Unicode Character Database and the files of
// Microsoft for USE, and gives the USE category of each code point. The
// generator usegen writes the table of the USE shaper of harfbuzz from it,
// and a test of harfbuzz compares that table with it (D101).
package usetable

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Files are the nine input files of gen-use-table.py, in its order. Each
// path is relative to the folder of the data. The last two files come from
// src/ms-use of HarfBuzz.
var Files = [...]string{
	"IndicSyllabicCategory.txt",
	"IndicPositionalCategory.txt",
	"ArabicShaping.txt",
	"DerivedCoreProperties.txt",
	"UnicodeData.txt",
	"Blocks.txt",
	"Scripts.txt",
	"ms-use/IndicSyllabicCategory-Additional.txt",
	"ms-use/IndicPositionalCategory-Additional.txt",
}

// Categories gives the value of each USE category in the syllable machine
// of harfbuzz, the constants useSM_ex_* of ot_shape_use_machine.go and
// useSM_ex_SP of ot_shape_use.go.
var Categories = map[string]uint8{
	"B":     1,
	"CGJ":   6,
	"CMAbv": 31,
	"CMBlw": 32,
	"CS":    43,
	"FAbv":  24,
	"FBlw":  25,
	"FMAbv": 45,
	"FMBlw": 46,
	"FMPst": 47,
	"FPst":  26,
	"G":     49,
	"GB":    5,
	"H":     12,
	"HM":    54,
	"HN":    13,
	"HR":    55,
	"HVM":   53,
	"IS":    44,
	"J":     50,
	"MAbv":  27,
	"MBlw":  28,
	"MPre":  30,
	"MPst":  29,
	"N":     4,
	"O":     0,
	"R":     18,
	"RK":    56,
	"SB":    51,
	"SE":    52,
	"SMAbv": 41,
	"SMBlw": 42,
	"SP":    57,
	"SUB":   11,
	"Sk":    48,
	"VAbv":  33,
	"VBlw":  34,
	"VMAbv": 37,
	"VMBlw": 38,
	"VMPre": 23,
	"VMPst": 39,
	"VPre":  22,
	"VPst":  35,
	"WJ":    16,
	"ZWNJ":  14,
}

// Table is the result of gen-use-table.py.
type Table struct {
	// Headers holds the first two lines of each input file except
	// UnicodeData.txt, which has no header.
	Headers []string
	// Category holds the USE category of each code point that the table
	// holds. The category of another code point is O.
	Category map[rune]string
}

// FindDir looks for upstream/unicode-ucd in the working folder and in each
// folder above it, and returns its path.
func FindDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("finding upstream/unicode-ucd: %w", err)
	}
	for {
		p := filepath.Join(dir, "upstream", "unicode-ucd")
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("finding upstream/unicode-ucd: no such folder")
		}
		dir = parent
	}
}

var disabledScripts = map[string]bool{
	"Arabic":    true,
	"Lao":       true,
	"Samaritan": true,
	"Syriac":    true,
	"Thai":      true,
}

// The properties of a code point, in the order of the input files: UISC,
// UIPC, AJT, UDI, UGC, the block and the script.
type properties [7]string

var defaults = properties{"Other", "Not_Applicable", "jt_X", "", "Cn", "No_Block", "Unknown"}

// Load reads the input files from dir and gives the table.
func Load(dir string) (*Table, error) {
	var t Table
	var unicodeData [7]map[rune]string
	for i := range unicodeData {
		unicodeData[i] = map[rune]string{}
	}
	for i, name := range Files {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return nil, fmt.Errorf("reading the USE data: %w", err)
		}
		lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
		if i != 4 {
			for _, line := range lines[:min(2, len(lines))] {
				t.Headers = append(t.Headers, strings.TrimSpace(line))
			}
		}
		for n, line := range lines {
			if j := strings.IndexByte(line, '#'); j >= 0 {
				line = line[:j]
			}
			fields := strings.Split(line, ";")
			if len(fields) == 1 {
				continue
			}
			for k := range fields {
				fields[k] = strings.TrimSpace(fields[k])
			}
			start, end, err := parseRange(fields[0])
			if err != nil {
				return nil, fmt.Errorf("reading line %d of %s: %w", n+1, name, err)
			}
			k := 1
			if i == 2 || i == 4 {
				k = 2
			}
			if k >= len(fields) {
				return nil, fmt.Errorf("reading line %d of %s: too few fields", n+1, name)
			}
			v := fields[k]
			switch {
			case i == 2:
				v = "jt_" + v
			case i == 3 && v != "Default_Ignorable_Code_Point":
				continue
			case i == 7 && v == "Consonant_Final_Modifier":
				// TODO: https://github.com/MicrosoftDocs/typography-issues/issues/336
				v = "Syllable_Modifier"
			case i == 8 && v == "NA":
				v = "Not_Applicable"
			}
			i0 := i
			if i >= 7 {
				i0 = i - 7
			}
			for u := start; u <= end; u++ {
				unicodeData[i0][u] = v
			}
		}
	}

	// Merge data into one map.
	combined := map[rune]*properties{}
	for i, d := range unicodeData {
		for u, v := range d {
			p, ok := combined[u]
			if !ok {
				if i >= 4 && (i != 4 || v != "Zs") {
					continue
				}
				p = new(properties)
				*p = defaults
				combined[u] = p
			}
			p[i] = v
		}
	}
	for u, p := range combined {
		if disabledScripts[p[6]] {
			delete(combined, u)
		}
	}

	cat, err := mapToUse(combined)
	if err != nil {
		return nil, err
	}
	t.Category = cat
	return &t, nil
}

func parseRange(s string) (start, end rune, err error) {
	a, b, isRange := strings.Cut(s, "..")
	x, err := strconv.ParseUint(a, 16, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("parsing the code point %q: %w", a, err)
	}
	y := x
	if isRange {
		y, err = strconv.ParseUint(b, 16, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("parsing the code point %q: %w", b, err)
		}
	}
	return rune(x), rune(y), nil
}

func in(v string, vs ...string) bool {
	for _, s := range vs {
		if v == s {
			return true
		}
	}
	return false
}

func inR(v rune, vs ...rune) bool {
	for _, s := range vs {
		if v == s {
			return true
		}
	}
	return false
}

func isBase(_ rune, uisc, _, ugc, ajt string) bool {
	return in(uisc, "Number", "Consonant", "Consonant_Head_Letter",
		"Tone_Letter",
		"Vowel_Independent",
	) ||
		// TODO: https://github.com/MicrosoftDocs/typography-issues/issues/484
		in(ajt, "jt_C", "jt_D", "jt_L", "jt_R") && uisc != "Joiner" ||
		(ugc == "Lo" && in(uisc, "Avagraha", "Bindu", "Consonant_Final", "Consonant_Medial",
			"Consonant_Subjoined", "Vowel", "Vowel_Dependent"))
}

func isBaseNum(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Brahmi_Joining_Number"
}

func isBaseOther(u rune, uisc, _, _, _ string) bool {
	if uisc == "Consonant_Placeholder" {
		return true
	}
	return inR(u, 0x2015, 0x2022, 0x25FB, 0x25FC, 0x25FD, 0x25FE)
}

func isCGJ(_ rune, uisc, udi, ugc, _ string) bool {
	// Also includes VARIATION_SELECTOR and ZWJ
	return uisc == "Joiner" || udi != "" && in(ugc, "Mc", "Me", "Mn")
}

func isConsFinal(_ rune, uisc, _, ugc, _ string) bool {
	return (uisc == "Consonant_Final" && ugc != "Lo") ||
		uisc == "Consonant_Succeeding_Repha"
}

func isConsFinalMod(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Syllable_Modifier"
}

func isConsMed(_ rune, uisc, _, ugc, _ string) bool {
	// Consonant_Initial_Postfixed is new in Unicode 11; not in the spec.
	return uisc == "Consonant_Medial" && ugc != "Lo" ||
		uisc == "Consonant_Initial_Postfixed"
}

func isConsMod(_ rune, uisc, _, _, _ string) bool {
	return in(uisc, "Nukta", "Gemination_Mark", "Consonant_Killer")
}

func isConsSub(_ rune, uisc, _, ugc, _ string) bool {
	return uisc == "Consonant_Subjoined" && ugc != "Lo"
}

func isConsWithStacker(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Consonant_With_Stacker"
}

func isHalant(u rune, uisc, udi, ugc, ajt string) bool {
	return uisc == "Virama" && !isHalantOrVowelModifier(u, uisc, udi, ugc, ajt)
}

func isHalantOrVowelModifier(u rune, _, _, _, _ string) bool {
	// Split off of HALANT
	return u == 0x0DCA
}

func isHalantNum(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Number_Joiner"
}

func isHieroglyph(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Hieroglyph"
}

func isHieroglyphJoiner(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Hieroglyph_Joiner"
}

func isHieroglyphMirror(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Hieroglyph_Mirror"
}

func isHieroglyphMod(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Hieroglyph_Modifier"
}

func isHieroglyphSegmentBegin(_ rune, uisc, _, _, _ string) bool {
	return in(uisc, "Hieroglyph_Mark_Begin", "Hieroglyph_Segment_Begin")
}

func isHieroglyphSegmentEnd(_ rune, uisc, _, _, _ string) bool {
	return in(uisc, "Hieroglyph_Mark_End", "Hieroglyph_Segment_End")
}

func isInvisibleStacker(u rune, uisc, udi, ugc, ajt string) bool {
	// Split off of HALANT
	return uisc == "Invisible_Stacker" &&
		!isSakot(u, uisc, udi, ugc, ajt)
}

func isZwnj(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Non_Joiner"
}

func isOther(u rune, uisc, udi, ugc, ajt string) bool {
	// Also includes BASE_IND and SYM
	return (ugc == "Po" || in(uisc, "Consonant_Dead", "Joiner", "Modifying_Letter", "Other")) &&
		!isBase(u, uisc, udi, ugc, ajt) &&
		!isBaseOther(u, uisc, udi, ugc, ajt) &&
		!isCGJ(u, uisc, udi, ugc, ajt) &&
		!isSpace(u, uisc, udi, ugc, ajt) &&
		!isSymMod(u, uisc, udi, ugc, ajt) &&
		!isWordJoiner(u, uisc, udi, ugc, ajt)
}

func isReorderingKiller(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Reordering_Killer"
}

func isRepha(_ rune, uisc, _, _, _ string) bool {
	return in(uisc, "Consonant_Preceding_Repha", "Consonant_Prefixed")
}

func isSakot(u rune, _, _, _, _ string) bool {
	// Split off of HALANT
	return u == 0x1A60
}

func isSpace(u rune, uisc, udi, ugc, ajt string) bool {
	return ugc == "Zs" && !isBaseOther(u, uisc, udi, ugc, ajt)
}

func isSymMod(_ rune, uisc, _, _, _ string) bool {
	return uisc == "Symbol_Modifier"
}

func isVowel(_ rune, uisc, _, ugc, _ string) bool {
	return uisc == "Pure_Killer" ||
		ugc != "Lo" && in(uisc, "Vowel", "Vowel_Dependent")
}

func isVowelMod(_ rune, uisc, _, ugc, _ string) bool {
	return in(uisc, "Tone_Mark", "Cantillation_Mark", "Register_Shifter", "Visarga") ||
		ugc != "Lo" && uisc == "Bindu"
}

func isWordJoiner(u rune, uisc, udi, ugc, ajt string) bool {
	// Also includes Rsv
	return (udi != "" && !inR(u, 0x115F, 0x1160, 0x3164, 0xFFA0, 0x1BCA0, 0x1BCA1, 0x1BCA2, 0x1BCA3) &&
		uisc == "Other" &&
		!isCGJ(u, uisc, udi, ugc, ajt)) || ugc == "Cn"
}

type predicate func(u rune, uisc, udi, ugc, ajt string) bool

var useMapping = [...]struct {
	name string
	f    predicate
}{
	{"B", isBase},
	{"N", isBaseNum},
	{"GB", isBaseOther},
	{"CGJ", isCGJ},
	{"F", isConsFinal},
	{"FM", isConsFinalMod},
	{"M", isConsMed},
	{"CM", isConsMod},
	{"SUB", isConsSub},
	{"CS", isConsWithStacker},
	{"H", isHalant},
	{"HVM", isHalantOrVowelModifier},
	{"HN", isHalantNum},
	{"IS", isInvisibleStacker},
	{"G", isHieroglyph},
	{"HM", isHieroglyphMod},
	{"HR", isHieroglyphMirror},
	{"J", isHieroglyphJoiner},
	{"SB", isHieroglyphSegmentBegin},
	{"SE", isHieroglyphSegmentEnd},
	{"ZWNJ", isZwnj},
	{"O", isOther},
	{"SP", isSpace},
	{"RK", isReorderingKiller},
	{"R", isRepha},
	{"Sk", isSakot},
	{"SM", isSymMod},
	{"V", isVowel},
	{"VM", isVowelMod},
	{"WJ", isWordJoiner},
}

type position struct {
	suffix string
	uipc   []string
}

// usePositions holds the categories of use_positions of gen-use-table.py.
// A category with no position, such as H, maps to nil.
var usePositions = map[string][]position{
	"F": {
		{"Abv", []string{"Top"}},
		{"Blw", []string{"Bottom"}},
		{"Pst", []string{"Right"}},
	},
	"M": {
		{"Abv", []string{"Top"}},
		{"Blw", []string{"Bottom", "Bottom_And_Left", "Bottom_And_Right"}},
		{"Pst", []string{"Right"}},
		{"Pre", []string{"Left", "Top_And_Bottom_And_Left"}},
	},
	"CM": {
		{"Abv", []string{"Top"}},
		{"Blw", []string{"Bottom", "Overstruck"}},
	},
	"V": {
		{"Abv", []string{"Top", "Top_And_Bottom", "Top_And_Bottom_And_Right", "Top_And_Right"}},
		{"Blw", []string{"Bottom", "Overstruck", "Bottom_And_Right"}},
		{"Pst", []string{"Right"}},
		{"Pre", []string{"Left", "Top_And_Left", "Top_And_Left_And_Right", "Left_And_Right"}},
	},
	"VM": {
		{"Abv", []string{"Top"}},
		{"Blw", []string{"Bottom", "Overstruck"}},
		{"Pst", []string{"Right"}},
		{"Pre", []string{"Left"}},
	},
	"SM": {
		{"Abv", []string{"Top"}},
		{"Blw", []string{"Bottom"}},
	},
	"H":   nil,
	"HM":  nil,
	"HR":  nil,
	"HVM": nil,
	"IS":  nil,
	"B":   nil,
	"FM": {
		{"Abv", []string{"Top"}},
		{"Blw", []string{"Bottom"}},
		{"Pst", []string{"Not_Applicable"}},
	},
	"R":   nil,
	"RK":  nil,
	"SUB": nil,
}

func mapToUse(data map[rune]*properties) (map[rune]string, error) {
	out := make(map[rune]string, len(data))
	for u, p := range data {
		uisc, uipc, ajt, udi, ugc := p[0], p[1], p[2], p[3], p[4]

		// Resolve Indic_Syllabic_Category

		// TODO: These don't have UISC assigned in Unicode 13.0.0, but have UIPC
		if 0x1CE2 <= u && u <= 0x1CE8 {
			uisc = "Cantillation_Mark"
		}

		// Tibetan:
		// TODO: These don't have UISC assigned in Unicode 13.0.0, but have UIPC
		if 0x0F18 <= u && u <= 0x0F19 || 0x0F3E <= u && u <= 0x0F3F {
			uisc = "Vowel_Dependent"
		}

		// TODO: U+1CED should only be allowed after some of
		// the nasalization marks, maybe only for U+1CE9..U+1CF1.
		if u == 0x1CED {
			uisc = "Tone_Mark"
		}

		var values []string
		for _, m := range useMapping {
			if m.f(u, uisc, udi, ugc, ajt) {
				values = append(values, m.name)
			}
		}
		if len(values) != 1 {
			return nil, fmt.Errorf("mapping %U (%s %s %s %s) to USE: the categories %v match", u, uisc, udi, ugc, ajt, values)
		}
		use := values[0]

		// Resolve Indic_Positional_Category

		// TODO: https://github.com/harfbuzz/harfbuzz/pull/1037
		//  and https://github.com/harfbuzz/harfbuzz/issues/1631
		if inR(u, 0x11302, 0x11303, 0x114C1) {
			uipc = "Top"
		}

		posMapping, hasPos := usePositions[use]
		if !in(uipc, "Not_Applicable", "Visual_Order_Left") && u != 0x0F7F && !hasPos {
			return nil, fmt.Errorf("mapping %U (%s %s %s %s %s %s) to USE: the category has no position", u, uipc, use, uisc, udi, ugc, ajt)
		}

		if len(posMapping) != 0 {
			var values []string
			for _, pos := range posMapping {
				if in(uipc, pos.uipc...) {
					values = append(values, pos.suffix)
				}
			}
			if len(values) != 1 {
				return nil, fmt.Errorf("mapping %U (%s %s %s %s %s %s) to USE: the positions %v match", u, uipc, use, uisc, udi, ugc, ajt, values)
			}
			use += values[0]
		}

		out[u] = use
	}
	return out, nil
}
