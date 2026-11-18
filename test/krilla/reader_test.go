package krilla

import (
	"bytes"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
	"image"
	"strings"
	"testing"
)

// buildPDF writes a PDF file with one object for each body, numbered from
// 1, with a cross-reference table and a trailer. A body that holds the
// keyword stream must give its own /Length.
func buildPDF(trailer string, bodies ...string) []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.7\n%\x80\x80\x80\x80\n\n")
	offsets := make([]int, len(bodies))
	for i, body := range bodies {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n\n", i+1, body)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f\r\n", len(bodies)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n\r\n", o)
	}
	fmt.Fprintf(&b, "trailer\n%s\nstartxref\n%d\n%%%%EOF", trailer, xref)
	return b.Bytes()
}

// stream writes the body of a stream object.
func stream(dict string, data []byte) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data)
}

func deflate(data []byte) []byte {
	var b bytes.Buffer
	w := zlib.NewWriter(&b)
	_, _ = w.Write(data)
	_ = w.Close()
	return b.Bytes()
}

// TestReader reads a file with each filter that the reader decodes, and
// compares it with the same objects without filters. It runs without the
// oracle and without the outside programs.
func TestReader(t *testing.T) {
	content := []byte("q 1 0 0 rg 0 0 10 10 re f Q")
	// Two rows of 3 bytes with the PNG filters Up and Paeth.
	rows := []byte{1, 2, 3, 5, 7, 9}
	predicted := []byte{2, 1, 2, 3, 4, 4, 5, 6}
	var a85 bytes.Buffer
	enc := ascii85.NewEncoder(&a85)
	_, _ = enc.Write(content)
	_ = enc.Close()
	a85.WriteString("~>")
	trailer := "<< /Size 6 /Root 1 0 R /ID [<00> <01>] >>"
	encoded := buildPDF(trailer,
		"<< /Type /Catalog /Text (a\\(b\\)\\n\\101) /Hex <4142> >>",
		stream("/Filter /FlateDecode", deflate(content)),
		stream("/Filter [/ASCIIHexDecode /FlateDecode]", []byte(strings.ToUpper(hexString(deflate(content)))+">")),
		stream("/Filter /ASCII85Decode", a85.Bytes()),
		stream("/Filter /FlateDecode /DecodeParms << /Predictor 12 /Colors 3 /Columns 1 >>", deflate(predicted)),
		stream("/Filter [/FlateDecode /DCTDecode]", deflate([]byte("jpeg"))),
	)
	plain := buildPDF("<< /Size 6 /Root 1 0 R /ID [<02> <03>] >>",
		"<<\n  /Type /Catalog\n  /Text (a\\(b\\)\\nA)\n  /Hex <4142>\n>>",
		stream("", content),
		stream("", content),
		stream("", content),
		stream("", rows),
		stream("/Filter /DCTDecode", []byte("jpeg")),
	)
	got, err := readPDF(encoded)
	if err != nil {
		t.Fatal(err)
	}
	want, err := readPDF(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range got.Objects {
		if o.DecodeErr != nil {
			t.Errorf("object %d: %v", o.Ref.Num, o.DecodeErr)
		}
	}
	if diffs := comparePDF(got, want); len(diffs) != 0 {
		t.Errorf("the files differ:\n%s", strings.Join(diffs, "\n"))
	}
	if s := canon(got.Objects[0].Value); s != `<< /Type /Catalog /Text "a(b)\nA" /Hex <4142> >>` {
		t.Errorf("the catalog is %s", s)
	}

	changed := buildPDF(trailer,
		"<< /Type /Catalog /Text (a\\(b\\)\\nA) /Hex (AB) >>",
		stream("", content),
		stream("", []byte("q 1 0 0 rg 0 0 10 11 re f Q")),
		stream("", content),
		stream("", rows),
		stream("/Filter /DCTDecode", []byte("jpeg")),
		"7",
	)
	other, err := readPDF(changed)
	if err != nil {
		t.Fatal(err)
	}
	diffs := comparePDF(other, want)
	wantDiffs := []string{"object 1 0: got", "object 3 0: the data of the stream differs", "object 7 0 is extra"}
	if len(diffs) != len(wantDiffs) {
		t.Fatalf("got %d differences, want %d:\n%s", len(diffs), len(wantDiffs), strings.Join(diffs, "\n"))
	}
	for i, d := range diffs {
		if !strings.HasPrefix(d, wantDiffs[i]) {
			t.Errorf("difference %d is %q, want the prefix %q", i, d, wantDiffs[i])
		}
	}
}

// TestDiffPixels makes sure of the metric of D31.
func TestDiffPixels(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	b := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	b.Pix[0] = 1
	b.Pix[5] = 2
	b.Pix[8+7] = 40
	if n := diffPixels(a, b, 1); n != 4 {
		t.Errorf("got %d pixels with the threshold 1, want 4", n)
	}
	if n := diffPixels(a, b, 32); n != 3 {
		t.Errorf("got %d pixels with the threshold 32, want 3", n)
	}
}
