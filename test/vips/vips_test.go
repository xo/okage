//go:build vips

package vips

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cshum/vipsgen/v2/vips"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/svg"
)

// svgDir is the folder of the SVG files of the tests of resvg, as svg holds
// them.
var svgDir = filepath.Join("..", "..", "svg", "testdata", "tests")

// fontDB holds the fonts of the tests of resvg, as GLOBAL_FONTDB of resvg
// tests/integration/main.rs and the integration tests of svg load them, in
// the same order and with the same generic families.
var fontDB = sync.OnceValue(func() *svg.FontDatabase {
	db := svg.NewFontDatabase()
	dir := filepath.Join("..", "..", "svg", "testdata", "fonts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return db
	}
	var names []string
	for _, e := range entries {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".ttf", ".ttc", ".otf", ".otc":
			names = append(names, filepath.Join(dir, e.Name()))
		}
	}
	slices.Sort(names)
	for _, name := range names {
		if data, err := os.ReadFile(name); err == nil {
			db.LoadFontData(data)
		}
	}
	db.SetSerifFamily("Noto Serif")
	db.SetSansSerifFamily("Noto Sans")
	db.SetCursiveFamily("Yellowtail")
	db.SetFantasyFamily("Sedgwick Ave Display")
	db.SetMonospaceFamily("Noto Mono")
	return db
})

func TestMain(m *testing.M) {
	vips.Startup(nil)
	os.Exit(m.Run())
}

// renderOkage renders file as the normal mode of the integration tests of
// resvg does, with the fonts of the tests, at a width of 300 pixels. It
// returns the RGBA bytes without premultiplied alpha.
func renderOkage(file string) (int, int, []byte, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, nil, false
	}
	tree, err := svg.FromData(data, svg.WithResourcesDir(filepath.Dir(file)), svg.WithFontDatabase(fontDB()))
	if err != nil {
		return 0, 0, nil, false
	}
	size, ok := tree.Size().ToIntSize().ScaleToWidth(300)
	if !ok {
		return 0, 0, nil, false
	}
	pixmap, ok := okage.NewPixmap(size.Width(), size.Height())
	if !ok {
		return 0, 0, nil, false
	}
	ts := path.TransformFromScale(float32(size.Width())/tree.Size().Width(), float32(size.Height())/tree.Size().Height())
	m := pixmap.AsMut()
	svg.Render(tree, ts, &m)
	b := append([]byte(nil), pixmap.Data()...)
	for i := 0; i < len(b); i += 4 {
		if a := b[i+3]; a != 0 && a != 255 {
			for c := range 3 {
				b[i+c] = uint8(min(255, (float64(b[i+c])*255+float64(a)/2)/float64(a)))
			}
		}
	}
	return int(size.Width()), int(size.Height()), b, true
}

// renderVips renders file with libvips at the scale that gives a width of w,
// and returns its RGBA bytes.
func renderVips(file string, w int) (int, int, []byte, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, 0, nil, false
	}
	probe, err := vips.NewSvgloadBuffer(data, nil)
	if err != nil {
		return 0, 0, nil, false
	}
	scale := float64(w) / float64(probe.Width())
	probe.Close()
	opt := vips.DefaultSvgloadBufferOptions()
	opt.Scale = scale
	img, err := vips.NewSvgloadBuffer(data, opt)
	if err != nil {
		return 0, 0, nil, false
	}
	defer img.Close()
	if img.Bands() != 4 {
		return 0, 0, nil, false
	}
	b, err := img.WriteToMemory()
	if err != nil {
		return 0, 0, nil, false
	}
	return img.Width(), img.Height(), b, true
}

// TestVips renders each SVG file of the tests of resvg without text with okage
// and with libvips, and reports how many pixels differ by more than 32 in a
// channel (D31 rule 5). It has no gate.
func TestVips(t *testing.T) {
	var files []string
	err := filepath.WalkDir(svgDir, func(p string, d fs.DirEntry, err error) error {
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
	compared, listed, failed := 0, 0, 0
	for _, f := range files {
		src, _ := os.ReadFile(f)
		if strings.Contains(string(src), "<text") {
			continue
		}
		w, h, a, ok := renderOkage(f)
		if !ok {
			continue
		}
		vw, vh, b, ok := renderVips(f, w)
		if !ok || vw != w || vh != h {
			failed++
			continue
		}
		compared++
		n := countApart(a, b)
		if float64(n) > 0.1*float64(w*h) {
			listed++
			t.Logf("%s: %d of %d pixels differ from libvips", f, n, w*h)
		}
	}
	t.Logf("%d files compared, %d with more than 10 percent of their pixels apart, %d that libvips did not render at the same size", compared, listed, failed)
}
