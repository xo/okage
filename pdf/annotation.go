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

// This file ports krilla src/interactive/annotation.rs, without the widget
// annotations: WidgetAnnotation, WidgetAnnotationKind and the appearances
// are for the forms, which the base port leaves out (D62). So the enum
// AnnotationType of upstream has the one case Link, and Annotation holds a
// LinkAnnotation. The struct parent of an annotation is for the tags, which
// the base port leaves out too.
//
// PDF annotations, allowing you to add extra "content" to specific pages.
//
// PDF has the concept of annotations, which allow you to associate certain
// regions of a page with an "annotation". The PDF reference defines many
// different actions, however, krilla does not and never will expose all of
// them. As of right now, the only annotations that are supported are "link
// annotations", which allow you associate a certain region of the page with
// a link.

// Annotation is Annotation in krilla src/interactive/annotation.rs. It
// satisfies the interface annotation of page.go.
//
// An annotation.
type Annotation struct {
	link LinkAnnotation
	// alt is the alt text. hasAlt is false for None.
	alt    string
	hasAlt bool
	// location is the location of the annotation. Zero is None.
	location Location
}

// NewAnnotationLink is Annotation::new_link in krilla
// src/interactive/annotation.rs. A nil alt text is None.
//
// Create a new link annotation with some alt text.
//
// Note that the alt text might be required in some cases, for example when
// exporting to PDF/UA.
func NewAnnotationLink(annotation LinkAnnotation, altText *string) Annotation {
	a := Annotation{link: annotation}
	if altText != nil {
		a.alt, a.hasAlt = *altText, true
	}
	return a
}

// AnnotationFromLink is the impl From<LinkAnnotation> for Annotation in
// krilla src/interactive/annotation.rs. The annotation has no alt text.
func AnnotationFromLink(annotation LinkAnnotation) Annotation {
	return Annotation{link: annotation}
}

// WithLocation is Annotation::with_location in krilla
// src/interactive/annotation.rs. A location of 0 is None.
//
// Sets the location of the annotation.
func (a Annotation) WithLocation(location Location) Annotation {
	a.location = location
	return a
}

// serialize is Annotation::serialize in krilla
// src/interactive/annotation.rs. A link annotation can have alt text, so
// the check of AnnotationType::can_have_alt_text is always true.
func (a Annotation) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref, pageHeight float32) error {
	if !a.hasAlt || a.alt == "" {
		sc.registerValidationError(validationError{kind: validationMissingAnnotationAltText, location: a.location})
	}

	annotation, err := a.link.serializeType(sc, cc, rootRef, pageHeight, a.location)
	if err != nil {
		return err
	}

	if a.hasAlt {
		annotation.Contents(pdfwriter.TextStr(a.alt))
	}

	annotation.Finish()
	return nil
}

// AddAnnotation is Page::add_annotation in krilla src/page.rs. An annotation
// that is added after Page.Finish is not written.
//
// Add an annotation to the page.
func (p *Page) AddAnnotation(annotation Annotation) {
	if p.finished {
		return
	}
	p.annotations = append(p.annotations, annotation)
}

// Target is Target in krilla src/interactive/annotation.rs. The case
// Target::Destination is a Destination, and the case Target::Action is an
// Action. Each one satisfies Target.
//
// An annotation target.
type Target interface {
	// serializeTarget writes the target into a link annotation, as the match
	// on the target in LinkAnnotation::serialize_type does.
	serializeTarget(sc *serializeContext, a *pdfwriter.Annotation, location Location) error
}

// LinkBorder is LinkBorder in krilla src/interactive/annotation.rs.
//
// Border of a link annotation.
type LinkBorder struct {
	width float32
	color Color
}

// NewLinkBorder is LinkBorder::new in krilla src/interactive/annotation.rs.
//
// Create a new link annotation border.
//
// width: The width of the border in pt. color: The color of the border.
func NewLinkBorder(width float32, color Color) LinkBorder {
	return LinkBorder{width: width, color: color}
}

// LinkAnnotation is LinkAnnotation in krilla src/interactive/annotation.rs.
// krilla panics in NewLinkAnnotationWithQuadPoints for quadrilaterals that
// give no rectangle. The port keeps the error in the field err, and
// Document.Finish returns it.
//
// A link annotation.
type LinkAnnotation struct {
	rect       Rect
	quadPoints []Quadrilateral
	target     Target
	border     *LinkBorder
	err        error
}

// NewLinkAnnotation is LinkAnnotation::new in krilla
// src/interactive/annotation.rs. A nil target makes Document.Finish return
// ErrPDFInvalidAnnotation.
//
// Create a new link annotation.
//
// rect: The bounding box of the link annotation that it should cover on the
// page. target: The target of the link annotation.
func NewLinkAnnotation(rect Rect, target Target) LinkAnnotation {
	return LinkAnnotation{rect: rect, target: target}
}

// rustMinF32 is f32::min of Rust, which returns the other value when one
// value is NaN. The builtin min of Go returns NaN.
func rustMinF32(a, b float32) float32 {
	switch {
	case math.IsNaN(float64(a)):
		return b
	case math.IsNaN(float64(b)):
		return a
	}
	return min(a, b)
}

// rustMaxF32 is f32::max of Rust, which returns the other value when one
// value is NaN.
func rustMaxF32(a, b float32) float32 {
	switch {
	case math.IsNaN(float64(a)):
		return b
	case math.IsNaN(float64(b)):
		return a
	}
	return max(a, b)
}

// NewLinkAnnotationWithQuadPoints is LinkAnnotation::new_with_quad_points in
// krilla src/interactive/annotation.rs. krilla panics when the list is empty,
// or when the bounding box of the points is not a valid rectangle, such as
// for a point that is not finite. Document.Finish returns
// ErrPDFInvalidAnnotation in its place.
//
// Create a new link annotation.
//
// target: The target of the link annotation. quadPoints: An array of
// quadrilaterals that define where the link annotation should be activated.
// This is useful if you for example have a link annotation that is broken
// to one or multiple lines.
func NewLinkAnnotationWithQuadPoints(quadPoints []Quadrilateral, target Target) LinkAnnotation {
	if len(quadPoints) == 0 {
		return LinkAnnotation{
			target: target,
			err:    fmt.Errorf("%w: a link annotation has no quadrilateral", okage.ErrPDFInvalidAnnotation),
		}
	}

	minX := float32(math.Inf(1))
	minY := float32(math.Inf(1))
	maxX := float32(math.Inf(-1))
	maxY := float32(math.Inf(-1))

	for _, q := range quadPoints {
		for _, point := range q {
			minX = rustMinF32(minX, point.X)
			minY = rustMinF32(minY, point.Y)
			maxX = rustMaxF32(maxX, point.X)
			maxY = rustMaxF32(maxY, point.Y)
		}
	}

	// Expand the bounding box by a little. There is a bug in adobe acrobat
	// that sometimes prevents the quadpoints from being used if the quad
	// points lie exactly on the bounding rectangle.
	const epsilon = 0.001
	rect, ok := path.RectFromLTRB(minX-epsilon, minY-epsilon, maxX+epsilon, maxY+epsilon)
	l := LinkAnnotation{
		rect:       rect,
		quadPoints: append([]Quadrilateral(nil), quadPoints...),
		target:     target,
	}
	if !ok {
		l.err = fmt.Errorf("%w: the quadrilaterals of a link annotation give no rectangle", okage.ErrPDFInvalidAnnotation)
	}
	return l
}

// WithBorder is LinkAnnotation::with_border in krilla
// src/interactive/annotation.rs.
//
// Set a border for this link annotation. The border will be visible on
// screen but not when printed, unless when exporting with PDF/A standard.
func (l LinkAnnotation) WithBorder(border LinkBorder) LinkAnnotation {
	l.border = &border
	return l
}

// serializeType is LinkAnnotation::serialize_type in krilla
// src/interactive/annotation.rs. krilla panics when the rectangle does not
// flip into the coordinates of PDF. The port returns an error that wraps
// ErrPDFInvalidAnnotation, as it does for a nil target and for the error of
// NewLinkAnnotationWithQuadPoints. It checks them before it writes the
// annotation.
func (l LinkAnnotation) serializeType(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref, pageHeight float32, location Location) (*pdfwriter.Annotation, error) {
	if l.err != nil {
		return nil, l.err
	}
	if l.target == nil {
		return nil, fmt.Errorf("%w: a link annotation has no target", okage.ErrPDFInvalidAnnotation)
	}
	actualRect, ok := l.rect.Transform(pageRootTransform(pageHeight))
	if !ok {
		return nil, fmt.Errorf("%w: the rectangle %s of a link annotation", okage.ErrPDFInvalidAnnotation, l.rect)
	}

	chunk := cc.nonStream.annotations
	annotation := chunk.Annotation(rootRef)

	annotation.Subtype(pdfwriter.AnnotationTypeLink)

	annotation.Rect(rectToPDFRect(actualRect))
	var width float32
	if l.border != nil {
		width = l.border.width
	}
	annotation.Border(0, 0, width, nil)

	if l.border != nil && l.border.color != nil {
		c := l.border.color.toPDFColor()
		switch l.border.color.(type) {
		case ColorRGB:
			annotation.ColorRGB(c[0], c[1], c[2])
		case ColorCMYK:
			annotation.ColorCMYK(c[0], c[1], c[2], c[3])
		case ColorLuma:
			annotation.ColorGray(c[0])
		}
	}

	if sc.serializeSettings().pdfVersion() >= PDF16 && l.quadPoints != nil {
		ts := pageRootTransform(pageHeight)
		coords := make([]float32, 0, 8*len(l.quadPoints))
		for _, q := range l.quadPoints {
			for _, p := range q {
				ts.MapPoint(&p)
				coords = append(coords, p.X, p.Y)
			}
		}
		annotation.QuadPoints(coords...)
	}

	if err := l.target.serializeTarget(sc, annotation, location); err != nil {
		return nil, err
	}

	// Only set the print flag when really necessary (only PDF/A). Don't set
	// it by default, so annotations with color borders will be shown on a
	// screen but not printed.
	if l.border == nil || sc.serializeSettings().validators().requiresAnnotationFlags() {
		annotation.Flags(pdfwriter.AnnotationFlagsPrint)
	}

	return annotation, nil
}
