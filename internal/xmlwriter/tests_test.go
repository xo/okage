// Copyright (c) 2019 Reizner Evgeniy
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmlwriter

import "testing"

// textEq is the macro text_eq! of xmlwriter tests/tests.rs.
func textEq(t *testing.T, result, expected string) {
	t.Helper()
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

// shouldPanic checks that f panics, as #[should_panic] does in Rust.
func shouldPanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	f()
}

// TestWriteElement01 ports the test write_element_01 of xmlwriter
// tests/tests.rs.
func TestWriteElement01(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg/>\n")
}

// TestWriteElement02 ports the test write_element_02 of xmlwriter
// tests/tests.rs.
func TestWriteElement02(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.StartElement("rect")
	w.EndElement()
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg>\n    <rect/>\n</svg>\n")
}

// TestWriteElement03 ports the test write_element_03 of xmlwriter
// tests/tests.rs.
func TestWriteElement03(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.EndElement()
	w.EndElement() // Should not panic.
	textEq(t, w.EndDocument(), "<svg/>\n")
}

// TestWriteElement05 ports the test write_element_05 of xmlwriter
// tests/tests.rs.
func TestWriteElement05(t *testing.T) {
	w := New()
	w.StartElement("svg")
	// end_document() will call `close_element` automatically.
	textEq(t, w.EndDocument(), "<svg/>\n")
}

// TestWriteElement06 ports the test write_element_06 of xmlwriter
// tests/tests.rs.
func TestWriteElement06(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.StartElement("rect")
	w.StartElement("rect")
	w.StartElement("rect")
	w.StartElement("rect")
	w.StartElement("rect")
	textEq(t, w.EndDocument(), "<svg>\n    <rect>\n        <rect>\n            <rect>\n                <rect>\n                    <rect/>\n                </rect>\n            </rect>\n        </rect>\n    </rect>\n</svg>\n")
}

// TestWriteAttribute01 ports the test write_attribute_01 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute01(t *testing.T) {
	shouldPanic(t, func() {
		w := New()
		// must be used only after write_element
		w.WriteAttribute("id", "q")
	})
}

// TestWriteAttribute02 ports the test write_attribute_02 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute02(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.WriteAttribute("id", "q")
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg id=\"q\"/>\n")
}

// TestWriteAttribute03 ports the test write_attribute_03 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute03(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.WriteAttribute("id", "\"")
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg id=\"&quot;\"/>\n")
}

// TestWriteAttribute04 ports the test write_attribute_04 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute04(t *testing.T) {
	w := New(WithUseSingleQuote(true))
	w.StartElement("svg")
	w.WriteAttribute("id", "'")
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg id='&apos;'/>\n")
}

// TestWriteAttribute05 ports the test write_attribute_05 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute05(t *testing.T) {
	w := New(WithUseSingleQuote(true))
	w.StartElement("svg")
	w.WriteAttribute("id", "'''''")
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg id='&apos;&apos;&apos;&apos;&apos;'/>\n")
}

// TestWriteAttribute06 ports the test write_attribute_06 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute06(t *testing.T) {
	w := New(WithUseSingleQuote(true))
	w.StartElement("svg")
	w.WriteAttribute("id", "'text'")
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg id='&apos;text&apos;'/>\n")
}

// TestWriteAttribute07 ports the test write_attribute_07 of xmlwriter
// tests/tests.rs.
func TestWriteAttribute07(t *testing.T) {
	w := New()
	w.StartElement("svg")
	// TODO: looks we need specialization to remove &
	w.WriteAttribute("x", 5)
	w.EndElement()
	textEq(t, w.EndDocument(), "<svg x=\"5\"/>\n")
}

// TestWriteDeclaration01 ports the test write_declaration_01 of xmlwriter
// tests/tests.rs.
func TestWriteDeclaration01(t *testing.T) {
	w := New()
	w.WriteDeclaration()
	textEq(t, w.EndDocument(), "<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"no\"?>\n")
}

// TestWriteDeclaration02 ports the test write_declaration_02 of xmlwriter
// tests/tests.rs.
func TestWriteDeclaration02(t *testing.T) {
	shouldPanic(t, func() {
		w := New()
		w.WriteDeclaration()
		w.WriteDeclaration() // declaration must be written once
	})
}

// TestWriteDeclaration03 ports the test write_declaration_03 of xmlwriter
// tests/tests.rs.
func TestWriteDeclaration03(t *testing.T) {
	shouldPanic(t, func() {
		w := New()
		w.WriteComment("test")
		w.WriteDeclaration() // declaration must be written first
	})
}

// TestWriteSingleQuote01 ports the test write_single_quote_01 of xmlwriter
// tests/tests.rs.
func TestWriteSingleQuote01(t *testing.T) {
	w := New(WithUseSingleQuote(true))
	w.WriteDeclaration()
	textEq(t, w.EndDocument(), "<?xml version='1.0' encoding='UTF-8' standalone='no'?>\n")
}

// TestWriteSingleQuote02 ports the test write_single_quote_02 of xmlwriter
// tests/tests.rs.
func TestWriteSingleQuote02(t *testing.T) {
	w := New(WithUseSingleQuote(true))
	w.StartElement("p")
	w.WriteAttribute("a", "b")
	w.EndElement()
	textEq(t, w.EndDocument(), "<p a='b'/>\n")
}

// TestWriteComment01 ports the test write_comment_01 of xmlwriter
// tests/tests.rs.
func TestWriteComment01(t *testing.T) {
	w := New()
	w.WriteComment("test")
	w.StartElement("svg")
	textEq(t, w.EndDocument(), "<!--test-->\n<svg/>\n")
}

// TestWriteComment02 ports the test write_comment_02 of xmlwriter
// tests/tests.rs.
func TestWriteComment02(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.WriteComment("test")
	textEq(t, w.EndDocument(), "<svg>\n    <!--test-->\n</svg>\n")
}

// TestWriteComment03 ports the test write_comment_03 of xmlwriter
// tests/tests.rs.
func TestWriteComment03(t *testing.T) {
	w := New()
	w.WriteComment("test")
	w.StartElement("svg")
	w.WriteComment("test")
	textEq(t, w.EndDocument(), "<!--test-->\n<svg>\n    <!--test-->\n</svg>\n")
}

// TestWriteComment04 ports the test write_comment_04 of xmlwriter
// tests/tests.rs.
func TestWriteComment04(t *testing.T) {
	w := New()
	w.WriteComment("test")
	w.StartElement("svg")
	w.StartElement("rect")
	w.WriteComment("test")
	textEq(t, w.EndDocument(), "<!--test-->\n<svg>\n    <rect>\n        <!--test-->\n    </rect>\n</svg>\n")
}

// TestWriteComment05 ports the test write_comment_05 of xmlwriter
// tests/tests.rs.
func TestWriteComment05(t *testing.T) {
	w := New()
	w.WriteComment("test")
	w.StartElement("svg")
	w.WriteComment("test")
	w.StartElement("rect")
	w.EndElement()
	textEq(t, w.EndDocument(), "<!--test-->\n<svg>\n    <!--test-->\n    <rect/>\n</svg>\n")
}

// TestWriteComment06 ports the test write_comment_06 of xmlwriter
// tests/tests.rs.
func TestWriteComment06(t *testing.T) {
	w := New()
	w.WriteComment("test")
	w.StartElement("svg")
	w.StartElement("rect")
	w.EndElement()
	w.WriteComment("test")
	textEq(t, w.EndDocument(), "<!--test-->\n<svg>\n    <rect/>\n    <!--test-->\n</svg>\n")
}

// TestWriteComment07 ports the test write_comment_07 of xmlwriter
// tests/tests.rs.
func TestWriteComment07(t *testing.T) {
	w := New()
	w.StartElement("svg")
	w.EndElement()
	w.WriteComment("test")
	textEq(t, w.EndDocument(), "<svg/>\n<!--test-->\n")
}

// TestWriteComment08 ports the test write_comment_08 of xmlwriter
// tests/tests.rs.
func TestWriteComment08(t *testing.T) {
	w := New()
	w.WriteComment("test")
	w.WriteComment("test")
	w.WriteComment("test")
	textEq(t, w.EndDocument(), "<!--test-->\n<!--test-->\n<!--test-->\n")
}

// TestWriteText01 ports the test write_text_01 of xmlwriter
// tests/tests.rs.
func TestWriteText01(t *testing.T) {
	shouldPanic(t, func() {
		w := New()
		w.WriteText("text") // Should be called after start_element()
	})
}

// TestWriteText02 ports the test write_text_02 of xmlwriter
// tests/tests.rs.
func TestWriteText02(t *testing.T) {
	shouldPanic(t, func() {
		w := New()
		w.WriteText("text") // Should be called after start_element()
	})
}

// TestWriteText03 ports the test write_text_03 of xmlwriter
// tests/tests.rs.
func TestWriteText03(t *testing.T) {
	shouldPanic(t, func() {
		w := New()
		w.StartElement("p")
		w.EndElement()
		w.WriteText("text") // Should be called after start_element()
	})
}

// TestWriteText04 ports the test write_text_04 of xmlwriter
// tests/tests.rs.
func TestWriteText04(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("text")
	w.WriteText("text")
	textEq(t, w.EndDocument(), "<p>\n    text\n    text\n</p>\n")
}

// TestWriteText05 ports the test write_text_05 of xmlwriter
// tests/tests.rs.
func TestWriteText05(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("text")
	textEq(t, w.EndDocument(), "<p>\n    text\n</p>\n")
}

// TestWriteText06 ports the test write_text_06 of xmlwriter
// tests/tests.rs.
func TestWriteText06(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("text")
	w.StartElement("p")
	w.WriteText("text")
	textEq(t, w.EndDocument(), "<p>\n    text\n    <p>\n        text\n    </p>\n</p>\n")
}

// TestWriteText07 ports the test write_text_07 of xmlwriter
// tests/tests.rs.
func TestWriteText07(t *testing.T) {
	w := New()
	w.StartElement("div")
	w.StartElement("p")
	w.WriteText("text")
	w.StartElement("p")
	w.WriteText("text")
	textEq(t, w.EndDocument(), "<div>\n    <p>\n        text\n        <p>\n            text\n        </p>\n    </p>\n</div>\n")
}

// TestWriteText08 ports the test write_text_08 of xmlwriter
// tests/tests.rs.
func TestWriteText08(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("<")
	textEq(t, w.EndDocument(), "<p>\n    &lt;\n</p>\n")
}

// TestWriteText09 ports the test write_text_09 of xmlwriter
// tests/tests.rs.
func TestWriteText09(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("<&>")
	textEq(t, w.EndDocument(), "<p>\n    &lt;&>\n</p>\n")
}

// TestWriteText10 ports the test write_text_10 of xmlwriter
// tests/tests.rs.
func TestWriteText10(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("&lt;")
	textEq(t, w.EndDocument(), "<p>\n    &lt;\n</p>\n")
}

// TestWriteText11 ports the test write_text_11 of xmlwriter
// tests/tests.rs.
func TestWriteText11(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.WriteText("text")
	w.StartElement("p")
	w.EndElement()
	w.WriteText("text")
	textEq(t, w.EndDocument(), "<p>\n    text\n    <p/>\n    text\n</p>\n")
}

// TestWritePreserveText01 ports the test write_preserve_text_01 of xmlwriter
// tests/tests.rs.
func TestWritePreserveText01(t *testing.T) {
	w := New()
	w.SetPreserveWhitespaces(true)
	w.StartElement("p")
	w.WriteText("text")
	w.StartElement("p")
	w.EndElement()
	w.WriteText("text")
	textEq(t, w.EndDocument(), "<p>text<p/>text</p>")
}

// TestWritePreserveText02 ports the test write_preserve_text_02 of xmlwriter
// tests/tests.rs.
func TestWritePreserveText02(t *testing.T) {
	w := New()
	w.StartElement("p")
	w.StartElement("p")
	w.SetPreserveWhitespaces(true)
	w.WriteText("text")
	w.StartElement("p")
	w.EndElement()
	w.WriteText("text")
	w.EndElement()
	w.SetPreserveWhitespaces(false)
	textEq(t, w.EndDocument(), "<p>\n    <p>text<p/>text</p>\n</p>\n")
}

// TestAttrsIndent01 ports the test attrs_indent_01 of xmlwriter
// tests/tests.rs.
func TestAttrsIndent01(t *testing.T) {
	w := New(WithAttributesIndent(Indent{Kind: IndentSpaces, Spaces: 2}))
	w.StartElement("rect")
	w.WriteAttribute("x", "5")
	w.StartElement("rect")
	w.WriteAttribute("x", "10")
	w.WriteAttribute("y", "15")
	textEq(t, w.EndDocument(), "<rect\n  x=\"5\">\n    <rect\n      x=\"10\"\n      y=\"15\"/>\n</rect>\n")
}
