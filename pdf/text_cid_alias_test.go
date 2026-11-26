package pdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"testing"

	"github.com/xo/okage/internal/pdfcompat"
	"github.com/xo/okage/path"
)

// aliasGlyphs draws the glyph 36 of a font with the text "a", then with the
// text "b", and the glyph 37 without text, as a mark after its base. It
// returns the bytes of the file, without compression.
func aliasGlyphs(t *testing.T, font *Font) []byte {
	t.Helper()
	d := NewDocument(WithPretty(true), WithCompression(false), WithXMPMetadata(false))
	ps, _ := PageSettingsFromWH(200, 200)
	s := d.StartPage(ps).Surface()
	glyphs := []Glyph{
		{GlyphID: 36, TextRange: TextRange{0, 1}, XAdvance: 0.5},
		{GlyphID: 36, TextRange: TextRange{1, 2}, XAdvance: 0.5},
		{GlyphID: 37, TextRange: TextRange{2, 2}, XAdvance: 0.5},
	}
	s.DrawGlyphs(path.PointFromXY(0, 50), glyphs, font, "ab", 12, false)
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// literalString matches a literal string of PDF.
var literalString = regexp.MustCompile(`\(((?:[^\\()]|\\.|\\[0-7]{1,3})*)\)`)

// shownStrings returns the bytes of the literal strings of data, one after
// the other, with their escapes decoded.
func shownStrings(data []byte) []byte {
	var out []byte
	for _, m := range literalString.FindAllSubmatch(data, -1) {
		s := m[1]
		for i := 0; i < len(s); i++ {
			if s[i] != '\\' || i+1 == len(s) {
				out = append(out, s[i])
				continue
			}
			i++
			if s[i] < '0' || s[i] > '7' {
				out = append(out, s[i])
				continue
			}
			v := 0
			for j := 0; j < 3 && i < len(s) && s[i] >= '0' && s[i] <= '7'; j++ {
				v = v*8 + int(s[i]-'0')
				i++
			}
			i--
			out = append(out, byte(v))
		}
	}
	return out
}

// cidToGIDRef matches the reference to a CIDToGIDMap stream.
var cidToGIDRef = regexp.MustCompile(`/CIDToGIDMap (\d+) 0 R`)

// TestTextSecondCID checks that a glyph of a TrueType font with a second
// text takes a second CID with its own entry in the ToUnicode map, and that
// a glyph without text takes an empty ActualText (D102).
func TestTextSecondCID(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	data := aliasGlyphs(t, font)
	alias := font.numGlyphs()

	if bytes.Contains(data, []byte("/ActualText (b)")) {
		t.Error("expected no ActualText for the second text of the glyph")
	}
	if !bytes.Contains(data, []byte("/ActualText ()")) {
		t.Error("expected an empty ActualText for the glyph without text")
	}
	for _, want := range []string{
		"<0001> <0061>",
		fmt.Sprintf("<%04X> <0062>", alias),
	} {
		if !bytes.Contains(data, []byte(want)) {
			t.Errorf("expected the ToUnicode entry %s", want)
		}
	}
	if want := []byte{0, 1, byte(alias >> 8), byte(alias)}; !bytes.Contains(shownStrings(data), want) {
		t.Errorf("expected the content stream to show the CIDs 1 and %d", alias)
	}
	if !bytes.Contains(data, fmt.Appendf(nil, "%d %d", alias, alias)) {
		t.Errorf("expected a width for the CID %d", alias)
	}

	m := cidToGIDRef.FindSubmatch(data)
	if m == nil {
		t.Fatal("expected a CIDToGIDMap stream")
	}
	start := bytes.Index(data, append(append([]byte("\n"), m[1]...), " 0 obj"...))
	stream := bytes.Index(data[start:], []byte("stream\n"))
	if start < 0 || stream < 0 {
		t.Fatalf("expected the object %s with a stream", m[1])
	}
	n, _ := strconv.Atoi(string(regexp.MustCompile(`/Length (\d+)`).FindSubmatch(data[start:])[1]))
	zr, err := zlib.NewReader(bytes.NewReader(data[start+stream+len("stream\n"):][:n]))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 2*(int(alias)+1) {
		t.Fatalf("expected a CIDToGIDMap of %d bytes, got %d", 2*(alias+1), len(body))
	}
	gid := func(cid int) int { return int(body[2*cid])<<8 | int(body[2*cid+1]) }
	if gid(1) != 1 || gid(2) != 2 || gid(3) != 0 || gid(int(alias)) != 1 {
		t.Errorf("expected the CIDs 1, 2, 3 and %d to map to 1, 2, 0 and 1, got %d, %d, %d and %d",
			alias, gid(1), gid(2), gid(3), gid(int(alias)))
	}
}

// TestTextSecondCIDCFF checks that a glyph of a CFF font, which has no
// CIDToGIDMap, keeps the ActualText of krilla for a second text (D102).
func TestTextSecondCIDCFF(t *testing.T) {
	data := aliasGlyphs(t, testFont(t, "LibertinusSerif-Regular.otf"))
	if !bytes.Contains(data, []byte("/ActualText (b)")) {
		t.Error("expected an ActualText for the second text of the glyph")
	}
	if cidToGIDRef.Match(data) {
		t.Error("expected no CIDToGIDMap stream")
	}
}

// TestTextSecondCIDKrilla checks that pdfcompat.Krilla turns D102 off.
func TestTextSecondCIDKrilla(t *testing.T) {
	pdfcompat.Krilla = true
	t.Cleanup(func() { pdfcompat.Krilla = false })
	data := aliasGlyphs(t, testFont(t, "NotoSans-Regular.ttf"))
	if !bytes.Contains(data, []byte("/ActualText (b)")) {
		t.Error("expected an ActualText for the second text of the glyph")
	}
	if bytes.Contains(data, []byte("/ActualText ()")) {
		t.Error("expected no empty ActualText")
	}
	if !bytes.Contains(data, []byte("/CIDToGIDMap /Identity")) {
		t.Error("expected the CIDToGIDMap Identity")
	}
}
