# D36. `svg` has a renderer that loads the system fonts

Status: Decided, amends D9.

`github.com/xo/resvg` wraps resvg through cgo. A program renders a file with
one call to it, and resvg finds the fonts of the system with fontconfig. To
render a file with `svg`, a program must build a font database, set the
generic families, parse the file, make a pixmap, fill the background and call
`Render`. iv moved to okage, and the maintainers want that work in okage, so
that a program that used `xo/resvg` can move with few changes.

## The decision

1. `svg` has a type `Renderer`, which `New` makes with the options of the
   package. `Renderer.Render` parses SVG data, plain or compressed with gzip,
   and returns an `*image.RGBA`. `Renderer.Parse` returns a `Document`, which
   gives the size of the image with `Config` and draws it with `Render`, as
   `xo/resvg` does. A `Renderer` is safe for concurrent use, and its renders
   share one font database.
2. The renderer uses the type `Option` of the parser. The options of the
   renderer are `WithBackground`, `WithWidth`, `WithHeight`, `WithScaleMode`,
   `WithTransform` and `WithLoadSystemFonts`. `FromData` and the other
   functions of the parser ignore them. `ScaleMode` computes the size and the
   scale as `xo/resvg` does.
3. By default, the renderer loads the fonts of the system and of the home
   directory. It loads them when the first call needs them, never in `New` and
   never when the package starts, so that a program that imports `svg` and
   renders no file does not pay for them. On Windows, the root of the file
   system is the volume of `%WINDIR%`. This is the one place outside
   `internal/wide` where the behavior depends on the platform.
4. `svg` keeps a list of candidates for each generic family of CSS. The
   renderer sets each generic family to the first candidate that the font
   database holds, and sets the default family of the parser to the family of
   serif. It reads no configuration of fontconfig. An option that sets a
   family wins. With `WithFontDatabase`, the renderer loads no font and sets
   no family, because the caller owns that database.
5. `Decode` and `DecodeConfig` render with a default renderer, and the
   package registers them with `image.RegisterFormat` for the format `svg`.
   The default renderer loads fonts only on its first call.
6. The code is in `svg/renderer.go`. It ports no upstream file.

## Why

fontconfig can resolve a generic family to a font that does not draw Latin
letters. On systems with the configuration of the urw fonts, resvg resolved
serif to Standard Symbols PS and drew Latin text as Greek letters. A fixed
list of common fonts gives the same result on each system that has them.

`xo/resvg` made its default renderer when the package started, and that loaded
every system font. It added about 30 ms to each run of iv, even to
`iv --version`.

## What changes

D9 said that `svg` reads no font of the system by itself. The functions of the
parser still do not, and only the renderer does. Hard rule 8 of `AGENTS.md`
names the renderer as the one other place that depends on the platform.
