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
// The package does not port the feature text of usvg yet, so a Text node has
// no layout.
package svg
