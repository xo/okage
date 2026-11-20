// Copyright (c) the go-tex/math authors.
// SPDX-License-Identifier: BSD-3-Clause

package texmath

import (
	"errors"
	"testing"

	"github.com/xo/okage"
)

func TestControlWordsKeepTheirAt(t *testing.T) {
	for _, c := range []struct {
		src  string
		want []string
	}{
		{`\the@inst`, []string{"the@inst"}},
		{`\@ifundefined`, []string{"@ifundefined"}},
		{`\@@affmark`, []string{"@@affmark"}},
		{`\@ x`, []string{"@"}},
		{`\alpha`, []string{"alpha"}},
		{`\{`, []string{"{"}},
	} {
		var got []string
		for _, tk := range tokenize(c.src) {
			if tk.kind == tCtrl {
				got = append(got, tk.text)
			}
		}
		if len(got) != len(c.want) || (len(got) > 0 && got[0] != c.want[0]) {
			t.Errorf("tokenize(%q) = %q, want %q", c.src, got, c.want)
		}
	}
}

func TestUnknownControlWordReportsItsWholeName(t *testing.T) {
	_, err := Parse(`x^{\the@inst }`)
	if err == nil {
		t.Fatal("no error for an unknown command")
	}
	if got := err.Error(); got != `parsing tex math: unknown command \the@inst` {
		t.Errorf("error %q, want the whole command", got)
	}
	var se *SyntaxError
	if !errors.As(err, &se) || se.Command != `\the@inst` || se.Pos != 3 {
		t.Errorf("error %#v, want Command \\the@inst at byte 3", err)
	}
	if !errors.Is(err, okage.ErrTeXMathUnknownCommand) || !errors.Is(err, okage.ErrTeXMathSyntax) {
		t.Errorf("error %v does not wrap both sentinel errors", err)
	}
	_, err = Parse(`\begin{nope}a\end{nope}`)
	if !errors.As(err, &se) || se.Command != `\begin{nope}` || !errors.Is(err, okage.ErrTeXMathUnknownCommand) {
		t.Errorf("an unknown environment gives %#v", err)
	}
}
