// Copyright 2013-2020 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

package tinycss2

import (
	"strings"
	"testing"
)

// This file ports no upstream file. It holds the fuzz test of the tokenizer
// and of the parsers (D50).

// FuzzParseStylesheet checks that no input makes the tokenizer, the parsers,
// the serializer or the color and An+B parsers panic or hang.
func FuzzParseStylesheet(f *testing.F) {
	for _, seed := range []string{
		"", "a{b:c}", "@media print { a { color: red !important } }",
		`url(a"b) "x\` + "\n" + `/* c`, "f(g([{a}]))", "u+1?-2 #abc 1e3px 50%",
		"rgb(1 2 3 / 50%) hsl(1turn, 2%, 3%) light-dark(red, blue) color(--a 1)",
		"device-cmyk(0 0 0 1) lab(50% 40 59) oklch(0.5 0.1 none)",
		"2n+1 -n- odd", strings.Repeat("(", 300) + "x",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<16 {
			return
		}
		css := string(data)
		rules := ParseStylesheet(css)
		_ = Serialize(rules)
		for _, rule := range rules {
			if r, ok := rule.(*QualifiedRule); ok {
				_ = Serialize(ParseBlocksContents(r.Content))
			}
			if r, ok := rule.(*AtRule); ok && r.Content != nil {
				_ = Serialize(ParseRuleList(r.Content))
			}
		}
		_ = ParseDeclarationList(css, WithSkipComments(true))
		_ = ParseOneRule(css)
		_ = ParseOneDeclaration(css)
		tokens := ParseComponentValueList(css, WithMaxDepth(8))
		_ = Serialize(tokens)
		for _, token := range tokens {
			_, _ = ParseColor3(token)
			if c, _ := ParseColor4(token); c != nil {
				for _, space := range []string{"srgb", "xyz-d50", "xyz-d65", "lab"} {
					_, _ = c.To(space)
				}
				_ = c.String()
			}
			_, _ = ParseColor5(token, "dark")
		}
		_, _, _ = ParseNth(css)
		_, _ = ParseStylesheetBytes(data, "", nil)
	})
}
