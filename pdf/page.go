// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"fmt"
	"math"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/pdfwriter"
	"github.com/xo/okage/path"
)

// PageSettings is PageSettings in krilla src/page.rs. An optional box of
// upstream is a pointer, and nil is None.
//
// The settings of a page.
type PageSettings struct {
	// mediaBox defines the visible area of the surface.
	mediaBox Rect
	// pageLabel is the page label of the page.
	pageLabel PageLabel
	// cropBox is the crop box of the page.
	cropBox *Rect
	// bleedBox is the bleed box of the page.
	bleedBox *Rect
	// trimBox is the trim box of the page.
	trimBox *Rect
	// artBox holds the actual content boundaries.
	artBox *Rect
}

// NewPageSettings is PageSettings::new in krilla src/page.rs.
//
// Create new page settings with the given media box.
//
// The media box defines the visible area of the page in the coordinate system
// of its surface. In the majority of cases, you want it to be aligned at the
// origin of the coordinate system, which is what PageSettingsFromWH does.
func NewPageSettings(mediaBox Rect) PageSettings {
	s := DefaultPageSettings()
	s.mediaBox = mediaBox
	return s
}

// PageSettingsFromWH is PageSettings::from_wh in krilla src/page.rs.
//
// Try to create new page settings with the specified width and height and a
// media box that is aligned at the origin.
//
// Returns false if either the width or the height is not > 0.
func PageSettingsFromWH(width, height float32) (PageSettings, bool) {
	size, ok := path.SizeFromWH(width, height)
	if !ok {
		return PageSettings{}, false
	}
	mediaBox, ok := path.RectFromXYWH(0, 0, size.Width(), size.Height())
	if !ok {
		return PageSettings{}, false
	}
	return NewPageSettings(mediaBox), true
}

// DefaultPageSettings is the Default of PageSettings in krilla src/page.rs:
// the size of A4, 595 by 842.
func DefaultPageSettings() PageSettings {
	// Default for A4.
	const width = 595.0
	const height = 842.0
	mediaBox, _ := path.RectFromXYWH(0, 0, width, height)
	return PageSettings{mediaBox: mediaBox}
}

// WithPageLabel is PageSettings::with_page_label in krilla src/page.rs.
//
// Change the page label.
func (s PageSettings) WithPageLabel(pageLabel PageLabel) PageSettings {
	s.pageLabel = pageLabel
	return s
}

// surfaceHeight is PageSettings::surface_height in krilla src/page.rs.
//
// The height that is used to convert from surface coordinates to PDF
// coordinates.
func (s PageSettings) surfaceHeight() float32 {
	return s.mediaBox.Height()
}

// optionalRect copies the rectangle that box points to, so that a later
// change of it does not change the settings.
func optionalRect(box *Rect) *Rect {
	if box == nil {
		return nil
	}
	r := *box
	return &r
}

// WithCropBox is PageSettings::with_crop_box in krilla src/page.rs.
//
// Change the crop box.
//
// The crop box defines the region to which the page contents are to be
// clipped when displayed or printed. Default is the media box.
//
// If nil, no /CropBox attribute will be written to the page.
func (s PageSettings) WithCropBox(cropBox *Rect) PageSettings {
	s.cropBox = optionalRect(cropBox)
	return s
}

// WithBleedBox is PageSettings::with_bleed_box in krilla src/page.rs.
//
// Change the bleed box.
//
// The bleed box defines the region to which the page contents needs to be
// clipped when output in a production environment. It includes any extra
// bleed area needed for printing.
//
// If nil, no /BleedBox attribute will be written to the page.
func (s PageSettings) WithBleedBox(bleedBox *Rect) PageSettings {
	s.bleedBox = optionalRect(bleedBox)
	return s
}

// WithTrimBox is PageSettings::with_trim_box in krilla src/page.rs.
//
// Change the trim box.
//
// The trim box defines the intended dimensions of the finished page after
// trimming. It can be smaller than the media box and bleed box to
// accommodate bleed for printing.
//
// If nil, no /TrimBox attribute will be written to the page.
func (s PageSettings) WithTrimBox(trimBox *Rect) PageSettings {
	s.trimBox = optionalRect(trimBox)
	return s
}

// WithArtBox is PageSettings::with_art_box in krilla src/page.rs.
//
// Change the art box.
//
// The art box defines the extent of the page's meaningful content (including
// potential white space) as intended by the page's creator.
//
// If nil, no /ArtBox attribute will be written to the page.
func (s PageSettings) WithArtBox(artBox *Rect) PageSettings {
	s.artBox = optionalRect(artBox)
	return s
}

// annotation is the part of Annotation of krilla src/interactive/annotation.rs
// that a page calls. The port of that file adds the type Annotation, which
// satisfies it, and the method AddAnnotation of Page, which appends to the
// field annotations.
type annotation interface {
	// serialize is Annotation::serialize. It writes the annotation with the
	// reference ref, on a page of the height pageHeight.
	serialize(sc *serializeContext, cc *chunkContainer, ref pdfwriter.Ref, pageHeight float32) error
}

// Page is Page in krilla src/page.rs. Document.StartPage makes one. The
// method Surface of upstream makes a Surface, and the port of surface.rs adds
// it, as the package comment says. The methods add_tagged_annotation and
// add_widget_annotation are for the tags and the forms, which the base port
// leaves out (D62).
//
// A single page.
//
// In most cases, all you need to do is to call the method Surface, so you can
// start drawing on the page. However, there are a few other operations you
// can perform, such as adding annotations to a page.
type Page struct {
	doc          *Document
	sc           *serializeContext
	cc           *chunkContainer
	pageSettings PageSettings
	pageIndex    int
	// pageStream is the stream that the surface of the page gives when it
	// finishes. It is empty when the page has no surface.
	pageStream  *Stream
	annotations []annotation
	// finishSurface finishes the surface of the page that is open, if any.
	// Rust drops the surface before the page. The method Surface sets it,
	// and Finish calls it.
	finishSurface func()
	finished      bool
}

// newPage is Page::new in krilla src/page.rs.
func newPage(doc *Document, pageIndex int, pageSettings PageSettings) *Page {
	return &Page{
		doc:          doc,
		sc:           doc.sc,
		cc:           doc.cc,
		pageSettings: pageSettings,
		pageIndex:    pageIndex,
		pageStream:   emptyStream(),
	}
}

// rootTransform is Page::root_transform in krilla src/page.rs.
func (p *Page) rootTransform() Transform {
	return pageRootTransform(p.pageSettings.surfaceHeight())
}

// pageRootTransform is page_root_transform in krilla src/page.rs. It flips
// the y axis, so that the origin of a surface is the top left corner of its
// page.
func pageRootTransform(height float32) Transform {
	return path.TransformFromRow(1, 0, 0, -1, 0, height)
}

// Finish is the impl Drop for Page in krilla src/page.rs, which the method
// finish of upstream runs. It finishes the open surface of the page first.
// A second call does nothing. Document.StartPage and Document.Finish call it
// for a page that is not finished.
//
// The struct parent of the page of upstream is for the tags, which the base
// port leaves out (D62).
func (p *Page) Finish() {
	if p.finished {
		return
	}
	if p.finishSurface != nil {
		finish := p.finishSurface
		p.finishSurface = nil
		finish()
	}
	p.finished = true
	if p.doc.page == p {
		p.doc.page = nil
	}

	annotations := p.annotations
	p.annotations = nil
	stream := p.pageStream
	p.pageStream = emptyStream()
	page := newInternalPage(stream, p.sc, annotations, p.pageSettings, p.pageIndex)
	p.sc.registerPage(page)
}

// NumberingStyle is NumberingStyle in krilla src/page.rs.
//
// The numbering style of a page label.
type NumberingStyle uint8

// The values of NumberingStyle.
const (
	// NumberingStyleArabic is NumberingStyle::Arabic. Arabic numerals.
	NumberingStyleArabic NumberingStyle = iota
	// NumberingStyleLowerRoman is NumberingStyle::LowerRoman. Lowercase
	// Roman numerals.
	NumberingStyleLowerRoman
	// NumberingStyleUpperRoman is NumberingStyle::UpperRoman. Uppercase
	// Roman numerals.
	NumberingStyleUpperRoman
	// NumberingStyleLowerAlpha is NumberingStyle::LowerAlpha. Lowercase
	// letters (a-z, then aa-zz, ...).
	NumberingStyleLowerAlpha
	// NumberingStyleUpperAlpha is NumberingStyle::UpperAlpha. Uppercase
	// letters (A-Z, then AA-ZZ, ...).
	NumberingStyleUpperAlpha
)

// toPDF is NumberingStyle::to_pdf in krilla src/page.rs.
func (s NumberingStyle) toPDF() pdfwriter.NumberingStyle {
	switch s {
	case NumberingStyleArabic:
		return pdfwriter.NumberingStyleArabic
	case NumberingStyleLowerRoman:
		return pdfwriter.NumberingStyleLowerRoman
	case NumberingStyleUpperRoman:
		return pdfwriter.NumberingStyleUpperRoman
	case NumberingStyleLowerAlpha:
		return pdfwriter.NumberingStyleLowerAlpha
	default:
		return pdfwriter.NumberingStyleUpperAlpha
	}
}

// internalPage is InternalPage in krilla src/page.rs. The deferred chunk of
// the stream of upstream is a chunk, which the port makes at once. The
// struct parent of upstream is for the tags, which the base port leaves out.
type internalPage struct {
	streamRef       pdfwriter.Ref
	streamResources resourceDictionary
	streamChunk     *pdfwriter.Chunk
	pageSettings    PageSettings
	pageIndex       int
	annotations     []annotation
}

// newInternalPage is InternalPage::new in krilla src/page.rs. It writes the
// content stream of the page into a chunk of its own.
func newInternalPage(stream *Stream, sc *serializeContext, annotations []annotation, pageSettings PageSettings, pageIndex int) *internalPage {
	for _, validationError := range stream.validationErrors {
		sc.registerValidationError(validationError)
	}

	streamRef := sc.newRef()
	settings := sc.serializeSettings()
	streamResources := stream.resourceDictionary
	stream.resourceDictionary = resourceDictionary{}
	chunk := sc.newChunk()

	write := func(chunk *pdfwriter.Chunk, content []byte) {
		pageStream := newFilterStreamFromContentStream(content, settings).finish(settings)
		s := chunk.Stream(streamRef, pageStream.encodedData())
		pageStream.writeFilters(s.Dict)
		s.Finish()
	}
	write(chunk, stream.content)
	// okage: Document.Finish writes the stream again when the subset of
	// one of its fonts fails (D80).
	sc.registerTextStream(chunk, &streamResources, write)

	return &internalPage{
		streamResources: streamResources,
		streamRef:       streamRef,
		streamChunk:     chunk,
		annotations:     annotations,
		pageSettings:    pageSettings,
		pageIndex:       pageIndex,
	}
}

// serialize is InternalPage::serialize in krilla src/page.rs. krilla panics
// when a box of the page does not flip into the coordinates of PDF. The port
// returns ErrPDFInvalidBox.
func (p *internalPage) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) error {
	var annotationRefs []pdfwriter.Ref

	hasAnnotations := len(p.annotations) != 0
	if hasAnnotations {
		for _, annotation := range p.annotations {
			annotRef := sc.newRef()
			if err := annotation.serialize(sc, cc, annotRef, p.pageSettings.surfaceHeight()); err != nil {
				return err
			}
			annotationRefs = append(annotationRefs, annotRef)
		}
	}

	chunk := cc.nonStream.pages
	page := chunk.Page(rootRef)
	p.streamResources.toPDFResources(page, sc, cc.nonStream.resourceDictionaries)

	transformRect := func(name string, rect Rect) (pdfwriter.Rect, error) {
		r, ok := rect.Transform(pageRootTransform(p.pageSettings.surfaceHeight()))
		if !ok {
			return pdfwriter.Rect{}, fmt.Errorf("%w: the %s %s of the page %d", okage.ErrPDFInvalidBox, name, rect, p.pageIndex)
		}
		return rectToPDFRect(r), nil
	}

	mediaBox, err := transformRect("media box", p.pageSettings.mediaBox)
	if err != nil {
		return err
	}
	page.MediaBox(mediaBox)

	// The remaining types of box are not mandatory, so we only set them if
	// they are present.
	boxes := []struct {
		name  string
		rect  *Rect
		write func(pdfwriter.Rect) *pdfwriter.Page
	}{
		{"crop box", p.pageSettings.cropBox, page.CropBox},
		{"bleed box", p.pageSettings.bleedBox, page.BleedBox},
		{"trim box", p.pageSettings.trimBox, page.TrimBox},
		{"art box", p.pageSettings.artBox, page.ArtBox},
	}
	for _, box := range boxes {
		if box.rect == nil {
			continue
		}
		r, err := transformRect(box.name, *box.rect)
		if err != nil {
			return err
		}
		box.write(r)
	}

	// Only required for PDF/UA, but might as well always set it if there are
	// annotations.
	//
	// Since the navigation order of annotations only has an effect if there
	// are annotations, we usually only set the key in that case. However,
	// the accessibility audit in Adobe Acrobat always requires the key to be
	// set, even if there are no annotations. Hence, we check if there is an
	// accessibility validator or one requiring tags to decide whether to
	// force the key.
	//
	// Since forcing /Tabs /S unconditionally for PDF/A and PDF/UA files
	// targeting PDF 1.4 and below would unconditionally raise a
	// RequiresNewerPdfVersion(StructureOrderTabbing) error, we also check
	// the target version.
	settings := sc.serializeSettings()
	minimum := featureStructureOrderTabbing.minimumPDFVersion()
	if (hasAnnotations ||
		((settings.validators().hasAccessibility() || settings.validators().requiresTagging()) &&
			settings.pdfVersion() >= minimum)) &&
		settings.enableTagging {
		if settings.pdfVersion() >= minimum {
			page.TabOrder(pdfwriter.TabOrderStructureOrder)
		} else {
			sc.registerValidationError(validationError{
				kind:     validationRequiresNewerPDFVersion,
				feature:  featureStructureOrderTabbing,
				location: sc.location,
			})
		}
	}

	page.Parent(sc.pageTreeRef)
	page.Contents(p.streamRef)

	if len(annotationRefs) != 0 {
		page.Annotations(annotationRefs...)
	}

	// Populate the refs for each annotation in page infos.
	sc.pageInfos[p.pageIndex].annotations = annotationRefs

	page.Finish()
	cc.streams.pages = append(cc.streams.pages, p.streamChunk)

	return nil
}

// PageLabel is PageLabel in krilla src/page.rs. Each field of upstream is an
// Option. A field of the port has a flag for Some, except the offset, which
// is a NonZeroU32 of upstream, so 0 is None.
//
// A page label.
type PageLabel struct {
	// style is the numbering style of the page label.
	style    NumberingStyle
	hasStyle bool
	// prefix is the prefix of the page label.
	prefix    string
	hasPrefix bool
	// offset is the numeric value of the page label. 0 is None.
	offset uint32
}

// NewPageLabel is PageLabel::new in krilla src/page.rs. A nil style or prefix
// is None, and an offset of 0 is None.
//
// Create a new page label.
func NewPageLabel(style *NumberingStyle, prefix *string, offset uint32) PageLabel {
	var l PageLabel
	if style != nil {
		l.style, l.hasStyle = *style, true
	}
	if prefix != nil {
		l.prefix, l.hasPrefix = *prefix, true
	}
	l.offset = offset
	return l
}

// isEmpty is PageLabel::is_empty in krilla src/page.rs.
func (l PageLabel) isEmpty() bool {
	return !l.hasStyle && !l.hasPrefix && l.offset == 0
}

// serialize is PageLabel::serialize in krilla src/page.rs. An offset that
// does not fit in an i32 is not written, as upstream does.
func (l PageLabel) serialize(cc *chunkContainer, rootRef pdfwriter.Ref) {
	chunk := cc.nonStream.pageLabels
	label := pdfwriter.Start[pdfwriter.PageLabel](chunk.Indirect(rootRef))

	if l.hasStyle {
		label.Style(l.style.toPDF())
	}

	if l.hasPrefix {
		label.Prefix(pdfwriter.TextStr(l.prefix))
	}

	if l.offset != 0 && l.offset <= math.MaxInt32 {
		label.Offset(int32(l.offset))
	}

	label.Finish()
}

// pageLabelContainer is PageLabelContainer in krilla src/page.rs.
type pageLabelContainer struct {
	labels []PageLabel
}

// newPageLabelContainer is PageLabelContainer::new in krilla src/page.rs. It
// returns false when no page has a label.
func newPageLabelContainer(labels []PageLabel) (pageLabelContainer, bool) {
	for _, l := range labels {
		if !l.isEmpty() {
			return pageLabelContainer{labels: labels}, true
		}
	}
	return pageLabelContainer{}, false
}

// offsetsFollow compares the offsets of two page labels as
// PageLabelContainer::serialize in krilla src/page.rs does: the offset of the
// previous label with the offset of the label plus 1, where a missing offset
// is None and the sum wraps as a u32 of a release build of Rust. Upstream
// compares the offset of the previous label with the next number of the
// label, not the other way, and the port keeps that.
func offsetsFollow(prev, label PageLabel) bool {
	if prev.offset == 0 || label.offset == 0 {
		return prev.offset == 0 && label.offset == 0
	}
	return prev.offset == label.offset+1
}

// serialize is PageLabelContainer::serialize in krilla src/page.rs. It writes
// the number tree of the labels, with an entry for each page whose label
// does not continue the label of the page before it.
func (c pageLabelContainer) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	// Will always contain at least one entry, since we ensured that a
	// pageLabelContainer cannot be empty.
	type entry struct {
		pageNum int
		label   PageLabel
	}
	var filteredEntries []entry
	var prev *PageLabel

	for i, label := range c.labels {
		if prev != nil {
			if prev.style != label.style || prev.hasStyle != label.hasStyle ||
				prev.prefix != label.prefix || prev.hasPrefix != label.hasPrefix ||
				!offsetsFollow(*prev, label) {
				filteredEntries = append(filteredEntries, entry{i, label})
				prev = &c.labels[i]
			}
		} else {
			filteredEntries = append(filteredEntries, entry{i, label})
			prev = &c.labels[i]
		}
	}

	chunk := sc.newChunk()
	numTree := pdfwriter.Start[pdfwriter.NumberTree[pdfwriter.Ref]](chunk.Indirect(rootRef))
	nums := numTree.Nums()

	for _, e := range filteredEntries {
		labelRef := sc.registerPageLabel(cc, e.label)
		nums.Insert(int32(e.pageNum), labelRef)
	}

	nums.Finish()
	numTree.Finish()
	cc.nonStream.pageLabelTree = &refChunk{ref: rootRef, chunk: chunk}
}
