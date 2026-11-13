// Package lottie renders Lottie animations with okage. It is a port of the
// crate velato, with the changes of D42 and D43 (D41).
//
// Parse reads a Lottie file into a Composition. RenderFrame draws a frame of
// it onto an okage pixmap:
//
//	c, err := lottie.Parse(data)
//	if err != nil {
//		return err
//	}
//	pm, _ := okage.NewPixmap(uint32(c.Width), uint32(c.Height))
//	m := pm.AsMut()
//	if err := lottie.RenderFrame(c, c.Frames[0], lottie.IdentityAffine, &m); err != nil {
//		return err
//	}
//
// A program that shows Lottie files can use the helpers of D44. Detect finds
// a Lottie document from the start of a file, OpenDotLottie reads a dotLottie
// archive, Load parses either one, and Composition.Image draws a frame into an
// *image.RGBA of a given size:
//
//	c, err := lottie.Load(data)
//	if err != nil {
//		return err
//	}
//	img, err := c.Image(c.FrameCount()/2, 512, 512)
//
// A Renderer draws onto any Sink, the interface of velato for a renderer.
// PixmapSink is the Sink that draws with okage.
//
// The package draws as ThorVG does where velato differs, as D43 lists. The
// test module compares it with velato and with ThorVG on the Noto animated
// emoji.
package lottie
