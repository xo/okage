# D49. `html` parses HTML documents, as a port of tinyhtml5

Status: Decided, amends D5, D13 and D38.

ivy lays out ebooks and web pages with a port of WeasyPrint, and ivy D21 asks
okage for the HTML parser of that work. WeasyPrint parses HTML with
[tinyhtml5](https://github.com/CourtBouillon/tinyhtml5), a Python port of
html5lib under the MIT license. It holds the tokenizer, the tree builder with
each insertion mode of the HTML standard, the named character references and
the detection of the encoding. The package `html` reads only the labels of
Graphviz (D38). D38 says that another parser of HTML also goes in `html`.

## The decision

1. `html` ports tinyhtml5 2.1.0 at the commit
   `2a933f5b7baaf1592aa2b3779c798575db82511f` of 2026-10-25, with the
   policy `port` (D7). The base commit is that commit, and it is also the
   matched commit. The checkout is `upstream/tinyhtml5` (D11), and the
   ledger is `docs/upstream/tinyhtml5.tsv` (D12).
2. The port is in the package `html`, as D38 says, and not in a package of
   its own. The parser of the labels and its API do not change. The names of
   the two parsers do not collide: the port names its types after the
   classes of upstream, such as `htmlParser` and `htmlTokenizer`.
3. A Go file of the port starts with `tinyhtml5_` and the name of the Python
   module, as a file of `svg` starts with the name of its crate (D5):
   `tinyhtml5_constants.go`, `tinyhtml5_inputstream.go`,
   `tinyhtml5_tokenizer.go`, `tinyhtml5_treebuilder.go` and
   `tinyhtml5_parser.go`. The tests of `tests/` port to
   `tinyhtml5_test.go`, `tinyhtml5_tokenizer_test.go`,
   `tinyhtml5_tree_construction_test.go` and `tinyhtml5_encoding_test.go`.
   The test data of upstream, from html5lib-tests, is in
   `html/testdata/tinyhtml5/`, byte for byte.
4. Three files port no file of tinyhtml5:
   - `tinyhtml5_entities.go` holds the table `html.entities.html5` of
     Python, which `tokenizer.py` imports. A script makes it from Python
     3.14.
   - `document_encoding.go` holds what tinyhtml5 takes from the package
     `webencodings` and from the codecs of Python: the labels of the WHATWG
     Encoding Standard, the lookup of a label, and the decoders.
   - `document.go` holds the tree, the options and the two functions of the
     API.
5. The API is small:
   - `ParseDocument(r, opts...)` returns a `*Document`.
     `ParseFragment(r, container, opts...)` parses the content of an
     element, as `parse_fragment` of upstream does.
   - A `Document` gives its root, its html element, the name of the
     encoding that decoded it, its quirks mode, and its parse errors as a
     list of `Issue`, the type of D38. A position counts the line and the
     column from 1.
   - A `Node` is a document, an element, a text, a comment or a doctype. An
     element gives its namespace, `NamespaceHTML`, `NamespaceSVG` or
     `NamespaceMathML`, its local name, and its attributes in the order of
     the source. Each node gives its parent, its children and its siblings.
     The tree does not change after the parse.
   - The function returns an error only when the reader fails. HTML that is
     not valid gives issues, not an error.
6. The encoding comes from a byte order mark, then `WithCharset`, then a
   meta element in the first 1024 bytes, then `WithDefaultCharset`, then
   windows-1252, as upstream does. A meta element that the tree builder
   finds later makes the parse start again once, with that encoding.
7. `html` decodes UTF-8, UTF-16LE, UTF-16BE, windows-1252 and
   x-user-defined itself, by the rules of the Encoding Standard. okage does
   not require `golang.org/x/text` (D6). A program gives a decoder for the
   other encodings with `WithCharsetReader`. Without one, `html` decodes such
   an input as windows-1252, and it adds an issue.
8. A file must never stop or hang the parser. These options bound the work:
   - `WithMaxDepth` bounds the stack of open elements, 512 by default.
   - `WithMaxNodes` bounds the nodes of the tree, 1048576 by default.
   - `WithMaxIssues` bounds the list of issues, 1000 by default.

   At the depth or the node limit, the parse stops as at the end of the
   input, and adds an issue. The bound on the stack also bounds the time of
   each token, because the scope tests walk the stack. The fuzz test
   `FuzzParseDocument` and the tests of hostile input in `document_test.go`
   hold this rule.

## The differences from upstream

1. tinyhtml5 builds an ElementTree of Python, which holds text in the
   `text` and the `tail` of an element. The port builds the tree of
   `document.go`, with a node for each run of text, and each method of the
   class `Element` of upstream does what its doc comment says on that tree.
   For example, a removed element no longer takes the text after it, and a
   node that `insert_before` added is a child like any other.
2. Where upstream raises an error, the port goes on. A start tag in the text
   phase adds an issue, as `ValueError` does not. An index past the start of
   the stack reads as Python reads a negative index. An `assert` of upstream
   has no port.
3. Upstream builds a name, a value or a comment with `+=`, and searches the
   list of attributes for each new attribute. The port appends to slices of
   bytes, and it keeps a set for a tag with many attributes, so that a long
   token or a long tag takes linear time.
4. The port decodes the whole input at once, where upstream reads chunks of
   10240 characters. So each `invalid-codepoint` issue comes at the start of
   the input.
5. The codec `cp1252` of Python gives U+FFFD for the bytes 0x81, 0x8D, 0x8F,
   0x90 and 0x9D. The port gives the control character of the same number,
   as the Encoding Standard does.
6. A numeric character reference above U+10FFFF stops its value at 0x110000,
   where Python computes the whole number. Both give U+FFFD.

## What is not ported

1. `namespace_html_elements=False`. An element of HTML always has the
   namespace of HTML.
2. The arguments `transport_encoding`, `same_origin_parent_encoding` and
   `likely_encoding` of upstream.
3. The scripting flag. Upstream parses with it off, and so does the port.
4. `<template>` and `<search>`, which tinyhtml5 does not support either.
   Their tests fail in upstream and in the port.

## The tests

The port runs each test of upstream on the namespaced tree. Of the 6806
tokenizer tests, each one passes, except the 15 that upstream marks as
expected failures. 1561 tree construction tests pass, and the 195 that
upstream marks as expected failures still fail. The 82 encoding tests pass in
the test of the parser and in the test of the prescan.

## The license

tinyhtml5 is under the MIT license. `html/LICENSE` holds the MIT license with
the copyright lines of okage and of tinyhtml5, as D13 says for a package
that ports MIT code. It now covers the parser of the labels too, which the
`LICENSE` of the root covered before.

## When upstream changes

A commit of tinyhtml5 that changes `tinyhtml5/<module>.py` changes
`html/tinyhtml5_<module>.go`, and a commit that changes `tests/` changes the
tests and `html/testdata/tinyhtml5/`. If a commit changes code that a
difference above replaces, follow the difference. A change to the table of
`webencodings` or of Python changes `document_encoding.go` or
`tinyhtml5_entities.go`.

## Why

ivy reads HTML that browsers read, and only the algorithm of the HTML
standard reads it the same way. The layout of ivy is a port of WeasyPrint, so
a port of the parser of WeasyPrint gives the same tree. D38 already reserves
`html` for a parser of HTML, so the port needs no new package, and a program
gets both parsers with one import.
