// Copyright 2019 the SimpleCSS Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package simplecss

import (
	"context"
	"log/slog"
	"strings"
	"testing"
)

// recordHandler is a slog.Handler that writes each record as
// "<LEVEL>: <message>\n", as the logger of the example parse of upstream
// does.
type recordHandler struct {
	b *strings.Builder
}

func (h recordHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.b.WriteString(r.Level.String() + ": " + r.Message + "\n")
	return nil
}

func (h recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h recordHandler) WithGroup(string) slog.Handler { return h }

// runProcess is run_process in simplecss tests/warnings.rs. Upstream runs
// the example parse and reads its standard error. The port parses input with
// a logger that records the warnings, and returns them.
func runProcess(input string) string {
	var b strings.Builder
	_ = ParseStyleSheet(input, WithLogger(slog.New(recordHandler{&b})))
	return b.String()
}

// TestWarnings ports the tests of simplecss tests/warnings.rs. Each subtest
// has the name of the upstream test.
func TestWarnings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"style_01", "> {}", "WARN: Selector parsing failed cause unexpected combinator.\n"},
		{"style_02", "@import 'subs.css';", "WARN: The @import rule is not supported. Skipped.\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := runProcess(test.input); got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

// TestWarningsMore checks a warning that the upstream tests do not reach,
// the warnings of ParseSelector, and that a parse without WithLogger does
// not fail. An unclosed comment does not give the warning "bytes were left",
// because skip_comment of upstream moves the stream to the end before it
// fails.
func TestWarningsMore(t *testing.T) {
	if got, want := runProcess("a:before { color:red }"), "WARN: ':before' is not supported. Selector skipped.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := runProcess("@a; b { color:red } /* x"), "WARN: The @a rule is not supported. Skipped.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	var b strings.Builder
	_, _ = ParseSelector("> a", WithLogger(slog.New(recordHandler{&b})))
	if got, want := b.String(), "WARN: Selector parsing failed cause unexpected combinator.\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	_ = ParseStyleSheet("> {}")
}
