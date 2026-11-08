# D9. Text uses go-text, and the layout of text of usvg is ported

Status: Decided.

usvg converts text into paths. It finds fonts with fontdb, shapes text with
harfrust, reads outlines and color glyphs with skrifa and read-fonts, and
orders text with unicode-bidi, unicode-script and unicode-vo. These crates
are more than 135,000 lines:

| Crate | Lines | What it does |
| --- | --- | --- |
| `fontdb` | about 2,100 | finds fonts in folders and with fontconfig |
| `harfrust` | about 28,000 | shapes text. It is a port of HarfBuzz |
| `skrifa` | about 29,000 | reads glyph outlines, including color glyphs |
| `read-fonts` | about 80,000 | reads the tables of a font file. Most of it is generated |

## The rules

1. okage ports none of these crates. It uses
   `github.com/go-text/typesetting`, a Go port of HarfBuzz with a font parser
   and its own `bidi` package (D6).
2. okage ports the layout of text of usvg, the files `mod.rs`, `layout.rs`,
   `flatten.rs`, `colr.rs` and `cache.rs` of `usvg/src/text`, about 2,300
   lines, file by file into `svg`. Where the Rust code calls one of the crates
   above, the Go code calls go-text, or a package that go-text uses, that does
   the same work. The doc comment of each such function says which call of a
   crate it replaces.
3. If go-text cannot do the work of a call, the fault goes in
   `docs/BACKLOG.md`, and the maintainers decide what okage does.
4. A later commit of resvg that changes `usvg/src/text` is ported, as for any
   other file of usvg. A commit that only raises the version of a crate of
   text changes nothing that okage ports.
5. The text that okage renders can differ from the text of resvg. That is an
   accepted difference, and `README.md` lists it. The 386 tests of resvg that
   render text are compared within the tolerance of D23.

## Fonts come from the caller

fontdb finds the fonts of a system in its folders and with fontconfig. A
program on WebAssembly, or one built with TinyGo, has no such folders, and a
browser does not give the bytes of its fonts to a program.

1. `svg` takes its fonts from the caller, as font files in bytes or as an
   `fs.FS`, through an option (D20).
2. A helper can read the fonts of the system from an `fs.FS`, such as
   `os.DirFS`. On WebAssembly it finds no fonts, and it returns no error for
   that.
3. No build tag selects the fonts of a platform. Only the `fs.FS` differs.
4. The tests use the fonts in `crates/resvg/tests/fonts` of resvg.

Astra, Gemini Pro and DeepSeek Pro each advised on 2026-11-09 not to port the
crates of text, and to take the fonts from the caller.
