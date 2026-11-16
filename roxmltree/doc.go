// Copyright 2018 Yevhenii Reizner
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package roxmltree is a port of the Rust crate roxmltree 0.21.1. It
// represents an XML document as a read-only tree.
//
// The root point of the package is Parse. The tree is a Document of nodes.
// A Node is the root, an element, a processing instruction, a comment or a
// text. The parser expands the entities of an internal DTD, also when the
// value of an entity is markup, and it resolves the namespaces (D8).
//
// The port keeps the limits of upstream: the depth of 10 entities and the
// 255 references of one entity reference, the 2^16 unique namespaces, the
// 2^32 attributes and the nodes limit of ParsingOptions. It keeps the feature
// positions of upstream on, so that each node and each attribute holds its
// range in the input text.
//
// Each error is a *Error that wraps a sentinel error of the root package,
// such as okage.ErrXMLUnknownToken (D21).
//
// The tree structure itself is a heavily modified
// <https://github.com/causal-agent/ego-tree>, under the license ISC.
package roxmltree
