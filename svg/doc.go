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
// The text of a file is laid out with the fonts of a FontDatabase, which the
// caller gives with WithFontDatabase (D9). The package reads no font of the
// system by itself. To use the fonts of a system as resvg does, load them
// into the database and set the generic families, because the package reads
// no configuration of fontconfig:
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
