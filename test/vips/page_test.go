//go:build vips

package vips

import (
	"cmp"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/svg"
)

var flagPage = flag.String("vips.page", "", "a folder in which to write a page that shows resvg, okage and libvips side by side")

// pageRow is one SVG file of the page.
type pageRow struct {
	Rel   string
	Group string
	Text  bool
	Vips  bool
	Pct   float64
	Label string
	W     int
	H     int
}

// TestVipsPage writes an HTML page to the folder of the flag -vips.page. The
// page shows, for each SVG file of the tests of resvg, the reference image of
// resvg, the image of okage, the image of libvips and the SVG file as the
// browser renders it, side by side. The folder links to the tests, so the page
// must be served by an HTTP server that follows a symbolic link. It also
// gives the percent of pixels where okage and libvips differ by more than 32
// in a channel, as TestVips counts them. The test skips without the flag:
//
//	go test -tags vips -run TestVipsPage ./vips/ -args -vips.page=/tmp/page
func TestVipsPage(t *testing.T) {
	if *flagPage == "" {
		t.Skip("the flag -vips.page is not set")
	}
	out, err := filepath.Abs(*flagPage)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	err = filepath.WalkDir(svgDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".svg") {
			files = append(files, p)
		}
		return err
	})
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the test files of svg are missing")
	}
	if err != nil {
		t.Fatal(err)
	}
	var rows []pageRow
	for _, f := range files {
		rel, err := filepath.Rel(svgDir, f)
		if err != nil {
			t.Fatal(err)
		}
		rel = strings.TrimSuffix(filepath.ToSlash(rel), ".svg")
		ref, err := os.ReadFile(strings.TrimSuffix(f, ".svg") + ".png")
		if err != nil {
			continue
		}
		w, h, a, ok := renderOkage(f)
		if !ok {
			continue
		}
		dir := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "resvg.png"), ref, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := writePNG(filepath.Join(dir, "okage.png"), w, h, a); err != nil {
			t.Fatal(err)
		}
		src, _ := os.ReadFile(f)
		row := pageRow{
			W:     w,
			H:     h,
			Rel:   rel,
			Group: rel[:strings.LastIndexByte(rel, '/')],
			Text:  strings.Contains(string(src), "<text"),
		}
		vw, vh, b, ok := renderVips(f, w)
		if ok && vw == w && vh == h {
			if err := writePNG(filepath.Join(dir, "vips.png"), w, h, b); err != nil {
				t.Fatal(err)
			}
			row.Vips = true
			row.Pct = 100 * float64(countApart(a, b)) / float64(w*h)
			row.Label = fmt.Sprintf("%.1f%%", row.Pct)
		} else {
			row.Label = "no image of the same size"
		}
		rows = append(rows, row)
	}
	// The page shows each SVG file from its folder, through a link to the
	// tests, so that the browser finds the images and the other files that
	// the file links to.
	link := filepath.Join(out, "svg")
	if _, err := os.Lstat(link); err != nil {
		abs, err := filepath.Abs(svgDir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Dir(abs), link); err != nil {
			t.Fatal(err)
		}
	}
	slices.SortFunc(rows, func(x, y pageRow) int { return cmp.Compare(x.Rel, y.Rel) })
	var groups []string
	for _, r := range rows {
		if len(groups) == 0 || groups[len(groups)-1] != r.Group {
			groups = append(groups, r.Group)
		}
	}
	page, err := os.Create(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	err = pageTemplate.Execute(page, map[string]any{"Rows": rows, "Groups": groups, "Fonts": fontFaceCSS()})
	if cerr := page.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d files to %s", len(rows), filepath.Join(out, "index.html"))
}

// fontFaceCSS returns an @font-face rule for each face of fontDB, with the
// bytes of its font embedded as a data URL, so that the browser renders the
// text of the SVG files with the fonts that okage and resvg use. A variable
// font takes the whole range of weights.
func fontFaceCSS() template.CSS {
	db := fontDB()
	var b strings.Builder
	seen := map[string]bool{}
	for face := range db.Faces() {
		data, index, ok := db.FaceData(face.ID)
		if !ok || index != 0 || len(face.Families) == 0 {
			continue
		}
		weight := strconv.FormatUint(uint64(face.Weight), 10)
		if hasTable(data, "fvar") {
			weight = "1 1000"
		}
		style := "normal"
		switch face.Style {
		case svg.FontStyleItalic:
			style = "italic"
		case svg.FontStyleOblique:
			style = "oblique"
		}
		family := strings.ReplaceAll(face.Families[0].Name, `"`, "")
		key := family + "|" + weight + "|" + style
		if seen[key] {
			continue
		}
		seen[key] = true
		fmt.Fprintf(&b, "@font-face { font-family: %q; font-weight: %s; font-style: %s; src: url(data:font/ttf;base64,%s); }\n",
			family, weight, style, base64.StdEncoding.EncodeToString(data))
	}
	return template.CSS(b.String())
}

// hasTable reports whether the font file data, which is not a collection,
// has the table tag in its table directory.
func hasTable(data []byte, tag string) bool {
	if len(data) < 12 {
		return false
	}
	n := int(binary.BigEndian.Uint16(data[4:]))
	for i := range n {
		off := 12 + 16*i
		if off+4 > len(data) {
			return false
		}
		if string(data[off:off+4]) == tag {
			return true
		}
	}
	return false
}

// countApart counts the pixels of a and b, RGBA bytes without premultiplied
// alpha, where a channel differs by more than 32 (D31 rule 5).
func countApart(a, b []byte) int {
	n := 0
	for i := 0; i < len(a); i += 4 {
		for c := range 4 {
			if d := int(a[i+c]) - int(b[i+c]); d > 32 || d < -32 {
				n++
				break
			}
		}
	}
	return n
}

// writePNG writes RGBA bytes without premultiplied alpha to a PNG file.
func writePNG(name string, w, h int, b []byte) error {
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("creating %s: %w", name, err)
	}
	img := &image.NRGBA{Pix: b, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("encoding %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", name, err)
	}
	return nil
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>resvg, okage, libvips</title>
<style>
:root { color-scheme: light dark; --bg: #fff; --fg: #1d1d1f; --muted: #6e6e73; --line: #d2d2d7; --c1: #eee; --c2: #fff; --hot: #c0392b; }
@media (prefers-color-scheme: dark) { :root { --bg: #161617; --fg: #f5f5f7; --muted: #a1a1a6; --line: #3a3a3c; --c1: #2c2c2e; --c2: #3a3a3c; --hot: #ff6b5b; } }
body { margin: 0; padding: 0 16px 48px; background: var(--bg); color: var(--fg); font: 14px/1.4 system-ui, sans-serif; }
header { position: sticky; top: 0; background: var(--bg); padding: 12px 0; border-bottom: 1px solid var(--line); z-index: 1; display: flex; flex-wrap: wrap; gap: 12px; align-items: center; }
h1 { font-size: 16px; margin: 0 12px 0 0; }
label { color: var(--muted); }
select, input[type=number] { font: inherit; }
#count { color: var(--muted); margin-left: auto; }
.row { padding: 16px 0; border-bottom: 1px solid var(--line); }
.name { font-family: ui-monospace, monospace; margin-bottom: 8px; display: flex; gap: 12px; flex-wrap: wrap; }
.pct { color: var(--muted); }
.pct.hot { color: var(--hot); font-weight: 600; }
.imgs { display: grid; grid-template-columns: repeat(4, minmax(0, 300px)); gap: 12px; }
@media (max-width: 720px) { .imgs { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
figure { margin: 0; }
figcaption { color: var(--muted); font-size: 12px; margin-bottom: 4px; }
img, object { width: 100%; height: auto; display: block; image-rendering: pixelated; background: repeating-conic-gradient(var(--c1) 0 25%, var(--c2) 0 50%) 0 0 / 16px 16px; border: 1px solid var(--line); }
.none { aspect-ratio: 1; border: 1px dashed var(--line); display: grid; place-items: center; color: var(--muted); font-size: 12px; }
</style>
<style id="fonts">{{.Fonts}}</style>
</head>
<body>
<header>
<h1>resvg, okage, libvips</h1>
<label>Group <select id="group"><option value="">all</option>{{range .Groups}}<option>{{.}}</option>{{end}}</select></label>
<label>libvips apart at least <input id="min" type="number" value="0" min="0" max="100" step="1" style="width:4em">%</label>
<label><input id="text" type="checkbox" checked> text</label>
<label>Sort <select id="sort"><option value="name">by name</option><option value="pct">by percent apart</option></select></label>
<span id="count"></span>
</header>
<main id="rows">
{{range .Rows}}<div class="row" data-group="{{.Group}}" data-pct="{{if .Vips}}{{printf "%.3f" .Pct}}{{else}}101{{end}}" data-text="{{.Text}}" data-name="{{.Rel}}">
<div class="name"><span>{{.Rel}}.svg</span><span class="pct{{if or (not .Vips) (gt .Pct 10.0)}} hot{{end}}">libvips: {{.Label}}</span>{{if .Text}}<span class="pct">text: fonts differ</span>{{end}}</div>
<div class="imgs">
<figure><figcaption>resvg (reference)</figcaption><img loading="lazy" src="{{.Rel}}/resvg.png" alt=""></figure>
<figure><figcaption>okage</figcaption><img loading="lazy" src="{{.Rel}}/okage.png" alt=""></figure>
<figure><figcaption>libvips (librsvg)</figcaption>{{if .Vips}}<img loading="lazy" src="{{.Rel}}/vips.png" alt="">{{else}}<div class="none">no image</div>{{end}}</figure>
<figure><figcaption>browser (live SVG, test fonts)</figcaption><object type="image/svg+xml" data-src="svg/tests/{{.Rel}}.svg" width="{{.W}}" height="{{.H}}" style="aspect-ratio: {{.W}} / {{.H}}"></object></figure>
</div>
</div>
{{end}}</main>
<script>
const rows = [...document.querySelectorAll('.row')];
const main = document.getElementById('rows');
function update() {
  const g = document.getElementById('group').value;
  const m = +document.getElementById('min').value || 0;
  const tx = document.getElementById('text').checked;
  const s = document.getElementById('sort').value;
  const sorted = rows.slice().sort((a, b) => s === 'pct' ? b.dataset.pct - a.dataset.pct : a.dataset.name.localeCompare(b.dataset.name));
  let n = 0;
  for (const r of sorted) {
    const show = (!g || r.dataset.group === g) && +r.dataset.pct >= m && (tx || r.dataset.text !== 'true');
    r.hidden = !show;
    if (show) n++;
    main.appendChild(r);
  }
  document.getElementById('count').textContent = n + ' of ' + rows.length + ' files';
}
// An SVG file in an object is a document of its own, so the fonts of this
// page do not reach it. Each one gets a copy of the @font-face rules, and its
// generic families become the families that the tests of resvg set. Each
// list of families ends with every embedded family, so that a character that
// the first font lacks falls back to a test font, as in resvg, and not to a
// font of the system.
const fontCSS = document.getElementById('fonts').textContent;
const generic = { 'serif': 'Noto Serif', 'sans-serif': 'Noto Sans', 'cursive': 'Yellowtail', 'fantasy': 'Sedgwick Ave Display', 'monospace': 'Noto Mono' };
const genericRE = /(?<![\w-])(sans-serif|serif|cursive|fantasy|monospace)(?![\w-])/g;
const mapGeneric = v => v.replace(genericRE, m => '"' + generic[m] + '"');
const fallback = [...new Set([...fontCSS.matchAll(/font-family: ("[^"]*")/g)].map(m => m[1]))].join(', ');
const withFallback = st => { if (st && st.fontFamily) st.fontFamily += ', ' + fallback; };
function addFonts(o) {
  const doc = o.contentDocument;
  if (!doc || !doc.documentElement) return;
  for (const el of doc.querySelectorAll('[font-family], [style], [font]')) {
    for (const a of ['font-family', 'style', 'font']) if (el.hasAttribute(a)) el.setAttribute(a, mapGeneric(el.getAttribute(a)));
  }
  for (const st of doc.getElementsByTagNameNS('http://www.w3.org/2000/svg', 'style')) st.textContent = mapGeneric(st.textContent);
  for (const el of doc.querySelectorAll('[font-family]')) el.setAttribute('font-family', el.getAttribute('font-family') + ', ' + fallback);
  for (const el of doc.querySelectorAll('[style]')) withFallback(el.style);
  for (const sheet of doc.styleSheets) {
    try { for (const rule of sheet.cssRules) withFallback(rule.style); } catch (e) { /* a sheet of another origin */ }
  }
  const st = doc.createElementNS('http://www.w3.org/2000/svg', 'style');
  st.textContent = fontCSS + ' :root { color-scheme: light dark; font-family: "Noto Serif", ' + fallback + '; }';
  doc.documentElement.insertBefore(st, doc.documentElement.firstChild);
}
const io = new IntersectionObserver(es => { for (const e of es) if (e.isIntersecting) { e.target.addEventListener('load', () => addFonts(e.target), { once: true }); e.target.data = e.target.dataset.src; io.unobserve(e.target); } }, { rootMargin: '600px' });
document.querySelectorAll('object[data-src]').forEach(o => io.observe(o));
for (const id of ['group', 'min', 'text', 'sort']) document.getElementById(id).addEventListener('input', update);
update();
</script>
</body>
</html>
`))
