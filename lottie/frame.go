package lottie

// This file ports no file of velato. It holds the helpers that count, time
// and draw the frames of a composition (D44).

import (
	"fmt"
	"image"
	"math"
	"time"

	"github.com/xo/okage"
)

// FrameCount returns the number of frames of c, from Frames[0] up to and not
// including Frames[1]. It returns 0 for a composition with no frames.
func (c *Composition) FrameCount() int {
	n := math.Floor(c.Frames[1] - c.Frames[0])
	if !(n > 0) || n > math.MaxInt32 {
		return 0
	}
	return int(n)
}

// FrameDuration returns the time that one frame of c shows, from the frame
// rate. It returns 0 for a frame rate that is not finite and positive.
func (c *Composition) FrameDuration() time.Duration {
	if !(c.FrameRate > 0) || math.IsInf(c.FrameRate, 1) {
		return 0
	}
	return time.Duration(float64(time.Second) / c.FrameRate)
}

// Duration returns the time that all the frames of c show.
func (c *Composition) Duration() time.Duration {
	return time.Duration(c.FrameCount()) * c.FrameDuration()
}

// Image draws frame n of c, counted from Frames[0], into a new image of w by
// h pixels. It scales the width of the composition by w/Width and its height
// by h/Height, so the composition fills the image. The image is
// premultiplied, as an image.RGBA is. Image returns an error that wraps
// okage.ErrLottieEmpty when c has no size or no frames, when n is not one of
// its frames, or when w or h is not positive.
func (c *Composition) Image(n, w, h int) (*image.RGBA, error) {
	if c.Width == 0 || c.Height == 0 || n < 0 || n >= c.FrameCount() {
		return nil, fmt.Errorf("lottie frame %d of %d: %w", n, c.FrameCount(), okage.ErrLottieEmpty)
	}
	if w <= 0 || h <= 0 || w > math.MaxInt32 || h > math.MaxInt32 {
		return nil, fmt.Errorf("lottie image %dx%d: %w", w, h, okage.ErrLottieEmpty)
	}
	pm, ok := okage.NewPixmap(uint32(w), uint32(h))
	if !ok {
		return nil, fmt.Errorf("lottie image %dx%d: %w", w, h, okage.ErrLottieEmpty)
	}
	m := pm.AsMut()
	s := AffineScaleNonUniform(float64(w)/float64(c.Width), float64(h)/float64(c.Height))
	if err := RenderFrame(c, c.Frames[0]+float64(n), s, &m); err != nil {
		return nil, fmt.Errorf("lottie frame %d: %w", n, err)
	}
	return pm.AsRGBA(), nil
}
