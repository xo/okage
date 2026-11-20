// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"iter"
	"strconv"

	"github.com/xo/okage/internal/pdfwriter"
)

// resourceKind names the type of a resource of krilla src/resource.rs. The
// trait Resource of upstream has functions without self: get_dict,
// get_prefix and get_mapper. The port gives them to the kind.
type resourceKind uint8

// The kinds of resources, in the order in which upstream writes them.
const (
	resourceColorSpace resourceKind = iota
	resourceExtGState
	resourcePattern
	resourceXObject
	resourceShading
	resourceFont
)

// getPrefix is Resource::get_prefix in krilla src/resource.rs. It is the
// prefix of the names of the resources of the kind.
func (k resourceKind) getPrefix() string {
	switch k {
	case resourceColorSpace:
		return "c"
	case resourceExtGState:
		return "g"
	case resourcePattern:
		return "p"
	case resourceXObject:
		return "x"
	case resourceShading:
		return "s"
	default:
		return "f"
	}
}

// getDict is Resource::get_dict in krilla src/resource.rs. It starts the
// dictionary of the kind in a resource dictionary.
func (k resourceKind) getDict(resources *pdfwriter.Resources) *pdfwriter.Dict {
	switch k {
	case resourceColorSpace:
		return resources.ColorSpaces()
	case resourceExtGState:
		return resources.ExtGStates()
	case resourcePattern:
		return resources.Patterns()
	case resourceXObject:
		return resources.XObjects()
	case resourceShading:
		return resources.Shadings()
	default:
		return resources.Fonts()
	}
}

// resource is the trait Resource in krilla src/resource.rs. The types of the
// resources below satisfy it. Resource::new is the conversion of a
// pdfwriter.Ref into the type, such as fontResource{ref}.
type resource interface {
	// getRef is Resource::get_ref.
	getRef() pdfwriter.Ref
	// kind gives the functions of the trait without self.
	kind() resourceKind
}

// resourceable is the trait Resourceable in krilla src/resource.rs. A
// cacheable object, such as a graphics state, gives its resource for the
// reference that the serializer gives to it. The method newResource is the
// associated type Resource of upstream.
type resourceable[R resource] interface {
	cacheable
	newResource(ref pdfwriter.Ref) R
}

// The types of the resources in krilla src/resource.rs. Each name ends with
// Resource, because the other modules of krilla have types of the same names,
// such as the Font of the text and the Pattern of the paints (D10).

// extGStateResource is ExtGState in krilla src/resource.rs.
type extGStateResource struct{ ref pdfwriter.Ref }

// colorSpaceResource is ColorSpace in krilla src/resource.rs.
type colorSpaceResource struct{ ref pdfwriter.Ref }

// shadingResource is Shading in krilla src/resource.rs.
type shadingResource struct{ ref pdfwriter.Ref }

// xObjectResource is XObject in krilla src/resource.rs.
type xObjectResource struct{ ref pdfwriter.Ref }

// patternResource is Pattern in krilla src/resource.rs.
type patternResource struct{ ref pdfwriter.Ref }

// fontResource is Font in krilla src/resource.rs.
type fontResource struct{ ref pdfwriter.Ref }

func (r extGStateResource) getRef() pdfwriter.Ref  { return r.ref }
func (r colorSpaceResource) getRef() pdfwriter.Ref { return r.ref }
func (r shadingResource) getRef() pdfwriter.Ref    { return r.ref }
func (r xObjectResource) getRef() pdfwriter.Ref    { return r.ref }
func (r patternResource) getRef() pdfwriter.Ref    { return r.ref }
func (r fontResource) getRef() pdfwriter.Ref       { return r.ref }

func (extGStateResource) kind() resourceKind  { return resourceExtGState }
func (colorSpaceResource) kind() resourceKind { return resourceColorSpace }
func (shadingResource) kind() resourceKind    { return resourceShading }
func (xObjectResource) kind() resourceKind    { return resourceXObject }
func (patternResource) kind() resourceKind    { return resourcePattern }
func (fontResource) kind() resourceKind       { return resourceFont }

// resourceDictionaryBuilder is ResourceDictionaryBuilder in krilla
// src/resource.rs.
type resourceDictionaryBuilder struct {
	colorSpaces resourceMapper
	extGStates  resourceMapper
	patterns    resourceMapper
	xObjects    resourceMapper
	shadings    resourceMapper
	fonts       resourceMapper

	// prefix is an extra prefix for all entries in this resource
	// dictionary. It prevents collisions between resources, such as in an
	// AcroForm, where a font in the /DA property can be shadowed by another
	// font in the resources dictionary of the XObject.
	prefix string
}

// newResourceDictionaryBuilder is ResourceDictionaryBuilder::new in krilla
// src/resource.rs.
func newResourceDictionaryBuilder() *resourceDictionaryBuilder {
	return newResourceDictionaryBuilderWithPrefix("")
}

// newResourceDictionaryBuilderWithPrefix is
// ResourceDictionaryBuilder::new_with_prefix in krilla src/resource.rs.
func newResourceDictionaryBuilderWithPrefix(prefix string) *resourceDictionaryBuilder {
	return &resourceDictionaryBuilder{
		colorSpaces: newResourceMapper(resourceColorSpace),
		extGStates:  newResourceMapper(resourceExtGState),
		patterns:    newResourceMapper(resourcePattern),
		xObjects:    newResourceMapper(resourceXObject),
		shadings:    newResourceMapper(resourceShading),
		fonts:       newResourceMapper(resourceFont),
		prefix:      prefix,
	}
}

// getMapper is Resource::get_mapper in krilla src/resource.rs.
func (b *resourceDictionaryBuilder) getMapper(k resourceKind) *resourceMapper {
	switch k {
	case resourceColorSpace:
		return &b.colorSpaces
	case resourceExtGState:
		return &b.extGStates
	case resourcePattern:
		return &b.patterns
	case resourceXObject:
		return &b.xObjects
	case resourceShading:
		return &b.shadings
	default:
		return &b.fonts
	}
}

// registerResource is ResourceDictionaryBuilder::register_resource in krilla
// src/resource.rs. It returns the name of the resource in the dictionary.
func (b *resourceDictionaryBuilder) registerResource(obj resource) string {
	return b.getMapper(obj.kind()).remapWithName(obj.getRef(), b.prefix)
}

// finish is ResourceDictionaryBuilder::finish in krilla src/resource.rs.
func (b *resourceDictionaryBuilder) finish() resourceDictionary {
	return resourceDictionary{
		colorSpaces: b.colorSpaces.intoResourceList(),
		extGStates:  b.extGStates.intoResourceList(),
		patterns:    b.patterns.intoResourceList(),
		xObjects:    b.xObjects.intoResourceList(),
		shadings:    b.shadings.intoResourceList(),
		fonts:       b.fonts.intoResourceList(),
		prefix:      b.prefix,
	}
}

// resourceDictionary is ResourceDictionary in krilla src/resource.rs. Its
// zero value is the Default of upstream.
type resourceDictionary struct {
	colorSpaces resourceList
	extGStates  resourceList
	patterns    resourceList
	xObjects    resourceList
	shadings    resourceList
	fonts       resourceList

	prefix string
}

// hash is the derived Hash of ResourceDictionary in krilla src/resource.rs.
func (d *resourceDictionary) hash(h *hasher) {
	for _, l := range []*resourceList{&d.colorSpaces, &d.extGStates, &d.patterns, &d.xObjects, &d.shadings, &d.fonts} {
		h.usize(len(l.entries))
		for _, r := range l.entries {
			h.i32(r.Get())
		}
	}
	h.str(d.prefix)
}

// resourcesExt is the trait ResourcesExt in krilla src/resource.rs. Upstream
// implements it for the writers of pdf-writer that have a resource
// dictionary: FormXObject, TilingPattern, Type3Font, Pages and Page. Each one
// has the method Resources, and the method Pair of its dictionary, so each
// one satisfies the interface. The writer Form of the forms is not in the
// base port (D62).
type resourcesExt interface {
	// Resources is ResourcesExt::resources.
	Resources() *pdfwriter.Resources
	// Pair writes the reference of ResourcesExt::set_resources.
	Pair(key pdfwriter.Name, value pdfwriter.Primitive) *pdfwriter.Dict
}

// toPDFResources is ResourceDictionary::to_pdf_resources in krilla
// src/resource.rs. It writes the dictionary as an indirect object into
// resourcesChunk, with its reference in parent. A dictionary of PDF 2.0
// without an entry is a direct empty dictionary.
func (d *resourceDictionary) toPDFResources(parent resourcesExt, sc *serializeContext, resourcesChunk *pdfwriter.Chunk) {
	d.writeResources(sc, resourcesChunk).write(parent)
}

// resourcesEntry is the entry /Resources of an object: the reference of
// its resource dictionary, or a direct empty dictionary when indirect is
// false. It is not in krilla.
type resourcesEntry struct {
	ref      pdfwriter.Ref
	indirect bool
}

// writeResources is the first part of ResourceDictionary::to_pdf_resources
// in krilla src/resource.rs. It writes the dictionary as an indirect object
// into resourcesChunk, and returns the entry that names it. The split lets
// Document.Finish write a content stream again (D80).
func (d *resourceDictionary) writeResources(sc *serializeContext, resourcesChunk *pdfwriter.Chunk) resourcesEntry {
	writeProcSets := !sc.serializeSettings().pdfVersion().deprecatesProcSets()
	hasResourceEntries := d.colorSpaces.len() > 0 ||
		d.extGStates.len() > 0 ||
		d.patterns.len() > 0 ||
		d.xObjects.len() > 0 ||
		d.shadings.len() > 0 ||
		d.fonts.len() > 0

	if !writeProcSets && !hasResourceEntries {
		// The Resources dictionary is mandatory (or rather, it's mandatory
		// if there isn't a parent resource dictionary to inherit from, but
		// we just assume it is always mandatory), so just write an empty
		// one as a direct object instead of an indirect object, for
		// aesthetic reasons.
		return resourcesEntry{}
	}

	resourcesRef := sc.newRef()
	resources := pdfwriter.Start[pdfwriter.Resources](resourcesChunk.Indirect(resourcesRef))
	if writeProcSets {
		resources.ProcSets(
			pdfwriter.ProcSetPDF,
			pdfwriter.ProcSetText,
			pdfwriter.ProcSetImageColor,
			pdfwriter.ProcSetImageGrayscale,
		)
	}
	rdPrefix := d.prefix
	writeResourceType(resources, resourceColorSpace, d.colorSpaces, rdPrefix)
	writeResourceType(resources, resourceExtGState, d.extGStates, rdPrefix)
	writeResourceType(resources, resourcePattern, d.patterns, rdPrefix)
	writeResourceType(resources, resourceXObject, d.xObjects, rdPrefix)
	writeResourceType(resources, resourceShading, d.shadings, rdPrefix)
	writeResourceType(resources, resourceFont, d.fonts, rdPrefix)
	resources.Finish()
	return resourcesEntry{ref: resourcesRef, indirect: true}
}

// write is the second part of ResourceDictionary::to_pdf_resources in
// krilla src/resource.rs. It writes the entry into parent.
func (e resourcesEntry) write(parent resourcesExt) {
	if !e.indirect {
		parent.Resources().Finish()
		return
	}
	parent.Pair(pdfwriter.Name("Resources"), e.ref)
}

// writeResourceType is write_resource_type in krilla src/resource.rs.
func writeResourceType(resources *pdfwriter.Resources, k resourceKind, list resourceList, rdPrefix string) {
	if list.len() > 0 {
		dict := k.getDict(resources)
		for name, entry := range list.getEntries(k, rdPrefix) {
			dict.Pair(pdfwriter.Name(name), entry)
		}
		dict.Finish()
	}
}

// resourceList is ResourceList in krilla src/resource.rs. The kind of its
// resources is the PhantomData of upstream, so the methods that need it take
// it.
type resourceList struct {
	entries []pdfwriter.Ref
}

// len is ResourceList::len in krilla src/resource.rs.
func (l resourceList) len() int {
	return len(l.entries)
}

// resourceName is ResourceList::name_from_number and
// ResourceMapper::name_from_number in krilla src/resource.rs.
func resourceName(k resourceKind, num uint32, rdPrefix string) string {
	return rdPrefix + k.getPrefix() + strconv.FormatUint(uint64(num), 10)
}

// getEntries is ResourceList::get_entries in krilla src/resource.rs.
func (l resourceList) getEntries(k resourceKind, rdPrefix string) iter.Seq2[string, pdfwriter.Ref] {
	return func(yield func(string, pdfwriter.Ref) bool) {
		for i, r := range l.entries {
			if !yield(resourceName(k, uint32(i), rdPrefix), r) {
				return
			}
		}
	}
}

// resourceMapper is ResourceMapper in krilla src/resource.rs. It numbers the
// references of one kind in the order of their first use.
type resourceMapper struct {
	kind     resourceKind
	forward  []pdfwriter.Ref
	backward map[pdfwriter.Ref]uint32
}

// newResourceMapper is ResourceMapper::new in krilla src/resource.rs.
func newResourceMapper(k resourceKind) resourceMapper {
	return resourceMapper{kind: k, backward: map[pdfwriter.Ref]uint32{}}
}

// remap is ResourceMapper::remap in krilla src/resource.rs.
func (m *resourceMapper) remap(ref pdfwriter.Ref) uint32 {
	if num, ok := m.backward[ref]; ok {
		return num
	}
	num := uint32(len(m.forward))
	m.forward = append(m.forward, ref)
	m.backward[ref] = num
	return num
}

// remapWithName is ResourceMapper::remap_with_name in krilla
// src/resource.rs.
func (m *resourceMapper) remapWithName(ref pdfwriter.Ref, rdPrefix string) string {
	return resourceName(m.kind, m.remap(ref), rdPrefix)
}

// intoResourceList is ResourceMapper::into_resource_list in krilla
// src/resource.rs.
func (m *resourceMapper) intoResourceList() resourceList {
	return resourceList{entries: m.forward}
}
