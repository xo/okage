// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

// This file ports roxmltree src/tokenizer_tests.rs. Each test! and test_err!
// of upstream is a row of tokenizerTests, and a subtest of
// TestTokenizerTests with the name of the Rust test.

import (
	"errors"
	"strings"
	"testing"
	"unsafe"
)

// TestTextPos1 is the test text_pos_1 in roxmltree src/tokenizer_tests.rs.
func TestTextPos1(t *testing.T) {
	s := newStream("text")
	s.Advance(2)
	if got, want := s.GenTextPos(), NewTextPos(1, 3); got != want {
		t.Errorf("GenTextPos() = %v, want %v", got, want)
	}
}

// TestTextPos2 is the test text_pos_2 in roxmltree src/tokenizer_tests.rs.
func TestTextPos2(t *testing.T) {
	s := newStream("text\ntext")
	s.Advance(6)
	if got, want := s.GenTextPos(), NewTextPos(2, 2); got != want {
		t.Errorf("GenTextPos() = %v, want %v", got, want)
	}
}

// TestTextPos3 is the test text_pos_3 in roxmltree src/tokenizer_tests.rs.
func TestTextPos3(t *testing.T) {
	s := newStream("текст\nтекст")
	s.Advance(15)
	if got, want := s.GenTextPos(), NewTextPos(2, 3); got != want {
		t.Errorf("GenTextPos() = %v, want %v", got, want)
	}
}

// TestTokenSize is the test token_size in roxmltree src/tokenizer_tests.rs.
func TestTokenSize(t *testing.T) {
	if size := unsafe.Sizeof(token{}); size > 196 {
		t.Errorf("unsafe.Sizeof(token{}) = %d, want <= 196", size)
	}
}

// TestSpanSize is the test span_size in roxmltree src/tokenizer_tests.rs.
func TestSpanSize(t *testing.T) {
	if size := unsafe.Sizeof(strSpan{}); size > 48 {
		t.Errorf("unsafe.Sizeof(strSpan{}) = %d, want <= 48", size)
	}
}

// testElementEnd is the enum ElementEnd in roxmltree
// src/tokenizer_tests.rs.
type testElementEnd struct {
	kind   elementEndKind
	prefix string
	local  string
}

var (
	eeOpen  = testElementEnd{kind: elementEndOpen}
	eeEmpty = testElementEnd{kind: elementEndEmpty}
)

func eeClose(prefix, local string) testElementEnd {
	return testElementEnd{kind: elementEndClose, prefix: prefix, local: local}
}

// testToken is the enum Token in roxmltree src/tokenizer_tests.rs.
type testToken struct {
	kind  string
	a     string
	b     string
	hasB  bool
	c     string
	start int
	end   int
	ee    testElementEnd
}

func tPI(target, value string, hasValue bool, start, end int) testToken {
	return testToken{kind: "PI", a: target, b: value, hasB: hasValue, start: start, end: end}
}

func tComment(text string, start, end int) testToken {
	return testToken{kind: "Comment", a: text, start: start, end: end}
}

func tEntityDecl(name, value string) testToken {
	return testToken{kind: "EntityDecl", a: name, b: value}
}

func tElementStart(prefix, local string, start int) testToken {
	return testToken{kind: "ElementStart", a: prefix, b: local, start: start}
}

func tAttribute(prefix, local, value string) testToken {
	return testToken{kind: "Attribute", a: prefix, b: local, c: value}
}

func tElementEnd(ee testElementEnd, start, end int) testToken {
	return testToken{kind: "ElementEnd", ee: ee, start: start, end: end}
}

func tText(text string, start, end int) testToken {
	return testToken{kind: "Text", a: text, start: start, end: end}
}

func tCdata(text string, start, end int) testToken {
	return testToken{kind: "Cdata", a: text, start: start, end: end}
}

func tError(msg string) testToken {
	return testToken{kind: "Error", a: msg}
}

// eventsCollector is EventsCollector in roxmltree src/tokenizer_tests.rs.
type eventsCollector struct {
	tokens []testToken
}

func (c *eventsCollector) token(tok token) error {
	var t testToken
	switch tok.kind {
	case tokenProcessingInstruction:
		t = tPI(tok.prefix, tok.local, tok.local != "", tok.rng.Start, tok.rng.End)
	case tokenComment:
		t = tComment(tok.local, tok.rng.Start, tok.rng.End)
	case tokenEntityDeclaration:
		t = tEntityDecl(tok.local, tok.span.AsStr())
	case tokenElementStart:
		t = tElementStart(tok.prefix, tok.local, tok.start)
	case tokenAttribute:
		t = tAttribute(tok.prefix, tok.local, tok.span.AsStr())
	case tokenElementEnd:
		var ee testElementEnd
		switch tok.end.kind {
		case elementEndOpen:
			ee = eeOpen
		case elementEndClose:
			ee = eeClose(tok.end.prefix, tok.end.local)
		case elementEndEmpty:
			ee = eeEmpty
		}
		t = tElementEnd(ee, tok.rng.Start, tok.rng.End)
	case tokenText:
		t = tText(tok.local, tok.rng.Start, tok.rng.End)
	case tokenCdata:
		t = tCdata(tok.local, tok.rng.Start, tok.rng.End)
	}
	c.tokens = append(c.tokens, t)
	return nil
}

// resolveEntity is the default XmlEvents::resolve_entity, which returns
// Ok(None).
func (c *eventsCollector) resolveEntity(string, bool, string) (string, bool, error) {
	return "", false, nil
}

// collectTokens is collect_tokens in roxmltree src/tokenizer_tests.rs.
func collectTokens(text string) []testToken {
	collector := &eventsCollector{}
	if err := parseTokenizer(text, true, collector); err != nil {
		var e *Error
		if !errors.As(err, &e) {
			panic(err)
		}
		collector.tokens = append(collector.tokens, tError(e.Message()))
	}
	return collector.tokens
}

// TestDtdEntity07 is the test dtd_entity_07 in roxmltree
// src/tokenizer_tests.rs.
//
// We do not support !ELEMENT DTD token and it will be skipped.
// Previously, we were calling `Tokenizer::next` after the skip,
// which is recursive and could cause a stack overflow when there are too many sequential
// unsupported tokens.
// This tests checks that the current code do not crash with stack overflow.
func TestDtdEntity07(t *testing.T) {
	var text strings.Builder
	text.WriteString("<!DOCTYPE svg [\n")
	for range 500 {
		text.WriteString("<!ELEMENT sgml ANY>\n")
	}
	text.WriteString("]>\n")

	collector := &eventsCollector{}
	if err := parseTokenizer(text.String(), true, collector); err != nil {
		t.Fatal(err)
	}
}

// TestTokenizerTests runs each test! and test_err! of roxmltree
// src/tokenizer_tests.rs as a subtest. A test! compares the tokens, and the
// error text as its last token. A test_err! checks only that the parse
// fails.
func TestTokenizerTests(t *testing.T) {
	for _, tt := range tokenizerTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isErr {
				collector := &eventsCollector{}
				if parseTokenizer(tt.text, true, collector) == nil {
					t.Errorf("parse(%q) is not an error", tt.text)
				}
				return
			}

			tokens := collectTokens(tt.text)
			for i, want := range tt.tokens {
				if i >= len(tokens) {
					t.Fatalf("token %d is missing, want %+v", i, want)
				}
				if tokens[i] != want {
					t.Errorf("token %d = %+v, want %+v", i, tokens[i], want)
				}
			}
			if len(tokens) > len(tt.tokens) {
				t.Errorf("extra tokens %+v", tokens[len(tt.tokens):])
			}
		})
	}
}

var tokenizerTests = []struct {
	name   string
	text   string
	tokens []testToken
	isErr  bool
}{
	{
		name: "cdata_01",
		text: "<p><![CDATA[content]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("content", 3, 22),
			tElementEnd(eeClose("", "p"), 22, 26),
		},
	},
	{
		name: "cdata_02",
		text: "<p><![CDATA[&amping]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("&amping", 3, 22),
			tElementEnd(eeClose("", "p"), 22, 26),
		},
	},
	{
		name: "cdata_03",
		text: "<p><![CDATA[&amping ]]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("&amping ]", 3, 24),
			tElementEnd(eeClose("", "p"), 24, 28),
		},
	},
	{
		name: "cdata_04",
		text: "<p><![CDATA[&amping]] ]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("&amping]] ", 3, 25),
			tElementEnd(eeClose("", "p"), 25, 29),
		},
	},
	{
		name: "cdata_05",
		text: "<p><![CDATA[<message>text</message>]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("<message>text</message>", 3, 38),
			tElementEnd(eeClose("", "p"), 38, 42),
		},
	},
	{
		name: "cdata_06",
		text: "<p><![CDATA[</this is malformed!</malformed</malformed & worse>]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("</this is malformed!</malformed</malformed & worse>", 3, 66),
			tElementEnd(eeClose("", "p"), 66, 70),
		},
	},
	{
		name: "cdata_07",
		text: "<p><![CDATA[1]]><![CDATA[2]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("1", 3, 16),
			tCdata("2", 16, 29),
			tElementEnd(eeClose("", "p"), 29, 33),
		},
	},
	{
		name: "cdata_08",
		text: "<p> \n <![CDATA[data]]> \t </p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText(" \n ", 3, 6),
			tCdata("data", 6, 22),
			tText(" \t ", 22, 25),
			tElementEnd(eeClose("", "p"), 25, 29),
		},
	},
	{
		name: "cdata_09",
		text: "<p><![CDATA[bracket ]after]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tCdata("bracket ]after", 3, 29),
			tElementEnd(eeClose("", "p"), 29, 33),
		},
	},
	{
		name: "cdata_err_01",
		text: "<p><![CDATA[\x01]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tError("a non-XML character '\\u{1}' found at 1:13"),
		},
	},
	{
		name: "comment_01",
		text: "<!--comment-->",
		tokens: []testToken{
			tComment("comment", 0, 14),
		},
	},
	{
		name: "comment_02",
		text: "<!--<head>-->",
		tokens: []testToken{
			tComment("<head>", 0, 13),
		},
	},
	{
		name: "comment_03",
		text: "<!--<!-x-->",
		tokens: []testToken{
			tComment("<!-x", 0, 11),
		},
	},
	{
		name: "comment_04",
		text: "<!--<!x-->",
		tokens: []testToken{
			tComment("<!x", 0, 10),
		},
	},
	{
		name: "comment_05",
		text: "<!--<<!x-->",
		tokens: []testToken{
			tComment("<<!x", 0, 11),
		},
	},
	{
		name: "comment_06",
		text: "<!--<<!-x-->",
		tokens: []testToken{
			tComment("<<!-x", 0, 12),
		},
	},
	{
		name: "comment_07",
		text: "<!--<x-->",
		tokens: []testToken{
			tComment("<x", 0, 9),
		},
	},
	{
		name: "comment_08",
		text: "<!--<>-->",
		tokens: []testToken{
			tComment("<>", 0, 9),
		},
	},
	{
		name: "comment_09",
		text: "<!--<-->",
		tokens: []testToken{
			tComment("<", 0, 8),
		},
	},
	{
		name: "comment_10",
		text: "<!--<!-->",
		tokens: []testToken{
			tComment("<!", 0, 9),
		},
	},
	{
		name: "comment_11",
		text: "<!---->",
		tokens: []testToken{
			tComment("", 0, 7),
		},
	},
	{name: "comment_err_01", text: "<!----!>", isErr: true},
	{name: "comment_err_02", text: "<!----!", isErr: true},
	{name: "comment_err_03", text: "<!----", isErr: true},
	{name: "comment_err_04", text: "<!--->", isErr: true},
	{name: "comment_err_05", text: "<!-----", isErr: true},
	{name: "comment_err_06", text: "<!-->", isErr: true},
	{name: "comment_err_07", text: "<!--", isErr: true},
	{name: "comment_err_08", text: "<!--x", isErr: true},
	{name: "comment_err_09", text: "<!--<", isErr: true},
	{name: "comment_err_10", text: "<!--<!", isErr: true},
	{name: "comment_err_11", text: "<!--<!-", isErr: true},
	{name: "comment_err_12", text: "<!--<!--", isErr: true},
	{name: "comment_err_13", text: "<!--<!--!", isErr: true},
	{name: "comment_err_14", text: "<!--<!--!>", isErr: true},
	{name: "comment_err_15", text: "<!--<!---", isErr: true},
	{name: "comment_err_16", text: "<!--<!--x", isErr: true},
	{name: "comment_err_17", text: "<!--<!--x-", isErr: true},
	{name: "comment_err_18", text: "<!--<!--x--", isErr: true},
	{name: "comment_err_19", text: "<!--<!--x-->", isErr: true},
	{name: "comment_err_20", text: "<!--<!-x", isErr: true},
	{name: "comment_err_21", text: "<!--<!-x-", isErr: true},
	{name: "comment_err_22", text: "<!--<!-x--", isErr: true},
	{name: "comment_err_23", text: "<!--<!x", isErr: true},
	{name: "comment_err_24", text: "<!--<!x-", isErr: true},
	{name: "comment_err_25", text: "<!--<!x--", isErr: true},
	{name: "comment_err_26", text: "<!--<<!--x-->", isErr: true},
	{name: "comment_err_27", text: "<!--<!<!--x-->", isErr: true},
	{name: "comment_err_28", text: "<!--<!-<!--x-->", isErr: true},
	{name: "comment_err_29", text: "<!----!->", isErr: true},
	{name: "comment_err_30", text: "<!----!x>", isErr: true},
	{name: "comment_err_31", text: "<!-----x>", isErr: true},
	{name: "comment_err_32", text: "<!----->", isErr: true},
	{name: "comment_err_33", text: "<!------>", isErr: true},
	{name: "comment_err_34", text: "<!-- --->", isErr: true},
	{name: "comment_err_35", text: "<!--a--->", isErr: true},
	{name: "dtd_01", text: "<!DOCTYPE greeting SYSTEM \"hello.dtd\">"},
	{name: "dtd_02", text: "<!DOCTYPE greeting PUBLIC \"hello.dtd\" \"goodbye.dtd\">"},
	{name: "dtd_03", text: "<!DOCTYPE greeting SYSTEM 'hello.dtd'>"},
	{name: "dtd_04", text: "<!DOCTYPE greeting>"},
	{name: "dtd_05", text: "<!DOCTYPE greeting []>"},
	{
		name: "dtd_06",
		text: "<!DOCTYPE greeting><a/>",
		tokens: []testToken{
			tElementStart("", "a", 19),
			tElementEnd(eeEmpty, 21, 23),
		},
	},
	{name: "dtd_07", text: "<!DOCTYPE greeting [] >"},
	{name: "dtd_08", text: "<!DOCTYPE greeting [ ] >"},
	{
		name: "dtd_entity_01",
		text: "<!DOCTYPE svg [\n    <!ENTITY ns_extend \"http://ns.adobe.com/Extensibility/1.0/\">\n]>",
		tokens: []testToken{
			tEntityDecl("ns_extend", "http://ns.adobe.com/Extensibility/1.0/"),
		},
	},
	{
		name: "dtd_entity_02",
		text: "<!DOCTYPE svg [\n    <!ENTITY Pub-Status \"This is a pre-release of the\nspecification.\">\n]>",
		tokens: []testToken{
			tEntityDecl("Pub-Status", "This is a pre-release of the\nspecification."),
		},
	},
	{name: "dtd_entity_03", text: "<!DOCTYPE svg [\n    <!ENTITY open-hatch SYSTEM \"http://www.textuality.com/boilerplate/OpenHatch.xml\">\n]>"},
	{name: "dtd_entity_04", text: "<!DOCTYPE svg [\n    <!ENTITY open-hatch\n             PUBLIC \"-//Textuality//TEXT Standard open-hatch boilerplate//EN\"\n             \"http://www.textuality.com/boilerplate/OpenHatch.xml\">\n]>"},
	{name: "dtd_entity_05", text: "<!DOCTYPE svg [\n    <!ENTITY hatch-pic SYSTEM \"../grafix/OpenHatch.gif\" NDATA gif >\n]>"},
	{
		name: "dtd_entity_06",
		text: "<!DOCTYPE svg [\n    <!ELEMENT sgml ANY>\n    <!ENTITY ns_extend \"http://ns.adobe.com/Extensibility/1.0/\">\n    <!NOTATION example1SVG-rdf SYSTEM \"example1.svg.rdf\">\n    <!ATTLIST img data ENTITY #IMPLIED>\n]>",
		tokens: []testToken{
			tEntityDecl("ns_extend", "http://ns.adobe.com/Extensibility/1.0/"),
		},
	},
	{
		name: "dtd_err_01",
		text: "<!DOCTYPEEG[<!ENTITY%ETT\nSSSSSSSS<D_IDYT;->\n<",
		tokens: []testToken{
			tError("expected a whitespace not 'E' at 1:10"),
		},
	},
	{
		name: "dtd_err_02",
		text: "<!DOCTYPE s [<!ENTITY % name S YSTEM",
		tokens: []testToken{
			tError("invalid ExternalID at 1:30"),
		},
	},
	{
		name: "dtd_err_03",
		text: "<!DOCTYPE s [<!ENTITY % name B",
		tokens: []testToken{
			tError("expected a quote, SYSTEM or PUBLIC not 'B' at 1:30"),
		},
	},
	{
		name: "dtd_err_04",
		text: "<!DOCTYPE s []",
		tokens: []testToken{
			tError("unexpected end of stream"),
		},
	},
	{
		name: "dtd_err_05",
		text: "<!DOCTYPE s [] !",
		tokens: []testToken{
			tError("expected '>' not '!' at 1:16"),
		},
	},
	{name: "document_01", text: ""},
	{name: "document_02", text: "    "},
	{name: "document_03", text: " \n\t\r "},
	{
		name: "document_05",
		text: "\xEF\xBB\xBF<a/>",
		tokens: []testToken{
			tElementStart("", "a", 3),
			tElementEnd(eeEmpty, 5, 7),
		},
	},
	{name: "document_06", text: "\xEF\xBB\xBF<?xml version='1.0'?>"},
	{
		name: "document_07",
		text: "<?xml version='1.0' encoding='utf-8'?>\n<!-- comment -->\n<!DOCTYPE svg PUBLIC '-//W3C//DTD SVG 1.1//EN' 'http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd'>",
		tokens: []testToken{
			tComment(" comment ", 39, 55),
		},
	},
	{
		name: "document_08",
		text: "<?xml-stylesheet?>\n<!DOCTYPE svg PUBLIC '-//W3C//DTD SVG 1.1//EN' 'http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd'>",
		tokens: []testToken{
			tPI("xml-stylesheet", "", false, 0, 18),
		},
	},
	{
		name: "document_09",
		text: "<?xml version='1.0' encoding='utf-8'?>\n<?xml-stylesheet?>\n<!DOCTYPE svg PUBLIC '-//W3C//DTD SVG 1.1//EN' 'http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd'>",
		tokens: []testToken{
			tPI("xml-stylesheet", "", false, 39, 57),
		},
	},
	{
		name: "document_err_01",
		text: "<![CDATA[text]]>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "document_err_02",
		text: " &www---------Ӥ+----------w-----www_",
		tokens: []testToken{
			tError("unknown token at 1:2"),
		},
	},
	{
		name: "document_err_03",
		text: "q",
		tokens: []testToken{
			tError("unknown token at 1:1"),
		},
	},
	{
		name: "document_err_04",
		text: "<!>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "document_err_05",
		text: "<!DOCTYPE greeting1><!DOCTYPE greeting2>",
		tokens: []testToken{
			tError("invalid name token at 1:22"),
		},
	},
	{
		name: "document_err_06",
		text: "&#x20;",
		tokens: []testToken{
			tError("unknown token at 1:1"),
		},
	},
	{
		name: "element_01",
		text: "<a/>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tElementEnd(eeEmpty, 2, 4),
		},
	},
	{
		name: "element_02",
		text: "<a></a>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tElementEnd(eeOpen, 2, 3),
			tElementEnd(eeClose("", "a"), 3, 7),
		},
	},
	{
		name: "element_03",
		text: "  \t  <a/>   \n ",
		tokens: []testToken{
			tElementStart("", "a", 5),
			tElementEnd(eeEmpty, 7, 9),
		},
	},
	{
		name: "element_04",
		text: "  \t  <b><a/></b>   \n ",
		tokens: []testToken{
			tElementStart("", "b", 5),
			tElementEnd(eeOpen, 7, 8),
			tElementStart("", "a", 8),
			tElementEnd(eeEmpty, 10, 12),
			tElementEnd(eeClose("", "b"), 12, 16),
		},
	},
	{
		name: "element_06",
		text: "<俄语 լեզու=\"ռուսերեն\">данные</俄语>", //nolint:gosmopolitan // the text of the upstream test.
		tokens: []testToken{
			tElementStart("", "俄语", 0), //nolint:gosmopolitan // the text of the upstream test.
			tAttribute("", "լեզու", "ռուսերեն"),
			tElementEnd(eeOpen, 37, 38),
			tText("данные", 38, 50),
			tElementEnd(eeClose("", "俄语"), 50, 59), //nolint:gosmopolitan // the text of the upstream test.
		},
	},
	{
		name: "element_07",
		text: "<svg:circle></svg:circle>",
		tokens: []testToken{
			tElementStart("svg", "circle", 0),
			tElementEnd(eeOpen, 11, 12),
			tElementEnd(eeClose("svg", "circle"), 12, 25),
		},
	},
	{
		name: "element_08",
		text: "<:circle/>",
		tokens: []testToken{
			tElementStart("", "circle", 0),
			tElementEnd(eeEmpty, 8, 10),
		},
	},
	{
		name: "element_err_01",
		text: "<>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_02",
		text: "</",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_03",
		text: "</a",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_04",
		text: "<a x='test' /",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tAttribute("", "x", "test"),
			tError("unexpected end of stream"),
		},
	},
	{
		name: "element_err_05",
		text: "<<",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_06",
		text: "< a",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_07",
		text: "< ",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_08",
		text: "<&#x9;",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_09",
		text: "<a></a></a>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tElementEnd(eeOpen, 2, 3),
			tElementEnd(eeClose("", "a"), 3, 7),
			tError("unknown token at 1:8"),
		},
	},
	{
		name: "element_err_10",
		text: "<a/><a/>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tElementEnd(eeEmpty, 2, 4),
			tError("unknown token at 1:5"),
		},
	},
	{
		name: "element_err_11",
		text: "<a></br/></a>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tElementEnd(eeOpen, 2, 3),
			tError("expected '>' not '/' at 1:8"),
		},
	},
	{
		name: "element_err_12",
		text: "<svg:/>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_13",
		text: "<root>\n</root>\n</root>",
		tokens: []testToken{
			tElementStart("", "root", 0),
			tElementEnd(eeOpen, 5, 6),
			tText("\n", 6, 7),
			tElementEnd(eeClose("", "root"), 7, 14),
			tError("unknown token at 3:1"),
		},
	},
	{
		name: "element_err_14",
		text: "<-svg/>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_15",
		text: "<svg:-svg/>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_16",
		text: "<svg::svg/>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_17",
		text: "<svg:s:vg/>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_18",
		text: "<::svg/>",
		tokens: []testToken{
			tError("invalid name token at 1:2"),
		},
	},
	{
		name: "element_err_19",
		text: "<a><",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tElementEnd(eeOpen, 2, 3),
			tError("unknown token at 1:4"),
		},
	},
	{
		name: "attribute_01",
		text: "<a ax=\"test\"/>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tAttribute("", "ax", "test"),
			tElementEnd(eeEmpty, 12, 14),
		},
	},
	{
		name: "attribute_02",
		text: "<a ax='test'/>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tAttribute("", "ax", "test"),
			tElementEnd(eeEmpty, 12, 14),
		},
	},
	{
		name: "attribute_03",
		text: "<a b='test1' c=\"test2\"/>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tAttribute("", "b", "test1"),
			tAttribute("", "c", "test2"),
			tElementEnd(eeEmpty, 22, 24),
		},
	},
	{
		name: "attribute_04",
		text: "<a b='\"test1\"' c=\"'test2'\"/>",
		tokens: []testToken{
			tElementStart("", "a", 0),
			tAttribute("", "b", "\"test1\""),
			tAttribute("", "c", "'test2'"),
			tElementEnd(eeEmpty, 26, 28),
		},
	},
	{
		name: "attribute_05",
		text: "<c a=\"test1' c='test2\" b='test1\" c=\"test2'/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tAttribute("", "a", "test1' c='test2"),
			tAttribute("", "b", "test1\" c=\"test2"),
			tElementEnd(eeEmpty, 42, 44),
		},
	},
	{
		name: "attribute_06",
		text: "<c   a   =    'test1'     />",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tAttribute("", "a", "test1"),
			tElementEnd(eeEmpty, 26, 28),
		},
	},
	{
		name: "attribute_07",
		text: "<c q:a='b'/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tAttribute("q", "a", "b"),
			tElementEnd(eeEmpty, 10, 12),
		},
	},
	{
		name: "attribute_err_01",
		text: "<c az=test>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tError("expected a quote not 't' at 1:7"),
		},
	},
	{
		name: "attribute_err_02",
		text: "<c a>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tError("expected '=' not '>' at 1:5"),
		},
	},
	{
		name: "attribute_err_03",
		text: "<c a/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tError("expected '=' not '/' at 1:5"),
		},
	},
	{
		name: "attribute_err_04",
		text: "<c a='b' q/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tAttribute("", "a", "b"),
			tError("expected '=' not '/' at 1:11"),
		},
	},
	{
		name: "attribute_err_05",
		text: "<c a='<'/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tError("expected ''' not '<' at 1:7"),
		},
	},
	{
		name: "attribute_err_06",
		text: "<c a='\x01'/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tError("a non-XML character '\\u{1}' found at 1:7"),
		},
	},
	{
		name: "attribute_err_07",
		text: "<c a='v'b='v'/>",
		tokens: []testToken{
			tElementStart("", "c", 0),
			tAttribute("", "a", "v"),
			tError("expected a whitespace not 'b' at 1:9"),
		},
	},
	{
		name: "pi_01",
		text: "<?xslt ma?>",
		tokens: []testToken{
			tPI("xslt", "ma", true, 0, 11),
		},
	},
	{
		name: "pi_02",
		text: "<?xslt \t\n m?>",
		tokens: []testToken{
			tPI("xslt", "m", true, 0, 13),
		},
	},
	{
		name: "pi_03",
		text: "<?xslt?>",
		tokens: []testToken{
			tPI("xslt", "", false, 0, 8),
		},
	},
	{
		name: "pi_04",
		text: "<?xslt ?>",
		tokens: []testToken{
			tPI("xslt", "", false, 0, 9),
		},
	},
	{
		name: "pi_05",
		text: "<?xml-stylesheet?>",
		tokens: []testToken{
			tPI("xml-stylesheet", "", false, 0, 18),
		},
	},
	{
		name: "pi_err_01",
		text: "<??xml \t\n m?>",
		tokens: []testToken{
			tError("invalid name token at 1:3"),
		},
	},
	{name: "declaration_01", text: "<?xml version=\"1.0\"?>"},
	{name: "declaration_02", text: "<?xml version='1.0'?>"},
	{name: "declaration_03", text: "<?xml version='1.0' encoding=\"UTF-8\"?>"},
	{name: "declaration_04", text: "<?xml version='1.0' encoding='UTF-8'?>"},
	{name: "declaration_05", text: "<?xml version='1.0' encoding='utf-8'?>"},
	{name: "declaration_06", text: "<?xml version='1.0' encoding='EUC-JP'?>"},
	{name: "declaration_07", text: "<?xml version='1.0' encoding='UTF-8' standalone='yes'?>"},
	{name: "declaration_08", text: "<?xml version='1.0' encoding='UTF-8' standalone='no'?>"},
	{name: "declaration_09", text: "<?xml version='1.0' standalone='no'?>"},
	{name: "declaration_10", text: "<?xml version='1.0' standalone='no' ?>"},
	{
		name: "declaration_err_01",
		text: "<?xml encoding='UTF-8' version='1.0'?>",
		tokens: []testToken{
			tError("expected 'version' at 1:7"),
		},
	},
	{
		name: "declaration_err_05",
		text: "<?xml version='1.0' yes='true'?>",
		tokens: []testToken{
			tError("expected '?>' at 1:21"),
		},
	},
	{
		name: "declaration_err_06",
		text: "<?xml version='1.0' encoding='UTF-8' standalone='yes' yes='true'?>",
		tokens: []testToken{
			tError("expected '?>' at 1:55"),
		},
	},
	{
		name: "declaration_err_07",
		text: "\n<?xml\n&jg'];",
		tokens: []testToken{
			tError("expected '?>' at 3:7"),
		},
	},
	{
		name: "declaration_err_08",
		text: "<?xml \t\n ?m?>",
		tokens: []testToken{
			tError("expected 'version' at 2:2"),
		},
	},
	{
		name: "declaration_err_09",
		text: "<?xml \t\n m?>",
		tokens: []testToken{
			tError("expected 'version' at 2:2"),
		},
	},
	{
		name: "declaration_err_10",
		text: " <?xml version='1.0'?>",
		tokens: []testToken{
			tError("unexpected XML declaration at 1:2"),
		},
	},
	{
		name: "declaration_err_11",
		text: "<!-- comment --><?xml version='1.0'?>",
		tokens: []testToken{
			tComment(" comment ", 0, 16),
			tError("unexpected XML declaration at 1:17"),
		},
	},
	{
		name: "declaration_err_12",
		text: "<?xml version='1.0'?><?xml version='1.0'?>",
		tokens: []testToken{
			tError("unexpected XML declaration at 1:22"),
		},
	},
	{
		name: "declaration_err_13",
		text: "<?target \x01content>",
		tokens: []testToken{
			tError("a non-XML character '\\u{1}' found at 1:10"),
		},
	},
	{
		name: "declaration_err_14",
		text: "<?xml version='1.0'encoding='UTF-8'?>",
		tokens: []testToken{
			tError("expected a whitespace not 'e' at 1:20"),
		},
	},
	{
		name: "declaration_err_15",
		text: "<?xml version='1.0' encoding='UTF-8'standalone='yes'?>",
		tokens: []testToken{
			tError("expected a whitespace not 's' at 1:37"),
		},
	},
	{
		name: "declaration_err_16",
		text: "<?xml version='1.0'",
		tokens: []testToken{
			tError("expected '?>' at 1:20"),
		},
	},
	{
		name: "text_01",
		text: "<p>text</p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText("text", 3, 7),
			tElementEnd(eeClose("", "p"), 7, 11),
		},
	},
	{
		name: "text_02",
		text: "<p> text </p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText(" text ", 3, 9),
			tElementEnd(eeClose("", "p"), 9, 13),
		},
	},
	{
		name: "text_03",
		text: "<p>欄</p>", //nolint:gosmopolitan // the text of the upstream test.
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText("欄", 3, 6), //nolint:gosmopolitan // the text of the upstream test.
			tElementEnd(eeClose("", "p"), 6, 10),
		},
	},
	{
		name: "text_04",
		text: "<p> </p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText(" ", 3, 4),
			tElementEnd(eeClose("", "p"), 4, 8),
		},
	},
	{
		name: "text_05",
		text: "<p> \r\n\t </p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText(" \r\n\t ", 3, 8),
			tElementEnd(eeClose("", "p"), 8, 12),
		},
	},
	{
		name: "text_06",
		text: "<p>&#x20;</p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText("&#x20;", 3, 9),
			tElementEnd(eeClose("", "p"), 9, 13),
		},
	},
	{
		name: "text_07",
		text: "<p>]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tText("]>", 3, 5),
			tElementEnd(eeClose("", "p"), 5, 9),
		},
	},
	{
		name: "text_err_01",
		text: "<p>]]></p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tError("']]>' at 1:7 is not allowed inside a character data"),
		},
	},
	{
		name: "text_err_02",
		text: "<p>\x0c</p>",
		tokens: []testToken{
			tElementStart("", "p", 0),
			tElementEnd(eeOpen, 2, 3),
			tError("a non-XML character '\\u{c}' found at 1:4"),
		},
	},
}
