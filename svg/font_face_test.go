package svg

import (
	"os"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/xo/okage/typesetting/font/opentype/tables"
)

// This file tests the faces, the character maps and the metrics that
// font_face.go gives a program (D57).

// testFontDB returns a database of the files of testdata/fonts, with the ID
// of the first face of each file.
func testFontDB(t *testing.T, names ...string) (*FontDatabase, []FontID) {
	t.Helper()
	db := NewFontDatabase()
	ids := make([]FontID, 0, len(names))
	for _, name := range names {
		data, err := os.ReadFile("testdata/fonts/" + name)
		if err != nil {
			t.Fatal(err)
		}
		loaded := db.LoadFontData(data)
		if len(loaded) == 0 {
			t.Fatalf("%s: no face", name)
		}
		ids = append(ids, loaded[0])
	}
	return db, ids
}

// TestFaceFont checks that FaceFont gives the font that the database keeps,
// and that NewFace makes faces of that font without a parse.
func TestFaceFont(t *testing.T) {
	db, ids := testFontDB(t, "NotoSans-Regular.ttf")
	f, ok := db.FaceFont(ids[0])
	if !ok {
		t.Fatal("FaceFont is false")
	}
	pf, _ := db.withFaceData(ids[0])
	if f != pf.font {
		t.Error("FaceFont is not the font of the database")
	}
	again, _ := db.FaceFont(ids[0])
	if again != f {
		t.Error("a second FaceFont parsed the font again")
	}
	face, ok := db.NewFace(ids[0], nil)
	if !ok {
		t.Fatal("NewFace is false")
	}
	if face.Font != f {
		t.Error("the face of NewFace has another font")
	}
	if other, _ := db.NewFace(ids[0], nil); other == face {
		t.Error("NewFace returned the same face twice")
	}
	if len(face.Coords()) != 0 {
		t.Errorf("Coords() = %v, want none for a font without axes", face.Coords())
	}
	if _, ok := db.FaceFont(ids[0] + 1); ok {
		t.Error("FaceFont is true for a face that does not exist")
	}
	if _, ok := db.NewFace(ids[0]+1, nil); ok {
		t.Error("NewFace is true for a face that does not exist")
	}
}

// TestNewFaceVariations checks that NewFace gives the coordinates of the font
// instance of the layout: the axes of the font only, within their ranges.
func TestNewFaceVariations(t *testing.T) {
	db, ids := testFontDB(t, "RobotoFlex.subset.ttf")
	cache := newFontCache(db, newOptions())
	l, ok := cache.load(ids[0])
	if !ok {
		t.Fatal("no font")
	}
	for _, variations := range [][]FontVariation{
		{NewFontVariation([4]byte{'w', 'g', 'h', 't'}, 700)},
		{NewFontVariation([4]byte{'w', 'g', 'h', 't'}, 5000), NewFontVariation([4]byte{'w', 'd', 't', 'h'}, 30)},
		{NewFontVariation([4]byte{'n', 'o', 'n', 'e'}, 1)},
		AutoFontVariations(nil, 300, FontStretchCondensed, FontStyleOblique),
	} {
		face, ok := db.NewFace(ids[0], variations)
		if !ok {
			t.Fatal("NewFace is false")
		}
		want := l.instance(variations).face.Coords()
		if !slices.Equal(face.Coords(), want) {
			t.Errorf("%v: Coords() = %v, want %v", variations, face.Coords(), want)
		}
	}
	face, _ := db.NewFace(ids[0], []FontVariation{NewFontVariation([4]byte{'w', 'g', 'h', 't'}, 5000)})
	if !slices.ContainsFunc(face.Coords(), func(c tables.Coord) bool { return c == 1<<14 }) {
		t.Errorf("Coords() = %v, want the largest weight of the axis", face.Coords())
	}
}

// TestNominalGlyph checks NominalGlyph against the cmap of the parsed font,
// and that it reads a font of an fs.FS without a parse of the face.
func TestNominalGlyph(t *testing.T) {
	data, err := os.ReadFile("testdata/fonts/NotoSans-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	db.LoadFontsFS(fstest.MapFS{"NotoSans-Regular.ttf": {Data: data}}, ".")
	id := FontID(0)
	chars := []rune{'A', 'z', 'б', 0x1F600, 0x10FFFF}
	type result struct {
		g  GlyphID
		ok bool
	}
	var got []result
	for _, c := range chars {
		g, ok := db.NominalGlyph(id, c)
		got = append(got, result{g, ok})
	}
	if face, _ := db.Face(id); face.source.faces != nil {
		t.Error("NominalGlyph parsed the face")
	}
	f, _ := db.FaceFont(id)
	for i, c := range chars {
		g, ok := f.NominalGlyph(c)
		if want := (result{GlyphID(g), ok}); got[i] != want {
			t.Errorf("NominalGlyph(%q) = %v, want %v", c, got[i], want)
		}
	}
	if !got[0].ok || got[3].ok {
		t.Errorf("NominalGlyph = %v, want A and no emoji", got)
	}
	if _, ok := db.NominalGlyph(id+1, 'A'); ok {
		t.Error("NominalGlyph is true for a face that does not exist")
	}
}

// TestDefaultFallbackSelectorReadsCmap checks that the fallback selector
// finds a face with the character map of the database, and does not parse
// the faces that it looks at.
func TestDefaultFallbackSelectorReadsCmap(t *testing.T) {
	db, ids := testFontDB(t, "NotoSans-Regular.ttf", "MPLUS1p-Regular.ttf")
	id, ok := DefaultFallbackSelector(nil)('あ', ids[:1], db)
	if !ok || id != ids[1] {
		t.Fatalf("DefaultFallbackSelector = %d, %v, want %d, true", id, ok, ids[1])
	}
	if face, _ := db.Face(ids[1]); face.source.faces != nil {
		t.Error("the selector parsed the face of the fallback")
	}
}

// TestFaceMetrics checks FaceMetrics against the font that the layout
// resolves, and against the box of ShapeText.
func TestFaceMetrics(t *testing.T) {
	db, ids := testFontDB(t, "NotoSans-Regular.ttf")
	m, ok := db.FaceMetrics(ids[0], nil)
	if !ok {
		t.Fatal("FaceMetrics is false")
	}
	f, ok := resolveFont(ids[0], nil, newFontCache(db, newOptions()))
	if !ok {
		t.Fatal("no resolved font")
	}
	want := FontMetrics{
		UnitsPerEm:          f.unitsPerEm,
		Ascent:              f.ascent,
		Descent:             f.descent,
		XHeight:             f.xHeight,
		UnderlinePosition:   f.underlinePosition,
		UnderlineThickness:  f.underlineThickness,
		LineThroughPosition: f.lineThroughPosition,
		SubscriptOffset:     f.subscriptOffset,
		SuperscriptOffset:   f.superscriptOffset,
	}
	if m != want {
		t.Errorf("FaceMetrics = %+v, want %+v", m, want)
	}
	if m.UnitsPerEm != 1000 || m.Ascent <= 0 || m.Descent >= 0 || m.XHeight == 0 {
		t.Errorf("FaceMetrics = %+v, want the metrics of Noto Sans", m)
	}

	run, err := ShapeText("x", TextStyle{Family: "Noto Sans", Size: 100}, WithFontDatabase(db))
	if err != nil {
		t.Fatal(err)
	}
	b := run.Bounds()
	scale := float32(100) / float32(m.UnitsPerEm)
	// Bottom is Top plus the height, so it can differ in the last bits.
	if b.Top() != -float32(m.Ascent)*scale || abs32(b.Bottom()+float32(m.Descent)*scale) > 1e-3 {
		t.Errorf("Bounds() = %v, want from %v to %v", b, -float32(m.Ascent)*scale, -float32(m.Descent)*scale)
	}
	if _, ok := db.FaceMetrics(ids[0]+1, nil); ok {
		t.Error("FaceMetrics is true for a face that does not exist")
	}
}
