// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package roxmltree

// This file ports roxmltree tests/ast.rs. Each test! of upstream is a name
// in astTests, and a subtest of TestAST with that name. to_yaml is ToYAML in
// ast.go.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// actualTest is actual_test in roxmltree tests/ast.rs.
func actualTest(t *testing.T, path string) {
	t.Helper()
	expected := loadFile(t, strings.TrimSuffix(path, filepath.Ext(path))+".yaml")

	opt := DefaultParsingOptions()
	opt.AllowDTD = true

	inputXML := loadFile(t, path)
	doc, err := ParseWithOptions(inputXML, opt)
	if err != nil {
		var e *Error
		if !errors.As(err, &e) {
			t.Fatalf("error %v is not a *Error", err)
		}
		if got, want := "error: \""+e.Message()+"\"", strings.TrimSpace(expected); got != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
		return
	}

	if got := ToYAML(doc); got != expected {
		t.Errorf("got:\n%s\nwant:\n%s", got, expected)
	}
}

// loadFile is load_file in roxmltree tests/ast.rs.
func loadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestAST runs each test! of roxmltree tests/ast.rs as a subtest.
func TestAST(t *testing.T) {
	for _, name := range astTests {
		t.Run(name, func(t *testing.T) {
			actualTest(t, filepath.Join("testdata", "files", name+".xml"))
		})
	}
}

// astTests are the names of the test! of roxmltree tests/ast.rs, in its
// order. Upstream leaves out tree_003, as "unsupported".
var astTests = []string{
	"attrs_001",
	"attrs_002",
	"attrs_003",
	"attrs_004",
	"attrs_005",
	"attrs_006",
	"attrs_err_001",
	"attrs_err_002",
	"cdata_001",
	"cdata_002",
	"cdata_003",
	"cdata_004",
	"cdata_005",
	"cdata_006",
	"cdata_007",
	"cdata_008",
	"cdata_009",
	"comments_001",
	"elems_err_001",
	"elems_err_002",
	"entity_001",
	"entity_002",
	"entity_003",
	"entity_004",
	"entity_005",
	"entity_006",
	"entity_007",
	"entity_008",
	"entity_009",
	"entity_010",
	"entity_011",
	"entity_012",
	"entity_013",
	"entity_014",
	"entity_err_001",
	"entity_err_002",
	"entity_err_003",
	"entity_err_004",
	"entity_err_005",
	"entity_err_006",
	"entity_err_007",
	"entity_err_008",
	"entity_err_009",
	"entity_err_010",
	"ns_001",
	"ns_002",
	"ns_003",
	"ns_004",
	"ns_005",
	"ns_006",
	"ns_007",
	"ns_008",
	"ns_009",
	"ns_010",
	"ns_011",
	"ns_012",
	"ns_013",
	"ns_014",
	"ns_015",
	"ns_016",
	"ns_017",
	"ns_err_001",
	"ns_err_002",
	"ns_err_003",
	"ns_err_004",
	"ns_err_005",
	"ns_err_006",
	"ns_err_007",
	"ns_err_008",
	"ns_err_009",
	"ns_err_010",
	"ns_err_011",
	"ns_err_012",
	"ns_err_013",
	"text_001",
	"text_002",
	"text_003",
	"text_004",
	"text_005",
	"text_006",
	"text_007",
	"text_008",
	"text_009",
	"text_010",
	"text_011",
	"text_012",
	"tree_001",
	"tree_002",
	"tree_err_001",
	"tree_err_002",
	"tree_err_003",
}
