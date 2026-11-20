// SPDX-License-Identifier: Unlicense OR BSD-3-Clause

package font

// okage: this file is not in go-text (D81).

// cmapCache4K is cache21_19_8 with 4096 entries in place of 256. The cache
// of the cmap of a Face holds the glyph of a rune. A text of Chinese or
// Japanese uses a few thousand runes, which miss in a cache of 256 entries,
// and each miss searches the cmap. The cache takes 16 KiB for each Face.
type cmapCache4K [1 << 12]uint32

// clear should be used as init function
func (c *cmapCache4K) clear() {
	for i := range c {
		c[i] = ^uint32(0)
	}
}

func (c *cmapCache4K) get(key uint32) (uint32, bool) {
	k := key & ((1 << 12) - 1)
	v := c[k]
	if v == ^uint32(0) || (v>>19) != uint32(key>>12) {
		return 0, false
	}
	return v & ((1 << 19) - 1), true
}

func (c *cmapCache4K) set(key uint32, value uint32) {
	if (key>>21) != 0 || (value>>19) != 0 { /* overflows */
		return
	}
	k := key & ((1 << 12) - 1)
	c[k] = (uint32(key>>12) << 19) | value
}
