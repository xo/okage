// Copyright (c) 2019 Reizner Evgeniy
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmlwriter

import (
	"fmt"
	"slices"
	"unicode/utf8"
)

// IndentKind is the case of an Indent.
type IndentKind uint8

// The cases of the enum Indent in xmlwriter src/lib.rs.
const (
	// IndentNone is Indent::None. It disables indention and new lines.
	IndentNone IndentKind = iota
	// IndentSpaces is Indent::Spaces. It indents with spaces. Preferred
	// range is 0..4.
	IndentSpaces
	// IndentTabs is Indent::Tabs. It indents with tabs.
	IndentTabs
)

// String returns the name of the case in upstream.
func (k IndentKind) String() string {
	switch k {
	case IndentNone:
		return "None"
	case IndentSpaces:
		return "Spaces"
	case IndentTabs:
		return "Tabs"
	}
	return "IndentKind(?)"
}

// Indent is Indent in xmlwriter src/lib.rs. It is an XML node indention.
//
// Port: only the case Spaces of the Rust enum holds a value, so it becomes a
// struct with a kind (D27). The zero value is IndentNone.
type Indent struct {
	Kind IndentKind
	// Spaces is the number of spaces of IndentSpaces.
	Spaces uint8
}

// Option is an option of New and NewWithCapacity. It sets a field of the
// struct Options in xmlwriter src/lib.rs (D20).
type Option func(*options)

// options is Options in xmlwriter src/lib.rs. It is an XML writing options.
type options struct {
	// useSingleQuote is Options::use_single_quote. Use single quote marks
	// instead of double quote.
	//
	// Before:
	//
	//	<rect fill="red"/>
	//
	// After:
	//
	//	<rect fill='red'/>
	//
	// Default: disabled
	useSingleQuote bool

	// indent is Options::indent. Set XML nodes indention.
	//
	// IndentNone
	// Before:
	//
	//	<svg>
	//	    <rect fill="red"/>
	//	</svg>
	//
	// After:
	//
	//	<svg><rect fill="red"/></svg>
	//
	// Default: 4 spaces
	indent Indent

	// attributesIndent is Options::attributes_indent. Set XML attributes
	// indention.
	//
	// Indent{Kind: IndentSpaces, Spaces: 2}
	//
	// Before:
	//
	//	<svg>
	//	    <rect fill="red" stroke="black"/>
	//	</svg>
	//
	// After:
	//
	//	<svg>
	//	    <rect
	//	      fill="red"
	//	      stroke="black"/>
	//	</svg>
	//
	// Default: IndentNone
	attributesIndent Indent
}

// defaultOptions is Default for Options in xmlwriter src/lib.rs.
func defaultOptions() options {
	return options{
		useSingleQuote:   false,
		indent:           Indent{Kind: IndentSpaces, Spaces: 4},
		attributesIndent: Indent{Kind: IndentNone},
	}
}

// WithUseSingleQuote is an option that sets Options::use_single_quote in
// xmlwriter src/lib.rs. If v is true, the writer uses single quote marks
// instead of double quote. Default: false.
func WithUseSingleQuote(v bool) Option {
	return func(o *options) {
		o.useSingleQuote = v
	}
}

// WithIndent is an option that sets Options::indent in xmlwriter src/lib.rs,
// the indention of XML nodes. Default: 4 spaces.
func WithIndent(indent Indent) Option {
	return func(o *options) {
		o.indent = indent
	}
}

// WithAttributesIndent is an option that sets Options::attributes_indent in
// xmlwriter src/lib.rs, the indention of XML attributes. Default:
// IndentNone.
func WithAttributesIndent(indent Indent) Option {
	return func(o *options) {
		o.attributesIndent = indent
	}
}

// state is State in xmlwriter src/lib.rs.
type state uint8

// The cases of the enum State in xmlwriter src/lib.rs.
const (
	// stateEmpty is State::Empty.
	stateEmpty state = iota
	// stateDocument is State::Document.
	stateDocument
	// stateAttributes is State::Attributes.
	stateAttributes
)

// String returns the name of the case in upstream.
func (s state) String() string {
	switch s {
	case stateEmpty:
		return "Empty"
	case stateDocument:
		return "Document"
	case stateAttributes:
		return "Attributes"
	}
	return "state(?)"
}

// depthData is DepthData in xmlwriter src/lib.rs.
type depthData struct {
	// start and end are the Range<usize> range of upstream.
	start       int
	end         int
	hasChildren bool
}

// XMLWriter is XmlWriter in xmlwriter src/lib.rs. It is an XML writer.
//
// Port: the Go name spells the initialism XML in capitals, as Go names do.
type XMLWriter struct {
	buf                 []byte
	state               state
	preserveWhitespaces bool
	depthStack          []depthData
	opt                 options
}

// fromVec is XmlWriter::from_vec in xmlwriter src/lib.rs.
func fromVec(buf []byte, opt options) *XMLWriter {
	return &XMLWriter{
		buf:                 buf,
		state:               stateEmpty,
		preserveWhitespaces: false,
		depthStack:          nil,
		opt:                 opt,
	}
}

// New is XmlWriter::new in xmlwriter src/lib.rs. It creates a new
// XMLWriter.
//
// Port: upstream takes a struct Options. The port takes an option for each
// field of it, and a field without an option keeps its default (D20).
func New(opts ...Option) *XMLWriter {
	return fromVec(nil, newOptions(opts))
}

// NewWithCapacity is XmlWriter::with_capacity in xmlwriter src/lib.rs. It
// creates a new XMLWriter with a specified capacity.
func NewWithCapacity(capacity int, opts ...Option) *XMLWriter {
	return fromVec(make([]byte, 0, capacity), newOptions(opts))
}

// newOptions applies opts to the default options and returns them.
func newOptions(opts []Option) options {
	o := defaultOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// WriteDeclaration is XmlWriter::write_declaration in xmlwriter src/lib.rs.
// It writes an XML declaration.
//
//	<?xml version="1.0" encoding="UTF-8" standalone="no"?>
//
// It panics when called twice.
func (w *XMLWriter) WriteDeclaration() {
	if w.state != stateEmpty {
		panic("declaration was already written")
	}

	// Pretend that we are writing an element.
	w.state = stateAttributes

	// <?xml version='1.0' encoding='UTF-8' standalone='yes'?>
	w.pushStr("<?xml")
	w.WriteAttribute("version", "1.0")
	w.WriteAttribute("encoding", "UTF-8")
	w.WriteAttribute("standalone", "no")
	w.pushStr("?>")

	w.state = stateDocument
}

// WriteComment is XmlWriter::write_comment in xmlwriter src/lib.rs. It
// writes a comment string.
func (w *XMLWriter) WriteComment(text string) {
	w.WriteCommentFmt("%s", text)
}

// WriteCommentFmt is XmlWriter::write_comment_fmt in xmlwriter src/lib.rs.
// It writes a formatted comment.
//
// Port: upstream takes fmt::Arguments. The port takes a format of the
// package fmt and its arguments.
func (w *XMLWriter) WriteCommentFmt(format string, args ...any) { //nolint:goprintffuncname // the Rust name, so that a search for it finds the port
	if w.state == stateAttributes {
		w.writeOpenElement()
	}

	if w.state != stateEmpty {
		w.writeNewLine()
	}

	w.writeNodeIndent()

	// <!--text-->
	w.pushStr("<!--")
	w.buf = fmt.Appendf(w.buf, format, args...) // TODO: check content
	w.pushStr("-->")

	if w.state == stateAttributes {
		w.depthStack = append(w.depthStack, depthData{
			start:       0,
			end:         0,
			hasChildren: false,
		})
	}

	w.state = stateDocument
}

// StartElement is XmlWriter::start_element in xmlwriter src/lib.rs. It
// starts writing a new element.
//
// This method writes only the `<tag-name` part.
func (w *XMLWriter) StartElement(name string) {
	if w.state == stateAttributes {
		w.writeOpenElement()
	}

	if w.state != stateEmpty {
		w.writeNewLine()
	}

	if !w.preserveWhitespaces {
		w.writeNodeIndent()
	}

	w.pushByte('<')
	start := len(w.buf)
	w.pushStr(name)

	w.depthStack = append(w.depthStack, depthData{
		start:       start,
		end:         len(w.buf),
		hasChildren: false,
	})

	w.state = stateAttributes
}

// WriteAttribute is XmlWriter::write_attribute in xmlwriter src/lib.rs. It
// writes an attribute.
//
// Quotes in the value will be escaped.
//
// It panics when called before StartElement, or after EndElement.
//
// Port: upstream takes any value that implements fmt::Display. The port
// takes any value and formats it with the verb %v of the package fmt. A
// string or an integer gives the text of upstream. A float can give other
// text than Rust, so a caller formats a float itself.
func (w *XMLWriter) WriteAttribute(name string, value any) {
	w.WriteAttributeFmt(name, "%v", value)
}

// WriteAttributeFmt is XmlWriter::write_attribute_fmt in xmlwriter
// src/lib.rs. It writes a formatted attribute value.
//
// Quotes in the value will be escaped.
//
// It panics when called before StartElement, or after EndElement.
//
// Port: upstream takes fmt::Arguments. The port takes a format of the
// package fmt and its arguments.
func (w *XMLWriter) WriteAttributeFmt(name, format string, args ...any) { //nolint:goprintffuncname // the Rust name, so that a search for it finds the port
	if w.state != stateAttributes {
		panic("must be called after start_element()")
	}

	w.writeAttributePrefix(name)
	start := len(w.buf)
	w.buf = fmt.Appendf(w.buf, format, args...)
	w.escapeAttributeValue(start)
	w.writeQuote()
}

// WriteAttributeRaw is XmlWriter::write_attribute_raw in xmlwriter
// src/lib.rs. It writes a raw attribute value.
//
// f appends to the internal buffer and returns it.
//
// Warning: this method is an escape hatch for cases when you need to write a
// lot of data very fast.
//
// It panics when called before StartElement, or after EndElement.
//
// Port: upstream gives the closure a mutable reference to the buffer. The
// port gives f the buffer, and f returns it with the bytes that it appends,
// as the function append does.
func (w *XMLWriter) WriteAttributeRaw(name string, f func(buf []byte) []byte) {
	if w.state != stateAttributes {
		panic("must be called after start_element()")
	}

	w.writeAttributePrefix(name)
	start := len(w.buf)
	w.buf = f(w.buf)
	w.escapeAttributeValue(start)
	w.writeQuote()
}

// writeAttributePrefix is XmlWriter::write_attribute_prefix in xmlwriter
// src/lib.rs.
func (w *XMLWriter) writeAttributePrefix(name string) {
	if w.opt.attributesIndent.Kind == IndentNone {
		w.pushByte(' ')
	} else {
		w.pushByte('\n')

		depth := len(w.depthStack)
		if depth > 0 {
			w.writeIndent(depth-1, w.opt.indent)
		}

		w.writeIndent(1, w.opt.attributesIndent)
	}

	w.pushStr(name)
	w.pushByte('=')
	w.writeQuote()
}

// escapeAttributeValue is XmlWriter::escape_attribute_value in xmlwriter
// src/lib.rs. It escapes the attribute value string.
//
//   - " -> &quot;
//   - ' -> &apos;
func (w *XMLWriter) escapeAttributeValue(start int) {
	quote := byte('"')
	if w.opt.useSingleQuote {
		quote = '\''
	}
	for {
		idx := slices.Index(w.buf[start:], quote)
		if idx < 0 {
			break
		}
		i := start + idx
		s := "&quot;"
		if w.opt.useSingleQuote {
			s = "&apos;"
		}
		w.buf = slices.Replace(w.buf, i, i+1, []byte(s)...)
		start = i + 6
	}
}

// SetPreserveWhitespaces is XmlWriter::set_preserve_whitespaces in
// xmlwriter src/lib.rs. It sets the preserve whitespaces flag.
//
//   - If set, text nodes will be written as is.
//   - If not set, text nodes will be indented.
//
// Can be set at any moment.
func (w *XMLWriter) SetPreserveWhitespaces(preserve bool) {
	w.preserveWhitespaces = preserve
}

// WriteText is XmlWriter::write_text in xmlwriter src/lib.rs. It writes a
// text node.
//
// See WriteTextFmt for details.
func (w *XMLWriter) WriteText(text string) {
	w.WriteTextFmt("%s", text)
}

// WriteTextFmt is XmlWriter::write_text_fmt in xmlwriter src/lib.rs. It
// writes a formatted text node.
//
// `<` will be escaped.
//
// It panics when not called after StartElement.
//
// Port: upstream takes fmt::Arguments. The port takes a format of the
// package fmt and its arguments.
func (w *XMLWriter) WriteTextFmt(format string, args ...any) { //nolint:goprintffuncname // the Rust name, so that a search for it finds the port
	if w.state == stateEmpty || len(w.depthStack) == 0 {
		panic("must be called after start_element()")
	}

	if w.state == stateAttributes {
		w.writeOpenElement()
	}

	if w.state != stateEmpty {
		w.writeNewLine()
	}

	w.writeNodeIndent()

	start := len(w.buf)
	w.buf = fmt.Appendf(w.buf, format, args...)
	w.escapeText(start)

	if w.state == stateAttributes {
		w.depthStack = append(w.depthStack, depthData{
			start:       0,
			end:         0,
			hasChildren: false,
		})
	}

	w.state = stateDocument
}

// escapeText is XmlWriter::escape_text in xmlwriter src/lib.rs.
func (w *XMLWriter) escapeText(start int) {
	for {
		idx := slices.Index(w.buf[start:], '<')
		if idx < 0 {
			break
		}
		i := start + idx
		w.buf = slices.Replace(w.buf, i, i+1, []byte("&lt;")...)
		start = i + 4
	}
}

// EndElement is XmlWriter::end_element in xmlwriter src/lib.rs. It closes
// an open element.
func (w *XMLWriter) EndElement() {
	if n := len(w.depthStack); n > 0 {
		depth := w.depthStack[n-1]
		w.depthStack = w.depthStack[:n-1]
		if depth.hasChildren {
			if !w.preserveWhitespaces {
				w.writeNewLine()
				w.writeNodeIndent()
			}

			w.pushStr("</")

			for i := depth.start; i < depth.end; i++ {
				w.pushByte(w.buf[i])
			}

			w.pushByte('>')
		} else {
			w.pushStr("/>")
		}
	}

	w.state = stateDocument
}

// EndDocument is XmlWriter::end_document in xmlwriter src/lib.rs. It closes
// all open elements and returns an internal XML buffer.
//
// Port: upstream takes the writer by value, so that no call can follow.
// The port leaves w closed, and a caller must not use it again.
func (w *XMLWriter) EndDocument() string {
	for len(w.depthStack) != 0 {
		w.EndElement()
	}

	w.writeNewLine()

	// The only way it can fail is if an invalid data
	// was written via `write_attribute_raw()`.
	if !utf8.Valid(w.buf) {
		panic("xmlwriter: the buffer is not valid UTF-8")
	}
	s := string(w.buf)
	w.buf = nil
	return s
}

// pushByte is XmlWriter::push_byte in xmlwriter src/lib.rs.
func (w *XMLWriter) pushByte(c byte) {
	w.buf = append(w.buf, c)
}

// pushStr is XmlWriter::push_str in xmlwriter src/lib.rs.
func (w *XMLWriter) pushStr(text string) {
	w.buf = append(w.buf, text...)
}

// getQuoteChar is XmlWriter::get_quote_char in xmlwriter src/lib.rs.
func (w *XMLWriter) getQuoteChar() byte {
	if w.opt.useSingleQuote {
		return '\''
	}
	return '"'
}

// writeQuote is XmlWriter::write_quote in xmlwriter src/lib.rs.
func (w *XMLWriter) writeQuote() {
	w.pushByte(w.getQuoteChar())
}

// writeOpenElement is XmlWriter::write_open_element in xmlwriter
// src/lib.rs.
func (w *XMLWriter) writeOpenElement() {
	if n := len(w.depthStack); n > 0 {
		w.depthStack[n-1].hasChildren = true
		w.pushByte('>')

		w.state = stateDocument
	}
}

// writeNodeIndent is XmlWriter::write_node_indent in xmlwriter src/lib.rs.
func (w *XMLWriter) writeNodeIndent() {
	w.writeIndent(len(w.depthStack), w.opt.indent)
}

// writeIndent is XmlWriter::write_indent in xmlwriter src/lib.rs.
func (w *XMLWriter) writeIndent(depth int, indent Indent) {
	if indent.Kind == IndentNone || w.preserveWhitespaces {
		return
	}

	for range depth {
		switch indent.Kind {
		case IndentNone:
		case IndentSpaces:
			for range indent.Spaces {
				w.pushByte(' ')
			}
		case IndentTabs:
			w.pushByte('\t')
		}
	}
}

// writeNewLine is XmlWriter::write_new_line in xmlwriter src/lib.rs.
func (w *XMLWriter) writeNewLine() {
	if w.opt.indent.Kind != IndentNone && !w.preserveWhitespaces {
		w.pushByte('\n')
	}
}
