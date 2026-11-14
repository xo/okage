package oracle

import (
	"path/filepath"
	"testing"
)

// TestLogo renders the logo of okage, docs/okage.svg, with okage and with
// resvg, and compares every byte.
func TestLogo(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	file, err := filepath.Abs(filepath.Join("..", "..", "docs", "okage.svg"))
	if err != nil {
		t.Fatal(err)
	}
	answers := ask(t, []renderQuestion{{Render: file}})
	got := renderAnswer(file)
	if len(got) <= 8 {
		t.Fatal("okage did not render the logo")
	}
	if len(got) != len(answers[0]) {
		t.Fatalf("expected %d bytes, got %d", len(answers[0]), len(got))
	}
	n, most := 0, 0
	for i := 8; i < len(got); i++ {
		d := int(got[i]) - int(answers[0][i])
		if d != 0 {
			n++
			most = max(most, d, -d)
		}
	}
	if n != 0 {
		t.Errorf("%d of %d bytes differ from resvg, by up to %d", n, len(got)-8, most)
	}
}
