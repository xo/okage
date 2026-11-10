// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package svg is a port of the Rust crates usvg 0.49.0 and resvg 0.49.0, in
// one package (D5).
//
// usvg (micro SVG) parses an SVG file into a tree of strongly typed nodes. It
// resolves the elements, the attributes, the references and the other features
// of SVG, so that the tree holds them in the simplest form. A caller does not
// have to handle most of the complexity of SVG, and can work on the rendering
// only. resvg renders that tree into a Pixmap of the package okage.
//
// The tree types of usvg are exported, because a user reads them, as upstream
// does. Most fields are private, and a method with the name of the Rust getter
// returns each one, such as Group.ID for Group::id.
//
// A Renderer renders SVG data into an *image.RGBA in one call (D36):
//
//	r := svg.New(svg.WithBackground(color.White), svg.WithWidth(800), svg.WithScaleMode(svg.ScaleBestFit))
//	img, err := r.Render(data)
//
// A Renderer loads the fonts of the system and of the home directory on its
// first call, and sets each generic family to the first of its candidates
// that is installed, such as SerifCandidates. It reads no configuration of
// fontconfig. It is safe for concurrent use. The package registers the format
// svg with image.RegisterFormat, so image.Decode reads SVG data after a
// program imports the package.
//
// ShapeText lays out a run of text without SVG data, and gives its advance
// width and the outlines of its glyphs as a path, with the same fonts,
// fallback and shaping as a <text> element (D37).
//
// The functions of the parser, such as FromData, read no font of the system.
// They lay out text with the fonts of a FontDatabase, which the caller gives
// with WithFontDatabase (D9). To use the fonts of a system with them, load the
// fonts into the database and set the generic families:
//
//	db := svg.NewFontDatabase()
//	db.LoadSystemFonts(os.DirFS("/"))
//	if home, err := os.UserHomeDir(); err == nil {
//		db.LoadUserFonts(os.DirFS(home))
//	}
//	db.SetSansSerifFamily("Noto Sans")
//	tree, err := svg.FromData(data, svg.WithFontDatabase(db))
//
// A text whose families match no font of the database is not drawn, as in
// resvg. The database reads only the names and the styles of each font file
// when it loads it, and reads the rest of a file when a face of it is used.
package svg
