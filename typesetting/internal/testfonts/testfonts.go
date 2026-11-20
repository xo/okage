// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

// Package testfonts reads the files of the module
// github.com/go-text/typesetting-utils that okage copies into
// typesetting/testdata (D63). Only the tests of typesetting use it. The
// packages opentype and harfbuzz below it take the place of the packages of
// the same names of typesetting-utils, so that a test of go-text changes only
// its imports.
//
// typesetting/testdata is a module of its own, so the module
// github.com/xo/okage does not hold it (D76). The folder is not there in the
// module cache of a program that requires okage. Missing tells that error
// apart, so that a test skips.
package testfonts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Dir reads the files of one folder of typesetting/testdata, as the embed.FS
// of typesetting-utils reads them. Its methods find typesetting/testdata from
// the working folder of the test, which is the folder of the package.
type Dir struct {
	name string
}

// NewDir returns the Dir of the folder name of typesetting/testdata.
func NewDir(name string) Dir {
	return Dir{name: name}
}

// Open opens the file name of the folder.
func (d Dir) Open(name string) (fs.File, error) {
	fsys, err := d.fsys()
	if err != nil {
		return nil, err
	}

	return fsys.Open(name)
}

// ReadFile reads the file name of the folder.
func (d Dir) ReadFile(name string) ([]byte, error) {
	fsys, err := d.fsys()
	if err != nil {
		return nil, err
	}

	return fs.ReadFile(fsys, name)
}

// ReadDir reads the folder name of the folder, sorted by file name.
func (d Dir) ReadDir(name string) ([]fs.DirEntry, error) {
	fsys, err := d.fsys()
	if err != nil {
		return nil, err
	}

	return fs.ReadDir(fsys, name)
}

func (d Dir) fsys() (fs.FS, error) {
	root, err := Find()
	if err != nil {
		return nil, err
	}

	return os.DirFS(filepath.Join(root, d.name)), nil
}

// missingError is the error of Find when typesetting/testdata is not there.
type missingError struct {
	from string
}

func (e *missingError) Error() string {
	return "typesetting/testdata is not in " + e.from + " or a folder above it, up to the root of the module"
}

func (e *missingError) Unwrap() error {
	return fs.ErrNotExist
}

// Missing reports whether err comes from Find because typesetting/testdata
// is not there.
func Missing(err error) bool {
	var m *missingError

	return errors.As(err, &m)
}

var (
	rootOnce sync.Once
	rootDir  string
	errRoot  error
)

// Find returns the path of typesetting/testdata. It looks for the file
// opentype/attributions.md in the folder testdata of the working folder and
// of each folder above it. It stops at the first folder that holds a file
// go.mod, which is the root of the module.
func Find() (string, error) {
	rootOnce.Do(func() {
		wd, err := os.Getwd()
		if err != nil {
			errRoot = fmt.Errorf("finding typesetting/testdata: %w", err)

			return
		}

		for dir := wd; ; {
			candidate := filepath.Join(dir, "testdata")
			if _, err := os.Stat(filepath.Join(candidate, "opentype", "attributions.md")); err == nil {
				rootDir = candidate

				return
			}

			parent := filepath.Dir(dir)
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil || parent == dir {
				errRoot = &missingError{from: wd}

				return
			}

			dir = parent
		}
	})

	return rootDir, errRoot
}
