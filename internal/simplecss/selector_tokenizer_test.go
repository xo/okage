// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"errors"
	"testing"

	"github.com/xo/okage"
)

// TestSelectorTokenizer ports the tests of simplecss
// tests/selector_tokenizer.rs that the macro tokenize! makes. Each subtest
// has the name of the upstream test.
func TestSelectorTokenizer(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []SelectorToken
	}{
		{"tokenize_01", "*", []SelectorToken{{Kind: SelectorTokenUniversalSelector}}},
		{"tokenize_02", "div", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}}},
		{"tokenize_03", "#div", []SelectorToken{{Kind: SelectorTokenIDSelector, Value: "div"}}},
		{"tokenize_04", ".div", []SelectorToken{{Kind: SelectorTokenClassSelector, Value: "div"}}},
		{"tokenize_05", "[id]", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorExists}}}},
		{"tokenize_06", "[id=test]", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "test"}}}},
		{"tokenize_07", "[id~=test]", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorContains, Value: "test"}}}},
		{"tokenize_08", "[id|=test]", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorStartsWith, Value: "test"}}}},
		{"tokenize_09", "[id='test']", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "test"}}}},
		{"tokenize_10", "[id=\"test\"]", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "test"}}}},
		{"tokenize_11", "[id='te\\'st']", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "te\\'st"}}}},
		{"tokenize_12", "[id=\"te\\\"st\"]", []SelectorToken{{Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "te\\\"st"}}}},
		{"tokenize_13", "div:first-child", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenPseudoClass, Value: "first-child"}}},
		{"tokenize_14", ":first-child", []SelectorToken{{Kind: SelectorTokenPseudoClass, Value: "first-child"}}},
		{"tokenize_15", "div p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_16", "div p a", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "a"}}},
		{"tokenize_17", "div>p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenChildCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_18", "div >p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenChildCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_19", "div> p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenChildCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_20", "div > p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenChildCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_21", "div .p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenClassSelector, Value: "p"}}},
		{"tokenize_22", "div *", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenUniversalSelector}}},
		{"tokenize_23", "div #p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenIDSelector, Value: "p"}}},
		{"tokenize_24", "div [id]", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorExists}}}},
		{"tokenize_25", "div :link", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenPseudoClass, Value: "link"}}},
		{"tokenize_26", "div+p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenAdjacentCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_27", "div +p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenAdjacentCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_28", "div+ p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenAdjacentCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_29", "div + p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenAdjacentCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_30", "div {", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}}},
		{"tokenize_31", "div,", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}}},
		{"tokenize_32", "div{", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}}},
		{"tokenize_33", "div ,", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}}},
		{"tokenize_34", "div.test", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenClassSelector, Value: "test"}}},
		{"tokenize_35", "div.test.warn", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenClassSelector, Value: "test"}, {Kind: SelectorTokenClassSelector, Value: "warn"}}},
		{"tokenize_36", "div#id", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenIDSelector, Value: "id"}}},
		{"tokenize_37", "*[id]", []SelectorToken{{Kind: SelectorTokenUniversalSelector}, {Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorExists}}}},
		{"tokenize_38", "*.test", []SelectorToken{{Kind: SelectorTokenUniversalSelector}, {Kind: SelectorTokenClassSelector, Value: "test"}}},
		{"tokenize_39", "*#id", []SelectorToken{{Kind: SelectorTokenUniversalSelector}, {Kind: SelectorTokenIDSelector, Value: "id"}}},
		{"tokenize_40", "div * p", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenUniversalSelector}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "p"}}},
		{"tokenize_41", "div[id=test][color=red]", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "div"}, {Kind: SelectorTokenAttributeSelector, Value: "id", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "test"}}, {Kind: SelectorTokenAttributeSelector, Value: "color", Operator: AttributeOperator{Kind: AttributeOperatorMatches, Value: "red"}}}},
		{"tokenize_42", "a.external:visited", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenClassSelector, Value: "external"}, {Kind: SelectorTokenPseudoClass, Value: "visited"}}},
		{"tokenize_43", ":lang(en)", []SelectorToken{{Kind: SelectorTokenLangPseudoClass, Value: "en"}}},
		{"tokenize_44", "a\nb", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}},
		{"tokenize_45", ".warn :first-child", []SelectorToken{{Kind: SelectorTokenClassSelector, Value: "warn"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenPseudoClass, Value: "first-child"}}},
		{"comment_01", "/**/a", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}}},
		{"comment_02", "/* */a", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}}},
		{"comment_03", "/* comment */a", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}}},
		{"comment_04", "/**/ /**/a", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}}},
		{"comment_05", "/**/ a /**/", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}}},
		{"comment_06", "a /**/ b", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}},
		{"comment_08", "a /**/b", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}},
		{"comment_09", "a/**/ b", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}},
		{"comment_10", "a/**/ /**/b", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}},
		{"comment_11", "a /**/ /**/ b", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}},
		{"comment_12", "a /**//**/ b", []SelectorToken{{Kind: SelectorTokenTypeSelector, Value: "a"}, {Kind: SelectorTokenDescendantCombinator}, {Kind: SelectorTokenTypeSelector, Value: "b"}}}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tok := NewSelectorTokenizer(test.text)
			for i, want := range test.want {
				got, ok, err := tok.Next()
				if err != nil || !ok {
					t.Fatalf("token %d: got %v %v, want %+v", i, ok, err, want)
				}
				if got != want {
					t.Fatalf("token %d: got %+v, want %+v", i, got, want)
				}
			}
			if got, ok, err := tok.Next(); ok || err != nil {
				t.Fatalf("got %+v %v, want none", got, err)
			}
		})
	}
}

// TestSelectorTokenizerMalformed ports the tests of simplecss
// tests/selector_tokenizer.rs that the macro malformed! makes. Each subtest
// has the name of the upstream test.
func TestSelectorTokenizerMalformed(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{"malformed_01", ">", "unexpected combinator"},
		{"malformed_02", "+", "unexpected combinator"},
		{"malformed_03", "> a", "unexpected combinator"},
		{"malformed_04", "a >", "selector missing"},
		{"malformed_05", "*a", "unexpected selector"},
		{"malformed_06", "a*", "unexpected selector"},
		{"malformed_07", "a > ,", "selector missing"},
		{"malformed_08", "a > >", "unexpected combinator"},
		{"malformed_09", "a > {", "selector missing"},
		{"malformed_10", "a/**/b", "unexpected selector"},
		{"malformed_11", "a < b", "invalid ident at 1:3"},
		{"malformed_12", ":lang()", "invalid language pseudo-class"},
		{"malformed_13", ":lang( )", "invalid language pseudo-class"},
		{"malformed_14", "::first-child", "invalid ident at 1:2"},
		{"malformed_15", "[olor:red", "invalid or unsupported attribute selector"},
		{"malformed_16", "", "selector missing"},
		{"malformed_17", " ", "selector missing"},
		{"malformed_18", "/**/", "selector missing"}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range NewSelectorTokenizer(test.text).All() {
				if err != nil {
					if got := err.Error(); got != test.want {
						t.Fatalf("got %q, want %q", got, test.want)
					}
					return
				}
			}
			t.Fatal("unreachable")
		})
	}
}

// TestSelectorTokenizerSentinel checks that an error of the tokenizer wraps
// the sentinel error of okage for its case (D21).
func TestSelectorTokenizerSentinel(t *testing.T) {
	tests := []struct {
		text string
		want error
	}{
		{">", okage.ErrCSSUnexpectedCombinator},
		{"", okage.ErrCSSSelectorMissing},
		{"a*", okage.ErrCSSUnexpectedSelector},
		{"a < b", okage.ErrCSSInvalidIdent},
		{":lang()", okage.ErrCSSInvalidLanguagePseudoClass},
		{"[olor:red", okage.ErrCSSInvalidAttributeSelector},
		{"[a~b]", okage.ErrCSSInvalidByte},
		{"a /*", okage.ErrCSSInvalidComment},
		{"[a", okage.ErrCSSUnexpectedEndOfStream},
	}
	for _, test := range tests {
		var got error
		for _, err := range NewSelectorTokenizer(test.text).All() {
			if err != nil {
				got = err
				break
			}
		}
		if !errors.Is(got, test.want) {
			t.Errorf("%q: got %v, want %v", test.text, got, test.want)
		}
	}
	tok := NewSelectorTokenizer("[a~b]")
	for {
		_, ok, err := tok.Next()
		if err != nil {
			if got, want := err.Error(), "expected '=' not 'b' at 1:4"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
			break
		}
		if !ok {
			t.Fatal("no error")
		}
	}
}
