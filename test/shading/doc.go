// Package shading compares the radial gradients of
// okage.NewRadialGradientExtend with MuPDF `mutool draw` (D64).
//
// Each test writes a small PDF file with one radial shading of type 3, with
// the circles of the test and one value of the key Extend. MuPDF renders the
// page at 72 dots for each inch, so that a pixel of the page is a pixel of
// okage. okage fills a white pixmap of the same size with the same gradient.
// The test then counts the pixels whose channels differ by more than a
// tolerance. MuPDF draws a radial shading as rings of small shapes, with a
// table of 256 colors and with anti-aliasing at the edge of the shading, so
// the two images are not the same bytes.
//
// The tests skip when mutool is not installed. The package has no code of
// its own.
package shading
