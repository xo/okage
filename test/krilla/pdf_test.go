package krilla

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// The reader of this file reads the PDF files that krilla and the port write,
// and decodes their streams, so that two files compare by their objects
// (D62). It reads each object in the order of the file, as pdf-writer writes
// them. It does not read the cross-reference table, so it does not read a
// file that a program changed with an incremental update, or a file with
// object streams. Neither krilla nor pdf-writer writes such a file.

// pdfRef is the number and the generation of an indirect object.
type pdfRef struct {
	Num, Gen int
}

// pdfName is a name, without its slash and with its # escapes decoded.
type pdfName string

// pdfNumber is a number, as the file writes it. Two numbers compare by
// their text, so 1 and 1.0 differ, as they differ in the bytes of a writer.
type pdfNumber string

// pdfString is a string, with its escapes decoded. Hex records whether the
// file writes it in hex.
type pdfString struct {
	Bytes []byte
	Hex   bool
}

// pdfKeyword is true, false or null.
type pdfKeyword string

// pdfArray is an array.
type pdfArray []any

// pdfDict is a dictionary, with its keys in the order of the file.
type pdfDict struct {
	Keys   []pdfName
	Values map[pdfName]any
}

// get returns the value of a key, or nil.
func (d *pdfDict) get(k pdfName) any {
	if d == nil {
		return nil
	}
	return d.Values[k]
}

// set sets the value of a key, and keeps the order of the keys.
func (d *pdfDict) set(k pdfName, v any) {
	if _, ok := d.Values[k]; !ok {
		d.Keys = append(d.Keys, k)
	}
	d.Values[k] = v
}

// del removes a key.
func (d *pdfDict) del(k pdfName) {
	if _, ok := d.Values[k]; !ok {
		return
	}
	delete(d.Values, k)
	d.Keys = slices.DeleteFunc(d.Keys, func(x pdfName) bool { return x == k })
}

// pdfObject is an indirect object. A stream has its dictionary in Value, its
// data in Raw as the file holds it, and in Data as it is after the filters
// that the reader decodes. Filters holds the filters that the reader does
// not decode, such as DCTDecode, and Parms their parameters.
type pdfObject struct {
	Ref      pdfRef
	Value    any
	IsStream bool
	Raw      []byte
	Data     []byte
	Filters  []pdfName
	Parms    []any
	// DecodeErr is the error of a filter that does not decode.
	DecodeErr error
}

// pdfFile is a PDF file that readPDF reads.
type pdfFile struct {
	// Version is the version of the header, such as "1.7".
	Version string
	// Objects holds the objects in the order of the file.
	Objects []*pdfObject
	// Trailer is the trailer dictionary, or the dictionary of the last
	// cross-reference stream.
	Trailer *pdfDict
}

// object returns the object with a number, or nil.
func (f *pdfFile) object(num int) *pdfObject {
	for _, o := range f.Objects {
		if o.Ref.Num == num {
			return o
		}
	}
	return nil
}

// resolve returns the value of a reference, or the value itself.
func (f *pdfFile) resolve(v any) any {
	if r, ok := v.(pdfRef); ok {
		if o := f.object(r.Num); o != nil {
			return o.Value
		}
		return nil
	}
	return v
}

// readPDF reads a PDF file and decodes the data of each stream.
func readPDF(data []byte) (*pdfFile, error) {
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		return nil, errors.New("the file does not start with %PDF-")
	}
	end := bytes.IndexAny(data, "\r\n")
	if end < 0 {
		return nil, errors.New("the header has no end of line")
	}
	f := &pdfFile{Version: string(data[5:end])}
	lx := &lexer{data: data, pos: end}
	for {
		tok, err := lx.token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch {
		case tok.kind == tokNumber:
			o, err := readObject(lx, tok)
			if err != nil {
				return nil, err
			}
			f.Objects = append(f.Objects, o)
			if d, ok := o.Value.(*pdfDict); ok && o.IsStream && d.get("Type") == pdfName("XRef") {
				f.Trailer = d
			}
		case tok.kind == tokKeyword && tok.text == "xref":
			if err := skipXref(lx); err != nil {
				return nil, err
			}
		case tok.kind == tokKeyword && tok.text == "trailer":
			v, err := lx.value()
			if err != nil {
				return nil, fmt.Errorf("reading the trailer: %w", err)
			}
			d, ok := v.(*pdfDict)
			if !ok {
				return nil, errors.New("the trailer is not a dictionary")
			}
			f.Trailer = d
		case tok.kind == tokKeyword && tok.text == "startxref":
			if _, err := lx.token(); err != nil {
				return nil, fmt.Errorf("reading startxref: %w", err)
			}
		default:
			return nil, fmt.Errorf("offset %d: the token %q does not start an object", tok.pos, tok.text)
		}
	}
	if f.Trailer == nil {
		return nil, errors.New("the file has no trailer")
	}
	return f, nil
}

// readObject reads an indirect object. num is the token of its number.
func readObject(lx *lexer, num token) (*pdfObject, error) {
	gen, err := lx.token()
	if err != nil {
		return nil, err
	}
	kw, err := lx.token()
	if err != nil {
		return nil, err
	}
	if gen.kind != tokNumber || kw.kind != tokKeyword || kw.text != "obj" {
		return nil, fmt.Errorf("offset %d: %s is not followed by a generation and obj", num.pos, num.text)
	}
	n, err1 := strconv.Atoi(num.text)
	g, err2 := strconv.Atoi(gen.text)
	if err1 != nil || err2 != nil {
		return nil, fmt.Errorf("offset %d: the reference %s %s is not valid", num.pos, num.text, gen.text)
	}
	o := &pdfObject{Ref: pdfRef{n, g}}
	if o.Value, err = lx.value(); err != nil {
		return nil, fmt.Errorf("reading object %d: %w", n, err)
	}
	tok, err := lx.token()
	if err != nil {
		return nil, fmt.Errorf("reading object %d: %w", n, err)
	}
	if tok.kind == tokKeyword && tok.text == "stream" {
		d, ok := o.Value.(*pdfDict)
		if !ok {
			return nil, fmt.Errorf("object %d has a stream without a dictionary", n)
		}
		o.IsStream = true
		if o.Raw, err = lx.streamData(d); err != nil {
			return nil, fmt.Errorf("reading the stream of object %d: %w", n, err)
		}
		decode(o, d)
		if tok, err = lx.token(); err != nil {
			return nil, fmt.Errorf("reading object %d: %w", n, err)
		}
	}
	if tok.kind != tokKeyword || tok.text != "endobj" {
		return nil, fmt.Errorf("object %d does not end with endobj", n)
	}
	return o, nil
}

// skipXref skips the rows of a cross-reference table.
func skipXref(lx *lexer) error {
	for {
		save := lx.pos
		tok, err := lx.token()
		if err != nil {
			return fmt.Errorf("reading the cross-reference table: %w", err)
		}
		if tok.kind != tokNumber && (tok.kind != tokKeyword || (tok.text != "f" && tok.text != "n")) {
			lx.pos = save
			return nil
		}
	}
}

type tokenKind int

const (
	tokNumber tokenKind = iota
	tokName
	tokString
	tokKeyword
	tokDelim
)

type token struct {
	kind tokenKind
	text string
	str  pdfString
	pos  int
}

// lexer reads the tokens and the values of a PDF file.
type lexer struct {
	data []byte
	pos  int
}

func isSpace(c byte) bool {
	return c == 0 || c == '\t' || c == '\n' || c == '\f' || c == '\r' || c == ' '
}

func isDelim(c byte) bool {
	return strings.IndexByte("()<>[]{}/%", c) >= 0
}

// skip skips white space and comments.
func (lx *lexer) skip() {
	for lx.pos < len(lx.data) {
		c := lx.data[lx.pos]
		switch {
		case isSpace(c):
			lx.pos++
		case c == '%':
			for lx.pos < len(lx.data) && lx.data[lx.pos] != '\n' && lx.data[lx.pos] != '\r' {
				lx.pos++
			}
		default:
			return
		}
	}
}

// token reads the next token. It returns io.EOF at the end of the data.
func (lx *lexer) token() (token, error) {
	lx.skip()
	if lx.pos >= len(lx.data) {
		return token{}, io.EOF
	}
	start := lx.pos
	c := lx.data[lx.pos]
	switch c {
	case '/':
		lx.pos++
		var b []byte
		for lx.pos < len(lx.data) && !isSpace(lx.data[lx.pos]) && !isDelim(lx.data[lx.pos]) {
			ch := lx.data[lx.pos]
			if ch == '#' && lx.pos+2 < len(lx.data) {
				if v, err := strconv.ParseUint(string(lx.data[lx.pos+1:lx.pos+3]), 16, 8); err == nil {
					b = append(b, byte(v))
					lx.pos += 3
					continue
				}
			}
			b = append(b, ch)
			lx.pos++
		}
		return token{kind: tokName, text: string(b), pos: start}, nil
	case '(':
		s, err := lx.literal()
		return token{kind: tokString, str: s, pos: start}, err
	case '<':
		if lx.pos+1 < len(lx.data) && lx.data[lx.pos+1] == '<' {
			lx.pos += 2
			return token{kind: tokDelim, text: "<<", pos: start}, nil
		}
		s, err := lx.hexString()
		return token{kind: tokString, str: s, pos: start}, err
	case '>':
		if lx.pos+1 < len(lx.data) && lx.data[lx.pos+1] == '>' {
			lx.pos += 2
			return token{kind: tokDelim, text: ">>", pos: start}, nil
		}
		return token{}, fmt.Errorf("offset %d: a single >", start)
	case '[', ']', '{', '}':
		lx.pos++
		return token{kind: tokDelim, text: string(c), pos: start}, nil
	case ')':
		return token{}, fmt.Errorf("offset %d: a ) without (", start)
	}
	for lx.pos < len(lx.data) && !isSpace(lx.data[lx.pos]) && !isDelim(lx.data[lx.pos]) {
		lx.pos++
	}
	text := string(lx.data[start:lx.pos])
	if strings.ContainsAny(text[:1], "+-.0123456789") {
		return token{kind: tokNumber, text: text, pos: start}, nil
	}
	return token{kind: tokKeyword, text: text, pos: start}, nil
}

// literal reads a literal string.
func (lx *lexer) literal() (pdfString, error) {
	start := lx.pos
	lx.pos++
	var b []byte
	depth := 1
	for lx.pos < len(lx.data) {
		c := lx.data[lx.pos]
		lx.pos++
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return pdfString{Bytes: b}, nil
			}
		case '\\':
			if lx.pos >= len(lx.data) {
				return pdfString{}, fmt.Errorf("offset %d: the string has no end", start)
			}
			e := lx.data[lx.pos]
			lx.pos++
			switch e {
			case 'n':
				c = '\n'
			case 'r':
				c = '\r'
			case 't':
				c = '\t'
			case 'b':
				c = '\b'
			case 'f':
				c = '\f'
			case '\r':
				if lx.pos < len(lx.data) && lx.data[lx.pos] == '\n' {
					lx.pos++
				}
				continue
			case '\n':
				continue
			default:
				if e >= '0' && e <= '7' {
					v := int(e - '0')
					for i := 0; i < 2 && lx.pos < len(lx.data) && lx.data[lx.pos] >= '0' && lx.data[lx.pos] <= '7'; i++ {
						v = v*8 + int(lx.data[lx.pos]-'0')
						lx.pos++
					}
					c = byte(v)
				} else {
					c = e
				}
			}
		}
		b = append(b, c)
	}
	return pdfString{}, fmt.Errorf("offset %d: the string has no end", start)
}

// hexString reads a hex string.
func (lx *lexer) hexString() (pdfString, error) {
	start := lx.pos
	lx.pos++
	end := bytes.IndexByte(lx.data[lx.pos:], '>')
	if end < 0 {
		return pdfString{}, fmt.Errorf("offset %d: the hex string has no end", start)
	}
	var digits []byte
	for _, c := range lx.data[lx.pos : lx.pos+end] {
		if !isSpace(c) {
			digits = append(digits, c)
		}
	}
	lx.pos += end + 1
	if len(digits)%2 == 1 {
		digits = append(digits, '0')
	}
	b, err := hex.DecodeString(string(digits))
	if err != nil {
		return pdfString{}, fmt.Errorf("offset %d: %w", start, err)
	}
	return pdfString{Bytes: b, Hex: true}, nil
}

// value reads a value. A reference is two numbers and R.
func (lx *lexer) value() (any, error) {
	tok, err := lx.token()
	if err != nil {
		return nil, err
	}
	switch tok.kind {
	case tokName:
		return pdfName(tok.text), nil
	case tokString:
		return tok.str, nil
	case tokKeyword:
		switch tok.text {
		case "true", "false", "null":
			return pdfKeyword(tok.text), nil
		}
		return nil, fmt.Errorf("offset %d: the keyword %q is not a value", tok.pos, tok.text)
	case tokNumber:
		save := lx.pos
		gen, err1 := lx.token()
		r, err2 := lx.token()
		if err1 == nil && err2 == nil && gen.kind == tokNumber && r.kind == tokKeyword && r.text == "R" {
			n, err1 := strconv.Atoi(tok.text)
			g, err2 := strconv.Atoi(gen.text)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("offset %d: the reference is not valid", tok.pos)
			}
			return pdfRef{n, g}, nil
		}
		lx.pos = save
		return pdfNumber(tok.text), nil
	case tokDelim:
		switch tok.text {
		case "[":
			a := pdfArray{}
			for {
				save := lx.pos
				t, err := lx.token()
				if err != nil {
					return nil, err
				}
				if t.kind == tokDelim && t.text == "]" {
					return a, nil
				}
				lx.pos = save
				v, err := lx.value()
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
		case "<<":
			d := &pdfDict{Values: map[pdfName]any{}}
			for {
				t, err := lx.token()
				if err != nil {
					return nil, err
				}
				if t.kind == tokDelim && t.text == ">>" {
					return d, nil
				}
				if t.kind != tokName {
					return nil, fmt.Errorf("offset %d: a key of a dictionary is not a name", t.pos)
				}
				v, err := lx.value()
				if err != nil {
					return nil, err
				}
				d.set(pdfName(t.text), v)
			}
		}
	}
	return nil, fmt.Errorf("offset %d: the token %q is not a value", tok.pos, tok.text)
}

// streamData reads the data of a stream after the keyword stream. It takes
// the length from /Length when it is a number and the data ends there.
// Otherwise it looks for the keyword endstream.
func (lx *lexer) streamData(d *pdfDict) ([]byte, error) {
	switch {
	case bytes.HasPrefix(lx.data[lx.pos:], []byte("\r\n")):
		lx.pos += 2
	case bytes.HasPrefix(lx.data[lx.pos:], []byte("\n")):
		lx.pos++
	}
	start := lx.pos
	if n, ok := d.get("Length").(pdfNumber); ok {
		if l, err := strconv.Atoi(string(n)); err == nil && l >= 0 && start+l <= len(lx.data) {
			rest := bytes.TrimLeft(lx.data[start+l:], "\r\n")
			if bytes.HasPrefix(rest, []byte("endstream")) {
				lx.pos = len(lx.data) - len(rest) + len("endstream")
				return lx.data[start : start+l], nil
			}
		}
	}
	end := bytes.Index(lx.data[start:], []byte("endstream"))
	if end < 0 {
		return nil, errors.New("the stream has no endstream")
	}
	data := bytes.TrimSuffix(bytes.TrimSuffix(lx.data[start:start+end], []byte("\n")), []byte("\r"))
	lx.pos = start + end + len("endstream")
	return data, nil
}

// decode applies the filters of a stream that the reader knows, from the
// first one on: FlateDecode with its predictors, ASCIIHexDecode,
// ASCII85Decode and RunLengthDecode. It stops at the first filter that it
// does not know, and keeps that filter and the ones after it in Filters.
func decode(o *pdfObject, d *pdfDict) {
	var filters []pdfName
	var parms []any
	switch f := d.get("Filter").(type) {
	case pdfName:
		filters = []pdfName{f}
		parms = []any{d.get("DecodeParms")}
	case pdfArray:
		for i, x := range f {
			n, ok := x.(pdfName)
			if !ok {
				o.DecodeErr = errors.New("a filter is not a name")
				return
			}
			filters = append(filters, n)
			var p any
			if a, ok := d.get("DecodeParms").(pdfArray); ok && i < len(a) {
				p = a[i]
			}
			parms = append(parms, p)
		}
	}
	data := o.Raw
	for i, f := range filters {
		var err error
		switch f {
		case "FlateDecode":
			data, err = inflate(data)
			if err == nil {
				data, err = unpredict(data, parms[i])
			}
		case "ASCIIHexDecode":
			data, err = decodeASCIIHex(data)
		case "ASCII85Decode":
			data, err = decodeASCII85(data)
		case "RunLengthDecode":
			data, err = decodeRunLength(data)
		default:
			o.Data, o.Filters, o.Parms = data, filters[i:], parms[i:]
			return
		}
		if err != nil {
			o.DecodeErr = fmt.Errorf("decoding %s: %w", f, err)
			o.Data, o.Filters, o.Parms = o.Raw, filters, parms
			return
		}
	}
	o.Data = data
}

// inflate decodes zlib data, or raw deflate data when the zlib header is
// missing.
func inflate(data []byte) ([]byte, error) {
	if r, err := zlib.NewReader(bytes.NewReader(data)); err == nil {
		b, err := io.ReadAll(r)
		if err == nil {
			return b, nil
		}
	}
	return io.ReadAll(flate.NewReader(bytes.NewReader(data)))
}

// unpredict undoes the PNG predictors and the TIFF predictor 2 with 8 bits
// for each component, as the parameters of FlateDecode give them.
func unpredict(data []byte, parms any) ([]byte, error) {
	p, ok := parms.(*pdfDict)
	if !ok {
		return data, nil
	}
	num := func(k pdfName, def int) int {
		if n, ok := p.get(k).(pdfNumber); ok {
			if v, err := strconv.Atoi(string(n)); err == nil {
				return v
			}
		}
		return def
	}
	predictor := num("Predictor", 1)
	if predictor < 2 {
		return data, nil
	}
	colors, bpc, columns := num("Colors", 1), num("BitsPerComponent", 8), num("Columns", 1)
	bpp := max(1, colors*bpc/8)
	row := (colors*bpc*columns + 7) / 8
	if predictor == 2 {
		if bpc != 8 {
			return nil, fmt.Errorf("the TIFF predictor with %d bits is not supported", bpc)
		}
		out := slices.Clone(data)
		for y := 0; y+row <= len(out); y += row {
			for x := bpp; x < row; x++ {
				out[y+x] += out[y+x-bpp]
			}
		}
		return out, nil
	}
	var out []byte
	prev := make([]byte, row)
	for len(data) > 0 {
		if len(data) < row+1 {
			return nil, errors.New("a row of the PNG predictor is short")
		}
		kind, cur := data[0], slices.Clone(data[1:row+1])
		data = data[row+1:]
		for x := range cur {
			var a, c byte
			if x >= bpp {
				a, c = cur[x-bpp], prev[x-bpp]
			}
			b := prev[x]
			switch kind {
			case 0:
			case 1:
				cur[x] += a
			case 2:
				cur[x] += b
			case 3:
				cur[x] += byte((int(a) + int(b)) / 2)
			case 4:
				cur[x] += paeth(a, b, c)
			default:
				return nil, fmt.Errorf("the PNG filter %d is not known", kind)
			}
		}
		out = append(out, cur...)
		prev = cur
	}
	return out, nil
}

func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa, pb, pc := absInt(p-int(a)), absInt(p-int(b)), absInt(p-int(c))
	switch {
	case pa <= pb && pa <= pc:
		return a
	case pb <= pc:
		return b
	}
	return c
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func decodeASCIIHex(data []byte) ([]byte, error) {
	var digits []byte
	for _, c := range data {
		if c == '>' {
			break
		}
		if !isSpace(c) {
			digits = append(digits, c)
		}
	}
	if len(digits)%2 == 1 {
		digits = append(digits, '0')
	}
	return hex.DecodeString(string(digits))
}

func decodeASCII85(data []byte) ([]byte, error) {
	var out []byte
	var group [5]byte
	n := 0
	flush := func(k int) {
		v := uint32(0)
		for i := range 5 {
			v = v*85 + uint32(group[i])
		}
		b := []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
		out = append(out, b[:k]...)
	}
	for _, c := range data {
		switch {
		case isSpace(c):
		case c == '~':
			if n > 0 {
				for j := n; j < 5; j++ {
					group[j] = 84
				}
				flush(n - 1)
			}
			return out, nil
		case c == 'z' && n == 0:
			out = append(out, 0, 0, 0, 0)
		case c >= '!' && c <= 'u':
			group[n] = c - '!'
			n++
			if n == 5 {
				flush(4)
				n = 0
			}
		default:
			return nil, fmt.Errorf("the byte %q is not ASCII85", c)
		}
	}
	return nil, errors.New("the data has no ~>")
}

func decodeRunLength(data []byte) ([]byte, error) {
	var out []byte
	for i := 0; i < len(data); {
		l := int(data[i])
		i++
		switch {
		case l == 128:
			return out, nil
		case l < 128:
			if i+l+1 > len(data) {
				return nil, errors.New("a run is short")
			}
			out = append(out, data[i:i+l+1]...)
			i += l + 1
		default:
			if i >= len(data) {
				return nil, errors.New("a run is short")
			}
			out = append(out, bytes.Repeat(data[i:i+1], 257-l)...)
			i++
		}
	}
	return out, nil
}

// canon writes a value as text, with one space between its tokens, so that
// two values with the same tokens give the same text.
func canon(v any) string {
	var b strings.Builder
	writeCanon(&b, v)
	return b.String()
}

func writeCanon(b *strings.Builder, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("<missing>")
	case pdfName:
		b.WriteString("/" + string(v))
	case pdfNumber:
		b.WriteString(string(v))
	case pdfKeyword:
		b.WriteString(string(v))
	case pdfRef:
		fmt.Fprintf(b, "%d %d R", v.Num, v.Gen)
	case pdfString:
		if v.Hex {
			b.WriteString("<" + hex.EncodeToString(v.Bytes) + ">")
		} else {
			b.WriteString(strconv.Quote(string(v.Bytes)))
		}
	case pdfArray:
		b.WriteString("[")
		for i, x := range v {
			if i > 0 {
				b.WriteString(" ")
			}
			writeCanon(b, x)
		}
		b.WriteString("]")
	case *pdfDict:
		b.WriteString("<<")
		for _, k := range v.Keys {
			b.WriteString(" /" + string(k) + " ")
			writeCanon(b, v.Values[k])
		}
		b.WriteString(" >>")
	default:
		fmt.Fprintf(b, "<%T>", v)
	}
}

// normalized returns the dictionary of a stream without /Length, and with
// /Filter and /DecodeParms only for the filters that the reader did not
// decode. Other values are returned as they are.
func normalized(o *pdfObject) any {
	d, ok := o.Value.(*pdfDict)
	if !ok || !o.IsStream {
		return o.Value
	}
	n := &pdfDict{Keys: slices.Clone(d.Keys), Values: maps.Clone(d.Values)}
	n.del("Length")
	n.del("Filter")
	n.del("DecodeParms")
	if len(o.Filters) > 0 {
		filters := make(pdfArray, len(o.Filters))
		for i, f := range o.Filters {
			filters[i] = f
		}
		n.set("Filter", filters)
		if slices.ContainsFunc(o.Parms, func(p any) bool { return p != nil }) {
			n.set("DecodeParms", pdfArray(o.Parms))
		}
	}
	return n
}

// comparePDF compares two files by their objects, in the order of the
// files. The data of each stream is compared after the reader decodes it.
// /Length, the filters that the reader decodes and their parameters, and /ID
// of the trailer can differ (D62). So can the data of a cross-reference
// stream, which holds offsets. The XMP metadata holds the two strings of
// /ID, so each stream is compared with the strings of /ID of its file
// replaced (D99). It returns one line for each difference.
func comparePDF(got, want *pdfFile) []string {
	gotIDs, wantIDs := fileIDs(got), fileIDs(want)
	var diffs []string
	if got.Version != want.Version {
		diffs = append(diffs, fmt.Sprintf("the header: got version %s, want %s", got.Version, want.Version))
	}
	gt := canon(withoutID(got.Trailer))
	wt := canon(withoutID(want.Trailer))
	if gt != wt {
		diffs = append(diffs, fmt.Sprintf("the trailer: got %s, want %s", gt, wt))
	}
	for i := range max(len(got.Objects), len(want.Objects)) {
		if i >= len(got.Objects) {
			diffs = append(diffs, fmt.Sprintf("object %d %d is missing", want.Objects[i].Ref.Num, want.Objects[i].Ref.Gen))
			continue
		}
		if i >= len(want.Objects) {
			diffs = append(diffs, fmt.Sprintf("object %d %d is extra", got.Objects[i].Ref.Num, got.Objects[i].Ref.Gen))
			continue
		}
		g, w := got.Objects[i], want.Objects[i]
		name := fmt.Sprintf("object %d %d", w.Ref.Num, w.Ref.Gen)
		if g.Ref != w.Ref {
			diffs = append(diffs, fmt.Sprintf("the object at index %d: got %d %d, want %d %d", i, g.Ref.Num, g.Ref.Gen, w.Ref.Num, w.Ref.Gen))
			continue
		}
		if gv, wv := canon(normalized(g)), canon(normalized(w)); gv != wv {
			diffs = append(diffs, fmt.Sprintf("%s: got %s, want %s", name, gv, wv))
		}
		if g.IsStream != w.IsStream {
			diffs = append(diffs, fmt.Sprintf("%s: got a stream %t, want %t", name, g.IsStream, w.IsStream))
			continue
		}
		if !g.IsStream {
			continue
		}
		if d, ok := w.Value.(*pdfDict); ok && d.get("Type") == pdfName("XRef") {
			continue
		}
		if g.DecodeErr != nil || w.DecodeErr != nil {
			diffs = append(diffs, fmt.Sprintf("%s: a stream does not decode: got %v, want %v", name, g.DecodeErr, w.DecodeErr))
			continue
		}
		if s := firstDiff(withoutIDs(g.Data, gotIDs), withoutIDs(w.Data, wantIDs)); s != "" {
			diffs = append(diffs, fmt.Sprintf("%s: the data of the stream differs: %s", name, s))
		}
	}
	return diffs
}

// fileIDs returns the strings of /ID of the trailer of a file, the longest
// first, so that a string that holds the other is replaced first. A string
// of fewer than 16 bytes, such as the IDs of the files of TestReader, is
// left out, so that a short ID does not match data by chance. The IDs of
// krilla and pdf are hashes in base64, which are longer.
func fileIDs(f *pdfFile) [][]byte {
	a, _ := f.Trailer.get("ID").(pdfArray)
	var ids [][]byte
	for _, v := range a {
		if str, ok := v.(pdfString); ok && len(str.Bytes) >= 16 {
			ids = append(ids, str.Bytes)
		}
	}
	slices.SortFunc(ids, func(x, y []byte) int { return len(y) - len(x) })
	return ids
}

// withoutIDs replaces each string of /ID in the data of a stream.
func withoutIDs(data []byte, ids [][]byte) []byte {
	for _, id := range ids {
		if bytes.Contains(data, id) {
			data = bytes.ReplaceAll(data, id, []byte("<ID>"))
		}
	}
	return data
}

// withoutID returns a copy of a trailer without /ID, and without the keys of
// a cross-reference stream that hold offsets and lengths.
func withoutID(d *pdfDict) *pdfDict {
	n := &pdfDict{Keys: slices.Clone(d.Keys), Values: maps.Clone(d.Values)}
	for _, k := range []pdfName{"ID", "Length", "Filter", "DecodeParms", "Prev"} {
		n.del(k)
	}
	return n
}

// firstDiff describes the first byte that differs between two byte slices,
// with the bytes around it, or returns "" when they are the same.
func firstDiff(got, want []byte) string {
	if bytes.Equal(got, want) {
		return ""
	}
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	ctx := func(b []byte) string {
		lo, hi := max(0, i-20), min(len(b), i+20)
		return strconv.Quote(string(b[lo:hi]))
	}
	return fmt.Sprintf("got %d bytes, want %d. The first difference is at offset %d: got %s, want %s",
		len(got), len(want), i, ctx(got), ctx(want))
}
