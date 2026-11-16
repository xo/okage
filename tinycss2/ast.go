// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"fmt"
	"strings"
)

// The values of Node.Type, one for each type of node. They are the values of
// the class attribute type of upstream, so that a port of code that reads
// the tree of tinycss2 compares the same strings.
const (
	TypeError               = "error"
	TypeComment             = "comment"
	TypeWhitespace          = "whitespace"
	TypeLiteral             = "literal"
	TypeIdent               = "ident"
	TypeAtKeyword           = "at-keyword"
	TypeHash                = "hash"
	TypeString              = "string"
	TypeURL                 = "url"
	TypeUnicodeRange        = "unicode-range"
	TypeNumber              = "number"
	TypePercentage          = "percentage"
	TypeDimension           = "dimension"
	TypeParenthesesBlock    = "() block"
	TypeSquareBracketsBlock = "[] block"
	TypeCurlyBracketsBlock  = "{} block"
	TypeFunction            = "function"
	TypeDeclaration         = "declaration"
	TypeQualifiedRule       = "qualified-rule"
	TypeAtRule              = "at-rule"
)

// Node is Node in tinycss2 tinycss2/ast.py. Every node type implements it.
// Each node type is a pointer to a struct of this file, and Type gives its
// type as a string, as the attribute type of upstream does:
//
//	if node.Type() == tinycss2.TypeWhitespace {
//
// A type switch on the struct type works too:
//
//	if t, ok := node.(*tinycss2.WhitespaceToken); ok {
type Node interface {
	// Type is the class attribute type of upstream. It is one of the Type
	// constants of this package.
	Type() string
	// Source returns the attributes source_line and source_column of
	// upstream: the line and the column of the start of the node in the CSS
	// source. Both start at 1.
	Source() (line, column int)
	// Serialize is Node.serialize. It serializes the node to CSS syntax.
	Serialize() string
	// String is the __repr__ of the node, such as "<IdentToken foo>".
	String() string
	// serializeTo is Node._serialize_to. It writes the node to b.
	serializeTo(b *strings.Builder)
}

// Location holds the attributes source_line and source_column of Node in
// tinycss2 tinycss2/ast.py. Each node type embeds it.
type Location struct {
	// SourceLine is the line number of the start of the node in the CSS
	// source. It starts at 1.
	SourceLine int
	// SourceColumn is the column number within SourceLine of the start of
	// the node in the CSS source. It starts at 1, and counts code points.
	SourceColumn int
}

// Source satisfies Node.
func (l Location) Source() (line, column int) {
	return l.SourceLine, l.SourceColumn
}

// serialize is Node.serialize in tinycss2 tinycss2/ast.py.
func serialize(n Node) string {
	var b strings.Builder
	n.serializeTo(&b)
	return b.String()
}

// ParseError is ParseError in tinycss2 tinycss2/ast.py. It is a syntax error
// of some sort, and it can occur anywhere in the tree.
//
// Syntax errors are not fatal in the parser, to allow for different error
// handling behaviors. For example, an error in a Selector list makes the
// whole rule invalid, but an error in a Media Query list only replaces one
// comma-separated query with "not all".
type ParseError struct {
	Location

	// Kind is a machine-readable string that tells the type of error, such
	// as "bad-url".
	Kind string
	// Message is a human-readable explanation of the error.
	Message string
}

// NewParseError makes a ParseError, as the constructor of upstream does.
func NewParseError(line, column int, kind, message string) *ParseError {
	return &ParseError{Location{line, column}, kind, message}
}

// Type satisfies Node.
func (*ParseError) Type() string { return TypeError }

// Serialize satisfies Node.
func (e *ParseError) Serialize() string { return serialize(e) }

// String satisfies Node.
func (e *ParseError) String() string { return "<ParseError " + e.Kind + ">" }

// serializeTo is ParseError._serialize_to. Upstream raises TypeError for a
// kind that it cannot serialize, such as "invalid". The port writes nothing
// for such a kind, so that no stylesheet makes it panic (D50).
func (e *ParseError) serializeTo(b *strings.Builder) {
	switch e.Kind {
	case "bad-string":
		b.WriteString("\"[bad string]\n")
	case "bad-url":
		b.WriteString("url([bad url])")
	case ")", "]", "}":
		b.WriteString(e.Kind)
	}
}

// Comment is Comment in tinycss2 tinycss2/ast.py, a CSS comment.
//
// The option WithSkipComments of the functions that parse ignores comments.
type Comment struct {
	Location

	// Value is the content of the comment, between "/*" and "*/".
	Value string
}

// Type satisfies Node.
func (*Comment) Type() string { return TypeComment }

// Serialize satisfies Node.
func (c *Comment) Serialize() string { return serialize(c) }

// String satisfies Node.
func (c *Comment) String() string { return "<Comment " + c.Value + ">" }

func (c *Comment) serializeTo(b *strings.Builder) {
	b.WriteString("/*")
	b.WriteString(c.Value)
	b.WriteString("*/")
}

// WhitespaceToken is WhitespaceToken in tinycss2 tinycss2/ast.py, a
// whitespace-token.
type WhitespaceToken struct {
	Location

	// Value is the whitespace sequence, as in the original CSS source.
	Value string
}

// NewWhitespaceToken makes a WhitespaceToken.
func NewWhitespaceToken(line, column int, value string) *WhitespaceToken {
	return &WhitespaceToken{Location{line, column}, value}
}

// Type satisfies Node.
func (*WhitespaceToken) Type() string { return TypeWhitespace }

// Serialize satisfies Node.
func (t *WhitespaceToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *WhitespaceToken) String() string { return "<WhitespaceToken>" }

func (t *WhitespaceToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Value)
}

// LiteralToken is LiteralToken in tinycss2 tinycss2/ast.py. It represents
// one or more characters as in the CSS source.
//
// Upstream compares a LiteralToken equal to its value, as in `node == ';'`.
// In Go, IsLiteral does that comparison.
//
// This regroups what the specification defines as separate token types:
// the colon ":", the semicolon ";", the comma ",", the cdc "-->", the cdo
// "<!--", the include-match "~=", the dash-match "|=", the prefix-match
// "^=", the suffix-match "$=", the substring-match "*=", the column "||",
// and the delim-token, a single ASCII character that is not part of any
// other token.
type LiteralToken struct {
	Location

	// Value is a string of one to four characters.
	Value string
}

// NewLiteralToken makes a LiteralToken.
func NewLiteralToken(line, column int, value string) *LiteralToken {
	return &LiteralToken{Location{line, column}, value}
}

// Type satisfies Node.
func (*LiteralToken) Type() string { return TypeLiteral }

// Serialize satisfies Node.
func (t *LiteralToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *LiteralToken) String() string { return "<LiteralToken " + t.Value + ">" }

func (t *LiteralToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Value)
}

// IsLiteral is LiteralToken.__eq__ in tinycss2 tinycss2/ast.py. It reports
// whether n is a LiteralToken with the value v, as `n == v` does in
// upstream. It is false for a nil node and for every other node type.
func IsLiteral(n Node, v string) bool {
	t, ok := n.(*LiteralToken)
	return ok && t != nil && t.Value == v
}

// IdentToken is IdentToken in tinycss2 tinycss2/ast.py, an ident-token.
type IdentToken struct {
	Location

	// Value is the unescaped value.
	Value string
	// LowerValue is Value in ASCII lower case. Compare it to a CSS keyword.
	LowerValue string
}

// NewIdentToken makes an IdentToken, and sets its LowerValue.
func NewIdentToken(line, column int, value string) *IdentToken {
	return &IdentToken{Location{line, column}, value, asciiLower(value)}
}

// Type satisfies Node.
func (*IdentToken) Type() string { return TypeIdent }

// Serialize satisfies Node.
func (t *IdentToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *IdentToken) String() string { return "<IdentToken " + t.Value + ">" }

func (t *IdentToken) serializeTo(b *strings.Builder) {
	b.WriteString(SerializeIdentifier(t.Value))
}

// AtKeywordToken is AtKeywordToken in tinycss2 tinycss2/ast.py, an
// at-keyword-token: "@" and its value.
type AtKeywordToken struct {
	Location

	// Value is the unescaped value, without the "@".
	Value string
	// LowerValue is Value in ASCII lower case. Compare it to a CSS
	// at-keyword.
	LowerValue string
}

// NewAtKeywordToken makes an AtKeywordToken, and sets its LowerValue.
func NewAtKeywordToken(line, column int, value string) *AtKeywordToken {
	return &AtKeywordToken{Location{line, column}, value, asciiLower(value)}
}

// Type satisfies Node.
func (*AtKeywordToken) Type() string { return TypeAtKeyword }

// Serialize satisfies Node.
func (t *AtKeywordToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *AtKeywordToken) String() string { return "<AtKeywordToken @" + t.Value + ">" }

func (t *AtKeywordToken) serializeTo(b *strings.Builder) {
	b.WriteByte('@')
	b.WriteString(SerializeIdentifier(t.Value))
}

// HashToken is HashToken in tinycss2 tinycss2/ast.py, a hash-token: "#"
// and its value.
type HashToken struct {
	Location

	// Value is the unescaped value, without the "#".
	Value string
	// IsIdentifier is true if the CSS source of the token was "#" and a
	// valid identifier. Only such hash tokens are valid ID selectors.
	IsIdentifier bool
}

// NewHashToken makes a HashToken.
func NewHashToken(line, column int, value string, isIdentifier bool) *HashToken {
	return &HashToken{Location{line, column}, value, isIdentifier}
}

// Type satisfies Node.
func (*HashToken) Type() string { return TypeHash }

// Serialize satisfies Node.
func (t *HashToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *HashToken) String() string { return "<HashToken #" + t.Value + ">" }

func (t *HashToken) serializeTo(b *strings.Builder) {
	b.WriteByte('#')
	if t.IsIdentifier {
		b.WriteString(SerializeIdentifier(t.Value))
	} else {
		b.WriteString(serializeName(t.Value))
	}
}

// StringToken is StringToken in tinycss2 tinycss2/ast.py, a string-token.
type StringToken struct {
	Location

	// Value is the unescaped value, without the quotes.
	Value string
	// Representation is the serialized form of the token.
	Representation string
}

// NewStringToken makes a StringToken.
func NewStringToken(line, column int, value, representation string) *StringToken {
	return &StringToken{Location{line, column}, value, representation}
}

// Type satisfies Node.
func (*StringToken) Type() string { return TypeString }

// Serialize satisfies Node.
func (t *StringToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *StringToken) String() string { return "<StringToken " + t.Representation + ">" }

func (t *StringToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Representation)
}

// URLToken is URLToken in tinycss2 tinycss2/ast.py, a url-token.
type URLToken struct {
	Location

	// Value is the unescaped URL, without "url(" and ")".
	Value string
	// Representation is the serialized form of the token.
	Representation string
}

// NewURLToken makes a URLToken.
func NewURLToken(line, column int, value, representation string) *URLToken {
	return &URLToken{Location{line, column}, value, representation}
}

// Type satisfies Node.
func (*URLToken) Type() string { return TypeURL }

// Serialize satisfies Node.
func (t *URLToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *URLToken) String() string { return "<URLToken " + t.Representation + ">" }

func (t *URLToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Representation)
}

// UnicodeRangeToken is UnicodeRangeToken in tinycss2 tinycss2/ast.py, a
// unicode-range-token.
type UnicodeRangeToken struct {
	Location

	// Start is the start of the range, from 0 to 0xFFFFFF.
	Start int
	// End is the end of the range. It is Start when the source gave one
	// value only.
	End int
}

// NewUnicodeRangeToken makes a UnicodeRangeToken.
func NewUnicodeRangeToken(line, column, start, end int) *UnicodeRangeToken {
	return &UnicodeRangeToken{Location{line, column}, start, end}
}

// Type satisfies Node.
func (*UnicodeRangeToken) Type() string { return TypeUnicodeRange }

// Serialize satisfies Node.
func (t *UnicodeRangeToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *UnicodeRangeToken) String() string {
	return fmt.Sprintf("<UnicodeRangeToken %d %d>", t.Start, t.End)
}

func (t *UnicodeRangeToken) serializeTo(b *strings.Builder) {
	if t.End == t.Start {
		fmt.Fprintf(b, "U+%X", t.Start)
	} else {
		fmt.Fprintf(b, "U+%X-%X", t.Start, t.End)
	}
}

// NumberToken is NumberToken in tinycss2 tinycss2/ast.py, a number-token.
type NumberToken struct {
	Location

	// Value is the numeric value.
	Value float64
	// IntValue is the numeric value as an integer when IsInteger is true,
	// and 0 when it is false. Upstream holds None for false. An integer
	// out of the range of int64 saturates, where Python keeps every digit.
	IntValue int64
	// IsInteger reports whether the token was syntactically an integer.
	IsInteger bool
	// Representation is the CSS representation of the value.
	Representation string
}

// NewNumberToken makes a NumberToken. Upstream takes an int_value of None
// for a number that is not an integer. In Go, isInteger is false for it.
func NewNumberToken(line, column int, value float64, intValue int64, isInteger bool, representation string) *NumberToken {
	return &NumberToken{Location{line, column}, value, intValue, isInteger, representation}
}

// Type satisfies Node.
func (*NumberToken) Type() string { return TypeNumber }

// Serialize satisfies Node.
func (t *NumberToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *NumberToken) String() string { return "<NumberToken " + t.Representation + ">" }

func (t *NumberToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Representation)
}

// PercentageToken is PercentageToken in tinycss2 tinycss2/ast.py, a
// percentage-token: the representation and "%".
type PercentageToken struct {
	Location

	// Value is the numeric value.
	Value float64
	// IntValue is the numeric value as an integer when IsInteger is true,
	// as in NumberToken.
	IntValue int64
	// IsInteger reports whether the value was syntactically an integer.
	IsInteger bool
	// Representation is the CSS representation of the value, without the
	// unit.
	Representation string
}

// NewPercentageToken makes a PercentageToken, as NewNumberToken does.
func NewPercentageToken(line, column int, value float64, intValue int64, isInteger bool, representation string) *PercentageToken {
	return &PercentageToken{Location{line, column}, value, intValue, isInteger, representation}
}

// Type satisfies Node.
func (*PercentageToken) Type() string { return TypePercentage }

// Serialize satisfies Node.
func (t *PercentageToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *PercentageToken) String() string {
	return "<PercentageToken " + t.Representation + "%>"
}

func (t *PercentageToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Representation)
	b.WriteByte('%')
}

// DimensionToken is DimensionToken in tinycss2 tinycss2/ast.py, a
// dimension-token: the representation and the unit.
type DimensionToken struct {
	Location

	// Value is the numeric value.
	Value float64
	// IntValue is the numeric value as an integer when IsInteger is true,
	// as in NumberToken.
	IntValue int64
	// IsInteger reports whether the value was syntactically an integer.
	IsInteger bool
	// Representation is the CSS representation of the value, without the
	// unit.
	Representation string
	// Unit is the unescaped unit.
	Unit string
	// LowerUnit is Unit in ASCII lower case. Compare it to a CSS unit.
	LowerUnit string
}

// NewDimensionToken makes a DimensionToken, as NewNumberToken does, and sets
// its LowerUnit.
func NewDimensionToken(line, column int, value float64, intValue int64, isInteger bool, representation, unit string) *DimensionToken {
	return &DimensionToken{Location{line, column}, value, intValue, isInteger, representation, unit, asciiLower(unit)}
}

// Type satisfies Node.
func (*DimensionToken) Type() string { return TypeDimension }

// Serialize satisfies Node.
func (t *DimensionToken) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *DimensionToken) String() string {
	return "<DimensionToken " + t.Representation + t.Unit + ">"
}

func (t *DimensionToken) serializeTo(b *strings.Builder) {
	b.WriteString(t.Representation)
	// Disambiguate with scientific notation
	unit := t.Unit
	if len(unit) > 0 && (unit[0] == 'e' || unit[0] == 'E') &&
		(len(unit) == 1 || strings.IndexByte("+-0123456789", unit[1]) >= 0) {
		b.WriteString(`\65 `)
		b.WriteString(serializeName(unit[1:]))
	} else {
		b.WriteString(SerializeIdentifier(unit))
	}
}

// ParenthesesBlock is ParenthesesBlock in tinycss2 tinycss2/ast.py, a
// ()-block.
type ParenthesesBlock struct {
	Location

	// Content is the content of the block, as a list of component values.
	// The "(" and ")" markers are not in the list.
	Content []Node
}

// NewParenthesesBlock makes a ParenthesesBlock.
func NewParenthesesBlock(line, column int, content []Node) *ParenthesesBlock {
	return &ParenthesesBlock{Location{line, column}, content}
}

// Type satisfies Node.
func (*ParenthesesBlock) Type() string { return TypeParenthesesBlock }

// Serialize satisfies Node.
func (t *ParenthesesBlock) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *ParenthesesBlock) String() string { return "<ParenthesesBlock ( … )>" }

func (t *ParenthesesBlock) serializeTo(b *strings.Builder) {
	b.WriteByte('(')
	serializeTo(t.Content, b)
	b.WriteByte(')')
}

// SquareBracketsBlock is SquareBracketsBlock in tinycss2 tinycss2/ast.py, a
// []-block.
type SquareBracketsBlock struct {
	Location

	// Content is the content of the block, as a list of component values.
	// The "[" and "]" markers are not in the list.
	Content []Node
}

// NewSquareBracketsBlock makes a SquareBracketsBlock.
func NewSquareBracketsBlock(line, column int, content []Node) *SquareBracketsBlock {
	return &SquareBracketsBlock{Location{line, column}, content}
}

// Type satisfies Node.
func (*SquareBracketsBlock) Type() string { return TypeSquareBracketsBlock }

// Serialize satisfies Node.
func (t *SquareBracketsBlock) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *SquareBracketsBlock) String() string { return "<SquareBracketsBlock [ … ]>" }

func (t *SquareBracketsBlock) serializeTo(b *strings.Builder) {
	b.WriteByte('[')
	serializeTo(t.Content, b)
	b.WriteByte(']')
}

// CurlyBracketsBlock is CurlyBracketsBlock in tinycss2 tinycss2/ast.py, a
// {}-block.
type CurlyBracketsBlock struct {
	Location

	// Content is the content of the block, as a list of component values.
	// The "{" and "}" markers are not in the list. The tokenizer never
	// leaves it nil, so that the Content of an AtRule tells a block from a
	// semicolon.
	Content []Node
}

// NewCurlyBracketsBlock makes a CurlyBracketsBlock.
func NewCurlyBracketsBlock(line, column int, content []Node) *CurlyBracketsBlock {
	return &CurlyBracketsBlock{Location{line, column}, content}
}

// Type satisfies Node.
func (*CurlyBracketsBlock) Type() string { return TypeCurlyBracketsBlock }

// Serialize satisfies Node.
func (t *CurlyBracketsBlock) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *CurlyBracketsBlock) String() string { return "<CurlyBracketsBlock { … }>" }

func (t *CurlyBracketsBlock) serializeTo(b *strings.Builder) {
	b.WriteByte('{')
	serializeTo(t.Content, b)
	b.WriteByte('}')
}

// FunctionBlock is FunctionBlock in tinycss2 tinycss2/ast.py, a
// function-block: the name, "(", the arguments and ")".
type FunctionBlock struct {
	Location

	// Name is the unescaped name of the function.
	Name string
	// LowerName is Name in ASCII lower case. Compare it to a CSS function
	// name.
	LowerName string
	// Arguments is the arguments of the function, as a list of component
	// values. The "(" and ")" markers are not in the list. Commas are not
	// special. They are LiteralToken values in the list.
	Arguments []Node
}

// NewFunctionBlock makes a FunctionBlock, and sets its LowerName.
func NewFunctionBlock(line, column int, name string, arguments []Node) *FunctionBlock {
	return &FunctionBlock{Location{line, column}, name, asciiLower(name), arguments}
}

// Type satisfies Node.
func (*FunctionBlock) Type() string { return TypeFunction }

// Serialize satisfies Node.
func (t *FunctionBlock) Serialize() string { return serialize(t) }

// String satisfies Node.
func (t *FunctionBlock) String() string { return "<FunctionBlock " + t.Name + "( … )>" }

func (t *FunctionBlock) serializeTo(b *strings.Builder) {
	b.WriteString(SerializeIdentifier(t.Name))
	b.WriteByte('(')
	serializeTo(t.Arguments, b)
	function := Node(t)
	for {
		f, ok := function.(*FunctionBlock)
		if !ok || len(f.Arguments) == 0 {
			break
		}
		last := f.Arguments[len(f.Arguments)-1]
		if e, ok := last.(*ParseError); ok && e.Kind == kindEOFInString {
			return
		}
		function = last
	}
	b.WriteByte(')')
}

// Declaration is Declaration in tinycss2 tinycss2/ast.py, a property or
// descriptor declaration: the name, ":", the value and an optional
// "!important".
type Declaration struct {
	Location

	// Name is the unescaped name.
	Name string
	// LowerName is Name in ASCII lower case. Compare it to a CSS property or
	// descriptor name.
	LowerName string
	// Value is the value of the declaration, as a list of component values:
	// anything between ":" and the end of the declaration, or "!important".
	Value []Node
	// Important is true if the declaration had an "!important" marker. The
	// consumer must reject it for a declaration that does not accept it,
	// such as a descriptor that is not a property.
	Important bool
}

// NewDeclaration makes a Declaration.
func NewDeclaration(line, column int, name, lowerName string, value []Node, important bool) *Declaration {
	return &Declaration{Location{line, column}, name, lowerName, value, important}
}

// Type satisfies Node.
func (*Declaration) Type() string { return TypeDeclaration }

// Serialize satisfies Node.
func (d *Declaration) Serialize() string { return serialize(d) }

// String satisfies Node.
func (d *Declaration) String() string { return "<Declaration " + d.Name + ": …>" }

func (d *Declaration) serializeTo(b *strings.Builder) {
	b.WriteString(SerializeIdentifier(d.Name))
	b.WriteByte(':')
	serializeTo(d.Value, b)
	if d.Important {
		b.WriteString("!important")
	}
}

// QualifiedRule is QualifiedRule in tinycss2 tinycss2/ast.py, a qualified
// rule: the prelude, "{", the content and "}".
//
// The meaning of a qualified rule depends on its context. At the top level
// of a stylesheet, or in a conditional rule such as "@media", it is a style
// rule. Its Prelude is a list of selectors, and its Content is a list of
// property declarations.
type QualifiedRule struct {
	Location

	// Prelude is the part of the rule before the {} block, as a list of
	// component values.
	Prelude []Node
	// Content is the part of the rule inside the {} block, as a list of
	// component values.
	Content []Node
}

// NewQualifiedRule makes a QualifiedRule.
func NewQualifiedRule(line, column int, prelude, content []Node) *QualifiedRule {
	return &QualifiedRule{Location{line, column}, prelude, content}
}

// Type satisfies Node.
func (*QualifiedRule) Type() string { return TypeQualifiedRule }

// Serialize satisfies Node.
func (r *QualifiedRule) Serialize() string { return serialize(r) }

// String satisfies Node.
func (r *QualifiedRule) String() string { return "<QualifiedRule … { … }>" }

func (r *QualifiedRule) serializeTo(b *strings.Builder) {
	serializeTo(r.Prelude, b)
	b.WriteByte('{')
	serializeTo(r.Content, b)
	b.WriteByte('}')
}

// AtRule is AtRule in tinycss2 tinycss2/ast.py, an at-rule: "@", the
// at-keyword, the prelude, and a {} block or ";".
//
// The meaning of an at-rule depends on its at-keyword and on its context.
// Most types of at-rules are only allowed in some contexts, and must end
// with a {} block or with a semicolon.
type AtRule struct {
	Location

	// AtKeyword is the unescaped value of the at-keyword of the rule,
	// without the "@".
	AtKeyword string
	// LowerAtKeyword is AtKeyword in ASCII lower case. Compare it to a CSS
	// at-keyword.
	LowerAtKeyword string
	// Prelude is the part of the rule before the {} block or the
	// semicolon, as a list of component values.
	Prelude []Node
	// Content is the content of the {} block of the rule, as a list of
	// component values. It is nil for an at-rule that ends with a
	// semicolon, where upstream holds None. It is not nil for an empty
	// block.
	Content []Node
}

// NewAtRule makes an AtRule. A nil content is the None of upstream.
func NewAtRule(line, column int, atKeyword, lowerAtKeyword string, prelude, content []Node) *AtRule {
	return &AtRule{Location{line, column}, atKeyword, lowerAtKeyword, prelude, content}
}

// Type satisfies Node.
func (*AtRule) Type() string { return TypeAtRule }

// Serialize satisfies Node.
func (r *AtRule) Serialize() string { return serialize(r) }

// String satisfies Node.
func (r *AtRule) String() string { return "<AtRule @" + r.AtKeyword + " … { … }>" }

func (r *AtRule) serializeTo(b *strings.Builder) {
	b.WriteByte('@')
	b.WriteString(SerializeIdentifier(r.AtKeyword))
	serializeTo(r.Prelude, b)
	if r.Content == nil {
		b.WriteByte(';')
	} else {
		b.WriteByte('{')
		serializeTo(r.Content, b)
		b.WriteByte('}')
	}
}
