// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import "testing"

// TestProcessDataURL checks the port of the crate data-url 0.3.2 against
// the results of the crate.
func TestProcessDataURL(t *testing.T) {
	for _, c := range []struct {
		in       string
		ok       bool
		mime     string
		body     string
		decodeOK bool
	}{
		{"data:,Hello%20World!", true, "text/plain", "Hello World!", true},
		{" DATA:image/PNG;base64,QUJD", true, "image/png", "ABC", true},
		{"data:image/png;base64,QUI=", true, "image/png", "AB", true},
		{"data:image/png;base64,QUI", true, "image/png", "AB", true},
		{"data:image/png;base64,Q", true, "image/png", "", false},
		{"data:image/png;base64,QU I=#frag", true, "image/png", "AB", true},
		{"data:image/png;base64,QUI==", true, "image/png", "", false},
		{"data:;base64,QUJD", true, "text/plain", "ABC", true},
		{"data:image/svg+xml;charset=utf8,%3Csvg%3E%zz", true, "image/svg+xml", "<svg>%zz", true},
		{"data:bad,x", true, "text/plain", "x", true},
		{"da\nta:text/plain,x", true, "text/plain", "x", true},
		{"data:text/plain", false, "", "", false},
		{"image.png", false, "", "", false},
	} {
		u, ok := processDataURL(c.in)
		if ok != c.ok {
			t.Errorf("processDataURL(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if got := u.mimeType + "/" + u.mimeSubtype; got != c.mime {
			t.Errorf("processDataURL(%q) mime = %q, want %q", c.in, got, c.mime)
		}
		body, ok := u.decodeToVec()
		if ok != c.decodeOK || ok && string(body) != c.body {
			t.Errorf("processDataURL(%q) body = %q, %v, want %q, %v", c.in, body, ok, c.body, c.decodeOK)
		}
	}
}

// TestGetImageDataFormat checks the magic bytes that imagesize::image_type
// reads.
func TestGetImageDataFormat(t *testing.T) {
	for _, c := range []struct {
		data string
		want imageFormat
	}{
		{"\xFF\xD8\xFF\x00\x00\x00\x00\x00\x00\x00\x00\x00", imageFormatJPEG},
		{"\x89PNG\r\n\x1a\n\x00\x00\x00\x00", imageFormatPNG},
		{"GIF89a\x00\x00\x00\x00\x00\x00", imageFormatGIF},
		{"RIFF\x00\x00\x00\x00WEBP", imageFormatWEBP},
		{"\x89PNG", imageFormatNone},
		{"<svg></svg>  ", imageFormatNone},
	} {
		if got, _ := getImageDataFormat([]byte(c.data)); got != c.want {
			t.Errorf("getImageDataFormat(%q) = %v, want %v", c.data, got, c.want)
		}
	}
}
