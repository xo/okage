// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// This file ports krilla src/interactive/destination.rs, with the methods of
// SerializeContext of src/serialize.rs and of Document of src/document.rs
// that register and write the destinations.
//
// Destinations in a PDF document.
//
// In some cases, you might want to refer to other locations within the same
// document, for example when defining the outline, or when link to a
// different section in the document from a link. To achieve this, you can
// use destinations, which are associated with a page and a specific location
// on that page.

// Destination is Destination in krilla src/interactive/destination.rs. A
// Rust enum with data becomes an interface with one type for each case:
// XYZDestination and NamedDestination. The impl From of upstream for each
// type is the conversion of the type into the interface. A Destination is a
// Target, as the case Target::Destination of upstream makes it.
//
// The type of destination.
type Destination interface {
	Target
	// serializeDestination is Destination::serialize. It writes the
	// destination into obj.
	serializeDestination(sc *serializeContext, obj pdfwriter.Obj) error
}

// NamedDestination is NamedDestination in krilla
// src/interactive/destination.rs.
//
// A destination associated with a name.
type NamedDestination struct {
	name    string
	xyzDest XYZDestination
}

// NewNamedDestination is NamedDestination::new in krilla
// src/interactive/destination.rs.
//
// Create a new named destination.
//
// When used as part of a link annotation, the destination will be
// automatically registered with the Document.
//
// That said, you can also manually register a destination without linking
// to it by calling Document.RegisterNamedDestination.
func NewNamedDestination(name string, xyzDest XYZDestination) NamedDestination {
	return NamedDestination{name: name, xyzDest: xyzDest}
}

// serializeDestination is NamedDestination::serialize in krilla
// src/interactive/destination.rs. It returns an error that wraps
// ErrPDFDuplicateNamedDestination when the name has another destination.
func (d NamedDestination) serializeDestination(sc *serializeContext, obj pdfwriter.Obj) error {
	if !sc.registerNamedDestination(d) {
		return duplicateNamedDestination(d.name)
	}
	obj.Primitive(pdfwriter.Str(d.name))
	return nil
}

// serializeTarget is the case Target::Destination of
// LinkAnnotation::serialize_type in krilla src/interactive/annotation.rs.
func (d NamedDestination) serializeTarget(sc *serializeContext, a *pdfwriter.Annotation, _ Location) error {
	return d.serializeDestination(sc, a.Insert(pdfwriter.Name("Dest")))
}

// duplicateNamedDestination is KrillaError::DuplicateNamedDestination of
// krilla src/error.rs, with the Display text of upstream.
func duplicateNamedDestination(name string) error {
	return fmt.Errorf("%w: %s", okage.ErrPDFDuplicateNamedDestination, name)
}

// XYZDestination is XyzDestination in krilla src/interactive/destination.rs.
// Upstream holds the page index and the point in an Arc. The port holds them
// as values.
//
// A destination pointing to a specific location at a specific page.
type XYZDestination struct {
	pageIndex int
	point     Point
}

// NewXYZDestination is XyzDestination::new in krilla
// src/interactive/destination.rs. krilla panics at the export when the page
// index is out of range. Document.Finish returns ErrPDFInvalidDestination
// in its place.
//
// Create a new XYZ destination. pageIndex should be the index (i.e. number)
// of the target page, and point indicates the specific location on that page
// that should be targeted.
func NewXYZDestination(pageIndex int, point Point) XYZDestination {
	return XYZDestination{pageIndex: pageIndex, point: point}
}

// equal is the impl PartialEq for XyzDestRepr in krilla
// src/interactive/destination.rs. It compares the coordinates as floats, so
// a NaN is not equal to itself.
func (d XYZDestination) equal(other XYZDestination) bool {
	return d.pageIndex == other.pageIndex &&
		d.point.X == other.point.X &&
		d.point.Y == other.point.Y
}

// typeID is the name of the type for sipHash.
func (XYZDestination) typeID() string {
	return "XYZDestination"
}

// hash is the impl Hash for XyzDestRepr in krilla
// src/interactive/destination.rs: the page index, then the bits of the
// coordinates.
func (d XYZDestination) hash(h *hasher) {
	h.usize(d.pageIndex)
	h.u32(math.Float32bits(d.point.X))
	h.u32(math.Float32bits(d.point.Y))
}

// serializeDestination is the case Destination::Xyz of Destination::serialize
// in krilla src/interactive/destination.rs.
func (d XYZDestination) serializeDestination(sc *serializeContext, obj pdfwriter.Obj) error {
	ref := sc.registerXYZDestination(d)
	obj.Primitive(ref)
	return nil
}

// serializeTarget is the case Target::Destination of
// LinkAnnotation::serialize_type in krilla src/interactive/annotation.rs.
func (d XYZDestination) serializeTarget(sc *serializeContext, a *pdfwriter.Annotation, _ Location) error {
	return d.serializeDestination(sc, a.Insert(pdfwriter.Name("Dest")))
}

// serialize is XyzDestination::serialize in krilla
// src/interactive/destination.rs. krilla panics when the page index is out of
// range. The port returns an error that wraps ErrPDFInvalidDestination, and
// writes nothing.
func (d XYZDestination) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) error {
	if d.pageIndex < 0 || d.pageIndex >= len(sc.pageInfos) {
		return fmt.Errorf("%w: attempted to link to page %d, but document only has %d pages",
			okage.ErrPDFInvalidDestination, int64(d.pageIndex)+1, len(sc.pageInfos))
	}
	pageInfo := sc.pageInfos[d.pageIndex]

	mappedPoint := d.point
	// Convert to PDF coordinates.
	invertTransform := path.TransformFromRow(1, 0, 0, -1, 0, pageInfo.surfaceHeight)
	invertTransform.MapPoint(&mappedPoint)

	cc.nonStream.destinations.Destination(rootRef).
		Page(pageInfo.ref).
		XYZ(mappedPoint.X, mappedPoint.Y, nil)
	return nil
}

// xyzDestinationEntry is an item of the field xyz_destinations of
// GlobalObjects in krilla src/serialize.rs.
type xyzDestinationEntry struct {
	ref  pdfwriter.Ref
	dest XYZDestination
}

// namedDestinationEntry is a value of the field named_destinations of
// GlobalObjects in krilla src/serialize.rs: the reference of the
// destination, and the destination.
type namedDestinationEntry struct {
	ref  pdfwriter.Ref
	dest XYZDestination
}

// registerXYZDestination is SerializeContext::register_xyz_destination in
// krilla src/serialize.rs. The destination is written at the end, when the
// references of all the pages are known.
func (sc *serializeContext) registerXYZDestination(dest XYZDestination) pdfwriter.Ref {
	return sc.registerCached(dest, func(sc *serializeContext, rootRef pdfwriter.Ref) {
		sc.globalObjects.xyzDestinations = append(sc.globalObjects.xyzDestinations,
			xyzDestinationEntry{ref: rootRef, dest: dest})
	})
}

// registerNamedDestination is SerializeContext::register_named_destination in
// krilla src/serialize.rs. It returns false, where upstream returns None,
// when the name already has another destination. Upstream returns the
// reference of the destination, which no caller reads, so the port returns
// a bool.
func (sc *serializeContext) registerNamedDestination(nd NamedDestination) bool {
	if existing, ok := sc.globalObjects.namedDestinations[nd.name]; ok {
		return existing.dest.equal(nd.xyzDest)
	}

	destRef := sc.registerXYZDestination(nd.xyzDest)
	if sc.globalObjects.namedDestinations == nil {
		sc.globalObjects.namedDestinations = map[string]namedDestinationEntry{}
	}
	sc.globalObjects.namedDestinations[nd.name] = namedDestinationEntry{ref: destRef, dest: nd.xyzDest}
	return true
}

// serializeXYZDestinations is SerializeContext::serialize_xyz_destinations in
// krilla src/serialize.rs.
func (sc *serializeContext) serializeXYZDestinations(cc *chunkContainer) error {
	xyzDestinations := sc.globalObjects.xyzDestinations
	sc.globalObjects.xyzDestinations = nil
	for _, e := range xyzDestinations {
		if err := e.dest.serialize(sc, cc, e.ref); err != nil {
			return err
		}
	}
	return nil
}

// serializeNamedDestinations is the part of ChunkContainer::finish in
// krilla src/chunk_container.rs that writes the name dictionary of the
// catalog: the named destinations, sorted by their names, and the embedded
// files, which embeddedFiles writes when writeEmbeddedFiles is true.
func serializeNamedDestinations(catalog *pdfwriter.Catalog, named map[string]namedDestinationEntry,
	remapper map[pdfwriter.Ref]pdfwriter.Ref, embeddedFiles func(*pdfwriter.Names), writeEmbeddedFiles bool,
) {
	if len(named) == 0 && !writeEmbeddedFiles {
		return
	}

	// Cannot use pdf-writer API here because it requires Ref's, while we
	// write our destinations directly into the array.
	names := catalog.Names()

	if len(named) != 0 {
		destNameTree := names.Destinations()
		destNameEntries := destNameTree.Names()

		// "The Names entries in the leaf (or root) nodes shall contain the
		// tree's keys and their associated values, arranged in key-value
		// pairs and shall be sorted lexically in ascending order by key.
		// Shorter keys shall appear before longer ones beginning with the
		// same byte sequence. Any encoding of the keys may be used as long
		// as it is self-consistent; keys shall be compared for equality on
		// a simple byte-by-byte basis."
		sorted := make([]string, 0, len(named))
		for name := range named {
			sorted = append(sorted, name)
		}
		// Note that named destinations are guaranteed to be unique, hence
		// just comparing by the name is enough. Go compares two strings
		// byte by byte, as upstream does.
		slices.Sort(sorted)

		for _, name := range sorted {
			destNameEntries.Insert(pdfwriter.Str(name), remapper[named[name].ref])
		}

		destNameEntries.Finish()
		destNameTree.Finish()
	}

	if writeEmbeddedFiles {
		embeddedFiles(names)
	}
	names.Finish()
}

// RegisterNamedDestination is Document::register_named_destination in
// krilla src/document.rs. It returns false where upstream returns None.
//
// Manually register a global named destination.
//
// Named destinations used in link annotations are automatically registered,
// so you don't need to call this function for them.
//
// Returns false if a named destination with the same name and a different
// destination has already been registered, and therefore could not be
// registered again.
func (d *Document) RegisterNamedDestination(dest NamedDestination) bool {
	return d.sc.registerNamedDestination(dest)
}
