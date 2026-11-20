// Package lottie renders Lottie animations with okage. It is a port of the
// crate velato, with the changes of D42, D43, D79, D82 and D85 (D41).
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
// PixmapSink is the Sink that draws with okage. It draws the image assets
// that a file embeds as data URLs, and the image files of the fs.FS of
// WithFS. RenderFrame and Composition.Image read the image files of a
// dotLottie archive from the archive (D75).
//
// The package also draws the text layers, which velato does not parse. A
// text layer draws the glyphs of the list chars of its file. A PixmapSink
// shapes each other character with the fonts of WithFontDatabase. A text
// layer also draws its text animators, the capitals of its text and its
// text on a path, as ThorVG does (D85). The
// PixmapSink also draws the Gaussian blur and the drop shadow of a layer, as
// ThorVG does (D79):
//
//	sink := lottie.NewPixmapSink(&m, lottie.WithFontDatabase(db))
//	err := lottie.NewRenderer().Append(c, frame, lottie.IdentityAffine, 1, sink)
//
// The package draws as ThorVG does where velato differs, as D43 lists. The
// test module compares it with velato and with ThorVG on the Noto animated
// emoji.
package lottie
