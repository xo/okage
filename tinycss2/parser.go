// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

// tokenIter is the iterator of component values that the functions of
// parser.py share. Upstream uses a Python iterator, and itertools.chain to put
// tokens back in front of it. pending holds the tokens of the chain, and base
// is the iterator that the chain wraps.
type tokenIter struct {
	pending []Node
	base    *tokenIter
	nodes   []Node
	pos     int
}

// next is next(iterator, None). It returns nil when the iterator is
// exhausted.
func (it *tokenIter) next() Node {
	if len(it.pending) > 0 {
		n := it.pending[0]
		it.pending = it.pending[1:]
		return n
	}
	if it.base != nil {
		return it.base.next()
	}
	if it.pos < len(it.nodes) {
		n := it.nodes[it.pos]
		it.pos++
		return n
	}
	return nil
}

// chain is itertools.chain(pending, it).
func chain(pending []Node, it *tokenIter) *tokenIter {
	return &tokenIter{pending: pending, base: it}
}

// toTokenIterator is _to_token_iterator in tinycss2 tinycss2/parser.py. It
// iterates the component values of a string or of a list of component
// values. WithSkipComments drops the comments of a string only.
func toTokenIterator[T Input](input T, o options) *tokenIter {
	switch v := any(input).(type) {
	case string:
		return &tokenIter{nodes: parseComponentValueList(v, o.skipComments, o.maxDepth)}
	case []Node:
		return &tokenIter{nodes: v}
	}
	return &tokenIter{}
}

// nextSignificant is _next_significant in tinycss2 tinycss2/parser.py. It
// returns the next token that is neither whitespace nor a comment, or nil.
func nextSignificant(tokens *tokenIter) Node {
	for token := tokens.next(); token != nil; token = tokens.next() {
		if t := token.Type(); t != TypeWhitespace && t != TypeComment {
			return token
		}
	}
	return nil
}

// ParseOneComponentValue is parse_one_component_value in tinycss2
// tinycss2/parser.py. It parses a single component value, which is used for
// example for an attribute value referred to by `attr(foo length)`.
//
// It returns a component value that is neither whitespace nor a comment, or
// a *ParseError.
func ParseOneComponentValue[T Input](input T, opts ...Option) Node {
	tokens := toTokenIterator(input, newOptions(opts))
	first := nextSignificant(tokens)
	second := nextSignificant(tokens)
	if first == nil {
		return NewParseError(1, 1, "empty", "Input is empty")
	}
	if second != nil {
		line, column := second.Source()
		return NewParseError(line, column, "extra-input", "Got more than one token")
	}
	return first
}

// ParseOneDeclaration is parse_one_declaration in tinycss2
// tinycss2/parser.py. It parses a single declaration, which is used for
// example for a declaration in an @supports test.
//
// It returns a *Declaration or a *ParseError. It drops any whitespace or
// comment before the ":" colon.
func ParseOneDeclaration[T Input](input T, opts ...Option) Node {
	tokens := toTokenIterator(input, newOptions(opts))
	firstToken := nextSignificant(tokens)
	if firstToken == nil {
		return NewParseError(1, 1, "empty", "Input is empty")
	}
	return parseDeclaration(firstToken, tokens, true)
}

// consumeRemnants is _consume_remnants in tinycss2 tinycss2/parser.py.
func consumeRemnants(input *tokenIter, nested bool) {
	for token := input.next(); token != nil; token = input.next() {
		if IsLiteral(token, ";") {
			return
		} else if nested && IsLiteral(token, "}") {
			return
		}
	}
}

// parseDeclaration is _parse_declaration in tinycss2 tinycss2/parser.py. It
// consumes tokens until the end of the declaration or the first error, and
// returns a *Declaration or a *ParseError.
func parseDeclaration(firstToken Node, tokens *tokenIter, nested bool) Node {
	name, ok := firstToken.(*IdentToken)
	if !ok {
		consumeRemnants(tokens, nested)
		line, column := firstToken.Source()
		return NewParseError(line, column, "invalid",
			"Expected <ident> for declaration name, got "+firstToken.Type()+".")
	}

	colon := nextSignificant(tokens)
	if colon == nil {
		consumeRemnants(tokens, nested)
		return NewParseError(name.SourceLine, name.SourceColumn, "invalid",
			"Expected ':' after declaration name, got EOF")
	} else if !IsLiteral(colon, ":") {
		consumeRemnants(tokens, nested)
		line, column := colon.Source()
		// Upstream forgot the f of the f-string here, so the message holds
		// the text {colon.type}.
		return NewParseError(line, column, "invalid",
			"Expected ':' after declaration name, got {colon.type}.")
	}

	// The states of the loop.
	const (
		stateValue     = "value"
		stateBang      = "bang"
		stateImportant = "important"
	)
	var value []Node
	state := stateValue
	containsNonWhitespace := false
	containsSimpleBlock := false
	bangPosition := 0
	for i, token := 0, tokens.next(); token != nil; i, token = i+1, tokens.next() {
		if state == stateValue && IsLiteral(token, "!") {
			state = stateBang
			bangPosition = i
		} else if ident, ok := token.(*IdentToken); state == stateBang && ok && ident.LowerValue == stateImportant {
			state = stateImportant
		} else if t := token.Type(); t != TypeWhitespace && t != TypeComment {
			state = stateValue
			if t == TypeCurlyBracketsBlock {
				if containsNonWhitespace {
					containsSimpleBlock = true
				} else {
					containsNonWhitespace = true
				}
			} else {
				containsNonWhitespace = true
			}
		}
		value = append(value, token)
	}

	if state == stateImportant {
		value = value[:bangPosition]
	}

	// TODO: Handle custom property names

	if containsSimpleBlock && containsNonWhitespace {
		line, column := colon.Source()
		return NewParseError(line, column, "invalid", "Declaration contains {} block")
	}

	// TODO: Handle unicode-range

	return NewDeclaration(name.SourceLine, name.SourceColumn, name.Value, name.LowerValue,
		nonNil(value), state == stateImportant)
}

// consumeBlocksContent is _consume_blocks_content in tinycss2
// tinycss2/parser.py. It consumes a declaration or a nested rule.
func consumeBlocksContent(firstToken Node, tokens *tokenIter) Node {
	var declarationTokens []Node
	var semicolonToken []Node
	if !IsLiteral(firstToken, ";") && firstToken.Type() != TypeCurlyBracketsBlock {
		for token := tokens.next(); token != nil; token = tokens.next() {
			if IsLiteral(token, ";") {
				semicolonToken = append(semicolonToken, token)
				break
			}
			declarationTokens = append(declarationTokens, token)
			if token.Type() == TypeCurlyBracketsBlock {
				break
			}
		}
	}
	declaration := parseDeclaration(firstToken, &tokenIter{nodes: declarationTokens}, true)
	if declaration.Type() == TypeDeclaration {
		return declaration
	}
	pending := make([]Node, 0, len(declarationTokens)+len(semicolonToken))
	pending = append(pending, declarationTokens...)
	pending = append(pending, semicolonToken...)
	return consumeQualifiedRule(firstToken, chain(pending, tokens), true, ";")
}

// consumeDeclarationInList is _consume_declaration_in_list in tinycss2
// tinycss2/parser.py. It is like parseDeclaration, but it stops at the first
// ";". Upstream deprecates it for consumeBlocksContent.
func consumeDeclarationInList(firstToken Node, tokens *tokenIter) Node {
	var otherDeclarationTokens []Node
	for token := tokens.next(); token != nil; token = tokens.next() {
		if IsLiteral(token, ";") {
			break
		}
		otherDeclarationTokens = append(otherDeclarationTokens, token)
	}
	return parseDeclaration(firstToken, &tokenIter{nodes: otherDeclarationTokens}, true)
}

// ParseBlocksContents is parse_blocks_contents in tinycss2
// tinycss2/parser.py. It parses the contents of a block, which is used for
// example for the Content of a style rule or of an @page rule, or for the
// style attribute of an HTML element.
//
// In a context that expects no at-rule or no qualified rule, reject each
// *AtRule or *QualifiedRule as invalid.
//
// WithSkipComments drops the comments at the top level of the list, and, if
// the input is a string, every comment. WithSkipWhitespace drops the
// whitespace at the top level of the list.
//
// It returns a list of *Declaration, *AtRule, *QualifiedRule, *Comment
// (without WithSkipComments), *WhitespaceToken (without WithSkipWhitespace)
// and *ParseError values.
func ParseBlocksContents[T Input](input T, opts ...Option) []Node {
	o := newOptions(opts)
	tokens := toTokenIterator(input, o)
	result := []Node{}
	for token := tokens.next(); token != nil; token = tokens.next() {
		switch t := token.(type) {
		case *WhitespaceToken:
			if !o.skipWhitespace {
				result = append(result, token)
			}
		case *Comment:
			if !o.skipComments {
				result = append(result, token)
			}
		case *AtKeywordToken:
			result = append(result, consumeAtRule(t, tokens))
		default:
			if !IsLiteral(token, ";") {
				result = append(result, consumeBlocksContent(token, tokens))
			}
		}
	}
	return result
}

// ParseDeclarationList is parse_declaration_list in tinycss2
// tinycss2/parser.py. It parses a declaration list, which can also hold
// at-rules. Upstream deprecates it for ParseBlocksContents, as CSS Syntax
// Level 3 removed it.
//
// The options work as in ParseBlocksContents. It returns a list of
// *Declaration, *AtRule, *Comment, *WhitespaceToken and *ParseError values.
func ParseDeclarationList[T Input](input T, opts ...Option) []Node {
	o := newOptions(opts)
	tokens := toTokenIterator(input, o)
	result := []Node{}
	for token := tokens.next(); token != nil; token = tokens.next() {
		switch t := token.(type) {
		case *WhitespaceToken:
			if !o.skipWhitespace {
				result = append(result, token)
			}
		case *Comment:
			if !o.skipComments {
				result = append(result, token)
			}
		case *AtKeywordToken:
			result = append(result, consumeAtRule(t, tokens))
		default:
			if !IsLiteral(token, ";") {
				result = append(result, consumeDeclarationInList(token, tokens))
			}
		}
	}
	return result
}

// ParseOneRule is parse_one_rule in tinycss2 tinycss2/parser.py. It parses a
// single qualified rule or at-rule, which is used for example by insertRule()
// of CSSOM.
//
// It returns a *QualifiedRule, an *AtRule or a *ParseError. It drops any
// whitespace or comment before or after the rule.
func ParseOneRule[T Input](input T, opts ...Option) Node {
	tokens := toTokenIterator(input, newOptions(opts))
	first := nextSignificant(tokens)
	if first == nil {
		return NewParseError(1, 1, "empty", "Input is empty")
	}

	rule := consumeRule(first, tokens)
	next := nextSignificant(tokens)
	if next != nil {
		line, column := next.Source()
		return NewParseError(line, column, "extra-input",
			"Expected a single rule, got "+next.Type()+" after the first rule.")
	}
	return rule
}

// ParseRuleList is parse_rule_list in tinycss2 tinycss2/parser.py. It parses
// a rule list that is not at the top level, such as the Content of an
// @media rule. Upstream deprecates it for ParseBlocksContents.
//
// It differs from ParseStylesheet: it keeps the "<!--" and "-->" tokens at
// the top level. The options work as in ParseBlocksContents. It returns a
// list of *QualifiedRule, *AtRule, *Comment, *WhitespaceToken and
// *ParseError values.
func ParseRuleList[T Input](input T, opts ...Option) []Node {
	o := newOptions(opts)
	tokens := toTokenIterator(input, o)
	result := []Node{}
	for token := tokens.next(); token != nil; token = tokens.next() {
		switch token.(type) {
		case *WhitespaceToken:
			if !o.skipWhitespace {
				result = append(result, token)
			}
		case *Comment:
			if !o.skipComments {
				result = append(result, token)
			}
		default:
			result = append(result, consumeRule(token, tokens))
		}
	}
	return result
}

// ParseStylesheet is parse_stylesheet in tinycss2 tinycss2/parser.py. It
// parses a stylesheet from text, such as the text of a style element of
// HTML.
//
// It differs from ParseRuleList: it drops the "<!--" and "-->" tokens at the
// top level, a legacy quirk of the style element of HTML. The options work as
// in ParseBlocksContents. It returns a list of *QualifiedRule, *AtRule,
// *Comment, *WhitespaceToken and *ParseError values.
func ParseStylesheet[T Input](input T, opts ...Option) []Node {
	o := newOptions(opts)
	tokens := toTokenIterator(input, o)
	return parseStylesheet(tokens, o)
}

// parseStylesheet is the loop of parse_stylesheet.
func parseStylesheet(tokens *tokenIter, o options) []Node {
	result := []Node{}
	for token := tokens.next(); token != nil; token = tokens.next() {
		switch token.(type) {
		case *WhitespaceToken:
			if !o.skipWhitespace {
				result = append(result, token)
			}
		case *Comment:
			if !o.skipComments {
				result = append(result, token)
			}
		default:
			if !IsLiteral(token, "<!--") && !IsLiteral(token, "-->") {
				result = append(result, consumeRule(token, tokens))
			}
		}
	}
	return result
}

// consumeRule is _consume_rule in tinycss2 tinycss2/parser.py. It parses a
// qualified rule or an at-rule, and consumes just enough tokens for this
// rule.
func consumeRule(firstToken Node, tokens *tokenIter) Node {
	if t, ok := firstToken.(*AtKeywordToken); ok {
		return consumeAtRule(t, tokens)
	}
	return consumeQualifiedRule(firstToken, tokens, false, "")
}

// consumeAtRule is _consume_at_rule in tinycss2 tinycss2/parser.py. It
// parses an at-rule, and consumes just enough tokens for this rule.
func consumeAtRule(atKeyword *AtKeywordToken, tokens *tokenIter) Node {
	prelude := []Node{}
	var content []Node
	for token := tokens.next(); token != nil; token = tokens.next() {
		if block, ok := token.(*CurlyBracketsBlock); ok {
			// TODO: handle nested at-rules
			// https://drafts.csswg.org/css-syntax-3/#consume-at-rule
			content = nonNil(block.Content)
			break
		} else if IsLiteral(token, ";") {
			break
		}
		prelude = append(prelude, token)
	}
	return NewAtRule(atKeyword.SourceLine, atKeyword.SourceColumn, atKeyword.Value,
		atKeyword.LowerValue, prelude, content)
}

// ruleError is _rule_error in tinycss2 tinycss2/parser.py. It makes the parse
// error of a rule because of the given token.
func ruleError(token Node, name string) *ParseError {
	line, column := token.Source()
	return NewParseError(line, column, "invalid",
		name+" reached before {} block for a qualified rule.")
}

// consumeQualifiedRule is _consume_qualified_rule in tinycss2
// tinycss2/parser.py. It consumes just enough tokens for a qualified rule. An
// empty stopToken is the None of upstream.
func consumeQualifiedRule(firstToken Node, tokens *tokenIter, nested bool, stopToken string) Node {
	_ = nested
	if stopToken != "" && IsLiteral(firstToken, stopToken) {
		return ruleError(firstToken, "Stop token")
	}
	var prelude []Node
	var block *CurlyBracketsBlock
	if b, ok := firstToken.(*CurlyBracketsBlock); ok {
		prelude = []Node{}
		block = b
	} else {
		prelude = []Node{firstToken}
		for token := tokens.next(); token != nil; token = tokens.next() {
			if stopToken != "" && IsLiteral(token, stopToken) {
				return ruleError(token, "Stop token")
			}
			if b, ok := token.(*CurlyBracketsBlock); ok {
				block = b
				// TODO: handle special case for CSS variables (using "nested")
				// https://drafts.csswg.org/css-syntax-3/#consume-qualified-rule
				break
			}
			prelude = append(prelude, token)
		}
		if block == nil {
			return ruleError(prelude[len(prelude)-1], "EOF")
		}
	}
	line, column := firstToken.Source()
	return NewQualifiedRule(line, column, prelude, nonNil(block.Content))
}

// nonNil returns s, or an empty list when s is nil, as a Python list is never
// None.
func nonNil(s []Node) []Node {
	if s == nil {
		return []Node{}
	}
	return s
}
