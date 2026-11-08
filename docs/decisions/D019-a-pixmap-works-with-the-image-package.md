# D19. A `Pixmap` works with the package `image`

Status: Decided.

A `Pixmap` works with the package `image` of the standard library in two
ways:

1. A method returns an `*image.RGBA` that shares the bytes of the `Pixmap`.
   A change to one is a change to the other.
2. `Pixmap` implements `image.Image` and `draw.Image`.

The bytes of a `Pixmap` have the layout of an `image.RGBA`: four bytes for
each pixel, in the order red, green, blue and alpha, premultiplied by the
alpha. So neither way converts a pixel.

These are APIs that upstream does not have. They port no Rust function, and
`README.md` lists them.
