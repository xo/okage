# D104. `pdf` links to a named destination by its name alone

Status: Decided, amends D62.

## The problem

A link annotation of krilla to a named destination takes a
`NamedDestination`, which holds the name and the page and point of the
destination. So the caller must know the place of the destination when it
draws the page of the link. ivy draws the pages of a document in order.
When a page links to an anchor of a later page, the PDF writer of ivy,
`pdf/pdfenc`, draws every page a second time.

ivy asked for a change on 2026-11-26, as item 2 of "Work for okage" in its
backlog. The maintainers asked for it on the same day.

## The rule

1. The new target `pdf.DestinationName` holds the name of a named
   destination and nothing else. A link annotation to it writes the name
   into the key `/Dest`, as a link to a `NamedDestination` does.
2. The caller registers the destination of the name at any time before
   `Document.Finish`: with `Document.RegisterNamedDestination`, or with a
   link to a `NamedDestination` of the same name on any page.
3. `Document.Finish` collects the names of the document before it writes
   the pages. krilla writes the pages, with their annotations, in
   `Document.Finish` too, so each name is known by then.
4. `Document.Finish` leaves out a link to a name that has no destination,
   and logs the name to the logger of `WithLogger`. It returns no error. A
   document from HTML or EPUB often links to an anchor that no page holds,
   and ivy already leaves out such a link. A page whose links are all left
   out has no `/Annots`.

A file with a link to a `DestinationName` has the same bytes as a file
with a link to the `NamedDestination` itself.

## The code

`DestinationName`, `serializeContext.collectNames` and
`serializeContext.unresolvedLink` are in `pdf/destination.go`.
`internalPage.serialize` of `pdf/page.go` skips a link that
`unresolvedLink` reports. `pdf/destination_name_test.go` tests them.
