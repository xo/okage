# D50. tinycss2 and cssselect2 are public ports

Status: Decided, amends D5, D8 and D13.

ivy lays out ebooks and web pages with a port of WeasyPrint 70.0 (ivy D15).
WeasyPrint reads its CSS with [tinycss2](https://github.com/Kozea/tinycss2)
and matches its selectors with
[cssselect2](https://github.com/Kozea/cssselect2). Both are Python packages
under the BSD-3-Clause license. ivy D21 asks okage for both ports. The CSS
parser of okage, `internal/simplecss`, reads no `@page`, no `@media` and few
selectors of CSS 3, and Go does not let another module import an internal
package. The maintainers decided this on 2026-11-16.

## The decision

1. The package `tinycss2` ports tinycss2 1.5.1 at the commit
   `2cab2705158e2b9131e0e8537b8c62b4fb5619f5` of 2026-10-25. It is a public
   package of the module `github.com/xo/okage`.
2. The package `cssselect2` ports cssselect2 0.10.1 at the commit
   `fc22accd5fdba764d95263fa8e6c97ce24f75f3e` of 2026-10-25. It is also
   public, and it imports `tinycss2`.
3. Each port has the policy `port` (D7). Its base commit is the commit of
   item 1 or item 2, and it is also the matched commit. The checkouts are
   `upstream/tinycss2` and `upstream/cssselect2` (D11). The ledgers are
   `docs/upstream/tinycss2.tsv` and `docs/upstream/cssselect2.tsv` (D12).
4. Each package keeps the BSD-3-Clause license of its upstream, with the
   copyright line of okage, in a `LICENSE` file in its folder (D13).
5. `internal/simplecss` stays as it is. `svg` still parses its CSS with it,
   as D8 says. The new packages are for ivy and for other programs.

## Why two packages, with the names of upstream

D5 gives each upstream a package of its own. Each upstream has its own
repository and its own ledger. A package for each one keeps one Go file for
each Python file, and keeps the names of each one apart. Both upstreams have
a file `parser.py`, for example. D47 and D48 kept the names of their crates,
`roxmltree` and `kurbo`, and these packages keep the names of their Python
packages too. A port of WeasyPrint then reads `tinycss2.ParseStylesheet`
where WeasyPrint reads `tinycss2.parse_stylesheet`, and
`cssselect2.CompileSelectorList` where it reads
`cssselect2.compile_selector_list`.

## How the Python types become Go types

The aim is that a port of WeasyPrint follows its source line by line. The
package comment of each package holds the full table. In brief:

1. Each class of `tinycss2/ast.py` is a struct with the same name, and a
   pointer to it implements the interface `tinycss2.Node`. `Node.Type`
   returns the class attribute `type` of upstream, such as `"ident"` or
   `"{} block"`, so that the port can switch on it as WeasyPrint does.
2. Each attribute is a field in MixedCaps: `Value`, `LowerValue`, `Unit`,
   `LowerUnit`, `IntValue`, `IsInteger`, `Arguments`, `Content` and
   `Prelude`. The line and the column are `SourceLine` and `SourceColumn`.
3. A function that takes a string or a list of component values takes a
   type parameter that is a `string` or a `[]tinycss2.Node`. The keyword
   arguments `skip_comments` and `skip_whitespace` are the options
   `WithSkipComments` and `WithSkipWhitespace` (D20).
4. A `None` of upstream is a nil value, or a second result of `false`. The
   `None` content of an at-rule that ends with a semicolon is a nil
   `Content`. The comparison `token == ','` is `tinycss2.IsLiteral(token, ",")`.
5. The parsers of colors of `color3.py`, `color4.py` and `color5.py` are
   `ParseColor3`, `ParseColor4` and `ParseColor5`. WeasyPrint 70.0 uses
   `color5`, so the port has it too.
6. cssselect2 matches the elements of ElementTree. In Go, a program adapts
   its tree to the small interface `cssselect2.Element`: the child elements,
   the local name, the namespace URL, an attribute, and the text before the
   first child. The port keeps `ElementWrapper` of upstream, which finds the
   parent, the siblings, the index and the language from the root. So ivy
   can adapt both its HTML tree and the tree of `roxmltree`.
7. `Matcher` is generic in the type of its payloads.
8. cssselect2 compiles each selector to Python source. The port compiles it
   to Go closures, with the same folding of the constants `1` and `0`.

## The guards

A stylesheet or a selector from a stranger must never make the parsers
crash or hang. Upstream has no limit of depth, and Python raises an
exception in some places where a direct port panics. The port differs from
upstream in these places only:

1. The tokenizer nests at most `tinycss2.DefaultMaxDepth` (256) blocks and
   functions. `tinycss2.WithMaxDepth` changes the limit. A deeper block
   becomes a `ParseError` of the kind `too-deep`, and the tokenizer drops
   its content. The tokenizer is a loop with a stack, so the limit keeps the
   tree small for the code that walks it.
2. The selector parser nests at most `cssselect2.MaxDepth` (32) selectors
   in `:is()`, `:where()`, `:not()`, `:has()` and the `of` of
   `:nth-child()`. The parser and the compiler recurse at each level. A
   goroutine of TinyGo has a small stack, and a test with TinyGo 0.44.0
   crashed between 40 and 80 levels, so this limit is lower than the limit
   of the tokenizer.
3. Where upstream raises an exception other than `SelectorError` for a
   selector, the port returns a `SelectorError`. Examples are `:lang()`,
   `:has()`, `[*|attr]`, `*|` and `:lang(en [x])`.
4. Serialization writes nothing for a `ParseError` of a kind that upstream
   cannot serialize, where upstream raises `TypeError`.
5. The parsers of colors return no color for `color()`, for the keyword
   `none` in `device-cmyk()`, and for an invalid alpha value in
   `device-cmyk()` or in `color()` of a custom color space. Upstream raises
   an exception for each of them.
6. `:local-link` does not match a link whose URL makes `urlparse` of Python
   raise `ValueError`.

Each package has a fuzz test: `FuzzParseStylesheet` and
`FuzzCompileSelectorList`.

The cost of matching is the cost of upstream. A selector such as
`:has(:has(:has(*)))` takes time in a power of the number of elements, in
the port as in upstream. A program that matches the selectors of a stranger
must bound the size of the document.

## What Go cannot hold

1. An escape of a surrogate code point gives U+FFFD, as the CSS syntax
   says. Python keeps a lone surrogate, and a Go string cannot.
2. An integer out of the range of `int64` saturates in `IntValue` and in
   the results of `ParseNth`. Python keeps every digit.
3. The bytes of a stylesheet decode from UTF-8, UTF-16, windows-1252 and
   x-user-defined, with all the labels that webencodings gives them. Python
   decodes each encoding of the Encoding Standard. Another label counts as
   unknown, and the decoding goes on to the next rule.
4. The flag `i` of an attribute selector compares with `strings.ToLower`,
   where upstream uses `str.lower` of Python.
5. `ElementWrapper.Ancestors` and `ElementWrapper.PreviousSiblings` make a
   new list on each call. Upstream caches a tuple of the previous siblings
   for each element, which takes memory in the square of the number of
   siblings.

## What is not ported

1. The methods `__iter__`, `__getitem__` and `__hash__` of `Color`, and
   the comparison of a `Color` with a tuple. `Color.Equal` ports the
   comparison of two colors.
2. The arguments of `ElementWrapper.matches`, `query` and `query_all` that
   are strings. In Go, compile the selectors first with
   `CompileSelectorList`.
3. The deprecated methods `iter_ancestors` and `iter_previous_siblings` of
   `ElementWrapper`. The port has `Ancestors` and `PreviousSiblings`.

## The tests

The JSON files of the tests of tinycss2 are the git submodule
`css-parsing-tests`, which the checkout lacks. In their place,
`tinycss2/testdata/make.py` runs tinycss2 on a list of cases, with 300
random cases from a fixed seed, and writes the results. The tests of the
port compare its results with them. `cssselect2/testdata/make.py` does the
same for cssselect2, on the documents of the tests of upstream and on the
selectors of web-platform-tests. The documents keep their license, in
`cssselect2/testdata/LICENSE-web-platform-tests`. The tests also port the
other test functions of upstream, and they build the trees in memory from
the documents with `roxmltree`.

## The rules that still hold

1. The errors of `cssselect2` wrap the sentinel error
   `okage.ErrCSSSelector` of the root package (D21). `tinycss2` returns no
   error. It puts each syntax error in the tree, as upstream does.
2. Both packages build with TinyGo (D25). They use no `reflect`,
   `encoding/json` or `regexp`.
3. A change to the API of either package is a change to a public API, so it
   follows the release rules of okage.
