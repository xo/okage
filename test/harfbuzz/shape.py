"""Shape texts in a font with uharfbuzz, for the oracle of typesetting/harfbuzz.

Usage: python3 -I shape.py FONT [--flags]

Each line of the standard input is a text, as the hexadecimal values of its
characters, separated by spaces. For each text, the script writes one line
for each glyph, "glyph cluster x_advance y_advance x_offset y_offset", then
the line "error" when the buffer is in error, then the line "--". With the
argument --flags, the line of a glyph ends with 1 when the glyph is unsafe
to break, and with 0 when it is not.

The script adds the characters with add_codepoints, as harfbuzz.AddRunes
does, so that the buffers of HarfBuzz and of harfbuzz grow in the same
steps. uharfbuzz raises MemoryError when the buffer is in error after the
shaping, and the script still writes the glyphs of the buffer.

With the argument --version, the script writes the version of HarfBuzz.
"""

import sys

import uharfbuzz as hb


def main():
    if sys.argv[1] == "--version":
        print(hb.version_string())
        return
    blob = hb.Blob.from_file_path(sys.argv[1])
    flags = sys.argv[2:] == ["--flags"]
    font = hb.Font(hb.Face(blob))
    out = sys.stdout
    for line in sys.stdin:
        buf = hb.Buffer()
        buf.add_codepoints([int(v, 16) for v in line.split()])
        buf.guess_segment_properties()
        error = False
        try:
            hb.shape(font, buf, {})
        except MemoryError:
            error = True
        for info, pos in zip(buf.glyph_infos, buf.glyph_positions):
            out.write(f"{info.codepoint} {info.cluster} {pos.x_advance} {pos.y_advance} {pos.x_offset} {pos.y_offset}")
            if flags:
                out.write(f" {int(info.flags & hb.GlyphFlags.UNSAFE_TO_BREAK != 0)}")
            out.write("\n")
        if error:
            out.write("error\n")
        out.write("--\n")


main()
