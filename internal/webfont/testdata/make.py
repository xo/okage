# make.py writes the test fonts of internal/webfont from the fonts of
# svg/testdata/fonts. Run it from this folder with fontTools, brotli and
# woff2_compress of google/woff2:
#
#     python3 make.py
import os
import struct
import subprocess

from fontTools import subset
from fontTools.ttLib import TTCollection, TTFont
from fontTools.ttLib.woff2 import WOFF2FlavorData

FONTS = "../../../svg/testdata/fonts/"
TEXT = "Hamburgefonstiv 0123 éÅï"


def subset_font(src, dst, text, drop=()):
    opts = subset.Options()
    opts.drop_tables += list(drop)
    opts.hinting = True
    opts.layout_features = ["*"]
    font = subset.load_font(src, opts)
    sub = subset.Subsetter(opts)
    sub.populate(text=text)
    sub.subset(font)
    font.save(dst)


def save_flavor(src, dst, flavor, transformed=None):
    # Keep the time of head, so that it is the same as in src.
    font = TTFont(src, recalcTimestamp=False)
    font.flavor = flavor
    if flavor == "woff2":
        font.flavorData = WOFF2FlavorData(transformedTables=transformed)
    font.save(dst, reorderTables=False)


def save_eot(src, dst, flags):
    data = open(src, "rb").read()
    if flags & 0x10000000:
        data = bytes(b ^ 0x50 for b in data)
    # The header of version 0x00020001, with empty names.
    header = struct.pack("<III", len(data), 0x00020001, flags)
    header += bytes(10)  # PANOSE
    header += struct.pack("<BBIHH", 1, 0, 400, 0, 0x504C)  # Charset to MagicNumber
    header += bytes(16 + 8 + 4 + 16)  # UnicodeRange to Reserved
    header += struct.pack("<HH", 0, 0) * 5  # Padding and the size of each name
    size = 4 + len(header) + len(data)
    open(dst, "wb").write(struct.pack("<I", size) + header + data)


subset_font(FONTS + "NotoSans-Regular.ttf", "noto.ttf", TEXT)
subset_font(FONTS + "NotoSans-Bold.ttf", "noto-bold.ttf", TEXT)
subset_font(FONTS + "CFF-and-SBIX.otf", "cff.otf", "AB", drop=["sbix", "DSIG"])

# woff2_compress drops the flag OVERLAP_SIMPLE, so the collection is made
# before noto.ttf has it.
coll = TTCollection()
coll.fonts = [TTFont("noto.ttf"), TTFont("noto-bold.ttf")]
coll.save("collection.ttc", shareTables=True)
subprocess.run(["woff2_compress", "collection.ttc"], check=True)

# Set OVERLAP_SIMPLE on the first point of each simple glyph of noto.ttf, so
# that the WOFF2 file holds the bitmap of overlaps.
font = TTFont("noto.ttf")
glyf = font["glyf"]
for name in glyf.keys():
    g = glyf[name]
    if g.numberOfContours > 0:
        g.flags[0] |= 0x40
font.save("noto.ttf")

save_flavor("noto.ttf", "noto.woff", "woff")
save_flavor("noto.ttf", "noto.woff2", "woff2", {"glyf", "loca", "hmtx"})
save_flavor("noto.ttf", "noto-null.woff2", "woff2", set())
save_flavor("cff.otf", "cff.woff2", "woff2")
save_eot("noto.ttf", "noto.eot", 0)
save_eot("noto.ttf", "noto-xor.eot", 0x10000000)
save_eot("noto.ttf", "noto-mtx.eot", 0x00000004)

os.remove("noto-bold.ttf")
