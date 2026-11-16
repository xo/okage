// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"fmt"
	"slices"
	"strings"

	"github.com/xo/okage"
	"github.com/xo/okage/tinycss2"
)

// supportedPseudoElements is SUPPORTED_PSEUDO_ELEMENTS in cssselect2
// cssselect2/parser.py.
var supportedPseudoElements = map[string]bool{
	// As per CSS Pseudo-Elements Module Level 4
	"first-line": true, "first-letter": true, "prefix": true, "postfix": true, "selection": true,
	"target-text": true, "spelling-error": true, "grammar-error": true, "before": true, "after": true,
	"marker": true, "placeholder": true, "file-selector-button": true,
	// As per CSS Generated Content for Paged Media Module
	"footnote-call": true, "footnote-marker": true,
	// As per CSS Notes
	"note-call": true, "note-marker": true, "note-callback": true,
	// As per CSS Scoping Module Level 1
	"content": true, "shadow": true,
}

// MaxDepth is the most selectors that the parser nests in each other, with
// :is(), :where(), :not(), :has() and the "of" of :nth-child(). Upstream has
// no such limit. A deeper selector is a SelectorError (D50). The parser and
// the compiler recurse at each level, and a goroutine of TinyGo has a small
// stack, so the limit is lower than the limit of the tokenizer.
const MaxDepth = 32

// Namespaces maps the namespace prefixes that are in scope for a selector to
// their namespace URLs. The key "" is the default namespace, which is the
// key None of upstream.
type Namespaces map[string]string

// parseState holds the arguments that the functions of parser.py pass down,
// and the depth of the nesting.
type parseState struct {
	namespaces Namespaces
	depth      int
}

// Parse is parse in cssselect2 cssselect2/parser.py. It parses the
// selectors of a comma-separated list. The input is a string or a list of
// component values of tinycss2.
//
// If forgiving is true, an invalid selector is dropped, and Parse returns no
// error. If relative is true, each selector can start with a combinator, as
// in :has().
func Parse[T tinycss2.Input](input T, namespaces Namespaces, forgiving, relative bool) ([]*Selector, error) {
	return parse(toNodes(input), parseState{namespaces: namespaces}, forgiving, relative)
}

// toNodes returns the component values of a string or of a list of them.
func toNodes[T tinycss2.Input](input T) []tinycss2.Node {
	switch v := any(input).(type) {
	case string:
		return tinycss2.ParseComponentValueList(v)
	case []tinycss2.Node:
		return v
	}
	return nil
}

// parse is parse in cssselect2 cssselect2/parser.py. Upstream is a
// generator, and every caller reads it to the end or to the first error, so
// the port returns a list.
func parse(input []tinycss2.Node, state parseState, forgiving, relative bool) ([]*Selector, error) {
	if state.depth > MaxDepth {
		return nil, newSelectorError(nil, "selectors nested too deep")
	}
	tokens := newTokenStream(input)
	if state.namespaces == nil {
		state.namespaces = Namespaces{}
	}
	var result []*Selector
	selector, err := parseSelector(tokens, state, relative)
	if err != nil {
		if forgiving {
			return result, nil
		}
		return nil, err
	}
	result = append(result, selector)
	for {
		next := tokens.next()
		if next == nil {
			return result, nil
		} else if tinycss2.IsLiteral(next, ",") {
			selector, err := parseSelector(tokens, state, relative)
			if err != nil {
				if !forgiving {
					return nil, err
				}
			} else {
				result = append(result, selector)
			}
		} else if !forgiving {
			return nil, newSelectorError(next, fmt.Sprintf("unexpected %s token.", next.Type()))
		}
	}
}

// parseSelector is parse_selector in cssselect2 cssselect2/parser.py.
func parseSelector(tokens *tokenStream, state parseState, relative bool) (*Selector, error) {
	tokens.skipWhitespaceAndComment()
	var initialCombinator string
	if relative {
		peek := tokens.peek()
		if isLiteralIn(peek, ">", "+", "~") {
			initialCombinator = literalValue(peek)
			tokens.next()
		} else {
			initialCombinator = " "
		}
		tokens.skipWhitespaceAndComment()
	}
	result, pseudoElement, err := parseCompoundSelector(tokens, state)
	if err != nil {
		return nil, err
	}
	var tree SelectorNode = result
	for {
		hasWhitespace := tokens.skipWhitespace()
		for tokens.skipComment() {
			hasWhitespace = tokens.skipWhitespace() || hasWhitespace
		}
		selector := newSelector(tree, pseudoElement)
		if relative {
			selector.Combinator = initialCombinator
		}
		if pseudoElement != "" {
			return selector, nil
		}
		peek := tokens.peek()
		var combinator string
		switch {
		case peek == nil || tinycss2.IsLiteral(peek, ","):
			return selector, nil
		case isLiteralIn(peek, ">", "+", "~"):
			combinator = literalValue(peek)
			tokens.next()
		case hasWhitespace:
			combinator = " "
		default:
			return selector, nil
		}
		var compound *CompoundSelector
		compound, pseudoElement, err = parseCompoundSelector(tokens, state)
		if err != nil {
			return nil, err
		}
		tree = &CombinedSelector{Left: tree, Combinator: combinator, Right: compound}
	}
}

// parseCompoundSelector is parse_compound_selector in cssselect2
// cssselect2/parser.py.
func parseCompoundSelector(tokens *tokenStream, state parseState) (*CompoundSelector, string, error) {
	typeSelectors, hasType, err := parseTypeSelector(tokens, state)
	if err != nil {
		return nil, "", err
	}
	simpleSelectors := typeSelectors
	var pseudoElement string
	for {
		var simpleSelector SelectorNode
		simpleSelector, pseudoElement, err = parseSimpleSelector(tokens, state)
		if err != nil {
			return nil, "", err
		}
		if pseudoElement != "" || simpleSelector == nil {
			break
		}
		simpleSelectors = append(simpleSelectors, simpleSelector)
	}

	if len(simpleSelectors) > 0 || hasType || pseudoElement != "" {
		return &CompoundSelector{SimpleSelectors: simpleSelectors}, pseudoElement, nil
	}

	peek := tokens.peek()
	peekType := "EOF"
	if peek != nil {
		peekType = peek.Type()
	}
	return nil, "", newSelectorError(peek, "expected a compound selector, got "+peekType)
}

// parseTypeSelector is parse_type_selector in cssselect2
// cssselect2/parser.py. It returns false for the None of upstream.
func parseTypeSelector(tokens *tokenStream, state parseState) ([]SelectorNode, bool, error) {
	tokens.skipWhitespace()
	qualifiedName, ok, err := parseQualifiedName(tokens, state.namespaces, false)
	if err != nil || !ok {
		return nil, false, err
	}

	simpleSelectors := []SelectorNode{}
	if qualifiedName.hasLocal {
		simpleSelectors = append(simpleSelectors, &LocalNameSelector{
			LocalName: qualifiedName.local, LowerLocalName: qualifiedName.lowerLocal})
	}
	if qualifiedName.hasNamespace {
		simpleSelectors = append(simpleSelectors, &NamespaceSelector{Namespace: qualifiedName.namespace})
	}
	return simpleSelectors, true, nil
}

// parseSimpleSelector is parse_simple_selector in cssselect2
// cssselect2/parser.py. It returns the simple selector or nil, and the
// pseudo-element or "".
func parseSimpleSelector(tokens *tokenStream, state parseState) (SelectorNode, string, error) {
	peek := tokens.peek()
	if peek == nil {
		return nil, "", nil
	}
	if hash, ok := peek.(*tinycss2.HashToken); ok && hash.IsIdentifier {
		tokens.next()
		return &IDSelector{Ident: hash.Value}, "", nil
	} else if tinycss2.IsLiteral(peek, ".") {
		tokens.next()
		next := tokens.next()
		ident, ok := next.(*tinycss2.IdentToken)
		if !ok {
			return nil, "", newSelectorError(next, "Expected a class name, got "+repr(next))
		}
		return &ClassSelector{ClassName: ident.Value}, "", nil
	} else if block, ok := peek.(*tinycss2.SquareBracketsBlock); ok {
		tokens.next()
		attr, err := parseAttributeSelector(newTokenStream(block.Content), state.namespaces)
		if err != nil {
			return nil, "", err
		}
		return attr, "", nil
	} else if tinycss2.IsLiteral(peek, ":") {
		tokens.next()
		next := tokens.next()
		if tinycss2.IsLiteral(next, ":") {
			next = tokens.next()
			ident, ok := next.(*tinycss2.IdentToken)
			if !ok {
				return nil, "", newSelectorError(next, "Expected a pseudo-element name, got "+repr(next))
			}
			value := ident.LowerValue
			if !supportedPseudoElements[value] {
				return nil, "", newSelectorError(next, "Expected a supported pseudo-element, got "+value)
			}
			return nil, value, nil
		} else if ident, ok := next.(*tinycss2.IdentToken); ok {
			name := ident.LowerValue
			switch name {
			case "before", "after", "first-line", "first-letter":
				return nil, name, nil
			}
			return &PseudoClassSelector{Name: name}, "", nil
		} else if function, ok := next.(*tinycss2.FunctionBlock); ok {
			name := function.LowerName
			switch name {
			case "is", "where", "not", "has":
				selector, err := parseLogicalCombination(function, state, name)
				return selector, "", err
			}
			return &FunctionalPseudoClassSelector{Name: name, Arguments: function.Arguments}, "", nil
		}
		return nil, "", newSelectorError(next, "unexpected "+repr(next)+" token.")
	}
	return nil, "", nil
}

// parseLogicalCombination is parse_logical_combination in cssselect2
// cssselect2/parser.py.
func parseLogicalCombination(matchesAnyToken *tinycss2.FunctionBlock, state parseState, name string) (SelectorNode, error) {
	const not = "not"
	forgiving := true
	relative := false
	switch name {
	case not:
		forgiving = false
	case "has":
		relative = true
	}

	state.depth++
	parsed, err := parse(matchesAnyToken.Arguments, state, forgiving, relative)
	if err != nil {
		return nil, err
	}
	var selectors []*Selector
	for _, selector := range parsed {
		if selector.PseudoElement == "" {
			selectors = append(selectors, selector)
		}
	}
	switch name {
	case "is":
		return &MatchesAnySelector{SelectorList: selectors}, nil
	case "where":
		return &SpecificityAdjustmentSelector{SelectorList: selectors}, nil
	case not:
		return &NegationSelector{SelectorList: selectors}, nil
	}
	return &RelationalSelector{SelectorList: selectors}, nil
}

// parseAttributeSelector is parse_attribute_selector in cssselect2
// cssselect2/parser.py.
func parseAttributeSelector(tokens *tokenStream, namespaces Namespaces) (*AttributeSelector, error) {
	tokens.skipWhitespace()
	qualifiedName, ok, err := parseQualifiedName(tokens, namespaces, true)
	if err != nil {
		return nil, err
	}
	if !ok {
		next := tokens.next()
		return nil, newSelectorError(next, "expected attribute name, got "+repr(next))
	}

	tokens.skipWhitespace()
	peek := tokens.peek()
	var operator, value string
	var hasOperator bool
	switch {
	case peek == nil:
	case isLiteralIn(peek, "=", "~=", "|=", "^=", "$=", "*="):
		operator = literalValue(peek)
		hasOperator = true
		tokens.next()
		tokens.skipWhitespace()
		next := tokens.next()
		switch t := next.(type) {
		case *tinycss2.IdentToken:
			value = t.Value
		case *tinycss2.StringToken:
			value = t.Value
		default:
			nextType := pyNone
			if next != nil {
				nextType = next.Type()
			}
			return nil, newSelectorError(next, "expected attribute value, got "+nextType)
		}
	default:
		return nil, newSelectorError(peek, "expected attribute selector operator, got "+repr(peek))
	}

	tokens.skipWhitespace()
	next := tokens.next()
	caseSensitivity := CaseDocument
	if next != nil {
		ident, ok := next.(*tinycss2.IdentToken)
		switch {
		case ok && (ident.Value == "i" || ident.Value == "I"):
			caseSensitivity = CaseInsensitive
		case ok && (ident.Value == "s" || ident.Value == "S"):
			caseSensitivity = CaseSensitive
		default:
			return nil, newSelectorError(next, "expected ], got "+next.Type())
		}
	}
	return &AttributeSelector{
		Namespace:     qualifiedName.namespace,
		HasNamespace:  qualifiedName.hasNamespace,
		Name:          qualifiedName.local,
		LowerName:     qualifiedName.lowerLocal,
		Operator:      operator,
		HasOperator:   hasOperator,
		Value:         value,
		CaseSensitive: caseSensitivity,
	}, nil
}

// qualifiedName is the result of parse_qualified_name: the namespace, which
// can be None, and the local name, which can be None for a wildcard.
type qualifiedName struct {
	namespace    string
	hasNamespace bool
	local        string
	lowerLocal   string
	hasLocal     bool
}

// parseQualifiedName is parse_qualified_name in cssselect2
// cssselect2/parser.py. It returns the namespace and the local name of the
// given tokens, and false for the None of upstream. The empty string for the
// namespace means "no namespace".
func parseQualifiedName(tokens *tokenStream, namespaces Namespaces, isAttribute bool) (qualifiedName, bool, error) {
	var q qualifiedName
	peek := tokens.peek()
	if peek == nil {
		return q, false, nil
	}
	if firstIdent, ok := peek.(*tinycss2.IdentToken); ok {
		tokens.next()
		peek = tokens.peek()
		if !tinycss2.IsLiteral(peek, "|") {
			if isAttribute {
				q.namespace, q.hasNamespace = "", true
			} else {
				q.namespace, q.hasNamespace = namespaces[""]
			}
			q.local, q.lowerLocal, q.hasLocal = firstIdent.Value, firstIdent.LowerValue, true
			return q, true, nil
		}
		tokens.next()
		namespace, ok := namespaces[firstIdent.Value]
		if !ok {
			return q, false, newSelectorError(firstIdent, "undefined namespace prefix: "+firstIdent.Value)
		}
		q.namespace, q.hasNamespace = namespace, true
	} else if tinycss2.IsLiteral(peek, "*") {
		next := tokens.next()
		peek = tokens.peek()
		if !tinycss2.IsLiteral(peek, "|") {
			if isAttribute {
				return q, false, newSelectorError(next, "expected local name, got "+next.Type())
			}
			q.namespace, q.hasNamespace = namespaces[""]
			return q, true, nil
		}
		tokens.next()
	} else if tinycss2.IsLiteral(peek, "|") {
		tokens.next()
		q.namespace, q.hasNamespace = "", true
	} else {
		return q, false, nil
	}

	// If we get here, we just consumed '|' and set the namespace.
	next := tokens.next()
	if ident, ok := next.(*tinycss2.IdentToken); ok {
		q.local, q.lowerLocal, q.hasLocal = ident.Value, ident.LowerValue, true
		return q, true, nil
	} else if tinycss2.IsLiteral(next, "*") && !isAttribute {
		return q, true, nil
	}
	// Upstream raises AttributeError here when next is None.
	nextType := "EOF"
	if next != nil {
		nextType = next.Type()
	}
	return q, false, newSelectorError(next, "expected local name, got "+nextType)
}

// SelectorError is SelectorError in cssselect2 cssselect2/parser.py, the
// error of an invalid or unsupported selector. It wraps
// okage.ErrCSSSelector, so that errors.Is finds it (D21).
//
// The port also returns it where upstream raises another exception for a
// selector, such as AttributeError, NotImplementedError or SyntaxError, so
// that no stylesheet makes it panic (D50).
type SelectorError struct {
	// Token is the token of the error, or nil.
	Token tinycss2.Node
	// Message is the message of upstream.
	Message string
}

// newSelectorError makes a SelectorError.
func newSelectorError(token tinycss2.Node, message string) *SelectorError {
	return &SelectorError{Token: token, Message: message}
}

// Error satisfies the error interface.
func (e *SelectorError) Error() string {
	return string(okage.ErrCSSSelector) + ": " + e.Message
}

// Unwrap returns okage.ErrCSSSelector.
func (e *SelectorError) Unwrap() error {
	return okage.ErrCSSSelector
}

// tokenStream is TokenStream in cssselect2 cssselect2/parser.py.
type tokenStream struct {
	tokens []tinycss2.Node
	pos    int
	peeked []tinycss2.Node // In reversed order
}

// newTokenStream makes a tokenStream.
func newTokenStream(tokens []tinycss2.Node) *tokenStream {
	return &tokenStream{tokens: tokens}
}

// nextToken is next(self.tokens, None).
func (s *tokenStream) nextToken() tinycss2.Node {
	if s.pos < len(s.tokens) {
		t := s.tokens[s.pos]
		s.pos++
		return t
	}
	return nil
}

// next is TokenStream.next.
func (s *tokenStream) next() tinycss2.Node {
	if len(s.peeked) > 0 {
		t := s.peeked[len(s.peeked)-1]
		s.peeked = s.peeked[:len(s.peeked)-1]
		return t
	}
	return s.nextToken()
}

// peek is TokenStream.peek.
func (s *tokenStream) peek() tinycss2.Node {
	if len(s.peeked) == 0 {
		s.peeked = append(s.peeked, s.nextToken())
	}
	return s.peeked[len(s.peeked)-1]
}

// skip is TokenStream.skip.
func (s *tokenStream) skip(skipTypes ...string) bool {
	found := false
	for {
		peek := s.peek()
		if peek == nil || !contains(skipTypes, peek.Type()) {
			break
		}
		s.next()
		found = true
	}
	return found
}

// skipWhitespace is TokenStream.skip_whitespace.
func (s *tokenStream) skipWhitespace() bool {
	return s.skip(tinycss2.TypeWhitespace)
}

// skipComment is TokenStream.skip_comment.
func (s *tokenStream) skipComment() bool {
	return s.skip(tinycss2.TypeComment)
}

// skipWhitespaceAndComment is TokenStream.skip_whitespace_and_comment.
// Upstream returns whether it skipped a token, and no caller reads it.
func (s *tokenStream) skipWhitespaceAndComment() {
	s.skip(tinycss2.TypeComment, tinycss2.TypeWhitespace)
}

// contains reports whether list holds s.
func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}

// isLiteralIn is `token in (...)` of upstream for literal tokens.
func isLiteralIn(token tinycss2.Node, values ...string) bool {
	for _, v := range values {
		if tinycss2.IsLiteral(token, v) {
			return true
		}
	}
	return false
}

// pyNone is repr(None) of Python.
const pyNone = "None"

// literalValue returns the value of a literal token, or "".
func literalValue(token tinycss2.Node) string {
	if t, ok := token.(*tinycss2.LiteralToken); ok {
		return t.Value
	}
	return ""
}

// repr is repr of Python for a token, or "None".
func repr(token tinycss2.Node) string {
	if token == nil {
		return pyNone
	}
	return token.String()
}

// Specificity is the specificity of a selector: three integers, as
// http://www.w3.org/TR/selectors/#specificity says.
type Specificity [3]int

// Compare compares two specificities in the order of a Python tuple. It
// returns -1, 0 or 1.
func (s Specificity) Compare(other Specificity) int {
	for i := range s {
		if s[i] != other[i] {
			if s[i] < other[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// add returns the sum of two specificities.
func (s Specificity) add(other Specificity) Specificity {
	return Specificity{s[0] + other[0], s[1] + other[1], s[2] + other[2]}
}

// maxSpecificity is max of Python on the specificities of a list of
// selectors, or (0, 0, 0) for an empty list.
func maxSpecificity(list []*Selector) Specificity {
	var m Specificity
	for i, selector := range list {
		if i == 0 || selector.Specificity.Compare(m) > 0 {
			m = selector.Specificity
		}
	}
	return m
}

// SelectorNode is a node of the tree of a parsed selector: one of the
// selector classes of cssselect2 cssselect2/parser.py.
type SelectorNode interface {
	// Specificity is the attribute specificity of upstream.
	Specificity() Specificity
	// String is the __repr__ of upstream.
	String() string
}

// Selector is Selector in cssselect2 cssselect2/parser.py, a parsed
// selector, and also RelativeSelector of the same file, which has a
// combinator.
type Selector struct {
	// ParsedTree is the tree of the selector.
	ParsedTree SelectorNode
	// PseudoElement is the pseudo-element of the selector, or "" for None.
	PseudoElement string
	// Specificity is the specificity of the selector.
	Specificity Specificity
	// Combinator is the combinator of a RelativeSelector: " ", ">", "+" or
	// "~". It is "" for a selector that is not relative.
	Combinator string
}

// newSelector is the constructor of Selector.
func newSelector(tree SelectorNode, pseudoElement string) *Selector {
	s := &Selector{ParsedTree: tree, PseudoElement: pseudoElement, Specificity: tree.Specificity()}
	if pseudoElement != "" {
		s.Specificity[2]++
	}
	return s
}

// String is Selector.__repr__, or RelativeSelector.__repr__.
func (s *Selector) String() string {
	var pseudo string
	if s.PseudoElement != "" {
		pseudo = "::" + s.PseudoElement
	}
	r := s.ParsedTree.String() + pseudo
	if s.Combinator != "" && s.Combinator != " " {
		return s.Combinator + " " + r
	}
	return r
}

// CombinedSelector is CombinedSelector in cssselect2 cssselect2/parser.py.
type CombinedSelector struct {
	// Left is a combined or a compound selector.
	Left SelectorNode
	// Combinator is " " (a single space), ">", "+" or "~".
	Combinator string
	// Right is a compound selector.
	Right *CompoundSelector
}

// Specificity satisfies SelectorNode.
func (s *CombinedSelector) Specificity() Specificity {
	return s.Left.Specificity().add(s.Right.Specificity())
}

// String satisfies SelectorNode.
func (s *CombinedSelector) String() string {
	return s.Left.String() + s.Combinator + s.Right.String()
}

// CompoundSelector is CompoundSelector in cssselect2 cssselect2/parser.py.
type CompoundSelector struct {
	SimpleSelectors []SelectorNode
}

// Specificity satisfies SelectorNode.
func (s *CompoundSelector) Specificity() Specificity {
	var sum Specificity
	for _, simple := range s.SimpleSelectors {
		sum = sum.add(simple.Specificity())
	}
	return sum
}

// String satisfies SelectorNode.
func (s *CompoundSelector) String() string {
	var b strings.Builder
	for _, simple := range s.SimpleSelectors {
		b.WriteString(simple.String())
	}
	return b.String()
}

// LocalNameSelector is LocalNameSelector in cssselect2
// cssselect2/parser.py.
type LocalNameSelector struct {
	LocalName      string
	LowerLocalName string
}

// Specificity satisfies SelectorNode.
func (*LocalNameSelector) Specificity() Specificity { return Specificity{0, 0, 1} }

// String satisfies SelectorNode.
func (s *LocalNameSelector) String() string { return s.LocalName }

// NamespaceSelector is NamespaceSelector in cssselect2
// cssselect2/parser.py.
type NamespaceSelector struct {
	// Namespace is the namespace URL, or the empty string for elements in
	// no namespace.
	Namespace string
}

// Specificity satisfies SelectorNode.
func (*NamespaceSelector) Specificity() Specificity { return Specificity{} }

// String satisfies SelectorNode.
func (s *NamespaceSelector) String() string {
	if s.Namespace == "" {
		return "|"
	}
	return "{" + s.Namespace + "}|"
}

// IDSelector is IDSelector in cssselect2 cssselect2/parser.py.
type IDSelector struct {
	Ident string
}

// Specificity satisfies SelectorNode.
func (*IDSelector) Specificity() Specificity { return Specificity{1, 0, 0} }

// String satisfies SelectorNode.
func (s *IDSelector) String() string { return "#" + s.Ident }

// ClassSelector is ClassSelector in cssselect2 cssselect2/parser.py.
type ClassSelector struct {
	ClassName string
}

// Specificity satisfies SelectorNode.
func (*ClassSelector) Specificity() Specificity { return Specificity{0, 1, 0} }

// String satisfies SelectorNode.
func (s *ClassSelector) String() string { return "." + s.ClassName }

// CaseSensitivity is the attribute case_sensitive of AttributeSelector,
// which is True, False or None in upstream.
type CaseSensitivity int

// The values of CaseSensitivity.
const (
	// CaseDocument depends on the document language. It is None.
	CaseDocument CaseSensitivity = iota
	// CaseSensitive is True, from the flag s.
	CaseSensitive
	// CaseInsensitive is False, from the flag i.
	CaseInsensitive
)

// AttributeSelector is AttributeSelector in cssselect2
// cssselect2/parser.py.
type AttributeSelector struct {
	// Namespace is the namespace of the attribute when HasNamespace is
	// true. HasNamespace is false for any namespace, where upstream holds
	// None.
	Namespace    string
	HasNamespace bool
	// Name and LowerName are the name of the attribute.
	Name      string
	LowerName string
	// Operator is a string such as "=" or "~=" when HasOperator is true.
	// HasOperator is false for an [attr] selector, where upstream holds
	// None.
	Operator    string
	HasOperator bool
	// Value is the value to compare, when HasOperator is true.
	Value string
	// CaseSensitive is the case sensitivity of the comparison.
	CaseSensitive CaseSensitivity
}

// Specificity satisfies SelectorNode.
func (*AttributeSelector) Specificity() Specificity { return Specificity{0, 1, 0} }

// String satisfies SelectorNode.
func (s *AttributeSelector) String() string {
	namespace := "*|"
	if s.HasNamespace {
		namespace = "{" + s.Namespace + "}"
	}
	var caseSensitive string
	switch s.CaseSensitive {
	case CaseSensitive:
		caseSensitive = " s"
	case CaseInsensitive:
		caseSensitive = " i"
	}
	value := pyNone
	operator := pyNone
	if s.HasOperator {
		value = fmt.Sprintf("%q", s.Value)
		operator = s.Operator
	}
	return "[" + namespace + s.Name + operator + value + caseSensitive + "]"
}

// PseudoClassSelector is PseudoClassSelector in cssselect2
// cssselect2/parser.py.
type PseudoClassSelector struct {
	Name string
}

// Specificity satisfies SelectorNode.
func (*PseudoClassSelector) Specificity() Specificity { return Specificity{0, 1, 0} }

// String satisfies SelectorNode.
func (s *PseudoClassSelector) String() string { return ":" + s.Name }

// FunctionalPseudoClassSelector is FunctionalPseudoClassSelector in
// cssselect2 cssselect2/parser.py.
type FunctionalPseudoClassSelector struct {
	Name      string
	Arguments []tinycss2.Node
}

// Specificity satisfies SelectorNode.
func (*FunctionalPseudoClassSelector) Specificity() Specificity { return Specificity{0, 1, 0} }

// String satisfies SelectorNode.
func (s *FunctionalPseudoClassSelector) String() string {
	return ":" + s.Name + "(" + tinycss2.Serialize(s.Arguments) + ")"
}

// selectorListString joins the selectors of a list with ", ".
func selectorListString(list []*Selector) string {
	parts := make([]string, len(list))
	for i, s := range list {
		parts[i] = s.String()
	}
	return strings.Join(parts, ", ")
}

// NegationSelector is NegationSelector in cssselect2 cssselect2/parser.py,
// for :not().
type NegationSelector struct {
	SelectorList []*Selector
}

// Specificity satisfies SelectorNode.
func (s *NegationSelector) Specificity() Specificity { return maxSpecificity(s.SelectorList) }

// String satisfies SelectorNode.
func (s *NegationSelector) String() string {
	return ":not(" + selectorListString(s.SelectorList) + ")"
}

// RelationalSelector is RelationalSelector in cssselect2
// cssselect2/parser.py, for :has(). Each selector of its list is relative.
type RelationalSelector struct {
	SelectorList []*Selector
}

// Specificity satisfies SelectorNode.
func (s *RelationalSelector) Specificity() Specificity { return maxSpecificity(s.SelectorList) }

// String satisfies SelectorNode.
func (s *RelationalSelector) String() string {
	return ":has(" + selectorListString(s.SelectorList) + ")"
}

// MatchesAnySelector is MatchesAnySelector in cssselect2
// cssselect2/parser.py, for :is().
type MatchesAnySelector struct {
	SelectorList []*Selector
}

// Specificity satisfies SelectorNode.
func (s *MatchesAnySelector) Specificity() Specificity { return maxSpecificity(s.SelectorList) }

// String satisfies SelectorNode.
func (s *MatchesAnySelector) String() string {
	return ":is(" + selectorListString(s.SelectorList) + ")"
}

// SpecificityAdjustmentSelector is SpecificityAdjustmentSelector in
// cssselect2 cssselect2/parser.py, for :where().
type SpecificityAdjustmentSelector struct {
	SelectorList []*Selector
}

// Specificity satisfies SelectorNode.
func (*SpecificityAdjustmentSelector) Specificity() Specificity { return Specificity{} }

// String satisfies SelectorNode.
func (s *SpecificityAdjustmentSelector) String() string {
	return ":where(" + selectorListString(s.SelectorList) + ")"
}
