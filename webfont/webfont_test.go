package webfont

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/okage/typesetting/font"
	"github.com/xo/okage/typesetting/font/opentype"

	"github.com/xo/okage"
)

// The test fonts come from make.py in testdata. Each converted file is
// compared with the sfnt file that it was made from.
func TestConvert(t *testing.T) {
	for _, test := range []struct {
		name   string
		src    string
		format Format
	}{
		{"noto.ttf", "noto.ttf", FormatSFNT},
		{"noto.woff", "noto.ttf", FormatWOFF},
		{"noto.woff2", "noto.ttf", FormatWOFF2},
		{"noto-null.woff2", "noto.ttf", FormatWOFF2},
		{"cff.woff2", "cff.otf", FormatWOFF2},
		{"collection.woff2", "collection.ttc", FormatWOFF2},
		{"noto.eot", "noto.ttf", FormatEOT},
		{"noto-xor.eot", "noto.ttf", FormatEOT},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := readTestdata(t, test.name)
			if got := DetectFormat(data); got != test.format {
				t.Fatalf("expected format %d, got %d", test.format, got)
			}
			got, err := Convert(data)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			exp := readTestdata(t, test.src)
			compareSFNT(t, exp, got)
			// go-text loads each face of the result.
			lds, err := opentype.NewLoaders(bytes.NewReader(got))
			if err != nil {
				t.Fatalf("go-text: %v", err)
			}
			for i, ld := range lds {
				if _, err := font.NewFont(ld); err != nil {
					t.Errorf("go-text: face %d: %v", i, err)
				}
			}
		})
	}
}

// TestConvertChecksum checks the checksum of a converted single font: the
// sum of the file is 0xB1B0AFBA, as the field checkSumAdjustment of head
// sets it.
func TestConvertChecksum(t *testing.T) {
	for _, name := range []string{"noto.woff", "noto.woff2", "cff.woff2"} {
		got, err := Convert(readTestdata(t, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if sum := checksum(got); sum != 0xb1b0afba {
			t.Errorf("%s: expected the sum 0xb1b0afba, got %#x", name, sum)
		}
	}
}

func TestConvertErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		data []byte
		exp  error
	}{
		{"empty", nil, okage.ErrFontUnsupported},
		{"png", []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"), okage.ErrFontUnsupported},
		{"mtx eot", readTestdata(t, "noto-mtx.eot"), okage.ErrFontUnsupported},
		{"woff header", []byte("wOFF\x00\x01\x00\x00"), okage.ErrFontData},
		{"woff2 header", []byte("wOF2\x00\x01\x00\x00"), okage.ErrFontData},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Convert(test.data); !errors.Is(err, test.exp) {
				t.Errorf("expected %v, got %v", test.exp, err)
			}
		})
	}
}

// TestConvertCorrupted cuts each web font short, and changes bytes of it,
// and checks that Convert returns an error or a result without a panic.
func TestConvertCorrupted(t *testing.T) {
	for _, name := range []string{"noto.woff", "noto.woff2", "noto-null.woff2", "cff.woff2", "collection.woff2", "noto.eot"} {
		data := readTestdata(t, name)
		for _, n := range []int{0, 3, 20, 47, 48, 60, 100, len(data) / 2, len(data) - 1} {
			if _, err := Convert(data[:n]); err == nil {
				t.Errorf("%s cut to %d bytes: expected an error", name, n)
			}
		}
		for i := 4; i < len(data); i += 97 {
			b := slices.Clone(data)
			b[i] ^= 0xff
			_, _ = Convert(b)
		}
	}
}

// FuzzConvert checks that Convert does not panic. The seeds are the test
// fonts.
func FuzzConvert(f *testing.F) {
	for _, name := range []string{"noto.woff", "noto.woff2", "cff.woff2", "collection.woff2", "noto.eot"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Convert(data)
	})
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// parseSFNT returns the tables of each font of an sfnt file or collection,
// by tag.
func parseSFNT(t *testing.T, data []byte) []map[string][]byte {
	t.Helper()
	be := binary.BigEndian
	offsets := []int{0}
	if be.Uint32(data) == sigCollection {
		n := int(be.Uint32(data[8:]))
		offsets = offsets[:0]
		for i := range n {
			offsets = append(offsets, int(be.Uint32(data[12+4*i:])))
		}
	}
	var fonts []map[string][]byte
	for _, o := range offsets {
		d := data[o:]
		n := int(be.Uint16(d[4:]))
		tables := make(map[string][]byte, n)
		for i := range n {
			e := d[12+16*i:]
			off, length := int(be.Uint32(e[8:])), int(be.Uint32(e[12:]))
			tables[string(e[:4])] = data[off : off+length]
		}
		fonts = append(fonts, tables)
	}
	return fonts
}

// compareSFNT compares the fonts of got with the fonts of exp. The tables
// are the same bytes, except that checkSumAdjustment and the bit 11 of the
// flags of head can differ,
// and that glyf and loca are compared glyph by glyph.
func compareSFNT(t *testing.T, exp, got []byte) {
	t.Helper()
	expFonts, gotFonts := parseSFNT(t, exp), parseSFNT(t, got)
	if len(expFonts) != len(gotFonts) {
		t.Fatalf("expected %d fonts, got %d", len(expFonts), len(gotFonts))
	}
	for i, e := range expFonts {
		g := gotFonts[i]
		for tag, eb := range e {
			gb, ok := g[tag]
			switch {
			case !ok:
				t.Errorf("font %d: no table %q", i, tag)
			case tag == "head":
				// A WOFF2 encoder sets the bit 11 of the flags, at byte 16,
				// for a font with transformed tables.
				eb, gb = slices.Clone(eb), slices.Clone(gb)
				for _, b := range [][]byte{eb, gb} {
					clear(b[8:12])
					b[16] &^= 0x08
				}
				if !bytes.Equal(eb, gb) {
					t.Errorf("font %d: the table head differs", i)
				}
			case tag == "glyf", tag == "loca":
			case !bytes.Equal(eb, gb):
				t.Errorf("font %d: the table %q differs", i, tag)
			}
		}
		if len(g) != len(e) {
			t.Errorf("font %d: expected %d tables, got %d", i, len(e), len(g))
		}
		if _, ok := e["glyf"]; ok {
			eg, gg := glyphs(t, e), glyphs(t, g)
			if len(eg) != len(gg) {
				t.Fatalf("font %d: expected %d glyphs, got %d", i, len(eg), len(gg))
			}
			for j := range eg {
				if eg[j] != gg[j] {
					t.Errorf("font %d: glyph %d differs:\nexpected %s\ngot      %s", i, j, eg[j], gg[j])
				}
			}
		}
	}
}

// glyphs returns a text for each glyph of the tables glyf and loca, with its
// contours, its points, its bounding box and its instructions, so that two
// encodings of the same glyph give the same text.
func glyphs(t *testing.T, tables map[string][]byte) []string {
	t.Helper()
	be := binary.BigEndian
	head, maxp, loca, glyf := tables["head"], tables["maxp"], tables["loca"], tables["glyf"]
	long := be.Uint16(head[50:]) == 1
	n := int(be.Uint16(maxp[4:]))
	offset := func(i int) int {
		if long {
			return int(be.Uint32(loca[4*i:]))
		}
		return 2 * int(be.Uint16(loca[2*i:]))
	}
	out := make([]string, n)
	for i := range n {
		g := glyf[offset(i):offset(i+1)]
		if len(g) == 0 {
			continue
		}
		contours := int16(be.Uint16(g))
		s := fmt.Sprintf("contours %d bbox %v", contours, g[2:10])
		if contours < 0 {
			// A composite glyph: its components and instructions as bytes,
			// without the padding.
			out[i] = s + fmt.Sprintf(" composite %x", bytes.TrimRight(g[10:], "\x00"))
			continue
		}
		p := g[10:]
		endPts := make([]int, contours)
		for j := range endPts {
			endPts[j] = int(be.Uint16(p[2*j:]))
		}
		p = p[2*contours:]
		ilen := int(be.Uint16(p))
		instr := p[2 : 2+ilen]
		p = p[2+ilen:]
		points := 0
		if contours > 0 {
			points = endPts[contours-1] + 1
		}
		flags := make([]byte, 0, points)
		for len(flags) < points {
			f := p[0]
			p = p[1:]
			flags = append(flags, f)
			if f&glyfRepeat != 0 {
				for range p[0] {
					flags = append(flags, f)
				}
				p = p[1:]
			}
		}
		read := func(short, same byte) []int {
			vs := make([]int, points)
			v := 0
			for j, f := range flags {
				switch {
				case f&short != 0:
					d := int(p[0])
					p = p[1:]
					if f&same == 0 {
						d = -d
					}
					v += d
				case f&same == 0:
					v += int(int16(be.Uint16(p)))
					p = p[2:]
				}
				vs[j] = v
			}
			return vs
		}
		xs := read(glyfXShort, glyfXSameOrPos)
		ys := read(glyfYShort, glyfYSameOrPos)
		pts := make([]string, points)
		for j, f := range flags {
			pts[j] = fmt.Sprintf("%d,%d,%d", xs[j], ys[j], f&glyfOnCurve)
		}
		overlap := points > 0 && flags[0]&glyfOverlapSimple != 0
		out[i] = s + fmt.Sprintf(" ends %v instr %x overlap %t points %v", endPts, instr, overlap, pts)
	}
	return out
}
