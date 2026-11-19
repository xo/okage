// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage/path"
)

// FuzzImage gives random files to the functions that make an image, and
// draws each image that they accept into a document. A function must not panic,
// and Finish must return a file or an error. The first byte of the input
// chooses the function and the settings, and the other bytes are the file.
// The seeds are the files of testdata/images.
func FuzzImage(f *testing.F) {
	for _, dir := range []string{"images", "images/png_native"} {
		entries, err := os.ReadDir(filepath.Join("testdata", dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			var kind byte
			switch filepath.Ext(name) {
			case ".png":
				kind = 0
			case ".jpg":
				kind = 2
			case ".gif":
				kind = 3
			case ".webp":
				kind = 4
			default:
				continue
			}
			data, err := os.ReadFile(filepath.Join("testdata", dir, name))
			if err != nil {
				f.Fatal(err)
			}
			f.Add(append([]byte{kind}, data...))
			if kind == 0 {
				f.Add(append([]byte{1 | 0x10}, data...))
			}
		}
	}
	f.Add([]byte{5, 2, 0, 1, 0, 1, 2, 3, 4, 5, 6, 7, 8})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		flags, file := data[0], data[1:]
		interpolate := flags&0x80 != 0
		var img *Image
		var err error
		switch flags % 8 {
		case 0:
			img, err = ImageFromPNG(file, interpolate)
		case 1:
			img, err = ImageFromPNGNative(file, interpolate)
		case 2:
			img, err = ImageFromJPEG(file, interpolate)
		case 3:
			img, err = ImageFromGIF(file, interpolate)
		case 4:
			img, err = ImageFromWebP(file, interpolate)
		case 5:
			// The size takes the first two bytes, at most 255 by 255.
			if len(file) < 2 {
				return
			}
			img = ImageFromRGBA8(file[2:], uint32(file[0]), uint32(file[1]))
		case 6:
			img, err = imageFromJPEGWithICC(file, srgbV4ICCData, interpolate)
		default:
			img, err = imageFromJPEGWithICC(file, file, interpolate)
		}
		if err != nil {
			return
		}
		d := NewDocument(
			WithASCIICompatible(flags&0x08 != 0),
			WithNoDeviceCS(flags&0x10 != 0),
			WithPDFVersion(PDFVersion(flags>>5)%5),
		)
		ps, _ := PageSettingsFromWH(100, 100)
		size, _ := path.SizeFromWH(50, 50)
		s := d.StartPage(ps).Surface()
		s.DrawImage(img, size)
		s.DrawImage(img, size)
		var buf bytes.Buffer
		if err := d.Finish(&buf); err != nil {
			return
		}
		if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
			t.Fatalf("the file starts with %q", buf.Bytes()[:min(buf.Len(), 8)])
		}
	})
}
