// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 tests/test_tokenizer.py (D49). It runs the tests
// of html5lib-tests in testdata/tinyhtml5/tokenizer.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// tokenizerTestParser is TokenizerTestParser in tinyhtml5
// tests/test_tokenizer.py.
type tokenizerTestParser struct {
	state        tokenizerState
	lastStartTag *string
	outputTokens []any
}

// parse is TokenizerTestParser.parse in tinyhtml5 tests/test_tokenizer.py.
func (tp *tokenizerTestParser) parse(stream string) []any {
	tokenizer := newHTMLTokenizer(newUnicodeInputStream(stream), nil)
	tp.outputTokens = nil

	tokenizer.state = tp.state
	if tp.lastStartTag != nil {
		tokenizer.currentToken = &token{typ: tokenStartTag, name: *tp.lastStartTag}
	}

	for tok := range tokenizer.tokens() {
		switch tok.typ {
		case tokenDoctype:
			tp.outputTokens = append(tp.outputTokens, []any{
				"DOCTYPE", tok.name, optional(tok.publicID, tok.hasPublicID),
				optional(tok.systemID, tok.hasSystemID), tok.correct,
			})
		case tokenStartTag:
			data := map[string]any{}
			for _, a := range tok.attrs {
				data[a.Name] = a.Value
			}
			tp.outputTokens = append(tp.outputTokens, []any{"StartTag", tok.name, data, tok.selfClosing})
		case tokenEndTag:
			tp.outputTokens = append(tp.outputTokens, []any{"EndTag", tok.name, tok.selfClosing})
		case tokenComment:
			tp.outputTokens = append(tp.outputTokens, []any{"Comment", tok.data})
		case tokenSpaceCharacters, tokenCharacters:
			tp.outputTokens = append(tp.outputTokens, []any{"Character", tok.data})
		case tokenParseError:
			tp.outputTokens = append(tp.outputTokens, []any{"ParseError", tok.data})
		}
	}

	return tp.outputTokens
}

// optional is a value that is None in upstream when ok is false.
func optional(s string, ok bool) any {
	if !ok {
		return nil
	}
	return s
}

// concatenateCharacterTokens is concatenate_character_tokens in tinyhtml5
// tests/test_tokenizer.py.
func concatenateCharacterTokens(tokens []any) []any {
	var outputTokens []any
	for _, tok := range tokens {
		t, _ := tok.([]any)
		if len(outputTokens) > 0 && t != nil && t[0] == "Character" {
			if last, ok := outputTokens[len(outputTokens)-1].([]any); ok && last[0] == "Character" {
				a, _ := last[1].(string)
				b, _ := t[1].(string)
				outputTokens[len(outputTokens)-1] = []any{"Character", a + b}
				continue
			}
		}
		outputTokens = append(outputTokens, tok)
	}
	return outputTokens
}

// tokensMatch is tokens_match in tinyhtml5 tests/test_tokenizer.py. It
// tests whether the test has passed or failed. The parse errors are not
// compared.
func tokensMatch(expected, received []any) bool {
	checkSelfClosing := false
	for _, tok := range expected {
		t, ok := tok.([]any)
		if ok && (t[0] == "StartTag" && len(t) == 4 || t[0] == "EndTag" && len(t) == 3) {
			checkSelfClosing = true
		}
	}
	if !checkSelfClosing {
		for i, tok := range received {
			if t, ok := tok.([]any); ok && (t[0] == "StartTag" || t[0] == "EndTag") {
				received[i] = t[:len(t)-1]
			}
		}
	}

	// Sort the tokens into two groups: non-parse errors and parse errors.
	filter := func(tokens []any) []any {
		var out []any
		for _, tok := range tokens {
			if tok != "ParseError" {
				out = append(out, tok)
			}
		}
		return concatenateCharacterTokens(out)
	}
	return reflect.DeepEqual(filter(expected), filter(received))
}

// decodeEscapes is decode in tinyhtml5 tests/test_tokenizer.py. It decodes
// the \uXXXX escapes. Go has no lone surrogate in a string, so one becomes
// U+FFFD on both sides of a comparison.
func decodeEscapes(input string) string {
	return escapeRE.ReplaceAllStringFunc(input, func(m string) string {
		n, _ := strconv.ParseUint(m[2:], 16, 32)
		return string(rune(n))
	})
}

// escapeRE matches a \uXXXX escape.
var escapeRE = regexp.MustCompile(`\\u([0-9A-Fa-f]{4})`)

// tokenizerXfails is _xfails in tinyhtml5 tests/test_tokenizer.py.
var tokenizerXfails = map[string]bool{
	"domjs-4":   true,
	"test2-0":   true,
	"test3-280": true, "test3-283": true, "test3-284": true, "test3-286": true, "test3-287": true, "test3-289": true,
	"test3-292": true, "test3-293": true, "test3-295": true, "test3-296": true, "test3-298": true, "test3-310": true,
	"test3-718": true,
}

// tokenizerStates maps the name of an initial state of a test to the state.
var tokenizerStates = map[string]tokenizerState{
	"data_state":          dataState,
	"plaintext_state":     plaintextState,
	"rcdata_state":        rcdataState,
	"rawtext_state":       rawtextState,
	"script_data_state":   scriptDataState,
	"cdata_section_state": cdataSectionState,
}

// tokenizerTest is one test of a .test file.
type tokenizerTest struct {
	Description   string   `json:"description"`
	Input         string   `json:"input"`
	Output        []any    `json:"output"`
	InitialStates []string `json:"initialStates"`
	LastStartTag  *string  `json:"lastStartTag"`
	DoubleEscaped bool     `json:"doubleEscaped"`
}

// TestTokenizer is test_tokenizer in tinyhtml5 tests/test_tokenizer.py.
func TestTokenizer(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "tinyhtml5", "tokenizer", "*.test"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("finding the tests: %v", err)
	}
	stateName := regexp.MustCompile(`\W+(\w)`)
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var file struct {
			Tests []tokenizerTest `json:"tests"`
		}
		if err := json.Unmarshal(src, &file); err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		stem := strings.TrimSuffix(filepath.Base(path), ".test")
		for i, test := range file.Tests {
			id := fmt.Sprintf("%s-%d", stem, i)
			t.Run(id, func(t *testing.T) {
				if len(test.InitialStates) == 0 {
					test.InitialStates = []string{"Data state"}
				}
				if test.DoubleEscaped {
					test.Input = decodeEscapes(test.Input)
					for _, tok := range test.Output {
						if t, ok := tok.([]any); ok {
							if s, ok := t[1].(string); ok {
								t[1] = decodeEscapes(s)
							}
						}
					}
				}
				allMatch := true
				for _, initialState := range test.InitialStates {
					name := stateName.ReplaceAllString(strings.ToLower(initialState), "_$1")
					state, ok := tokenizerStates[name]
					if !ok {
						t.Fatalf("unknown state %q", initialState)
					}
					expected := test.Output
					parser := &tokenizerTestParser{state: state, lastStartTag: test.LastStartTag}
					tokens := parser.parse(test.Input)
					received := make([]any, len(tokens))
					for i, tok := range tokens {
						if t, _ := tok.([]any); t[0] == "ParseError" {
							received[i] = "ParseError"
						} else {
							received[i] = tok
						}
					}
					match := tokensMatch(expected, received)
					switch {
					case !match && !tokenizerXfails[id]:
						t.Errorf("\nInitial state: %s\nInput: %q\nExpected: %#v\nReceived: %#v", name, test.Input, expected, tokens)
					case !match:
						allMatch = false
					}
				}
				if allMatch && tokenizerXfails[id] {
					t.Logf("%s passes but is marked as xfail", id)
				}
			})
		}
	}
}
