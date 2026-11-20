package harfbuzz_test

import (
	"bufio"
	"bytes"
	"cmp"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage/typesetting/font"
	ot "github.com/xo/okage/typesetting/font/opentype"
	"github.com/xo/okage/typesetting/harfbuzz"
)

var all = flag.Bool("harfbuzz.all", false, "shape all the lines of the texts of perf_reference")

// perfReference is the folder of the fonts and the texts of the
// benchmarks of HarfBuzz, in the test data of typesetting.
const perfReference = "../../typesetting/testdata/harfbuzz/perf_reference"

// shaped is the output of a shaping: one line for each glyph, "glyph
// cluster xAdvance yAdvance xOffset yOffset", with the flag unsafe-to-break
// at the end when the test asks for it, and whether HarfBuzz ends in error.
type shaped struct {
	glyphs  []string
	inError bool
}

// shapeHarfBuzz shapes each text in the font of the file with shape.py,
// with the flag unsafe-to-break of each glyph when flags is true, and with
// the other arguments of shape.py in args. It skips the test when python3
// or uharfbuzz with HarfBuzz 15.1.0 is missing.
func shapeHarfBuzz(t *testing.T, path string, texts []string, flags bool, args ...string) []shaped {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is missing")
	}
	version, err := exec.CommandContext(t.Context(), python, "-I", "shape.py", "--version").Output()
	if err != nil {
		t.Skip("uharfbuzz is missing")
	}
	if v := strings.TrimSpace(string(version)); v != "15.1.0" {
		t.Skipf("uharfbuzz has HarfBuzz %s, want 15.1.0", v)
	}
	var stdin bytes.Buffer
	for _, text := range texts {
		for i, r := range text {
			if i != 0 {
				stdin.WriteByte(' ')
			}
			fmt.Fprintf(&stdin, "%x", r)
		}
		stdin.WriteByte('\n')
	}
	var stderr bytes.Buffer
	args = append([]string{"-I", "shape.py", path}, args...)
	if flags {
		args = append(args, "--flags")
	}
	cmd := exec.CommandContext(t.Context(), python, args...)
	cmd.Stdin = &stdin
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("running shape.py on %s: %v\n%s", path, err, stderr.Bytes())
	}
	var (
		results []shaped
		cur     shaped
	)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		switch line := scanner.Text(); line {
		case "--":
			results = append(results, cur)
			cur = shaped{}
		case "error":
			cur.inError = true
		default:
			cur.glyphs = append(cur.glyphs, line)
		}
	}
	if len(results) != len(texts) {
		t.Fatalf("shape.py gave %d outputs for %d texts", len(results), len(texts))
	}
	return results
}

// shapeOkage shapes each text in the font data with harfbuzz, with the
// flag unsafe-to-break of each glyph when flags is true.
func shapeOkage(t *testing.T, data []byte, texts []string, flags bool) [][]string {
	t.Helper()
	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	hf := harfbuzz.NewFont(face)
	buf := harfbuzz.NewBuffer()
	out := make([][]string, len(texts))
	for i, text := range texts {
		buf.Clear()
		buf.AddRunes([]rune(text), 0, -1)
		buf.GuessSegmentProperties()
		buf.Shape(hf, nil)
		glyphs := make([]string, len(buf.Info))
		for j, info := range buf.Info {
			pos := buf.Pos[j]
			glyphs[j] = fmt.Sprintf("%d %d %d %d %d %d", info.Glyph, info.Cluster, pos.XAdvance, pos.YAdvance, pos.XOffset, pos.YOffset)
			if flags {
				unsafe := 0
				if info.Mask&harfbuzz.GlyphUnsafeToBreak != 0 {
					unsafe = 1
				}
				glyphs[j] += fmt.Sprintf(" %d", unsafe)
			}
		}
		out[i] = glyphs
	}
	return out
}

// compareShaping shapes each text in the font data with harfbuzz and with
// HarfBuzz, and fails the test for each difference. When flags is true, it
// also compares the flag unsafe-to-break of each glyph. It gives the output
// of HarfBuzz.
func compareShaping(t *testing.T, data []byte, texts []string, flags bool) []shaped {
	t.Helper()
	path := filepath.Join(t.TempDir(), "font.ttf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	want := shapeHarfBuzz(t, path, texts, flags)
	got := shapeOkage(t, data, texts, flags)
	for i, text := range texts {
		if slices.Equal(got[i], want[i].glyphs) {
			continue
		}
		t.Errorf("%+q: got %d glyphs, HarfBuzz gives %d", trim(text), len(got[i]), len(want[i].glyphs))
		for j := range max(len(got[i]), len(want[i].glyphs)) {
			g, w := "", ""
			if j < len(got[i]) {
				g = got[i][j]
			}
			if j < len(want[i].glyphs) {
				w = want[i].glyphs[j]
			}
			if g != w {
				t.Errorf("  glyph %d: got %q, HarfBuzz gives %q", j, g, w)
				break
			}
		}
	}
	return want
}

// trim shortens a long text for a message.
func trim(text string) string {
	if r := []rune(text); len(r) > 40 {
		return string(r[:40]) + "..."
	}
	return text
}

// TestPerfReference shapes the texts of perf_reference in each of its
// fonts, a line at a time, and compares harfbuzz with HarfBuzz. By default
// it shapes the first 200 lines of each text, and with -harfbuzz.all each
// line.
func TestPerfReference(t *testing.T) {
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
				compareShaping(t, data, lines, false)
			})
		}
	}
}

// The tests of D73 below build fonts by hand, as the tests of D73 in
// typesetting/harfbuzz do. Each test also checks whether HarfBuzz ends in
// error, so that the font makes the case that the test names.

// The glyphs of Roboto-Regular.ttf of typesetting/font/testdata.
const (
	robotoA = 69
	robotoB = 70
	robotoC = 71
	robotoX = 92
	robotoY = 93
	robotoZ = 94
)

// u16s gives the values as 16-bit integers in big-endian order.
func u16s(values ...int) []byte {
	out := make([]byte, 0, 2*len(values))
	for _, v := range values {
		out = binary.BigEndian.AppendUint16(out, uint16(v))
	}
	return out
}

// u32s gives the values as 32-bit integers in big-endian order.
func u32s(values ...uint32) []byte {
	out := make([]byte, 0, 4*len(values))
	for _, v := range values {
		out = binary.BigEndian.AppendUint32(out, v)
	}
	return out
}

// offsetTo gives a 16-bit offset, and fails the test when it does not fit.
func offsetTo(t *testing.T, offset int) int {
	t.Helper()
	if offset > 0xFFFF {
		t.Fatalf("the offset %d does not fit in 16 bits", offset)
	}
	return offset
}

// layoutFeature is a feature of a table GSUB or GPOS.
type layoutFeature struct {
	tag     string
	lookups []int
}

// layoutTable builds a table GSUB or GPOS. Each script has one default
// LangSys with all the features. A lookup is the bytes of a whole Lookup
// table. Only the last lookup can be larger than 64 KiB.
func layoutTable(t *testing.T, scripts []string, features []layoutFeature, lookups [][]byte) []byte {
	t.Helper()
	scriptList := u16s(len(scripts))
	for _, tag := range scripts {
		scriptList = append(scriptList, tag...)
		scriptList = append(scriptList, u16s(2+6*len(scripts))...)
	}
	scriptList = append(scriptList, u16s(4, 0)...)
	scriptList = append(scriptList, u16s(0, 0xFFFF, len(features))...)
	for i := range features {
		scriptList = append(scriptList, u16s(i)...)
	}

	featureList := u16s(len(features))
	var featureTables []byte
	for _, f := range features {
		featureList = append(featureList, f.tag...)
		featureList = append(featureList, u16s(offsetTo(t, 2+6*len(features)+len(featureTables)))...)
		featureTables = append(featureTables, u16s(0, len(f.lookups))...)
		featureTables = append(featureTables, u16s(f.lookups...)...)
	}
	featureList = append(featureList, featureTables...)

	lookupList := u16s(len(lookups))
	var lookupTables []byte
	for _, lookup := range lookups {
		lookupList = append(lookupList, u16s(offsetTo(t, 2+2*len(lookups)+len(lookupTables)))...)
		lookupTables = append(lookupTables, lookup...)
	}
	lookupList = append(lookupList, lookupTables...)

	out := []byte{0, 1, 0, 0}
	out = append(out, u16s(10, offsetTo(t, 10+len(scriptList)), offsetTo(t, 10+len(scriptList)+len(featureList)))...)
	out = append(out, scriptList...)
	out = append(out, featureList...)
	return append(out, lookupList...)
}

// lookupTable builds a Lookup table with its subtables. Only the last
// subtable can be larger than 64 KiB.
func lookupTable(t *testing.T, lookupType, flag int, subtables ...[]byte) []byte {
	t.Helper()
	out := u16s(lookupType, flag, len(subtables))
	var data []byte
	for _, subtable := range subtables {
		out = append(out, u16s(offsetTo(t, 6+2*len(subtables)+len(data)))...)
		data = append(data, subtable...)
	}
	return append(out, data...)
}

// coverage1 builds a Coverage table of format 1. The glyphs must be sorted.
func coverage1(glyphs ...int) []byte {
	out := u16s(1, len(glyphs))
	return append(out, u16s(glyphs...)...)
}

// replaceLayout gives the bytes of the font data with the tables of layout
// in place of its tables GDEF, GSUB and GPOS.
func replaceLayout(t *testing.T, data []byte, layout map[string][]byte) []byte {
	t.Helper()
	ld, err := ot.NewLoader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var tabs []ot.Table
	for _, tag := range ld.Tables() {
		switch tag.String() {
		case "GDEF", "GSUB", "GPOS":
			continue
		}
		raw, err := ld.RawTable(tag)
		if err != nil {
			t.Fatal(err)
		}
		tabs = append(tabs, ot.Table{Tag: tag, Content: raw})
	}
	for tag, content := range layout {
		tabs = append(tabs, ot.Table{Tag: ot.MustNewTag(tag), Content: content})
	}
	slices.SortFunc(tabs, func(a, b ot.Table) int { return cmp.Compare(a.Tag, b.Tag) })
	return ot.WriteOpentype(tabs, ot.TrueType)
}

// robotoWith gives Roboto-Regular.ttf with the tables of layout.
func robotoWith(t *testing.T, layout map[string][]byte) []byte {
	t.Helper()
	data, err := os.ReadFile("../../typesetting/font/testdata/Roboto-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	return replaceLayout(t, data, layout)
}

// testCase shapes the texts in the font data, compares harfbuzz with
// HarfBuzz, and checks whether HarfBuzz ends in error.
func testCase(t *testing.T, data []byte, inError bool, texts ...string) {
	t.Helper()
	testFlags(t, data, inError, false, texts...)
}

// testFlags is testCase, which also compares the flag unsafe-to-break of
// each glyph when flags is true.
func testFlags(t *testing.T, data []byte, inError, flags bool, texts ...string) {
	t.Helper()
	for i, out := range compareShaping(t, data, texts, flags) {
		if out.inError != inError {
			t.Errorf("%+q: HarfBuzz ends in error: %v, want %v", trim(texts[i]), out.inError, inError)
		}
	}
}

// TestPairPosClassZero: a PairPos of format 2 for a, whose ClassDef2 holds
// only c, and a PairPos of format 1 for a, b. b has the class 0 in the
// first subtable, which applies.
func TestPairPosClassZero(t *testing.T) {
	pair2 := u16s(2, 20, 4, 0, 26, 30, 1, 2, -100, -200)
	pair2 = append(pair2, coverage1(robotoA)...)
	pair2 = append(pair2, u16s(2, 0)...)
	pair2 = append(pair2, u16s(1, robotoC, 1, 1)...)
	pair1 := u16s(1, 18, 4, 0, 1, 12, 1, robotoB, -300)
	pair1 = append(pair1, coverage1(robotoA)...)
	gpos := layoutTable(t, []string{"DFLT"}, []layoutFeature{{"kern", []int{0}}},
		[][]byte{lookupTable(t, 2, 0, pair2, pair1)})
	testCase(t, robotoWith(t, map[string][]byte{"GPOS": gpos}), false, "ab", "ac", "abcab")
}

// TestDropOutputWithoutOperations: a context of one glyph calls a
// MultipleSubst once for x and 20,000 times for a. The MultipleSubst
// replaces a with b, b with a, and x with z, y. The buffer has no
// operations left in the middle of the lookup, and HarfBuzz drops the
// output of the lookup.
func TestDropOutputWithoutOperations(t *testing.T) {
	const records = 20000
	multiple := u16s(1, 26, 3, 12, 16, 20, 1, robotoB, 1, robotoA, 2, robotoZ, robotoY)
	multiple = append(multiple, coverage1(robotoA, robotoB, robotoX)...)
	context := u16s(1, 10, 2, 30, 18)
	context = append(context, coverage1(robotoA, robotoX)...)
	context = append(context, u16s(1, 4, 1, 1, 0, 0)...)
	context = append(context, u16s(1, 4, 1, records)...)
	for range records {
		context = append(context, u16s(0, 0)...)
	}
	gsub := layoutTable(t, []string{"DFLT"}, []layoutFeature{{"ccmp", []int{1}}},
		[][]byte{lookupTable(t, 2, 0, multiple), lookupTable(t, 5, 0, context)})
	testCase(t, robotoWith(t, map[string][]byte{"GSUB": gsub}), true, "xaaa", "xaaab", "axaa")
}

// TestBufferLengthLimit: a MultipleSubst replaces x with count glyphs y.
// The buffer of xx can hold 65,536 glyphs, but HarfBuzz compares the
// length with that limit only when its arrays grow, and they grow from
// 62,922 to 94,415 glyphs.
func TestBufferLengthLimit(t *testing.T) {
	for _, test := range []struct {
		count   int
		inError bool
	}{
		{40000, false},
		{50000, true},
	} {
		multiple := u16s(1, 8, 1, 14)
		multiple = append(multiple, coverage1(robotoX)...)
		multiple = append(multiple, u16s(test.count)...)
		for range test.count {
			multiple = append(multiple, u16s(robotoY)...)
		}
		gsub := layoutTable(t, []string{"DFLT"}, []layoutFeature{{"ccmp", []int{0}}},
			[][]byte{lookupTable(t, 2, 0, multiple)})
		testCase(t, robotoWith(t, map[string][]byte{"GSUB": gsub}), test.inError, "xx")
	}
}

// TestMorxSubtableOperations: a chain of morx with three insertion
// subtables, each of which inserts 31 times a after each a, then a
// noncontextual subtable that replaces a with b. The noncontextual
// subtable takes 32,768 operations, more than the buffer has left.
func TestMorxSubtableOperations(t *testing.T) {
	insertion := u32s(5, 20, 28, 48, 64)
	insertion = append(insertion, u16s(8, robotoA, 1, 4)...)
	insertion = append(insertion, u16s(0, 0, 0, 0, 1, 0, 0, 0, 0, 1)...)
	insertion = append(insertion, u16s(0, 0, 0xFFFF, 0xFFFF, 0, 31<<5, 0, 0xFFFF)...)
	for range 31 {
		insertion = append(insertion, u16s(robotoA)...)
	}
	noncontextual := u16s(8, robotoA, 1, robotoB)
	const count = 3
	length := 16 + count*(12+len(insertion)) + 12 + len(noncontextual)
	morx := u16s(2, 0)
	morx = append(morx, u32s(1, 1, uint32(length), 0, count+1)...)
	for range count {
		morx = append(morx, u32s(uint32(12+len(insertion)), 0x20000005, 1)...)
		morx = append(morx, insertion...)
	}
	morx = append(morx, u32s(uint32(12+len(noncontextual)), 0x20000004, 1)...)
	morx = append(morx, noncontextual...)
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), true, "a")
	testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "b")
}

// TestIndicWouldSubstituteOperations: NotoSansDevanagari-Regular.ttf with a
// table GSUB whose feature pres replaces ka with kha, and whose feature
// blwf has 32,000 rules for the virama. The questions of the Indic shaper
// about blwf take the operations of the buffer.
func TestIndicWouldSubstituteOperations(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(perfReference, "fonts", "NotoSansDevanagari-Regular.ttf"))
	if err != nil {
		t.Skip("the folder typesetting/testdata is missing")
	}
	const ka, kha, virama, rules = 25, 26, 81, 32000
	single := u16s(1, 6, kha-ka)
	single = append(single, coverage1(ka)...)
	context := u16s(1, 8, 1, 14)
	context = append(context, coverage1(virama)...)
	context = append(context, u16s(rules)...)
	for range rules {
		context = append(context, u16s(2+2*rules)...)
	}
	context = append(context, u16s(2, 0, 1)...)
	gsub := layoutTable(t, []string{"DFLT", "dev2"}, []layoutFeature{{"blwf", []int{1}}, {"pres", []int{0}}},
		[][]byte{lookupTable(t, 1, 0, single), lookupTable(t, 5, 0, context)})
	data = replaceLayout(t, data, map[string][]byte{"GSUB": gsub})
	testCase(t, data, true, "कककक", "कककककक")
	testCase(t, data, false, "कक")
}

// TestIndicFirstGlyphCoverage: NotoSansDevanagari-Regular.ttf with a table
// GSUB whose feature blwf has a chained context of format 1 that covers ka
// and matches no text, then a chained context of format 3 that matches kha
// and the virama. The question of the Indic shaper about ka and the virama
// does not test the coverage of ka in the context of format 3.
func TestIndicFirstGlyphCoverage(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(perfReference, "fonts", "NotoSansDevanagari-Regular.ttf"))
	if err != nil {
		t.Skip("the folder typesetting/testdata is missing")
	}
	const ka, kha, virama = 25, 26, 81
	format1 := u16s(1, 8, 1, 14)
	format1 = append(format1, coverage1(ka)...)
	format1 = append(format1, u16s(1, 4, 0, 2, 1, 0, 0)...)
	format3 := u16s(3, 0, 2, 14, 20, 0, 0)
	format3 = append(format3, coverage1(kha)...)
	format3 = append(format3, coverage1(virama)...)
	gsub := layoutTable(t, []string{"DFLT", "dev2"}, []layoutFeature{{"blwf", []int{0}}},
		[][]byte{lookupTable(t, 6, 0, format1, format3)})
	data = replaceLayout(t, data, map[string][]byte{"GSUB": gsub})
	testCase(t, data, false, "क्कि", "क्क", "क्ककि", "कि")
}

// kerxSubtable is a subtable of a table kerx: its coverage, with the format
// in the low byte, and its data after its header of 12 bytes. When length
// is not 0, it is the length that the header gives, in place of the real
// length.
type kerxSubtable struct {
	coverage uint32
	data     []byte
	length   int
}

// kerxTable builds a table kerx of version 2 with the subtables.
func kerxTable(subtables ...kerxSubtable) []byte {
	out := u16s(2, 0)
	out = append(out, u32s(uint32(len(subtables)))...)
	for _, st := range subtables {
		length := 12 + len(st.data)
		if st.length != 0 {
			length = st.length
		}
		out = append(out, u32s(uint32(length), st.coverage, 0)...)
		out = append(out, st.data...)
	}
	return out
}

// kerxPairs builds a subtable of format 0 of kerx with the pairs, each a
// left glyph, a right glyph and a value. The pairs must be sorted.
func kerxPairs(pairs ...int) kerxSubtable {
	n := uint32(len(pairs) / 3)
	data := u32s(n, 6*n, 0, 0)
	data = append(data, u16s(pairs...)...)
	return kerxSubtable{coverage: 0, data: data}
}

// TestAATSubtableLimit: tables kerx, kern and morx with 1,024 and 1,025
// subtables. In kerx and kern, the last subtable kerns a, b, and the other
// subtables are empty. In morx, one chain has subtables that each replace
// a with b. HarfBuzz refuses each table with 1,025 subtables.
func TestAATSubtableLimit(t *testing.T) {
	for _, count := range []int{1024, 1025} {
		subtables := make([]kerxSubtable, count)
		for i := range count - 1 {
			subtables[i] = kerxPairs()
		}
		subtables[count-1] = kerxPairs(robotoA, robotoB, -300)
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerxTable(subtables...)}), false, "ab", "aab")

		kern := u16s(0, count)
		for range count - 1 {
			kern = append(kern, u16s(0, 14, 1, 0, 0, 0, 0)...)
		}
		kern = append(kern, u16s(0, 20, 1, 1, 6, 0, 0, robotoA, robotoB, -300)...)
		testCase(t, robotoWith(t, map[string][]byte{"kern": kern}), false, "ab", "aab")

		noncontextual := u16s(8, robotoA, 1, robotoB)
		morx := u16s(2, 0)
		morx = append(morx, u32s(1, 1, uint32(16+count*(12+len(noncontextual))), 0, uint32(count))...)
		for range count {
			morx = append(morx, u32s(uint32(12+len(noncontextual)), 0x20000004, 1)...)
			morx = append(morx, noncontextual...)
		}
		testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "a", "ab")
	}
}

// The flags of an entry of a subtable of format 1 of kerx.
const (
	kerxPush  = 0x8000
	kerxReset = 0x2000
	kerxEnd   = 0xFFFF // the index of no list of actions, and the end of a list
)

// kerxFormat1 builds a subtable of format 1 of kerx. Its class table gives
// the classes 4, 5 and 6 to a, b and c. An entry is its flags and the index
// of its list of actions in the values. entries[0] is the entry of the
// classes 0 to 3, and entries[1], entries[2] and entries[3] are the
// entries of a, b and c, in each of the two states.
func kerxFormat1(entries [4][2]int, values ...int) kerxSubtable {
	const classes = 7
	classTable := u16s(8, robotoA, 3, 4, 5, 6)
	var states []byte
	for range 2 {
		states = append(states, u16s(0, 0, 0, 0, 1, 2, 3)...)
	}
	var entryTable []byte
	for _, e := range entries {
		entryTable = append(entryTable, u16s(0, e[0], e[1])...)
	}
	data := u32s(classes, 20, uint32(20+len(classTable)), uint32(20+len(classTable)+len(states)),
		uint32(20+len(classTable)+len(states)+len(entryTable)))
	data = append(data, classTable...)
	data = append(data, states...)
	data = append(data, entryTable...)
	data = append(data, u16s(values...)...)
	return kerxSubtable{coverage: 1, data: data}
}

// TestKerxFormat1History: a subtable of format 1 of kerx pushes a, pushes b
// and applies a list of three odd values, and applies a list of ten values
// at c, with the flag Reset and without a push. HarfBuzz keeps a history of
// eight glyphs, which each list walks again from the last glyph, and a slot
// that no glyph filled holds the first glyph. A list ends with 0xFFFF, not
// with an odd value, and HarfBuzz ignores the flag Reset. The glyphs that
// a list changes are unsafe to break.
func TestKerxFormat1History(t *testing.T) {
	kerx := kerxTable(kerxFormat1([4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {kerxReset, 4}},
		51, 41, -31, kerxEnd,
		-20, 15, 7, 3, 5, 9, 11, 13, 17, 19, kerxEnd))
	testFlags(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, true,
		"ab", "aab", "abab", "abc", "bc", "c", "xab", "abcabc", "aaaaaaaaab", "aaaaaaaaabc", "xxxxab")
}

// TestKerxFormat1Operations: a subtable of format 1 of kerx applies at b a
// list of 100,000 values 0 without an end, then a subtable of format 0
// kerns a, a. Each action takes one operation, and HarfBuzz runs out of
// operations in the first subtable. The table then stops, and the second
// subtable does not apply.
func TestKerxFormat1Operations(t *testing.T) {
	values := make([]int, 100000)
	kerx := kerxTable(kerxFormat1([4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {0, kerxEnd}}, values...),
		kerxPairs(robotoA, robotoA, -300))
	data := robotoWith(t, map[string][]byte{"kerx": kerx})
	testCase(t, data, true, "aab")
	testCase(t, data, false, "aa", "aaa")
}

// TestKernFormat1Offset: a subtable of format 1 of a table kern of Apple
// pushes a, and pushes b with a list of actions at the byte offset 0x2000.
// The list gives -40 to b and -30 to a, and its odd value ends it. In
// kern, the bit 0x2000 is a bit of the offset, and HarfBuzz does not empty
// the stack for it.
func TestKernFormat1Offset(t *testing.T) {
	const values = 0x2000
	data := u16s(7, 10, 18, 32, values)
	data = append(data, u16s(robotoA, 3)...)
	data = append(data, 4, 5, 6, 0)
	for range 2 {
		data = append(data, 0, 0, 0, 0, 1, 2, 0)
	}
	data = append(data, u16s(18, 0, 18, kerxPush, 18, kerxPush|values)...)
	data = append(data, make([]byte, values-len(data))...)
	data = append(data, u16s(-40, -29)...)
	kern := u32s(0x00010000, 1, uint32(8+len(data)), 0x00010000)
	kern = append(kern, data...)
	testCase(t, robotoWith(t, map[string][]byte{"kern": kern}), false, "ab", "aab", "abab")
}

// TestKerxFormat1Bounds: two subtables of format 1 of kerx. In the first,
// the list of b has no end, and HarfBuzz reads it up to the end of the
// subtable. The header of the second subtable gives a length without its
// values, and HarfBuzz reads the values of the last subtable up to the end
// of the table.
func TestKerxFormat1Bounds(t *testing.T) {
	entries := [4][2]int{{0, kerxEnd}, {kerxPush, kerxEnd}, {kerxPush, 0}, {0, kerxEnd}}
	last := kerxFormat1(entries, 5, kerxEnd)
	last.length = 12 + len(last.data) - 4
	kerx := kerxTable(kerxFormat1(entries, 10, 20), last)
	testFlags(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, true, "ab", "aab", "abab", "xab")
}

// stateMachine is an extended state table of morx or kerx. lookup is its
// class table. Each state is a row of nClasses entry indexes, and each
// entry is its new state, its flags and its data.
type stateMachine struct {
	lookup   []byte
	nClasses int
	states   [][]int
	entries  [][]int
}

// bytes gives the state table, with one 32-bit field after its header for
// each tail, and the tails after the table. A field is fields[i] OR the
// offset of tails[i] from the start of the state table.
func (m stateMachine) bytes(fields []uint32, tails ...[]byte) []byte {
	var states, entries []byte
	for _, row := range m.states {
		states = append(states, u16s(row...)...)
	}
	for _, e := range m.entries {
		entries = append(entries, u16s(e...)...)
	}
	header := 16 + 4*len(tails)
	offset := header + len(m.lookup)
	out := u32s(uint32(m.nClasses), uint32(header), uint32(offset), uint32(offset+len(states)))
	offset += len(states) + len(entries)
	for i, tail := range tails {
		out = append(out, u32s(fields[i]|uint32(offset))...)
		offset += len(tail)
	}
	out = append(out, m.lookup...)
	out = append(out, states...)
	out = append(out, entries...)
	for _, tail := range tails {
		out = append(out, tail...)
	}
	return out
}

// lookup8 builds a lookup table of format 8, which gives the values to the
// glyphs from first on.
func lookup8(first int, values ...int) []byte {
	return append(u16s(8, first, len(values)), u16s(values...)...)
}

// morxSubtable is a subtable of a chain of morx: its coverage, with the
// type in the low byte, and its data after its header of 12 bytes.
type morxSubtable struct {
	coverage uint32
	data     []byte
}

// morxChain builds a chain of morx with the subtables, whose default flags
// and subtable flags are 1.
func morxChain(subtables ...morxSubtable) []byte {
	var data []byte
	for _, st := range subtables {
		data = append(data, u32s(uint32(12+len(st.data)), st.coverage, 1)...)
		data = append(data, st.data...)
	}
	out := u32s(1, uint32(16+len(data)), 0, uint32(len(subtables)))
	return append(out, data...)
}

// morxTable builds a table morx of version 2 with the chains.
func morxTable(chains ...[]byte) []byte {
	out := u16s(2, 0)
	out = append(out, u32s(uint32(len(chains)))...)
	for _, c := range chains {
		out = append(out, c...)
	}
	return out
}

// The coverages of the subtables of morx and kerx that the tests use.
const (
	morxNoncontextual = 0x20000004
	morxInsertion     = 0x20000005
	kerxFormat4       = 4
	kerxCrossStream   = 0x40000000
	kerxBackwards     = 0x10000000
)

// The flags of the entries of the state machines.
const (
	kerxMark          = 0x8000 // the flag Mark of format 4 of kerx
	insertCurrentOne  = 1 << 5 // insert one glyph at the current glyph
	insertNone        = 0xFFFF // the index of no insertion
	kerx4Coordinates  = 2 << 30
	kerx4NoActionData = 0xFFFF
)

// TestAATInitialGlyphs: subtables with a state machine, whose class table
// gives the class 4 to a, and from whose start state the class 4 starts no
// action. HarfBuzz applies such a subtable only when the buffer holds a
// glyph whose class can start an action from the start state, and never
// takes a glyph outside the class table for one:
//   - In a subtable of format 1 of kerx, x is outside the class table. It
//     pushes itself and kerns itself by -100.
//   - In a subtable of format 4 of kerx, x is outside the class table. It
//     marks itself and attaches to the glyph that it marked before.
//   - A subtable of morx inserts b at the end of the text.
//   - In a chain of morx, a noncontextual subtable deletes a. Then an
//     insertion subtable, whose class table holds only c, inserts b after a
//     deleted glyph. HarfBuzz takes the deleted glyph for a glyph of the
//     subtable.
func TestAATInitialGlyphs(t *testing.T) {
	kerx1 := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{0, 1, 0, 0, 0}, {0, 1, 0, 0, 0}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxPush, 0}},
	}
	kerx := kerxTable(kerxSubtable{coverage: 1, data: kerx1.bytes([]uint32{0}, u16s(-100, kerxEnd))})
	testFlags(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, true, "xa", "ax", "xax", "x", "a")

	kerx4 := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{0, 1, 0, 0, 0}, {0, 1, 0, 0, 0}},
		entries:  [][]int{{0, 0, kerx4NoActionData}, {0, kerxMark, 0}},
	}
	kerx = kerxTable(kerxSubtable{coverage: kerxFormat4, data: kerx4.bytes([]uint32{kerx4Coordinates}, u16s(50, 60, 10, 20))})
	testFlags(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, true, "xxa", "axx", "xax", "xx")

	insertion := stateMachine{
		lookup:   lookup8(robotoA, 4),
		nClasses: 5,
		states:   [][]int{{1, 0, 0, 0, 0}, {1, 0, 0, 0, 0}},
		entries:  [][]int{{0, 0, insertNone, insertNone}, {0, insertCurrentOne, 0, insertNone}},
	}
	morx := morxTable(morxChain(morxSubtable{morxInsertion, insertion.bytes([]uint32{0}, u16s(robotoB))}))
	testFlags(t, robotoWith(t, map[string][]byte{"morx": morx}), false, true, "a", "aa", "xa", "x")

	insertion = stateMachine{
		lookup:   lookup8(robotoC, 4),
		nClasses: 5,
		states:   [][]int{{0, 0, 1, 0, 0}, {0, 2, 1, 0, 0}},
		entries:  [][]int{{0, 0, insertNone, insertNone}, {1, 0, insertNone, insertNone}, {0, insertCurrentOne, 0, insertNone}},
	}
	morx = morxTable(morxChain(
		morxSubtable{morxNoncontextual, lookup8(robotoA, 0xFFFF)},
		morxSubtable{morxInsertion, insertion.bytes([]uint32{0}, u16s(robotoB))},
	))
	testFlags(t, robotoWith(t, map[string][]byte{"morx": morx}), false, true, "ax", "axx", "xax", "aaxa", "x", "cx")
}

// kerxMarks builds a table kerx with two subtables. The first is of format
// 1 and cross-stream: a pushes itself and applies the list of values. The
// second is of format 4, with the coverage: a marks itself, and b attaches
// to the marked glyph with the coordinates of the anchors.
func kerxMarks(coverage uint32, values, anchors []int) []byte {
	cross := stateMachine{
		lookup:   lookup8(robotoA, 4, 5),
		nClasses: 6,
		states:   [][]int{{0, 0, 0, 0, 1, 0}, {0, 0, 0, 0, 1, 0}},
		entries:  [][]int{{0, 0, kerxEnd}, {0, kerxPush, 0}},
	}
	marks := stateMachine{
		lookup:   lookup8(robotoA, 4, 5),
		nClasses: 6,
		states:   [][]int{{0, 0, 0, 0, 1, 2}, {0, 0, 0, 0, 1, 2}},
		entries:  [][]int{{0, 0, kerx4NoActionData}, {0, kerxMark, kerx4NoActionData}, {0, 0, 0}},
	}
	return kerxTable(
		kerxSubtable{coverage: kerxCrossStream | 1, data: cross.bytes([]uint32{0}, u16s(append(values, kerxEnd)...))},
		kerxSubtable{coverage: coverage, data: marks.bytes([]uint32{kerx4Coordinates}, u16s(anchors...))},
	)
}

// TestKerxFormat4CrossOffset: a subtable of format 1 of kerx moves each a
// by 100 across the line, and a subtable of format 4 attaches b to the
// last a. HarfBuzz adds to the offset of b across the line the offsets of
// the a and of the glyphs that the a are attached to. It also does so when
// the subtable of format 4 goes backwards, which reverses the buffer.
func TestKerxFormat4CrossOffset(t *testing.T) {
	for _, coverage := range []uint32{kerxFormat4, kerxFormat4 | kerxBackwards} {
		kerx := kerxMarks(coverage, []int{100}, []int{50, 60, 10, 20})
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab", "aab", "abb", "ba", "bab", "abab", "aaba")
	}
}

// TestKerxFormat4Clamp: a subtable of format 1 of kerx moves a across the
// line by 65,537 times 32,767, and a subtable of format 4 attaches b to a
// with anchors 65,535 apart. HarfBuzz clamps the offset of b to the range
// of a position. The glyphs x give the buffer the operations that the
// actions take.
func TestKerxFormat4Clamp(t *testing.T) {
	values := make([]int, 65537)
	for i := range values {
		values[i] = 32767
	}
	kerx := kerxMarks(kerxFormat4, values, []int{0, 32767, 0, -32768})
	testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab"+strings.Repeat("x", 38))
}

// kernFormat1 builds the data of a subtable of format 1 of kern, after its
// header. Its class table gives the classes 4, 5 and 6 to a, b and c, and
// its two states give them the entries 1, 2 and 3. Each entry stays in the
// state 0 and has the flags of its index, which hold the byte offset of a
// list of values from the start of the state table. The values start at
// the byte offset valueTable, which is 48 or more.
func kernFormat1(flags [4]int, valueTable int, values ...int) []byte {
	data := u16s(7, 10, 18, 32, valueTable)
	data = append(data, u16s(robotoA, 3)...)
	data = append(data, 4, 5, 6, 0)
	for range 2 {
		data = append(data, 0, 0, 0, 0, 1, 2, 3)
	}
	for _, f := range flags {
		data = append(data, u16s(18, f)...)
	}
	data = append(data, make([]byte, valueTable-len(data))...)
	return append(data, u16s(values...)...)
}

// kernTable builds a table kern of Apple of version 1 with subtables of
// format 1, each the data after its header.
func kernTable(subtables ...[]byte) []byte {
	out := u32s(0x00010000, uint32(len(subtables)))
	for _, data := range subtables {
		out = append(out, u32s(uint32(8+len(data)), 0x00010000)...)
		out = append(out, data...)
	}
	return out
}

// TestKernFormat1ValueTable: subtables of format 1 of kern, where a pushes
// itself:
//   - b has a list of values at the byte offset 2, before the value table,
//     and c has the list -40, -29 at the start of the value table.
//     HarfBuzz applies the list of b with an index out of range, and
//     empties the stack.
//   - b and c push themselves too, and the list of c is 11, 20, 30. The
//     first value is odd and ends the list, but HarfBuzz requires a value
//     for each glyph of the stack.
//   - The same subtable with the list 20, 30, which ends with the
//     subtable, before a second subtable. HarfBuzz requires the values for
//     the stack in the subtable.
func TestKernFormat1ValueTable(t *testing.T) {
	kern := kernTable(kernFormat1([4]int{0, kerxPush, 2, 48}, 48, -40, -29))
	testFlags(t, robotoWith(t, map[string][]byte{"kern": kern}), false, true, "abc", "ac", "abac", "aabc", "acbc")

	push := [4]int{0, kerxPush, kerxPush, kerxPush | 48}
	kern = kernTable(kernFormat1(push, 48, 11, 20, 30))
	testFlags(t, robotoWith(t, map[string][]byte{"kern": kern}), false, true, "abc", "ac", "c", "abcc", "aaaaac")

	kern = kernTable(kernFormat1(push, 48, 20, 30), kernFormat1([4]int{}, 48))
	testFlags(t, robotoWith(t, map[string][]byte{"kern": kern}), false, true, "abc", "ac", "c", "abcc")
}

// kernPairs builds a subtable of format 0 of a table kern of Microsoft,
// with the length and the pairs, each a left glyph, a right glyph and a
// value.
func kernPairs(length int, pairs ...int) []byte {
	n := len(pairs) / 3
	out := u16s(0, length, 1, n, 6*n, 0, 0)
	return append(out, u16s(pairs...)...)
}

// kernOT builds a table kern of Microsoft with the subtables.
func kernOT(subtables ...[]byte) []byte {
	out := u16s(0, len(subtables))
	for _, st := range subtables {
		out = append(out, st...)
	}
	return out
}

// withLength gives the chain of morx or the subtable with the length at
// the byte offset at in place of its length.
func withLength(data []byte, at, length int) []byte {
	out := slices.Clone(data)
	copy(out[at:], u32s(uint32(length)))
	return out
}

// withCoverage gives the chain of morx with a glyph coverage of version 3
// after its subtables, which holds the offsets and room for one bitfield,
// and with its length.
func withCoverage(chain []byte, offsets ...uint32) []byte {
	out := append(slices.Clone(chain), u32s(offsets...)...)
	out = append(out, make([]byte, 4096)...)
	return withLength(out, 4, len(out))
}

// TestAATTableErrors: tables kern, kerx and morx with an error, or with a
// subtable that HarfBuzz does not apply. A subtable kerns a, b by -300, or
// replaces a with b or c with x:
//   - In kern, a subtable of format 0 has more pairs than its length holds.
//     HarfBuzz refuses the table, except when the subtable is the last
//     one, whose length it ignores, as for some versions of Calibri. It
//     refuses a table whose last subtable is longer than the table, and a
//     subtable shorter than its header. It does not apply a subtable of an
//     unknown format, and applies the subtables after it.
//   - In kerx, HarfBuzz refuses the whole table when a subtable after the
//     first one is longer than the table, when a subtable is shorter than
//     its header, and when a subtable that is not the last has more pairs
//     than its length holds. It does not apply a subtable of an unknown
//     format, and applies the subtables after it. In version 3, it refuses
//     the table when the glyph coverage of a subtable is not in the table.
//   - In morx, HarfBuzz refuses the whole table when a chain after the
//     first one is longer than the table, when a chain is shorter than its
//     header, and when a subtable is shorter than its header. It does not
//     apply a subtable of an unknown type, and finds the next chain at the
//     length of a chain, after the glyph coverage of version 3. It refuses
//     the table when the glyph coverage of a subtable is not in the table.
func TestAATTableErrors(t *testing.T) {
	for _, kern := range [][]byte{
		kernOT(kernPairs(14, robotoA, robotoB, -300)),
		kernOT(kernPairs(20+100, robotoA, robotoB, -300)),
		kernOT(u16s(0, 14, 1, 2, 12, 1, 0), kernPairs(20, robotoA, robotoB, -300)),
		kernOT(kernPairs(0), kernPairs(20, robotoA, robotoB, -300)),
		kernOT(u16s(0, 8, 5<<8|1, 0), kernPairs(20, robotoA, robotoB, -300)),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"kern": kern}), false, "ab")
	}

	// coverage3 gives the table kerx in version 3, with the offsets of its
	// glyph coverage and room for one bitfield.
	pairs := kerxPairs(robotoA, robotoB, -300)
	coverage3 := func(kerx []byte, offsets ...uint32) []byte {
		out := append(slices.Clone(kerx), u32s(offsets...)...)
		out = append(out, make([]byte, 4096)...)
		out[1] = 3
		return out
	}
	for _, kerx := range [][]byte{
		kerxTable(pairs, kerxSubtable{data: u32s(0, 0, 0, 0), length: 1000}),
		kerxTable(kerxSubtable{coverage: 3, data: u32s(0)}, pairs),
		kerxTable(kerxSubtable{data: u32s(0, 0, 0, 0), length: 8}, pairs),
		kerxTable(kerxSubtable{data: u32s(1, 6, 0, 0)}, pairs),
		coverage3(kerxTable(kerxPairs(), pairs), 0, 0xFFFFFFFF),
		coverage3(kerxTable(kerxPairs(), pairs), 0, 8),
		coverage3(kerxTable(kerxPairs(), pairs), 0, 4096),
		coverage3(kerxTable(kerxPairs(), pairs), 0, 100000),
	} {
		testCase(t, robotoWith(t, map[string][]byte{"kerx": kerx}), false, "ab")
	}

	ab := morxChain(morxSubtable{morxNoncontextual, lookup8(robotoA, robotoB)})
	cx := morxChain(morxSubtable{morxNoncontextual, lookup8(robotoC, robotoX)})
	unknown := morxSubtable{0x20000003, u32s(0)}
	padded := withLength(append(slices.Clone(ab), 0, 0, 0, 0), 4, len(ab)+4)
	twoSubtables := morxChain(morxSubtable{morxNoncontextual, lookup8(robotoA, robotoB)},
		morxSubtable{morxNoncontextual, lookup8(robotoC, robotoX)})
	for _, test := range []struct {
		version int
		chains  [][]byte
	}{
		{2, [][]byte{ab, withLength(cx, 4, 1000)}},
		{2, [][]byte{cx, withLength(ab, 4, 8)}},
		{2, [][]byte{morxChain(unknown, morxSubtable{morxNoncontextual, lookup8(robotoA, robotoB)})}},
		{2, [][]byte{padded, cx}},
		{2, [][]byte{withLength(twoSubtables, 16, 8)}},
		{3, [][]byte{withCoverage(ab, 0), withCoverage(cx, 0xFFFFFFFF)}},
		{3, [][]byte{withCoverage(ab, 0), withCoverage(cx, 4)}},
		{3, [][]byte{withCoverage(ab, 0), withCoverage(cx, 4096)}},
		{3, [][]byte{withCoverage(ab, 0), withCoverage(cx, 100000)}},
	} {
		morx := morxTable(test.chains...)
		morx[1] = byte(test.version)
		testCase(t, robotoWith(t, map[string][]byte{"morx": morx}), false, "ac", "ab")
	}
}
