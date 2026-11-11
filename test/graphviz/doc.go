// Package graphviz compares the layout of the package html with dot of
// Graphviz 16.1.0, the oracle of D38.
//
// Each test label goes into a graph as the label of a node with
// shape=plaintext and fontname="Courier", and dot -Tjson lays it out. The
// test measures text as Courier in dot does: each character is 0.6 of the
// size wide, and the height and the ascent of a line come from a probe label
// that dot lays out for each size. So the test compares the geometry, and not
// the fonts. It compares the box of each table and cell, each rule, and the
// left end and the baseline of each run of text.
//
// The tests skip when dot is not installed. The package has no code of its
// own.
package graphviz
