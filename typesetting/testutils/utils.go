// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package testutils

import (
	iofs "io/fs"
	"path"
	"testing"

	"github.com/xo/okage/typesetting/internal/testfonts"
	"github.com/xo/okage/typesetting/internal/testfonts/opentype"
)

func Assert(t testing.TB, b bool) {
	t.Helper()
	AssertC(t, b, "assertion error")
}

func AssertNoErr(t testing.TB, err error) {
	t.Helper()
	// okage: typesetting/testdata is a module of its own, and it is not in
	// the module cache. A test that reads it skips then (D76).
	if testfonts.Missing(err) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func AssertC(t testing.TB, b bool, context string) {
	t.Helper()
	if !b {
		t.Fatal(context)
	}
}

// SkipWithoutTestdata skips the test when typesetting/testdata is not there,
// for a test that reads it without a check of the error.
//
// okage: this function is not in go-text (D76).
func SkipWithoutTestdata(t testing.TB) {
	t.Helper()
	_, err := testfonts.Find()
	AssertNoErr(t, err)
}

// Filenames return the "absolute" file names of the given directory
// excluding directories, and not recursing.
// It uses the opentype embed file system.
func Filenames(t testing.TB, dir string) []string {
	return FilenamesFS(t, &opentype.Files, dir)
}

// okage: fs is an fs.ReadDirFS, not an *embed.FS, so that it also reads the
// copies of typesetting-utils in typesetting/testdata (D63).
func FilenamesFS(t testing.TB, fs iofs.ReadDirFS, dir string) []string {
	t.Helper()

	files, err := fs.ReadDir(dir)
	AssertNoErr(t, err)

	var out []string
	for _, entry := range files {
		if entry.IsDir() {
			continue
		}
		// We should not use filepath.Join here because embed.FS still uses
		// unix-style paths on Windows.
		filename := path.Join(dir, entry.Name())
		out = append(out, filename)
	}
	return out
}
