# D20. Options use the functional options pattern, and logs go to `slog`

Status: Decided.

## Options

okage applies options and changes to its parts with the functional options
pattern. An option is a function that changes the configuration of the value
that it is given.

1. A function or a constructor that takes options takes them as the last
   parameters, as `...Option`, such as `svg.Parse(data, svg.WithLogger(l))`.
2. The name of each option starts with `With`, such as `WithLogger` or
   `WithFonts`.
3. Each option holds its value in the value that it configures, never in a
   package variable, as dbmeta hard rule 6 says.
4. A Rust struct of options, such as `usvg::Options`, becomes the private
   configuration of the Go function, with one exported option for each field
   that a user sets. The default of each field is the default of upstream.
5. The doc comment of each option names the field of upstream that it sets.

## Logs

tiny-skia logs from `edge_builder.rs`, `mask.rs` and `shaders/gradient.rs`.
usvg and resvg log from 23 files, with 65 warnings and 1 debug message. They
use the crate `log`.

1. okage logs to a `*slog.Logger` of the package `log/slog`, which the caller
   gives with an option.
2. A Rust `log::warn!` becomes a call of `Warn` on that logger, and
   `log::debug!` a call of `Debug`, with the same text.
3. Without the option, okage logs nothing. It never logs to a global logger.

Options and the logger are APIs that upstream does not have in this form.
