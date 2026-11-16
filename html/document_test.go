package html

// This file ports no upstream file. It tests the API of document.go, the
// encodings, the limits that guard against hostile input, and it holds the
// fuzz test of ParseDocument (D49).

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// outline writes the tree under n in the form of the tree construction
// tests, with no indent for the children of the root.
func outline(doc *Document) string {
	out := serialize(nil, doc.Root(), 0, true)
	s := strings.Join(out, "\n")
	for _, ns := range []string{NamespaceHTML, NamespaceMathML, NamespaceSVG, NamespaceXLink, NamespaceXML, NamespaceXMLNS} {
		repl := prefixes(ns) + " "
		if ns == NamespaceHTML {
			repl = ""
		}
		s = strings.ReplaceAll(s, "{"+ns+"}", repl)
	}
	return s
}

// mustParse parses src with ParseDocument.
func mustParse(t *testing.T, src string, opts ...DocumentOption) *Document {
	t.Helper()
	doc, err := ParseDocument(strings.NewReader(src), opts...)
	if err != nil {
		t.Fatalf("ParseDocument(%q): %v", src, err)
	}
	return doc
}

func TestParseDocumentTree(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			"implied html, head and body",
			"<title>T</title><p>One<p>Two",
			"<html>\n  <head>\n    <title>\n      \"T\"\n  <body>\n    <p>\n      \"One\"\n    <p>\n      \"Two\"",
		},
		{
			"implied tbody and misplaced text",
			"<!DOCTYPE html><table><tr><td>a</td></tr>x</table>",
			"<!DOCTYPE html>\n<html>\n  <head>\n  <body>\n    \"x\"\n    <table>\n      <tbody>\n        <tr>\n          <td>\n            \"a\"",
		},
		{
			"adoption agency",
			"<!DOCTYPE html><b>1<p>2</b>3</p>",
			"<!DOCTYPE html>\n<html>\n  <head>\n  <body>\n    <b>\n      \"1\"\n    <p>\n      <b>\n        \"2\"\n      \"3\"",
		},
		{
			"entities",
			"<!DOCTYPE html><p title='a&amp;b'>&lt;&eacute;&#x41;&noti;&#128;",
			"<!DOCTYPE html>\n<html>\n  <head>\n  <body>\n    <p>\n      title=\"a&b\"\n      \"<éA¬i;€\"",
		},
		{
			"svg and math",
			`<!DOCTYPE html><svg viewbox="0 0 1 1"><foreignobject><p>x</p></foreignobject><a xlink:href="#y"/></svg><math><mi>z</mi></math>`,
			"<!DOCTYPE html>\n<html>\n  <head>\n  <body>\n    <svg svg>\n      viewBox=\"0 0 1 1\"\n      <svg foreignObject>\n        <p>\n          \"x\"\n      <svg a>\n        xlink href=\"#y\"\n    <math math>\n      <math mi>\n        \"z\"",
		},
		{
			"comment and doctype",
			"<!-- c --><!DOCTYPE html><html><body></body></html><!-- d -->",
			"<!--  c  -->\n<!DOCTYPE html>\n<html>\n  <head>\n  <body>\n<!--  d  -->",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParse(t, tt.src)
			if got := outline(doc); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestParseDocumentAPI(t *testing.T) {
	doc := mustParse(t, `<!DOCTYPE html><html lang=en><body><a href="x" id=1 href="y">t</a>`)
	html := doc.DocumentElement()
	if html == nil || html.Name() != "html" || html.Namespace() != NamespaceHTML {
		t.Fatalf("DocumentElement = %v", html)
	}
	if v, ok := html.Attr("lang"); !ok || v != "en" {
		t.Errorf("lang = %q, %v", v, ok)
	}
	if doc.Root().Type() != DocumentNode || doc.Root().FirstChild().Type() != DoctypeNode {
		t.Errorf("the root does not start with the doctype")
	}
	if doc.Mode() != NoQuirks {
		t.Errorf("Mode = %v", doc.Mode())
	}
	body := html.LastChild()
	a := body.FirstChild()
	if a.Name() != "a" || a.Parent() != body || a.FirstChild().Data() != "t" {
		t.Fatalf("unexpected tree: %s", outline(doc))
	}
	attrs := a.Attributes()
	if len(attrs) != 2 || attrs[0] != (Attribute{Name: "href", Value: "x"}) || attrs[1] != (Attribute{Name: "id", Value: "1"}) {
		t.Errorf("Attributes = %v", attrs)
	}
	attrs[0].Value = "changed"
	if v, _ := a.Attr("href"); v != "x" {
		t.Errorf("Attributes does not return a copy")
	}
	var names []string
	for c := range html.Children() {
		names = append(names, c.Name())
	}
	if strings.Join(names, ",") != "head,body" {
		t.Errorf("Children = %v", names)
	}
	if body.PrevSibling() != html.FirstChild() || html.FirstChild().NextSibling() != body {
		t.Errorf("the siblings are wrong")
	}
	issues := doc.Issues()
	if len(issues) != 1 || !strings.HasPrefix(issues[0].Msg, "duplicate-attribute") || issues[0].Pos.Line != 1 {
		t.Errorf("Issues = %v", issues)
	}

	quirks := mustParse(t, "<p>no doctype")
	if quirks.Mode() != Quirks {
		t.Errorf("Mode without a doctype = %v", quirks.Mode())
	}
	if len(quirks.Issues()) == 0 || quirks.Issues()[0].Msg != "expected-doctype-but-got-start-tag (name: \"p\")" {
		t.Errorf("Issues = %v", quirks.Issues())
	}
}

func TestParseFragment(t *testing.T) {
	doc, err := ParseFragment(strings.NewReader("<td>a</td><b>b"), "tr")
	if err != nil {
		t.Fatal(err)
	}
	want := "<td>\n  \"a\"\n<b>\n  \"b\""
	if got := outline(doc); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if doc.DocumentElement() != nil {
		t.Errorf("a fragment has a document element")
	}
}

func TestParseDocumentEncoding(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts []DocumentOption
		enc  string
		text string
	}{
		{"default windows-1252", "<p>\x93q\x94 \xe9", nil, "windows-1252", "“q” é"},
		{"utf-8 bom", "\xef\xbb\xbf<p>é", nil, "utf-8", "é"},
		{"utf-16le bom", "\xff\xfe<\x00p\x00>\x00\xe9\x00", nil, "utf-16le", "é"},
		{"utf-16be bom", "\xfe\xff\x00<\x00p\x00>\x00\xe9", nil, "utf-16be", "é"},
		{"meta charset", "<meta charset=utf-8><p>\xc3\xa9", nil, "utf-8", "é"},
		{"meta content", `<meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><p>` + "\xc3\xa9", nil, "utf-8", "é"},
		{"meta after 1024 bytes", "<!--" + strings.Repeat("x", 1100) + "--><meta charset=utf-8><p>\xc3\xa9", nil, "utf-8", "é"},
		{"override", "<meta charset=windows-1252><p>\xc3\xa9", []DocumentOption{WithCharset("utf-8")}, "utf-8", "é"},
		{"default charset", "<p>\xc3\xa9", []DocumentOption{WithDefaultCharset("utf8")}, "utf-8", "é"},
		{"bad utf-8", "<meta charset=utf-8><p>a\xe2\x82b\xff", nil, "utf-8", "a�b�"},
		{"unsupported", "<meta charset=shift_jis><p>\xe9", nil, "windows-1252", "é"},
		{"charset reader", "<meta charset=iso-8859-2><p>\xe9", []DocumentOption{WithCharsetReader(func(charset string, r io.Reader) (io.Reader, error) {
			if charset != "iso-8859-2" {
				return nil, errors.New("unexpected charset")
			}
			return strings.NewReader("<p>é"), nil
		})}, "iso-8859-2", "é"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := mustParse(t, tt.src, tt.opts...)
			if doc.Encoding() != tt.enc {
				t.Errorf("Encoding = %q, want %q", doc.Encoding(), tt.enc)
			}
			body := doc.DocumentElement().LastChild()
			p := body.LastChild()
			if p == nil || p.FirstChild() == nil || p.FirstChild().Data() != tt.text {
				t.Errorf("tree:\n%s\nwant the text %q", outline(doc), tt.text)
			}
		})
	}
	doc := mustParse(t, "<meta charset=shift_jis><p>x")
	if !hasIssue(doc, "unsupported-encoding") {
		t.Errorf("no issue for an encoding that html does not decode: %v", doc.Issues())
	}
}

// errReader is a reader that fails.
type errReader struct{}

var errRead = errors.New("read failed")

func (errReader) Read([]byte) (int, error) {
	return 0, errRead
}

func TestParseDocumentReadError(t *testing.T) {
	if _, err := ParseDocument(errReader{}); !errors.Is(err, errRead) {
		t.Errorf("ParseDocument error = %v, want %v", err, errRead)
	}
	if _, err := ParseFragment(errReader{}, "div"); !errors.Is(err, errRead) {
		t.Errorf("ParseFragment error = %v, want %v", err, errRead)
	}
}

// hostile returns inputs that try to make the parser slow or deep.
func hostile() map[string]string {
	return map[string]string{
		"deep divs":           strings.Repeat("<div>", 200000),
		"deep formatting":     strings.Repeat("<b><i><u>", 100000),
		"reopened formatting": strings.Repeat("<p><b id=1><i id=2></p>", 50000),
		"misnested":           strings.Repeat("<a><p><b></a></p>", 50000),
		"tables":              strings.Repeat("<table><tr><td><b>", 50000),
		"many attributes":     "<p " + strings.Repeat("a", 1) + manyAttrs(100000) + ">",
		"html attributes":     "<html " + manyAttrs(50000) + "><html " + manyAttrs(50000) + ">",
		"entities":            strings.Repeat("&amp;", 500000),
		"long comment":        "<!--" + strings.Repeat("-a", 500000),
		"long name":           "<" + strings.Repeat("a", 500000),
		"long doctype":        "<!DOCTYPE " + strings.Repeat("a", 500000),
		"nulls":               strings.Repeat("\x00", 500000),
		"end tags":            "<div>" + strings.Repeat("</x>", 200000),
		"many elements":       strings.Repeat("<br>", 2000000),
	}
}

// manyAttrs returns n attributes with names that differ.
func manyAttrs(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString(" a")
		for j := i; j > 0; j /= 26 {
			b.WriteByte(byte('a' + j%26))
		}
	}
	return b.String()
}

func TestParseDocumentHostile(t *testing.T) {
	for name, src := range hostile() {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			doc := mustParse(t, src)
			if d := time.Since(start); d > 20*time.Second {
				t.Errorf("the parse took %v", d)
			}
			if n := len(doc.Issues()); n > DefaultMaxIssues+1 {
				t.Errorf("%d issues, more than the limit", n)
			}
			checkTree(t, doc.Root())
		})
	}
}

func TestParseDocumentLimits(t *testing.T) {
	doc := mustParse(t, strings.Repeat("<div>", 100), WithMaxDepth(10))
	if !hasIssue(doc, "too-deep (limit: 10)") {
		t.Errorf("no issue for the depth: %v", doc.Issues())
	}
	if n := countNodes(doc.Root()); n > 20 {
		t.Errorf("the tree has %d nodes", n)
	}

	doc = mustParse(t, strings.Repeat("<p>x", 100), WithMaxNodes(10))
	if !hasIssue(doc, "too-many-nodes (limit: 10)") {
		t.Errorf("no issue for the nodes: %v", doc.Issues())
	}
	if n := countNodes(doc.Root()); n > 20 {
		t.Errorf("the tree has %d nodes", n)
	}

	doc = mustParse(t, strings.Repeat("</x>", 100), WithMaxIssues(5))
	if n := len(doc.Issues()); n != 5 {
		t.Errorf("%d issues, want 5", n)
	}
}

// hasIssue says if the document has an issue that starts with prefix.
func hasIssue(doc *Document, prefix string) bool {
	for _, issue := range doc.Issues() {
		if strings.HasPrefix(issue.Msg, prefix) {
			return true
		}
	}
	return false
}

// countNodes counts the nodes under n, n included.
func countNodes(n *Node) int {
	count := 1
	for c := range n.Children() {
		count += countNodes(c)
	}
	return count
}

// checkTree makes sure that the links of the tree under root agree, with no
// recursion.
func checkTree(t *testing.T, root *Node) {
	t.Helper()
	type item struct{ n *Node }
	stack := []item{{root}}
	for len(stack) > 0 {
		n := stack[len(stack)-1].n
		stack = stack[:len(stack)-1]
		var prev *Node
		for c := n.firstChild; c != nil; c = c.nextSibling {
			if c.parent != n || c.prevSibling != prev {
				t.Fatalf("the links of a %v node are wrong", c.typ)
			}
			if c.text != nil {
				t.Fatalf("a text node kept its parse buffer")
			}
			prev = c
			stack = append(stack, item{c})
		}
		if n.lastChild != prev {
			t.Fatalf("the last child of a %v node is wrong", n.typ)
		}
	}
}

func FuzzParseDocument(f *testing.F) {
	seeds := []string{
		"",
		"<!DOCTYPE html><html><head><title>t</title></head><body><p>x</p></body></html>",
		"<table><tr><td>a<b>b</td></tr></table>c",
		"<b>1<p>2</b>3</p>",
		"<svg><foreignObject><p>x</svg><math><mi><b>y</math>",
		"<select><option>a<optgroup><option>b</select>",
		"<frameset><frame></frameset><noframes>x</noframes>",
		"<meta charset=utf-16><p>\xff\xfe",
		"\xff\xfe<\x00p\x00>\x00",
		"<script><!--<script></script>--></script>",
		"<textarea>\n&amp;</textarea><pre>\nx</pre>",
		"<a><div><a>x</div></a>",
		"<!DOCTYPE html PUBLIC \"-//W3C//DTD HTML 4.01 Transitional//EN\">",
		"<math><annotation-xml encoding=text/html><div>x</div></annotation-xml></math>",
		"<table><caption><table></caption><colgroup><col></colgroup>",
		"<isindex prompt=x action=y>",
		"<html a=1><body b=2><html c=3><body d=4>",
		"&#0;&#x110000;&#xD800;&notit;&amp",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := ParseDocument(bytes.NewReader(data), WithMaxDepth(64), WithMaxNodes(4096))
		if err != nil {
			t.Fatal(err)
		}
		checkTree(t, doc.Root())
		if doc.DocumentElement() == nil {
			t.Fatal("no html element")
		}
		frag, err := ParseFragment(bytes.NewReader(data), "td", WithMaxDepth(64), WithMaxNodes(4096))
		if err != nil {
			t.Fatal(err)
		}
		checkTree(t, frag.Root())
	})
}
