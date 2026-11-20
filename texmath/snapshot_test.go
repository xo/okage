package texmath

import (
	"bufio"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/typesetting/font"
)

// This file holds the snapshots of D78 section 7, item 2. For each formula
// of test/texmath/corpus.txt and each input of testdata/gate.txt, a file in
// testdata/snapshots holds the formula and its layout with STIX Two Math at
// the size 10, in display style and in text style: the width, the height,
// the depth and the italic correction of the box, and each item. Each float
// is written with strconv.FormatFloat(v, 'g', -1, 64), so a changed bit
// changes the file. A fused multiply and add on one platform shows as a
// changed byte (D16, D71). The nightly jobs run the test on amd64, arm64,
// s390x, wasip1 and TinyGo.
//
// The test reads the formula from the first line of each file, so it needs
// neither the corpus nor the test module. To write the files again, run the
// test with the environment variable TEXMATH_SNAPSHOT_UPDATE=1 in a checkout
// that holds the test module:
//
//	TEXMATH_SNAPSHOT_UPDATE=1 go test -run TestSnapshots ./texmath
//
// The update reads the corpus and the gate, writes a file for each formula,
// and removes the files of the formulas that are no longer there. A person
// reads the diff before the change is staged.

// snapshotDir is the folder of the snapshots.
const snapshotDir = "testdata/snapshots"

// snapshotCorpus is the corpus of the test module, from the folder of the
// package.
const snapshotCorpus = "../test/texmath/corpus.txt"

// snapshotSize is the size of the layout of a snapshot.
const snapshotSize = 10

// snapshotName returns the name of the file of the formula tex: the FNV-1a
// hash of 64 bits of tex, in hexadecimal.
func snapshotName(tex string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(tex))
	return fmt.Sprintf("%016x.txt", h.Sum64())
}

// formatFloat writes v as the snapshots do.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// floats writes each value of vs after a space.
func floats(sb *strings.Builder, vs ...float64) {
	for _, v := range vs {
		sb.WriteByte(' ')
		sb.WriteString(formatFloat(v))
	}
}

// writeColor writes the color c of an item.
func writeColor(sb *strings.Builder, c okage.Color) {
	sb.WriteString(" color")
	floats(sb, float64(c.Red()), float64(c.Green()), float64(c.Blue()), float64(c.Alpha()))
}

// snapshot returns the text of the snapshot of tex, with face for the
// layout.
func snapshot(tex string, face *font.Face) string {
	var sb strings.Builder
	sb.WriteString("tex " + strconv.Quote(tex) + "\n")
	for _, st := range []Style{Display, Text} {
		if st == Display {
			sb.WriteString("\nstyle display\n")
		} else {
			sb.WriteString("\nstyle text\n")
		}
		b, err := LayoutTeX(tex, face, snapshotSize, st)
		if err != nil {
			sb.WriteString("error " + strconv.Quote(err.Error()) + "\n")
			continue
		}
		sb.WriteString("box")
		floats(&sb, b.Width, b.Height, b.Depth, b.Italic)
		sb.WriteByte('\n')
		for i := range b.Items {
			writeItem(&sb, &b.Items[i], face)
		}
	}
	return sb.String()
}

// writeItem writes one item on a line, and the segments of the path of a
// path item on the lines after it.
func writeItem(sb *strings.Builder, it *Item, face *font.Face) {
	switch it.Kind {
	case ItemGlyph:
		sb.WriteString("glyph " + strconv.Itoa(int(it.Glyph)))
		if it.Face != face {
			sb.WriteString(" face other")
		}
		floats(sb, it.Size, it.X, it.Y)
		if it.Text != "" {
			sb.WriteString(" text " + strconv.Quote(it.Text))
		}
	case ItemRule:
		sb.WriteString("rule")
		floats(sb, it.X, it.Y, it.W, it.H)
	case ItemPath:
		sb.WriteString("path")
		floats(sb, it.X, it.Y)
	default:
		sb.WriteString("kind " + strconv.Itoa(int(it.Kind)))
	}
	writeColor(sb, it.Color)
	if t := it.Transform; !t.IsIdentity() {
		sb.WriteString(" transform")
		floats(sb, float64(t.SX), float64(t.KY), float64(t.KX), float64(t.SY), float64(t.TX), float64(t.TY))
	}
	sb.WriteByte('\n')
	if it.Kind != ItemPath || it.Path == nil {
		return
	}
	pts := it.Path.Points()
	for _, v := range it.Path.Verbs() {
		n := 0
		switch v {
		case path.PathVerbMove:
			sb.WriteString("  M")
			n = 1
		case path.PathVerbLine:
			sb.WriteString("  L")
			n = 1
		case path.PathVerbQuad:
			sb.WriteString("  Q")
			n = 2
		case path.PathVerbCubic:
			sb.WriteString("  C")
			n = 3
		case path.PathVerbClose:
			sb.WriteString("  Z")
		}
		for _, p := range pts[:n] {
			floats(sb, float64(p.X), float64(p.Y))
		}
		pts = pts[n:]
		sb.WriteByte('\n')
	}
}

// snapshotFormulas returns the formulas of the corpus and of the gate, once
// each, in that order.
func snapshotFormulas(t *testing.T) []string {
	t.Helper()
	f, err := os.Open(snapshotCorpus)
	if err != nil {
		t.Fatalf("the update needs the corpus of the test module: %v", err)
	}
	defer func() { _ = f.Close() }()
	var out []string
	seen := map[string]bool{}
	add := func(tex string) {
		if !seen[tex] {
			seen[tex] = true
			out = append(out, tex)
		}
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		add(line)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	for _, in := range gateInputs(t) {
		add(in.tex)
	}
	return out
}

// TestSnapshots lays out the formula of each snapshot again, and compares the
// text byte for byte.
func TestSnapshots(t *testing.T) {
	face := testFace(t)
	if os.Getenv("TEXMATH_SNAPSHOT_UPDATE") != "" {
		updateSnapshots(t, face)
		return
	}
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) < 2000 {
		t.Fatalf("%s holds %d snapshots", snapshotDir, len(entries))
	}
	changed := 0
	for _, e := range entries {
		name := e.Name()
		data, err := os.ReadFile(filepath.Join(snapshotDir, name))
		if err != nil {
			t.Fatal(err)
		}
		first, _, _ := strings.Cut(string(data), "\n")
		quoted, ok := strings.CutPrefix(first, "tex ")
		if !ok {
			t.Fatalf("%s: the first line is %q", name, first)
		}
		tex, err := strconv.Unquote(quoted)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := snapshotName(tex); name != want {
			t.Errorf("%s: the name of the snapshot of %q is %s", name, tex, want)
		}
		got := snapshot(tex, face)
		if got == string(data) {
			continue
		}
		changed++
		if changed <= 10 {
			t.Errorf("%s: the snapshot of %q changed:\n%s", name, tex, firstDiff(string(data), got))
		}
	}
	if changed > 0 {
		t.Errorf("%d of %d snapshots changed. Run the test with TEXMATH_SNAPSHOT_UPDATE=1 to write them again", changed, len(entries))
	}
}

// firstDiff returns the first line that differs between want and got, with
// its number.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := range max(len(wl), len(gl)) {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n  want %s\n  got  %s", i+1, w, g)
		}
	}
	return "the same lines"
}

// updateSnapshots writes the snapshot of each formula of the corpus and of
// the gate, and removes the other files of the folder.
func updateSnapshots(t *testing.T, face *font.Face) {
	t.Helper()
	formulas := snapshotFormulas(t)
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := map[string]bool{}
	for _, tex := range formulas {
		name := snapshotName(tex)
		if keep[name] {
			t.Fatalf("two formulas have the name %s", name)
		}
		keep[name] = true
		if err := os.WriteFile(filepath.Join(snapshotDir, name), []byte(snapshot(tex, face)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		t.Fatal(err)
	}
	var removed []string
	for _, e := range entries {
		if !keep[e.Name()] {
			removed = append(removed, e.Name())
			if err := os.Remove(filepath.Join(snapshotDir, e.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
	slices.Sort(removed)
	t.Logf("wrote %d snapshots, removed %d: %v", len(formulas), len(removed), removed)
}

// snapshotInputs returns the formula of each snapshot, in the order of the
// names of the files.
func snapshotInputs(tb testing.TB) []string {
	tb.Helper()
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		tb.Fatal(err)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(snapshotDir, e.Name()))
		if err != nil {
			tb.Fatal(err)
		}
		first, _, _ := strings.Cut(string(data), "\n")
		tex, err := strconv.Unquote(strings.TrimPrefix(first, "tex "))
		if err != nil {
			tb.Fatalf("%s: %v", e.Name(), err)
		}
		out = append(out, tex)
	}
	return out
}

// fuzzSeeds returns the seeds of FuzzParse and FuzzLayout: the inputs of the
// gate, and the formulas of the snapshots, once each.
func fuzzSeeds(tb testing.TB) []string {
	tb.Helper()
	var out []string
	seen := map[string]bool{}
	for _, in := range gateInputs(tb) {
		if !seen[in.tex] {
			seen[in.tex] = true
			out = append(out, in.tex)
		}
	}
	for _, tex := range snapshotInputs(tb) {
		if !seen[tex] {
			seen[tex] = true
			out = append(out, tex)
		}
	}
	return out
}
