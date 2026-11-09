// Package vips compares the images of okage with libvips, the second oracle
// of D22, through github.com/cshum/vipsgen/v2. libvips renders SVG files with
// librsvg, so the comparison uses rule 5 of D31: it counts the pixels where a
// channel differs by more than 32, reports the result of each file, and lists
// a file where more than 10 percent of the pixels differ.
//
// The tests need libvips and cgo, so they build only with the tag vips:
//
//	go test -tags vips ./vips/
//
// TestVipsPage writes an HTML page that shows the reference image of resvg,
// the image of okage, the image of libvips and the SVG file as the browser
// renders it, with the fonts of the tests embedded, side by side:
//
//	go test -tags vips -run TestVipsPage ./vips/ -args -vips.page=../.cache/vips-page
//	python3 -m http.server -d ../.cache/vips-page
//
// The package has no code of its own.
package vips
