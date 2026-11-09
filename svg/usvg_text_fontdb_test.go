package svg

import (
	"os"
	"testing"
	"testing/fstest"
)

// This file tests the font database of usvg_text_fontdb.go, which takes the
// place of fontdb 0.24. fontdb has no test of its matching, so the cases are
// the rules of find_best_match, on the fonts of resvg.

// queryPostScriptName returns the PostScript name of the face that query
// finds in db, or the empty string.
func queryPostScriptName(t *testing.T, db *FontDatabase, query FontQuery) string {
	t.Helper()
	id, ok := db.Query(query)
	if !ok {
		return ""
	}
	face, ok := db.Face(id)
	if !ok {
		t.Fatalf("Query returned the ID %d, which Face does not find", id)
	}
	return face.PostScriptName
}

// TestFontDatabaseQuery checks the faces and the CSS matching of the font
// database.
func TestFontDatabaseQuery(t *testing.T) {
	db := writeFontDB()
	if db.Len() < 20 {
		t.Fatalf("Len() = %d, want the faces of testdata/fonts", db.Len())
	}

	sans := []FontFamily{{Kind: FontFamilySansSerif}}
	for _, c := range []struct {
		name  string
		query FontQuery
		want  string
	}{
		{"regular", FontQuery{Families: sans, Weight: 400}, "NotoSans-Regular"},
		{"bold", FontQuery{Families: sans, Weight: 700}, "NotoSans-Bold"},
		// Above 500, the closest fatter weight comes first.
		{"600", FontQuery{Families: sans, Weight: 600}, "NotoSans-Bold"},
		// Between 400 and 450, 500 comes first, then thinner weights.
		{"420", FontQuery{Families: sans, Weight: 420}, "NotoSans-Regular"},
		// Below 400, the closest thinner weight comes first.
		{"250", FontQuery{Families: sans, Weight: 250}, "NotoSans-Thin"},
		{"italic", FontQuery{Families: sans, Weight: 400, Style: FontStyleItalic}, "NotoSans-Italic"},
		// Oblique falls back to italic before normal.
		{"oblique", FontQuery{Families: sans, Weight: 400, Style: FontStyleOblique}, "NotoSans-Italic"},
		{"named", FontQuery{Families: []FontFamily{{Kind: FontFamilyNamed, Name: "Noto Serif"}}, Weight: 400}, "NotoSerif-Regular"},
		{"monospace", FontQuery{Families: []FontFamily{{Kind: FontFamilyMonospace}}, Weight: 400}, "NotoMono"},
		{
			"first family that matches",
			FontQuery{Families: []FontFamily{{Kind: FontFamilyNamed, Name: "No Such Font"}, {Kind: FontFamilyCursive}}, Weight: 400},
			"Yellowtail-Regular",
		},
		{"no match", FontQuery{Families: []FontFamily{{Kind: FontFamilyNamed, Name: "No Such Font"}}, Weight: 400}, ""},
	} {
		if got := queryPostScriptName(t, db, c.query); got != c.want {
			t.Errorf("%s: Query found %q, want %q", c.name, got, c.want)
		}
	}
}

// TestFontDatabaseClone checks that a font loaded into a copy does not go
// into the database it was copied from.
func TestFontDatabaseClone(t *testing.T) {
	data, err := os.ReadFile("testdata/fonts/NotoMono-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	db.LoadFontData(data)
	c := db.Clone()
	c.SetSerifFamily("Noto Mono")
	c.LoadFontData(data)
	if db.Len() != 1 || c.Len() != 2 {
		t.Errorf("Len() = %d and %d, want 1 and 2", db.Len(), c.Len())
	}
	if db.FamilyName(FontFamily{Kind: FontFamilySerif}) != "Times New Roman" {
		t.Error("SetSerifFamily of a copy changed the database it was copied from")
	}
}

// TestFontDatabaseLoadFS checks LoadFontsFS and LoadSystemFonts on a file
// system in memory.
func TestFontDatabaseLoadFS(t *testing.T) {
	data, err := os.ReadFile("testdata/fonts/NotoMono-Regular.ttf")
	if err != nil {
		t.Fatal(err)
	}
	fsys := fstest.MapFS{
		"usr/share/fonts/truetype/NotoMono.TTF": {Data: data},
		"usr/share/fonts/readme.txt":            {Data: []byte("not a font")},
		"usr/share/fonts/broken.ttf":            {Data: []byte("not a font")},
	}
	db := NewFontDatabase()
	db.LoadSystemFonts(fsys)
	if db.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", db.Len())
	}
	face, _ := db.Face(0)
	if face.Families[0].Name != "Noto Mono" || face.Families[0].Language != FontLanguageEnglishUnitedStates {
		t.Errorf("Families[0] = %+v, want Noto Mono in English", face.Families[0])
	}
	if !face.Monospaced {
		t.Error("Monospaced = false, want true")
	}

	// A file system without the folders of a system loads no font.
	db = NewFontDatabase()
	db.LoadSystemFonts(fstest.MapFS{})
	if !db.IsEmpty() {
		t.Errorf("Len() = %d, want 0", db.Len())
	}
}
