// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package pdfsvg is a port of the Rust crate krilla-svg, from the base commit
// e3459d06 of krilla (D62). It draws a tree of the package svg onto a
// surface of the package pdf. docs/STATUS.md names the last commit of krilla
// that it follows.
//
// It passes nearly the whole test suite of resvg. A program parses an SVG
// file with svg, and draws the tree onto a surface:
//
//	tree, err := svg.FromData(data)
//	if err != nil {
//		return err
//	}
//	d := pdf.NewDocument(pdf.WithSVGGlyphs(pdfsvg.RenderGlyph))
//	ps, _ := pdf.PageSettingsFromWH(tree.Size().Width(), tree.Size().Height())
//	page := d.StartPage(ps)
//	pdfsvg.Draw(page.Surface(), tree, tree.Size())
//	page.Finish()
//	return d.Finish(w)
//
// RenderGlyph draws the SVG glyphs of a font, and pdf.WithSVGGlyphs takes
// it. pdfsvg imports pdf and svg, so a program that writes PDF without SVG
// does not link svg (D62).
//
// # The state of the port
//
// Phase 17 of "The PDF writer" in docs/PLAN.md ports the 9 files of
// krilla-svg 0.8.1. Each file of src is a file of the same name, and lib.rs
// is lib.go. Draw is SurfaceExt::draw_svg, the options WithEmbedText and
// WithFilterScale are the fields of SvgSettings, and RenderGlyph is
// render_svg_glyph. Each function named render in a module of upstream
// takes the name of its module as a suffix, such as renderGroup and
// renderClipPath (D10).
//
// # The differences from krilla-svg
//
// krilla-svg 0.8.1 requires usvg 0.47. The tree of svg follows a later
// usvg, after 0.49 (D62). Each change that the newer tree needs has a
// comment that starts with "okage:":
//
//  1. The layer bounding box of a group is optional. A group without one
//     has no content, and the raster fallback of filter.go draws nothing
//     for it.
//  2. usvg has the blend mode PlusLighter, which PDF does not have. A group
//     blends with the groups before it, up to the closest group that svg
//     isolates, or up to the root of a tree or of a subtree. When a group
//     in that range has PlusLighter, the whole range is drawn with svg into
//     an image, with the code of filter.go. The image holds the opacity,
//     the clip path and the mask of the range. renderRoot draws a root,
//     where upstream calls render of group.rs.
//  3. usvg does not give the crate fontdb to its users. The font database
//     of the tree is the FontDatabase of svg, and the tags of the axes of a
//     variable font come from its table fvar, where upstream reads them
//     with skrifa.
//
// The port also differs in these ways:
//
//  1. resvg draws a filter in upstream. svg draws it in the port, so the
//     pixels of the image are the pixels of svg (D62, item 10).
//  2. Draw and RenderGlyph return false where upstream panics: for a tree
//     whose size gives no rectangle, and for a glyph whose units per em are
//     not positive.
//  3. pdf and svg take the Transform, the Path and the Rect of the package
//     path, so the conversions of util.rs between the types of krilla and
//     of usvg have no port, except PathExt::to_krilla, which builds the
//     path again from its segments, as upstream does.
//  4. convertPaint takes the surface, and makes a StreamBuilder only for a
//     pattern. Upstream makes one for each paint, and a StreamBuilder that
//     is not finished writes nothing.
package pdfsvg
