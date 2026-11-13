package lottie

// This file ports no file of velato. It holds the reader of a dotLottie
// archive (D44).

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"

	"github.com/xo/okage"
)

// zipMagic is the signature of the first local file header of a zip archive.
var zipMagic = []byte("PK\x03\x04")

// IsDotLottie reports whether head is the start of a zip archive, and so of a
// dotLottie archive and not of a bare Lottie document. It does not read the
// archive.
func IsDotLottie(head []byte) bool {
	return bytes.HasPrefix(head, zipMagic)
}

// DotLottie is a dotLottie archive: a zip archive that holds one or more
// Lottie documents.
type DotLottie struct {
	// Animations holds the paths of the Lottie documents in the archive, in
	// sorted order.
	Animations []string

	zr *zip.Reader
}

// OpenDotLottie reads the directory of the dotLottie archive in r, which has
// size bytes.
//
// The dotLottie format puts its documents in the folder animations/ and
// describes them in manifest.json. OpenDotLottie takes each .json file of
// animations/. If that folder has no .json file, it takes each other .json
// file of the archive except manifest.json. It returns an error that wraps
// okage.ErrLottieDotLottie when r is not a zip archive or when it has no
// document.
func OpenDotLottie(r io.ReaderAt, size int64) (*DotLottie, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", okage.ErrLottieDotLottie, err)
	}
	var names, others []string
	for _, f := range zr.File {
		switch {
		case f.FileInfo().IsDir(), path.Ext(f.Name) != ".json":
		case path.Dir(f.Name) == "animations":
			names = append(names, f.Name)
		case path.Base(f.Name) != "manifest.json":
			others = append(others, f.Name)
		}
	}
	if len(names) == 0 {
		names = others
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%w: no animation in the archive", okage.ErrLottieDotLottie)
	}
	sort.Strings(names)
	return &DotLottie{Animations: names, zr: zr}, nil
}

// FS returns the files of the archive.
func (d *DotLottie) FS() fs.FS {
	return d.zr
}

// Data returns the bytes of the file name of the archive, such as one of
// Animations.
func (d *DotLottie) Data(name string) ([]byte, error) {
	buf, err := fs.ReadFile(d.zr, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", okage.ErrLottieDotLottie, err)
	}
	return buf, nil
}

// Parse returns the composition of the document name of the archive, such as
// one of Animations.
func (d *DotLottie) Parse(name string) (*Composition, error) {
	buf, err := d.Data(name)
	if err != nil {
		return nil, err
	}
	return Parse(buf)
}

// Load returns the composition of data, which is a Lottie document or a
// dotLottie archive. For an archive, it returns the composition of the first
// of its Animations.
func Load(data []byte) (*Composition, error) {
	if !IsDotLottie(data) {
		return Parse(data)
	}
	d, err := OpenDotLottie(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	return d.Parse(d.Animations[0])
}
