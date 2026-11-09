package oracle

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
	"github.com/xo/okage/svg"
)

type renderQuestion struct {
	Render string `json:"render"`
}

// renderAnswer is the answer of okage to a render question: the width and the
// height as two little-endian uint32, then the premultiplied RGBA bytes, as
// the normal mode of the integration tests of resvg renders the file. A file
// that does not parse, or a size that does not make a pixmap, gives no answer.
func renderAnswer(file string) []byte {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	tree, err := svg.FromData(data, svg.WithResourcesDir(filepath.Dir(file)))
	if err != nil {
		return nil
	}
	size, ok := tree.Size().ToIntSize().ScaleToWidth(300)
	if !ok {
		return nil
	}
	pixmap, ok := okage.NewPixmap(size.Width(), size.Height())
	if !ok {
		return nil
	}
	ts := path.TransformFromScale(
		float32(size.Width())/tree.Size().Width(),
		float32(size.Height())/tree.Size().Height(),
	)
	m := pixmap.AsMut()
	svg.Render(tree, ts, &m)
	out := binary.LittleEndian.AppendUint32(nil, size.Width())
	out = binary.LittleEndian.AppendUint32(out, size.Height())
	return append(out, pixmap.Data()...)
}

// TestResvg renders each SVG file of the tests of resvg with okage and with
// resvg, both without text, and compares every byte (phase 8, D31 gate 1). A
// file without text and without an image must give the same bytes. The files
// with text or an image are counted. resvg links the scalar code of tiny-skia
// here, as build says.
func TestResvg(t *testing.T) {
	if skipReason != "" {
		t.Skip(skipReason)
	}
	files := svgFiles(t)
	questions := make([]renderQuestion, len(files))
	for i, f := range files {
		questions[i] = renderQuestion{Render: f}
	}
	answers := ask(t, questions)
	total := map[string]int{}
	diffs := map[string]int{}
	shown := 0
	for i, f := range files {
		kind := usvgKind(f)
		total[kind]++
		got, err := safely(func() []byte { return renderAnswer(f) })
		if err != nil {
			got = []byte("panic")
		}
		if string(got) == string(answers[i]) {
			continue
		}
		diffs[kind]++
		if kind == "plain" {
			shown++
			if shown <= 10 {
				t.Errorf("%s: the pixels differ from resvg (%d and %d bytes)", f, len(got), len(answers[i]))
			}
		}
	}
	for _, kind := range []string{"plain", "image", "text"} {
		t.Logf("%s: %d files, %d differ", kind, total[kind], diffs[kind])
	}
}
