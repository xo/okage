# D8. The XML and CSS parsers are ports of roxmltree and simplecss

Status: Decided.

okage parses XML with a port of the crate roxmltree, in `internal/roxmltree`,
and CSS with a port of the crate simplecss, in `internal/simplecss`. Each port
stays as close to the code of upstream as it can, file by file. okage uses no
other Go package for XML or CSS.

## Why roxmltree

1. usvg reads the tree that roxmltree builds. Its rules for entities,
   namespaces, white space and errors decide what usvg sees.
2. roxmltree expands the entities of an internal DTD, also when the value of
   an entity is markup. 4 of the 1,742 tests of resvg need this, such as
   `structure/svg/elements-via-ENTITY-reference-1.svg`.
3. roxmltree is about 3,800 lines, with its own tokenizer.

## Why simplecss

1. usvg parses each `style` element with `StyleSheet::parse_more` and matches
   each selector against the nodes of roxmltree through the trait `Element`
   of simplecss. It reads each `style` attribute with `DeclarationTokenizer`.
   On 2026-11-09, 21 of the SVG files of the tests of resvg have a `style`
   element, and 50 have a `style` attribute.
2. The selectors are the type, universal, class, ID, attribute and
   `:first-child` selectors, with the descendant, child and adjacent sibling
   combinators. The rules apply in the order of their specificity, and
   `!important` wins. Which pseudo-classes and at-rules simplecss skips, and
   how it recovers from bad CSS, decide what usvg sees.
3. simplecss is about 1,150 lines in three files, and it depends on no other
   crate.

## What was measured

On 2026-11-09 twelve Go packages without reflection were compiled with Go and
with TinyGo 0.44.0, for the host and for `wasip1`. Only the probe of the
standard library was run, because the code of the other packages was not run
on the machine that measured.

| Package | Kind | Size for `wasip1`, TinyGo and Go | What it lacks |
| --- | --- | --- | --- |
| `encoding/xml`, the token API | XML tokens | 275 KiB and 2,575 KiB | It does not read a DTD. An entity of a DTD is an error in strict mode, and stays as text in the other mode. Its map `Entity` replaces text only, not markup |
| `github.com/tdewolff/parse/v2/xml` | XML lexer | 234 KiB and 2,501 KiB | No tree, no namespaces and no entities |
| `github.com/orisano/gosax` | XML events | 227 KiB and 2,472 KiB | It imports `encoding/xml`, with the same limits |
| `github.com/muktihari/xmltokenizer` | XML tokens | 234 KiB and 2,458 KiB | No namespaces. Version 0.0.5 |
| `github.com/beevik/etree` | XML tree | 294 KiB and 2,860 KiB | It builds on `encoding/xml`, with the same limits |
| `github.com/HBTGmbH/gosaxml` | XML tokens | 240 KiB and 2,497 KiB, from `main` only | An error for a `<!DOCTYPE>`, no decoding of entities, and a depth of 255 at most. On amd64 and arm64 it links the assembly of `github.com/klauspost/cpuid/v2`, which TinyGo cannot link. The tag `v0.0.44` has no code for other architectures |
| `github.com/andybalholm/cascadia` | CSS selectors | 874 KiB and 3,664 KiB | It matches only an `*html.Node` of `golang.org/x/net/html`, with the rules of HTML. No style sheet and no declarations. It imports `golang.org/x/net` and `regexp` |
| `github.com/tdewolff/parse/v2/css` | CSS lexer and parser | 235 KiB and 2,514 KiB | No selectors, no specificity and no matching |
| `github.com/gorilla/css` | CSS tokens | 430 KiB and 2,979 KiB | Tokens only, with `regexp` |
| `github.com/aymerick/douceur` | CSS parser | 479 KiB and 3,081 KiB | No matching, with `regexp` |
| `github.com/vanng822/css` | CSS parser | 480 KiB and 3,078 KiB | No matching, with `regexp` |
| `github.com/napsy/go-css` | CSS parser | 249 KiB and 2,502 KiB | It returns a `map`, so the order of the rules and of the declarations is lost |

Each package reaches `reflect` through `fmt` or `encoding/binary`, which TinyGo
supports. With each one, okage still has to write most of the work of the
crate, and none gives the same result as upstream. The goal of no
dependencies (D6) points to the ports too.

If the parsers are ever too slow, the scanners of gosaxml and the lexers of
`github.com/tdewolff/parse/v2` are a source of ideas, not a dependency.
