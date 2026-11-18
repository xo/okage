// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"fmt"
	"iter"
	"math"
	"unicode/utf16"
)

// This file ports pdf-writer src/object.rs.

// Primitive is the traits Primitive and Sealed in pdf-writer src/object.rs.
// Its methods are not exported, so that only this package has primitive
// types, as the sealed trait of upstream does.
//
// A primitive PDF object.
type Primitive interface {
	// startsWithDelimiter is Sealed::STARTS_WITH_DELIMITER.
	//
	// Whether the primitive object starts with one of the PDF delimiter
	// characters.
	startsWithDelimiter() bool
	// write is Sealed::write.
	//
	// Write the object into a buffer.
	write(buf *Buf)
}

// Bool is the impl Primitive for bool in pdf-writer src/object.rs.
type Bool bool

func (Bool) startsWithDelimiter() bool { return false }

func (v Bool) write(buf *Buf) {
	if v {
		buf.extendString("true")
	} else {
		buf.extendString("false")
	}
}

// Int is the impl Primitive for i32 in pdf-writer src/object.rs.
type Int int32

func (Int) startsWithDelimiter() bool { return false }

func (v Int) write(buf *Buf) {
	buf.pushInt(int32(v))
}

// Real is the impl Primitive for f32 in pdf-writer src/object.rs.
type Real float32

func (Real) startsWithDelimiter() bool { return false }

func (v Real) write(buf *Buf) {
	buf.pushFloat(float32(v))
}

// Str is Str in pdf-writer src/object.rs. A Go string holds the bytes.
//
// A string object (any byte sequence).
//
// This is written as `(Thing)`.
type Str string

// isBalanced is Str::is_balanced in pdf-writer src/object.rs.
//
// Whether the parentheses in the byte string are balanced.
func (s Str) isBalanced() bool {
	depth := 0
	for i := range len(s) {
		switch byte := s[i]; {
		case byte == '(':
			depth++
		case byte == ')' && depth > 0:
			depth--
		case byte == ')':
			return false
		}
	}
	return depth == 0
}

func (Str) startsWithDelimiter() bool { return true }

func (s Str) write(buf *Buf) {
	buf.limits.registerStrLen(len(s))

	// We use:
	// - Literal strings for ASCII with nice escape sequences to make it
	//   also be represented fully in visible ASCII. We also escape
	//   parentheses because they are delimiters.
	// - Hex strings for anything non-ASCII.
	ascii := true
	for i := range len(s) {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		buf.reserve(len(s))
		buf.push('(')

		// balanced is the Option<bool> of upstream: 0 is None, 1 is
		// Some(false) and 2 is Some(true).
		balanced := 0
		for i := range len(s) {
			switch byte := s[i]; {
			case byte == '(' || byte == ')':
				if balanced == 0 {
					balanced = 1
					if byte != ')' && s.isBalanced() {
						balanced = 2
					}
				}
				if balanced != 2 {
					buf.push('\\')
				}
				buf.push(byte)
			case byte == '\\':
				buf.extendString(`\\`)
			case ' ' <= byte && byte <= '~':
				buf.push(byte)
			case byte == '\n':
				buf.extendString(`\n`)
			case byte == '\r':
				buf.extendString(`\r`)
			case byte == '\t':
				buf.extendString(`\t`)
			case byte == '\x08':
				buf.extendString(`\b`)
			case byte == '\x0c':
				buf.extendString(`\f`)
			default:
				buf.push('\\')
				buf.pushOctal(byte)
			}
		}

		buf.push(')')
	} else {
		buf.reserve(2 + 2*len(s))
		buf.push('<')

		for i := range len(s) {
			buf.pushHex(s[i])
		}

		buf.push('>')
	}
}

// TextStr is TextStr in pdf-writer src/object.rs. Rust holds a string that is
// valid UTF-8. The port writes each byte of a Go string that is not valid
// UTF-8 as U+FFFD.
//
// A unicode text string object.
//
// This is written as a Str containing either bare ASCII (if possible) or a
// byte order mark followed by UTF-16-BE bytes.
//
// The natural language is inherited from the document catalog's /Lang key.
// If you need to specify another language or if the string contains multiple
// natural languages, see TextStrWithLang.
type TextStr string

func (TextStr) startsWithDelimiter() bool { return true }

func (s TextStr) write(buf *Buf) {
	buf.limits.registerStrLen(len(s))

	// ASCII and PDFDocEncoding match for 32 up to 126.
	printable := true
	for i := range len(s) {
		if s[i] < 32 || s[i] > 126 {
			printable = false
			break
		}
	}
	if printable {
		Str(s).write(buf)
	} else {
		buf.reserve(6 + 4*len(s))
		writeUTF16BETextStrHeader(buf)
		for _, value := range utf16.Encode([]rune(string(s))) {
			buf.pushHexU16(value)
		}
		writeUTF16BETextStrFooter(buf)
	}
}

// LanguageIdentifier is LanguageIdentifier in pdf-writer src/object.rs.
//
// An identifier for the natural language in a section of a TextStrWithLang.
type LanguageIdentifier struct {
	// A two-byte ISO 639 language code.
	lang [2]byte
	// A two-byte ISO 3166 country code. hasRegion is false for None.
	region    [2]byte
	hasRegion bool
}

// NewLanguageIdentifier is LanguageIdentifier::new in pdf-writer
// src/object.rs. A nil region is None.
//
// Create a new language identifier.
//
// Language and region codes are not checked for validity.
func NewLanguageIdentifier(lang [2]byte, region *[2]byte) LanguageIdentifier {
	id := LanguageIdentifier{lang: lang}
	if region != nil {
		id.region = *region
		id.hasRegion = true
	}
	return id
}

// LanguageIdentifierFromLang is LanguageIdentifier::from_lang in pdf-writer
// src/object.rs.
//
// Create a new language identifier from a language, with an unset region.
//
// The method returns true if the argument has two alphanumeric ASCII bytes.
func LanguageIdentifierFromLang(lang string) (LanguageIdentifier, bool) {
	code, ok := strToCode(lang)
	if !ok {
		return LanguageIdentifier{}, false
	}
	return NewLanguageIdentifier(code, nil), true
}

// LanguageIdentifierFromLangRegion is LanguageIdentifier::from_lang_region
// in pdf-writer src/object.rs.
//
// Create a new language identifier from a language and a region.
//
// The method returns true if both arguments have two alphanumeric ASCII
// bytes.
func LanguageIdentifierFromLangRegion(lang, region string) (LanguageIdentifier, bool) {
	l, ok := strToCode(lang)
	if !ok {
		return LanguageIdentifier{}, false
	}
	r, ok := strToCode(region)
	if !ok {
		return LanguageIdentifier{}, false
	}
	return NewLanguageIdentifier(l, &r), true
}

// len is LanguageIdentifier::len in pdf-writer src/object.rs.
//
// Returns the length of the language identifier. It does not include the
// enclosing escape bytes.
func (id LanguageIdentifier) len() int {
	if id.hasRegion {
		return 4
	}
	return 2
}

// strToCode is LanguageIdentifier::str_to_code in pdf-writer src/object.rs.
func strToCode(s string) ([2]byte, bool) {
	for _, c := range s {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z') {
			return [2]byte{}, false
		}
	}
	if len(s) != 2 {
		return [2]byte{}, false
	}
	return [2]byte{s[0], s[1]}, true
}

// LangText is one item of a TextStrWithLang: the tuple (LanguageIdentifier,
// &str) of upstream.
type LangText struct {
	Lang LanguageIdentifier
	Text string
}

// TextStrWithLang is TextStrWithLang in pdf-writer src/object.rs.
//
// A text string with a natural language specified.
//
// This is written as a string containing either bare ASCII (if possible) or
// a byte order mark followed by UTF-16-BE bytes. Both forms are interspersed
// by the requisite ASCII language escape sequences.
//
// For a text string with an undefined natural language, see TextStr.
type TextStrWithLang []LangText

func (TextStrWithLang) startsWithDelimiter() bool { return true }

func (s TextStrWithLang) write(buf *Buf) {
	n := 0
	bufLen := 6

	for _, item := range s {
		// Each language tag is enclosed in two escape characters, plus the
		// two-letter language code and optional two-letter region code.
		n += len(item.Text) + item.Lang.len() + 2
		// Each hexadecimal character is four bytes long, plus four bytes
		// for the escape sequence. The language tag is encoded in
		// hexadecimal, so each byte becomes two hexadecimal characters.
		bufLen += 4*len(item.Text) + item.Lang.len()*2 + 4
	}

	buf.limits.registerStrLen(n)

	// Escape sequences for languages may only appear in Unicode-encoded
	// text strings, see clause 7.9.2.2 of ISO 32000-1:2008.
	buf.reserve(bufLen)
	writeUTF16BETextStrHeader(buf)

	for _, item := range s {
		writeUTF16BELangCode(item.Lang, buf)
		for _, value := range utf16.Encode([]rune(item.Text)) {
			buf.pushHexU16(value)
		}
	}

	writeUTF16BETextStrFooter(buf)
}

// writeUTF16BETextStrHeader is write_utf16be_text_str_header in pdf-writer
// src/object.rs.
func writeUTF16BETextStrHeader(buf *Buf) {
	buf.push('<')
	buf.pushHex(254)
	buf.pushHex(255)
}

// writeUTF16BETextStrFooter is write_utf16be_text_str_footer in pdf-writer
// src/object.rs.
func writeUTF16BETextStrFooter(buf *Buf) {
	buf.push('>')
}

// writeUTF16BELangCode is write_utf16be_lang_code in pdf-writer
// src/object.rs.
func writeUTF16BELangCode(lang LanguageIdentifier, buf *Buf) {
	// The escape character U+001B encloses the language tag. It must not
	// otherwise appear in a text string and the spec offers no opportunity to
	// escape it. In the future, `pdf-writer` may offer a constructor for
	// [`TextStrWithLang`] and [`TextStr`] that either checks for it or replaces
	// it with the object replacement character U+FFFD.
	buf.pushHexU16(0x001B)
	buf.pushHexU16(uint16(lang.lang[0]))
	buf.pushHexU16(uint16(lang.lang[1]))
	if lang.hasRegion {
		buf.pushHexU16(uint16(lang.region[0]))
		buf.pushHexU16(uint16(lang.region[1]))
	}
	buf.pushHexU16(0x001B)
}

// TextStrLike is TextStrLike in pdf-writer src/object.rs.
//
// A trait for types that can be used everywhere a text string is expected.
// This includes both TextStr and TextStrWithLang.
//
// Methods that accept an implementor of this trait expect strings in natural
// language for which a language specification makes sense, often for use in
// the UI or with AT.
type TextStrLike interface {
	Primitive
	textStrLike()
}

func (TextStr) textStrLike()         {}
func (TextStrWithLang) textStrLike() {}

// Name is Name in pdf-writer src/object.rs. A Go string holds the bytes.
//
// A name object.
//
// Written as `/Thing`.
type Name string

func (Name) startsWithDelimiter() bool { return true }

func (n Name) write(buf *Buf) {
	buf.limits.registerNameLen(len(n))

	buf.reserve(1 + len(n))
	buf.push('/')
	for i := range len(n) {
		byte := n[i]
		// - Number sign shall use hexadecimal escape
		// - Regular characters within the range exlacamation mark .. tilde
		//   can be written directly
		if byte != '#' && '!' <= byte && byte <= '~' && isRegularCharacter(byte) {
			buf.push(byte)
		} else {
			buf.push('#')
			buf.pushHex(byte)
		}
	}
}

// isRegularCharacter is is_regular_character in pdf-writer src/object.rs.
//
// Regular characters are a PDF concept.
func isRegularCharacter(byte byte) bool {
	switch byte {
	case '\x00', '\t', '\n', '\x0C', '\r', ' ', '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return false
	}
	return true
}

// isDelimiterCharacter is is_delimiter_character in pdf-writer
// src/object.rs.
func isDelimiterCharacter(byte byte) bool {
	switch byte {
	case '(', ')', '<', '>', '[', ']', '/', '%':
		return true
	}
	return false
}

// Null is Null in pdf-writer src/object.rs.
//
// The null object.
type Null struct{}

func (Null) startsWithDelimiter() bool { return false }

func (Null) write(buf *Buf) {
	buf.extendString("null")
}

// Ref is Ref in pdf-writer src/object.rs. The zero value is not a valid
// reference. Make one with NewRef.
//
// A reference to an indirect object.
type Ref struct {
	id int32
}

// NewRef is Ref::new in pdf-writer src/object.rs.
//
// Create a new indirect reference.
//
// The provided value must be greater than zero.
//
// Panics if id is out of the valid range.
func NewRef(id int32) Ref {
	if id <= 0 {
		panic("indirect reference out of valid range")
	}
	return Ref{id: id}
}

// Get is Ref::get in pdf-writer src/object.rs.
//
// Return the underlying number as a primitive type.
func (r Ref) Get() int32 {
	return r.id
}

// Next is Ref::next in pdf-writer src/object.rs.
//
// The next consecutive ID.
func (r Ref) Next() Ref {
	return NewRef(r.Get() + 1)
}

// Bump is Ref::bump in pdf-writer src/object.rs.
//
// Increase this ID by one and return the old one. Useful to turn this ID
// into a bump allocator of sorts.
func (r *Ref) Bump() Ref {
	prev := *r
	*r = r.Next()
	return prev
}

func (Ref) startsWithDelimiter() bool { return false }

func (r Ref) write(buf *Buf) {
	buf.pushInt(r.id)
	buf.extendString(" 0 R")
}

// Rect is Rect in pdf-writer src/object.rs.
//
// A rectangle, specified by two opposite corners.
type Rect struct {
	// The x-coordinate of the first (typically, lower-left) corner.
	X1 float32
	// The y-coordinate of the first (typically, lower-left) corner.
	Y1 float32
	// The x-coordinate of the second (typically, upper-right) corner.
	X2 float32
	// The y-coordinate of the second (typically, upper-right) corner.
	Y2 float32
}

// NewRect is Rect::new in pdf-writer src/object.rs.
//
// Create a new rectangle from four coordinate values.
func NewRect(x1, y1, x2, y2 float32) Rect {
	return Rect{X1: x1, Y1: y1, X2: x2, Y2: y2}
}

// ToQuadPoints is Rect::to_quad_points in pdf-writer src/object.rs.
//
// Convert this rectangle into 8 floats describing the four corners of the
// rectangle in counterclockwise order.
func (r Rect) ToQuadPoints() [8]float32 {
	return [8]float32{r.X1, r.Y1, r.X2, r.Y1, r.X2, r.Y2, r.X1, r.Y2}
}

func (Rect) startsWithDelimiter() bool { return true }

func (r Rect) write(buf *Buf) {
	buf.push('[')
	buf.pushVal(Real(r.X1))
	buf.push(' ')
	buf.pushVal(Real(r.Y1))
	buf.push(' ')
	buf.pushVal(Real(r.X2))
	buf.push(' ')
	buf.pushVal(Real(r.Y2))
	buf.push(']')

	buf.limits.registerArrayLen(4)
}

// optU8 is an Option<u8> of Rust.
type optU8 struct {
	v  uint8
	ok bool
}

// Date is Date in pdf-writer src/object.rs.
//
// A date, written as a text string.
//
// A field is only respected if all superior fields are supplied. For
// example, to set the minute, the hour, day, etc. have to be set. Similarly,
// in order for the time zone information to be written, all time information
// (including seconds) must be written. utc_offset_minute is optional if
// supplying time zone info. It must only be used to specify sub-hour time
// zone offsets.
type Date struct {
	// The year (0-9999).
	year uint16
	// The month (0-11).
	month optU8
	// The month (0-30).
	day optU8
	// The hour (0-23).
	hour optU8
	// The minute (0-59).
	minute optU8
	// The second (0-59).
	second optU8
	// The hour offset from UTC (-23 through 23). hasUTCOffsetHour is false
	// for None.
	utcOffsetHour    int8
	hasUTCOffsetHour bool
	// The minute offset from UTC (0-59). Will carry over the sign from
	// utcOffsetHour.
	utcOffsetMinute uint8
}

// NewDate is Date::new in pdf-writer src/object.rs.
//
// Create a new, minimal date. The year will be clamped within the range
// 0-9999.
func NewDate(year uint16) Date {
	return Date{year: min(year, 9999)}
}

// Month is Date::month in pdf-writer src/object.rs.
//
// Add the month field. It will be clamped within the range 1-12.
func (d Date) Month(month uint8) Date {
	d.month = optU8{min(max(month, 1), 12), true}
	return d
}

// Day is Date::day in pdf-writer src/object.rs.
//
// Add the day field. It will be clamped within the range 1-31.
func (d Date) Day(day uint8) Date {
	d.day = optU8{min(max(day, 1), 31), true}
	return d
}

// Hour is Date::hour in pdf-writer src/object.rs.
//
// Add the hour field. It will be clamped within the range 0-23.
func (d Date) Hour(hour uint8) Date {
	d.hour = optU8{min(hour, 23), true}
	return d
}

// Minute is Date::minute in pdf-writer src/object.rs.
//
// Add the minute field. It will be clamped within the range 0-59.
func (d Date) Minute(minute uint8) Date {
	d.minute = optU8{min(minute, 59), true}
	return d
}

// Second is Date::second in pdf-writer src/object.rs.
//
// Add the second field. It will be clamped within the range 0-59.
func (d Date) Second(second uint8) Date {
	d.second = optU8{min(second, 59), true}
	return d
}

// UTCOffsetHour is Date::utc_offset_hour in pdf-writer src/object.rs.
//
// Add the offset from UTC in hours. If not specified, the time will be
// assumed to be local to the viewer's time zone. It will be clamped within
// the range -23-23.
func (d Date) UTCOffsetHour(hour int8) Date {
	d.utcOffsetHour = min(max(hour, -23), 23)
	d.hasUTCOffsetHour = true
	return d
}

// UTCOffsetMinute is Date::utc_offset_minute in pdf-writer src/object.rs.
//
// Add the offset from UTC in minutes. This will have the same sign as set in
// UTCOffsetHour. It will be clamped within the range 0-59.
func (d Date) UTCOffsetMinute(minute uint8) Date {
	d.utcOffsetMinute = min(minute, 59)
	return d
}

func (Date) startsWithDelimiter() bool { return true }

func (d Date) write(buf *Buf) {
	buf.extendString("(D:")

	func() {
		buf.inner = fmt.Appendf(buf.inner, "%04d", d.year)
		for _, f := range []optU8{d.month, d.day, d.hour, d.minute, d.second} {
			if !f.ok {
				return
			}
			buf.inner = fmt.Appendf(buf.inner, "%02d", f.v)
		}
		if !d.hasUTCOffsetHour {
			return
		}
		if d.utcOffsetHour == 0 && d.utcOffsetMinute == 0 {
			buf.push('Z')
		} else {
			buf.inner = fmt.Appendf(buf.inner, "%+03d'%02d", d.utcOffsetHour, d.utcOffsetMinute)
		}
	}()

	buf.push(')')
}

// Obj is Obj in pdf-writer src/object.rs.
//
// Writer for an arbitrary object.
//
// An Obj writes one object, once. It panics if it writes after another write
// to its buffer, as the borrow checker of Rust forbids that.
type Obj struct {
	buf          *Buf
	indirect     bool
	indent       uint8
	settings     Settings
	needsPadding bool
	seq          uint64
}

// directObj is Obj::direct in pdf-writer src/object.rs.
//
// Start a new direct object.
func directObj(buf *Buf, indent uint8, settings Settings, needsPadding bool) Obj {
	return Obj{
		buf:          buf,
		indirect:     false,
		indent:       indent,
		settings:     settings,
		needsPadding: needsPadding,
		seq:          buf.newObj(),
	}
}

// indirectObj is Obj::indirect in pdf-writer src/object.rs.
//
// Start a new indirect object.
func indirectObj(buf *Buf, id Ref, settings Settings) Obj {
	buf.pushInt(id.Get())
	buf.extendString(" 0 obj\n")
	return Obj{
		buf:          buf,
		indirect:     true,
		indent:       0,
		settings:     settings,
		needsPadding: false,
		seq:          buf.newObj(),
	}
}

// use takes the right to write from the Obj. It panics if the Obj cannot
// write.
func (o Obj) use() {
	o.buf.useObj(o.seq)
}

// Primitive is Obj::primitive in pdf-writer src/object.rs.
//
// Write a primitive object.
func (o Obj) Primitive(value Primitive) {
	o.use()

	// Usually, we need to separate different PDF objects by a whitespace.
	// The key to the optimizations applied here are explained in 7.2.3 in
	// the PDF reference:
	//
	// > The delimiter characters (, ), <, >, [, ], /, and % are special.
	// > They delimit syntactic entities such as arrays, names, and
	// > comments. Any of these delimiters terminates the entity preceding
	// > it and is not included in the entity.
	//
	// Therefore, if either the previous byte is a delimiter character or
	// the current token starts with one, we don't need to add a whitespace
	// for padding.

	inner := o.buf.inner
	endsWithDelimiter := len(inner) > 0 && isDelimiterCharacter(inner[len(inner)-1])

	if o.needsPadding && !value.startsWithDelimiter() && !endsWithDelimiter {
		o.buf.extendString(" ")
	}

	value.write(o.buf)

	if o.indirect {
		o.buf.extendString("\nendobj\n")

		if o.settings.Pretty {
			o.buf.extendString("\n")
		}
	}
}

// Note: Arrays and dictionaries always start with a delimiter, so we don't
// need to do any case distinction, unlike in `primitive`.

// Array is Obj::array in pdf-writer src/object.rs.
//
// Start writing an array.
func (o Obj) Array() *Array {
	a := new(Array)
	a.start(o)
	return a
}

// Dict is Obj::dict in pdf-writer src/object.rs.
//
// Start writing a dictionary.
func (o Obj) Dict() *Dict {
	d := new(Dict)
	d.start(o)
	return d
}

// Start is Obj::start in pdf-writer src/object.rs. Go has no type
// parameters on methods, so it is a function. Write Start[Type1Font](obj)
// where Rust writes obj.start::<Type1Font>().
//
// Start writing with an arbitrary writer.
//
// For example, using this, you could write a Type 1 font directly into a
// page's resource dictionary.
func Start[W any, PW Writer[W]](obj Obj) PW {
	w := PW(new(W))
	w.start(obj)
	return w
}

// Writer is the trait Writer in pdf-writer src/object.rs. W is the writer
// type, and the methods are on *W. The method start takes the place of
// Writer::start, and it starts the writer in place.
//
// A writer for a specific type of PDF object.
//
// The trait Rewrite of upstream changes the lifetimes of writers, which Go
// does not have, so it has no port.
type Writer[W any] interface {
	*W
	// start is Writer::start.
	//
	// Start writing the object.
	start(obj Obj)
}

// Array is Array in pdf-writer src/object.rs.
//
// Writer for an array.
type Array struct {
	buf      *Buf
	indirect bool
	indent   uint8
	settings Settings
	len      int32

	level int
	done  bool
}

// start is the macro writer! for Array in pdf-writer src/object.rs.
func (a *Array) start(obj Obj) {
	obj.use()
	obj.buf.push('[')
	*a = Array{
		buf:      obj.buf,
		indirect: obj.indirect,
		indent:   obj.indent,
		settings: obj.settings,
		len:      0,
	}
	a.level = a.buf.openWriter(a)
}

// enter finishes the writers inside the array, and panics if the array is
// finished.
func (a *Array) enter() {
	if a.done {
		panic("pdfwriter: the Array is finished")
	}
	a.buf.finishFrom(a.level + 1)
}

// Len is Array::len in pdf-writer src/object.rs.
//
// The number of written items.
func (a Array) Len() int32 {
	return a.len
}

// IsEmpty is Array::is_empty in pdf-writer src/object.rs.
//
// Whether no items have been written so far.
func (a Array) IsEmpty() bool {
	return a.len == 0
}

// Push is Array::push in pdf-writer src/object.rs.
//
// Start writing an arbitrary item.
func (a *Array) Push() Obj {
	a.enter()
	needsPadding := false
	if a.len != 0 {
		if a.settings.Pretty {
			a.buf.push(' ')
		} else {
			needsPadding = true
		}
	}

	a.len++

	return directObj(a.buf, a.indent, a.settings, needsPadding)
}

// Item is Array::item in pdf-writer src/object.rs.
//
// Write an item with a primitive value.
//
// This is a shorthand for array.Push().Primitive(value).
func (a *Array) Item(value Primitive) *Array {
	a.Push().Primitive(value)
	return a
}

// Items is Array::items in pdf-writer src/object.rs.
//
// Write a sequence of items with primitive values.
func (a *Array) Items(values ...Primitive) *Array {
	for _, value := range values {
		a.Item(value)
	}
	return a
}

// Finish is the trait Finish in pdf-writer src/object.rs for an Array. It
// writes the end of the array, as Rust does when it drops the array. The
// writers inside the array finish first. Finish does nothing when the array
// is already finished.
func (a *Array) Finish() {
	if !a.done {
		a.buf.finishFrom(a.level)
	}
}

// close is the impl Drop for Array in pdf-writer src/object.rs.
func (a *Array) close() {
	a.done = true
	a.buf.limits.registerArrayLen(int(a.Len()))
	a.buf.push(']')
	if a.indirect {
		a.buf.extendString("\nendobj\n")

		if a.settings.Pretty {
			a.buf.extendString("\n")
		}
	}
}

// TypedArray is TypedArray in pdf-writer src/object.rs, for items that are
// primitive objects. WriterArray is the TypedArray of upstream for items
// that a writer writes.
//
// Writer for an array of items of a fixed type.
type TypedArray[T Primitive] struct {
	array *Array
}

// start is the impl Writer for TypedArray in pdf-writer src/object.rs.
func (a *TypedArray[T]) start(obj Obj) {
	a.array = obj.Array()
}

// NewTypedArray is TypedArray::wrap and Array::typed in pdf-writer
// src/object.rs.
//
// Wrap an array to make it type-safe.
func NewTypedArray[T Primitive](array *Array) *TypedArray[T] {
	return &TypedArray[T]{array: array}
}

// Len is TypedArray::len in pdf-writer src/object.rs.
//
// The number of written items.
func (a TypedArray[T]) Len() int32 {
	return a.array.Len()
}

// IsEmpty is TypedArray::is_empty in pdf-writer src/object.rs.
//
// Whether no items have been written so far.
func (a TypedArray[T]) IsEmpty() bool {
	return a.Len() == 0
}

// Item is TypedArray::item in pdf-writer src/object.rs.
//
// Write an item.
func (a *TypedArray[T]) Item(value T) *TypedArray[T] {
	a.array.Item(value)
	return a
}

// Items is TypedArray::items in pdf-writer src/object.rs.
//
// Write a sequence of items.
func (a *TypedArray[T]) Items(values ...T) *TypedArray[T] {
	for _, value := range values {
		a.array.Item(value)
	}
	return a
}

// Finish finishes the array, as Array.Finish does.
func (a *TypedArray[T]) Finish() {
	a.array.Finish()
}

// WriterArray is TypedArray in pdf-writer src/object.rs, for items that a
// writer of the type W writes, such as TypedArray<FileSpec>. Go has no
// methods that only some type arguments have, so the port splits the type
// in two: TypedArray for primitive items, and WriterArray for writers.
//
// Writer for an array of items of a fixed type.
type WriterArray[W any, PW Writer[W]] struct {
	array *Array
}

// start is the impl Writer for TypedArray in pdf-writer src/object.rs.
func (a *WriterArray[W, PW]) start(obj Obj) {
	a.array = obj.Array()
}

// NewWriterArray is TypedArray::wrap and Array::typed in pdf-writer
// src/object.rs, for a WriterArray.
//
// Wrap an array to make it type-safe.
func NewWriterArray[W any, PW Writer[W]](array *Array) *WriterArray[W, PW] {
	return &WriterArray[W, PW]{array: array}
}

// Len is TypedArray::len in pdf-writer src/object.rs.
//
// The number of written items.
func (a WriterArray[W, PW]) Len() int32 {
	return a.array.Len()
}

// IsEmpty is TypedArray::is_empty in pdf-writer src/object.rs.
//
// Whether no items have been written so far.
func (a WriterArray[W, PW]) IsEmpty() bool {
	return a.Len() == 0
}

// Push is TypedArray::push in pdf-writer src/object.rs.
//
// Start writing an item with the typed writer.
func (a *WriterArray[W, PW]) Push() PW {
	return Start[W, PW](a.array.Push())
}

// Finish finishes the array, as Array.Finish does.
func (a *WriterArray[W, PW]) Finish() {
	a.array.Finish()
}

// Dict is Dict in pdf-writer src/object.rs.
//
// Writer for a dictionary.
type Dict struct {
	buf      *Buf
	indirect bool
	indent   uint8
	settings Settings
	len      int32

	level int
	done  bool
	// isStream is true for the dictionary of a Stream, which ends with
	// data. It takes the place of the ManuallyDrop of upstream.
	isStream bool
	data     []byte
}

// start is the macro writer! for Dict in pdf-writer src/object.rs.
func (d *Dict) start(obj Obj) {
	obj.use()
	obj.buf.extendString("<<")
	indent := obj.indent
	if indent > math.MaxUint8-2 {
		indent = math.MaxUint8
	} else {
		indent += 2
	}
	*d = Dict{
		buf:      obj.buf,
		indirect: obj.indirect,
		indent:   indent,
		settings: obj.settings,
		len:      0,
	}
	d.level = d.buf.openWriter(d)
}

// enter finishes the writers inside the dictionary, and panics if the
// dictionary is finished.
func (d *Dict) enter() {
	if d.done {
		panic("pdfwriter: the Dict is finished")
	}
	d.buf.finishFrom(d.level + 1)
}

// Len is Dict::len in pdf-writer src/object.rs.
//
// The number of written pairs.
func (d Dict) Len() int32 {
	return d.len
}

// IsEmpty is Dict::is_empty in pdf-writer src/object.rs.
//
// Whether no pairs have been written so far.
func (d Dict) IsEmpty() bool {
	return d.len == 0
}

// Insert is Dict::insert in pdf-writer src/object.rs.
//
// Start writing a pair with an arbitrary value.
func (d *Dict) Insert(key Name) Obj {
	d.enter()
	d.len++

	// Keys always start with a delimiter since they are names, so we never
	// need padding unless `pretty` is activated.
	if d.settings.Pretty {
		d.buf.push('\n')

		for range d.indent {
			d.buf.push(' ')
		}
	}

	d.buf.pushVal(key)

	needsPadding := true
	if d.settings.Pretty {
		d.buf.push(' ')
		needsPadding = false
	}

	return directObj(d.buf, d.indent, d.settings, needsPadding)
}

// Pair is Dict::pair in pdf-writer src/object.rs.
//
// Write a pair with a primitive value.
//
// This is a shorthand for dict.Insert(key).Primitive(value).
func (d *Dict) Pair(key Name, value Primitive) *Dict {
	d.Insert(key).Primitive(value)
	return d
}

// Pairs is Dict::pairs in pdf-writer src/object.rs.
//
// Write a sequence of pairs with primitive values.
func (d *Dict) Pairs(pairs iter.Seq2[Name, Primitive]) *Dict {
	for key, value := range pairs {
		d.Pair(key, value)
	}
	return d
}

// Finish is the trait Finish in pdf-writer src/object.rs for a Dict. It
// writes the end of the dictionary, as Rust does when it drops the
// dictionary. The writers inside the dictionary finish first. Finish does
// nothing when the dictionary is already finished.
func (d *Dict) Finish() {
	if !d.done {
		d.buf.finishFrom(d.level)
	}
}

// close is the impl Drop for Dict in pdf-writer src/object.rs, or the impl
// Drop for Stream for the dictionary of a stream.
func (d *Dict) close() {
	d.done = true
	if d.isStream {
		d.closeStream()
		return
	}
	d.buf.limits.registerDictEntries(int(d.len))

	if d.len != 0 && d.settings.Pretty {
		d.buf.push('\n')
		for range d.indent - 2 {
			d.buf.push(' ')
		}
	}

	d.buf.extendString(">>")

	if d.indirect {
		d.buf.extendString("\nendobj\n")

		if d.settings.Pretty {
			d.buf.extendString("\n")
		}
	}
}

// TypedDict is TypedDict in pdf-writer src/object.rs, for values that are
// primitive objects. WriterDict is the TypedDict of upstream for values that
// a writer writes.
//
// Writer for a dictionary mapping to a fixed type.
type TypedDict[T Primitive] struct {
	dict *Dict
}

// start is the impl Writer for TypedDict in pdf-writer src/object.rs.
func (d *TypedDict[T]) start(obj Obj) {
	d.dict = obj.Dict()
}

// NewTypedDict is TypedDict::wrap and Dict::typed in pdf-writer
// src/object.rs.
//
// Wrap a dictionary to make it type-safe.
func NewTypedDict[T Primitive](dict *Dict) *TypedDict[T] {
	return &TypedDict[T]{dict: dict}
}

// Len is TypedDict::len in pdf-writer src/object.rs.
//
// The number of written pairs.
func (d TypedDict[T]) Len() int32 {
	return d.dict.Len()
}

// IsEmpty is TypedDict::is_empty in pdf-writer src/object.rs.
//
// Whether no pairs have been written so far.
func (d TypedDict[T]) IsEmpty() bool {
	return d.Len() == 0
}

// Pair is TypedDict::pair in pdf-writer src/object.rs.
//
// Write a key-value pair.
func (d *TypedDict[T]) Pair(key Name, value T) *TypedDict[T] {
	d.dict.Pair(key, value)
	return d
}

// Pairs is TypedDict::pairs in pdf-writer src/object.rs.
//
// Write a sequence of key-value pairs.
func (d *TypedDict[T]) Pairs(pairs iter.Seq2[Name, T]) *TypedDict[T] {
	for key, value := range pairs {
		d.dict.Pair(key, value)
	}
	return d
}

// Finish finishes the dictionary, as Dict.Finish does.
func (d *TypedDict[T]) Finish() {
	d.dict.Finish()
}

// WriterDict is TypedDict in pdf-writer src/object.rs, for values that a
// writer of the type W writes, such as TypedDict<Destination>. It is to
// TypedDict what WriterArray is to TypedArray.
//
// Writer for a dictionary mapping to a fixed type.
type WriterDict[W any, PW Writer[W]] struct {
	dict *Dict
}

// start is the impl Writer for TypedDict in pdf-writer src/object.rs.
func (d *WriterDict[W, PW]) start(obj Obj) {
	d.dict = obj.Dict()
}

// NewWriterDict is TypedDict::wrap and Dict::typed in pdf-writer
// src/object.rs, for a WriterDict.
//
// Wrap a dictionary to make it type-safe.
func NewWriterDict[W any, PW Writer[W]](dict *Dict) *WriterDict[W, PW] {
	return &WriterDict[W, PW]{dict: dict}
}

// Len is TypedDict::len in pdf-writer src/object.rs.
//
// The number of written pairs.
func (d WriterDict[W, PW]) Len() int32 {
	return d.dict.Len()
}

// IsEmpty is TypedDict::is_empty in pdf-writer src/object.rs.
//
// Whether no pairs have been written so far.
func (d WriterDict[W, PW]) IsEmpty() bool {
	return d.Len() == 0
}

// Insert is TypedDict::insert in pdf-writer src/object.rs.
//
// Start writing a pair with the typed writer.
func (d *WriterDict[W, PW]) Insert(key Name) PW {
	return Start[W, PW](d.dict.Insert(key))
}

// Finish finishes the dictionary, as Dict.Finish does.
func (d *WriterDict[W, PW]) Finish() {
	d.dict.Finish()
}

// Stream is Stream in pdf-writer src/object.rs. It holds the dictionary of
// the stream, and the dictionary holds the data. The methods of Dict write
// the pairs of the dictionary, as the impl Deref of upstream gives them.
//
// Writer for an indirect stream object.
type Stream struct {
	*Dict
}

// startStream is Stream::start in pdf-writer src/object.rs.
//
// Start writing a stream.
//
// Panics if the object writer is not indirect or the stream length exceeds
// math.MaxInt32.
func startStream(obj Obj, data []byte) *Stream {
	if !obj.indirect {
		panic("assertion failed: obj.indirect")
	}

	dict := obj.Dict()
	if int64(len(data)) > math.MaxInt32 {
		panic(fmt.Sprintf("data length (is `%d`) must be <= i32::MAX", len(data)))
	}
	dict.Pair(Name("Length"), Int(len(data)))

	dict.isStream = true
	dict.data = data
	return &Stream{Dict: dict}
}

// Filter is Stream::filter in pdf-writer src/object.rs.
//
// Write the /Filter attribute.
func (s *Stream) Filter(filter Filter) *Stream {
	s.Pair(Name("Filter"), filter.toName())
	return s
}

// DecodeParms is Stream::decode_parms in pdf-writer src/object.rs.
//
// Start writing the /DecodeParms attribute.
//
// This is a dictionary that specifies parameters to be used in decoding the
// stream data using the filter specified by the /Filter attribute.
func (s *Stream) DecodeParms() *DecodeParms {
	return Start[DecodeParms](s.Insert(Name("DecodeParms")))
}

// closeStream is the impl Drop for Stream in pdf-writer src/object.rs.
func (d *Dict) closeStream() {
	dictLen := int(d.len)
	d.buf.limits.registerDictEntries(dictLen)

	if d.settings.Pretty {
		d.buf.extendString("\n")
	}

	d.buf.extendString(">>")
	d.buf.extendString("\nstream\n")
	d.buf.extend(d.data)
	d.buf.extendString("\nendstream")
	d.buf.extendString("\nendobj\n")

	if d.settings.Pretty {
		d.buf.extendString("\n")
	}
}

// Filter is Filter in pdf-writer src/object.rs.
//
// A compression filter for a stream.
type Filter uint8

// The values of Filter. The Go names spell the initialisms in capitals, so
// that FilterASCIIHexDecode is Filter::AsciiHexDecode.
const (
	FilterASCIIHexDecode Filter = iota
	FilterASCII85Decode
	// FilterLZWDecode is Lempel-Ziv-Welch (LZW) compression.
	//
	// Note that this filter is forbidden in PDF/A.
	FilterLZWDecode
	FilterFlateDecode
	FilterRunLengthDecode
	FilterCCITTFaxDecode
	FilterJBIG2Decode
	// FilterDCTDecode decodes JPEG/JFIF files with a SOF0, SOF1, or (PDF
	// 1.3+) SOF2 marker.
	//
	// See ISO 32000-1:2008, Section 7.4.8 and Adobe Technical Note #5116.
	FilterDCTDecode
	// FilterJPXDecode decodes JPEG2000 files with a JPX baseline marker.
	//
	// Note that additional restrictions are imposed by PDF/A and PDF/X.
	FilterJPXDecode
	// FilterCrypt encrypts the stream.
	//
	// Note that this filter is restricted in PDF/A.
	FilterCrypt
)

// String returns the PDF name of the filter, without the slash.
func (f Filter) String() string {
	return string(f.toName())
}

// toName is Filter::to_name in pdf-writer src/object.rs.
func (f Filter) toName() Name {
	switch f {
	case FilterASCIIHexDecode:
		return Name("ASCIIHexDecode")
	case FilterASCII85Decode:
		return Name("ASCII85Decode")
	case FilterLZWDecode:
		return Name("LZWDecode")
	case FilterFlateDecode:
		return Name("FlateDecode")
	case FilterRunLengthDecode:
		return Name("RunLengthDecode")
	case FilterCCITTFaxDecode:
		return Name("CCITTFaxDecode")
	case FilterJBIG2Decode:
		return Name("JBIG2Decode")
	case FilterDCTDecode:
		return Name("DCTDecode")
	case FilterJPXDecode:
		return Name("JPXDecode")
	case FilterCrypt:
		return Name("Crypt")
	}
	panic(fmt.Sprintf("pdfwriter: the filter %d is not known", uint8(f)))
}

// DecodeParms is DecodeParms in pdf-writer src/object.rs.
//
// Writer for a filter decode parameters dictionary.
//
// This struct is created by Stream.DecodeParms.
type DecodeParms struct {
	*Dict
}

// start is the macro writer! for DecodeParms in pdf-writer src/object.rs.
func (p *DecodeParms) start(obj Obj) {
	p.Dict = obj.Dict()
}

// Properties for `FlateDecode` and `LzwDecode`.

// Predictor is DecodeParms::predictor in pdf-writer src/object.rs.
//
// Write the /Predictor attribute for FlateDecode and LzwDecode.
//
// No predictor is used by default.
func (p *DecodeParms) Predictor(predictor Predictor) *DecodeParms {
	p.Pair(Name("Predictor"), Int(predictor.toI32()))
	return p
}

// Colors is DecodeParms::colors in pdf-writer src/object.rs.
//
// Write the /Colors attribute for FlateDecode and LzwDecode.
//
// Must be greater than 0. /Predictor must be set. Defaults to 1.
func (p *DecodeParms) Colors(colors int32) *DecodeParms {
	if colors <= 0 {
		panic("`Colors` must be greater than 0")
	}

	p.Pair(Name("Colors"), Int(colors))
	return p
}

// BitsPerComponent is DecodeParms::bits_per_component in pdf-writer
// src/object.rs.
//
// Write the /BitsPerComponent attribute for FlateDecode and LzwDecode.
//
// Must be one of 1, 2, 4, 8, or 16. /Predictor must be set. Defaults to 8.
func (p *DecodeParms) BitsPerComponent(bits int32) *DecodeParms {
	switch bits {
	case 1, 2, 4, 8, 16:
	default:
		panic("`BitsPerComponent` must be one of 1, 2, 4, 8, or 16")
	}

	p.Pair(Name("BitsPerComponent"), Int(bits))
	return p
}

// Columns is DecodeParms::columns in pdf-writer src/object.rs.
//
// Write the /Columns attribute for FlateDecode and LzwDecode or
// CcittFaxDecode.
//
// When used with FlateDecode and LzwDecode, it indicates the number of
// samples in each row. In that case, /Predictor must be set and the default
// is 1.
//
// When used with CcittFaxDecode it denominates the width of the image in
// pixels and defaults to 1728.
func (p *DecodeParms) Columns(columns int32) *DecodeParms {
	p.Pair(Name("Columns"), Int(columns))
	return p
}

// EarlyChange is DecodeParms::early_change in pdf-writer src/object.rs.
//
// Write the /EarlyChange attribute for LzwDecode.
//
// If true (1), the code length increases one code earlier, if false (0),
// length change is postponed as long as possible.
//
// Defaults to 1.
func (p *DecodeParms) EarlyChange(earlyChange bool) *DecodeParms {
	v := Int(0)
	if earlyChange {
		v = 1
	}
	p.Pair(Name("EarlyChange"), v)
	return p
}

// Properties for `CcittFaxDecode`. Also see [`Self::columns`].

// K is DecodeParms::k in pdf-writer src/object.rs.
//
// Write the /K attribute for CcittFaxDecode.
//
// Defaults to 0.
func (p *DecodeParms) K(k int32) *DecodeParms {
	p.Pair(Name("K"), Int(k))
	return p
}

// EndOfLine is DecodeParms::end_of_line in pdf-writer src/object.rs.
//
// Write the /EndOfLine attribute for CcittFaxDecode.
//
// Whether the EOL bit pattern is present in the encoding. Defaults to
// false.
func (p *DecodeParms) EndOfLine(eol bool) *DecodeParms {
	p.Pair(Name("EndOfLine"), Bool(eol))
	return p
}

// EncodedByteAlign is DecodeParms::encoded_byte_align in pdf-writer
// src/object.rs.
//
// Write the /EncodedByteAlign attribute for CcittFaxDecode.
//
// Whether to expect zero bits before each encoded line. Defaults to false.
func (p *DecodeParms) EncodedByteAlign(encodedByteAlign bool) *DecodeParms {
	p.Pair(Name("EncodedByteAlign"), Bool(encodedByteAlign))
	return p
}

// Rows is DecodeParms::rows in pdf-writer src/object.rs.
//
// Write the /Rows attribute for CcittFaxDecode.
//
// The image's height. Defaults to 0.
func (p *DecodeParms) Rows(rows int32) *DecodeParms {
	p.Pair(Name("Rows"), Int(rows))
	return p
}

// EndOfBlock is DecodeParms::end_of_block in pdf-writer src/object.rs.
//
// Write the /EndOfBlock attribute for CcittFaxDecode.
//
// Whether to expect an EOB code at the end of the data. Defaults to true.
func (p *DecodeParms) EndOfBlock(endOfBlock bool) *DecodeParms {
	p.Pair(Name("EndOfBlock"), Bool(endOfBlock))
	return p
}

// BlackIs1 is DecodeParms::black_is_1 in pdf-writer src/object.rs.
//
// Write the /BlackIs1 attribute for CcittFaxDecode.
//
// Whether to invert the bits in the image. Defaults to false.
func (p *DecodeParms) BlackIs1(blackIs1 bool) *DecodeParms {
	p.Pair(Name("BlackIs1"), Bool(blackIs1))
	return p
}

// DamagedRowsBeforeError is DecodeParms::damaged_rows_before_error in
// pdf-writer src/object.rs.
//
// Write the /DamagedRowsBeforeError attribute for CcittFaxDecode.
//
// How many damaged rows are allowed before an error is raised. Defaults to
// 0.
func (p *DecodeParms) DamagedRowsBeforeError(count int32) *DecodeParms {
	p.Pair(Name("DamagedRowsBeforeError"), Int(count))
	return p
}

// Properties for `Jbig2Decode`.

// Jbig2Globals is DecodeParms::jbig2_globals in pdf-writer src/object.rs.
//
// Write the /JBIG2Globals attribute for Jbig2Decode.
//
// A reference to a stream containing global segments.
func (p *DecodeParms) Jbig2Globals(globals Ref) *DecodeParms {
	p.Pair(Name("JBIG2Globals"), globals)
	return p
}

// Properties for `JpxDecode`.

// ColorTransform is DecodeParms::color_transform in pdf-writer
// src/object.rs.
//
// Write the /ColorTransform attribute for JpxDecode.
//
// How to handle color data. If true (1), images with three color channels
// shall be decoded from the YCbCr space and images with four color channels
// are decoded from YCbCrK. If false (0), no transformation is applied. The
// default depends on the APP14 marker in the data stream.
func (p *DecodeParms) ColorTransform(colorTransform bool) *DecodeParms {
	v := Int(0)
	if colorTransform {
		v = 1
	}
	p.Pair(Name("ColorTransform"), v)
	return p
}

// Properties for `Crypt`.

// CryptType is DecodeParms::crypt_type in pdf-writer src/object.rs.
//
// Write the /Type attribute for Crypt as CryptFilterDecodeParms.
func (p *DecodeParms) CryptType() *DecodeParms {
	p.Pair(Name("Type"), Name("CryptFilterDecodeParms"))
	return p
}

// Name is DecodeParms::name in pdf-writer src/object.rs.
//
// Write the /Name attribute for Crypt.
//
// The name of the crypt filter corresponding to a CF entry of the encryption
// dictionary.
func (p *DecodeParms) Name(name Name) *DecodeParms {
	p.Pair(Name("Name"), name)
	return p
}

// Predictor is Predictor in pdf-writer src/object.rs. The zero value is
// PredictorNone, the default of upstream.
//
// Which kind of predictor to use for a FlateDecode or LzwDecode stream.
type Predictor uint8

// The values of Predictor.
const (
	// PredictorNone is no prediction.
	PredictorNone Predictor = iota
	// PredictorTiff is TIFF Predictor 2.
	PredictorTiff
	PredictorPngNone
	PredictorPngSub
	PredictorPngUp
	PredictorPngAverage
	PredictorPngPaeth
	PredictorPngOptimum
)

// predictorNames holds the Rust name of each value of Predictor.
var predictorNames = [...]string{
	PredictorNone:       "None",
	PredictorTiff:       "Tiff",
	PredictorPngNone:    "PngNone",
	PredictorPngSub:     "PngSub",
	PredictorPngUp:      "PngUp",
	PredictorPngAverage: "PngAverage",
	PredictorPngPaeth:   "PngPaeth",
	PredictorPngOptimum: "PngOptimum",
}

// String returns the Rust name of the predictor.
func (p Predictor) String() string {
	if int(p) < len(predictorNames) {
		return predictorNames[p]
	}
	return "Predictor(?)"
}

// toI32 is Predictor::to_i32 in pdf-writer src/object.rs.
//
// Convert the predictor to its integer representation according to ISO
// 32000-2:2020, Table E.
func (p Predictor) toI32() int32 {
	switch p {
	case PredictorTiff:
		return 2
	case PredictorPngNone:
		return 10
	case PredictorPngSub:
		return 11
	case PredictorPngUp:
		return 12
	case PredictorPngAverage:
		return 13
	case PredictorPngPaeth:
		return 14
	case PredictorPngOptimum:
		return 15
	}
	return 1
}

// NameTree is NameTree in pdf-writer src/object.rs.
//
// Writer for a name tree node.
//
// Name trees associate a large number of names with PDF objects. They are
// lexically ordered search trees. Root nodes may directly contain all leafs,
// however, this might degrade performance for very large numbers of
// name-object pairs.
//
// For each node, either the /Kids or /Names attribute must be set, but never
// both.
type NameTree[T Primitive] struct {
	dict *Dict
}

// start is the impl Writer for NameTree in pdf-writer src/object.rs.
func (t *NameTree[T]) start(obj Obj) {
	t.dict = obj.Dict()
}

// Kids is NameTree::kids in pdf-writer src/object.rs.
//
// Start writing the /Kids attribute with the children of this node.
func (t *NameTree[T]) Kids() *TypedArray[Ref] {
	return NewTypedArray[Ref](t.dict.Insert(Name("Kids")).Array())
}

// Names is NameTree::names in pdf-writer src/object.rs.
//
// Start writing the /Names attribute to set the immediate name-to-object
// mappings of this node.
func (t *NameTree[T]) Names() *NameTreeEntries[T] {
	return Start[NameTreeEntries[T]](t.dict.Insert(Name("Names")))
}

// Limits is NameTree::limits in pdf-writer src/object.rs.
//
// Write the /Limits array to set the range of names in this node. This is
// required for every node except the root node.
func (t *NameTree[T]) Limits(min, max Name) *NameTree[T] {
	NewTypedArray[Name](t.dict.Insert(Name("Limits")).Array()).Items(min, max)
	return t
}

// Finish finishes the node, as Dict.Finish does.
func (t *NameTree[T]) Finish() {
	t.dict.Finish()
}

// NameTreeEntries is NameTreeEntries in pdf-writer src/object.rs.
//
// Writer for a name tree names array.
//
// The children must be added in ascending lexical order. Their minimum and
// maximum keys must not exceed the /Limits property of the parent NameTree
// node. This struct is created by NameTree.Names.
type NameTreeEntries[T Primitive] struct {
	arr *Array
}

// start is the impl Writer for NameTreeEntries in pdf-writer src/object.rs.
func (e *NameTreeEntries[T]) start(obj Obj) {
	e.arr = obj.Array()
}

// Insert is NameTreeEntries::insert in pdf-writer src/object.rs.
//
// Insert a name-value pair.
func (e *NameTreeEntries[T]) Insert(key Str, value T) *NameTreeEntries[T] {
	e.arr.Item(key)
	e.arr.Item(value)
	return e
}

// Finish finishes the array, as Array.Finish does.
func (e *NameTreeEntries[T]) Finish() {
	e.arr.Finish()
}

// NumberTree is NumberTree in pdf-writer src/object.rs.
//
// Writer for a number tree node.
//
// Number trees associate a many integers with PDF objects. They are search
// trees in ascending order. Root nodes may directly contain all leafs,
// however, this might degrade performance for very large numbers of
// integer-object pairs.
//
// For each node, either the /Kids or /Nums attribute must be set, but never
// both.
type NumberTree[T Primitive] struct {
	dict *Dict
}

// start is the impl Writer for NumberTree in pdf-writer src/object.rs.
func (t *NumberTree[T]) start(obj Obj) {
	t.dict = obj.Dict()
}

// Kids is NumberTree::kids in pdf-writer src/object.rs.
//
// Start writing the /Kids attribute with the children of this node.
func (t *NumberTree[T]) Kids() *TypedArray[Ref] {
	return NewTypedArray[Ref](t.dict.Insert(Name("Kids")).Array())
}

// Nums is NumberTree::nums in pdf-writer src/object.rs.
//
// Start writing the /Nums attribute to set the immediate number-to-object
// mappings of this node.
func (t *NumberTree[T]) Nums() *NumberTreeEntries[T] {
	return Start[NumberTreeEntries[T]](t.dict.Insert(Name("Nums")))
}

// Limits is NumberTree::limits in pdf-writer src/object.rs.
//
// Write the /Limits array to set the range of numbers in this node. This is
// required for every node except the root node.
func (t *NumberTree[T]) Limits(min, max int32) *NumberTree[T] {
	NewTypedArray[Int](t.dict.Insert(Name("Limits")).Array()).Items(Int(min), Int(max))
	return t
}

// Finish finishes the node, as Dict.Finish does.
func (t *NumberTree[T]) Finish() {
	t.dict.Finish()
}

// NumberTreeEntries is NumberTreeEntries in pdf-writer src/object.rs.
//
// Writer for a number tree numbers array.
//
// The children must be added in ascending order. Their minimum and maximum
// keys must not exceed the /Limits property of the parent NumberTree node.
// This struct is created by NumberTree.Nums.
type NumberTreeEntries[T Primitive] struct {
	arr *Array
}

// start is the impl Writer for NumberTreeEntries in pdf-writer
// src/object.rs.
func (e *NumberTreeEntries[T]) start(obj Obj) {
	e.arr = obj.Array()
}

// Insert is NumberTreeEntries::insert in pdf-writer src/object.rs.
//
// Insert a number-value pair.
func (e *NumberTreeEntries[T]) Insert(key int32, value T) *NumberTreeEntries[T] {
	e.arr.Item(Int(key))
	e.arr.Item(value)
	return e
}

// Finish finishes the array, as Array.Finish does.
func (e *NumberTreeEntries[T]) Finish() {
	e.arr.Finish()
}

// The trait Finish of upstream drops a writer. Each writer of the port has
// the method Finish, which writes the end of the object.

// itemsF32 writes each value as a Real item of the array, as Array::items
// does in Rust for an iterator of f32. The writers of several files use it.
func itemsF32(a *Array, values []float32) {
	for _, v := range values {
		a.Item(Real(v))
	}
}
