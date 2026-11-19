// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/path"
)

// The tests of this file are the tests of krilla src/graphics/image/mod.rs,
// of the tests of PdfVersion::supports_icc in src/configure/version.rs, and
// the snapshot tests of krilla tests/src/image.rs and tests/src/png.rs. The
// files of testdata/images are the files of assets/images of krilla, byte for
// byte, with the license of krilla.

// readImageFile reads a file of testdata/images.
func readImageFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "images", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// loadImage is load_png_image, load_jpg_image, load_gif_image and
// load_webp_image of krilla tests/src/main.rs, by the extension of the
// file.
func loadImage(t *testing.T, name string) *Image {
	t.Helper()
	data := readImageFile(t, name)
	var img *Image
	var err error
	switch filepath.Ext(name) {
	case ".png":
		img, err = ImageFromPNG(data, false)
	case ".jpg":
		img, err = ImageFromJPEG(data, false)
	case ".gif":
		img, err = ImageFromGIF(data, false)
	default:
		img, err = ImageFromWebP(data, false)
	}
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// imageSize is the size of the image, for draw_image.
func imageSize(t *testing.T, img *Image) Size {
	t.Helper()
	w, h := img.Size()
	size, ok := path.SizeFromWH(float32(w), float32(h))
	if !ok {
		t.Fatal("invalid size")
	}
	return size
}

// TestInvalidPNGImage is the test invalid_png_image of krilla
// src/graphics/image/mod.rs.
func TestInvalidPNGImage(t *testing.T) {
	if _, err := ImageFromPNG([]byte("dfngiudfg"), false); !errors.Is(err, okage.ErrPDFImage) {
		t.Errorf("got %v, want ErrPDFImage", err)
	}
}

// TestInvalidJPEGImage is the test invalid_jpeg_image of krilla
// src/graphics/image/mod.rs.
func TestInvalidJPEGImage(t *testing.T) {
	if _, err := ImageFromJPEG([]byte("dfngiudfg"), false); !errors.Is(err, okage.ErrPDFImage) {
		t.Errorf("got %v, want ErrPDFImage", err)
	}
}

// TestPNGNativeRequiresNoDecoding is the test png_native_requires_no_decoding
// of krilla tests/src/png.rs.
func TestPNGNativeRequiresNoDecoding(t *testing.T) {
	data := readImageFile(t, "png_native/rgba_8.png")
	_, err := ImageFromPNGNative(data, false)
	if !errors.Is(err, okage.ErrPDFImage) || !strings.HasSuffix(err.Error(), "PNG cannot be embedded without decoding") {
		t.Errorf("got %v", err)
	}
	if _, err := ImageFromPNG(data, false); err != nil {
		t.Error(err)
	}
}

// TestICCV24SupportedFromPDF15 is the test icc_v2_4_supported_from_pdf_1_5 of
// krilla src/configure/version.rs.
func TestICCV24SupportedFromPDF15(t *testing.T) {
	m := iccMetadata{major: 2, minor: 4, colorSpace: iccColorSpaceCMYK}
	for v, want := range map[PDFVersion]bool{PDF14: false, PDF15: true, PDF16: true, PDF17: true, PDF20: true} {
		if got := v.supportsICC(m); got != want {
			t.Errorf("%v: got %v, want %v", v, got, want)
		}
	}
}

// TestICCMaxVersionPerPDFVersion is the test icc_max_version_per_pdf_version
// of krilla src/configure/version.rs.
func TestICCMaxVersionPerPDFVersion(t *testing.T) {
	for _, c := range []struct {
		v            PDFVersion
		major, minor uint8
		want         bool
	}{
		{PDF14, 2, 3, true}, {PDF14, 4, 0, false},
		{PDF15, 4, 0, true}, {PDF15, 4, 1, false},
		{PDF16, 4, 1, true}, {PDF16, 4, 2, false},
		{PDF17, 4, 2, true}, {PDF17, 4, 3, false},
		{PDF20, 4, 3, true}, {PDF20, 4, 4, false},
	} {
		m := iccMetadata{major: c.major, minor: c.minor, colorSpace: iccColorSpaceCMYK}
		if got := c.v.supportsICC(m); got != c.want {
			t.Errorf("%v with ICC %d.%d: got %v, want %v", c.v, c.major, c.minor, got, c.want)
		}
	}
}

// imageSnapshot finishes the document and compares it with the snapshot of
// krilla. It returns true when the bytes are the same, and false when they
// are the same only after the streams are decoded, because the bytes of
// compress/zlib differ from the bytes of flate2 (D62).
func imageSnapshot(t *testing.T, name string, d *Document) bool {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Finish(&buf); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "snapshots", name+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(buf.Bytes(), want) {
		return true
	}
	got, wantS := normalizeSnapshot(t, buf.Bytes()), normalizeSnapshot(t, want)
	if got == wantS {
		return false
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(wantS, "\n")
	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Fatalf("%s differs from the decoded snapshot at the line %d:\ngot  %.200q\nwant %.200q", name, i+1, g, w)
		}
	}
	return false
}

// imageSnapshotCase is a snapshot test of tests/src/image.rs or
// tests/src/png.rs.
type imageSnapshotCase struct {
	name string
	// exact is true when the file of pdf is the snapshot, byte for byte. It
	// is false when the PDF file holds a stream that pdf compresses, which
	// differs from krilla only in its compressed bytes.
	exact bool
	run   func(t *testing.T) *Document
}

// imageVisregImpl is image_visreg_impl of krilla tests/src/image.rs on a
// snapshot page.
func imageVisregImpl(name string) func(t *testing.T) *Document {
	return func(t *testing.T) *Document {
		t.Helper()
		d, page := snapshotPage()
		img := loadImage(t, name)
		page.Surface().DrawImage(img, imageSize(t, img))
		return d
	}
}

// pngImpl is png_impl of krilla tests/src/png.rs. The image is 200 points
// wide.
func pngImpl(name string, opts ...Option) func(t *testing.T) *Document {
	return func(t *testing.T) *Document {
		t.Helper()
		d := NewDocument(append(settings1(), opts...)...)
		ps, _ := PageSettingsFromWH(200, 200)
		page := d.StartPage(ps)
		img, err := ImageFromPNG(readImageFile(t, "png_native/"+name+".png"), false)
		if err != nil {
			t.Fatal(err)
		}
		width, height := img.Size()
		scale := 200 / float32(width)
		size, ok := path.SizeFromWH(200, float32(height)*scale)
		if !ok {
			t.Fatal("invalid size")
		}
		page.Surface().DrawImage(img, size)
		return d
	}
}

// imageSnapshotCases returns the 49 snapshot tests of images and PNG files.
// A PNG file that krilla copies without a decode gives the snapshot byte for
// byte.
func imageSnapshotCases() []imageSnapshotCase {
	cases := []imageSnapshotCase{
		{"image_rgba8_opaque", false, imageVisregImpl("rgba8_opaque.png")},
		{"image_rgba16_opaque", false, imageVisregImpl("rgba16_opaque.png")},
		{"image_rgb8", true, imageVisregImpl("rgb8.png")},
		{"image_rgb8_app14_cmyk", true, imageVisregImpl("rgb8_app14_cmyk.jpg")},
		{"image_luma8", true, imageVisregImpl("luma8.png")},
		{"image_rgba16", false, imageVisregImpl("rgba16.png")},
		{"image_luma16", true, imageVisregImpl("luma16.png")},
		{"image_deduplicate", true, func(t *testing.T) *Document {
			t.Helper()
			d := NewDocument(settings1()...)
			size := imageSize(t, loadImage(t, "luma8.png"))
			page := d.StartPage(DefaultPageSettings())
			surface := page.Surface()
			surface.DrawImage(loadImage(t, "luma8.png"), size)
			surface.DrawImage(loadImage(t, "luma8.png"), size)
			surface.Finish()
			page.Finish()

			page = d.StartPage(DefaultPageSettings())
			page.Surface().DrawImage(loadImage(t, "luma8.png"), size)
			return d
		}},
		{"image_interpolate", false, func(t *testing.T) *Document {
			t.Helper()
			img, err := ImageFromPNG(readImageFile(t, "rgba8.png"), true)
			if err != nil {
				t.Fatal(err)
			}
			d, page := snapshotPage()
			page.Surface().DrawImage(img, imageSize(t, img))
			return d
		}},
		// settings_2 of krilla tests/src/main.rs: no_device_cs.
		{"image_no_device_cs", false, func(t *testing.T) *Document {
			t.Helper()
			d := NewDocument(append(settings1(), WithNoDeviceCS(true))...)
			ps, _ := PageSettingsFromWH(200, 200)
			img := loadImage(t, "rgb8.png")
			d.StartPage(ps).Surface().DrawImage(img, imageSize(t, img))
			return d
		}},
		{"png_rgb_indexed_no_device_cs", false, pngImpl("rgb_indexed_4", WithNoDeviceCS(true))},
	}
	// The files that krilla copies without a decode give the same bytes.
	for _, name := range []string{
		"grayscale_1", "grayscale_2", "grayscale_4", "grayscale_8", "grayscale_16",
		"rgb_8", "rgb_8_plte", "rgb_16", "rgb_16_plte", "rgb_8_multiple_idat",
		"rgb_indexed_1", "rgb_indexed_2", "rgb_indexed_4", "rgb_indexed_8",
		"rgb_8_filter_none", "rgb_8_filter_sub", "rgb_8_filter_up", "rgb_8_filter_average", "rgb_8_filter_paeth",
	} {
		cases = append(cases, imageSnapshotCase{"png_" + name, true, pngImpl(name)})
	}
	// The files with an ICC profile, which pdf compresses, and the files
	// that krilla decodes.
	for _, name := range []string{
		"grayscale_8_icc", "rgb_8_icc", "grayscale_8_interlaced",
		"grayscale_trns_1", "grayscale_trns_2", "grayscale_trns_4", "grayscale_trns_8", "grayscale_trns_16",
		"rgb_trns_8", "rgb_trns_16",
		"rgb_indexed_trns_1", "rgb_indexed_trns_2", "rgb_indexed_trns_4", "rgb_indexed_trns_8", "rgb_indexed_trns_binary_8",
		"grayscale_alpha_8", "grayscale_alpha_16", "rgba_8", "rgba_16",
	} {
		cases = append(cases, imageSnapshotCase{"png_" + name, false, pngImpl(name)})
	}
	return cases
}

// TestImageSnapshots runs the 49 snapshot tests of krilla tests/src/image.rs
// and tests/src/png.rs. A case that pdf writes without a compressed stream
// must give the snapshot byte for byte. The others must give it after the
// streams are decoded.
func TestImageSnapshots(t *testing.T) {
	cases := imageSnapshotCases()
	if len(cases) != 49 {
		t.Fatalf("got %d cases, want 49", len(cases))
	}
	exact := 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if imageSnapshot(t, c.name, c.run(t)) {
				exact++
			} else if c.exact {
				t.Errorf("%s gives the snapshot only after the streams are decoded", c.name)
			}
		})
	}
	t.Logf("%d of %d snapshots are the same byte for byte, and the others after the streams are decoded", exact, len(cases))
}

// TestPNGNativeBytes checks that the stream of a PNG file that pdf copies
// without a decode holds the data of the chunks IDAT of the file.
func TestPNGNativeBytes(t *testing.T) {
	for _, name := range []string{"rgb_8", "rgb_8_multiple_idat", "rgb_indexed_2", "grayscale_16", "rgb_8_icc"} {
		data := readImageFile(t, "png_native/"+name+".png")
		pd, ok := newPNGData(data)
		if !ok {
			t.Fatalf("%s: no data", name)
		}
		img, err := ImageFromPNGNative(data, false)
		if err != nil {
			t.Fatal(err)
		}
		d := NewDocument(WithCompression(false))
		ps, _ := PageSettingsFromWH(100, 100)
		d.StartPage(ps).Surface().DrawImage(img, imageSize(t, img))
		var buf bytes.Buffer
		if err := d.Finish(&buf); err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(buf.Bytes(), append(append([]byte("stream\n"), pd.idat...), "\nendstream"...)) {
			t.Errorf("%s: the file does not hold the data of the chunks IDAT", name)
		}
	}
}

// TestImageErrors checks the errors that Document.Finish returns for an
// image: a sixteen bit image in PDF 1.4, an image that does not decode, and
// a nil image.
func TestImageErrors(t *testing.T) {
	draw := func(img *Image, opts ...Option) error {
		d := NewDocument(opts...)
		ps, _ := PageSettingsFromWH(100, 100)
		size, _ := path.SizeFromWH(10, 10)
		d.StartPage(ps).Surface().DrawImage(img, size)
		return d.Finish(&bytes.Buffer{})
	}
	if err := draw(loadImage(t, "luma16.png"), WithPDFVersion(PDF14)); !errors.Is(err, okage.ErrPDFSixteenBitImage) {
		t.Errorf("a sixteen bit image in PDF 1.4 gives %v", err)
	}
	if err := draw(loadImage(t, "luma16.png"), WithPDFVersion(PDF15)); err != nil {
		t.Errorf("a sixteen bit image in PDF 1.5 gives %v", err)
	}
	// An image whose IDAT is cut off decodes with an error.
	data := readImageFile(t, "rgba8.png")
	i := bytes.Index(data, []byte("IDAT"))
	broken := append(append([]byte{}, data[:i+100]...), data[len(data)-12:]...)
	img, err := ImageFromPNG(broken, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := draw(img); !errors.Is(err, okage.ErrPDFImage) {
		t.Errorf("a broken PNG file gives %v", err)
	}
	if err := draw(nil); !errors.Is(err, okage.ErrPDFImage) {
		t.Errorf("a nil image gives %v", err)
	}
}

// TestImageLimits checks that an image whose header gives a large size
// returns an error before it allocates the samples.
func TestImageLimits(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	// Give the header the size 20000 by 20000, and fix its CRC.
	data[16], data[17], data[18], data[19] = 0, 0, 0x4e, 0x20
	data[20], data[21], data[22], data[23] = 0, 0, 0x4e, 0x20
	fixCRC(data[8:33])
	// On a machine of 32 bits, image/png finds that the size overflows an
	// int, and ImageFromPNG returns the error.
	img, err := ImageFromPNG(data, false)
	switch {
	case err != nil:
		if !errors.Is(err, okage.ErrPDFImage) {
			t.Errorf("got %v", err)
		}
	case img.err == nil || !strings.Contains(img.err.Error(), "too large"):
		t.Errorf("got %v", img.err)
	}
	if _, err := ImageFromImage(largeImage{}, false); !errors.Is(err, okage.ErrPDFImage) {
		t.Errorf("a large image gives %v", err)
	}
}

// largeImage is an image of 10000 by 10000 pixels of 16 bits, which holds
// no pixels.
type largeImage struct{}

func (largeImage) ColorModel() color.Model { return color.NRGBA64Model }
func (largeImage) Bounds() image.Rectangle { return image.Rect(0, 0, 10000, 10000) }
func (largeImage) At(int, int) color.Color { return color.NRGBA64{} }

// fixCRC writes the CRC of a chunk of a PNG file, which holds the length,
// the type, the data and the CRC.
func fixCRC(chunk []byte) {
	crc := crc32.ChecksumIEEE(chunk[4 : len(chunk)-4])
	binary.BigEndian.PutUint32(chunk[len(chunk)-4:], crc)
}

// TestImageFromImage draws the images of Go in each color model, and checks
// the color space and the bits of the image.
func TestImageFromImage(t *testing.T) {
	r := image.Rect(0, 0, 3, 2)
	nrgba := image.NewNRGBA(r)
	nrgba.SetNRGBA(1, 1, color.NRGBA{10, 20, 30, 40})
	for _, c := range []struct {
		img      image.Image
		cs       imageColorSpace
		bpc      bitsPerComponent
		hasAlpha bool
	}{
		{image.NewGray(r), imageColorSpaceLuma, bitsPerComponentEight, false},
		{image.NewGray16(r), imageColorSpaceLuma, bitsPerComponentSixteen, false},
		{image.NewCMYK(r), imageColorSpaceCMYK, bitsPerComponentEight, false},
		{nrgba, imageColorSpaceRGB, bitsPerComponentEight, true},
		{image.NewNRGBA64(r), imageColorSpaceRGB, bitsPerComponentSixteen, true},
		{image.NewRGBA(r), imageColorSpaceRGB, bitsPerComponentEight, true},
		{image.NewYCbCr(r, image.YCbCrSubsampleRatio420), imageColorSpaceRGB, bitsPerComponentEight, false},
		{image.NewUniform(color.Gray{9}), imageColorSpaceLuma, bitsPerComponentEight, false},
	} {
		img, err := ImageFromImage(c.img, false)
		if _, ok := c.img.(*image.Uniform); ok {
			// A uniform image has infinite bounds.
			if err == nil {
				t.Errorf("%T gives no error", c.img)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if img.err != nil {
			t.Fatalf("%T: %v", c.img, img.err)
		}
		m := img.metadata
		if m.colorSpace != c.cs || m.bitsPerComponent != c.bpc || m.hasAlpha != c.hasAlpha {
			t.Errorf("%T gives %+v", c.img, m)
		}
		d := NewDocument()
		ps, _ := PageSettingsFromWH(100, 100)
		d.StartPage(ps).Surface().DrawImage(img, imageSize(t, img))
		if err := d.Finish(&bytes.Buffer{}); err != nil {
			t.Errorf("%T: %v", c.img, err)
		}
	}
}

// TestImageFromJPEGWithICC checks imageFromJPEGWithICC, which upstream hides.
// A profile of the color space of the image replaces the profile of the
// file, and a profile of another color space is ignored.
func TestImageFromJPEGWithICC(t *testing.T) {
	data := readImageFile(t, "rgb8.jpg")
	img, err := imageFromJPEGWithICC(data, srgbV4ICCData, false)
	if err != nil {
		t.Fatal(err)
	}
	if img.metadata.icc.profile == nil {
		t.Error("the image has no profile")
	}
	img, err = imageFromJPEGWithICC(data, greyV4ICCData, false)
	if err != nil {
		t.Fatal(err)
	}
	if img.metadata.icc.profile != nil {
		t.Error("the image has a gray profile")
	}
}
