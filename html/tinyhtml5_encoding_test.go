// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 tests/test_encoding.py (D49). It runs the tests
// of html5lib-tests in testdata/tinyhtml5/encoding. html decodes few
// encodings itself, so the parser test gives a CharsetReader that maps each
// byte to the character of the same number, as Latin-1 does.

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// encodingTests returns the tests of the files in
// testdata/tinyhtml5/encoding, each with its id.
func encodingTests(t *testing.T) ([]string, []testData) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "tinyhtml5", "encoding", "*.dat"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("finding the tests: %v", err)
	}
	var ids []string
	var tests []testData
	for _, path := range paths {
		stem := strings.TrimSuffix(filepath.Base(path), ".dat")
		for i, test := range readTestData(t, path, "data") {
			ids = append(ids, fmt.Sprintf("%s-%d", stem, i))
			tests = append(tests, test)
		}
	}
	return ids, tests
}

// latin1Reader is a CharsetReader that maps each byte to the character of
// the same number.
func latin1Reader(_ string, input io.Reader) (io.Reader, error) {
	b, err := io.ReadAll(input)
	if err != nil {
		return nil, err
	}
	var s strings.Builder
	for _, c := range b {
		s.WriteRune(rune(c))
	}
	return strings.NewReader(s.String()), nil
}

// TestParserEncoding is test_parser_encoding in tinyhtml5
// tests/test_encoding.py.
func TestParserEncoding(t *testing.T) {
	ids, tests := encodingTests(t)
	for i, test := range tests {
		t.Run(ids[i], func(t *testing.T) {
			doc, err := ParseDocument(bytes.NewReader([]byte(test["data"])), WithCharsetReader(latin1Reader))
			if err != nil {
				t.Fatal(err)
			}
			encoding := strings.ToLower(test["encoding"])
			if doc.Encoding() != encoding {
				t.Errorf("\nData: %q\nExpected encoding: %s\nParser encoding: %s", test["data"], encoding, doc.Encoding())
			}
		})
	}
}

// TestPrescanEncoding is test_prescan_encoding in tinyhtml5
// tests/test_encoding.py.
func TestPrescanEncoding(t *testing.T) {
	ids, tests := encodingTests(t)
	for i, test := range tests {
		t.Run(ids[i], func(t *testing.T) {
			stream := newBinaryInputStream([]byte(test["data"]), newDocumentOptions(nil))

			// Very crude way to ignore irrelevant tests.
			if len(test["data"]) > stream.numberBytesMeta {
				return
			}

			encoding := strings.ToLower(test["encoding"])
			if stream.encoding != encoding {
				t.Errorf("\nData: %q\nExpected encoding: %s\nParser encoding: %s", test["data"], encoding, stream.encoding)
			}
		})
	}
}
