// Copyright 2012-2018 Simon Sapin and contributors
//
// Use of this source code is governed by the BSD-3-Clause license that can be
// found in the LICENSE file.

// Package cssselect2 is a port of the Python package cssselect2 0.10.1, the
// selectors of CSS 4 for a document tree. It parses selectors from the
// tokens of the package tinycss2, compiles them, gives their specificity,
// and matches them against the elements of a tree (D50). It is public, so
// that ivy can port the layout of WeasyPrint, which matches its selectors
// with cssselect2.
//
// # How the Python types map to Go
//
// Upstream matches an Element of xml.etree.ElementTree. In Go, a program
// adapts its tree to the interface Element, and wraps its root with
// FromXMLRoot or FromHTMLRoot, as upstream does. An HTML tree and a tree of
// roxmltree can both satisfy Element.
//
//   - compile_selector_list is CompileSelectorList, and CompiledSelector is
//     CompiledSelector. Upstream compiles each selector to Python source,
//     and the port compiles it to closures, with the same folding of the
//     constants 1 and 0.
//   - Matcher is the generic Matcher, whose type parameter is the type of
//     the payloads. A tuple of Matcher.match is a Match.
//   - The properties of ElementWrapper are methods, such as ID, Classes,
//     Lang and EtreeChildren. A property that can be None returns a second
//     result of false for None.
//   - A namespace prefix of None, the default namespace, is the key "" of
//     Namespaces.
//   - SelectorError wraps okage.ErrCSSSelector (D21).
//
// # Differences from upstream
//
// The port keeps the behavior of upstream, with its faults, except for
// these differences (D50):
//
//   - Where upstream raises an exception other than SelectorError for a
//     selector, such as AttributeError, NotImplementedError or SyntaxError,
//     the port returns a SelectorError. For example, :lang() with no
//     argument and [*|attr] are errors.
//   - The parser nests at most MaxDepth selectors in :is(), :where(),
//     :not() and :has().
//   - :local-link does not match a link whose URL makes urlparse of Python
//     raise ValueError, where upstream raises the error.
//   - Ancestors and PreviousSiblings make a new list on each call. Upstream
//     caches a tuple of the previous siblings for each element, which takes
//     memory in the square of the number of siblings.
//   - The flag i of an attribute selector compares with strings.ToLower,
//     where upstream uses str.lower of Python.
//   - The wrappers of one tree keep the result of each relative selector of
//     :has() for each element (D58). The results are the results of
//     upstream, and the time of the matching is about linear in the number
//     of elements. Upstream takes time in the square of the number of
//     elements, and in a power of it for :has() inside :has(). Two
//     different elements of a tree must not be equal as values of Element.
//   - IterSubtree and IterChildren make the wrappers of the children of
//     one parent as one slice. An Element with the optional method
//     AppendChildren of ChildrenAppender gives its children without the
//     closures of Children (D105).
package cssselect2
