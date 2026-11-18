# D58. SVG images for a layout, and :has() in linear time

Status: Decided, amends D47 and D50.

The layout of ivy is a port of WeasyPrint (ivy D15). It reads the size of an
SVG image before it draws the image, and it draws the inline `svg` elements
of an HTML document. On 2026-11-18 the maintainers asked okage for four
changes. With them, ivy does not parse an image twice, and does not write an
inline `svg` element back to XML text. This decision records the new API and
each difference from upstream.

## The size of an image before a parse

1. `svg.ReadDimensions(data)` returns the `svg.Dimensions` of SVG data,
   plain or compressed with gzip. These are the attributes `width`,
   `height` and `viewBox` of the root element. A `Has` field is false for an
   attribute that is missing or not valid.
2. It parses the XML only up to the start tag of the root element, with the
   new function `roxmltree.ParseRoot`. It reads the DTD and expands its
   entities, as a full parse does. It reads no style sheet.
3. `svg.NodeDimensions(node)` reads the same attributes of an element of
   roxmltree that is already parsed.
4. A width and a height are `svg.Length` values with their units, as the
   attributes give them. The caller resolves `em`, `ex` and percentages,
   because WeasyPrint resolves them with the font size of the layout.
   `svg.Length`, `svg.LengthUnit` and the constants `svg.LengthUnit*` are
   aliases of the types of `internal/svgtypes`.
5. A `viewBox` with a width or a height that is not larger than zero is not
   valid, as in usvg.
6. The errors are the errors of `FromData`: data that is not UTF-8, XML that
   does not parse up to the root element, and a root element that is not
   `svg`. `ParseRoot` does not read past the start tag. So data that
   `ReadDimensions` accepts can still fail in `FromData`.

## A tree from a document of roxmltree

1. `svg.FromXMLTree(doc, opts...)` exports `Tree::from_xmltree` of usvg,
   which okage had as the unexported function `fromXMLTree`. This replaces
   item 5 of D47.
2. `roxmltree.Builder` makes a `Document` from calls, without XML text. Its
   API is `NewBuilder`, `StartElement`, `Namespace`, `Attribute`, `Text`,
   `EndElement`, `Copy` and `Document`. A program that holds the tree of
   another parser, such as the HTML tree of ivy, builds the document of an
   inline `svg` element with it.
3. A name is an expanded name, with its namespace URI. The builder does not
   declare a prefix for a URI, and it does not check that a name is a valid
   XML name. `Namespace` declares a prefix, for `Node.Namespaces` and
   `Node.LookupPrefix`.
4. A built document has no input text. Its nodes and its attributes have
   empty ranges.
5. An error of the builder is a sentinel error itself, without a position,
   because a position means nothing without text. A call out of order
   returns the new sentinel error `okage.ErrXMLBuildOrder`. Examples are a
   second root element, text outside the root element and an attribute
   after the content of an element.
6. The builder takes `ParsingOptions` and reads only its `NodesLimit`. It
   does not take a functional option (D20), because roxmltree configures
   each parse with that struct.
7. `svg.FromXMLNode(node, opts...)` parses an `svg` element inside a larger
   document, such as an XHTML page. It copies the element and its
   descendants into a new document with `Builder.Copy`. The style sheets,
   the IDs and the links outside the element do not apply to the image, as
   for a separate file.

## An image of size zero

1. usvg returns `okage.ErrSVGInvalidSize` when the width or the height of
   the image is zero. The new option `svg.WithZeroSize(true)` accepts an
   image whose width or height resolves to zero. The parse then returns a
   `Tree` with an empty root group, and `Tree.Size` returns the zero `Size`.
   A layout lays the image out as an empty box.
2. A negative size, or a size that is not finite, is still an error.
3. The images that the image links to do not take the option. A nested
   image of size zero is still left out, as in usvg.
4. The `Renderer` returns `okage.ErrSVGInvalidRenderSize` for a tree of the
   zero size. It does not render an image of one pixel.
5. The default is false, which keeps the behavior of usvg.

## The time of :has()

1. D50 says that the cost of matching is the cost of upstream. Upstream
   tests `:has()` on an element with a walk of its subtree or of its next
   siblings. On all the elements of a document, that takes time in the
   square of the number of elements. For `:has()` inside `:has()`, it takes
   time in a power of the number of elements.
2. The port keeps the result of each relative selector of `:has()` for each
   element. All the wrappers of one root, from `FromXMLRoot` or
   `FromHTMLRoot`, share one cache. The descendant combinator computes the
   result of an element from the results of its children. The combinator
   `~` computes it from the result of the next sibling. So each relative
   selector tests each element about once, and the time of the matching is
   about linear in the number of elements.
3. A relative selector with its own descendant combinator, such as
   `:has(a b)`, still walks the ancestors of each element, as upstream
   does.
4. The results do not change. The tests of cssselect2 give the same
   results. `TestHasIsLinear` counts the reads of trees of about 4,000
   elements, and fails without the cache.
5. A value of `Element` is a key of the cache. Two values for the same
   element must be equal, and two values for different elements must not
   be. The documentation of `Element` says so.
6. `SetEtreeChildren` drops the cache, because it changes the tree.
7. A wrapper that a program makes without `FromXMLRoot` or `FromHTMLRoot`
   has no cache. A test on it uses a cache for that call only.
8. The cache holds about one entry for each element and each relative
   selector. It lives as long as the wrappers of the tree.

A budget that returns no match after a limit changes the results. A cache
does not. So the port has a cache, and no option for a budget.

## The rules that still hold

1. Each error is or wraps a sentinel error of the root package (D21).
2. `roxmltree` and `cssselect2` still build with TinyGo (D25). The builder,
   `ParseRoot` and the cache use no `reflect`, and they walk a tree with a
   stack, not with recursion.
3. Each change is a change to a public API, so it follows the release rules
   of okage.
