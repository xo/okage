# D75. lottie draws image assets in the pixmap sink

Status: Decided, amends D41.

velato parses the image assets and the image layers of a Lottie file, and
gives each image to the sink with `DrawImage`. velato leaves the decoding of
the image to the sink. `PixmapSink`, the sink of okage, draws nothing for an
image (item 3 of "lottie" in `docs/BACKLOG.md`). A file with a picture then
draws without it, and ThorVG draws it.

## The decision

1. `PixmapSink.DrawImage` draws an image asset with the transform and the
   alpha that velato gives, as ThorVG draws it.
2. The sink decodes an image that the asset embeds as a `data:` URL. It
   decodes PNG, JPEG and GIF with the standard library, and WebP with
   `golang.org/x/image/webp` (D7).
3. For an image that the asset names by a path, the sink reads the file from
   an `fs.FS`. `DotLottie.FS` gives the files of a dotLottie archive (D44).
   A functional option of the sink sets the `fs.FS` (D20). Without it, the
   sink draws nothing for such an image, as before.
4. The sink decodes each image once and keeps it for the next frames.
5. An image that does not decode draws nothing. The sink logs it to the
   logger of its options, if one is set (D20).
6. A test draws a file with an embedded image and a dotLottie archive with
   an image file, and compares each frame with ThorVG within the tolerance
   of D43.

## When upstream changes

If velato decodes images itself, follow these steps:

1. Port the commit of velato as `docs/UPSTREAM.md` says.
2. Make `PixmapSink.DrawImage` draw the image that velato gives, and remove
   the decoding of this decision that the commit replaces.

## Why

Image assets are a common part of Lottie files, such as stickers and
banners. The decoders are already in okage for `svg`, so the sink needs
only the drawing.
