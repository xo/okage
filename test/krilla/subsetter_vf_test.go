package krilla

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The tests of this file compare the instances of variable fonts of
// internal/subsetter with the instances of subsetter, which the oracle gives
// with the question "subset" (item 5 of "Testing" under "The PDF writer" in
// docs/PLAN.md). subsetter draws each glyph of an instance with skrifa, and
// internal/subsetter draws it with typesetting. Before D84, typesetting
// computed the deltas of the variations in float32 where skrifa computes them
// in fixed point, and a point of an instance could differ by 1 unit. The
// tests still count such points. At the default location, a subset of a CFF2
// font is still an instance, and it must be the same, byte for byte.

// vfFont is a variable font of the comparison.
type vfFont struct {
	file string
	// glyphs is a set of glyphs that a test of subsetter takes from the
	// font, or nil.
	glyphs string
}

// vfFonts returns the variable fonts of the comparison: the variable fonts
// of subsetter, and the variable fonts of the tests of okage.
func vfFonts(t *testing.T) []vfFont {
	t.Helper()
	sub := func(name string) string { return abs(t, "upstream", "subsetter", "fonts", name) }
	fonts := []vfFont{
		{sub("NotoSans-Regular_var.ttf"), "10,40,58,201-205"},
		{sub("Cantarell-VF.otf"), "1,15,30-35,40,103-105"},
		{abs(t, "svg", "testdata", "fonts", "RobotoFlex.subset.ttf"), ""},
		{abs(t, "typesetting", "font", "testdata", "Selawik-VF-Subset.ttf"), ""},
	}
	for _, f := range fonts {
		if _, err := os.Stat(f.file); err != nil {
			t.Skipf("a variable font is missing: %v", err)
		}
	}
	return fonts
}

// vfAxis is an axis of the fvar table, with its range.
type vfAxis struct {
	tag           string
	min, def, max float32
}

// vfAxes returns the axes of the fvar table of a font.
func vfAxes(f sfnt) []vfAxis {
	fvar := f.table("fvar")
	if len(fvar) < 16 {
		return nil
	}
	off := int(binary.BigEndian.Uint16(fvar[4:]))
	n := int(binary.BigEndian.Uint16(fvar[8:]))
	size := int(binary.BigEndian.Uint16(fvar[10:]))
	fixed := func(b []byte) float32 { return float32(int32(binary.BigEndian.Uint32(b))) / 65536 }
	var out []vfAxis
	for i := range n {
		r := off + i*size
		if r+20 > len(fvar) {
			break
		}
		out = append(out, vfAxis{string(fvar[r : r+4]), fixed(fvar[r+4:]), fixed(fvar[r+8:]), fixed(fvar[r+12:])})
	}
	return out
}

// vfCases returns the cases of a variable font: the locations of the tests
// of subsetter, the default location, the ends of each axis and points
// between them, and random locations of all the axes. Each location takes
// the glyphs of the test of subsetter, every glyph, and random sets.
func vfCases(t *testing.T, font vfFont) []subsetCase {
	t.Helper()
	data, err := os.ReadFile(font.file)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := openFont(data, 0)
	if !ok {
		t.Fatalf("%s: cannot read the table directory", font.file)
	}
	n := f.numGlyphs()
	axes := vfAxes(f)
	rng := rand.New(rand.NewPCG(uint64(len(data)), 13))
	name := filepath.Base(font.file)

	var locations [][]coord
	if font.glyphs != "" {
		// The locations of tests/data/fonttools.tests of subsetter. Upstream
		// skipped the last two of NotoSans-Regular_var.ttf in unit 2 of phase
		// 13, because they need the instances.
		locations = append(locations,
			[]coord{{"wght", 400}}, []coord{{"wght", 900}}, []coord{{"wght", 800}, {"wdth", 70}}, []coord{{"wght", 800}})
	}
	var def []coord
	for _, a := range axes {
		def = append(def, coord{a.tag, a.def})
	}
	locations = append(locations, def)
	for _, a := range axes {
		for _, v := range []float32{a.min, a.max, (a.min + a.def) / 2, (a.def + a.max) / 2, a.def + (a.max-a.def)/3} {
			locations = append(locations, []coord{{a.tag, v}})
		}
	}
	for range 12 {
		var loc []coord
		for _, a := range axes {
			loc = append(loc, coord{a.tag, a.min + rng.Float32()*(a.max-a.min)})
		}
		locations = append(locations, loc)
	}

	sets := map[string][]uint16{"all": parseGids("*", n)}
	if font.glyphs != "" {
		sets["subsetter"] = parseGids(font.glyphs, n)
	}
	for i := range 3 {
		glyphs := make([]uint16, 1+rng.IntN(40))
		for j := range glyphs {
			glyphs[j] = uint16(rng.IntN(int(n)))
		}
		sets[fmt.Sprintf("random %d", i)] = glyphs
	}

	var cases []subsetCase
	for _, loc := range locations {
		for _, set := range slices.Sorted(func(yield func(string) bool) {
			for k := range sets {
				if !yield(k) {
					return
				}
			}
		}) {
			loc := slices.Clone(loc)
			cases = append(cases, subsetCase{
				name:   fmt.Sprintf("%s %v %s", name, loc, set),
				font:   font.file,
				glyphs: sets[set],
				coords: &loc,
			})
		}
	}
	return cases
}

// vfMaxBeyond is the largest share of the glyphs of all the instances of a
// font that can differ by more than 1 unit from the glyphs of subsetter.
// The maintainers accepted this tolerance on 2026-11-18 (item 5 of
// "Testing" under "The PDF writer" in docs/PLAN.md). skrifa computes
// the deltas in fixed point, and rounds each step, so a coordinate that is
// very near half a unit can round the other way. A point of a component and
// the offset of the component can both round the other way, and give a
// point that differs by 2 units. In a CFF2 font, a point of a cubic curve
// that differs by 1 unit can change the number of quadratic curves of the
// curve.
const vfMaxBeyond = 0.001

// TestSubsetterVariable compares the instances of the variable fonts with
// the oracle. A subset at the default location must be the same, byte for
// byte. In an instance, each coordinate, advance and left side bearing must
// be within 1 unit of the instance of subsetter. An on-curve point that one
// instance implies and the other does not counts as within 1 unit when the
// points are within 1 unit. The test counts the values that differ, and
// fails when more than vfMaxBeyond of the glyphs differ by more than 1 unit.
func TestSubsetterVariable(t *testing.T) {
	needOracle(t)
	var cases []subsetCase
	for _, font := range vfFonts(t) {
		cases = append(cases, vfCases(t, font)...)
	}
	questions := make([]any, len(cases))
	for i, c := range cases {
		questions[i] = subsetQuestion{Subset: subsetSpec{Font: c.font, Index: c.index, Glyphs: c.glyphs, Coords: c.coords}}
	}
	answers := ask(t, questions...)
	totals := map[string]*vfDiff{}
	for i, c := range cases {
		data, err := os.ReadFile(c.font)
		if err != nil {
			t.Fatal(err)
		}
		total := totals[filepath.Base(c.font)]
		if total == nil {
			total = &vfDiff{}
			totals[filepath.Base(c.font)] = total
		}
		total.cases++
		tag, got, err := goSubset(data, c.index, c.glyphs, c.coords)
		a := answers[i]
		if err != nil || a.Err != "" {
			sameResult(t, c.name, a, tag, got, err)
			continue
		}
		if string(a.Data[:6]) != tag {
			t.Errorf("%s: the tag is %s, and subsetter gives %q", c.name, tag, a.Data[:6])
			continue
		}
		if f, ok := openFont(got, 0); ok {
			total.allGlyphs += int(f.numGlyphs())
		}
		if slices.Equal(got, a.Data[6:]) {
			total.same++
			continue
		}
		if vfIsDefault(t, c) {
			sameResult(t, c.name, a, tag, got, err)
			continue
		}
		d, err := vfCompare(got, a.Data[6:])
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		for _, msg := range d.messages {
			t.Logf("%s: %s", c.name, msg)
		}
		total.add(d)
	}
	for _, name := range slices.Sorted(maps.Keys(totals)) {
		d := totals[name]
		t.Logf("%s: %d cases, %d the same as subsetter, byte for byte, with %d glyphs in all. In the other cases, of %d points, %d differ by 1 unit. "+
			"Of their %d glyphs, %d differ in an implied on-curve point, %d differ by more than 1 unit, and %d advances and %d left side bearings differ by 1 unit",
			name, d.cases, d.same, d.allGlyphs, d.points, d.pointsOff, d.glyphs, d.implied, d.beyond, d.advancesOff, d.lsbsOff)
		if d.same == 0 {
			t.Errorf("%s: no subset is the same as the subset of subsetter", name)
		}
		if float64(d.beyond) > vfMaxBeyond*float64(d.allGlyphs) {
			t.Errorf("%s: %d of the %d glyphs of the instances differ by more than 1 unit", name, d.beyond, d.allGlyphs)
		}
	}
}

// vfIsDefault reports whether the case is at the default location of its
// font.
func vfIsDefault(t *testing.T, c subsetCase) bool {
	t.Helper()
	data, err := os.ReadFile(c.font)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := openFont(data, c.index)
	for _, a := range vfAxes(f) {
		for _, v := range *c.coords {
			if v.Tag == a.tag && v.Value != a.def {
				return false
			}
		}
	}
	return true
}

// vfDiff counts the differences of the instances of a font.
type vfDiff struct {
	cases, same int
	// allGlyphs counts the glyphs of all the instances, also of the
	// instances that are the same as the instances of subsetter.
	allGlyphs                    int
	points, pointsOff            int
	glyphs, advancesOff, lsbsOff int
	// implied counts the glyphs where one instance has an implied on-curve
	// point that the other has as a point of its own.
	implied int
	// beyond counts the glyphs that differ by more than 1 unit, and messages
	// describe them.
	beyond   int
	messages []string
}

func (d *vfDiff) add(o vfDiff) {
	d.points += o.points
	d.pointsOff += o.pointsOff
	d.glyphs += o.glyphs
	d.advancesOff += o.advancesOff
	d.lsbsOff += o.lsbsOff
	d.implied += o.implied
	d.beyond += o.beyond
}

// vfGlyph is a simple glyph: the end point of each contour, and the flag
// on-curve and the coordinates of each point.
type vfGlyph struct {
	ends   []uint16
	on     []bool
	xs, ys []int
}

// vfParseGlyph decodes a simple glyph of the glyf table. An empty glyph has
// no contour.
func vfParseGlyph(d []byte) (vfGlyph, error) {
	var g vfGlyph
	if len(d) == 0 {
		return g, nil
	}
	if len(d) < 10 {
		return g, fmt.Errorf("a glyph of %d bytes", len(d))
	}
	n := int(int16(binary.BigEndian.Uint16(d)))
	if n < 0 {
		return g, errors.New("a composite glyph")
	}
	at := 10
	for range n {
		if at+2 > len(d) {
			return g, errors.New("the glyph ends in its contours")
		}
		g.ends = append(g.ends, binary.BigEndian.Uint16(d[at:]))
		at += 2
	}
	if at+2 > len(d) {
		return g, errors.New("the glyph ends before its instructions")
	}
	at += 2 + int(binary.BigEndian.Uint16(d[at:]))
	points := 0
	if n > 0 {
		points = int(g.ends[n-1]) + 1
	}
	flags := make([]byte, 0, points)
	for len(flags) < points {
		if at >= len(d) {
			return g, errors.New("the glyph ends in its flags")
		}
		f := d[at]
		at++
		flags = append(flags, f)
		if f&8 != 0 {
			if at >= len(d) {
				return g, errors.New("the glyph ends in its flags")
			}
			for range d[at] {
				flags = append(flags, f)
			}
			at++
		}
	}
	read := func(short, same byte) ([]int, error) {
		out := make([]int, points)
		v := 0
		for i, f := range flags[:points] {
			switch {
			case f&short != 0:
				if at >= len(d) {
					return nil, errors.New("the glyph ends in its coordinates")
				}
				if f&same != 0 {
					v += int(d[at])
				} else {
					v -= int(d[at])
				}
				at++
			case f&same == 0:
				if at+2 > len(d) {
					return nil, errors.New("the glyph ends in its coordinates")
				}
				v += int(int16(binary.BigEndian.Uint16(d[at:])))
				at += 2
			}
			out[i] = v
		}
		return out, nil
	}
	var err error
	if g.xs, err = read(0x02, 0x10); err != nil {
		return g, err
	}
	if g.ys, err = read(0x04, 0x20); err != nil {
		return g, err
	}
	for _, f := range flags[:points] {
		g.on = append(g.on, f&1 != 0)
	}
	return g, nil
}

// vfMetrics returns the advance and the left side bearing of each glyph of
// the hmtx table.
func vfMetrics(f sfnt) ([][2]int, error) {
	hhea, hmtx := f.table("hhea"), f.table("hmtx")
	if len(hhea) < 36 {
		return nil, errors.New("no hhea table")
	}
	long := int(binary.BigEndian.Uint16(hhea[34:]))
	var out [][2]int
	adv := 0
	for g := range int(f.numGlyphs()) {
		if g < long {
			if 4*g+4 > len(hmtx) {
				return nil, errors.New("the hmtx table is short")
			}
			adv = int(binary.BigEndian.Uint16(hmtx[4*g:]))
			out = append(out, [2]int{adv, int(int16(binary.BigEndian.Uint16(hmtx[4*g+2:])))})
			continue
		}
		at := 4*long + 2*(g-long)
		if at+2 > len(hmtx) {
			return nil, errors.New("the hmtx table is short")
		}
		out = append(out, [2]int{adv, int(int16(binary.BigEndian.Uint16(hmtx[at:])))})
	}
	return out, nil
}

// vfCompare compares two instances glyph by glyph. It returns an error when
// they have other glyphs or tables that it cannot read, and counts the
// glyphs that differ by more than 1 unit.
func vfCompare(got, want []byte) (vfDiff, error) {
	var d vfDiff
	fg, ok1 := openFont(got, 0)
	fw, ok2 := openFont(want, 0)
	if !ok1 || !ok2 {
		return d, errors.New("cannot read the table directories")
	}
	if fg.numGlyphs() != fw.numGlyphs() {
		return d, fmt.Errorf("%d glyphs, and subsetter gives %d", fg.numGlyphs(), fw.numGlyphs())
	}
	mg, err := vfMetrics(fg)
	if err != nil {
		return d, err
	}
	mw, err := vfMetrics(fw)
	if err != nil {
		return d, err
	}
	near := func(a, b int) (bool, bool) {
		diff := a - b
		return diff >= -1 && diff <= 1, diff != 0
	}
	beyond := func(format string, args ...any) {
		d.beyond++
		d.messages = append(d.messages, fmt.Sprintf(format, args...))
	}
	for g := range fg.numGlyphs() {
		d.glyphs++
		ok, off := near(mg[g][0], mw[g][0])
		if !ok {
			beyond("the glyph %d has the advance %d, and subsetter gives %d", g, mg[g][0], mw[g][0])
			continue
		}
		if off {
			d.advancesOff++
		}
		ok, off = near(mg[g][1], mw[g][1])
		if !ok {
			beyond("the glyph %d has the left side bearing %d, and subsetter gives %d", g, mg[g][1], mw[g][1])
			continue
		}
		if off {
			d.lsbsOff++
		}
		a, err := vfParseGlyph(fg.glyph(g))
		if err != nil {
			return d, fmt.Errorf("the glyph %d: %w", g, err)
		}
		b, err := vfParseGlyph(fw.glyph(g))
		if err != nil {
			return d, fmt.Errorf("the glyph %d of subsetter: %w", g, err)
		}
		if slices.Equal(a.ends, b.ends) && slices.Equal(a.on, b.on) {
			points, pointsOff := 0, 0
			for i := range a.xs {
				points++
				okx, offx := near(a.xs[i], b.xs[i])
				oky, offy := near(a.ys[i], b.ys[i])
				if !okx || !oky {
					beyond("the point %d of the glyph %d is (%d, %d), and subsetter gives (%d, %d)",
						i, g, a.xs[i], a.ys[i], b.xs[i], b.ys[i])
					points = -1
					break
				}
				if offx || offy {
					pointsOff++
				}
			}
			if points >= 0 {
				d.points += points
				d.pointsOff += pointsOff
			}
			continue
		}
		// One of the glyphs drops an on-curve point that the other keeps,
		// because a point at the middle of two off-curve points is implied.
		// The glyphs are compared with each implied point made explicit.
		points, pointsOff, err := vfCompareExplicit(a, b)
		if err != nil {
			beyond("the glyph %d: %v", g, err)
			continue
		}
		d.points += points
		d.pointsOff += pointsOff
		d.implied++
	}
	return d, nil
}

// vfExplicit returns the contours of a glyph with each implied on-curve
// point made explicit. The coordinates are doubled, so that the midpoint of
// two points is an integer.
func vfExplicit(g vfGlyph) [][][3]int {
	var out [][][3]int
	start := 0
	for _, end := range g.ends {
		var c [][3]int
		n := int(end) + 1 - start
		for i := range n {
			p := start + i
			c = append(c, [3]int{2 * g.xs[p], 2 * g.ys[p], vfBool(g.on[p])})
			q := start + (i+1)%n
			if !g.on[p] && !g.on[q] {
				c = append(c, [3]int{g.xs[p] + g.xs[q], g.ys[p] + g.ys[q], 1})
			}
		}
		out = append(out, c)
		start = int(end) + 1
	}
	return out
}

func vfBool(b bool) int {
	if b {
		return 1
	}
	return 0
}

// vfCompareExplicit compares two glyphs with their implied on-curve points
// made explicit. A contour can start at another point, when the first point
// of one glyph is implied. It returns the number of points and of points
// that differ by 1 unit.
func vfCompareExplicit(a, b vfGlyph) (int, int, error) {
	ca, cb := vfExplicit(a), vfExplicit(b)
	if len(ca) != len(cb) {
		return 0, 0, fmt.Errorf("%d contours, and subsetter gives %d", len(ca), len(cb))
	}
	points, off := 0, 0
	for i := range ca {
		pa, pb := ca[i], cb[i]
		if len(pa) != len(pb) {
			return 0, 0, fmt.Errorf("the contour %d has %d points with the implied points, and subsetter gives %d", i, len(pa), len(pb))
		}
		found := false
		for r := range pb {
			n, ok := 0, true
			for j := range pa {
				p, q := pa[j], pb[(j+r)%len(pb)]
				dx, dy := p[0]-q[0], p[1]-q[1]
				if p[2] != q[2] || dx < -2 || dx > 2 || dy < -2 || dy > 2 {
					ok = false
					break
				}
				if dx != 0 || dy != 0 {
					n++
				}
			}
			if ok {
				points += len(pa)
				off += n
				found = true
				break
			}
		}
		if !found {
			return 0, 0, fmt.Errorf("the contour %d differs by more than 1 unit from the contour of subsetter", i)
		}
	}
	return points, off, nil
}
