// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package testutils

import (
	"embed"
	"path"
	"testing"
)

func Assert(t testing.TB, b bool) {
	t.Helper()
	AssertC(t, b, "assertion error")
}

func AssertNoErr(t testing.TB, err error) {
	t.Helper()
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

func FilenamesFS(t testing.TB, fs *embed.FS, dir string) []string {
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
