# D105. roxmltree and cssselect2 allocate less for a large document

Status: Decided, amends D47 and D50.

## The problem

ivy reads the XHTML of a book with `roxmltree` and matches its CSS with
`cssselect2`. `idpf/epub/linear-algebra.epub` of the corpus of ivy holds
25 MB of XHTML with about 270,000 elements. ivy measured the allocations of
one run of its command `test/cmd/perf` on that book, and asked for fewer,
as item 3 of "Work for okage" in its backlog, on 2026-11-26. The
maintainers asked for the change on the same day.

## The changes

Each change has a comment that starts with `okage:` and names this
decision.

1. `roxmltree` guesses the number of nodes before a parse. roxmltree
   counts the characters `<`, which misses the text nodes, so the list of
   the nodes grew several times. `guessNodes` counts a node for each `<`
   that does not start an end tag, and a text node for each `>` that a
   character other than `<` follows.
2. `shrinkToFit` of `roxmltree` ports `Vec::shrink_to_fit` of Rust, which
   copied the list of the nodes into a list of its length. It now copies a
   list only when more than a quarter of its capacity is spare. A
   document then keeps at most a quarter of its lists as spare memory.
3. `processText` of `roxmltree` made a new text buffer for each text with
   an entity or a carriage return. One buffer in the context of the parse
   now serves each text, and keeps its memory. The string of each text is
   still a copy of its own.
4. `IterSubtree` of `cssselect2` held its frames on the heap, one for each
   element. Its stack now holds them by value.
5. `IterSubtree` and `IterChildren` made a wrapper for each element. They
   now make the wrappers of the children of one parent as one slice.
6. The new optional interface `cssselect2.ChildrenAppender` has the method
   `AppendChildren(dst []Element) []Element`. `EtreeChildren` calls it in
   place of `Children` when an `Element` has it. The iterator of
   `Children` makes closures for each element, and the list of the
   children grew one element at a time.

## Measurements

`BenchmarkParseLarge` of `roxmltree` parses a document of 20,000
paragraphs. It allocated 87.9 MB in 40,021 objects, and now allocates
26.1 MB in 20,020 objects. It is about 30% faster.

`BenchmarkIterSubtree` of `cssselect2` walks a tree of 32,001 elements. It
allocated 11.5 MB in 185,020 objects, and now allocates 11.0 MB in 132,020
objects. With `AppendChildren`, in `BenchmarkIterSubtreeAppender`, it
allocates 9.0 MB in 22,007 objects.

ivy, without a change of its own, ran `test/cmd/perf` on
`linear-algebra.epub` with a `go.work` that points at this checkout. The
run allocated 1,744 MB in 10.07 million objects before the change, and
1,441 MB in 8.90 million objects after it. An `Element` of ivy with
`AppendChildren` removes about 780,000 more objects, the closures and the
growth of the lists of the children.
