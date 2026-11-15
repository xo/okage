package svg

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// webfontTestdata is the folder of the test fonts of internal/webfont.
var webfontTestdata = filepath.Join("..", "internal", "webfont", "testdata")

// TestLoadFontDataWebFonts loads the same font as TrueType, WOFF, WOFF2 and
// EOT, and checks that each one gives the same face and the same outlines.
func TestLoadFontDataWebFonts(t *testing.T) {
	ref := shapeTestFont(t, "noto.ttf")
	for _, name := range []string{"noto.woff", "noto.woff2", "noto-null.woff2", "noto.eot", "noto-xor.eot"} {
		t.Run(name, func(t *testing.T) {
			if got := shapeTestFont(t, name); got != ref {
				t.Errorf("expected %s, got %s", ref, got)
			}
		})
	}
}

// shapeTestFont loads a test font into a new database and returns its
// family, its weight and the box of the outlines of a run of text.
func shapeTestFont(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(webfontTestdata, name))
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	ids := db.LoadFontData(data)
	if len(ids) != 1 {
		t.Fatalf("expected 1 face, got %d", len(ids))
	}
	face, _ := db.Face(ids[0])
	sfnt, _, ok := db.FaceData(ids[0])
	if !ok || !bytes.HasPrefix(sfnt, []byte{0, 1, 0, 0}) {
		t.Errorf("FaceData does not give TrueType data")
	}
	run, err := ShapeText("Hamburgefonstiv", TextStyle{Family: `"` + face.Families[0].Name + `"`, Size: 48}, WithFontDatabase(db))
	if err != nil {
		t.Fatal(err)
	}
	pb := path.NewPathBuilder()
	run.AppendPath(pb, 0, 0)
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("no outlines")
	}
	return face.Families[0].Name + " " + face.PostScriptName + " " + p.Bounds().String()
}

// TestLoadFontDataCollection loads a collection in WOFF2.
func TestLoadFontDataCollection(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(webfontTestdata, "collection.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	ids := db.LoadFontData(data)
	if len(ids) != 2 {
		t.Fatalf("expected 2 faces, got %d", len(ids))
	}
	for i, exp := range []uint16{400, 700} {
		if face, _ := db.Face(ids[i]); face.Weight != exp {
			t.Errorf("face %d: expected the weight %d, got %d", i, exp, face.Weight)
		}
	}
}

// TestLoadFontDataMalformed checks that malformed web fonts and an EOT file
// with MicroType Express compression load no face.
func TestLoadFontDataMalformed(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(webfontTestdata, "noto.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	mtx, err := os.ReadFile(filepath.Join(webfontTestdata, "noto-mtx.eot"))
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{data[:100], mtx} {
		if ids := NewFontDatabase().LoadFontData(b); len(ids) != 0 {
			t.Errorf("expected no face, got %d", len(ids))
		}
	}
}

func TestConvertFontData(t *testing.T) {
	ttf, err := os.ReadFile(filepath.Join(webfontTestdata, "noto.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	// TrueType data is returned as it is.
	if got, err := ConvertFontData(ttf); err != nil || &got[0] != &ttf[0] {
		t.Errorf("TrueType data: expected the same data, got error %v", err)
	}
	woff2, err := os.ReadFile(filepath.Join(webfontTestdata, "noto.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ConvertFontData(woff2); err != nil || !bytes.HasPrefix(got, []byte{0, 1, 0, 0}) {
		t.Errorf("WOFF2: expected TrueType data, got error %v", err)
	}
	if _, err := ConvertFontData(woff2[:100]); !errors.Is(err, okage.ErrFontData) {
		t.Errorf("cut WOFF2: expected okage.ErrFontData, got %v", err)
	}
	if _, err := ConvertFontData([]byte("not a font")); !errors.Is(err, okage.ErrFontUnsupported) {
		t.Errorf("text: expected okage.ErrFontUnsupported, got %v", err)
	}
}

func TestFaceName(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(webfontTestdata, "noto.woff2"))
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	id := db.LoadFontData(data)[0]
	for _, test := range []struct {
		nameID uint16
		exp    string
		ok     bool
	}{
		{1, "Noto Sans", true},
		{2, "Regular", true},
		{4, "Noto Sans Regular", true},
		{5, "Version 2.000;GOOG;noto-source:20170915:90ef993387c0; ttfautohint (v1.7)", true},
		{6, "NotoSans-Regular", true},
		{19, "", false},
	} {
		got, ok := db.FaceName(id, test.nameID)
		if got != test.exp || ok != test.ok {
			t.Errorf("name ID %d: expected %q %t, got %q %t", test.nameID, test.exp, test.ok, got, ok)
		}
	}
	if _, ok := db.FaceName(id+1, 1); ok {
		t.Error("expected no name for a face that does not exist")
	}
}
