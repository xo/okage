// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"fmt"
)

// This file ports the part of pdf-writer src/structure.rs that krilla calls:
// the catalog, the viewer preferences, the page labels, the document
// information, the page tree, the pages, the outline, the names, the
// destinations and the metadata stream. It also holds the methods of
// src/chunk.rs and src/lib.rs that start these writers.
//
// The writers of the structure tree, the marked content and the namespaces
// are for tagged PDF, which krilla writes only with tagging. They have no
// port. A method that starts a writer of another part that has no port,
// such as Page::transition or Catalog::form, has no port either.

// Catalog is Catalog in pdf-writer src/structure.rs.
//
// Writer for a document catalog dictionary.
//
// This struct is created by Pdf.Catalog.
type Catalog struct {
	*Dict
}

// start is the macro writer! for Catalog in pdf-writer src/structure.rs.
func (c *Catalog) start(obj Obj) {
	c.Dict = obj.Dict()
	c.Pair(Name("Type"), Name("Catalog"))
}

// Pages is Catalog::pages in pdf-writer src/structure.rs.
//
// Write the /Pages attribute pointing to the root page tree.
func (c *Catalog) Pages(id Ref) *Catalog {
	c.Pair(Name("Pages"), id)
	return c
}

// PageLayout is Catalog::page_layout in pdf-writer src/structure.rs.
//
// Write the /PageLayout attribute to determine how the viewer will display
// the document's pages.
func (c *Catalog) PageLayout(layout PageLayout) *Catalog {
	c.Pair(Name("PageLayout"), layout.toName())
	return c
}

// PageLabels is Catalog::page_labels in pdf-writer src/structure.rs.
//
// Start writing the /PageLabels number tree. PDF 1.3+.
func (c *Catalog) PageLabels() *NumberTree[Ref] {
	return Start[NumberTree[Ref]](c.Insert(Name("PageLabels")))
}

// PageMode is Catalog::page_mode in pdf-writer src/structure.rs.
//
// Write the /PageMode attribute to set which chrome elements the viewer
// should show.
func (c *Catalog) PageMode(mode PageMode) *Catalog {
	c.Pair(Name("PageMode"), mode.toName())
	return c
}

// ViewerPreferences is Catalog::viewer_preferences in pdf-writer
// src/structure.rs.
//
// Start writing the /ViewerPreferences dictionary. PDF 1.2+.
func (c *Catalog) ViewerPreferences() *ViewerPreferences {
	return Start[ViewerPreferences](c.Insert(Name("ViewerPreferences")))
}

// Names is Catalog::names in pdf-writer src/structure.rs.
//
// Start writing the /Names dictionary. PDF 1.2+.
func (c *Catalog) Names() *Names {
	return Start[Names](c.Insert(Name("Names")))
}

// Destinations is Catalog::destinations in pdf-writer src/structure.rs.
//
// Write the /Dests attribute pointing to a named destinations dictionary
// (Chunk.Destinations). PDF 1.1+.
func (c *Catalog) Destinations(id Ref) *Catalog {
	c.Pair(Name("Dests"), id)
	return c
}

// Outlines is Catalog::outlines in pdf-writer src/structure.rs.
//
// Write the /Outlines attribute pointing to the root outline dictionary
// (Outline).
func (c *Catalog) Outlines(id Ref) *Catalog {
	c.Pair(Name("Outlines"), id)
	return c
}

// Lang is Catalog::lang in pdf-writer src/structure.rs.
//
// Write the /Lang attribute to specify the language of the document as a
// RFC 3066 language tag. PDF 1.4+.
//
// Required in some PDF/A profiles like PDF/A-2a.
func (c *Catalog) Lang(lang TextStr) *Catalog {
	c.Pair(Name("Lang"), lang)
	return c
}

// Version is Catalog::version in pdf-writer src/structure.rs.
//
// Write the /Version attribute to override the PDF version stated in the
// header. PDF 1.4+.
func (c *Catalog) Version(major, minor uint8) *Catalog {
	c.Pair(Name("Version"), Name(fmt.Sprintf("%d.%d", major, minor)))
	return c
}

// Metadata is Catalog::metadata in pdf-writer src/structure.rs.
//
// Write the /Metadata attribute to specify the document's metadata. PDF
// 1.4+.
//
// The reference shall point to a metadata stream (Metadata).
func (c *Catalog) Metadata(id Ref) *Catalog {
	c.Pair(Name("Metadata"), id)
	return c
}

// Catalog is Pdf::catalog in pdf-writer src/lib.rs.
//
// Start writing the document catalog. Required.
//
// This will also register the document catalog with the file trailer,
// meaning that you don't need to provide the given id anywhere else.
func (p *Pdf) Catalog(id Ref) *Catalog {
	return Start[Catalog](p.startCatalog(id))
}

// PageLayout is PageLayout in pdf-writer src/structure.rs. The zero value
// is PageLayoutSinglePage, the default of upstream.
//
// How the viewer should lay out the pages in the document.
type PageLayout uint8

// The values of PageLayout.
const (
	// PageLayoutSinglePage shows only a single page at a time.
	PageLayoutSinglePage PageLayout = iota
	// PageLayoutOneColumn shows a single, continuously scrolling column of
	// pages.
	PageLayoutOneColumn
	// PageLayoutTwoColumnLeft shows two continuously scrolling columns of
	// pages, laid out with odd-numbered pages on the left.
	PageLayoutTwoColumnLeft
	// PageLayoutTwoColumnRight shows two continuously scrolling columns of
	// pages, laid out with odd-numbered pages on the right (like in a
	// left-bound book).
	PageLayoutTwoColumnRight
	// PageLayoutTwoPageLeft shows only two pages at a time, laid out with
	// odd-numbered pages on the left. PDF 1.5+.
	PageLayoutTwoPageLeft
	// PageLayoutTwoPageRight shows only two pages at a time, laid out with
	// odd-numbered pages on the right (like in a left-bound book). PDF 1.5+.
	PageLayoutTwoPageRight
)

// pageLayoutNames holds the PDF name of each value of PageLayout, which is
// also its Rust name.
var pageLayoutNames = [...]Name{
	PageLayoutSinglePage:     "SinglePage",
	PageLayoutOneColumn:      "OneColumn",
	PageLayoutTwoColumnLeft:  "TwoColumnLeft",
	PageLayoutTwoColumnRight: "TwoColumnRight",
	PageLayoutTwoPageLeft:    "TwoPageLeft",
	PageLayoutTwoPageRight:   "TwoPageRight",
}

// String returns the Rust name of the layout.
func (l PageLayout) String() string {
	return string(l.toName())
}

// toName is PageLayout::to_name in pdf-writer src/structure.rs.
func (l PageLayout) toName() Name {
	if int(l) < len(pageLayoutNames) {
		return pageLayoutNames[l]
	}
	panic(fmt.Sprintf("pdfwriter: the page layout %d is not known", uint8(l)))
}

// PageMode is PageMode in pdf-writer src/structure.rs. The zero value is
// PageModeUseNone, the default of upstream.
//
// Elements of the viewer chrome that should be visible when opening the
// document.
type PageMode uint8

// The values of PageMode.
const (
	// PageModeUseNone shows neither the document outline panel nor a panel
	// with page preview images.
	PageModeUseNone PageMode = iota
	// PageModeUseOutlines shows the document outline panel.
	PageModeUseOutlines
	// PageModeUseThumbs shows a panel with page preview images.
	PageModeUseThumbs
	// PageModeFullScreen shows the document page in full screen mode, with
	// no chrome.
	PageModeFullScreen
	// PageModeUseOC shows the optional content group panel. PDF 1.5+.
	PageModeUseOC
	// PageModeUseAttachments shows the attachments panel. PDF 1.6+.
	PageModeUseAttachments
)

// pageModeNames holds the PDF name of each value of PageMode, which is also
// its Rust name.
var pageModeNames = [...]Name{
	PageModeUseNone:        "UseNone",
	PageModeUseOutlines:    "UseOutlines",
	PageModeUseThumbs:      "UseThumbs",
	PageModeFullScreen:     "FullScreen",
	PageModeUseOC:          "UseOC",
	PageModeUseAttachments: "UseAttachments",
}

// String returns the Rust name of the mode.
func (m PageMode) String() string {
	return string(m.toName())
}

// toName is PageMode::to_name in pdf-writer src/structure.rs.
func (m PageMode) toName() Name {
	if int(m) < len(pageModeNames) {
		return pageModeNames[m]
	}
	panic(fmt.Sprintf("pdfwriter: the page mode %d is not known", uint8(m)))
}

// ViewerPreferences is ViewerPreferences in pdf-writer src/structure.rs.
//
// Writer for a viewer preference dictionary.
//
// This struct is created by Catalog.ViewerPreferences.
type ViewerPreferences struct {
	*Dict
}

// start is the macro writer! for ViewerPreferences in pdf-writer
// src/structure.rs.
func (v *ViewerPreferences) start(obj Obj) {
	v.Dict = obj.Dict()
}

// HideToolbar is ViewerPreferences::hide_toolbar in pdf-writer
// src/structure.rs.
//
// Write the /HideToolbar attribute to set whether the viewer should hide its
// toolbars while the document is open.
func (v *ViewerPreferences) HideToolbar(hide bool) *ViewerPreferences {
	v.Pair(Name("HideToolbar"), Bool(hide))
	return v
}

// HideMenubar is ViewerPreferences::hide_menubar in pdf-writer
// src/structure.rs.
//
// Write the /HideMenubar attribute to set whether the viewer should hide its
// menu bar while the document is open.
func (v *ViewerPreferences) HideMenubar(hide bool) *ViewerPreferences {
	v.Pair(Name("HideMenubar"), Bool(hide))
	return v
}

// FitWindow is ViewerPreferences::fit_window in pdf-writer
// src/structure.rs.
//
// Write the /FitWindow attribute to set whether the viewer should resize its
// window to the size of the first page.
func (v *ViewerPreferences) FitWindow(fit bool) *ViewerPreferences {
	v.Pair(Name("FitWindow"), Bool(fit))
	return v
}

// CenterWindow is ViewerPreferences::center_window in pdf-writer
// src/structure.rs.
//
// Write the /CenterWindow attribute to set whether the viewer should center
// its window on the screen.
func (v *ViewerPreferences) CenterWindow(center bool) *ViewerPreferences {
	v.Pair(Name("CenterWindow"), Bool(center))
	return v
}

// DisplayDocTitle is ViewerPreferences::display_doc_title in pdf-writer
// src/structure.rs.
//
// Write the /DisplayDocTitle attribute to set whether the viewer should
// display the document's title from the Title entry as the window's title.
// PDF 1.4+.
func (v *ViewerPreferences) DisplayDocTitle(display bool) *ViewerPreferences {
	v.Pair(Name("DisplayDocTitle"), Bool(display))
	return v
}

// NonFullScreenPageMode is ViewerPreferences::non_full_screen_page_mode in
// pdf-writer src/structure.rs.
//
// Write the /NonFullScreenPageMode attribute to set which chrome elements the
// viewer should show for a document which requests full screen rendering in
// its catalog when it is not shown in full screen mode.
//
// Panics if mode is PageModeFullScreen.
func (v *ViewerPreferences) NonFullScreenPageMode(mode PageMode) *ViewerPreferences {
	if mode == PageModeFullScreen {
		panic("mode must not full screen")
	}
	v.Pair(Name("NonFullScreenPageMode"), mode.toName())
	return v
}

// Direction is ViewerPreferences::direction in pdf-writer src/structure.rs.
//
// Write the /Direction attribute to aid the viewer in how to lay out the
// pages visually. PDF 1.3+.
func (v *ViewerPreferences) Direction(dir Direction) *ViewerPreferences {
	v.Pair(Name("Direction"), dir.toName())
	return v
}

// Direction is Direction in pdf-writer src/structure.rs. The zero value is
// DirectionL2R, the default of upstream.
//
// Predominant reading order of text.
//
// Used to aid the viewer with the special ordering in which to display
// pages.
type Direction uint8

// The values of Direction.
const (
	// DirectionL2R is left-to-right.
	DirectionL2R Direction = iota
	// DirectionR2L is right-to-left as well as vertical writing systems.
	DirectionR2L
)

// String returns the Rust name of the direction.
func (d Direction) String() string {
	return string(d.toName())
}

// toName is Direction::to_name in pdf-writer src/structure.rs.
func (d Direction) toName() Name {
	switch d {
	case DirectionL2R:
		return Name("L2R")
	case DirectionR2L:
		return Name("R2L")
	}
	panic(fmt.Sprintf("pdfwriter: the direction %d is not known", uint8(d)))
}

// PageLabel is PageLabel in pdf-writer src/structure.rs.
//
// Writer for a page label dictionary.
type PageLabel struct {
	*Dict
}

// start is the macro writer! for PageLabel in pdf-writer src/structure.rs.
func (l *PageLabel) start(obj Obj) {
	l.Dict = obj.Dict()
	l.Pair(Name("Type"), Name("PageLabel"))
}

// Style is PageLabel::style in pdf-writer src/structure.rs.
//
// Write the /S attribute to set the page label's numbering style.
//
// If this attribute is omitted, only the prefix will be used, there will be
// no page number.
func (l *PageLabel) Style(style NumberingStyle) *PageLabel {
	l.Pair(Name("S"), style.toName())
	return l
}

// Prefix is PageLabel::prefix in pdf-writer src/structure.rs.
//
// Write the /P attribute to set the page label's prefix.
func (l *PageLabel) Prefix(prefix TextStrLike) *PageLabel {
	l.Pair(Name("P"), prefix)
	return l
}

// Offset is PageLabel::offset in pdf-writer src/structure.rs.
//
// Write the /St attribute to set the page label's offset.
//
// This must be greater or equal to 1 if set.
func (l *PageLabel) Offset(offset int32) *PageLabel {
	l.Pair(Name("St"), Int(offset))
	return l
}

// NumberingStyle is NumberingStyle in pdf-writer src/structure.rs.
//
// The numbering style of a page label.
type NumberingStyle uint8

// The values of NumberingStyle.
const (
	// NumberingStyleArabic is Arabic numerals.
	NumberingStyleArabic NumberingStyle = iota
	// NumberingStyleLowerRoman is lowercase Roman numerals.
	NumberingStyleLowerRoman
	// NumberingStyleUpperRoman is uppercase Roman numerals.
	NumberingStyleUpperRoman
	// NumberingStyleLowerAlpha is lowercase letters (a-z, then aa-zz, ...).
	NumberingStyleLowerAlpha
	// NumberingStyleUpperAlpha is uppercase letters (A-Z, then AA-ZZ, ...).
	NumberingStyleUpperAlpha
)

// numberingStyleNames holds the Rust name of each value of NumberingStyle.
var numberingStyleNames = [...]string{
	NumberingStyleArabic:     "Arabic",
	NumberingStyleLowerRoman: "LowerRoman",
	NumberingStyleUpperRoman: "UpperRoman",
	NumberingStyleLowerAlpha: "LowerAlpha",
	NumberingStyleUpperAlpha: "UpperAlpha",
}

// String returns the Rust name of the style.
func (s NumberingStyle) String() string {
	if int(s) < len(numberingStyleNames) {
		return numberingStyleNames[s]
	}
	return "NumberingStyle(?)"
}

// toName is NumberingStyle::to_name in pdf-writer src/structure.rs.
func (s NumberingStyle) toName() Name {
	switch s {
	case NumberingStyleArabic:
		return Name("D")
	case NumberingStyleLowerRoman:
		return Name("r")
	case NumberingStyleUpperRoman:
		return Name("R")
	case NumberingStyleLowerAlpha:
		return Name("a")
	case NumberingStyleUpperAlpha:
		return Name("A")
	}
	panic(fmt.Sprintf("pdfwriter: the numbering style %d is not known", uint8(s)))
}

// DocumentInfo is DocumentInfo in pdf-writer src/structure.rs.
//
// Writer for a document information dictionary.
//
// This struct is created by Pdf.DocumentInfo.
type DocumentInfo struct {
	*Dict
}

// start is the macro writer! for DocumentInfo in pdf-writer
// src/structure.rs.
func (i *DocumentInfo) start(obj Obj) {
	i.Dict = obj.Dict()
}

// Title is DocumentInfo::title in pdf-writer src/structure.rs.
//
// Write the /Title attribute to set the document's title. PDF 1.1+.
func (i *DocumentInfo) Title(title TextStrLike) *DocumentInfo {
	i.Pair(Name("Title"), title)
	return i
}

// Author is DocumentInfo::author in pdf-writer src/structure.rs.
//
// Write the /Author attribute to set the document's author.
func (i *DocumentInfo) Author(author TextStrLike) *DocumentInfo {
	i.Pair(Name("Author"), author)
	return i
}

// Subject is DocumentInfo::subject in pdf-writer src/structure.rs.
//
// Write the /Subject attribute to set the document's subject. PDF 1.1+.
func (i *DocumentInfo) Subject(subject TextStrLike) *DocumentInfo {
	i.Pair(Name("Subject"), subject)
	return i
}

// Keywords is DocumentInfo::keywords in pdf-writer src/structure.rs.
//
// Write the /Keywords attribute to set terms associated to the document.
// PDF 1.1+.
func (i *DocumentInfo) Keywords(keywords TextStrLike) *DocumentInfo {
	i.Pair(Name("Keywords"), keywords)
	return i
}

// Creator is DocumentInfo::creator in pdf-writer src/structure.rs.
//
// Write the /Creator attribute to set the name of the product that converted
// or wrote the file that this PDF has been converted from.
func (i *DocumentInfo) Creator(creator TextStrLike) *DocumentInfo {
	i.Pair(Name("Creator"), creator)
	return i
}

// Producer is DocumentInfo::producer in pdf-writer src/structure.rs.
//
// Write the /Producer attribute to set the name of the product that
// converted or wrote this PDF.
func (i *DocumentInfo) Producer(producer TextStrLike) *DocumentInfo {
	i.Pair(Name("Producer"), producer)
	return i
}

// CreationDate is DocumentInfo::creation_date in pdf-writer
// src/structure.rs.
//
// Write the /CreationDate attribute to set the date the document was
// created.
func (i *DocumentInfo) CreationDate(date Date) *DocumentInfo {
	i.Pair(Name("CreationDate"), date)
	return i
}

// ModifiedDate is DocumentInfo::modified_date in pdf-writer
// src/structure.rs.
//
// Write the /ModDate attribute to set the date the document was last
// modified.
//
// Required if /PieceInfo is set in the document catalog.
func (i *DocumentInfo) ModifiedDate(date Date) *DocumentInfo {
	i.Pair(Name("ModDate"), date)
	return i
}

// Trapped is DocumentInfo::trapped in pdf-writer src/structure.rs.
//
// Write the /Trapped attribute to set whether the document is fully or
// partially trapped. PDF 1.3+.
func (i *DocumentInfo) Trapped(trapped TrappingStatus) *DocumentInfo {
	i.Pair(Name("Trapped"), trapped.toName())
	return i
}

// DocumentInfo is Pdf::document_info in pdf-writer src/lib.rs.
//
// Start writing the document information.
//
// This will also register the document information dictionary with the
// file trailer, meaning that you don't need to provide the given id
// anywhere else.
func (p *Pdf) DocumentInfo(id Ref) *DocumentInfo {
	return Start[DocumentInfo](p.startDocumentInfo(id))
}

// TrappingStatus is TrappingStatus in pdf-writer src/structure.rs. The zero
// value is TrappingStatusTrapped. The default of upstream is
// TrappingStatusUnknown.
//
// Whether a document has been adjusted with traps.
//
// Those account for colorant misregistration during the printing process.
type TrappingStatus uint8

// The values of TrappingStatus.
const (
	// TrappingStatusTrapped means that the document is fully trapped.
	TrappingStatusTrapped TrappingStatus = iota
	// TrappingStatusNotTrapped means that the document has not been
	// trapped.
	TrappingStatusNotTrapped
	// TrappingStatusUnknown means that the document is partially trapped or
	// the trapping status is unknown.
	TrappingStatusUnknown
)

// trappingStatusNames holds the Rust name of each value of TrappingStatus.
var trappingStatusNames = [...]string{
	TrappingStatusTrapped:    "Trapped",
	TrappingStatusNotTrapped: "NotTrapped",
	TrappingStatusUnknown:    "Unknown",
}

// String returns the Rust name of the status.
func (s TrappingStatus) String() string {
	if int(s) < len(trappingStatusNames) {
		return trappingStatusNames[s]
	}
	return "TrappingStatus(?)"
}

// toName is TrappingStatus::to_name in pdf-writer src/structure.rs.
func (s TrappingStatus) toName() Name {
	switch s {
	case TrappingStatusTrapped:
		return Name("True")
	case TrappingStatusNotTrapped:
		return Name("False")
	case TrappingStatusUnknown:
		return Name("Unknown")
	}
	panic(fmt.Sprintf("pdfwriter: the trapping status %d is not known", uint8(s)))
}

// Pages is Pages in pdf-writer src/structure.rs.
//
// Writer for a page tree dictionary.
//
// This struct is created by Chunk.Pages.
type Pages struct {
	*Dict
}

// start is the macro writer! for Pages in pdf-writer src/structure.rs.
func (p *Pages) start(obj Obj) {
	p.Dict = obj.Dict()
	p.Pair(Name("Type"), Name("Pages"))
}

// Parent is Pages::parent in pdf-writer src/structure.rs.
//
// Write the /Parent attribute. Required except in root node.
func (p *Pages) Parent(parent Ref) *Pages {
	p.Pair(Name("Parent"), parent)
	return p
}

// Kids is Pages::kids in pdf-writer src/structure.rs.
//
// Write the /Kids attributes, listing the immediate children of this node in
// the page tree. Required.
func (p *Pages) Kids(kids ...Ref) *Pages {
	NewTypedArray[Ref](p.Insert(Name("Kids")).Array()).Items(kids...)
	return p
}

// Count is Pages::count in pdf-writer src/structure.rs.
//
// Write the /Count attribute, specifying how many descendants this node in
// the page tree has. This may be different to the length of /Kids when the
// tree has multiple layers. Required.
func (p *Pages) Count(count int32) *Pages {
	p.Pair(Name("Count"), Int(count))
	return p
}

// MediaBox is Pages::media_box in pdf-writer src/structure.rs.
//
// Write the /MediaBox attribute.
func (p *Pages) MediaBox(rect Rect) *Pages {
	p.Pair(Name("MediaBox"), rect)
	return p
}

// Resources is Pages::resources in pdf-writer src/structure.rs.
//
// Start writing the /Resources dictionary.
func (p *Pages) Resources() *Resources {
	return Start[Resources](p.Insert(Name("Resources")))
}

// Pages is Chunk::pages in pdf-writer src/chunk.rs.
//
// Start writing a page tree.
func (c *Chunk) Pages(id Ref) *Pages {
	return Start[Pages](c.Indirect(id))
}

// Page is Page in pdf-writer src/structure.rs.
//
// Writer for a page dictionary.
//
// This struct is created by Chunk.Page.
type Page struct {
	*Dict
}

// start is the macro writer! for Page in pdf-writer src/structure.rs.
func (p *Page) start(obj Obj) {
	p.Dict = obj.Dict()
	p.Pair(Name("Type"), Name("Page"))
}

// Parent is Page::parent in pdf-writer src/structure.rs.
//
// Write the /Parent attribute. Required.
func (p *Page) Parent(parent Ref) *Page {
	p.Pair(Name("Parent"), parent)
	return p
}

// LastModified is Page::last_modified in pdf-writer src/structure.rs.
//
// Write the /LastModified attribute. PDF 1.3+.
func (p *Page) LastModified(date Date) *Page {
	p.Pair(Name("LastModified"), date)
	return p
}

// MediaBox is Page::media_box in pdf-writer src/structure.rs.
//
// Write the /MediaBox attribute. This is the size of the physical medium the
// page gets printed onto.
func (p *Page) MediaBox(rect Rect) *Page {
	p.Pair(Name("MediaBox"), rect)
	return p
}

// CropBox is Page::crop_box in pdf-writer src/structure.rs.
//
// Write the /CropBox attribute. This is the size of the area within which
// content is visible.
func (p *Page) CropBox(rect Rect) *Page {
	p.Pair(Name("CropBox"), rect)
	return p
}

// BleedBox is Page::bleed_box in pdf-writer src/structure.rs.
//
// Write the /BleedBox attribute. This is the size of the area within which
// content is visible in a print production environment. Most
// production-aiding marks should be outside of this box. PDF 1.3+.
func (p *Page) BleedBox(rect Rect) *Page {
	p.Pair(Name("BleedBox"), rect)
	return p
}

// TrimBox is Page::trim_box in pdf-writer src/structure.rs.
//
// Write the /TrimBox attribute. This is the size of the produced document
// after trimming is applied. PDF 1.3+.
func (p *Page) TrimBox(rect Rect) *Page {
	p.Pair(Name("TrimBox"), rect)
	return p
}

// ArtBox is Page::art_box in pdf-writer src/structure.rs.
//
// Write the /ArtBox attribute. This is the area that another program
// importing this file should use. PDF 1.3+.
func (p *Page) ArtBox(rect Rect) *Page {
	p.Pair(Name("ArtBox"), rect)
	return p
}

// Resources is Page::resources in pdf-writer src/structure.rs.
//
// Start writing the /Resources dictionary.
func (p *Page) Resources() *Resources {
	return Start[Resources](p.Insert(Name("Resources")))
}

// Contents is Page::contents in pdf-writer src/structure.rs.
//
// Write the /Contents attribute as reference to a single content stream.
//
// Such a content stream can be created using the Content builder and
// written to the file using Chunk.Stream.
func (p *Page) Contents(id Ref) *Page {
	p.Pair(Name("Contents"), id)
	return p
}

// ContentsArray is Page::contents_array in pdf-writer src/structure.rs.
//
// Write the /Contents attribute as an array.
func (p *Page) ContentsArray(ids ...Ref) *Page {
	NewTypedArray[Ref](p.Insert(Name("Contents")).Array()).Items(ids...)
	return p
}

// Rotate is Page::rotate in pdf-writer src/structure.rs.
//
// Write the /Rotate attribute. This is the number of degrees the page should
// be rotated clockwise when displayed. This should be a multiple of 90.
func (p *Page) Rotate(degrees int32) *Page {
	p.Pair(Name("Rotate"), Int(degrees))
	return p
}

// Thumbnail is Page::thumbnail in pdf-writer src/structure.rs.
//
// Write the /Thumb attribute to set an image (ImageXObject) as the page
// thumbnail. Must be RGB, Grayscale, or an indexed color space based on the
// former two.
func (p *Page) Thumbnail(id Ref) *Page {
	p.Pair(Name("Thumb"), id)
	return p
}

// Duration is Page::duration in pdf-writer src/structure.rs.
//
// Write the /Dur attribute. This is the amount of seconds the page should be
// displayed before advancing to the next one. PDF 1.1+.
func (p *Page) Duration(seconds float32) *Page {
	p.Pair(Name("Dur"), Real(seconds))
	return p
}

// Annotations is Page::annotations in pdf-writer src/structure.rs.
//
// Write the /Annots (annotations) array.
func (p *Page) Annotations(ids ...Ref) *Page {
	NewTypedArray[Ref](p.Insert(Name("Annots")).Array()).Items(ids...)
	return p
}

// StructParents is Page::struct_parents in pdf-writer src/structure.rs.
//
// Write the /StructParents attribute to indicate the structure tree
// elements the contents of this XObject may belong to. PDF 1.3+.
func (p *Page) StructParents(key int32) *Page {
	p.Pair(Name("StructParents"), Int(key))
	return p
}

// TabOrder is Page::tab_order in pdf-writer src/structure.rs.
//
// Write the /Tabs attribute. This specifies the order in which the
// annotations should be focused by hitting tab. PDF 1.5+.
func (p *Page) TabOrder(order TabOrder) *Page {
	p.Pair(Name("Tabs"), order.toName())
	return p
}

// UserUnit is Page::user_unit in pdf-writer src/structure.rs.
//
// Write the /UserUnit attribute. This sets how large a user space unit is in
// printer's points (1/72 inch). This defaults to 1.0. PDF 1.6+.
func (p *Page) UserUnit(value float32) *Page {
	p.Pair(Name("UserUnit"), Real(value))
	return p
}

// Metadata is Page::metadata in pdf-writer src/structure.rs.
//
// Write the /Metadata attribute to specify the page's metadata. PDF 1.4+.
//
// The reference shall point to a metadata stream (Metadata).
//
// Required in PDF/A.
func (p *Page) Metadata(id Ref) *Page {
	p.Pair(Name("Metadata"), id)
	return p
}

// Page is Chunk::page in pdf-writer src/chunk.rs.
//
// Start writing a page.
func (c *Chunk) Page(id Ref) *Page {
	return Start[Page](c.Indirect(id))
}

// Outline is Outline in pdf-writer src/structure.rs.
//
// Writer for an outline dictionary.
//
// This struct is created by Chunk.Outline.
type Outline struct {
	*Dict
}

// start is the macro writer! for Outline in pdf-writer src/structure.rs.
func (o *Outline) start(obj Obj) {
	o.Dict = obj.Dict()
	o.Pair(Name("Type"), Name("Outlines"))
}

// First is Outline::first in pdf-writer src/structure.rs.
//
// Write the /First attribute which points to the first item (OutlineItem) in
// the document's outline.
func (o *Outline) First(item Ref) *Outline {
	o.Pair(Name("First"), item)
	return o
}

// Last is Outline::last in pdf-writer src/structure.rs.
//
// Write the /Last attribute which points to the last item (OutlineItem) in
// the document's outline.
func (o *Outline) Last(item Ref) *Outline {
	o.Pair(Name("Last"), item)
	return o
}

// Count is Outline::count in pdf-writer src/structure.rs.
//
// Write the /Count attribute. This tells the viewer how many outline
// elements (at all levels) are currently visible.
//
// Panics if count is negative.
func (o *Outline) Count(count int32) *Outline {
	if count < 0 {
		panic("visible outline count must not be negative")
	}
	o.Pair(Name("Count"), Int(count))
	return o
}

// Outline is Chunk::outline in pdf-writer src/chunk.rs.
//
// Start writing an outline.
func (c *Chunk) Outline(id Ref) *Outline {
	return Start[Outline](c.Indirect(id))
}

// OutlineItem is OutlineItem in pdf-writer src/structure.rs.
//
// Writer for an outline item dictionary.
//
// This struct is created by Chunk.OutlineItem.
type OutlineItem struct {
	*Dict
}

// start is the macro writer! for OutlineItem in pdf-writer
// src/structure.rs.
func (o *OutlineItem) start(obj Obj) {
	o.Dict = obj.Dict()
}

// Title is OutlineItem::title in pdf-writer src/structure.rs.
//
// Write the /Title attribute.
func (o *OutlineItem) Title(title TextStrLike) *OutlineItem {
	o.Pair(Name("Title"), title)
	return o
}

// Parent is OutlineItem::parent in pdf-writer src/structure.rs.
//
// Write the /Parent attribute which points to the item's parent or the
// top-level outline dictionary.
func (o *OutlineItem) Parent(outline Ref) *OutlineItem {
	o.Pair(Name("Parent"), outline)
	return o
}

// Prev is OutlineItem::prev in pdf-writer src/structure.rs.
//
// Write the /Prev attribute which points to the previous item on the item's
// level.
func (o *OutlineItem) Prev(outline Ref) *OutlineItem {
	o.Pair(Name("Prev"), outline)
	return o
}

// Next is OutlineItem::next in pdf-writer src/structure.rs.
//
// Write the /Next attribute which points to the next item on the item's
// level.
func (o *OutlineItem) Next(outline Ref) *OutlineItem {
	o.Pair(Name("Next"), outline)
	return o
}

// First is OutlineItem::first in pdf-writer src/structure.rs.
//
// Write the /First attribute which points to the item's first child.
func (o *OutlineItem) First(outline Ref) *OutlineItem {
	o.Pair(Name("First"), outline)
	return o
}

// Last is OutlineItem::last in pdf-writer src/structure.rs.
//
// Write the /Last attribute which points to the item's last child.
func (o *OutlineItem) Last(outline Ref) *OutlineItem {
	o.Pair(Name("Last"), outline)
	return o
}

// Count is OutlineItem::count in pdf-writer src/structure.rs.
//
// Write the /Count attribute. This tells the viewer how many outline element
// children are currently visible. If the item is collapsed, this number
// shall be negative indicating how many elements you would be able to see if
// it was open.
func (o *OutlineItem) Count(items int32) *OutlineItem {
	o.Pair(Name("Count"), Int(items))
	return o
}

// Dest is OutlineItem::dest in pdf-writer src/structure.rs.
//
// Start writing the /Dest attribute to set the destination of this outline
// item.
func (o *OutlineItem) Dest() *Destination {
	return Start[Destination](o.Insert(Name("Dest")))
}

// DestName is OutlineItem::dest_name in pdf-writer src/structure.rs.
//
// Write the /Dest attribute to set the destination of this outline item to
// a named destination.
func (o *OutlineItem) DestName(name Name) *OutlineItem {
	o.Pair(Name("Dest"), name)
	return o
}

// ColorRGB is OutlineItem::color_rgb in pdf-writer src/structure.rs.
//
// Write the /C attribute using an RGB color. This sets the color in which
// the outline item's title should be rendered. PDF 1.4+.
func (o *OutlineItem) ColorRGB(r, g, b float32) *OutlineItem {
	o.Insert(Name("C")).Array().Items(Real(r), Real(g), Real(b))
	return o
}

// Flags is OutlineItem::flags in pdf-writer src/structure.rs.
//
// Write the /F attribute. PDF 1.4+.
func (o *OutlineItem) Flags(flags OutlineItemFlags) *OutlineItem {
	o.Pair(Name("F"), Int(int32(flags)))
	return o
}

// OutlineItem is Chunk::outline_item in pdf-writer src/chunk.rs.
//
// Start writing an outline item.
func (c *Chunk) OutlineItem(id Ref) *OutlineItem {
	return Start[OutlineItem](c.Indirect(id))
}

// OutlineItemFlags is OutlineItemFlags in pdf-writer src/structure.rs, a
// set of bit flags.
//
// Bitflags describing the appearance of an outline item.
type OutlineItemFlags uint32

// The flags of OutlineItemFlags.
const (
	// OutlineItemFlagsItalic renders the outline item italicized.
	OutlineItemFlagsItalic OutlineItemFlags = 1 << 0
	// OutlineItemFlagsBold renders the outline item emboldened.
	OutlineItemFlagsBold OutlineItemFlags = 1 << 1
)

// Names is Names in pdf-writer src/structure.rs.
//
// Writer for a names dictionary.
//
// This dictionary can map various objects to names using name trees. This
// struct is created by Catalog.Names.
type Names struct {
	*Dict
}

// start is the macro writer! for Names in pdf-writer src/structure.rs.
func (n *Names) start(obj Obj) {
	n.Dict = obj.Dict()
}

// Destinations is Names::destinations in pdf-writer src/structure.rs.
//
// Start writing the /Dests attribute to provide associations for
// destinations (Destination).
func (n *Names) Destinations() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("Dests")))
}

// Appearances is Names::appearances in pdf-writer src/structure.rs.
//
// Start writing the /AP attribute to provide associations for appearance
// streams. PDF 1.3+.
func (n *Names) Appearances() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("AP")))
}

// JavaScript is Names::javascript in pdf-writer src/structure.rs.
//
// Start writing the /JavaScript attribute to provide associations for
// JavaScript actions. PDF 1.3+.
func (n *Names) JavaScript() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("JavaScript")))
}

// Pages is Names::pages in pdf-writer src/structure.rs.
//
// Start writing the /Pages attribute to name pages (Page). PDF 1.3+.
func (n *Names) Pages() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("Pages")))
}

// Templates is Names::templates in pdf-writer src/structure.rs.
//
// Start writing the /Template attribute to name pages (Pages) outside of
// the page tree as templates for interactive forms. PDF 1.3+.
func (n *Names) Templates() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("Templates")))
}

// CaptureIDs is Names::capture_ids in pdf-writer src/structure.rs.
//
// Start writing the /IDS attribute to map identifiers to Web Capture content
// sets. PDF 1.3+.
func (n *Names) CaptureIDs() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("IDS")))
}

// CaptureURLs is Names::capture_urls in pdf-writer src/structure.rs.
//
// Start writing the /URLS attribute to map URLs to Web Capture content sets.
// PDF 1.3+.
func (n *Names) CaptureURLs() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("URLS")))
}

// EmbeddedFiles is Names::embedded_files in pdf-writer src/structure.rs.
//
// Start writing the /EmbeddedFiles attribute to name embedded files
// (EmbeddedFile). PDF 1.4+.
//
// Note that this key is forbidden in PDF/A-1, and restricted in PDF/A-2 and
// PDF/A-4.
func (n *Names) EmbeddedFiles() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("EmbeddedFiles")))
}

// AlternatePresentations is Names::alternate_presentations in pdf-writer
// src/structure.rs.
//
// Start writing the /AlternatePresentations attribute to name alternate
// presentations. PDF 1.4+.
//
// Note that this key is forbidden in PDF/A.
func (n *Names) AlternatePresentations() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("AlternatePresentations")))
}

// Renditions is Names::renditions in pdf-writer src/structure.rs.
//
// Start writing the /Renditions attribute to name renditions. The names
// must conform to Unicode. PDF 1.5+.
func (n *Names) Renditions() *NameTree[Ref] {
	return Start[NameTree[Ref]](n.Insert(Name("Renditions")))
}

// Destination is Destination in pdf-writer src/structure.rs.
//
// Writer for a destination array.
//
// A dictionary mapping to this struct is created by Chunk.Destinations.
// This struct is also created by Action.Destination.
//
// Upstream takes the writer by value in each method. A method that ends the
// destination, such as XYZ, drops the writer there, so its port finishes the
// array.
type Destination struct {
	*Array
}

// start is the macro writer! for Destination in pdf-writer
// src/structure.rs.
func (d *Destination) start(obj Obj) {
	d.Array = obj.Array()
}

// Page is Destination::page in pdf-writer src/structure.rs.
//
// The target page. Required.
func (d *Destination) Page(page Ref) *Destination {
	d.Item(page)
	return d
}

// XYZ is Destination::xyz in pdf-writer src/structure.rs. A nil zoom is the
// None of upstream.
//
// Write the /XYZ command which skips to the specified coordinated.
func (d *Destination) XYZ(left, top float32, zoom *float32) {
	d.Item(Name("XYZ"))
	d.Item(Real(left))
	d.Item(Real(top))
	var z float32
	if zoom != nil {
		z = *zoom
	}
	d.Item(Real(z))
	d.Finish()
}

// Fit is Destination::fit in pdf-writer src/structure.rs.
//
// Write the /Fit command which fits all of the referenced page on screen.
func (d *Destination) Fit() {
	d.Item(Name("Fit"))
	d.Finish()
}

// FitHorizontal is Destination::fit_horizontal in pdf-writer
// src/structure.rs.
//
// Write the /FitH command which fits the referenced page to the screen width
// and skips to the specified offset.
func (d *Destination) FitHorizontal(top float32) {
	d.Item(Name("FitH"))
	d.Item(Real(top))
	d.Finish()
}

// FitVertical is Destination::fit_vertical in pdf-writer src/structure.rs.
//
// Write the /FitV command which fits the referenced page to the screen
// height and skips to the specified offset.
func (d *Destination) FitVertical(left float32) {
	d.Item(Name("FitV"))
	d.Item(Real(left))
	d.Finish()
}

// FitRect is Destination::fit_rect in pdf-writer src/structure.rs.
//
// Write the /FitR command which fits the rectangle argument on the screen.
func (d *Destination) FitRect(rect Rect) {
	d.Item(Name("FitR"))
	d.Items(Real(rect.X1), Real(rect.Y1), Real(rect.X2), Real(rect.Y2))
	d.Finish()
}

// FitBoundingBox is Destination::fit_bounding_box in pdf-writer
// src/structure.rs.
//
// Write the /FitB command which fits all of the referenced page's content on
// screen. PDF 1.1+.
func (d *Destination) FitBoundingBox() {
	d.Item(Name("FitB"))
	d.Finish()
}

// FitBoundingBoxHorizontal is Destination::fit_bounding_box_horizontal in
// pdf-writer src/structure.rs.
//
// Write the /FitBH command which fits the referenced page's content to the
// screen width and skips to the specified offset. PDF 1.1+.
func (d *Destination) FitBoundingBoxHorizontal(top float32) {
	d.Item(Name("FitBH"))
	d.Item(Real(top))
	d.Finish()
}

// FitBoundingBoxVertical is Destination::fit_bounding_box_vertical in
// pdf-writer src/structure.rs.
//
// Write the /FitBV command which fits the referenced page's content to the
// screen height and skips to the specified offset. PDF 1.1+.
func (d *Destination) FitBoundingBoxVertical(left float32) {
	d.Item(Name("FitBV"))
	d.Item(Real(left))
	d.Finish()
}

// Destination is Chunk::destination in pdf-writer src/chunk.rs.
//
// Start writing a destination for use in a name tree.
func (c *Chunk) Destination(id Ref) *Destination {
	return Start[Destination](c.Indirect(id))
}

// Destinations is Chunk::destinations in pdf-writer src/chunk.rs.
//
// Start writing a named destination dictionary.
func (c *Chunk) Destinations(id Ref) *WriterDict[Destination, *Destination] {
	return NewWriterDict[Destination](c.Indirect(id).Dict())
}

// TabOrder is TabOrder in pdf-writer src/structure.rs.
//
// What order to tab through the annotations on a page.
type TabOrder uint8

// The values of TabOrder.
const (
	// TabOrderRowOrder navigates the annotations horizontally, then
	// vertically.
	TabOrderRowOrder TabOrder = iota
	// TabOrderColumnOrder navigates the annotations vertically, then
	// horizontally.
	TabOrderColumnOrder
	// TabOrderStructureOrder navigates the annotations in the order they
	// appear in the structure tree.
	TabOrderStructureOrder
)

// tabOrderNames holds the Rust name of each value of TabOrder.
var tabOrderNames = [...]string{
	TabOrderRowOrder:       "RowOrder",
	TabOrderColumnOrder:    "ColumnOrder",
	TabOrderStructureOrder: "StructureOrder",
}

// String returns the Rust name of the order.
func (o TabOrder) String() string {
	if int(o) < len(tabOrderNames) {
		return tabOrderNames[o]
	}
	return "TabOrder(?)"
}

// toName is TabOrder::to_name in pdf-writer src/structure.rs.
func (o TabOrder) toName() Name {
	switch o {
	case TabOrderRowOrder:
		return Name("R")
	case TabOrderColumnOrder:
		return Name("C")
	case TabOrderStructureOrder:
		return Name("S")
	}
	panic(fmt.Sprintf("pdfwriter: the tab order %d is not known", uint8(o)))
}

// Metadata is Metadata in pdf-writer src/structure.rs.
//
// Writer for a metadata stream. PDF 1.4+.
//
// This struct is created by Chunk.Metadata.
type Metadata struct {
	*Stream
}

// startMetadata is Metadata::start in pdf-writer src/structure.rs.
//
// Create a new metadata stream writer.
func startMetadata(stream *Stream) *Metadata {
	stream.Pair(Name("Type"), Name("Metadata"))
	stream.Pair(Name("Subtype"), Name("XML"))
	return &Metadata{Stream: stream}
}

// Metadata is Chunk::metadata in pdf-writer src/chunk.rs.
//
// Start writing a metadata stream.
func (c *Chunk) Metadata(id Ref, bytes []byte) *Metadata {
	return startMetadata(c.Stream(id, bytes))
}

// Group is Page::group in pdf-writer src/structure.rs.
//
// Start writing the /Group dictionary.
func (p *Page) Group() *Group {
	return Start[Group](p.Insert(Name("Group")))
}
