package harfbuzz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests of D90 shape at a scale other than the units per em of the
// font. HarfBuzz scales each value of the font with em_mult, which rounds
// to the nearest unit and keeps 32 bits.

// scales are the scales of the tests of D90: the scale of shaping for a
// size of 12, and a scale that is not a power of 2 of it.
var scales = []int32{768, 1000}

// TestScale shapes texts at the scales, and compares harfbuzz with
// HarfBuzz:
//   - Roboto, whose GPOS kerns the pairs of capitals.
//   - The first 30 lines of each text of perf_reference in each of its
//     fonts, with GPOS, marks and cursive attachments.
//   - Roboto with a table kerx whose subtable of format 0 kerns a, b by
//     -301 and b, c by 77.
//   - Roboto with the subtables of kerxMarks: format 1 moves a across the
//     line by 101, and format 4 attaches b to a with the anchors 51, 61 and
//     11, 21.
//   - Roboto with a table kern of Microsoft that kerns a, b by -301.
func TestScale(t *testing.T) {
	roboto, err := os.ReadFile("../../typesetting/font/testdata/Roboto-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	for _, scale := range scales {
		compareShapingAt(t, roboto, []string{"AVATAWAYTo.LTA", "Type faces", "WAVE"}, false, scale)
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
			raw, err := os.ReadFile(textPath)
			if err != nil {
				t.Fatal(err)
			}
			var lines []string
			for line := range strings.SplitSeq(string(raw), "\n") {
				if strings.TrimSpace(line) != "" && len(lines) < 30 {
					lines = append(lines, line)
				}
			}
			for _, scale := range scales {
				t.Run(filepath.Base(fontPath)+"/"+filepath.Base(textPath), func(t *testing.T) {
					compareShapingAt(t, data, lines, false, scale)
				})
			}
		}
	}

	for _, layout := range []map[string][]byte{
		{"kerx": kerxTable(kerxPairs(robotoA, robotoB, -301, robotoB, robotoC, 77))},
		{"kerx": kerxMarks(kerxFormat4, []int{101}, []int{51, 61, 11, 21})},
		{"kern": kernOT(kernPairs(14+6, robotoA, robotoB, -301))},
	} {
		data := robotoWith(t, layout)
		for _, scale := range scales {
			compareShapingAt(t, data, []string{"ab", "abc", "ba", "aab", "abab", "cab"}, false, scale)
		}
	}
}

// TestKerxFormat6LongValues: a subtable of format 6 of kerx, whose rows
// give 0 to a and 2 to b, and whose columns give 0 to a and 1 to b, with
// the long values 0, -100,000, 70,000 and 0x12345. HarfBuzz keeps the 32
// bits of each value, at the units per em of the font and at the scales.
func TestKerxFormat6LongValues(t *testing.T) {
	rows := append(u16s(8, robotoA, 2), u32s(0, 2)...)
	columns := append(u16s(8, robotoA, 2), u32s(0, 1)...)
	neg := int32(-100000)
	long := u32s(0, uint32(neg), 70000, 0x12345)
	data := robotoWith(t, map[string][]byte{"kerx": kerxTable(kerxFormat6Data(1, 2, rows, columns, long, nil))})
	texts := []string{"ab", "ba", "bb", "aa", "abba"}
	testCase(t, data, false, texts...)
	for _, scale := range scales {
		compareShapingAt(t, data, texts, false, scale)
	}
}
