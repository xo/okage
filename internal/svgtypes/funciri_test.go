// Copyright 2021 the SVG Types Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svgtypes

import "testing"

// The tests of svgtypes src/funciri.rs.

func TestParseIRI1(t *testing.T) {
	if got, err := ParseIRI("#id"); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseIRI2(t *testing.T) {
	if got, err := ParseIRI("   #id   "); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseIRI3(t *testing.T) {
	// Trailing data is ok for the Stream, by not for IRI.
	s := NewStream("   #id   text")
	if got, err := s.ParseIRI(); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
	_, err := ParseIRI("   #id   text")
	errText(t, err, "unexpected data at position 10")
}

func TestParseIRI4(t *testing.T) {
	if got, err := ParseIRI("#1"); err != nil || got != "1" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseErrIRI1(t *testing.T) {
	_, err := ParseIRI("# id")
	errText(t, err, "invalid value")
}

func TestParseFuncIRI1(t *testing.T) {
	if got, err := ParseFuncIRI("url(#id)"); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseFuncIRI2(t *testing.T) {
	if got, err := ParseFuncIRI("url(#1)"); err != nil || got != "1" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseFuncIRI3(t *testing.T) {
	if got, err := ParseFuncIRI("    url(    #id    )   "); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseFuncIRI4(t *testing.T) {
	// Trailing data is ok for the Stream, by not for FuncIRI.
	s := NewStream("url(#id) qwe")
	if got, err := s.ParseFuncIRI(); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
	_, err := ParseFuncIRI("url(#id) qwe")
	errText(t, err, "unexpected data at position 10")
}

func TestParseFuncIRI5(t *testing.T) {
	if got, err := ParseFuncIRI("url('#id')"); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := ParseFuncIRI("url(' #id ')"); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseFuncIRI6(t *testing.T) {
	if got, err := ParseFuncIRI("url(\"#id\")"); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := ParseFuncIRI("url(\" #id \")"); err != nil || got != "id" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseErrFuncIRI1(t *testing.T) {
	_, err := ParseFuncIRI("url ( #1 )")
	errText(t, err, "expected 'url(' not 'url ' at position 1")
}

func TestParseErrFuncIRI2(t *testing.T) {
	_, err := ParseFuncIRI("url(#)")
	errText(t, err, "invalid value")
}

func TestParseErrFuncIRI3(t *testing.T) {
	_, err := ParseFuncIRI("url(# id)")
	errText(t, err, "invalid value")
}

func TestParseErrFuncIRI4(t *testing.T) {
	// If single quotes are present around the ID, they should be on both sides
	_, err := ParseFuncIRI("url('#id)")
	errText(t, err, "unexpected end of stream")
	_, err = ParseFuncIRI("url(#id')")
	errText(t, err, "invalid value")
}
