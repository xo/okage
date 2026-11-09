// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xo/okage/path"
)

// usvg src/parser/options.rs has no tests. These tests check the defaults of
// upstream and each option.

func TestDefaultOptions(t *testing.T) {
	o := newOptions()
	if o.resourcesDir != "" || o.dpi != 96 || o.fontFamily != "Times New Roman" || o.fontSize != 12 ||
		len(o.languages) != 1 || o.languages[0] != "en" ||
		o.shapeRendering != ShapeRenderingGeometricPrecision ||
		o.textRendering != TextRenderingOptimizeLegibility ||
		o.imageRendering != ImageRenderingOptimizeQuality ||
		o.defaultSize.Width() != 100 || o.defaultSize.Height() != 100 ||
		o.imageHrefResolver.ResolveData != nil || o.imageHrefResolver.ResolveString != nil ||
		o.styleSheet != "" || o.logger != nil {
		t.Errorf("newOptions() = %+v", o)
	}
}

func TestOptions(t *testing.T) {
	size, _ := path.SizeFromWH(10, 20)
	languages := []string{"de", "en-US"}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	o := newOptions(
		WithResourcesDir("res"),
		WithDPI(72),
		WithFontFamily("Noto Sans"),
		WithFontSize(16),
		WithLanguages(languages...),
		WithShapeRendering(ShapeRenderingCrispEdges),
		WithTextRendering(TextRenderingGeometricPrecision),
		WithImageRendering(ImageRenderingPixelated),
		WithDefaultSize(size),
		WithImageHrefResolver(ImageHrefResolver{
			ResolveString: func(string) (ImageKind, bool) { return nil, false },
		}),
		WithStyleSheet("rect { fill: red }"),
		WithLogger(logger),
	)
	languages[0] = "fr"
	if o.resourcesDir != "res" || o.dpi != 72 || o.fontFamily != "Noto Sans" || o.fontSize != 16 ||
		len(o.languages) != 2 || o.languages[0] != "de" ||
		o.shapeRendering != ShapeRenderingCrispEdges ||
		o.textRendering != TextRenderingGeometricPrecision ||
		o.imageRendering != ImageRenderingPixelated ||
		o.defaultSize != size ||
		o.imageHrefResolver.ResolveData != nil || o.imageHrefResolver.ResolveString == nil ||
		o.styleSheet != "rect { fill: red }" || o.logger != logger {
		t.Errorf("newOptions() = %+v", o)
	}

	o.warnf("'%s' is not a path to an image.", "a.png")
	o.debugf("debug %d", 1)
	if s := buf.String(); !strings.Contains(s, "level=WARN msg=\"'a.png' is not a path to an image.\"") ||
		!strings.Contains(s, "level=DEBUG msg=\"debug 1\"") {
		t.Errorf("log = %q", s)
	}

	// Without a logger, nothing is logged and nothing panics.
	newOptions().warnf("x")
	var nilOptions *options
	nilOptions.debugf("x")
}

func TestGetAbsPath(t *testing.T) {
	o := newOptions()
	if got := o.getAbsPath("img.png"); got != "img.png" {
		t.Errorf("getAbsPath without a dir = %q", got)
	}
	o = newOptions(WithResourcesDir("res"))
	if got, want := o.getAbsPath("img.png"), filepath.Join("res", "img.png"); got != want {
		t.Errorf("getAbsPath = %q, want %q", got, want)
	}
	abs, _ := filepath.Abs("img.png")
	if got := o.getAbsPath(abs); got != abs {
		t.Errorf("getAbsPath of an absolute path = %q, want %q", got, abs)
	}
}
