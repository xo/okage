// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package lottie

// This file ports no file of velato. It holds the options of the PixmapSink
// and the code that decodes and draws an image asset (D75). velato leaves the
// decoding of an image to the sink.

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io/fs"
	"log/slog"
	"math"
	"net/url"
	"path"
	"strings"
	"sync"

	"golang.org/x/image/webp"

	"github.com/xo/okage"
	"github.com/xo/okage/kurbo"
	okagepath "github.com/xo/okage/path"
)

// SinkOption is an option of a PixmapSink (D20).
type SinkOption func(*PixmapSink)

// WithFS sets the files that the sink reads an image asset from, when the
// asset names a path and not a data URL. The path is the directory of the
// asset, its key u, followed by its file name, its key p, without a leading
// slash. DotLottie.FS gives the files of a dotLottie archive. Without this
// option, the sink draws nothing for such an asset. velato has no such field,
// because it leaves the images to the sink (D75).
func WithFS(fsys fs.FS) SinkOption {
	return func(s *PixmapSink) {
		s.fsys = fsys
	}
}

// WithLogger sets the logger that the sink logs to, for an image asset that
// it cannot read or decode. Without this option, the sink logs nothing.
// velato has no such field (D20, D75).
func WithLogger(l *slog.Logger) SinkOption {
	return func(s *PixmapSink) {
		s.logger = l
	}
}

// withImageCache makes the sink keep its decoded images in c, so that the
// sinks of several frames decode each image once.
func withImageCache(c *imageCache) SinkOption {
	return func(s *PixmapSink) {
		if c != nil {
			s.images = c
		}
	}
}

// imageCache holds the decoded images of the image assets, by the location
// of each asset. A nil pixmap marks an image that does not decode.
type imageCache struct {
	mu     sync.Mutex
	images map[string]*okage.Pixmap
}

// newImageCache returns an empty imageCache.
func newImageCache() *imageCache {
	return &imageCache{images: map[string]*okage.Pixmap{}}
}

// image returns the decoded image of asset, or nil when it does not decode.
// It decodes each image once, and logs the fault of an image that does not
// decode to logger.
func (c *imageCache) image(asset *ImageAsset, fsys fs.FS, logger *slog.Logger) *okage.Pixmap {
	location := asset.Location()
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.images[location]; ok {
		return p
	}
	p, err := loadImage(asset, fsys)
	if err != nil && logger != nil {
		logger.Warn("lottie: cannot draw an image asset", "id", asset.ID, "error", err)
	}
	c.images[location] = p
	return p
}

// loadImage reads and decodes the image of asset, from its data URL or from
// fsys.
func loadImage(asset *ImageAsset, fsys fs.FS) (*okage.Pixmap, error) {
	var data []byte
	var err error
	if asset.IsDataURL() {
		data, err = decodeDataURL(asset.FileName)
		if err != nil {
			return nil, err
		}
		return decodeImage(data)
	}
	if fsys == nil {
		return nil, errors.New("reading image: the sink has no files")
	}
	name := path.Clean(strings.TrimPrefix(asset.Location(), "/"))
	data, err = fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("reading image %s: %w", name, err)
	}
	return decodeImage(data)
}

// decodeDataURL returns the bytes of a data URL, such as
// "data:image/png;base64,iVBORw0...". The data is base64 when the media type
// ends with ";base64", and percent-encoded when it does not.
func decodeDataURL(s string) ([]byte, error) {
	header, body, ok := strings.Cut(strings.TrimPrefix(s, "data:"), ",")
	if !ok {
		return nil, errors.New("decoding data url: no comma")
	}
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(header)), ";base64") {
		text, err := url.PathUnescape(body)
		if err != nil {
			return nil, fmt.Errorf("decoding data url: %w", err)
		}
		return []byte(text), nil
	}
	// Base64 in a data URL can hold white space and can omit the padding.
	body = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '=':
			return -1
		}
		return r
	}, body)
	data, err := base64.RawStdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("decoding data url: %w", err)
	}
	return data, nil
}

// decodeImage decodes a PNG, JPEG, GIF or WebP image into a pixmap. It finds
// the format from the first bytes of data, and not from a media type or a
// file name. A GIF image gives its first frame.
func decodeImage(data []byte) (*okage.Pixmap, error) {
	r := bytes.NewReader(data)
	var img image.Image
	var err error
	switch {
	case bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		img, err = png.Decode(r)
	case bytes.HasPrefix(data, []byte("\xff\xd8")):
		img, err = jpeg.Decode(r)
	case bytes.HasPrefix(data, []byte("GIF8")):
		img, err = gif.Decode(r)
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		img, err = webp.Decode(r)
	default:
		return nil, errors.New("decoding image: unknown format")
	}
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || b.Dx() > math.MaxInt32 || b.Dy() > math.MaxInt32 {
		return nil, fmt.Errorf("decoding image: invalid size %dx%d", b.Dx(), b.Dy())
	}
	p, ok := okage.NewPixmap(uint32(b.Dx()), uint32(b.Dy()))
	if !ok {
		return nil, fmt.Errorf("decoding image: invalid size %dx%d", b.Dx(), b.Dy())
	}
	// An image.RGBA is premultiplied, as a pixmap is, so draw.Draw converts
	// each pixel.
	draw.Draw(p.AsRGBA(), p.Bounds(), img, b.Min, draw.Src)
	return p, nil
}

// DrawImage draws the image of asset under transform, with its opacity
// scaled by alpha, as ThorVG draws it (D75). The top-left corner of the
// image is at the origin. The image is scaled to fit in the width and the
// height of the asset, and it keeps its aspect ratio, as ThorVG does. A side
// that the asset does not give takes the size of the decoded image. The sink decodes a data URL, or reads the path of the
// asset from the files of WithFS. It decodes each image once, and keeps it
// for the next frames. An image that the sink cannot read or decode draws
// nothing, and the sink logs it to the logger of WithLogger.
func (s *PixmapSink) DrawImage(asset *ImageAsset, transform kurbo.Affine, alpha float64) {
	target := s.current()
	if target == nil {
		return
	}
	p := s.images.image(asset, s.fsys, s.logger)
	if p == nil {
		return
	}
	iw, ih := float64(p.Width()), float64(p.Height())
	w, h := iw, ih
	if asset.Width != nil && *asset.Width > 0 {
		w = *asset.Width
	}
	if asset.Height != nil && *asset.Height > 0 {
		h = *asset.Height
	}
	// ThorVG keeps the aspect ratio of the image: it scales both sides by
	// the smaller of the two ratios.
	scale := math.Min(w/iw, h/ih)
	ts := transform.Mul(kurbo.AffineScale(scale))
	rect, ok := okagepath.RectFromXYWH(0, 0, float32(iw), float32(ih))
	if !ok {
		return
	}
	s.touch(s.bounds(ts, kurbo.NewRect(0, 0, iw, ih).PathElements(0.1), 0))
	// ThorVG samples a scaled image with a bilinear filter.
	paint := okage.DefaultPaint()
	paint.Shader = okage.NewPattern(p.AsRef(), okage.SpreadModePad, okage.FilterQualityBilinear, clampF32(float32(alpha)), okagepath.IdentityTransform())
	paint.AntiAlias = true
	target.FillRect(rect, &paint, okageTransform(ts), nil)
}
