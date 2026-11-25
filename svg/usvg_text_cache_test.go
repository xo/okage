package svg

import (
	"errors"
	"io/fs"
	"os"
	"testing"
)

// TestBitmapStrikesEBDT checks that the strikes of the table EBDT are read
// as read-fonts 0.46 reads them, and that usvg draws none of their glyphs
// (D100). KacstQurn has two strikes of masks of one bit. The values are the
// ones that the Rust program of D100 printed with
// BitmapStrikes::select_bitmap of read-fonts.
func TestBitmapStrikesEBDT(t *testing.T) {
	data, err := os.ReadFile("../typesetting/testdata/opentype/toys/KacstQurn.ttf")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the test fonts of typesetting are not there")
	}
	if err != nil {
		t.Fatal(err)
	}
	db := NewFontDatabase()
	id := db.LoadFontData(data)[0]
	pf, _ := db.withFaceData(id)

	strikes := bitmapStrikes(pf)
	if len(strikes) != 2 || strikes[0].ppem() != 16 || strikes[1].ppem() != 21 {
		t.Fatalf("strikes are %+v, want the sizes 16 and 21", strikes)
	}
	for _, c := range []struct {
		glyph                        GlyphID
		width, height                uint32
		innerXBearing, innerYBearing int32
	}{
		{0, 15, 26, 0, 21},
		{10, 2, 16, 0, 17},
		{36, 16, 24, 0, 11},
		{100, 5, 4, 1, 19},
		{200, 16, 27, 0, 14},
	} {
		b, ok := selectBitmap(strikes, c.glyph)
		if !ok {
			t.Errorf("glyph %d: no bitmap", c.glyph)
			continue
		}
		if b.pixels != bitmapPixelsMask || b.png != nil || b.width != c.width || b.height != c.height ||
			b.innerXBearing != c.innerXBearing || b.innerYBearing != c.innerYBearing || b.ppemX != 21 || b.ppemY != 21 {
			t.Errorf("glyph %d: bitmap is %+v, want a mask of %dx%d at %d, %d", c.glyph, b, c.width, c.height, c.innerXBearing, c.innerYBearing)
		}
	}

	fonts := newFontCache(db, newOptions())
	if img, ok := fonts.raster(id, 0); ok || img != nil {
		t.Error("raster draws a mask of EBDT")
	}
}
