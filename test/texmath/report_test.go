package texmath_test

import (
	"context"
	"fmt"
	"html"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xo/okage"
	"github.com/xo/okage/texmath"
	"github.com/xo/okage/texmath/stixtwomath"
	"github.com/xo/okage/typesetting/font"
)

// The report of Chromium of D78 section 7, item 4. It is not a gate. When a
// headless Chromium or Chrome is on the machine, TestChromiumReport writes
// pages of HTML that set each formula of the corpus that KaTeX and texmath
// both lay out twice: the HTML of KaTeX, with its style sheet and its fonts,
// and the image that Box.Draw of texmath fills. Chromium then prints each
// page to a PDF file beside it, so that a person can look at the two side by
// side.
//
// The report goes to the folder of the environment variable
// TEXMATH_REPORT_DIR, or else to okage-texmath-report in the temporary folder
// of the system. The test writes its files there and removes nothing. The
// environment variable TEXMATH_CHROME names the program of Chromium, when it
// is not on the PATH as chromium, chromium-browser, google-chrome or
// google-chrome-stable.

// reportPageSize is the number of formulas on one page of the report.
const reportPageSize = 100

// reportSize is the size in pixels of the formulas of texmath in the
// report. The style sheet of KaTeX sets a formula at 1.21 em, and the page
// sets its text at 16 pixels.
const reportSize = 16 * 1.21

// findChrome returns the program of Chromium, or "" when there is none.
func findChrome() string {
	if c := os.Getenv("TEXMATH_CHROME"); c != "" {
		return c
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// drawPNG returns the PNG data of the image that Box.Draw fills with b, the
// layout of tex, on white.
func drawPNG(tex string, b *texmath.Box) ([]byte, error) {
	const pad = 4
	w := uint32(math.Ceil(max(b.Width, 1))) + 2*pad
	h := uint32(math.Ceil(max(b.Height+b.Depth, 1))) + 2*pad
	pm, ok := okage.NewPixmap(w, h)
	if !ok {
		return nil, fmt.Errorf("making a pixmap of %d by %d for %q", w, h, tex)
	}
	pm.Fill(okage.ColorWhite)
	paint := okage.DefaultPaint()
	b.Draw(pm, okage.TransformFromTranslate(pad, float32(pad+b.Height)), &paint)
	data, err := pm.EncodePNG()
	if err != nil {
		return nil, fmt.Errorf("encoding the image of %q: %w", tex, err)
	}
	return data, nil
}

// TestChromiumReport writes the report.
func TestChromiumReport(t *testing.T) {
	t.Parallel()
	chrome := findChrome()
	if chrome == "" {
		t.Skip("Chromium is missing")
	}
	results := runKaTeX(t, "--html")
	dir := os.Getenv("TEXMATH_REPORT_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "okage-texmath-report")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	css, err := filepath.Abs(filepath.Join("katex", "node_modules", "katex", "dist", "katex.min.css"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := stixtwomath.Font()
	if err != nil {
		t.Fatal(err)
	}
	face := font.NewFace(f)
	var rows []string
	for _, r := range results {
		if !r.OK {
			continue
		}
		b, err := texmath.LayoutTeX(r.TeX, face, reportSize, texmath.Display)
		if err != nil {
			continue
		}
		data, err := drawPNG(r.TeX, b)
		if err != nil {
			t.Fatal(err)
		}
		img := fmt.Sprintf("okage-%04d.png", len(rows)+1)
		if err := os.WriteFile(filepath.Join(dir, img), data, 0o644); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, fmt.Sprintf("<tr><td class=n>%d</td><td class=tex><code>%s</code></td><td>%s</td><td><img src=%q></td></tr>\n",
			len(rows)+1, html.EscapeString(r.TeX), r.HTML, img))
	}
	profile := t.TempDir()
	pages := 0
	for start := 0; start < len(rows); start += reportPageSize {
		pages++
		end := min(start+reportPageSize, len(rows))
		var sb strings.Builder
		sb.WriteString("<!doctype html>\n<html><head><meta charset=utf-8>\n")
		fmt.Fprintf(&sb, "<title>texmath and KaTeX %s, page %d</title>\n", katexVersion, pages)
		fmt.Fprintf(&sb, "<link rel=stylesheet href=%q>\n", (&url.URL{Scheme: "file", Path: css}).String())
		sb.WriteString("<style>body{font:16px serif;margin:16px} td{border-bottom:1px solid #ccc;padding:4px 8px;vertical-align:middle}" +
			" .n{color:#888} .tex{max-width:300px;word-break:break-all;font-size:12px} tr{break-inside:avoid}</style>\n</head><body>\n")
		fmt.Fprintf(&sb, "<p>The formulas %d to %d: the TeX, KaTeX %s and texmath with STIX Two Math.</p>\n<table>\n", start+1, end, katexVersion)
		for _, row := range rows[start:end] {
			sb.WriteString(row)
		}
		sb.WriteString("</table>\n</body></html>\n")
		page := filepath.Join(dir, fmt.Sprintf("page-%02d.html", pages))
		if err := os.WriteFile(page, []byte(sb.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
		out, err := exec.CommandContext(ctx, chrome, "--headless", "--disable-gpu", "--no-first-run",
			"--user-data-dir="+profile, "--no-pdf-header-footer",
			"--print-to-pdf="+strings.TrimSuffix(page, ".html")+".pdf",
			(&url.URL{Scheme: "file", Path: page}).String()).CombinedOutput()
		cancel()
		if err != nil {
			// The report is not a gate.
			t.Logf("Chromium cannot print %s: %v\n%s", page, err, out)
		}
	}
	t.Logf("the report of %d formulas on %d pages is in %s", len(rows), pages, dir)
}
