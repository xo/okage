// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "github.com/xo/okage/internal/xmlwriter"

// This file ports usvg src/lib.rs, without the items of the feature text. The
// crate documentation of upstream is the package comment in doc.go.
//
// The modules parser, tree and writer of upstream are the files
// usvg_parser_*.go, usvg_tree*.go and usvg_writer.go of this package, so the
// re-exports `pub use parser::*` and `pub use tree::*` need no port. The
// re-export of the crate roxmltree has no port either, because
// internal/roxmltree is internal (D5). The re-export of WriteOptions goes in
// usvg_writer.go, the port of src/writer.rs.
//
// TODO(phase 9): the module text and the re-export of the crate fontdb of the
// feature text.

// Indent is the re-export of xmlwriter::Indent in usvg src/lib.rs. It is an
// XML node indention.
type Indent = xmlwriter.Indent

// IndentKind is the case of an Indent. See xmlwriter::Indent.
type IndentKind = xmlwriter.IndentKind

// The cases of the enum xmlwriter::Indent, which usvg src/lib.rs exports
// again.
const (
	// IndentNone disables indention and new lines.
	IndentNone = xmlwriter.IndentNone
	// IndentSpaces indents with spaces. Preferred range is 0..4.
	IndentSpaces = xmlwriter.IndentSpaces
	// IndentTabs indents with tabs.
	IndentTabs = xmlwriter.IndentTabs
)
