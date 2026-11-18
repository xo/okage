package krilla

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math/big"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/siphasher"
	"github.com/xo/okage/internal/subsetter"
)

// The tests of this file compare the subsets of internal/subsetter with the
// subsets of subsetter, which the oracle gives with the question "subset"
// (item 5 of "Testing" under "The PDF writer" in docs/PLAN.md). The tag and
// the bytes of each subset must be the same.

// subsetCase is one subset to compare: a font, the index of the font in its
// file, the glyphs in the order of their first use, and the coordinates of
// the variations. Coords is nil to call Subset, and not nil to call
// SubsetWithVariations.
type subsetCase struct {
	name   string
	font   string
	index  uint32
	glyphs []uint16
	coords *[]coord
}

// subsetterFonts lists the TrueType fonts of upstream/subsetter/fonts, which
// tests/data/subsets.tests and tests/data/fonttools.tests of subsetter read.
var subsetterFonts = []string{
	"ClickerScript-Regular.ttf",
	"DejaVuSansMono.ttf",
	"MPLUS1p-Regular.ttf",
	"NotoSans-Regular.ttf",
	"NotoSans-Regular_var.ttf",
	"Roboto-Regular.ttf",
	"TestTTC.ttc",
}

// TestSubsetterTrueType compares the subsets of the TrueType fonts of
// subsetter and of okage with the oracle: the glyph sets of the tests of
// subsetter, the glyph 0 only, every glyph, each composite glyph with its
// components, and random sets.
func TestSubsetterTrueType(t *testing.T) {
	needOracle(t)
	cases := subsetterTestCases(t)
	for _, font := range okageFonts(t) {
		cases = append(cases, fontCases(t, font)...)
	}
	compareSubsets(t, cases)
}

// TestSubsetterMalformed changes bytes of small TrueType fonts at random,
// and compares the result of internal/subsetter with the result of
// subsetter: the same tag and bytes, or an error of the same kind.
func TestSubsetterMalformed(t *testing.T) {
	needOracle(t)
	dir := t.TempDir()
	rng := rand.New(rand.NewPCG(13, 62))
	read := func(parts ...string) []byte {
		data, err := os.ReadFile(abs(t, parts...))
		if err != nil {
			t.Skip(err)
		}
		return data
	}
	smallSubset := func(data []byte, glyphs ...uint16) []byte {
		out, err := subsetter.Subset(data, 0, subsetter.NewGlyphRemapperFromGlyphs(glyphs))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	noto := read("upstream", "subsetter", "fonts", "NotoSans-Regular.ttf")
	seeds := [][]byte{
		read("upstream", "subsetter", "fonts", "TestTTC.ttc"),
		smallSubset(read("upstream", "subsetter", "fonts", "ClickerScript-Regular.ttf"), 5, 8, 10, 100, 101),
		// The glyphs 120 to 125 of Noto Sans are composite glyphs.
		smallSubset(noto, 36, 120, 121, 122, 123, 124, 125),
	}
	var cases []subsetCase
	for i := range 5000 {
		seed := seeds[rng.IntN(len(seeds))]
		data := slices.Clone(seed)
		for range 1 + rng.IntN(3) {
			if len(data) == 0 {
				break
			}
			// start and end are the bounds of a table of the directory of the
			// seed, or of the whole font.
			start, end := 0, len(data)
			if f, ok := openFont(seed, 0); ok && len(f.tables) > 0 {
				tags := slices.Sorted(maps.Keys(f.tables))
				tag := tags[rng.IntN(len(tags))]
				start = int(binary.BigEndian.Uint32(seedRecord(seed, tag)[8:]))
				end = start + len(f.tables[tag])
			}
			end = min(end, len(data))
			switch rng.IntN(5) {
			case 0:
				// A byte of the table directory.
				data[rng.IntN(min(len(data), 12+16*16))] = byte(rng.Uint32())
			case 1:
				data = data[:rng.IntN(len(data))]
			case 2:
				// A 16-bit value of a table: 0, 1, a small value or the
				// largest value.
				if at := start + rng.IntN(max(1, end-start)); at+2 <= len(data) {
					v := []uint16{0, 1, uint16(rng.IntN(64)), 0x7FFF, 0x8000, 0xFFFF}[rng.IntN(6)]
					binary.BigEndian.PutUint16(data[at:], v)
				}
			default:
				// A byte of a table.
				if start < end {
					data[start+rng.IntN(end-start)] = byte(rng.Uint32())
				}
			}
		}
		file := filepath.Join(dir, fmt.Sprintf("font%03d.ttf", i))
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		glyphs := make([]uint16, rng.IntN(8))
		for j := range glyphs {
			glyphs[j] = uint16(rng.IntN(10))
		}
		c := subsetCase{name: filepath.Base(file), font: file, glyphs: glyphs}
		if rng.IntN(3) == 0 {
			c.coords = &[]coord{}
		}
		cases = append(cases, c)
	}
	compareSubsets(t, cases)
}

// seedRecord returns the table record of a tag in the directory of the first
// font of a font file.
func seedRecord(data []byte, tag string) []byte {
	at := 0
	if string(data[:4]) == "ttcf" {
		at = int(binary.BigEndian.Uint32(data[12:]))
	}
	n := int(binary.BigEndian.Uint16(data[at+4:]))
	for i := range n {
		r := at + 12 + 16*i
		if string(data[r:r+4]) == tag {
			return data[r : r+16]
		}
	}
	return nil
}

// subsetterTestCases returns the cases of tests/data/subsets.tests and
// tests/data/fonttools.tests of subsetter for its TrueType fonts, and the
// cases of fontCases for each of these fonts.
func subsetterTestCases(t *testing.T) []subsetCase {
	t.Helper()
	dir := abs(t, "upstream", "subsetter")
	if _, err := os.Stat(filepath.Join(dir, "fonts")); err != nil {
		t.Skipf("the checkout of subsetter is missing: %v", err)
	}
	var cases []subsetCase
	for _, name := range []string{"subsets.tests", "fonttools.tests"} {
		f, err := os.Open(filepath.Join(dir, "tests", "data", name))
		if err != nil {
			t.Fatal(err)
		}
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			parts := strings.Split(line, ";")
			if !slices.Contains(subsetterFonts, parts[0]) {
				continue
			}
			font := filepath.Join(dir, "fonts", parts[0])
			c := subsetCase{
				name:   fmt.Sprintf("%s:%d", name, n),
				font:   font,
				glyphs: parseGids(parts[1], numGlyphs(t, font, 0)),
			}
			if len(parts) > 2 {
				coords := parseVariations(parts[2])
				c.coords = &coords
			}
			cases = append(cases, c)
		}
		if err := sc.Err(); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range subsetterFonts {
		cases = append(cases, fontCases(t, filepath.Join(dir, "fonts", name))...)
	}
	return cases
}

// okageFonts returns the TrueType fonts of okage that the tests of other
// packages read, which hold a glyf table.
func okageFonts(t *testing.T) []string {
	t.Helper()
	var fonts []string
	for _, dir := range []string{"svg", "webfont", "typesetting"} {
		err := filepath.WalkDir(abs(t, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".ttf", ".ttc", ".otf":
				fonts = append(fonts, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return fonts
}

// fontCases returns the cases of one font file: for each font of the file,
// the glyph 0 only, every glyph, each composite glyph with and without its
// components, random sets, and the default coordinates of the variations.
// A font that has no glyf table gives no case.
func fontCases(t *testing.T, file string) []subsetCase {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(uint64(len(data)), 62))
	var cases []subsetCase
	for index := range uint32(fontCount(data)) {
		f, ok := openFont(data, index)
		if !ok || f.table("glyf") == nil {
			continue
		}
		n := f.numGlyphs()
		name := fmt.Sprintf("%s#%d", filepath.Base(file), index)
		add := func(what string, glyphs []uint16, coords *[]coord) {
			cases = append(cases, subsetCase{name: name + " " + what, font: file, index: index, glyphs: glyphs, coords: coords})
		}
		add("glyph 0", []uint16{}, nil)
		add("all", parseGids("*", n), nil)
		add("all reversed", reversed(parseGids("*", n)), nil)
		composites := f.composites()
		for i, g := range composites {
			if i >= 20 {
				break
			}
			add(fmt.Sprintf("composite %d", g), []uint16{g}, nil)
			add(fmt.Sprintf("composite %d and components", g), append(f.components(g), g), nil)
		}
		if len(composites) > 0 {
			add("all composites", composites, nil)
		}
		for i := range 8 {
			size := 1 + rng.IntN(min(int(n)+1, 1+i*i*8))
			glyphs := make([]uint16, size)
			for j := range glyphs {
				glyphs[j] = uint16(rng.IntN(int(n)))
			}
			add(fmt.Sprintf("random %d", i), glyphs, nil)
		}
		add("out of the font", []uint16{1, n, n + 7}, nil)
		add("empty coordinates", []uint16{1, 2, 3}, &[]coord{})
		if axes := f.axes(); len(axes) > 0 {
			var def []coord
			for _, a := range axes {
				def = append(def, coord{a.tag, a.def})
			}
			add("default coordinates", []uint16{1, 2, 3}, &def)
			add("unknown axis", []uint16{1, 2, 3}, &[]coord{{"zzzz", 300}})
		}
	}
	return cases
}

// compareSubsets asks the oracle for the subset of each case, and compares
// the tag and the bytes with internal/subsetter. A case for which
// internal/subsetter returns an error that a unit of phase 13 that is not
// ported yet gives is skipped. An instance of a variable font at a location
// other than the default can differ by 1 unit, and is compared as
// TestSubsetterVariable compares it.
func compareSubsets(t *testing.T, cases []subsetCase) {
	t.Helper()
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = subsetQuestion{Subset: subsetSpec{Font: c.font, Index: c.index, Glyphs: c.glyphs, Coords: c.coords}}
	}
	answers := ask(t, questions...)
	compared, skipped, failed := 0, 0, 0
	for i, c := range cases {
		data, err := os.ReadFile(c.font)
		if err != nil {
			t.Fatal(err)
		}
		tag, got, err := goSubset(data, c.index, c.glyphs, c.coords)
		if notPorted(err) {
			skipped++
			continue
		}
		if c.coords != nil && err == nil && answers[i].Err == "" && !vfIsDefault(t, c) &&
			len(answers[i].Data) >= 6 && string(answers[i].Data[:6]) == tag {
			d, err := vfCompare(got, answers[i].Data[6:])
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
				continue
			}
			for _, msg := range d.messages {
				t.Errorf("%s: %s", c.name, msg)
			}
			compared++
			continue
		}
		if !sameResult(t, c.name, answers[i], tag, got, err) {
			continue
		}
		compared++
		if err != nil {
			failed++
		}
	}
	t.Logf("%d results are the same as the results of subsetter, %d of them errors, and %d need a unit that is not ported",
		compared, failed, skipped)
	if compared == 0 {
		t.Error("no subset was compared")
	}
}

// sameResult reports whether the result of internal/subsetter is the answer
// of the oracle, and fails the test when it is not.
func sameResult(t *testing.T, name string, a answer, tag string, got []byte, err error) bool {
	t.Helper()
	switch {
	case a.Err != "" && err == nil:
		t.Errorf("%s: subsetter gives the error %q, and okage gives a subset", name, a.Err)
		return false
	case a.Err == "" && err != nil:
		t.Errorf("%s: okage gives the error %v, and subsetter gives a subset", name, err)
		return false
	case a.Err != "":
		want := strings.TrimPrefix(a.Err, "subsetting: ")
		if strings.HasPrefix(want, "panic: ") {
			// okage returns an error where subsetter panics.
			return true
		}
		if !strings.HasPrefix(err.Error(), "subsetting font: "+want) {
			t.Errorf("%s: okage gives the error %q, and subsetter gives %q", name, err, a.Err)
			return false
		}
		return true
	}
	if len(a.Data) < 6 || string(a.Data[:6]) != tag {
		t.Errorf("%s: the tag is %s, and subsetter gives %q", name, tag, a.Data[:min(6, len(a.Data))])
		return false
	}
	if want := a.Data[6:]; !slices.Equal(got, want) {
		at := 0
		for at < min(len(got), len(want)) && got[at] == want[at] {
			at++
		}
		t.Errorf("%s: the subset has %d bytes, and the subset of subsetter has %d bytes. They differ first at the byte %d",
			name, len(got), len(want), at)
		return false
	}
	return true
}

// goSubset makes the subset of a font with internal/subsetter, as the oracle
// does with subsetter, and returns the tag that krilla gives to it.
func goSubset(data []byte, index uint32, glyphs []uint16, coords *[]coord) (string, []byte, error) {
	m := subsetter.NewGlyphRemapper()
	for _, g := range glyphs {
		m.Remap(g)
	}
	tag := subsetTag(m)
	if coords == nil {
		out, err := subsetter.Subset(data, index, m)
		return tag, out, err
	}
	vars := make([]subsetter.Variation, 0, len(*coords))
	for _, c := range *coords {
		t, ok := subsetter.ParseTag(c.Tag)
		if !ok {
			return tag, nil, fmt.Errorf("the tag %q does not have four bytes", c.Tag)
		}
		vars = append(vars, subsetter.Variation{Tag: t, Value: c.Value})
	}
	out, err := subsetter.SubsetWithVariations(data, index, vars, m)
	return tag, out, err
}

// notPorted reports whether err comes from a seam of internal/subsetter for
// a unit of phase 13 that is not ported yet.
func notPorted(err error) bool {
	return errors.Is(err, okage.ErrSubsetUnimplemented) && strings.Contains(err.Error(), "is not ported yet")
}

// stableHasher is StableHasher of stable_hash128 of krilla src/util.rs: a
// SipHasher13 that writes a usize as a u64.
type stableHasher struct {
	siphasher.SipHasher13
}

func (h *stableHasher) WriteUsize(i uint) {
	h.WriteU64(uint64(i))
}

// subsetTag is subset_tag of krilla src/text/cid.rs. It hashes the remapper
// into the tag of six capital letters of the name of the font.
func subsetTag(m *subsetter.GlyphRemapper) string {
	h := &stableHasher{siphasher.NewSipHasher13()}
	m.Hash(h)
	hi, lo := h.Finish128().AsU128()
	hash := new(big.Int).Lsh(new(big.Int).SetUint64(hi), 64)
	hash.Or(hash, new(big.Int).SetUint64(lo))
	base := big.NewInt(26)
	var letters [6]byte
	for i := range letters {
		var rem big.Int
		hash.DivMod(hash, base, &rem)
		letters[i] = 'A' + byte(rem.Int64())
	}
	return string(letters[:])
}

// parseGids is parse_gids of tests/src/main.rs of subsetter.
func parseGids(gids string, maxGlyph uint16) []uint16 {
	if gids == "*" {
		out := make([]uint16, maxGlyph)
		for i := range out {
			out[i] = uint16(i)
		}
		return out
	}
	var out []uint16
	for el := range strings.SplitSeq(gids, ",") {
		if el == "" {
			continue
		}
		if first, second, ok := strings.Cut(el, "-"); ok {
			a, _ := strconv.ParseUint(first, 10, 16)
			b, _ := strconv.ParseUint(second, 10, 16)
			for g := a; g <= b; g++ {
				out = append(out, uint16(g))
			}
			continue
		}
		g, _ := strconv.ParseUint(el, 10, 16)
		out = append(out, uint16(g))
	}
	return out
}

// parseVariations is parse_variations of tests/src/main.rs of subsetter.
func parseVariations(input string) []coord {
	out := []coord{}
	for pair := range strings.SplitSeq(input, ",") {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 32)
		if err != nil {
			continue
		}
		out = append(out, coord{strings.TrimSpace(name), float32(v)})
	}
	return out
}

func reversed(s []uint16) []uint16 {
	s = slices.Clone(s)
	slices.Reverse(s)
	return s
}

// The functions below read the few tables of a font that the cases need.

// fontCount returns the number of fonts in a font file.
func fontCount(data []byte) int {
	if len(data) >= 12 && string(data[:4]) == "ttcf" {
		return int(binary.BigEndian.Uint32(data[8:]))
	}
	return 1
}

// sfnt is a font of a font file, with its table directory.
type sfnt struct {
	data   []byte
	tables map[string][]byte
}

// openFont reads the table directory of a font of a font file.
func openFont(data []byte, index uint32) (sfnt, bool) {
	at := 0
	if fontCount(data) > 1 || (len(data) >= 4 && string(data[:4]) == "ttcf") {
		p := 12 + 4*int(index)
		if p+4 > len(data) {
			return sfnt{}, false
		}
		at = int(binary.BigEndian.Uint32(data[p:]))
	}
	if at+12 > len(data) {
		return sfnt{}, false
	}
	n := int(binary.BigEndian.Uint16(data[at+4:]))
	f := sfnt{data: data, tables: map[string][]byte{}}
	for i := range n {
		r := at + 12 + 16*i
		if r+16 > len(data) {
			return sfnt{}, false
		}
		off := int(binary.BigEndian.Uint32(data[r+8:]))
		size := int(binary.BigEndian.Uint32(data[r+12:]))
		if off+size <= len(data) {
			f.tables[string(data[r:r+4])] = data[off : off+size]
		}
	}
	return f, true
}

func (f sfnt) table(tag string) []byte {
	return f.tables[tag]
}

// numGlyphs returns the number of glyphs of the maxp table.
func (f sfnt) numGlyphs() uint16 {
	maxp := f.table("maxp")
	if len(maxp) < 6 {
		return 0
	}
	return binary.BigEndian.Uint16(maxp[4:])
}

// glyph returns the data of a glyph of the glyf table.
func (f sfnt) glyph(g uint16) []byte {
	head, loca, glyf := f.table("head"), f.table("loca"), f.table("glyf")
	if len(head) < 54 {
		return nil
	}
	var from, to int
	if binary.BigEndian.Uint16(head[50:]) != 0 {
		if 4*int(g)+8 > len(loca) {
			return nil
		}
		from = int(binary.BigEndian.Uint32(loca[4*int(g):]))
		to = int(binary.BigEndian.Uint32(loca[4*int(g)+4:]))
	} else {
		if 2*int(g)+4 > len(loca) {
			return nil
		}
		from = 2 * int(binary.BigEndian.Uint16(loca[2*int(g):]))
		to = 2 * int(binary.BigEndian.Uint16(loca[2*int(g)+2:]))
	}
	if from > to || to > len(glyf) {
		return nil
	}
	return glyf[from:to]
}

// composites returns the composite glyphs of the font.
func (f sfnt) composites() []uint16 {
	var out []uint16
	for g := range f.numGlyphs() {
		if d := f.glyph(g); len(d) >= 2 && int16(binary.BigEndian.Uint16(d)) < 0 {
			out = append(out, g)
		}
	}
	return out
}

// components returns the components of a composite glyph, without the
// components of the components.
func (f sfnt) components(g uint16) []uint16 {
	d := f.glyph(g)
	var out []uint16
	at := 10
	for at+4 <= len(d) {
		flags := binary.BigEndian.Uint16(d[at:])
		out = append(out, binary.BigEndian.Uint16(d[at+2:]))
		at += 4
		if flags&1 != 0 {
			at += 4
		} else {
			at += 2
		}
		switch {
		case flags&0x08 != 0:
			at += 2
		case flags&0x40 != 0:
			at += 4
		case flags&0x80 != 0:
			at += 8
		}
		if flags&0x20 == 0 {
			break
		}
	}
	return out
}

// axis is an axis of the fvar table.
type axis struct {
	tag string
	def float32
}

// axes returns the axes of the fvar table.
func (f sfnt) axes() []axis {
	fvar := f.table("fvar")
	if len(fvar) < 16 {
		return nil
	}
	off := int(binary.BigEndian.Uint16(fvar[4:]))
	n := int(binary.BigEndian.Uint16(fvar[8:]))
	size := int(binary.BigEndian.Uint16(fvar[10:]))
	var out []axis
	for i := range n {
		r := off + i*size
		if r+20 > len(fvar) {
			break
		}
		def := float32(int32(binary.BigEndian.Uint32(fvar[r+8:]))) / 65536
		out = append(out, axis{string(fvar[r : r+4]), def})
	}
	return out
}

// numGlyphs returns the number of glyphs of a font of a font file.
func numGlyphs(t *testing.T, file string, index uint32) uint16 {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := openFont(data, index)
	if !ok {
		t.Fatalf("%s: cannot read the table directory", file)
	}
	return f.numGlyphs()
}
