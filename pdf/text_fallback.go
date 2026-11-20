// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"compress/zlib"
	"io"
	"strconv"

	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// This file ports no file of krilla. It draws the text of a CID font whose subset
// fails as paths (D80).
//
// The content streams are written before Document.Finish makes the subsets,
// and they are compressed. The text of a font must then change in the
// streams that show it, and only there, so that every other byte of the file
// stays the bytes of krilla. Each stream that names a font is registered
// with the function that writes its object. When the subset of a font
// fails, the CID font is written without its font program, and
// outlineFailedFonts then reads each stream that shows the font from its
// chunk, changes the text of the font, and writes the stream again into the
// same chunk. In each text object, a run of the font gets the text rendering
// mode 3, which draws nothing, and the glyphs of the run follow the operator
// ET as paths, with the fill and the stroke of the text.

// textStream is a content stream that names a font in its resources.
type textStream struct {
	// chunk holds the object of the stream, and no other object.
	chunk *pdfwriter.Chunk
	// fonts holds the references of the fonts of the resources, and
	// prefix the prefix of their names.
	fonts  []pdfwriter.Ref
	prefix string
	// write writes the object of the stream with the content into a chunk.
	write func(chunk *pdfwriter.Chunk, content []byte)
}

// registerTextStream keeps a content stream whose resources name a font, so
// that outlineFailedFonts can write it again. chunk holds the object of the
// stream only, and write writes it.
func (sc *serializeContext) registerTextStream(chunk *pdfwriter.Chunk, resources *resourceDictionary, write func(chunk *pdfwriter.Chunk, content []byte)) {
	if !sc.serializeSettings().subsetFallback || resources.fonts.len() == 0 {
		return
	}
	sc.textStreams = append(sc.textStreams, textStream{
		chunk:  chunk,
		fonts:  append([]pdfwriter.Ref(nil), resources.fonts.entries...),
		prefix: resources.prefix,
		write:  write,
	})
}

// failFont records a CID font whose subset failed, with the reference of its
// font object, and logs the error.
func (sc *serializeContext) failFont(f *cidFont, ref pdfwriter.Ref, err error) {
	if sc.failedFonts == nil {
		sc.failedFonts = map[pdfwriter.Ref]*failedFont{}
	}
	sc.failedFonts[ref] = newFailedFont(f)
	if logger := sc.serializeSettings().logger; logger != nil {
		name, _ := f.f.postscriptName()
		logger.Warn("pdf: the glyphs of a font are paths, because its subset failed", "font", name, "err", err)
	}
}

// outlineFailedFonts writes each content stream that shows the text of a
// font whose subset failed again, with the glyphs of that font as paths.
func (sc *serializeContext) outlineFailedFonts() {
	streams := sc.textStreams
	sc.textStreams = nil
	if len(sc.failedFonts) == 0 {
		return
	}
	settings := sc.serializeSettings()
	for _, ts := range streams {
		fonts := map[string]*failedFont{}
		for i, ref := range ts.fonts {
			if f, ok := sc.failedFonts[ref]; ok {
				fonts[resourceName(resourceFont, uint32(i), ts.prefix)] = f
			}
		}
		if len(fonts) == 0 {
			continue
		}
		content, ok := decodeContentStream(ts.chunk.AsBytes(), settings)
		if !ok {
			continue
		}
		content, ok = outlineText(content, fonts, sc)
		if !ok {
			continue
		}
		chunk := sc.newChunk()
		ts.write(chunk, content)
		*ts.chunk = *chunk
	}
}

// failedFont is a CID font whose subset failed, with the glyph ID of each
// CID, and the outline of each glyph that a stream drew, or nil for a glyph
// without an outline.
type failedFont struct {
	font  *cidFont
	gids  []uint16
	paths map[GlyphID]*Path
}

// newFailedFont returns the failed font of f.
func newFailedFont(f *cidFont) *failedFont {
	var gids []uint16
	for g := range f.glyphRemapper.RemappedGids() {
		gids = append(gids, g)
	}
	return &failedFont{font: f, gids: gids, paths: map[GlyphID]*Path{}}
}

// path returns a copy of the outline of a glyph in font units.
func (f *failedFont) path(glyph GlyphID) (*Path, bool) {
	p, seen := f.paths[glyph]
	if !seen {
		if q, ok := glyphPath(f.font.f, glyph); ok {
			p = q
		}
		f.paths[glyph] = p
	}
	if p == nil {
		return nil, false
	}
	return p.Clone(), true
}

// decodeContentStream returns the content of the stream of the object of a
// chunk that holds one object. The stream has the filters that
// newFilterStreamFromContentStream gives for the settings.
func decodeContentStream(object []byte, settings *serializeSettings) ([]byte, bool) {
	const start = ">>\nstream\n"
	suffix := "\nendstream\nendobj\n"
	if settings.pretty {
		suffix += "\n"
	}
	i := bytes.Index(object, []byte(start))
	if i < 0 || !bytes.HasSuffix(object, []byte(suffix)) || i+len(start) > len(object)-len(suffix) {
		return nil, false
	}
	data := object[i+len(start) : len(object)-len(suffix)]
	if !settings.compressContentStreams {
		return data, true
	}
	if settings.asciiCompatible {
		var ok bool
		if data, ok = hexDecode(data); !ok {
			return nil, false
		}
	}
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, false
	}
	return content, true
}

// hexDecode decodes the data of hexEncode.
func hexDecode(data []byte) ([]byte, bool) {
	out := make([]byte, 0, len(data)/2)
	var hi byte
	half := false
	for _, c := range data {
		v, ok := hexValue(c)
		switch {
		case ok && half:
			out = append(out, hi<<4|v)
			half = false
		case ok:
			hi, half = v, true
		case !isPDFWhitespace(c):
			return nil, false
		}
	}
	return out, !half
}

// hexValue returns the value of a hex digit.
func hexValue(c byte) (byte, bool) {
	switch {
	case '0' <= c && c <= '9':
		return c - '0', true
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10, true
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10, true
	}
	return 0, false
}

// isPDFWhitespace reports whether c is a white-space character of PDF.
func isPDFWhitespace(c byte) bool {
	switch c {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

// isPDFDelimiter reports whether c is a delimiter character of PDF.
func isPDFDelimiter(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}

// contentTokenKind is the kind of a token of a content stream.
type contentTokenKind uint8

// The kinds of tokens.
const (
	tokenOther contentTokenKind = iota
	tokenNumber
	tokenName
	tokenString
	tokenArrayStart
	tokenArrayEnd
	tokenDictStart
	tokenDictEnd
	tokenOperator
)

// contentToken is a token of a content stream, with its bytes from start to
// end. A string holds its decoded bytes, a name its text without the slash,
// and a number its value.
type contentToken struct {
	kind       contentTokenKind
	start, end int
	text       []byte
	number     float32
}

// contentLexer splits a content stream into tokens.
type contentLexer struct {
	data []byte
	pos  int
	// bad is true after data that is not valid.
	bad bool
}

// next returns the next token, or false at the end of the data or at data
// that is not valid, which sets bad.
func (l *contentLexer) next() (contentToken, bool) {
	data := l.data
	for l.pos < len(data) {
		c := data[l.pos]
		if isPDFWhitespace(c) {
			l.pos++
			continue
		}
		if c == '%' {
			for l.pos < len(data) && data[l.pos] != '\n' && data[l.pos] != '\r' {
				l.pos++
			}
			continue
		}
		break
	}
	if l.pos >= len(data) {
		return contentToken{}, false
	}
	start := l.pos
	tok := contentToken{start: start}
	fail := func() (contentToken, bool) {
		l.bad = true
		return contentToken{}, false
	}
	switch c := data[l.pos]; {
	case c == '(':
		text, ok := l.literalString()
		if !ok {
			return fail()
		}
		tok.kind, tok.text = tokenString, text
	case c == '<' && l.pos+1 < len(data) && data[l.pos+1] == '<':
		l.pos += 2
		tok.kind = tokenDictStart
	case c == '<':
		end := bytes.IndexByte(data[l.pos:], '>')
		if end < 0 {
			return fail()
		}
		text, ok := hexDecode(data[l.pos+1 : l.pos+end])
		if !ok {
			// A hex string with an odd number of digits ends with a 0.
			text, ok = hexDecode(append(append([]byte(nil), data[l.pos+1:l.pos+end]...), '0'))
		}
		if !ok {
			return fail()
		}
		l.pos += end + 1
		tok.kind, tok.text = tokenString, text
	case c == '>':
		if l.pos+1 >= len(data) || data[l.pos+1] != '>' {
			return fail()
		}
		l.pos += 2
		tok.kind = tokenDictEnd
	case c == '[':
		l.pos++
		tok.kind = tokenArrayStart
	case c == ']':
		l.pos++
		tok.kind = tokenArrayEnd
	case c == '/':
		l.pos++
		l.regular()
		tok.kind, tok.text = tokenName, data[start+1:l.pos]
	case c == ')' || c == '{' || c == '}':
		l.pos++
	default:
		l.regular()
		word := data[start:l.pos]
		if v, err := strconv.ParseFloat(string(word), 32); err == nil && isNumberStart(word[0]) {
			tok.kind, tok.number = tokenNumber, float32(v)
		} else {
			tok.kind, tok.text = tokenOperator, word
		}
	}
	tok.end = l.pos
	return tok, true
}

// isNumberStart reports whether a number of PDF can start with c.
func isNumberStart(c byte) bool {
	return c == '+' || c == '-' || c == '.' || ('0' <= c && c <= '9')
}

// regular moves past the regular characters at the position.
func (l *contentLexer) regular() {
	for l.pos < len(l.data) && !isPDFWhitespace(l.data[l.pos]) && !isPDFDelimiter(l.data[l.pos]) {
		l.pos++
	}
}

// literalString reads a literal string at the position, and returns its
// decoded bytes.
func (l *contentLexer) literalString() ([]byte, bool) {
	data := l.data
	l.pos++
	var out []byte
	depth := 1
	for l.pos < len(data) {
		c := data[l.pos]
		l.pos++
		switch c {
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth == 0 {
				return out, true
			}
			out = append(out, c)
		case '\\':
			if l.pos >= len(data) {
				return nil, false
			}
			e := data[l.pos]
			l.pos++
			switch e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '\r':
				if l.pos < len(data) && data[l.pos] == '\n' {
					l.pos++
				}
			case '\n':
			default:
				if '0' <= e && e <= '7' {
					v := e - '0'
					for n := 1; n < 3 && l.pos < len(data) && '0' <= data[l.pos] && data[l.pos] <= '7'; n++ {
						v = v<<3 | (data[l.pos] - '0')
						l.pos++
					}
					out = append(out, v)
				} else {
					out = append(out, e)
				}
			}
		default:
			out = append(out, c)
		}
	}
	return nil, false
}

// textOutliner changes the text of the failed fonts of one content stream.
type textOutliner struct {
	sc      *serializeContext
	fonts   map[string]*failedFont
	content []byte
	// out holds the content up to the position copied, with the changes.
	out    []byte
	copied int
	// The state of the text: the text object is open, the font and its
	// size, the text matrix, the position on the line since the text
	// matrix, and the text rendering mode.
	inText bool
	font   *failedFont
	size   float32
	tm     Transform
	tx     float32
	mode   int
	// paths holds the paths of the glyphs of the open text object.
	paths *contentBuilder
}

// outlineText returns content with each run of text of the fonts invisible,
// and the glyphs of each run as paths after the end of its text object.
// fonts maps the name of each font of the resources whose subset failed. It
// returns false for content that it cannot read, such as an inline image.
func outlineText(content []byte, fonts map[string]*failedFont, sc *serializeContext) ([]byte, bool) {
	o := &textOutliner{sc: sc, fonts: fonts, content: content, out: make([]byte, 0, len(content)+len(content)/2)}
	l := contentLexer{data: content}
	var operands []contentToken
	// array holds the tokens of an array of the operands, and depth counts
	// the arrays and the dictionaries that are open.
	var array []contentToken
	inArray := false
	depth := 0
	for {
		tok, ok := l.next()
		if !ok {
			break
		}
		switch tok.kind {
		case tokenArrayStart, tokenDictStart:
			if depth == 0 && tok.kind == tokenArrayStart {
				inArray, array = true, array[:0]
				operands = append(operands, tok)
			}
			depth++
			continue
		case tokenArrayEnd, tokenDictEnd:
			depth--
			if depth == 0 {
				inArray = false
			}
			continue
		}
		if depth > 0 {
			if inArray && depth == 1 {
				array = append(array, tok)
			}
			continue
		}
		if tok.kind != tokenOperator || isOperandKeyword(tok.text) {
			operands = append(operands, tok)
			continue
		}
		if string(tok.text) == "BI" {
			return nil, false
		}
		o.operator(tok, operands, array)
		operands = operands[:0]
	}
	if l.bad {
		return nil, false
	}
	o.out = append(o.out, content[o.copied:]...)
	return o.out, true
}

// isOperandKeyword reports whether a keyword is an operand.
func isOperandKeyword(word []byte) bool {
	switch string(word) {
	case "true", "false", "null":
		return true
	}
	return false
}

// insert copies the content up to pos, and then text, with a new line
// before and after it where no white space separates it from the content.
// Each call has a position at or after the position of the call before.
func (o *textOutliner) insert(pos int, text []byte) {
	o.out = append(o.out, o.content[o.copied:pos]...)
	if n := len(o.out); n > 0 && !isPDFWhitespace(o.out[n-1]) {
		o.out = append(o.out, '\n')
	}
	o.out = append(o.out, text...)
	if pos < len(o.content) && !isPDFWhitespace(o.content[pos]) {
		o.out = append(o.out, '\n')
	}
	o.copied = pos
}

// operator applies one operator. operands holds its operands, and array
// the items of the array of its operands, if it has one.
func (o *textOutliner) operator(tok contentToken, operands, array []contentToken) {
	switch string(tok.text) {
	case "BT":
		o.inText = true
		o.tm = path.IdentityTransform()
		o.tx = 0
	case "ET":
		o.inText = false
		if o.paths != nil {
			buf := o.paths.content.Finish()
			o.paths = nil
			if len(buf.AsSlice()) != 0 {
				o.insert(tok.end, buf.AsSlice())
			}
		}
	case "Tf":
		if len(operands) == 2 && operands[0].kind == tokenName && operands[1].kind == tokenNumber {
			o.font = o.fonts[string(operands[0].text)]
			o.size = operands[1].number
		}
	case "Tm":
		if len(operands) == 6 {
			var v [6]float32
			for i, op := range operands {
				if op.kind != tokenNumber {
					return
				}
				v[i] = op.number
			}
			o.tm = path.TransformFromRow(v[0], v[1], v[2], v[3], v[4], v[5])
			o.tx = 0
		}
	case "Tr":
		if len(operands) == 1 && operands[0].kind == tokenNumber {
			o.mode = int(operands[0].number)
		}
	case "Tj", "TJ":
		if !o.inText || o.font == nil || len(operands) != 1 {
			return
		}
		items := array
		if string(tok.text) == "Tj" {
			if operands[0].kind != tokenString {
				return
			}
			items = operands[:1]
		} else if operands[0].kind != tokenArrayStart {
			return
		}
		// The run draws nothing, and the mode of the text comes back after
		// it.
		o.insert(operands[0].start, []byte("3 Tr"))
		o.insert(tok.end, []byte(strconv.Itoa(o.mode)+" Tr"))
		o.outline(items)
	}
}

// outline draws the glyphs of the items of a text showing operator as paths,
// and moves the position on the line. A string holds a CID of 2 bytes for
// each glyph, and a number moves the next glyph back by a thousandth of the
// size of the font.
func (o *textOutliner) outline(items []contentToken) {
	f := o.font.font
	upem := f.f.UnitsPerEm()
	if upem == 0 {
		return
	}
	for _, item := range items {
		switch item.kind {
		case tokenNumber:
			// The conversion rounds the product, so that the Go compiler
			// cannot fuse it with the difference (D16).
			o.tx -= float32(item.number / 1000 * o.size)
		case tokenString:
			for i := 0; i+1 < len(item.text); i += 2 {
				cid := uint16(item.text[i])<<8 | uint16(item.text[i+1])
				if int(cid) >= len(o.font.gids) || int(cid) >= len(f.widths) {
					continue
				}
				o.drawGlyph(GlyphID(o.font.gids[cid]), upem)
				o.tx += float32(f.widths[cid] * o.size / upem)
			}
		}
	}
}

// drawGlyph draws one glyph as a path at the position on the line, with
// the paint of the text rendering mode.
func (o *textOutliner) drawGlyph(glyph GlyphID, upem float32) {
	mode := o.mode % 4
	if mode == int(pdfwriter.TextRenderingModeInvisible) {
		return
	}
	p, ok := o.font.path(glyph)
	if !ok {
		return
	}
	scale := o.size / upem
	ts := o.tm.PreConcat(path.TransformFromTranslate(o.tx, 0)).PreConcat(path.TransformFromScale(scale, scale))
	if p, ok = p.Transform(ts); !ok {
		return
	}
	if o.paths == nil {
		o.paths = &contentBuilder{content: o.sc.newContent()}
	}
	o.paths.contentDrawPath(p.Segments())
	switch mode {
	case int(pdfwriter.TextRenderingModeFill):
		o.paths.content.FillNonzero()
	case int(pdfwriter.TextRenderingModeStroke):
		o.paths.content.Stroke()
	default:
		o.paths.content.FillNonzeroAndStroke()
	}
}
