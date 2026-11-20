// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file cover the fallback of D80: a font whose subset
// fails is drawn as paths, with its text as invisible text.

// brokenFont returns the data of NotoSans-Regular.ttf of testdata/fonts with
// a table post that the subsetter refuses: the table has the version 2, and
// its number of glyph indexes is 0, so the subsetter finds no name for a
// glyph and returns okage.ErrSubsetMalformedFont. typesetting reads the
// outlines of the font as before.
func brokenFont(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "fonts", "NotoSans-Regular.ttf"))
	if err != nil {
		tb.Fatal(err)
	}
	numTables := int(binary.BigEndian.Uint16(data[4:]))
	for i := range numTables {
		record := data[12+16*i:]
		if string(record[:4]) != "post" {
			continue
		}
		offset := int(binary.BigEndian.Uint32(record[8:]))
		post := data[offset:]
		if binary.BigEndian.Uint32(post) != 0x00020000 {
			tb.Fatal("the table post of NotoSans-Regular.ttf does not have the version 2")
		}
		binary.BigEndian.PutUint16(post[32:], 0)
		return data
	}
	tb.Fatal("NotoSans-Regular.ttf has no table post")
	return nil
}

// fallbackDocument draws the text "hi there" with the font: once filled,
// once stroked, and once filled in an isolated group. It returns the file of
// Document.Finish and its error.
func fallbackDocument(t *testing.T, font *Font, opts ...Option) ([]byte, error) {
	t.Helper()
	d := NewDocument(opts...)
	ps, _ := PageSettingsFromWH(200, 200)
	s := d.StartPage(ps).Surface()
	s.DrawText(path.PointFromXY(10, 50), font, 16, "hi there", false, TextDirectionAuto)
	stroke := DefaultStroke()
	s.SetStroke(&stroke)
	s.DrawText(path.PointFromXY(10, 100), font, 16, "hi there", false, TextDirectionAuto)
	s.SetStroke(nil)
	s.PushIsolated()
	s.DrawText(path.PointFromXY(10, 150), font, 16, "hi there", false, TextDirectionAuto)
	s.Pop()
	var buf bytes.Buffer
	err := d.Finish(&buf)
	return buf.Bytes(), err
}

// TestBrokenFont checks that the subsetter refuses the font of brokenFont.
func TestBrokenFont(t *testing.T) {
	font, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fallbackDocument(t, font, WithSubsetFallback(false))
	if !errors.Is(err, okage.ErrPDFFont) || !errors.Is(err, okage.ErrSubsetMalformedFont) {
		t.Fatalf("Finish gives %v, want %v and %v", err, okage.ErrPDFFont, okage.ErrSubsetMalformedFont)
	}
}

// TestSubsetFallback draws the text of a font whose subset fails, with each
// setting of the streams, and checks the paths and the invisible text.
func TestSubsetFallback(t *testing.T) {
	font, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{"plain", []Option{WithCompression(false), WithPretty(true)}},
		{"compressed", nil},
		{"ascii", []Option{WithASCIICompatible(true)}},
		{"ascii plain", []Option{WithASCIICompatible(true), WithCompression(false)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := fallbackDocument(t, font, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(data, []byte("/FontFile2")) {
				t.Error("the file embeds the font")
			}
			if !bytes.Contains(data, []byte("/BaseFont /NotoSans-Regular")) && !bytes.Contains(data, []byte("/BaseFont/NotoSans-Regular")) {
				t.Error("the font does not have the name NotoSans-Regular without a tag")
			}
			plain, err := fallbackDocument(t, font, WithCompression(false))
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"\n3 Tr\n", "\n0 Tr\n", "\n1 Tr\n", "\nf\n", "\nS\n"} {
				if !bytes.Contains(plain, []byte(want)) {
					t.Errorf("the file has no %q", want)
				}
			}
		})
	}
}

// TestSubsetFallbackContent checks the content of a page whose text is
// paths: the run of text draws nothing, the mode comes back after it, and
// the paths of the glyphs follow the text object.
func TestSubsetFallbackContent(t *testing.T) {
	font, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fallbackDocument(t, font, WithCompression(false), WithPretty(true))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	// The first text object of the page: a fill of the 8 glyphs, of which
	// 7 have an outline.
	start := strings.Index(text, "BT\n")
	end := strings.Index(text[start:], "\nQ") + start
	if start < 0 || end < start {
		t.Fatal("the file has no text object")
	}
	run := text[start:end]
	for _, want := range []string{"0 Tr\n/f0 16 Tf\n", "3 Tr\n[", "] TJ\n0 Tr\nET\n"} {
		if !strings.Contains(run, want) {
			t.Errorf("the text object has no %q:\n%s", want, run)
		}
	}
	if got := strings.Count(run, "\nf"); got != 7 {
		t.Errorf("the text object has %d fills, want 7:\n%s", got, run)
	}
}

// TestSubsetFallbackKeepsBytes checks that the fallback changes no byte of
// a document whose fonts give their subsets.
func TestSubsetFallbackKeepsBytes(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	for _, opts := range [][]Option{nil, {WithCompression(false), WithPretty(true)}, {WithASCIICompatible(true)}} {
		on, err := fallbackDocument(t, font, opts...)
		if err != nil {
			t.Fatal(err)
		}
		off, err := fallbackDocument(t, font, append(opts, WithSubsetFallback(false))...)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(on, off) {
			t.Error("the fallback changes the bytes of a document whose fonts give their subsets")
		}
	}
}

// TestSubsetFallbackTwoFonts draws a font whose subset fails and one whose
// subset does not fail on one page. Only the first one becomes paths.
func TestSubsetFallbackTwoFonts(t *testing.T) {
	broken, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	good := testFont(t, "DejaVuSansMono.ttf")
	d := NewDocument(WithCompression(false), WithPretty(true))
	ps, _ := PageSettingsFromWH(200, 200)
	s := d.StartPage(ps).Surface()
	s.DrawText(path.PointFromXY(10, 50), good, 16, "good", false, TextDirectionAuto)
	s.DrawText(path.PointFromXY(10, 100), broken, 16, "hi", false, TextDirectionAuto)
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	text := buf.String()
	if strings.Count(text, "/FontFile2") != 1 {
		t.Error("the file does not embed exactly one font")
	}
	if strings.Count(text, "\n3 Tr\n") != 1 {
		t.Errorf("the file has %d invisible runs, want 1", strings.Count(text, "\n3 Tr\n"))
	}
	if !strings.Contains(text, "/f0 16 Tf\n1 0 0 -1 10 50 Tm\n") {
		t.Error("the text of the good font changed")
	}
}

// recordHandler keeps the records that it handles.
type recordHandler struct {
	records []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordHandler) WithGroup(string) slog.Handler { return h }

// TestSubsetFallbackLogs checks the record of the logger for a font whose
// subset fails.
func TestSubsetFallbackLogs(t *testing.T) {
	font, err := NewFont(brokenFont(t), 0)
	if err != nil {
		t.Fatal(err)
	}
	h := &recordHandler{}
	if _, err := fallbackDocument(t, font, WithLogger(slog.New(h))); err != nil {
		t.Fatal(err)
	}
	if len(h.records) != 1 {
		t.Fatalf("the logger has %d records, want 1", len(h.records))
	}
	r := h.records[0]
	if r.Level != slog.LevelWarn {
		t.Errorf("the record has the level %v, want %v", r.Level, slog.LevelWarn)
	}
	attrs := map[string]string{}
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.String()
		return true
	})
	if attrs["font"] != "NotoSans-Regular" {
		t.Errorf("the record names the font %q, want NotoSans-Regular", attrs["font"])
	}
	if !strings.Contains(attrs["err"], okage.ErrSubsetMalformedFont.Error()) {
		t.Errorf("the record has the error %q", attrs["err"])
	}
}

// TestCheckGlyphs checks that CheckGlyphs returns the error that
// Document.Finish returns for the same glyphs.
func TestCheckGlyphs(t *testing.T) {
	font := testFont(t, "NotoSans-Regular.ttf")
	for _, tc := range []struct {
		name   string
		glyphs []Glyph
		text   string
		ok     bool
	}{
		{"valid", []Glyph{{GlyphID: 1, TextRange: TextRange{0, 3}}}, "hé", false},
		{"empty", []Glyph{{GlyphID: 1, TextRange: TextRange{1, 1}}}, "hi", false},
		{"outside", []Glyph{{GlyphID: 1, TextRange: TextRange{0, 3}}}, "hi", true},
		{"reversed", []Glyph{{GlyphID: 1, TextRange: TextRange{2, 1}}}, "hi", true},
		{"negative", []Glyph{{GlyphID: 1, TextRange: TextRange{-1, 1}}}, "hi", true},
		{"inside a character", []Glyph{{GlyphID: 1, TextRange: TextRange{0, 2}}}, "hé", true},
		{"large glyph", []Glyph{{GlyphID: 0x10000, TextRange: TextRange{0, 1}}}, "h", true},
	} {
		err := CheckGlyphs(tc.glyphs, tc.text)
		if (err != nil) != tc.ok {
			t.Errorf("%s: CheckGlyphs gives %v", tc.name, err)
			continue
		}
		if err != nil && !errors.Is(err, okage.ErrPDFInvalidGlyph) {
			t.Errorf("%s: %v is not %v", tc.name, err, okage.ErrPDFInvalidGlyph)
		}
		d := NewDocument()
		ps, _ := PageSettingsFromWH(200, 200)
		d.StartPage(ps).Surface().DrawGlyphs(path.PointFromXY(0, 50), tc.glyphs, font, tc.text, 12, false)
		finishErr := d.Finish(&bytes.Buffer{})
		if err == nil && finishErr != nil || err != nil && (finishErr == nil || finishErr.Error() != err.Error()) {
			t.Errorf("%s: Finish gives %v, and CheckGlyphs gives %v", tc.name, finishErr, err)
		}
	}
}

// TestOutlineTextLexer checks the strings and the operators that
// outlineText reads, and the content that it does not change.
func TestOutlineTextLexer(t *testing.T) {
	for _, tc := range []struct {
		content string
		want    []string
	}{
		{`(a\(b\)c\\\n\101\0) Tj`, []string{"string a(b)c\\\nA\x00", "operator Tj"}},
		{`(a(b)c)Tj`, []string{"string a(b)c", "operator Tj"}},
		{`<0041 0 >Tj`, []string{"string \x00A\x00", "operator Tj"}},
		{`/f0 12 Tf[(a)-5.5(b)]TJ`, []string{"name f0", "number 12", "operator Tf", "[", "string a", "number -5.5", "string b", "]", "operator TJ"}},
		{"/Span<</ActualText(x)>>BDC", []string{"name Span", "<<", "name ActualText", "string x", ">>", "operator BDC"}},
	} {
		l := contentLexer{data: []byte(tc.content)}
		var got []string
		for {
			tok, ok := l.next()
			if !ok {
				break
			}
			switch tok.kind {
			case tokenString:
				got = append(got, "string "+string(tok.text))
			case tokenName:
				got = append(got, "name "+string(tok.text))
			case tokenNumber:
				got = append(got, "number "+tc.content[tok.start:tok.end])
			case tokenOperator:
				got = append(got, "operator "+string(tok.text))
			case tokenArrayStart:
				got = append(got, "[")
			case tokenArrayEnd:
				got = append(got, "]")
			case tokenDictStart:
				got = append(got, "<<")
			case tokenDictEnd:
				got = append(got, ">>")
			}
		}
		if l.bad {
			t.Errorf("%q: the lexer stops at %d", tc.content, l.pos)
		}
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("%q: the tokens are\n%q, want\n%q", tc.content, got, tc.want)
		}
	}

	for _, content := range []string{"q BI /W 1 ID \xff EI Q", "(a", "<0g>", "> x"} {
		if _, ok := outlineText([]byte(content), nil, nil); ok {
			t.Errorf("%q: outlineText changes the content", content)
		}
	}
	content := "q 1 0 0 RG BT /f0 12 Tf (a) Tj ET Q"
	got, ok := outlineText([]byte(content), map[string]*failedFont{}, nil)
	if !ok || string(got) != content {
		t.Errorf("outlineText changes the content without a failed font to %q", got)
	}
}

// FuzzOutlineText gives random content to outlineText, with the font f0
// whose subset failed. It must not panic, and it must keep the content of
// a stream that shows no text.
func FuzzOutlineText(f *testing.F) {
	font, err := NewFont(brokenFont(f), 0)
	if err != nil {
		f.Fatal(err)
	}
	cid := newCIDFont(font)
	for g := range GlyphID(80) {
		cid.addGlyph(g)
	}
	fonts := map[string]*failedFont{"f0": newFailedFont(cid)}
	f.Add([]byte("q BT 0 Tr /f0 16 Tf 1 0 0 -1 10 50 Tm [(\x00\x01\x00\x02) -20 <0003>] TJ ET Q"))
	f.Add([]byte("BT 2 Tr /f0 9 Tf (\x00\x04) Tj /Span<</ActualText(a)>>BDC 1 Tr [(\x00\x05)] TJ EMC ET"))
	f.Add([]byte("q 1 0 0 RG 0 0 10 10 re f Q"))
	sc := newSerializeContext(defaultSerializeSettings())
	f.Fuzz(func(t *testing.T, content []byte) {
		got, ok := outlineText(content, fonts, sc)
		if ok && !bytes.Contains(content, []byte("Tj")) && !bytes.Contains(content, []byte("TJ")) && !bytes.Equal(got, content) {
			t.Errorf("outlineText changes %q to %q", content, got)
		}
	})
}
