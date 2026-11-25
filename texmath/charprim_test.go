// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import "testing"

func TestCharPrimitive(t *testing.T) {
	for _, c := range []struct {
		tex  string
		want rune
	}{
		{"\\char`\\^", '^'},
		{"\\char`A", 'A'},
		{`\char"ff`, 'ÿ'},
		{`\char98`, 'b'},
	} {
		l := mustParse(t, c.tex)
		if a, ok := l[0].(Atom); !ok || a.Text != string(c.want) || a.Variant != Upright {
			t.Errorf("Parse(%q) = %#v, want the upright %q", c.tex, l, c.want)
		}
	}
	for _, tex := range []string{"a + \\char`\\^ + b", `z = \char98 + 1`} {
		mustParse(t, tex)
	}
}

// TestCharPrimitiveSpecialCharacter checks that \char`^ is the character ^,
// as KaTeX reads it. The source refused it (D95).
func TestCharPrimitiveSpecialCharacter(t *testing.T) {
	for tex, want := range map[string]string{"\\char`^": "^", "\\char`_": "_", "\\char`&": "&"} {
		l := mustParse(t, tex)
		if a, ok := l[0].(Atom); !ok || a.Text != want {
			t.Errorf("%q = %#v, want the character %q", tex, l[0], want)
		}
	}
}

func TestCharPrimitiveErrors(t *testing.T) {
	for name, tex := range map[string]string{
		"no number":        `\char`,
		"non-digit":        `\char x`,
		"backtick alone":   "\\char`",
		"backtick bare cs": "\\char`\\",
		"out of range":     `\char"200000`,
		"overflows int32":  `\char"FFFFFFFF`,
	} {
		if _, err := Parse(tex); err == nil {
			t.Errorf("%q (%s): want an error, got none", tex, name)
		}
	}
}
