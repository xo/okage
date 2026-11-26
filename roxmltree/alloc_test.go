package roxmltree

import (
	"strings"
	"testing"
)

// largeDocument returns an XHTML document of n paragraphs, each with
// inline elements, an attribute and an entity, as a book in EPUB has.
func largeDocument(n int) string {
	var b strings.Builder
	b.WriteString("<html xmlns=\"http://www.w3.org/1999/xhtml\"><body>\n")
	for i := range n {
		b.WriteString("<p class=\"para\">Text with <em>emphasis</em> and an entity &amp; more <a href=\"#n\">link</a> ")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteString("</p>\n")
	}
	b.WriteString("</body></html>")
	return b.String()
}

// BenchmarkParseLarge parses a document of 20,000 paragraphs, for the
// allocations of the parser.
func BenchmarkParseLarge(b *testing.B) {
	text := largeDocument(20000)
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(text); err != nil {
			b.Fatal(err)
		}
	}
}
