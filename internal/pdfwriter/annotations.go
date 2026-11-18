// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import (
	"fmt"
)

// This file ports the part of pdf-writer src/annotations.rs that krilla
// calls for a link annotation: the writer Annotation, with
// Chunk::annotation of src/chunk.rs, the types of annotations, their flags
// and the effects of a highlight.
//
// krilla writes the appearance, the appearance characteristics and the
// additional actions only for the widgets of a form, which have no port. So
// the writers Appearance, AppearanceEntry, AppearanceCharacteristics,
// IconFit and BorderStyle have no port, nor the methods that start them or a
// file specification. The icon of Annotation::icon has no port either.

// Annotation is Annotation in pdf-writer src/annotations.rs.
//
// Writer for an annotation dictionary.
//
// An array of this struct is created by Chunk.Annotation.
type Annotation struct {
	*Dict
}

// start is the macro writer! for Annotation in pdf-writer
// src/annotations.rs.
func (a *Annotation) start(obj Obj) {
	a.Dict = obj.Dict()
	a.Pair(Name("Type"), Name("Annot"))
}

// Subtype is Annotation::subtype in pdf-writer src/annotations.rs.
//
// Write the /Subtype attribute to tell the viewer the type of this
// particular annotation.
func (a *Annotation) Subtype(kind AnnotationType) *Annotation {
	a.Pair(Name("Subtype"), kind.toName())
	return a
}

// Rect is Annotation::rect in pdf-writer src/annotations.rs.
//
// Write the /Rect attribute. This is the location and dimensions of the
// annotation on the page.
func (a *Annotation) Rect(rect Rect) *Annotation {
	a.Pair(Name("Rect"), rect)
	return a
}

// Contents is Annotation::contents in pdf-writer src/annotations.rs.
//
// Write the /Contents attribute. This is the content or alt-text, depending
// on the AnnotationType.
func (a *Annotation) Contents(text TextStrLike) *Annotation {
	a.Pair(Name("Contents"), text)
	return a
}

// Page is Annotation::page in pdf-writer src/annotations.rs.
//
// Write the /P attribute. This provides an indirect reference to the page
// object with which this annotation is associated. Required for the subtype
// Screen associated with rendition actions. PDF 1.3+.
func (a *Annotation) Page(id Ref) *Annotation {
	a.Pair(Name("P"), id)
	return a
}

// Name is Annotation::name in pdf-writer src/annotations.rs.
//
// Write the /NM attribute. This uniquely identifies the annotation on the
// page. PDF 1.3+.
func (a *Annotation) Name(text TextStrLike) *Annotation {
	a.Pair(Name("NM"), text)
	return a
}

// Modified is Annotation::modified in pdf-writer src/annotations.rs.
//
// Write the /M attribute, specifying the date the annotation was last
// modified. PDF 1.1+.
func (a *Annotation) Modified(date Date) *Annotation {
	a.Pair(Name("M"), date)
	return a
}

// Flags is Annotation::flags in pdf-writer src/annotations.rs.
//
// Write the /F attribute.
//
// Required for all annotations in PDF/A except for Popup.
func (a *Annotation) Flags(flags AnnotationFlags) *Annotation {
	a.Pair(Name("F"), Int(int32(flags)))
	return a
}

// AppearanceState is Annotation::appearance_state in pdf-writer
// src/annotations.rs.
//
// Write the /AS attribute to set the annotation's current appearance state
// from the /AP subdictionary. Must be set if the appearance dictionary has
// one or more subdictionaries. PDF 1.2+.
func (a *Annotation) AppearanceState(name Name) *Annotation {
	a.Pair(Name("AS"), name)
	return a
}

// Border is Annotation::border in pdf-writer src/annotations.rs. A nil
// dashPattern is the None of upstream, and an empty one is Some(&[]).
//
// Write the /Border attribute. This describes the look of the border around
// the annotation, including width and horizontal and vertical border radii.
// The function may also receive a dash pattern which specifies the lengths
// and gaps of the border segments on a dashed border. Although all PDF
// versions accept /Border, this feature specifically is only available in
// PDF 1.1 or later.
func (a *Annotation) Border(hRadius, vRadius, width float32, dashPattern []float32) *Annotation {
	array := a.Insert(Name("Border")).Array()
	array.Item(Real(hRadius))
	array.Item(Real(vRadius))
	array.Item(Real(width))

	if dashPattern != nil {
		pattern := array.Push().Array()
		for _, v := range dashPattern {
			pattern.Item(Real(v))
		}
	}

	array.Finish()
	return a
}

// ColorTransparent is Annotation::color_transparent in pdf-writer
// src/annotations.rs.
//
// Write the /C attribute forcing a transparent color. This sets the
// annotations background color and its popup title bar color. PDF 1.1+.
func (a *Annotation) ColorTransparent() *Annotation {
	a.Insert(Name("C")).Array()
	return a
}

// ColorGray is Annotation::color_gray in pdf-writer src/annotations.rs.
//
// Write the /C attribute using a grayscale color. This sets the annotations
// background color and its popup title bar color. PDF 1.1+.
func (a *Annotation) ColorGray(gray float32) *Annotation {
	a.Insert(Name("C")).Array().Item(Real(gray))
	return a
}

// ColorRGB is Annotation::color_rgb in pdf-writer src/annotations.rs.
//
// Write the /C attribute using an RGB color. This sets the annotations
// background color and its popup title bar color. PDF 1.1+.
func (a *Annotation) ColorRGB(r, g, b float32) *Annotation {
	a.Insert(Name("C")).Array().Items(Real(r), Real(g), Real(b))
	return a
}

// ColorCMYK is Annotation::color_cmyk in pdf-writer src/annotations.rs.
//
// Write the /C attribute using a CMYK color. This sets the annotations
// background color and its popup title bar color. PDF 1.1+.
func (a *Annotation) ColorCMYK(c, m, y, k float32) *Annotation {
	a.Insert(Name("C")).Array().Items(Real(c), Real(m), Real(y), Real(k))
	return a
}

// StructParent is Annotation::struct_parent in pdf-writer
// src/annotations.rs.
//
// Write the /StructParent attribute to indicate the structure tree element
// this annotation belongs to. PDF 1.3+.
func (a *Annotation) StructParent(key int32) *Annotation {
	a.Pair(Name("StructParent"), Int(key))
	return a
}

// Action is Annotation::action in pdf-writer src/annotations.rs.
//
// Start writing the /A dictionary. Only permissible for the subtypes Link
// and Widget.
//
// Note that this attribute is forbidden in PDF/A.
func (a *Annotation) Action() *Action {
	return Start[Action](a.Insert(Name("A")))
}

// Highlight is Annotation::highlight in pdf-writer src/annotations.rs.
//
// Write the /H attribute to set what effect is used to convey that the user
// is pressing a link or widget annotation. Only permissible for the subtypes
// Link and Widget. PDF 1.2+.
func (a *Annotation) Highlight(effect HighlightEffect) *Annotation {
	a.Pair(Name("H"), effect.toName())
	return a
}

// Author is Annotation::author in pdf-writer src/annotations.rs.
//
// Write the /T attribute. This is in the title bar of markup annotations and
// should be the name of the annotation author. PDF 1.1+.
func (a *Annotation) Author(text TextStrLike) *Annotation {
	a.Pair(Name("T"), text)
	return a
}

// Subject is Annotation::subject in pdf-writer src/annotations.rs.
//
// Write the /Subj attribute. This is the subject of the annotation. PDF
// 1.5+.
func (a *Annotation) Subject(text TextStrLike) *Annotation {
	a.Pair(Name("Subj"), text)
	return a
}

// QuadPoints is Annotation::quad_points in pdf-writer src/annotations.rs.
//
// Write the /QuadPoints attribute, specifying the region in which the link
// should be activated. PDF 1.6+.
func (a *Annotation) QuadPoints(coordinates ...float32) *Annotation {
	array := a.Insert(Name("QuadPoints")).Array()
	for _, c := range coordinates {
		array.Item(Real(c))
	}
	return a
}

// LineTo is Annotation::line_to in pdf-writer src/annotations.rs.
//
// Write the /L attribute. This defines the start and end point of a line
// annotation.
func (a *Annotation) LineTo(x1, y1, x2, y2 float32) *Annotation {
	a.Insert(Name("L")).Array().Items(Real(x1), Real(y1), Real(x2), Real(y2))
	return a
}

// Parent is Annotation::parent in pdf-writer src/annotations.rs.
//
// Write the /Parent attribute. Only permissible for the subtype Widget.
func (a *Annotation) Parent(id Ref) *Annotation {
	a.Pair(Name("Parent"), id)
	return a
}

// Annotation is Chunk::annotation in pdf-writer src/chunk.rs.
//
// Start writing an annotation dictionary.
func (c *Chunk) Annotation(id Ref) *Annotation {
	return Start[Annotation](c.Indirect(id))
}

// AnnotationType is AnnotationType in pdf-writer src/annotations.rs.
//
// Kind of the annotation to produce.
type AnnotationType uint8

// The values of AnnotationType.
const (
	// AnnotationTypeText is inline text.
	AnnotationTypeText AnnotationType = iota
	// AnnotationTypeLink is a link.
	AnnotationTypeLink
	// AnnotationTypeLine is a line. PDF 1.3+.
	AnnotationTypeLine
	// AnnotationTypeSquare is a square. PDF 1.3+.
	AnnotationTypeSquare
	// AnnotationTypeCircle is a circle. PDF 1.3+.
	AnnotationTypeCircle
	// AnnotationTypeHighlight highlights the text on the page. PDF 1.3+.
	AnnotationTypeHighlight
	// AnnotationTypeUnderline underlines the text on the page. PDF 1.3+.
	AnnotationTypeUnderline
	// AnnotationTypeSquiggly is a squiggly underline of the text on the
	// page. PDF 1.4+.
	AnnotationTypeSquiggly
	// AnnotationTypeStrikeOut strikes out the text on the page. PDF 1.3+.
	AnnotationTypeStrikeOut
	// AnnotationTypeFileAttachment is a reference to another file. PDF
	// 1.3+.
	//
	// Note that this annotation type is forbidden in PDF/A-1 and restricted
	// in other PDF/A parts.
	AnnotationTypeFileAttachment
	// AnnotationTypeWidget is a widget annotation. PDF 1.2+.
	AnnotationTypeWidget
	// AnnotationTypeScreen is a screen annotation. PDF 1.5+.
	//
	// Note that this annotation type is forbidden in PDF/A.
	AnnotationTypeScreen
)

// annotationTypeNames holds the PDF name of each value of AnnotationType,
// which is also its Rust name.
var annotationTypeNames = [...]Name{
	AnnotationTypeText:           "Text",
	AnnotationTypeLink:           "Link",
	AnnotationTypeLine:           "Line",
	AnnotationTypeSquare:         "Square",
	AnnotationTypeCircle:         "Circle",
	AnnotationTypeHighlight:      "Highlight",
	AnnotationTypeUnderline:      "Underline",
	AnnotationTypeSquiggly:       "Squiggly",
	AnnotationTypeStrikeOut:      "StrikeOut",
	AnnotationTypeFileAttachment: "FileAttachment",
	AnnotationTypeWidget:         "Widget",
	AnnotationTypeScreen:         "Screen",
}

// String returns the Rust name of the type.
func (t AnnotationType) String() string {
	return string(t.toName())
}

// toName is AnnotationType::to_name in pdf-writer src/annotations.rs.
func (t AnnotationType) toName() Name {
	if int(t) < len(annotationTypeNames) {
		return annotationTypeNames[t]
	}
	panic(fmt.Sprintf("pdfwriter: the annotation type %d is not known", uint8(t)))
}

// AnnotationFlags is AnnotationFlags in pdf-writer src/annotations.rs, a set
// of bit flags.
//
// Bitflags describing various characteristics of annotations.
type AnnotationFlags uint32

// The flags of AnnotationFlags.
const (
	// AnnotationFlagsInvisible will hide the annotation if the viewer does
	// not recognize its subtype. Otherwise, it will be rendered as specified
	// in its appearance stream.
	//
	// Must not be set for PDF/A.
	AnnotationFlagsInvisible AnnotationFlags = 1 << 0
	// AnnotationFlagsHidden hides the annotation from view and disallows
	// interaction. PDF 1.2+.
	//
	// Must not be set for PDF/A.
	AnnotationFlagsHidden AnnotationFlags = 1 << 1
	// AnnotationFlagsPrint prints the annotation. If not set, it will be
	// always hidden on print. PDF 1.2+.
	//
	// Must be set for PDF/A.
	AnnotationFlagsPrint AnnotationFlags = 1 << 2
	// AnnotationFlagsNoZoom does not zoom the annotation appearance if the
	// document is zoomed in. PDF 1.3+.
	//
	// Must be set for text annotations in PDF/A.
	AnnotationFlagsNoZoom AnnotationFlags = 1 << 3
	// AnnotationFlagsNoRotate does not rotate the annotation appearance if
	// the document is zoomed in. PDF 1.3+.
	//
	// Must be set for text annotations in PDF/A.
	AnnotationFlagsNoRotate AnnotationFlags = 1 << 4
	// AnnotationFlagsNoView does not view the annotation on screen. It may
	// still show on print. PDF 1.3+.
	//
	// Must not be set for PDF/A.
	AnnotationFlagsNoView AnnotationFlags = 1 << 5
	// AnnotationFlagsReadOnly does not allow interactions. PDF 1.3+.
	AnnotationFlagsReadOnly AnnotationFlags = 1 << 6
	// AnnotationFlagsLocked does not allow the user to delete or reposition
	// the annotation. Contents may still be changed. PDF 1.4+.
	AnnotationFlagsLocked AnnotationFlags = 1 << 7
	// AnnotationFlagsToggleNoView inverts the interpretation of the no_view
	// flag for certain events. PDF 1.5+.
	//
	// Must not be set for PDF/A.
	AnnotationFlagsToggleNoView AnnotationFlags = 1 << 8
	// AnnotationFlagsLockedContents does not allow content changes. PDF
	// 1.7+.
	AnnotationFlagsLockedContents AnnotationFlags = 1 << 9
)

// HighlightEffect is HighlightEffect in pdf-writer src/annotations.rs. The
// zero value is HighlightEffectNone. The default of upstream is
// HighlightEffectInvert.
//
// Highlighting effect applied when a user holds the mouse button over an
// annotation.
type HighlightEffect uint8

// The values of HighlightEffect.
const (
	// HighlightEffectNone is no effect.
	HighlightEffectNone HighlightEffect = iota
	// HighlightEffectInvert inverts the colors inside of the annotation
	// rect.
	HighlightEffectInvert
	// HighlightEffectOutline inverts the colors on the annotation border.
	HighlightEffectOutline
	// HighlightEffectPush makes the annotation rect's area appear
	// depressed.
	HighlightEffectPush
)

// highlightEffectNames holds the Rust name of each value of
// HighlightEffect.
var highlightEffectNames = [...]string{
	HighlightEffectNone:    "None",
	HighlightEffectInvert:  "Invert",
	HighlightEffectOutline: "Outline",
	HighlightEffectPush:    "Push",
}

// String returns the Rust name of the effect.
func (e HighlightEffect) String() string {
	if int(e) < len(highlightEffectNames) {
		return highlightEffectNames[e]
	}
	return "HighlightEffect(?)"
}

// toName is HighlightEffect::to_name in pdf-writer src/annotations.rs.
func (e HighlightEffect) toName() Name {
	switch e {
	case HighlightEffectNone:
		return Name("N")
	case HighlightEffectInvert:
		return Name("I")
	case HighlightEffectOutline:
		return Name("O")
	case HighlightEffectPush:
		return Name("P")
	}
	panic(fmt.Sprintf("pdfwriter: the highlight effect %d is not known", uint8(e)))
}

// FileSpec is Annotation::file_spec in pdf-writer src/annotations.rs.
//
// Start writing the /FS attribute.
func (a *Annotation) FileSpec() *FileSpec {
	return Start[FileSpec](a.Insert(Name("FS")))
}
