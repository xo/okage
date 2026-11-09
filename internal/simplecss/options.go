package simplecss

import (
	"fmt"
	"log/slog"
)

// This file ports no upstream file. It holds the options of simplecss and
// the logger that they set (D20).

// Option is an option of ParseStyleSheet, StyleSheet.ParseMore and
// ParseSelector. It changes the configuration of one call.
type Option func(*options)

// options is the configuration that the options of simplecss set.
type options struct {
	// logger is the logger that simplecss logs its warnings to. Nil logs
	// nothing.
	logger *slog.Logger
}

// WithLogger is an option that sets the logger of a parse. The warnings
// that simplecss logs with log::warn! go to l, with the text of upstream.
// Without this option, simplecss logs nothing.
func WithLogger(l *slog.Logger) Option {
	return func(o *options) {
		o.logger = l
	}
}

// newOptions applies opts to a new configuration and returns it.
func newOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// warnf is log::warn! of the crate log. It formats the message and logs it
// to l as a warning. A nil logger logs nothing, and the message is not
// formatted.
func warnf(l *slog.Logger, format string, args ...any) {
	if l != nil {
		l.Warn(fmt.Sprintf(format, args...))
	}
}
