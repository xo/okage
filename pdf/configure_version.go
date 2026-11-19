// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	"strconv"

	"github.com/xo/okage/internal/pdfwriter"
)

// PDFVersion is PdfVersion in krilla src/configure/version.rs. The versions
// are in their order, so that a comparison of two versions is the comparison
// of upstream.
//
// The version of a PDF document.
type PDFVersion uint8

// The values of PDFVersion. The default of a document is PDF17.
const (
	// PDF14 is PdfVersion::Pdf14, PDF 1.4.
	PDF14 PDFVersion = iota
	// PDF15 is PdfVersion::Pdf15, PDF 1.5.
	PDF15
	// PDF16 is PdfVersion::Pdf16, PDF 1.6.
	PDF16
	// PDF17 is PdfVersion::Pdf17, PDF 1.7.
	PDF17
	// PDF20 is PdfVersion::Pdf20, PDF 2.0.
	PDF20
)

// The limits of PDFVersion.
const (
	// PDFVersionMin is PdfVersion::MIN. The minimum supported PDF version.
	PDFVersionMin = PDF14
	// PDFVersionMax is PdfVersion::MAX. The maximum supported PDF version.
	PDFVersionMax = PDF20
)

// String is PdfVersion::as_str in krilla src/configure/version.rs.
//
// Get a string representation of the PDF version.
func (v PDFVersion) String() string {
	switch v {
	case PDF14:
		return "PDF 1.4"
	case PDF15:
		return "PDF 1.5"
	case PDF16:
		return "PDF 1.6"
	case PDF17:
		return "PDF 1.7"
	case PDF20:
		return "PDF 2.0"
	}
	return "PDFVersion(" + strconv.Itoa(int(v)) + ")"
}

// The method write_xmp of PdfVersion is for the XMP metadata, which the base
// port leaves out (D62). The methods rgb_icc and grey_icc give the ICC
// profiles of graphics/icc.rs, and the port of that file adds them. The
// methods supports_icc and supports_bit_depth are for the images, and they
// are in graphics_image.go. specifies_associated_files is for the
// embedded files, which the base port leaves out.

// setVersion is PdfVersion::set_version in krilla src/configure/version.rs.
func (v PDFVersion) setVersion(pdf *pdfwriter.Pdf) {
	switch v {
	case PDF14:
		pdf.SetVersion(1, 4)
	case PDF15:
		pdf.SetVersion(1, 5)
	case PDF16:
		pdf.SetVersion(1, 6)
	case PDF17:
		pdf.SetVersion(1, 7)
	case PDF20:
		pdf.SetVersion(2, 0)
	}
}

// deprecatesProcSets is PdfVersion::deprecates_proc_sets in krilla
// src/configure/version.rs.
func (v PDFVersion) deprecatesProcSets() bool {
	return v >= PDF20
}

// deprecatesCIDSet is PdfVersion::deprecates_cid_set in krilla
// src/configure/version.rs.
func (v PDFVersion) deprecatesCIDSet() bool {
	return v >= PDF20
}
