package lottie

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/xo/okage"
)

// makeZip returns a zip archive with the files of files, by name. A value
// that starts with "testdata:" is the content of that file of testdata.
func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		content := []byte(files[name])
		if s, ok := bytes.CutPrefix(content, []byte("testdata:")); ok {
			var err error
			if content, err = os.ReadFile(filepath.Join("testdata", string(s))); err != nil {
				t.Fatal(err)
			}
		}
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDotLottie(t *testing.T) {
	for _, test := range []struct {
		name  string
		files map[string]string
		exp   []string
	}{
		{
			"animations folder",
			map[string]string{
				"manifest.json":         `{"animations":[{"id":"b"},{"id":"a"}]}`,
				"animations/b.json":     "testdata:1f60a-blush.json",
				"animations/a.json":     "testdata:1f525-fire.json",
				"other.json":            "{}",
				"images/img_0.png":      "",
				"animations/notes.txt":  "",
				"animations/sub/x.json": "{}",
			},
			[]string{"animations/a.json", "animations/b.json"},
		},
		{
			// An archive with no animations folder gives its other documents.
			"no animations folder",
			map[string]string{
				"manifest.json": `{}`,
				"x.json":        "testdata:1f525-fire.json",
			},
			[]string{"x.json"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := makeZip(t, test.files)
			if !IsDotLottie(data) {
				t.Fatal("expected a dotLottie archive")
			}
			d, err := OpenDotLottie(bytes.NewReader(data), int64(len(data)))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(d.Animations, test.exp) {
				t.Errorf("expected %q, got %q", test.exp, d.Animations)
			}
			if _, err := d.FS().Open(test.exp[0]); err != nil {
				t.Errorf("FS: %v", err)
			}
			c, err := d.Parse(test.exp[0])
			if err != nil {
				t.Fatal(err)
			}
			// Load takes the first animation.
			l, err := Load(data)
			if err != nil {
				t.Fatal(err)
			}
			if l.FrameCount() != c.FrameCount() || l.FrameCount() != 65 {
				t.Errorf("expected the fire with 65 frames, got %d and %d", c.FrameCount(), l.FrameCount())
			}
		})
	}
}

func TestDotLottieErrors(t *testing.T) {
	data := makeZip(t, map[string]string{"manifest.json": "{}", "images/a.png": ""})
	if _, err := OpenDotLottie(bytes.NewReader(data), int64(len(data))); !errors.Is(err, okage.ErrLottieDotLottie) {
		t.Errorf("no animation: expected ErrLottieDotLottie, got %v", err)
	}
	bad := []byte("PK\x03\x04 not a zip")
	if _, err := Load(bad); !errors.Is(err, okage.ErrLottieDotLottie) {
		t.Errorf("not a zip: expected ErrLottieDotLottie, got %v", err)
	}
	if IsDotLottie([]byte(`{"fr":60}`)) {
		t.Error("a document is not a dotLottie archive")
	}
	data = makeZip(t, map[string]string{"animations/a.json": "testdata:1f525-fire.json"})
	d, err := OpenDotLottie(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Data("animations/missing.json"); !errors.Is(err, okage.ErrLottieDotLottie) {
		t.Errorf("missing file: expected ErrLottieDotLottie, got %v", err)
	}
}
