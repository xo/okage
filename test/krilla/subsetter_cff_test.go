package krilla

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xo/okage/internal/subsetter"
	"github.com/xo/okage/typesetting/font/cff"
)

// The tests of this file compare the subsets of the CFF fonts of
// internal/subsetter with the subsets of subsetter, as the tests of
// subsetter_test.go do for the TrueType fonts (item 5 of "Testing" under
// "The PDF writer" in docs/PLAN.md). A CFF font is an OpenType font with a
// CFF table. The tests read the CFF fonts of upstream/subsetter/fonts and of
// the tests of okage, and CFF fonts with bytes changed at random.

// TestSubsetterCFF compares the subsets of the CFF fonts of subsetter and of
// okage with the oracle: the glyph sets of the tests of subsetter, the glyph
// 0 only, every glyph, the glyphs whose endchar has the arguments of seac,
// and random sets.
func TestSubsetterCFF(t *testing.T) {
	needOracle(t)
	dir := abs(t, "upstream", "subsetter")
	if _, err := os.Stat(filepath.Join(dir, "fonts")); err != nil {
		t.Skipf("the checkout of subsetter is missing: %v", err)
	}
	cases := cfTestCases(t, dir)
	entries, err := os.ReadDir(filepath.Join(dir, "fonts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		cases = append(cases, cfFontCases(t, filepath.Join(dir, "fonts", e.Name()))...)
	}
	n := len(cases)
	for _, font := range okageFonts(t) {
		cases = append(cases, cfFontCases(t, font)...)
	}
	if len(cases) == n {
		t.Error("the tests of okage have no CFF font")
	}
	compareSubsets(t, cases)
}

// TestSubsetterMalformedCFF changes bytes of small CFF fonts at random, and
// compares the result of internal/subsetter with the result of subsetter:
// the same tag and bytes, or an error of the same kind.
func TestSubsetterMalformedCFF(t *testing.T) {
	needOracle(t)
	dir := t.TempDir()
	rng := rand.New(rand.NewPCG(3, 62))
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
	fonts := func(name string) []string { return []string{"upstream", "subsetter", "fonts", name} }
	seeds := [][]byte{
		// SID-keyed fonts.
		read(fonts("EndcharInSubroutine.otf")...),
		read("webfont", "testdata", "cff.otf"),
		// CID-keyed fonts, one with a FontMatrix in its font DICT.
		read(fonts("NotoSansCJKsc-Bold-subset1.otf")...),
		read(fonts("NotoSansCJKsc-Regular_custom_font_matrix.otf")...),
		// A subset of subsetter is a CID-keyed font without subroutines.
		smallSubset(read(fonts("LatinModernRoman-Regular.otf")...), 5, 8, 10, 100, 101, 290, 292),
	}
	var cases []subsetCase
	for i := range 6000 {
		seed := seeds[rng.IntN(len(seeds))]
		data := cfMutate(rng, seed)
		n := 0
		if f, ok := openFont(seed, 0); ok {
			n = int(f.numGlyphs())
		}
		file := filepath.Join(dir, fmt.Sprintf("font%04d.otf", i))
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
		// Most glyphs are in the font.
		glyphs := make([]uint16, rng.IntN(8))
		for j := range glyphs {
			glyphs[j] = uint16(rng.IntN(n + n/8 + 1))
		}
		c := subsetCase{name: filepath.Base(file), font: file, glyphs: glyphs}
		if rng.IntN(4) == 0 {
			c.coords = &[]coord{}
		}
		cases = append(cases, c)
	}
	compareSubsets(t, cases)
}

// cfMutate returns a copy of a CFF font with one to three changes. Most
// changes are in the CFF table, and the others in another table or in the
// whole font.
func cfMutate(rng *rand.Rand, seed []byte) []byte {
	data := slices.Clone(seed)
	f, ok := openFont(seed, 0)
	if !ok {
		return data
	}
	tags := slices.Sorted(maps.Keys(f.tables))
	for range 1 + rng.IntN(3) {
		if len(data) == 0 {
			break
		}
		tag := "CFF "
		if rng.IntN(4) == 0 {
			tag = tags[rng.IntN(len(tags))]
		}
		start, end := 0, len(data)
		if r := seedRecord(seed, tag); r != nil {
			start = int(binary.BigEndian.Uint32(r[8:]))
			end = start + int(binary.BigEndian.Uint32(r[12:]))
		}
		end = min(end, len(data))
		if start >= end {
			continue
		}
		at := start + rng.IntN(end-start)
		switch rng.IntN(12) {
		case 0:
			data = data[:at]
		case 1:
			// A 16-bit value: 0, 1, a small value or the largest value.
			if at+2 <= len(data) {
				v := []uint16{0, 1, uint16(rng.IntN(64)), 0x7FFF, 0x8000, 0xFFFF}[rng.IntN(6)]
				binary.BigEndian.PutUint16(data[at:], v)
			}
		case 2:
			// A byte that starts a number or an operator of a DICT or a
			// charstring.
			data[at] = []byte{12, 14, 19, 28, 29, 30, 255, 10, 11}[rng.IntN(9)]
		case 3:
			// The nibbles of a real number.
			data[at] = byte(rng.IntN(16))<<4 | byte(rng.IntN(16))
		default:
			data[at] = byte(rng.Uint32())
		}
	}
	return data
}

// cfTestCases returns the cases of tests/data/subsets.tests,
// tests/data/fonttools.tests and tests/data/cff.tests of subsetter for its
// CFF fonts.
func cfTestCases(t *testing.T, dir string) []subsetCase {
	t.Helper()
	var cases []subsetCase
	for _, name := range []string{"subsets.tests", "fonttools.tests", "cff.tests"} {
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
			font := filepath.Join(dir, "fonts", parts[0])
			data, err := os.ReadFile(font)
			if err != nil {
				t.Fatal(err)
			}
			if fnt, ok := openFont(data, 0); !ok || fnt.table("CFF ") == nil {
				continue
			}
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
	if len(cases) == 0 {
		t.Error("the tests of subsetter have no line for a CFF font")
	}
	return cases
}

// cfFontCases returns the cases of one font file: for each font of the file
// with a CFF table, the glyph 0 only, every glyph, the glyphs whose endchar
// has the arguments of seac, random sets, and glyphs that are not in the
// font. A font without a CFF table gives no case.
func cfFontCases(t *testing.T, file string) []subsetCase {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(uint64(len(data)), 3))
	var cases []subsetCase
	for index := range uint32(fontCount(data)) {
		f, ok := openFont(data, index)
		if !ok || f.table("CFF ") == nil {
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
		seac := cfSeacGlyphs(t, file, f.table("CFF "))
		for i, g := range seac {
			if i >= 10 {
				break
			}
			add(fmt.Sprintf("seac %d", g), []uint16{g}, nil)
		}
		if len(seac) > 0 {
			add("all seac", seac, nil)
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
		add("unknown axis", []uint16{1, 2, 3}, &[]coord{{"wght", 700}})
	}
	return cases
}

// cfSeacGlyphs returns the glyphs of a CFF table whose charstring ends with
// an endchar that has four arguments or more, the arguments of seac.
// subsetter does not subset these glyphs. cfSeacGlyphs reads only the
// charstring of the glyph and not its subroutines, which is where the fonts
// of the tests put the seac.
func cfSeacGlyphs(t *testing.T, file string, table []byte) []uint16 {
	t.Helper()
	font, err := cff.Parse(table)
	if err != nil {
		t.Logf("%s: typesetting cannot read the CFF table: %v", file, err)
		return nil
	}
	var out []uint16
	for g, cs := range font.Charstrings {
		if cfHasSeac(cs) {
			out = append(out, uint16(g))
		}
	}
	return out
}

// cfHasSeac reports whether a charstring has an endchar with four arguments
// or more before any call of a subroutine.
func cfHasSeac(cs []byte) bool {
	args, hints := 0, 0
	for i := 0; i < len(cs); {
		b := cs[i]
		switch {
		case b == 28:
			args++
			i += 3
			continue
		case b >= 32 && b <= 246:
			args++
			i++
			continue
		case b >= 247 && b <= 254:
			args++
			i += 2
			continue
		case b == 255:
			args++
			i += 5
			continue
		}
		i++
		switch b {
		case 1, 3, 18, 23:
			hints += args / 2
		case 19, 20:
			hints += args / 2
			i += (hints + 7) / 8
		case 14:
			return args >= 4
		case 10, 29, 11:
			return false
		case 12:
			i++
		}
		args = 0
	}
	return false
}

// TestSubsetterGeneratedCFF compares the subsets of CFF fonts that the test
// makes at random with the oracle. The fonts are SID-keyed and CID-keyed,
// and their charstrings call subroutines, hold hint masks, the numbers of
// each form and the arguments of seac. Some fonts have faults that subsetter
// finds, such as a deep recursion of subroutines, too many arguments and
// unknown operators, and some have real numbers in their DICTs that are not
// finite.
func TestSubsetterGeneratedCFF(t *testing.T) {
	needOracle(t)
	dir := t.TempDir()
	rng := rand.New(rand.NewPCG(5, 62))
	var cases []subsetCase
	add := func(name string, font []byte, glyphs []uint16) {
		file := filepath.Join(dir, fmt.Sprintf("gen%04d.otf", len(cases)))
		if err := os.WriteFile(file, font, 0o644); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, subsetCase{name: name, font: file, glyphs: glyphs})
	}
	for i := range 3000 {
		spec := cfRandomSpec(rng)
		font := cfBuildFont(spec)
		n := len(spec.charstrings)
		glyphs := make([]uint16, rng.IntN(n+2))
		for j := range glyphs {
			glyphs[j] = uint16(rng.IntN(n + 1))
		}
		add(fmt.Sprintf("generated %d", i), font, glyphs)
		add(fmt.Sprintf("generated %d all", i), font, parseGids("*", uint16(n)))
	}

	// 256 font DICTs, each for one glyph: the remapper of the font DICTs
	// overflows, and subsetter panics.
	spec := cfSpec{cid: true}
	for i := range 256 {
		spec.charstrings = append(spec.charstrings, []byte{139, 139, 21, 14})
		spec.privates = append(spec.privates, nil)
		spec.lsubrs = append(spec.lsubrs, nil)
		spec.fdExtra = append(spec.fdExtra, nil)
		spec.fdSelect = append(spec.fdSelect, byte(i))
	}
	spec.fdSelect = append([]byte{0}, spec.fdSelect...)
	font := cfBuildFont(spec)
	add("256 font DICTs", font, parseGids("*", 256))
	add("255 font DICTs", font, parseGids("1-254", 256))

	compareSubsets(t, cases)
}

// cfSpec describes a CFF font that cfBuildFont makes. Each DICT and each
// charstring holds its bytes as they are.
type cfSpec struct {
	// cid makes a CID-keyed font, with a ROS, an FDArray and an FDSelect.
	cid bool
	// topExtra holds more entries of the Top DICT.
	topExtra []byte
	// strings is the String INDEX.
	strings [][]byte
	// gsubrs is the Global Subr INDEX.
	gsubrs [][]byte
	// charstrings is the CharStrings INDEX, one charstring for each glyph.
	charstrings [][]byte
	// privates holds the Private DICT, without its Subrs entry, of the
	// font, or of each font DICT of a CID-keyed font.
	privates [][]byte
	// lsubrs holds the local subroutines of each Private DICT. A Private
	// DICT with no subroutine has no Subrs entry.
	lsubrs [][][]byte
	// fdExtra holds more entries of each font DICT.
	fdExtra [][]byte
	// fdSelect is the FDSelect, with its format.
	fdSelect []byte
}

// cfIndex encodes an INDEX with the smallest offset size, or with the size
// 4 when wide is true.
func cfIndex(items [][]byte, wide bool) []byte {
	out := binary.BigEndian.AppendUint16(nil, uint16(len(items)))
	if len(items) == 0 {
		return out
	}
	total := 1
	for _, it := range items {
		total += len(it)
	}
	size := 4
	switch {
	case wide:
	case total <= 0xFF:
		size = 1
	case total <= 0xFFFF:
		size = 2
	case total <= 0xFFFFFF:
		size = 3
	}
	out = append(out, byte(size))
	off := 1
	put := func(v int) {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(v))
		out = append(out, b[4-size:]...)
	}
	put(off)
	for _, it := range items {
		off += len(it)
		put(off)
	}
	for _, it := range items {
		out = append(out, it...)
	}
	return out
}

// cfInt5 encodes a DICT integer in 5 bytes.
func cfInt5(v int) []byte {
	return binary.BigEndian.AppendUint32([]byte{29}, uint32(int32(v)))
}

// cfInt encodes an integer of a DICT or a charstring in its shortest form.
func cfInt(v int) []byte {
	switch {
	case v >= -107 && v <= 107:
		return []byte{byte(v + 139)}
	case v >= 108 && v <= 1131:
		v -= 108
		return []byte{byte(v/256 + 247), byte(v % 256)}
	case v >= -1131 && v <= -108:
		v = -v - 108
		return []byte{byte(v/256 + 251), byte(v % 256)}
	}
	return binary.BigEndian.AppendUint16([]byte{28}, uint16(int16(v)))
}

// cfReal encodes the text of a real number of a DICT, with "E" and "E-"
// for the exponent.
func cfReal(s string) []byte {
	var nibbles []byte
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			nibbles = append(nibbles, c-'0')
		case c == '.':
			nibbles = append(nibbles, 0xa)
		case c == 'E' && i+1 < len(s) && s[i+1] == '-':
			nibbles = append(nibbles, 0xc)
			i++
		case c == 'E':
			nibbles = append(nibbles, 0xb)
		case c == '-':
			nibbles = append(nibbles, 0xe)
		}
	}
	nibbles = append(nibbles, 0xf)
	if len(nibbles)%2 != 0 {
		nibbles = append(nibbles, 0xf)
	}
	out := []byte{30}
	for i := 0; i < len(nibbles); i += 2 {
		out = append(out, nibbles[i]<<4|nibbles[i+1])
	}
	return out
}

// cfBuildCFF encodes the CFF table of a spec. The offsets of the DICTs have
// 5 bytes, so the table is laid out once with the offsets 0 and once with
// the real offsets.
func cfBuildCFF(s cfSpec) []byte {
	build := func(charStrings, fdSelect, fdArray int, privOffs []int) []byte {
		privSizes := make([]int, len(s.privates))
		privs := make([][]byte, len(s.privates))
		for i, p := range s.privates {
			privs[i] = p
			if len(s.lsubrs[i]) > 0 {
				// The Subrs offset is relative to the Private DICT.
				privs[i] = append(slices.Clone(p), cfInt5(len(p)+6)...)
				privs[i] = append(privs[i], 19)
			}
			privSizes[i] = len(privs[i])
		}
		var top []byte
		if s.cid {
			top = append(top, cfInt(391)...)
			top = append(top, cfInt(392)...)
			top = append(top, cfInt(0)...)
			top = append(top, 12, 30)
		}
		top = append(top, s.topExtra...)
		top = append(top, cfInt5(charStrings)...)
		top = append(top, 17)
		if s.cid {
			top = append(top, cfInt5(fdArray)...)
			top = append(top, 12, 36)
			top = append(top, cfInt5(fdSelect)...)
			top = append(top, 12, 37)
		} else {
			top = append(top, cfInt5(privSizes[0])...)
			top = append(top, cfInt5(privOffs[0])...)
			top = append(top, 18)
		}
		out := []byte{1, 0, 4, 4}
		out = append(out, cfIndex([][]byte{[]byte("Test")}, false)...)
		out = append(out, cfIndex([][]byte{top}, false)...)
		out = append(out, cfIndex(s.strings, false)...)
		out = append(out, cfIndex(s.gsubrs, false)...)
		out = append(out, cfIndex(s.charstrings, true)...)
		if s.cid {
			out = append(out, s.fdSelect...)
			var fds [][]byte
			for i := range s.privates {
				fd := slices.Clone(s.fdExtra[i])
				fd = append(fd, cfInt5(privSizes[i])...)
				fd = append(fd, cfInt5(privOffs[i])...)
				fd = append(fd, 18)
				fds = append(fds, fd)
			}
			out = append(out, cfIndex(fds, true)...)
		}
		for i, p := range privs {
			out = append(out, p...)
			if len(s.lsubrs[i]) > 0 {
				out = append(out, cfIndex(s.lsubrs[i], false)...)
			}
		}
		return out
	}
	privOffs := make([]int, len(s.privates))
	first := build(0, 0, 0, privOffs)
	// The layout of the first pass gives each offset. The Private DICTs
	// and their subroutines are at the end.
	at := len(first)
	for i := len(s.privates) - 1; i >= 0; i-- {
		if len(s.lsubrs[i]) > 0 {
			at -= len(cfIndex(s.lsubrs[i], false))
		}
		size := len(s.privates[i])
		if len(s.lsubrs[i]) > 0 {
			size += 6
		}
		at -= size
		privOffs[i] = at
	}
	charStrings := 4 + len(cfIndex([][]byte{[]byte("Test")}, false))
	charStrings += len(cfIndex([][]byte{cfTopOf(first, charStrings)}, false))
	charStrings += len(cfIndex(s.strings, false)) + len(cfIndex(s.gsubrs, false))
	fdSelect := charStrings + len(cfIndex(s.charstrings, true))
	fdArray := fdSelect + len(s.fdSelect)
	return build(charStrings, fdSelect, fdArray, privOffs)
}

// cfTopOf returns the Top DICT of a CFF table whose Top DICT INDEX starts at
// the offset at.
func cfTopOf(cffTable []byte, at int) []byte {
	r := cffTable[at:]
	size := int(r[2])
	var end int
	for _, b := range r[3+size : 3+2*size] {
		end = end<<8 | int(b)
	}
	return r[3+2*size : 3+2*size+end-1]
}

// cfBuildFont makes an OpenType font with the CFF table of a spec, and the
// tables head, hhea, hmtx and maxp.
func cfBuildFont(s cfSpec) []byte {
	n := len(s.charstrings)
	head := make([]byte, 54)
	binary.BigEndian.PutUint32(head, 0x00010000)
	binary.BigEndian.PutUint16(head[18:], 1000)
	hhea := make([]byte, 36)
	binary.BigEndian.PutUint32(hhea, 0x00010000)
	binary.BigEndian.PutUint16(hhea[34:], uint16(n))
	var hmtx []byte
	for i := range n {
		hmtx = binary.BigEndian.AppendUint16(hmtx, uint16(500+i))
		hmtx = binary.BigEndian.AppendUint16(hmtx, uint16(i))
	}
	maxp := make([]byte, 6)
	binary.BigEndian.PutUint32(maxp, 0x00005000)
	binary.BigEndian.PutUint16(maxp[4:], uint16(n))
	tables := []struct {
		tag  string
		data []byte
	}{{"CFF ", cfBuildCFF(s)}, {"head", head}, {"hhea", hhea}, {"hmtx", hmtx}, {"maxp", maxp}}
	out := binary.BigEndian.AppendUint32(nil, 0x4F54544F)
	out = binary.BigEndian.AppendUint16(out, uint16(len(tables)))
	out = append(out, 0, 0, 0, 0, 0, 0)
	off := 12 + 16*len(tables)
	for _, tb := range tables {
		out = append(out, tb.tag...)
		out = binary.BigEndian.AppendUint32(out, 0)
		out = binary.BigEndian.AppendUint32(out, uint32(off))
		out = binary.BigEndian.AppendUint32(out, uint32(len(tb.data)))
		off += (len(tb.data) + 3) &^ 3
	}
	for _, tb := range tables {
		out = append(out, tb.data...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	return out
}

// cfRandomSpec makes the spec of a random CFF font.
func cfRandomSpec(rng *rand.Rand) cfSpec {
	var s cfSpec
	s.cid = rng.IntN(2) == 0
	s.strings = [][]byte{[]byte("Adobe"), []byte("Identity"), []byte("Copyright test"), []byte("Notice"), []byte("FontA"), []byte("FontB")}
	if rng.IntN(3) == 0 {
		// Copyright, Notice and FullName, with a SID that can be out of the
		// String INDEX.
		s.topExtra = append(s.topExtra, cfInt(393+rng.IntN(2))...)
		s.topExtra = append(s.topExtra, 12, 0)
		s.topExtra = append(s.topExtra, cfInt(394+rng.IntN(5))...)
		s.topExtra = append(s.topExtra, 1)
		s.topExtra = append(s.topExtra, cfInt(rng.IntN(400))...)
		s.topExtra = append(s.topExtra, 2)
	}
	if rng.IntN(2) == 0 {
		s.topExtra = append(s.topExtra, cfMatrix(rng)...)
	}
	if rng.IntN(2) == 0 {
		for range 4 {
			s.topExtra = append(s.topExtra, cfInt(rng.IntN(2000)-500)...)
		}
		s.topExtra = append(s.topExtra, 5)
	}
	nfd := 1
	if s.cid {
		nfd = 1 + rng.IntN(3)
	}
	// Each font DICT has the same number of local subroutines, so that a
	// call reaches a subroutine in each one.
	g := cfGen{rng: rng, gsubrs: rng.IntN(5), lsubrs: rng.IntN(5)}
	for i := range g.gsubrs {
		s.gsubrs = append(s.gsubrs, g.charstring(true, i))
	}
	for range nfd {
		s.privates = append(s.privates, cfRandomPrivate(rng))
		var subrs [][]byte
		for i := range g.lsubrs {
			subrs = append(subrs, g.charstring(true, i))
		}
		s.lsubrs = append(s.lsubrs, subrs)
		var extra []byte
		if rng.IntN(2) == 0 {
			extra = append(extra, cfMatrix(rng)...)
		}
		if rng.IntN(2) == 0 {
			extra = append(extra, cfInt(395+rng.IntN(3))...)
			extra = append(extra, 12, 38)
		}
		s.fdExtra = append(s.fdExtra, extra)
	}
	n := 1 + rng.IntN(12)
	for range n {
		s.charstrings = append(s.charstrings, g.charstring(false, g.gsubrs+g.lsubrs))
	}
	fd := func() byte {
		if rng.IntN(10) == 0 {
			return byte(nfd)
		}
		return byte(rng.IntN(nfd))
	}
	if s.cid {
		if rng.IntN(2) == 0 {
			s.fdSelect = []byte{0}
			for range n {
				s.fdSelect = append(s.fdSelect, fd())
			}
		} else {
			s.fdSelect = []byte{3}
			nr := 1 + rng.IntN(3)
			s.fdSelect = binary.BigEndian.AppendUint16(s.fdSelect, uint16(nr))
			first := 0
			for range nr {
				s.fdSelect = binary.BigEndian.AppendUint16(s.fdSelect, uint16(first))
				s.fdSelect = append(s.fdSelect, fd())
				first += rng.IntN(n/nr + 1)
			}
			sentinel := n
			if rng.IntN(10) == 0 {
				sentinel = rng.IntN(n + 1)
			}
			s.fdSelect = binary.BigEndian.AppendUint16(s.fdSelect, uint16(max(first, sentinel)))
		}
	}
	return s
}

// cfMatrix returns a FontMatrix entry of a DICT, with real numbers.
func cfMatrix(rng *rand.Rand) []byte {
	vals := [][]string{
		{"0.001", "0", "0", "0.001", "0", "0"},
		{"0.0005", "0", "0.0001", "0.0005", "10", "-3"},
		{"1", "0", "0", "1", "0", "0"},
		{"2.5E-4", "0", "0", ".0003", "0", "0"},
		{"0.001", "0.0002", "-0.0001", "0.001", "0.5", "-0.25"},
		// An infinity, and a matrix of seven numbers, which subsetter does
		// not read.
		{"1E40", "0", "0", "1", "0", "0"},
		{"0.001", "0", "0", "0.001", "0", "0", "7"},
	}[rng.IntN(5+rng.IntN(2)*2)]
	var out []byte
	for _, v := range vals {
		if v == "0" && rng.IntN(2) == 0 {
			out = append(out, cfInt(0)...)
		} else {
			out = append(out, cfReal(v)...)
		}
	}
	return append(out, 12, 7)
}

// cfRandomPrivate returns the entries of a random Private DICT.
func cfRandomPrivate(rng *rand.Rand) []byte {
	var out []byte
	for range rng.IntN(6) {
		k := rng.IntN(60)
		if k >= 20 {
			// The case 6 is rare.
			k %= 6
		}
		switch k {
		case 0, 7, 8, 9:
			// BlueValues
			for range 2 * (1 + rng.IntN(3)) {
				out = append(out, cfInt(rng.IntN(800)-100)...)
			}
			out = append(out, 6)
		case 1, 10, 11, 12:
			// StdHW and StdVW
			out = append(out, cfInt(rng.IntN(100))...)
			out = append(out, byte(10+rng.IntN(2)))
		case 2:
			// BlueScale
			out = append(out, cfReal([]string{"0.039625", "0.04379", "-1.5E-3", "12345678901234567890", "1E99"}[rng.IntN(4+rng.IntN(2))])...)
			out = append(out, 12, 9)
		case 3, 13, 14, 15, 16:
			// defaultWidthX and nominalWidthX
			out = append(out, cfInt(rng.IntN(3000)-1000)...)
			out = append(out, byte(20+rng.IntN(2)))
		case 4:
			// StemSnapH with many arguments
			for range rng.IntN(60) {
				out = append(out, cfInt(rng.IntN(200))...)
			}
			out = append(out, 12, 12)
		case 5:
			// A large integer
			out = append(out, binary.BigEndian.AppendUint32([]byte{29}, rng.Uint32())...)
			out = append(out, 21)
		case 6:
			// A Subrs entry before the one of cfBuildCFF, which subsetter
			// reads instead.
			out = append(out, cfInt(rng.IntN(50))...)
			out = append(out, 19)
		}
	}
	return out
}

// cfGen makes random charstrings for a font with gsubrs global and lsubrs
// local subroutines.
type cfGen struct {
	rng            *rand.Rand
	gsubrs, lsubrs int
}

// charstring returns a random charstring. A subroutine ends with return,
// endchar or nothing, and a glyph with endchar or nothing. A call of a
// subroutine reaches a subroutine whose index is below limit, so that the
// calls end, or rarely any index.
func (g cfGen) charstring(subr bool, limit int) []byte {
	rng := g.rng
	var out []byte
	num := func() {
		switch rng.IntN(8) {
		case 0:
			out = append(out, 28)
			out = binary.BigEndian.AppendUint16(out, uint16(rng.Uint32()))
		case 1:
			// A fixed number, sometimes a whole number.
			v := rng.Int32() >> rng.IntN(16)
			if rng.IntN(2) == 0 {
				v &^= 0xFFFF
			}
			out = binary.BigEndian.AppendUint32(append(out, 255), uint32(v))
		default:
			out = append(out, cfInt(rng.IntN(2400)-1200)...)
		}
	}
	nums := func(k int) {
		for range k {
			num()
		}
	}
	stems := 0
	if rng.IntN(3) == 0 {
		// A width.
		num()
	}
	for range rng.IntN(12) {
		switch rng.IntN(32) {
		case 0, 1, 2, 3:
			k := 1 + rng.IntN(4)
			nums(2 * k)
			stems += k
			out = append(out, []byte{1, 3, 18, 23}[rng.IntN(4)])
		case 4, 5:
			// A hint mask, with implicit vertical stems sometimes.
			if rng.IntN(2) == 0 {
				k := 1 + rng.IntN(10)
				nums(2 * k)
				stems += k
			}
			out = append(out, byte(19+rng.IntN(2)))
			for range (stems + 7) / 8 {
				out = append(out, byte(rng.Uint32()))
			}
		case 6, 7, 8:
			nums(2)
			out = append(out, 21)
		case 9, 10, 11:
			nums(2 * (1 + rng.IntN(3)))
			out = append(out, 5)
		case 12, 13, 14:
			nums(6 * (1 + rng.IntN(2)))
			out = append(out, []byte{8, 24, 25}[rng.IntN(3)])
		case 15, 16, 17:
			nums(1 + rng.IntN(4))
			out = append(out, []byte{4, 6, 7, 22, 26, 27, 30, 31}[rng.IntN(8)])
		case 18, 19:
			nums([]int{7, 13, 9, 11}[rng.IntN(4)])
			out = append(out, 12, byte(34+rng.IntN(4)))
		case 20, 21, 22, 23, 24:
			// A call of a subroutine, with the bias 107. The index is a
			// number of each form.
			op := []byte{10, 29}[rng.IntN(2)]
			count := g.lsubrs
			if op == 29 {
				count = g.gsubrs
			}
			i := -1
			if reach := min(count, limit); reach > 0 {
				i = rng.IntN(reach)
			}
			if rng.IntN(20) == 0 {
				i = rng.IntN(count + 2)
			}
			if i < 0 {
				continue
			}
			switch rng.IntN(4) {
			case 0:
				out = binary.BigEndian.AppendUint32(append(out, 255), uint32(int32(i-107)<<16))
			case 1:
				out = binary.BigEndian.AppendUint16(append(out, 28), uint16(int16(i-107)))
			default:
				out = append(out, cfInt(i-107)...)
			}
			out = append(out, op)
		case 25:
			// Rare faults: too many arguments, an unknown operator, a
			// number at the end, a call without an index.
			switch rng.IntN(5) {
			case 0:
				nums(49)
				out = append(out, 5)
			case 1:
				out = append(out, []byte{0, 2, 9, 13, 15, 16, 17}[rng.IntN(7)])
			case 2:
				out = append(out, 12, byte(rng.IntN(40)))
			case 3:
				out = append(out, 10)
			default:
				out = append(out, 28, 1)
			}
		default:
			nums(2 * rng.IntN(2))
			out = append(out, 5)
		}
	}
	switch rng.IntN(10) {
	case 0:
		// The arguments of seac, with and without a width.
		nums(4 + rng.IntN(2))
		out = append(out, 14)
	case 1:
		// Three arguments of endchar, which subsetter keeps.
		nums(3)
		out = append(out, 14)
	case 2:
	default:
		if subr && rng.IntN(2) == 0 {
			out = append(out, 11)
		} else {
			out = append(out, 14)
		}
	}
	return out
}
