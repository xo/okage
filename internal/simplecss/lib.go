// Copyright 2016 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"iter"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/xo/okage"
)

// cssError is the enum Error in simplecss src/lib.rs. It is a list of
// possible errors.
//
// Port: each case of upstream is a sentinel error of okage (D21), such as
// okage.ErrCSSInvalidIdent, which kind holds. The other fields hold the data
// of the case. Error gives the text of upstream, and Unwrap gives the
// sentinel error, so that errors.Is finds it. The cases are:
//
//   - ErrCSSUnexpectedEndOfStream: the stream ended earlier than we
//     expected. Should only appear on invalid input data.
//   - ErrCSSInvalidIdent: an invalid ident, at pos.
//   - ErrCSSInvalidComment: an unclosed comment, at pos.
//   - ErrCSSInvalidValue: an invalid declaration value, at pos.
//   - ErrCSSInvalidByte: an invalid byte, with expected, actual and pos.
//   - ErrCSSSelectorMissing: a missing selector.
//   - ErrCSSUnexpectedSelector: an unexpected selector.
//   - ErrCSSUnexpectedCombinator: an unexpected combinator.
//   - ErrCSSInvalidAttributeSelector: an invalid or unsupported attribute
//     selector.
//   - ErrCSSInvalidLanguagePseudoClass: an invalid language pseudo-class.
type cssError struct {
	kind     okage.Error
	pos      TextPos
	expected byte
	actual   byte
}

// newError returns the error of a case of the enum Error in simplecss
// src/lib.rs that holds no data.
func newError(kind okage.Error) error {
	return &cssError{kind: kind}
}

// newErrorAt returns the error of a case of the enum Error in simplecss
// src/lib.rs that holds a TextPos.
func newErrorAt(kind okage.Error, pos TextPos) error {
	return &cssError{kind: kind, pos: pos}
}

// newErrorInvalidByte returns Error::InvalidByte in simplecss src/lib.rs.
func newErrorInvalidByte(expected, actual byte, pos TextPos) error {
	return &cssError{kind: okage.ErrCSSInvalidByte, pos: pos, expected: expected, actual: actual}
}

// Error is fmt::Display for Error in simplecss src/lib.rs.
func (e *cssError) Error() string {
	switch e.kind {
	case okage.ErrCSSUnexpectedEndOfStream:
		return "unexpected end of stream"
	case okage.ErrCSSInvalidIdent:
		return "invalid ident at " + e.pos.String()
	case okage.ErrCSSInvalidComment:
		return "invalid comment at " + e.pos.String()
	case okage.ErrCSSInvalidValue:
		return "invalid value at " + e.pos.String()
	case okage.ErrCSSInvalidByte:
		return "expected '" + string(rune(e.expected)) + "' not '" + string(rune(e.actual)) + "' at " + e.pos.String()
	case okage.ErrCSSSelectorMissing:
		return "selector missing"
	case okage.ErrCSSUnexpectedSelector:
		return "unexpected selector"
	case okage.ErrCSSUnexpectedCombinator:
		return "unexpected combinator"
	case okage.ErrCSSInvalidAttributeSelector:
		return "invalid or unsupported attribute selector"
	case okage.ErrCSSInvalidLanguagePseudoClass:
		return "invalid language pseudo-class"
	}
	return string(e.kind)
}

// Unwrap returns the sentinel error of okage for the case of e.
func (e *cssError) Unwrap() error {
	return e.kind
}

// TextPos is TextPos in simplecss src/lib.rs. It is a position in text.
//
// Position indicates a row/line and a column in the original text. Starting
// from 1:1.
type TextPos struct {
	Row uint32
	Col uint32
}

// NewTextPos is TextPos::new in simplecss src/lib.rs. It constructs a new
// TextPos.
//
// Should not be invoked manually, but rather via Stream::gen_text_pos.
func NewTextPos(row, col uint32) TextPos {
	return TextPos{Row: row, Col: col}
}

// String is fmt::Display for TextPos in simplecss src/lib.rs.
func (p TextPos) String() string {
	return strconv.FormatUint(uint64(p.Row), 10) + ":" + strconv.FormatUint(uint64(p.Col), 10)
}

// Declaration is Declaration in simplecss src/lib.rs. It is a declaration.
type Declaration struct {
	Name      string
	Value     string
	Important bool
}

// Rule is Rule in simplecss src/lib.rs. It is a rule.
type Rule struct {
	// Selector is a rule selector.
	Selector Selector
	// Declarations are a rule declarations.
	Declarations []Declaration
}

// StyleSheet is StyleSheet in simplecss src/lib.rs. It is a style sheet.
//
// The zero value is an empty style sheet, as Default for StyleSheet gives.
type StyleSheet struct {
	// Rules is a list of rules.
	Rules []Rule
}

// NewStyleSheet is StyleSheet::new in simplecss src/lib.rs. It creates an
// empty style sheet.
func NewStyleSheet() StyleSheet {
	return StyleSheet{Rules: nil}
}

// ParseStyleSheet is StyleSheet::parse in simplecss src/lib.rs. It parses a
// style sheet from text.
//
// At-rules are not supported and will be skipped.
//
// Doesn't produce any errors. In worst case scenario will return an empty
// stylesheet.
//
// All warnings will be logged, to the logger that WithLogger gives.
func ParseStyleSheet(text string, opts ...Option) StyleSheet {
	sheet := NewStyleSheet()
	sheet.ParseMore(text, opts...)
	return sheet
}

// ParseMore is StyleSheet::parse_more in simplecss src/lib.rs. It parses a
// style sheet from a text to the current style sheet.
//
// All warnings will be logged, to the logger that WithLogger gives.
func (ss *StyleSheet) ParseMore(text string, opts ...Option) {
	o := newOptions(opts)
	s := newStream(text)

	if s.SkipSpacesAndComments() != nil {
		return
	}

	for !s.AtEnd() {
		if s.SkipSpacesAndComments() != nil {
			break
		}

		_ = consumeStatement(&s, &ss.Rules, o.logger)
	}

	if !s.AtEnd() {
		warnf(o.logger, "%d bytes were left.", len(s.SliceTail()))
	}

	// Remove empty rules.
	ss.Rules = slices.DeleteFunc(ss.Rules, func(rule Rule) bool {
		return len(rule.Declarations) == 0
	})

	// Sort the rules by specificity.
	//
	// Port: sort_by_cached_key keeps the order of equal elements, so the port
	// sorts with a stable sort.
	slices.SortStableFunc(ss.Rules, func(a, b Rule) int {
		sa, sb := a.Selector.Specificity(), b.Selector.Specificity()
		return slices.Compare(sa[:], sb[:])
	})
}

// String is fmt::Display for StyleSheet in simplecss src/lib.rs.
func (ss StyleSheet) String() string {
	var f strings.Builder
	for i, rule := range ss.Rules {
		f.WriteString(rule.Selector.String())
		f.WriteString(" { ")
		for _, dec := range rule.Declarations {
			f.WriteString(dec.Name)
			f.WriteByte(':')
			f.WriteString(dec.Value)
			if dec.Important {
				f.WriteString(" !important")
			}
			f.WriteByte(';')
		}
		f.WriteString(" }")

		if i != len(ss.Rules)-1 {
			f.WriteByte('\n')
		}
	}

	return f.String()
}

// consumeStatement is consume_statement in simplecss src/lib.rs.
func consumeStatement(s *stream, rules *[]Rule, l *slog.Logger) error {
	if s.isCurrByte('@') {
		s.Advance(1)
		return consumeAtRule(s, l)
	}
	return consumeRuleSet(s, rules, l)
}

// consumeAtRule is consume_at_rule in simplecss src/lib.rs.
func consumeAtRule(s *stream, l *slog.Logger) error {
	ident, err := s.ConsumeIdent()
	if err != nil {
		return err
	}
	warnf(l, "The @%s rule is not supported. Skipped.", ident)

	s.SkipBytes(func(c byte) bool { return c != ';' && c != '{' })

	c, err := s.CurrByte()
	if err != nil {
		return err
	}
	switch c {
	case ';':
		s.Advance(1)
	case '{':
		consumeBlock(s)
	}

	return nil
}

// consumeRuleSet is consume_rule_set in simplecss src/lib.rs.
func consumeRuleSet(s *stream, rules *[]Rule, l *slog.Logger) error {
	startRuleIdx := len(*rules)

loop:
	for {
		c, err := s.CurrByte()
		if err != nil {
			return err
		}
		if c != ',' && startRuleIdx != len(*rules) {
			break
		}

		if c == ',' {
			s.Advance(1)
		}

		selector, ok, offset := parse(s.SliceTail(), l)
		s.Advance(offset)
		s.SkipSpaces()

		if ok {
			*rules = append(*rules, Rule{
				Selector:     selector,
				Declarations: nil,
			})
		}

		c, err = s.CurrByte()
		if err != nil {
			return err
		}
		switch c {
		case '{':
			break loop
		case ',':
		default:
			s.SkipBytes(func(c byte) bool { return c != '{' })
			break loop
		}
	}

	s.TryConsumeByte('{')

	declarations, err := consumeDeclarations(s)
	if err != nil {
		return err
	}
	for i := startRuleIdx; i < len(*rules); i++ {
		(*rules)[i].Declarations = slices.Clone(declarations)
	}

	s.TryConsumeByte('}')

	return nil
}

// consumeBlock is consume_block in simplecss src/lib.rs.
func consumeBlock(s *stream) {
	s.TryConsumeByte('{')
	consumeUntilBlockEnd(s)
}

// consumeUntilBlockEnd is consume_until_block_end in simplecss src/lib.rs.
func consumeUntilBlockEnd(s *stream) {
	// Block can have nested blocks, so we have to check for matching braces.
	// We simply counting the number of opening braces, which is incorrect,
	// since `{` can be inside a string, but it's fine for majority of the cases.

	braces := 0
loop:
	for !s.AtEnd() {
		switch s.CurrByteUnchecked() {
		case '{':
			braces++
		case '}':
			if braces == 0 {
				break loop
			}
			braces--
		}

		s.Advance(1)
	}

	s.TryConsumeByte('}')
}

// consumeDeclarations is consume_declarations in simplecss src/lib.rs.
func consumeDeclarations(s *stream) ([]Declaration, error) { //nolint:unparam // upstream returns a Result that is never an error
	var declarations []Declaration

	for !s.AtEnd() && !s.isCurrByte('}') {
		declaration, err := consumeDeclaration(s)
		if err != nil {
			consumeUntilBlockEnd(s)
			break
		}
		declarations = append(declarations, declaration)
	}

	return declarations, nil
}

// DeclarationTokenizer is DeclarationTokenizer in simplecss src/lib.rs. It
// is a declaration tokenizer.
//
// Tokenizer will stop at the first invalid token.
type DeclarationTokenizer struct {
	stream stream
}

// NewDeclarationTokenizer is From<&str> for DeclarationTokenizer in
// simplecss src/lib.rs.
func NewDeclarationTokenizer(text string) DeclarationTokenizer {
	return DeclarationTokenizer{
		stream: newStream(text),
	}
}

// Next is Iterator::next of DeclarationTokenizer in simplecss src/lib.rs. It
// returns the next declaration, or false at the end.
func (t *DeclarationTokenizer) Next() (Declaration, bool) {
	_ = t.stream.SkipSpacesAndComments()

	if t.stream.AtEnd() {
		return Declaration{}, false
	}

	v, err := consumeDeclaration(&t.stream)
	if err != nil {
		t.stream.JumpToEnd()
		return Declaration{}, false
	}
	return v, true
}

// All returns an iterator over the remaining declarations, for use with
// range. It iterates over a copy of t, so it does not change t.
//
// Port: it ports no upstream function. It is the Go form of the Rust
// Iterator impl of DeclarationTokenizer.
func (t DeclarationTokenizer) All() iter.Seq[Declaration] {
	return func(yield func(Declaration) bool) {
		for {
			d, ok := t.Next()
			if !ok || !yield(d) {
				return
			}
		}
	}
}

// consumeDeclaration is consume_declaration in simplecss src/lib.rs.
func consumeDeclaration(s *stream) (Declaration, error) {
	if err := s.SkipSpacesAndComments(); err != nil {
		return Declaration{}, err
	}

	// Parse name.

	// https://snook.ca/archives/html_and_css/targetting_ie7
	if s.isCurrByte('*') {
		s.Advance(1)
	}

	name, err := s.ConsumeIdent()
	if err != nil {
		return Declaration{}, err
	}

	if err := s.SkipSpacesAndComments(); err != nil {
		return Declaration{}, err
	}
	if err := s.ConsumeByte(':'); err != nil {
		return Declaration{}, err
	}
	if err := s.SkipSpacesAndComments(); err != nil {
		return Declaration{}, err
	}

	// Parse value.
	start := s.Pos()
	end := s.Pos()
	for consumeTerm(s) == nil {
		end = s.Pos()
		if err := s.SkipSpacesAndComments(); err != nil {
			return Declaration{}, err
		}
	}
	value := strings.TrimSpace(s.SliceRange(start, end))

	if err := s.SkipSpacesAndComments(); err != nil {
		return Declaration{}, err
	}

	// Check for `important`.
	important := false
	if s.isCurrByte('!') {
		s.Advance(1)
		if err := s.SkipSpacesAndComments(); err != nil {
			return Declaration{}, err
		}
		if strings.HasPrefix(s.SliceTail(), "important") {
			s.Advance(9)
			important = true
		}
	}

	if err := s.SkipSpacesAndComments(); err != nil {
		return Declaration{}, err
	}

	for s.isCurrByte(';') {
		s.Advance(1)
		if err := s.SkipSpacesAndComments(); err != nil {
			return Declaration{}, err
		}
	}

	if err := s.SkipSpacesAndComments(); err != nil {
		return Declaration{}, err
	}

	if value == "" {
		return Declaration{}, newErrorAt(okage.ErrCSSInvalidValue, s.GenTextPosFrom(start))
	}

	return Declaration{
		Name:      name,
		Value:     value,
		Important: important,
	}, nil
}

// consumeDigits is consume_digits inside consume_term in simplecss
// src/lib.rs.
func consumeDigits(s *stream) {
	for !s.AtEnd() {
		c := s.CurrByteUnchecked()
		if c < '0' || c > '9' {
			break
		}
		s.Advance(1)
	}
}

// consumeTerm is consume_term in simplecss src/lib.rs.
func consumeTerm(s *stream) error {
	c, err := s.CurrByte()
	if err != nil {
		return err
	}

	switch {
	case c == '#':
		s.Advance(1)
		if _, err := s.ConsumeIdent(); err != nil {
			// Try consume as a hex color.
		hex:
			for !s.AtEnd() {
				switch c := s.CurrByteUnchecked(); {
				case '0' <= c && c <= '9', 'a' <= c && c <= 'f', 'A' <= c && c <= 'F':
					s.Advance(1)
				default:
					break hex
				}
			}
		}
	case c == '+', c == '-', '0' <= c && c <= '9', c == '.':
		// Consume number.

		s.Advance(1)
		consumeDigits(s)
		if s.isCurrByte('.') {
			s.Advance(1)
			consumeDigits(s)
		}

		if s.isCurrByte('%') {
			s.Advance(1)
		} else {
			// Consume suffix if any.
			_, _ = s.ConsumeIdent()
		}
	case c == '\'', c == '"':
		if _, err := s.ConsumeString(); err != nil {
			return err
		}
	case c == ',':
		s.Advance(1)
	default:
		if _, err := s.ConsumeIdent(); err != nil {
			return err
		}

		// Consume function.
		if s.isCurrByte('(') {
			s.SkipBytes(func(c byte) bool { return c != ')' })
			if err := s.ConsumeByte(')'); err != nil {
				return err
			}
		}
	}

	return nil
}
