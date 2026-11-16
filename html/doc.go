// Package html parses and lays out the HTML-like labels of Graphviz (D38),
// and parses HTML documents (D49).
//
// A label of the DOT language, label=<...>, holds lines of text or one table.
// The caller cuts the text between the outer < and > and gives it to Parse:
//
//	l, issues, err := html.Parse(`<table><tr><td port="id">id: integer</td></tr></table>`)
//	if err != nil {
//		return err
//	}
//	for _, issue := range issues {
//		log.Print(issue)
//	}
//	lay, err := l.Layout(html.WithTextStyle(html.TextStyle{Size: 11}))
//
// The layout gives the boxes, the rules and the runs of text in points, with
// y down and the origin at the top left of the label. The package draws
// nothing: the caller draws the layout, for example with the root package
// okage, or as SVG data.
//
// The parser and the layout follow Graphviz 16.1.0, and the test module
// compares the layout with the output of dot.
//
// ParseDocument parses a web page or a chapter of an ebook with the parsing
// algorithm of the HTML standard. It is a port of tinyhtml5, the parser of
// WeasyPrint. It finds the encoding, recovers from each error as a browser
// does, and returns a tree of nodes:
//
//	doc, err := html.ParseDocument(r)
//	if err != nil {
//		return err
//	}
//	for n := range doc.DocumentElement().Children() {
//		fmt.Println(n.Type(), n.Name())
//	}
//
// The options WithMaxDepth, WithMaxNodes and WithMaxIssues bound the work
// that a hostile file can cause.
package html
