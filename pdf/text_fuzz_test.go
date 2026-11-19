// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage/internal/subsetter"
	"github.com/xo/okage/path"
)

// maxFuzzTextFont is the largest font that FuzzText reads. A subset copies
// the data of each glyph that a composite glyph uses, so its memory grows
// with the size of the font.
const maxFuzzTextFont = 1 << 15

// FuzzText gives malformed fonts to NewFont, and random runs of glyphs and
// texts to Surface.DrawGlyphs and Surface.DrawText (item 8 of "Testing"
// under "The PDF writer" in docs/PLAN.md). The first bytes of calls choose
// the index and the coordinates of the font, and the rest makes the calls.
// Neither NewFont, the calls nor Document.Finish must panic or hang.
func FuzzText(f *testing.F) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("testdata", "fonts", name))
		if err != nil {
			f.Fatal(err)
		}
		return data
	}
	calls := []byte{0, 0, 3, 1, 2, 0, 0x41, 0x42, 0x43, 0x20, 0x44, 1, 4, 9, 0, 3, 2, 1, 1, 7, 0, 1, 5, 2}
	f.Add(read("NotoSerifSC_subset1.ttf"), calls)
	for _, s := range []struct {
		name   string
		glyphs []uint16
	}{
		{"NotoSans-Regular.ttf", []uint16{36, 68, 69, 70, 3}},
		{"LatinModernRoman-Regular.otf", []uint16{5, 8, 290, 292}},
	} {
		sub, err := subsetter.Subset(read(s.name), 0, subsetter.NewGlyphRemapperFromGlyphs(s.glyphs))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(sub, calls)
		f.Add(sub, []byte{0, 1, 2, 0, 3, 0, 1, 2, 3, 4, 5, 6})
	}
	f.Add([]byte("OTTO\x00\x00"), calls)

	f.Fuzz(func(t *testing.T, data, calls []byte) {
		if len(data) > maxFuzzTextFont || len(calls) > 256 {
			return
		}
		r := &fuzzReader{data: calls}
		var opts []FontOption
		index := int(r.u8() % 2)
		if v := r.u8(); v != 0 {
			opts = append(opts, WithVariations(VariationCoord{Tag: Tag{'w', 'g', 'h', 't'}, Value: float32(v) * 4}))
		}
		font, err := NewFont(data, index, opts...)
		if err != nil {
			return
		}
		d := NewDocument(WithPDFVersion(PDFVersion(r.u8() % 6)))
		ps, _ := PageSettingsFromWH(200, 200)
		s := d.StartPage(ps).Surface()
		for range r.u8() % 8 {
			n := int(r.u8() % 16)
			text := string(calls[:min(len(calls), n)])
			start := path.PointFromXY(r.coord(), r.coord())
			size := r.coord()
			switch r.u8() % 6 {
			case 0:
				s.SetFill(&Fill{Paint: r.color(), Opacity: r.opacity()})
			case 1:
				k := DefaultStroke()
				k.Paint = r.color()
				k.Opacity = r.opacity()
				s.SetStroke(&k)
			case 2:
				s.PushIsolated()
				s.DrawText(start, font, size, text, r.u8()%4 == 0, TextDirection(r.u8()%5))
				s.Pop()
			case 3:
				s.DrawText(start, font, size, text, r.u8()%4 == 0, TextDirection(r.u8()%5))
			default:
				glyphs := make([]Glyph, r.u8()%8)
				for i := range glyphs {
					a, b := int(r.u8()%20), int(r.u8()%20)
					glyphs[i] = Glyph{
						GlyphID:   GlyphID(r.u8()) | GlyphID(r.u8()%2)<<8,
						TextRange: TextRange{Start: a, End: b},
						XAdvance:  r.coord(),
						XOffset:   r.coord(),
						YOffset:   r.coord(),
						YAdvance:  r.coord(),
					}
				}
				s.DrawGlyphs(start, glyphs, font, text, size, r.u8()%4 == 0)
			}
		}
		s.Finish()
		var buf bytes.Buffer
		if err := d.Finish(&buf); err != nil {
			return
		}
		if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
			t.Fatalf("the file starts with %q", buf.Bytes()[:min(buf.Len(), 8)])
		}
	})
}
