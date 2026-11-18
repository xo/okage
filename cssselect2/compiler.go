// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package cssselect2

import (
	"math/big"
	"strings"
	"unicode"

	"github.com/xo/okage/tinycss2"
)

// splitWhitespace is split_whitespace in cssselect2 cssselect2/compiler.py,
// the findall of the regular expression `[^ \t\r\n\f]+`, from
// http://dev.w3.org/csswg/selectors/#whitespace.
func splitWhitespace(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\r' || r == '\n' || r == '\f'
	})
}

// CompileSelectorList is compile_selector_list in cssselect2
// cssselect2/compiler.py. It compiles a comma-separated list of selectors.
//
// The input is a string, or a list of component values of tinycss2, such as
// the Prelude of a style rule. namespaces holds the namespace prefixes in
// scope for the selectors. A nil namespaces declares no prefix.
func CompileSelectorList[T tinycss2.Input](input T, namespaces Namespaces) ([]*CompiledSelector, error) {
	selectors, err := parse(toNodes(input), parseState{namespaces: namespaces}, false, false)
	if err != nil {
		return nil, err
	}
	compiled := make([]*CompiledSelector, 0, len(selectors))
	for _, selector := range selectors {
		c, err := newCompiledSelector(selector)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, c)
	}
	return compiled, nil
}

// CompiledSelector is CompiledSelector in cssselect2
// cssselect2/compiler.py, the compiled form of a selector.
type CompiledSelector struct {
	// NeverMatches is true when the selector matches no element.
	NeverMatches bool
	// Specificity is the specificity of the selector.
	Specificity Specificity
	// PseudoElement is the pseudo-element of the selector, or "".
	PseudoElement string
	// ID is the ID of the last compound selector, or "".
	ID string
	// ClassName is a class of the last compound selector, or "".
	ClassName string
	// LocalName and LowerLocalName are the type of the last compound
	// selector, or "".
	LocalName      string
	LowerLocalName string
	// Namespace is the namespace of the last compound selector, when
	// HasNamespace is true.
	Namespace    string
	HasNamespace bool
	// RequiresLangAttr is true when the last compound selector tests the
	// attribute lang.
	RequiresLangAttr bool

	// test is the attribute test of upstream, a lambda that upstream
	// compiles from Python source.
	test expr

	// subjectNames holds the local names in ASCII lower case that a subject
	// of the selector can have, when hasSubjectNames is true. Upstream does
	// not have it. The Matcher reads it (D61).
	subjectNames    []string
	hasSubjectNames bool
}

// Test is the lambda test of upstream. It reports whether el is a subject of
// the selector.
func (c *CompiledSelector) Test(el *ElementWrapper) bool {
	return c.test.eval(el)
}

// newCompiledSelector is the constructor of CompiledSelector.
func newCompiledSelector(parsedSelector *Selector) (*CompiledSelector, error) {
	source, err := compileNode(parsedSelector.ParsedTree, 0)
	if err != nil {
		return nil, err
	}
	c := &CompiledSelector{
		NeverMatches:  source.isFalse(),
		test:          source,
		Specificity:   parsedSelector.Specificity,
		PseudoElement: parsedSelector.PseudoElement,
	}

	c.subjectNames, c.hasSubjectNames = subjectNames(parsedSelector.ParsedTree, 0)

	node := parsedSelector.ParsedTree
	if combined, ok := node.(*CombinedSelector); ok {
		node = combined.Right
	}
	compound, _ := node.(*CompoundSelector)
	if compound == nil {
		return c, nil
	}
	for _, simpleSelector := range compound.SimpleSelectors {
		switch s := simpleSelector.(type) {
		case *IDSelector:
			c.ID = s.Ident
		case *ClassSelector:
			c.ClassName = s.ClassName
		case *LocalNameSelector:
			c.LocalName = s.LocalName
			c.LowerLocalName = s.LowerLocalName
		case *NamespaceSelector:
			c.Namespace = s.Namespace
			c.HasNamespace = true
		case *AttributeSelector:
			if s.Name == "lang" {
				c.RequiresLangAttr = true
			}
		}
	}
	return c, nil
}

// subjectNames returns the local names in ASCII lower case that a subject of
// selector can have. It returns false when the subject can have any name.
// Upstream does not have it. The names come from the type selectors of the
// last compound selector, and from the selectors in :is() and :where() of
// that compound selector, at any depth. An element that matches selector has
// a name of the list, in ASCII lower case, in an HTML document and in an XML
// document (D61).
func subjectNames(selector SelectorNode, depth int) ([]string, bool) {
	if depth > MaxDepth {
		return nil, false
	}
	depth++
	if combined, ok := selector.(*CombinedSelector); ok {
		selector = combined.Right
	}
	compound, ok := selector.(*CompoundSelector)
	if !ok {
		return nil, false
	}
	var names []string
	found := false
	for _, simple := range compound.SimpleSelectors {
		var list []*Selector
		switch s := simple.(type) {
		case *LocalNameSelector:
			names, found = intersectNames(names, found, []string{s.LowerLocalName}), true
			continue
		case *MatchesAnySelector:
			list = s.SelectorList
		case *SpecificityAdjustmentSelector:
			list = s.SelectorList
		default:
			continue
		}
		var union []string
		anyName := false
		for _, s := range list {
			sub, ok := subjectNames(s.ParsedTree, depth)
			if !ok {
				anyName = true
				break
			}
			for _, name := range sub {
				if !contains(union, name) {
					union = append(union, name)
				}
			}
		}
		if !anyName {
			names, found = intersectNames(names, found, union), true
		}
	}
	return names, found
}

// intersectNames returns the names of b that are in a. If found is false, a
// holds no list yet, and intersectNames returns b.
func intersectNames(a []string, found bool, b []string) []string {
	if !found {
		return b
	}
	var names []string
	for _, name := range b {
		if contains(a, name) {
			names = append(names, name)
		}
	}
	return names
}

// expr is a boolean expression of an element, the Python source that
// _compile_node returns. Upstream folds the constants 1 and 0, and the port
// keeps them apart, so that it folds the same expressions.
type expr struct {
	// constant is '1' or '0' for the source "1" or "0", and 0 for another
	// expression.
	constant byte
	fn       func(el *ElementWrapper) bool
}

var (
	exprTrue  = expr{constant: '1'}
	exprFalse = expr{constant: '0'}
)

// isTrue reports whether the source is "1".
func (e expr) isTrue() bool { return e.constant == '1' }

// isFalse reports whether the source is "0".
func (e expr) isFalse() bool { return e.constant == '0' }

// eval evaluates the expression for el.
func (e expr) eval(el *ElementWrapper) bool {
	switch e.constant {
	case '1':
		return true
	case '0':
		return false
	}
	return e.fn(el)
}

// fn makes an expression that is not a constant.
func fn(f func(el *ElementWrapper) bool) expr {
	return expr{fn: f}
}

// compileNode is _compile_node in cssselect2 cssselect2/compiler.py. It
// returns a boolean expression that tells whether an element is a subject of
// selector.
func compileNode(selector SelectorNode, depth int) (expr, error) {
	if depth > MaxDepth {
		return expr{}, newSelectorError(nil, "selectors nested too deep")
	}
	depth++
	switch selector := selector.(type) {
	case *CombinedSelector:
		leftInside, err := compileNode(selector.Left, depth)
		if err != nil {
			return expr{}, err
		}
		var left expr
		switch {
		case leftInside.isFalse():
			return exprFalse, nil // 0 and x == 0
		case leftInside.isTrue():
			// 1 and x == x, but the element matching 1 still needs to exist.
			switch selector.Combinator {
			case " ", ">":
				left = fn(func(el *ElementWrapper) bool { return el.Parent != nil })
			case "~", "+":
				left = fn(func(el *ElementWrapper) bool { return el.Previous != nil })
			default:
				return expr{}, newSelectorError(nil, "Unknown combinator "+selector.Combinator)
			}
		case selector.Combinator == " ":
			left = fn(func(el *ElementWrapper) bool {
				for a := el.Parent; a != nil; a = a.Parent {
					if leftInside.eval(a) {
						return true
					}
				}
				return false
			})
		case selector.Combinator == ">":
			left = fn(func(el *ElementWrapper) bool {
				return el.Parent != nil && leftInside.eval(el.Parent)
			})
		case selector.Combinator == "+":
			left = fn(func(el *ElementWrapper) bool {
				return el.Previous != nil && leftInside.eval(el.Previous)
			})
		case selector.Combinator == "~":
			left = fn(func(el *ElementWrapper) bool {
				for s := el.Previous; s != nil; s = s.Previous {
					if leftInside.eval(s) {
						return true
					}
				}
				return false
			})
		default:
			return expr{}, newSelectorError(nil, "Unknown combinator "+selector.Combinator)
		}

		right, err := compileNode(selector.Right, depth)
		if err != nil {
			return expr{}, err
		}
		switch {
		case right.isFalse():
			return exprFalse, nil // 0 and x == 0
		case right.isTrue():
			return left, nil // 1 and x == x
		}
		// Evaluate combinators right to left
		return fn(func(el *ElementWrapper) bool {
			return right.eval(el) && left.eval(el)
		}), nil

	case *CompoundSelector:
		var subExpressions []expr
		for _, s := range selector.SimpleSelectors {
			e, err := compileNode(s, depth)
			if err != nil {
				return expr{}, err
			}
			if !e.isTrue() {
				subExpressions = append(subExpressions, e)
			}
		}
		if len(subExpressions) == 1 {
			return subExpressions[0], nil
		}
		for _, e := range subExpressions {
			if e.isFalse() {
				return exprFalse, nil
			}
		}
		if len(subExpressions) > 0 {
			return fn(func(el *ElementWrapper) bool {
				for _, e := range subExpressions {
					if !e.eval(el) {
						return false
					}
				}
				return true
			}), nil
		}
		return exprTrue, nil // all([]) == True

	case *NegationSelector:
		subExpressions, err := compileList(selector.SelectorList, depth, exprTrue)
		if err != nil {
			return expr{}, err
		}
		if len(subExpressions) == 0 {
			return exprFalse, nil
		}
		return fn(func(el *ElementWrapper) bool {
			return !anyExpr(subExpressions, el)
		}), nil

	case *RelationalSelector:
		if len(selector.SelectorList) == 0 {
			// Upstream compiles the empty source "lambda el: ", and raises
			// SyntaxError.
			return expr{}, newSelectorError(nil, "empty :has()")
		}
		var subExpressions []func(el *ElementWrapper) bool
		for _, relativeSelector := range selector.SelectorList {
			expression, err := compileNode(relativeSelector.ParsedTree, depth)
			if err != nil {
				return expr{}, err
			}
			// Port: upstream walks the elements of the combinator each
			// time. The port keeps the result for each element (D58).
			r := &relation{combinator: relativeSelector.Combinator, expression: expression}
			sub := r.matches
			subExpressions = append(subExpressions, sub)
		}
		return fn(func(el *ElementWrapper) bool {
			for _, sub := range subExpressions {
				if sub(el) {
					return true
				}
			}
			return false
		}), nil

	case *MatchesAnySelector:
		return compileMatchesAny(selector.SelectorList, depth)
	case *SpecificityAdjustmentSelector:
		return compileMatchesAny(selector.SelectorList, depth)

	case *LocalNameSelector:
		localName, lowerLocalName := selector.LocalName, selector.LowerLocalName
		if lowerLocalName == localName {
			return fn(func(el *ElementWrapper) bool { return el.LocalName() == localName }), nil
		}
		return fn(func(el *ElementWrapper) bool {
			if el.InHTMLDocument {
				return el.LocalName() == lowerLocalName
			}
			return el.LocalName() == localName
		}), nil

	case *NamespaceSelector:
		namespace := selector.Namespace
		return fn(func(el *ElementWrapper) bool { return el.NamespaceURL() == namespace }), nil

	case *ClassSelector:
		className := selector.ClassName
		return fn(func(el *ElementWrapper) bool { return contains(el.Classes(), className) }), nil

	case *IDSelector:
		ident := selector.Ident
		return fn(func(el *ElementWrapper) bool {
			id, ok := el.ID()
			return ok && id == ident
		}), nil

	case *AttributeSelector:
		return compileAttributeSelector(selector)

	case *PseudoClassSelector:
		return compilePseudoClass(selector)

	case *FunctionalPseudoClassSelector:
		return compileFunctionalPseudoClass(selector, depth)
	}
	return expr{}, newSelectorError(nil, "unknown selector type")
}

// compileList compiles the parsed trees of a list of selectors, and drops
// each expression that is the constant drop.
func compileList(list []*Selector, depth int, drop expr) ([]expr, error) {
	var subExpressions []expr
	for _, s := range list {
		e, err := compileNode(s.ParsedTree, depth)
		if err != nil {
			return nil, err
		}
		if e.constant != drop.constant {
			subExpressions = append(subExpressions, e)
		}
	}
	return subExpressions, nil
}

// anyExpr reports whether an expression of the list is true for el.
func anyExpr(list []expr, el *ElementWrapper) bool {
	for _, e := range list {
		if e.eval(el) {
			return true
		}
	}
	return false
}

// compileMatchesAny is the branch of _compile_node for MatchesAnySelector
// and SpecificityAdjustmentSelector.
func compileMatchesAny(list []*Selector, depth int) (expr, error) {
	subExpressions, err := compileList(list, depth, exprFalse)
	if err != nil {
		return expr{}, err
	}
	if len(subExpressions) == 0 {
		return exprFalse, nil
	}
	return fn(func(el *ElementWrapper) bool {
		return anyExpr(subExpressions, el)
	}), nil
}

// pyIsSpace is str.isspace of Python for one character.
func pyIsSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

// compileAttributeSelector is the branch of _compile_node for
// AttributeSelector.
func compileAttributeSelector(selector *AttributeSelector) (expr, error) {
	if !selector.HasNamespace {
		// In any namespace. Upstream raises NotImplementedError.
		return expr{}, newSelectorError(nil, "attribute selector in any namespace")
	}
	namespace, name, lowerName := selector.Namespace, selector.Name, selector.LowerName
	key := func(el *ElementWrapper) string {
		if el.InHTMLDocument {
			return lowerName
		}
		return name
	}
	value := selector.Value
	caseInsensitive := selector.CaseSensitive == CaseInsensitive
	if caseInsensitive {
		if !selector.HasOperator {
			// Upstream calls lower() on None, and raises AttributeError.
			return expr{}, newSelectorError(nil, "case flag on an attribute selector with no value")
		}
		value = strings.ToLower(value)
	}
	attributeValue := func(el *ElementWrapper) string {
		v, _ := el.Element.Attr(namespace, key(el))
		if caseInsensitive {
			v = strings.ToLower(v)
		}
		return v
	}
	has := func(el *ElementWrapper) bool {
		_, ok := el.Element.Attr(namespace, key(el))
		return ok
	}
	if !selector.HasOperator {
		return fn(has), nil
	}
	switch selector.Operator {
	case "=":
		return fn(func(el *ElementWrapper) bool {
			return has(el) && attributeValue(el) == value
		}), nil
	case "~=":
		if len(strings.FieldsFunc(value, pyIsSpace)) != 1 || strings.TrimFunc(value, pyIsSpace) != value {
			return exprFalse, nil
		}
		return fn(func(el *ElementWrapper) bool {
			return contains(splitWhitespace(attributeValue(el)), value)
		}), nil
	case "|=":
		return fn(func(el *ElementWrapper) bool {
			return has(el) && attributeValue(el) == value ||
				strings.HasPrefix(attributeValue(el), value+"-")
		}), nil
	case "^=":
		if value == "" {
			return exprFalse, nil
		}
		return fn(func(el *ElementWrapper) bool {
			return strings.HasPrefix(attributeValue(el), value)
		}), nil
	case "$=":
		if value == "" {
			return exprFalse, nil
		}
		return fn(func(el *ElementWrapper) bool {
			return strings.HasSuffix(attributeValue(el), value)
		}), nil
	case "*=":
		if value == "" {
			return exprFalse, nil
		}
		return fn(func(el *ElementWrapper) bool {
			return strings.Contains(attributeValue(el), value)
		}), nil
	}
	return expr{}, newSelectorError(nil, "Unknown attribute operator "+selector.Operator)
}

// The namespace URLs that the pseudo-classes use.
const (
	xhtmlNamespace = "http://www.w3.org/1999/xhtml"
	xmlNamespace   = "http://www.w3.org/XML/1998/namespace"
)

// htmlTagEq is html_tag_eq in cssselect2 cssselect2/compiler.py. It tests
// equality with HTML local names.
func htmlTagEq(el *ElementWrapper, localNames ...string) bool {
	if el.InHTMLDocument {
		return contains(localNames, el.LocalName())
	}
	return el.NamespaceURL() == xhtmlNamespace && contains(localNames, el.LocalName())
}

// attr is etree_element.get(name) of upstream, for an attribute in no
// namespace. It returns false for None.
func attr(el *ElementWrapper, name string) (string, bool) {
	return el.Element.Attr("", name)
}

// sameTag reports whether two elements have the same tag of ElementTree: the
// same namespace URL and the same local name.
func sameTag(a, b Element) bool {
	return a.LocalName() == b.LocalName() && a.NamespaceURL() == b.NamespaceURL()
}

// compilePseudoClass is the branch of _compile_node for
// PseudoClassSelector.
func compilePseudoClass(selector *PseudoClassSelector) (expr, error) {
	switch selector.Name {
	case "link", "any-link", "local-link":
		local := selector.Name == "local-link"
		return fn(func(el *ElementWrapper) bool {
			if !htmlTagEq(el, "a", "area", "link") {
				return false
			}
			href, ok := attr(el, "href")
			if !ok {
				return false
			}
			if local {
				scheme, valid := urlScheme(href)
				return valid && scheme == ""
			}
			return true
		}), nil
	case "enabled":
		return fn(func(el *ElementWrapper) bool {
			_, disabled := attr(el, "disabled")
			_, href := attr(el, "href")
			return (htmlTagEq(el, "button", "input", "select", "textarea", "option") &&
				!disabled && !el.InDisabledFieldset()) ||
				(htmlTagEq(el, "optgroup", "menuitem", "fieldset") && !disabled) ||
				(htmlTagEq(el, "a", "area", "link") && href)
		}), nil
	case "disabled":
		return fn(func(el *ElementWrapper) bool {
			_, disabled := attr(el, "disabled")
			return (htmlTagEq(el, "button", "input", "select", "textarea", "option") &&
				(disabled || el.InDisabledFieldset())) ||
				(htmlTagEq(el, "optgroup", "menuitem", "fieldset") && disabled)
		}), nil
	case "checked":
		return fn(func(el *ElementWrapper) bool {
			_, checked := attr(el, "checked")
			typ, _ := attr(el, "type")
			typ = tinycss2.ASCIILower(typ)
			_, selected := attr(el, "selected")
			return (htmlTagEq(el, "input", "menuitem") && checked &&
				(typ == "checkbox" || typ == "radio")) ||
				(htmlTagEq(el, "option") && selected)
		}), nil
	case "visited", "hover", "active", "focus", "focus-within",
		"focus-visible", "target", "target-within", "current", "past",
		"future", "playing", "paused", "seeking", "buffering",
		"stalled", "muted", "volume-locked", "user-valid",
		"user-invalid", "host":
		// Not applicable in a static context: never match.
		return exprFalse, nil
	case "root", "scope":
		return fn(func(el *ElementWrapper) bool { return el.Parent == nil }), nil
	case "first-child":
		return fn(func(el *ElementWrapper) bool { return el.Index == 0 }), nil
	case "last-child":
		return fn(func(el *ElementWrapper) bool { return el.Index+1 == len(el.EtreeSiblings()) }), nil
	case "first-of-type":
		return fn(func(el *ElementWrapper) bool {
			for _, s := range el.EtreeSiblings()[:el.Index] {
				if sameTag(s, el.Element) {
					return false
				}
			}
			return true
		}), nil
	case "last-of-type":
		return fn(func(el *ElementWrapper) bool {
			for _, s := range el.EtreeSiblings()[el.Index+1:] {
				if sameTag(s, el.Element) {
					return false
				}
			}
			return true
		}), nil
	case "only-child":
		return fn(func(el *ElementWrapper) bool { return len(el.EtreeSiblings()) == 1 }), nil
	case "only-of-type":
		return fn(func(el *ElementWrapper) bool {
			for i, s := range el.EtreeSiblings() {
				if sameTag(s, el.Element) && i != el.Index {
					return false
				}
			}
			return true
		}), nil
	case "empty":
		return fn(func(el *ElementWrapper) bool {
			return !(len(el.EtreeChildren()) > 0 || el.Element.Text() != "")
		}), nil
	}
	return expr{}, newSelectorError(nil, "Unknown pseudo-class "+selector.Name)
}

// urlScheme is urlparse(href).scheme of Python. It returns false where
// urlparse raises ValueError, for an invalid IPv6 address. The port does not
// match such a link, where upstream raises the error (D50).
func urlScheme(url string) (string, bool) {
	url = strings.TrimLeft(url, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f"+
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f ")
	url = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(url)
	var scheme string
	if i := strings.IndexByte(url, ':'); i > 0 && isASCIIAlpha(url[0]) {
		valid := true
		for j := range i {
			c := url[j]
			if !isASCIIAlpha(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
				valid = false
				break
			}
		}
		if valid {
			scheme, url = strings.ToLower(url[:i]), url[i+1:]
		}
	}
	if strings.HasPrefix(url, "//") {
		netloc := url[2:]
		if i := strings.IndexAny(netloc, "/?#"); i >= 0 {
			netloc = netloc[:i]
		}
		open, closing := strings.Contains(netloc, "["), strings.Contains(netloc, "]")
		if open != closing {
			return "", false
		}
	}
	return scheme, true
}

// isASCIIAlpha reports whether c is an ASCII letter.
func isASCIIAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// compileFunctionalPseudoClass is the branch of _compile_node for
// FunctionalPseudoClassSelector.
func compileFunctionalPseudoClass(selector *FunctionalPseudoClassSelector, depth int) (expr, error) {
	if selector.Name == "lang" {
		var langs []string
		var tokens []tinycss2.Node
		for _, token := range selector.Arguments {
			if t := token.Type(); t != tinycss2.TypeWhitespace && t != tinycss2.TypeComment {
				tokens = append(tokens, token)
			}
		}
		for len(tokens) > 0 {
			token := tokens[0]
			tokens = tokens[1:]
			switch t := token.(type) {
			case *tinycss2.IdentToken:
				langs = append(langs, t.LowerValue)
			case *tinycss2.StringToken:
				langs = append(langs, tinycss2.ASCIILower(t.Value))
			default:
				return expr{}, newSelectorError(nil, "Invalid arguments for :lang()")
			}
			if len(tokens) > 0 {
				token = tokens[0]
				tokens = tokens[1:]
				if token.Type() != tinycss2.TypeIdent && !tinycss2.IsLiteral(token, ",") {
					return expr{}, newSelectorError(nil, "Invalid arguments for :lang()")
				}
			}
		}
		if len(langs) == 0 {
			// Upstream compiles the empty source "lambda el: ", and raises
			// SyntaxError.
			return expr{}, newSelectorError(nil, "Invalid arguments for :lang()")
		}
		return fn(func(el *ElementWrapper) bool {
			lang := el.Lang()
			for _, l := range langs {
				if lang == l || strings.HasPrefix(lang, l+"-") {
					return true
				}
			}
			return false
		}), nil
	}

	var nth, selectorList []tinycss2.Node
	ofSeen := false
	for _, argument := range selector.Arguments {
		if ident, ok := argument.(*tinycss2.IdentToken); ok && ident.Value == "of" && !ofSeen {
			ofSeen = true
			continue
		}
		if ofSeen {
			selectorList = append(selectorList, argument)
		} else {
			nth = append(nth, argument)
		}
	}

	// count returns the number of siblings before or after the element, and
	// false for the float("nan") of upstream.
	var count func(el *ElementWrapper) (int, bool)
	if len(selectorList) > 0 {
		selectors, err := parse(selectorList, parseState{depth: depth}, false, false)
		if err != nil {
			return expr{}, err
		}
		var tests []expr
		for _, s := range selectors {
			e, err := compileNode(s.ParsedTree, depth)
			if err != nil {
				return expr{}, err
			}
			tests = append(tests, e)
		}
		test := func(el *ElementWrapper) bool {
			for _, t := range tests {
				if !t.eval(el) {
					return false
				}
			}
			return true
		}
		switch selector.Name {
		case "nth-child":
			count = func(el *ElementWrapper) (int, bool) {
				n := 0
				for s := el.Previous; s != nil; s = s.Previous {
					if test(s) {
						n++
					}
				}
				return n, true
			}
		case "nth-last-child":
			count = func(el *ElementWrapper) (int, bool) {
				n := 0
				for s := range el.iterSiblingsAfter() {
					if test(s) {
						n++
					}
				}
				return n, true
			}
		case "nth-of-type":
			count = func(el *ElementWrapper) (int, bool) {
				n := 0
				for s := el.Previous; s != nil; s = s.Previous {
					if test(s) && sameTag(s.Element, el.Element) {
						n++
					}
				}
				return n, true
			}
		case "nth-last-of-type":
			count = func(el *ElementWrapper) (int, bool) {
				n := 0
				for s := range el.iterSiblingsAfter() {
					if test(s) && sameTag(s.Element, el.Element) {
						n++
					}
				}
				return n, true
			}
		default:
			return expr{}, newSelectorError(nil, "Unknown pseudo-class "+selector.Name)
		}
		inner := count
		count = func(el *ElementWrapper) (int, bool) {
			if !test(el) {
				return 0, false
			}
			return inner(el)
		}
	} else {
		if ofSeen {
			return expr{}, newSelectorError(nil, "Invalid arguments for :"+selector.Name+"()")
		}
		switch selector.Name {
		case "nth-child":
			count = func(el *ElementWrapper) (int, bool) { return el.Index, true }
		case "nth-last-child":
			count = func(el *ElementWrapper) (int, bool) {
				return len(el.EtreeSiblings()) - el.Index - 1, true
			}
		case "nth-of-type":
			count = func(el *ElementWrapper) (int, bool) {
				n := 0
				for _, s := range el.EtreeSiblings()[:el.Index] {
					if sameTag(s, el.Element) {
						n++
					}
				}
				return n, true
			}
		case "nth-last-of-type":
			count = func(el *ElementWrapper) (int, bool) {
				n := 0
				for _, s := range el.EtreeSiblings()[el.Index+1:] {
					if sameTag(s, el.Element) {
						n++
					}
				}
				return n, true
			}
		default:
			return expr{}, newSelectorError(nil, "Unknown pseudo-class "+selector.Name)
		}
	}

	a, b, ok := tinycss2.ParseNth(nth)
	if !ok {
		return expr{}, newSelectorError(nil, "Invalid arguments for :"+selector.Name+"()")
	}
	// x is the number of siblings before/after the element
	// Matches if a positive or zero integer n exists so that:
	// x = a*n + b-1
	// x = a*n + B
	bigA := big.NewInt(a)
	bigB := new(big.Int).Sub(big.NewInt(b), big.NewInt(1))
	return fn(func(el *ElementWrapper) bool {
		x, ok := count(el)
		if !ok {
			return false
		}
		return nthMatches(int64(x), a, b, bigA, bigB)
	}), nil
}

// nthMatches reports whether x = a*n + B for an integer n >= 0, with B =
// b-1, as divmod of Python does. It computes with big integers when a or b
// is so large that int64 can overflow.
func nthMatches(x, a, b int64, bigA, bigB *big.Int) bool {
	const limit = 1 << 40
	if a > -limit && a < limit && b > -limit && b < limit {
		bb := b - 1
		if a == 0 {
			// x = B
			return x == bb
		}
		// n = (x - B) / a
		d := x - bb
		return d%a == 0 && d/a >= 0
	}
	d := new(big.Int).Sub(big.NewInt(x), bigB)
	if bigA.Sign() == 0 {
		return d.Sign() == 0
	}
	q, r := new(big.Int).QuoRem(d, bigA, new(big.Int))
	return r.Sign() == 0 && q.Sign() >= 0
}
