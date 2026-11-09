// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"iter"
	"log/slog"
	"strings"

	"github.com/xo/okage"
)

// AttributeOperatorKind is the case of an AttributeOperator.
type AttributeOperatorKind uint8

// The cases of the enum AttributeOperator in simplecss src/selector.rs.
const (
	// AttributeOperatorExists is AttributeOperator::Exists, `[attr]`.
	AttributeOperatorExists AttributeOperatorKind = iota
	// AttributeOperatorMatches is AttributeOperator::Matches,
	// `[attr=value]`.
	AttributeOperatorMatches
	// AttributeOperatorContains is AttributeOperator::Contains,
	// `[attr~=value]`.
	AttributeOperatorContains
	// AttributeOperatorStartsWith is AttributeOperator::StartsWith,
	// `[attr|=value]`.
	AttributeOperatorStartsWith
)

// String returns the name of the case in upstream.
func (k AttributeOperatorKind) String() string {
	switch k {
	case AttributeOperatorExists:
		return "Exists"
	case AttributeOperatorMatches:
		return "Matches"
	case AttributeOperatorContains:
		return "Contains"
	case AttributeOperatorStartsWith:
		return "StartsWith"
	}
	return "AttributeOperatorKind(?)"
}

// AttributeOperator is AttributeOperator in simplecss src/selector.rs. It is
// an attribute selector operator.
//
// Port: the Rust enum holds a value in each case but Exists, so it becomes a
// struct with a kind (D27).
type AttributeOperator struct {
	Kind AttributeOperatorKind
	// Value is the value of Matches, Contains and StartsWith.
	Value string
}

// Matches is AttributeOperator::matches in simplecss src/selector.rs. It
// checks that value is matching the operator.
func (op AttributeOperator) Matches(value string) bool {
	switch op.Kind {
	case AttributeOperatorExists:
		return true
	case AttributeOperatorMatches:
		return value == op.Value
	case AttributeOperatorContains:
		for s := range strings.SplitSeq(value, " ") {
			if s == op.Value {
				return true
			}
		}
		return false
	case AttributeOperatorStartsWith:
		v := op.Value
		// exactly `v` or beginning with `v` immediately followed by `-`
		switch {
		case value == v:
			return true
		case strings.HasPrefix(value, v):
			return len(value) > len(v) && value[len(v)] == '-'
		default:
			return false
		}
	}
	return false
}

// PseudoClassKind is the case of a PseudoClass.
type PseudoClassKind uint8

// The cases of the enum PseudoClass in simplecss src/selector.rs.
const (
	// PseudoClassFirstChild is PseudoClass::FirstChild.
	PseudoClassFirstChild PseudoClassKind = iota
	// PseudoClassLink is PseudoClass::Link.
	PseudoClassLink
	// PseudoClassVisited is PseudoClass::Visited.
	PseudoClassVisited
	// PseudoClassHover is PseudoClass::Hover.
	PseudoClassHover
	// PseudoClassActive is PseudoClass::Active.
	PseudoClassActive
	// PseudoClassFocus is PseudoClass::Focus.
	PseudoClassFocus
	// PseudoClassLang is PseudoClass::Lang.
	PseudoClassLang
)

// String returns the name of the case in upstream.
func (k PseudoClassKind) String() string {
	switch k {
	case PseudoClassFirstChild:
		return "FirstChild"
	case PseudoClassLink:
		return "Link"
	case PseudoClassVisited:
		return "Visited"
	case PseudoClassHover:
		return "Hover"
	case PseudoClassActive:
		return "Active"
	case PseudoClassFocus:
		return "Focus"
	case PseudoClassLang:
		return "Lang"
	}
	return "PseudoClassKind(?)"
}

// PseudoClass is PseudoClass in simplecss src/selector.rs. It is a
// pseudo-class.
//
// Port: only the case Lang of the Rust enum holds a value, so it becomes a
// struct with a kind (D27).
type PseudoClass struct {
	Kind PseudoClassKind
	// Lang is the language of PseudoClassLang.
	Lang string
}

// String is fmt::Display for PseudoClass in simplecss src/selector.rs.
func (c PseudoClass) String() string {
	switch c.Kind {
	case PseudoClassFirstChild:
		return "first-child"
	case PseudoClassLink:
		return "link"
	case PseudoClassVisited:
		return "visited"
	case PseudoClassHover:
		return "hover"
	case PseudoClassActive:
		return "active"
	case PseudoClassFocus:
		return "focus"
	case PseudoClassLang:
		return "lang(" + c.Lang + ")"
	}
	return ""
}

// Element is the trait Element in simplecss src/selector.rs. It queries an
// element node metadata.
//
// Port: the Rust trait returns Option<Self>. The Go interface returns an
// Element and a bool.
type Element interface {
	// ParentElement returns a parent element.
	ParentElement() (Element, bool)

	// PrevSiblingElement returns a previous sibling element.
	PrevSiblingElement() (Element, bool)

	// HasLocalName checks that the element has a specified local name.
	HasLocalName(name string) bool

	// AttributeMatches checks that the element has a specified attribute.
	AttributeMatches(localName string, operator AttributeOperator) bool

	// PseudoClassMatches checks that the element matches a specified
	// pseudo-class.
	PseudoClassMatches(class PseudoClass) bool
}

// simpleSelectorTypeKind is the case of a simpleSelectorType.
type simpleSelectorTypeKind uint8

// The cases of the enum SimpleSelectorType in simplecss src/selector.rs.
const (
	// simpleSelectorTypeType is SimpleSelectorType::Type.
	simpleSelectorTypeType simpleSelectorTypeKind = iota
	// simpleSelectorTypeUniversal is SimpleSelectorType::Universal.
	simpleSelectorTypeUniversal
)

// simpleSelectorType is SimpleSelectorType in simplecss src/selector.rs.
type simpleSelectorType struct {
	kind simpleSelectorTypeKind
	// ident is the name of simpleSelectorTypeType.
	ident string
}

// subSelectorKind is the case of a subSelector.
type subSelectorKind uint8

// The cases of the enum SubSelector in simplecss src/selector.rs.
const (
	// subSelectorAttribute is SubSelector::Attribute.
	subSelectorAttribute subSelectorKind = iota
	// subSelectorPseudoClass is SubSelector::PseudoClass.
	subSelectorPseudoClass
)

// subSelector is SubSelector in simplecss src/selector.rs.
type subSelector struct {
	kind subSelectorKind
	// name and operator are the data of subSelectorAttribute.
	name     string
	operator AttributeOperator
	// class is the data of subSelectorPseudoClass.
	class PseudoClass
}

// simpleSelector is SimpleSelector in simplecss src/selector.rs.
type simpleSelector struct {
	kind         simpleSelectorType
	subselectors []subSelector
}

// combinator is Combinator in simplecss src/selector.rs.
type combinator uint8

// The cases of the enum Combinator in simplecss src/selector.rs.
const (
	// combinatorNone is Combinator::None.
	combinatorNone combinator = iota
	// combinatorDescendant is Combinator::Descendant.
	combinatorDescendant
	// combinatorChild is Combinator::Child.
	combinatorChild
	// combinatorAdjacentSibling is Combinator::AdjacentSibling.
	combinatorAdjacentSibling
)

// String returns the name of the case in upstream.
func (c combinator) String() string {
	switch c {
	case combinatorNone:
		return "None"
	case combinatorDescendant:
		return "Descendant"
	case combinatorChild:
		return "Child"
	case combinatorAdjacentSibling:
		return "AdjacentSibling"
	}
	return "combinator(?)"
}

// component is Component in simplecss src/selector.rs.
type component struct {
	// combinator is a combinator that precede the selector.
	combinator combinator
	selector   simpleSelector
}

// Selector is Selector in simplecss src/selector.rs. It is a selector.
type Selector struct {
	components []component
}

// ParseSelector is Selector::parse in simplecss src/selector.rs. It parses a
// selector from a string.
//
// Will log any errors as a warnings, to the logger that WithLogger gives.
//
// Parsing will be stopped at EOF, `,` or `{`.
func ParseSelector(text string, opts ...Option) (Selector, bool) {
	o := newOptions(opts)
	selector, ok, _ := parse(text, o.logger)
	return selector, ok
}

// Specificity is Selector::specificity in simplecss src/selector.rs. It
// computes the selector's specificity.
//
// Cf. <https://www.w3.org/TR/selectors/#specificity>.
func (s Selector) Specificity() [3]uint8 {
	var spec [3]uint8

	for i := range s.components {
		selector := &s.components[i].selector
		if selector.kind.kind == simpleSelectorTypeType {
			spec[2] = saturatingAddU8(spec[2], 1)
		}

		for _, sub := range selector.subselectors {
			if sub.kind == subSelectorAttribute && sub.name == "id" {
				spec[0] = saturatingAddU8(spec[0], 1)
			} else {
				spec[1] = saturatingAddU8(spec[1], 1)
			}
		}
	}

	return spec
}

// Matches is Selector::matches in simplecss src/selector.rs. It checks that
// the provided element matches the current selector.
func (s Selector) Matches(element Element) bool {
	if len(s.components) == 0 {
		panic("selector must not be empty")
	}
	if s.components[0].combinator != combinatorNone {
		panic("the first component must not have a combinator")
	}

	return s.matchesImpl(len(s.components)-1, element)
}

// matchesImpl is Selector::matches_impl in simplecss src/selector.rs.
func (s Selector) matchesImpl(idx int, element Element) bool {
	component := &s.components[idx]

	if !matchSelector(&component.selector, element) {
		return false
	}

	switch component.combinator {
	case combinatorDescendant:
		parent, ok := element.ParentElement()
		for ok {
			if s.matchesImpl(idx-1, parent) {
				return true
			}

			parent, ok = parent.ParentElement()
		}

		return false
	case combinatorChild:
		if parent, ok := element.ParentElement(); ok {
			if s.matchesImpl(idx-1, parent) {
				return true
			}
		}

		return false
	case combinatorAdjacentSibling:
		if prev, ok := element.PrevSiblingElement(); ok {
			if s.matchesImpl(idx-1, prev) {
				return true
			}
		}

		return false
	case combinatorNone:
		return true
	}
	return true
}

// matchSelector is match_selector in simplecss src/selector.rs.
func matchSelector(selector *simpleSelector, element Element) bool {
	if selector.kind.kind == simpleSelectorTypeType {
		if !element.HasLocalName(selector.kind.ident) {
			return false
		}
	}

	for _, sub := range selector.subselectors {
		switch sub.kind {
		case subSelectorAttribute:
			if !element.AttributeMatches(sub.name, sub.operator) {
				return false
			}
		case subSelectorPseudoClass:
			if !element.PseudoClassMatches(sub.class) {
				return false
			}
		}
	}

	return true
}

// parse is parse in simplecss src/selector.rs. It returns the selector,
// whether there is one, and the offset in text where parsing stopped. It
// logs its warnings to l.
func parse(text string, l *slog.Logger) (Selector, bool, int) {
	var components []component
	combinator := combinatorNone

	tokenizer := NewSelectorTokenizer(text)
	for {
		token, ok, err := tokenizer.Next()
		if err == nil && !ok {
			break
		}

		addSub := func(sub subSelector) {
			if combinator == combinatorNone && len(components) != 0 {
				component := &components[len(components)-1]
				component.selector.subselectors = append(component.selector.subselectors, sub)
			} else {
				components = append(components, component{
					selector: simpleSelector{
						kind:         simpleSelectorType{kind: simpleSelectorTypeUniversal},
						subselectors: []subSelector{sub},
					},
					combinator: combinator,
				})

				combinator = combinatorNone
			}
		}

		if err != nil {
			warnf(l, "Selector parsing failed cause %s.", err)
			return Selector{}, false, tokenizer.stream.Pos()
		}

		switch token.Kind {
		case SelectorTokenUniversalSelector:
			components = append(components, component{
				selector: simpleSelector{
					kind:         simpleSelectorType{kind: simpleSelectorTypeUniversal},
					subselectors: nil,
				},
				combinator: combinator,
			})

			combinator = combinatorNone
		case SelectorTokenTypeSelector:
			components = append(components, component{
				selector: simpleSelector{
					kind:         simpleSelectorType{kind: simpleSelectorTypeType, ident: token.Value},
					subselectors: nil,
				},
				combinator: combinator,
			})

			combinator = combinatorNone
		case SelectorTokenClassSelector:
			addSub(subSelector{
				kind:     subSelectorAttribute,
				name:     "class",
				operator: AttributeOperator{Kind: AttributeOperatorContains, Value: token.Value},
			})
		case SelectorTokenIDSelector:
			addSub(subSelector{
				kind:     subSelectorAttribute,
				name:     "id",
				operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: token.Value},
			})
		case SelectorTokenAttributeSelector:
			addSub(subSelector{kind: subSelectorAttribute, name: token.Value, operator: token.Operator})
		case SelectorTokenPseudoClass:
			var class PseudoClass
			switch token.Value {
			case "first-child":
				class = PseudoClass{Kind: PseudoClassFirstChild}
			case "link":
				class = PseudoClass{Kind: PseudoClassLink}
			case "visited":
				class = PseudoClass{Kind: PseudoClassVisited}
			case "hover":
				class = PseudoClass{Kind: PseudoClassHover}
			case "active":
				class = PseudoClass{Kind: PseudoClassActive}
			case "focus":
				class = PseudoClass{Kind: PseudoClassFocus}
			default:
				warnf(l, "':%s' is not supported. Selector skipped.", token.Value)
				return Selector{}, false, tokenizer.stream.Pos()
			}

			// TODO: duplicates
			// TODO: order

			addSub(subSelector{kind: subSelectorPseudoClass, class: class})
		case SelectorTokenLangPseudoClass:
			addSub(subSelector{
				kind:  subSelectorPseudoClass,
				class: PseudoClass{Kind: PseudoClassLang, Lang: token.Value},
			})
		case SelectorTokenDescendantCombinator:
			combinator = combinatorDescendant
		case SelectorTokenChildCombinator:
			combinator = combinatorChild
		case SelectorTokenAdjacentCombinator:
			combinator = combinatorAdjacentSibling
		}
	}

	switch {
	case len(components) == 0:
		return Selector{}, false, tokenizer.stream.Pos()
	case components[0].combinator != combinatorNone:
		// Port: upstream has a debug_assert_eq! here, which the release build
		// of upstream does not run.
		return Selector{}, false, tokenizer.stream.Pos()
	default:
		return Selector{components: components}, true, tokenizer.stream.Pos()
	}
}

// String is fmt::Display for Selector in simplecss src/selector.rs.
func (s Selector) String() string {
	var f strings.Builder
	for _, component := range s.components {
		switch component.combinator {
		case combinatorDescendant:
			f.WriteString(" ")
		case combinatorChild:
			f.WriteString(" > ")
		case combinatorAdjacentSibling:
			f.WriteString(" + ")
		case combinatorNone:
		}

		switch component.selector.kind.kind {
		case simpleSelectorTypeUniversal:
			f.WriteString("*")
		case simpleSelectorTypeType:
			f.WriteString(component.selector.kind.ident)
		}

		for _, sel := range component.selector.subselectors {
			switch sel.kind {
			case subSelectorAttribute:
				switch sel.operator.Kind {
				case AttributeOperatorExists:
					f.WriteString("[" + sel.name + "]")
				case AttributeOperatorMatches:
					f.WriteString("[" + sel.name + "='" + sel.operator.Value + "']")
				case AttributeOperatorContains:
					f.WriteString("[" + sel.name + "~='" + sel.operator.Value + "']")
				case AttributeOperatorStartsWith:
					f.WriteString("[" + sel.name + "|='" + sel.operator.Value + "']")
				}
			case subSelectorPseudoClass:
				f.WriteString(":" + sel.class.String())
			}
		}
	}

	return f.String()
}

// SelectorTokenKind is the case of a SelectorToken.
type SelectorTokenKind uint8

// The cases of the enum SelectorToken in simplecss src/selector.rs.
const (
	// SelectorTokenUniversalSelector is SelectorToken::UniversalSelector,
	// `*`.
	SelectorTokenUniversalSelector SelectorTokenKind = iota
	// SelectorTokenTypeSelector is SelectorToken::TypeSelector, `div`.
	SelectorTokenTypeSelector
	// SelectorTokenClassSelector is SelectorToken::ClassSelector, `.class`.
	SelectorTokenClassSelector
	// SelectorTokenIDSelector is SelectorToken::IdSelector, `#id`.
	SelectorTokenIDSelector
	// SelectorTokenAttributeSelector is SelectorToken::AttributeSelector,
	// `[color=red]`.
	SelectorTokenAttributeSelector
	// SelectorTokenPseudoClass is SelectorToken::PseudoClass,
	// `:first-child`.
	SelectorTokenPseudoClass
	// SelectorTokenLangPseudoClass is SelectorToken::LangPseudoClass,
	// `:lang(en)`.
	SelectorTokenLangPseudoClass
	// SelectorTokenDescendantCombinator is
	// SelectorToken::DescendantCombinator, `a b`.
	SelectorTokenDescendantCombinator
	// SelectorTokenChildCombinator is SelectorToken::ChildCombinator,
	// `a > b`.
	SelectorTokenChildCombinator
	// SelectorTokenAdjacentCombinator is SelectorToken::AdjacentCombinator,
	// `a + b`.
	SelectorTokenAdjacentCombinator
)

// String returns the name of the case in upstream.
func (k SelectorTokenKind) String() string {
	switch k {
	case SelectorTokenUniversalSelector:
		return "UniversalSelector"
	case SelectorTokenTypeSelector:
		return "TypeSelector"
	case SelectorTokenClassSelector:
		return "ClassSelector"
	case SelectorTokenIDSelector:
		return "IdSelector"
	case SelectorTokenAttributeSelector:
		return "AttributeSelector"
	case SelectorTokenPseudoClass:
		return "PseudoClass"
	case SelectorTokenLangPseudoClass:
		return "LangPseudoClass"
	case SelectorTokenDescendantCombinator:
		return "DescendantCombinator"
	case SelectorTokenChildCombinator:
		return "ChildCombinator"
	case SelectorTokenAdjacentCombinator:
		return "AdjacentCombinator"
	}
	return "SelectorTokenKind(?)"
}

// SelectorToken is SelectorToken in simplecss src/selector.rs. It is a
// selector token.
//
// Port: the cases of the Rust enum hold a string, an attribute operator or
// nothing, so it becomes a struct with a kind (D27).
type SelectorToken struct {
	Kind SelectorTokenKind
	// Value is the identifier of TypeSelector, ClassSelector, IdSelector and
	// PseudoClass, the name of AttributeSelector, and the language of
	// LangPseudoClass.
	Value string
	// Operator is the operator of AttributeSelector.
	Operator AttributeOperator
}

// SelectorTokenizer is SelectorTokenizer in simplecss src/selector.rs. It is
// a selector tokenizer.
type SelectorTokenizer struct {
	stream          stream
	afterCombinator bool
	finished        bool
}

// NewSelectorTokenizer is From<&str> for SelectorTokenizer in simplecss
// src/selector.rs.
func NewSelectorTokenizer(text string) SelectorTokenizer {
	return SelectorTokenizer{
		stream:          newStream(text),
		afterCombinator: true,
		finished:        false,
	}
}

// Next is Iterator::next of SelectorTokenizer in simplecss src/selector.rs.
//
// Upstream returns Option<Result<SelectorToken, Error>>. Next returns the
// token and true for Some(Ok(token)), a non-nil error for Some(Err(e)), and
// false with a nil error for None, at the end.
func (t *SelectorTokenizer) Next() (SelectorToken, bool, error) {
	if t.finished || t.stream.AtEnd() {
		if t.afterCombinator {
			t.afterCombinator = false
			return SelectorToken{}, false, newError(okage.ErrCSSSelectorMissing)
		}

		return SelectorToken{}, false, nil
	}

	// try2 is the macro try2! of upstream. It marks t as finished and
	// returns err.
	try2 := func(err error) (SelectorToken, bool, error) {
		t.finished = true
		return SelectorToken{}, false, err
	}

	switch t.stream.CurrByteUnchecked() {
	case '*':
		if !t.afterCombinator {
			t.finished = true
			return SelectorToken{}, false, newError(okage.ErrCSSUnexpectedSelector)
		}

		t.afterCombinator = false
		t.stream.Advance(1)
		return SelectorToken{Kind: SelectorTokenUniversalSelector}, true, nil
	case '#':
		t.afterCombinator = false
		t.stream.Advance(1)
		ident, err := t.stream.ConsumeIdent()
		if err != nil {
			return try2(err)
		}
		return SelectorToken{Kind: SelectorTokenIDSelector, Value: ident}, true, nil
	case '.':
		t.afterCombinator = false
		t.stream.Advance(1)
		ident, err := t.stream.ConsumeIdent()
		if err != nil {
			return try2(err)
		}
		return SelectorToken{Kind: SelectorTokenClassSelector, Value: ident}, true, nil
	case '[':
		t.afterCombinator = false
		t.stream.Advance(1)
		ident, err := t.stream.ConsumeIdent()
		if err != nil {
			return try2(err)
		}

		c, err := t.stream.CurrByte()
		if err != nil {
			return try2(err)
		}
		var op AttributeOperator
		switch c {
		case ']':
			op = AttributeOperator{Kind: AttributeOperatorExists}
		case '=':
			t.stream.Advance(1)
			value, err := t.stream.ConsumeString()
			if err != nil {
				return try2(err)
			}
			op = AttributeOperator{Kind: AttributeOperatorMatches, Value: value}
		case '~':
			t.stream.Advance(1)
			if err := t.stream.ConsumeByte('='); err != nil {
				return try2(err)
			}
			value, err := t.stream.ConsumeString()
			if err != nil {
				return try2(err)
			}
			op = AttributeOperator{Kind: AttributeOperatorContains, Value: value}
		case '|':
			t.stream.Advance(1)
			if err := t.stream.ConsumeByte('='); err != nil {
				return try2(err)
			}
			value, err := t.stream.ConsumeString()
			if err != nil {
				return try2(err)
			}
			op = AttributeOperator{Kind: AttributeOperatorStartsWith, Value: value}
		default:
			t.finished = true
			return SelectorToken{}, false, newError(okage.ErrCSSInvalidAttributeSelector)
		}

		if err := t.stream.ConsumeByte(']'); err != nil {
			return try2(err)
		}

		return SelectorToken{Kind: SelectorTokenAttributeSelector, Value: ident, Operator: op}, true, nil
	case ':':
		t.afterCombinator = false
		t.stream.Advance(1)
		ident, err := t.stream.ConsumeIdent()
		if err != nil {
			return try2(err)
		}

		if ident == "lang" {
			if err := t.stream.ConsumeByte('('); err != nil {
				return try2(err)
			}
			lang := strings.TrimSpace(t.stream.ConsumeBytes(func(c byte) bool { return c != ')' }))
			if err := t.stream.ConsumeByte(')'); err != nil {
				return try2(err)
			}

			if lang == "" {
				t.finished = true
				return SelectorToken{}, false, newError(okage.ErrCSSInvalidLanguagePseudoClass)
			}

			return SelectorToken{Kind: SelectorTokenLangPseudoClass, Value: lang}, true, nil
		}
		return SelectorToken{Kind: SelectorTokenPseudoClass, Value: ident}, true, nil
	case '>':
		if t.afterCombinator {
			t.afterCombinator = false
			t.finished = true
			return SelectorToken{}, false, newError(okage.ErrCSSUnexpectedCombinator)
		}

		t.stream.Advance(1)
		t.afterCombinator = true
		return SelectorToken{Kind: SelectorTokenChildCombinator}, true, nil
	case '+':
		if t.afterCombinator {
			t.afterCombinator = false
			t.finished = true
			return SelectorToken{}, false, newError(okage.ErrCSSUnexpectedCombinator)
		}

		t.stream.Advance(1)
		t.afterCombinator = true
		return SelectorToken{Kind: SelectorTokenAdjacentCombinator}, true, nil
	case ' ', '\t', '\n', '\r', '\x0C':
		t.stream.SkipSpaces()

		if t.afterCombinator {
			return t.Next()
		}

		for t.stream.isCurrByte('/') {
			if err := t.stream.SkipComment(); err != nil {
				return try2(err)
			}
			t.stream.SkipSpaces()
		}

		c, err := t.stream.CurrByte()
		if err != nil {
			return t.Next()
		}
		switch c {
		case '>', '+', ',', '{':
			return t.Next()
		default:
			if t.afterCombinator {
				t.afterCombinator = false
				t.finished = true
				return SelectorToken{}, false, newError(okage.ErrCSSUnexpectedSelector)
			}

			t.afterCombinator = true
			return SelectorToken{Kind: SelectorTokenDescendantCombinator}, true, nil
		}
	case '/':
		if t.stream.isNextByte('*') {
			if err := t.stream.SkipComment(); err != nil {
				return try2(err)
			}
		} else {
			t.finished = true
		}

		return t.Next()
	case ',', '{':
		t.finished = true
		return t.Next()
	default:
		ident, err := t.stream.ConsumeIdent()
		if err != nil {
			return try2(err)
		}

		if !t.afterCombinator {
			t.finished = true
			return SelectorToken{}, false, newError(okage.ErrCSSUnexpectedSelector)
		}

		t.afterCombinator = false
		return SelectorToken{Kind: SelectorTokenTypeSelector, Value: ident}, true, nil
	}
}

// All returns an iterator over the remaining tokens and their errors, for
// use with range. It iterates over a copy of t, so it does not change t.
//
// Port: it ports no upstream function. It is the Go form of the Rust
// Iterator impl of SelectorTokenizer. An error is the item that upstream
// returns as Some(Err(e)), and it comes with a zero token.
func (t SelectorTokenizer) All() iter.Seq2[SelectorToken, error] {
	return func(yield func(SelectorToken, error) bool) {
		for {
			tok, ok, err := t.Next()
			if err == nil && !ok {
				return
			}
			if !yield(tok, err) {
				return
			}
		}
	}
}

// saturatingAddU8 is u8::saturating_add of Rust.
func saturatingAddU8(a, b uint8) uint8 {
	if a > 255-b {
		return 255
	}
	return a + b
}
