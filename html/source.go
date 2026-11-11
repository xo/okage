package html

// This file ports no upstream file. It turns the text of a label into a
// document for internal/roxmltree, and maps a position in that document back
// to the text (D38).

import (
	stdhtml "html"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The root element around the text of a label. A label can hold text and
// more than one element at its top, and XML needs one root element.
const (
	rootName   = "okage-html-label"
	rootPrefix = "<" + rootName + ">"
	rootSuffix = "</" + rootName + ">"
)

// piece is a part of the document that maps to a part of the text. In a
// copied piece each byte maps to one byte. A piece that is not copied, such
// as an entity that became a character reference, maps to its start.
type piece struct {
	dst, src int
	copied   bool
}

// source is the text of a label and the document that internal/roxmltree
// parses.
type source struct {
	// src is the text of the label.
	src string

	// doc is src inside the root element, with each named entity of HTML as
	// a character reference.
	doc string

	// pieces map the offsets of doc to the offsets of src, sorted by dst.
	pieces []piece
}

// newSource makes the document of the text of a label. XML knows only five
// named entities, and Graphviz reads the named entities of HTML, so each
// other entity that HTML knows becomes a character reference. An entity that
// HTML does not know stays, and internal/roxmltree reports it.
func newSource(src string) *source {
	s := &source{src: src}
	var b strings.Builder
	b.Grow(len(rootPrefix) + len(src) + len(rootSuffix))
	b.WriteString(rootPrefix)
	s.pieces = append(s.pieces, piece{dst: b.Len(), src: 0, copied: true})
	last := 0
	for i := 0; i < len(src); i++ {
		if src[i] != '&' {
			continue
		}
		name, ok := entityName(src[i+1:])
		if !ok {
			continue
		}
		ref, ok := entityRef(name)
		if !ok {
			continue
		}
		b.WriteString(src[last:i])
		s.pieces = append(s.pieces, piece{dst: b.Len(), src: i})
		b.WriteString(ref)
		last = i + 1 + len(name) + 1
		s.pieces = append(s.pieces, piece{dst: b.Len(), src: last, copied: true})
		i = last - 1
	}
	b.WriteString(src[last:])
	s.pieces = append(s.pieces, piece{dst: b.Len(), src: len(src)})
	b.WriteString(rootSuffix)
	s.doc = b.String()
	return s
}

// entityName returns the name of a named entity at the start of s, which
// follows an '&', when a ';' ends it.
func entityName(s string) (string, bool) {
	for i := range len(s) {
		c := s[i]
		switch {
		case c == ';':
			return s[:i], i > 0
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9' && i > 0:
		default:
			return "", false
		}
	}
	return "", false
}

// entityRef returns the character references of a named entity of HTML that
// XML does not know.
func entityRef(name string) (string, bool) {
	switch name {
	case "amp", "lt", "gt", "quot", "apos":
		return "", false
	}
	ent := "&" + name + ";"
	text := stdhtml.UnescapeString(ent)
	if text == ent {
		return "", false
	}
	var b strings.Builder
	for _, r := range text {
		b.WriteString("&#")
		b.WriteString(strconv.Itoa(int(r)))
		b.WriteByte(';')
	}
	return b.String(), true
}

// srcOffset maps an offset of doc to an offset of src.
func (s *source) srcOffset(dst int) int {
	i := sort.Search(len(s.pieces), func(i int) bool { return s.pieces[i].dst > dst }) - 1
	if i < 0 {
		return 0
	}
	p := s.pieces[i]
	if !p.copied {
		return p.src
	}
	return min(p.src+dst-p.dst, len(s.src))
}

// docOffset returns the offset of a line and a column of doc, as
// internal/roxmltree counts them: from 1, with a column for each character.
func (s *source) docOffset(line, col uint32) int {
	off := 0
	for l := uint32(1); l < line; l++ {
		i := strings.IndexByte(s.doc[off:], '\n')
		if i < 0 {
			return len(s.doc)
		}
		off += i + 1
	}
	for c := uint32(1); c < col && off < len(s.doc) && s.doc[off] != '\n'; c++ {
		_, size := utf8.DecodeRuneInString(s.doc[off:])
		off += size
	}
	return off
}

// Pos is a position in the text of a label: a line and a column, from 1,
// with a column for each character.
type Pos struct {
	Line   int
	Column int
}

// String returns the position as line:column.
func (p Pos) String() string {
	return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Column)
}

// pos returns the position of an offset of doc in src.
func (s *source) pos(dst int) Pos {
	off := s.srcOffset(dst)
	p := Pos{Line: 1, Column: 1}
	for _, r := range s.src[:off] {
		if r == '\n' {
			p.Line++
			p.Column = 1
			continue
		}
		p.Column++
	}
	return p
}
