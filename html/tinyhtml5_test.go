// Copyright (c) 2006-2013 James Graham and other contributors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package html

// This file ports tinyhtml5 tests/__init__.py (D49): the reader of the files
// of html5lib-tests in testdata/tinyhtml5.

import (
	"bytes"
	"os"
	"testing"
)

// testData is one test of a file of html5lib-tests: the text of each
// heading. A heading that the test does not have is missing from the map.
type testData map[string]string

// readTestData is Data.__iter__ in tinyhtml5 tests/__init__.py. It reads the
// tests of a file, and a heading named newTestHeading starts a test.
func readTestData(t *testing.T, filename, newTestHeading string) []testData {
	t.Helper()
	src, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("reading %s: %v", filename, err)
	}
	var tests []testData
	data := testData{}
	key := ""
	hasKey := false
	lines := bytes.SplitAfter(src, []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		// Remove trailing newline.
		line = line[:len(line)-1]
		if bytes.HasPrefix(line, []byte("#")) {
			heading := string(bytes.TrimSpace(line[1:]))
			if len(data) > 0 && heading == newTestHeading {
				// Remove empty line.
				if v := data[key]; v != "" {
					data[key] = v[:len(v)-1]
				}
				tests = append(tests, data)
				data = testData{}
			}
			key, hasKey = heading, true
			data[key] = ""
		} else if hasKey {
			if data[key] != "" {
				data[key] += "\n"
			}
			data[key] += string(line)
		}
	}
	if len(data) > 0 {
		tests = append(tests, data)
	}
	return tests
}
