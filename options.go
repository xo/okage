package okage

import "log/slog"

// This file ports no upstream file. It holds the options of okage and the
// logger that they set (D20).

// Option is an option of a constructor of okage, such as NewPixmap or
// NewMask. It changes the configuration of the value that the constructor
// makes.
type Option func(*options)

// options is the configuration that the options of okage set.
type options struct {
	// logger is the logger that okage logs its warnings to. Nil logs
	// nothing.
	logger *slog.Logger
}

// WithLogger is an option that sets the logger of a Pixmap or a Mask. The
// warnings that tiny-skia logs with log::warn! go to l, with the text of
// upstream. Without this option, okage logs nothing.
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

// warn is log::warn! of the crate log. It logs msg to l as a warning. A nil
// logger logs nothing.
func warn(l *slog.Logger, msg string) {
	if l != nil {
		l.Warn(msg)
	}
}
