package language

import (
	"encoding/binary"
	"fmt"
)

// Script identifies different writing systems.
// It is represented as the binary encoding of a script tag of 4 (case sensitive) letters,
// as specified by ISO 15924.
// Note that the default value is usually the Unknown script, not the 0 value (which is invalid)
type Script uint32

// ParseScript converts a 4 bytes string into its binary encoding,
// enforcing the conventional capitalized case.
// If [script] is longer, only its 4 first bytes are used.
func ParseScript(script string) (Script, error) {
	if len(script) < 4 {
		return 0, fmt.Errorf("invalid script string: %s", script)
	}
	s := binary.BigEndian.Uint32([]byte(script))
	// ensure capitalized case : make first letter upper, others lower
	const mask uint32 = 0x20000000
	return Script(s & ^mask | 0x00202020), nil
}

// LookupScript looks up the script for a particular character (as defined by
// Unicode Standard Annex #24), and returns Unknown if not found.
func LookupScript(r rune) Script {
	// okage: D72.
	if uint32(r) < uint32(len(latin1Scripts)) {
		return latin1Scripts[r]
	}
	return lookupScriptBlock(r)
}

// okage: D72. scriptBlockBits is the number of the low bits of a rune that
// select the rune in its block of scriptBlocks. A block holds 256 runes.
const scriptBlockBits = 8

// okage: D72. scriptBlocks holds the index in [ScriptRanges] of the first
// range that ends in each block of 256 runes or after it, up to the block
// of the last range, and one more item for the block after it. The range
// that holds a rune of the block b, if one does, is one of the ranges
// scriptBlocks[b] to scriptBlocks[b+1], both included. In most blocks they
// are one or two ranges.
var scriptBlocks = func() []uint16 {
	const _ = uint16(len(ScriptRanges)) // the indices fit in a uint16
	n := int(ScriptRanges[len(ScriptRanges)-1].End>>scriptBlockBits) + 2
	t := make([]uint16, n)
	i := 0
	for b := range t {
		for i < len(ScriptRanges) && ScriptRanges[i].End < rune(b)<<scriptBlockBits {
			i++
		}
		t[b] = uint16(i)
	}
	return t
}()

// okage: D72. lookupScriptBlock is lookupScript with a binary search over
// the ranges of the block of r in scriptBlocks only, which gives the same
// script.
func lookupScriptBlock(r rune) Script {
	b := uint32(r) >> scriptBlockBits
	if b+1 >= uint32(len(scriptBlocks)) {
		return Unknown
	}
	for i, j := int(scriptBlocks[b]), min(int(scriptBlocks[b+1])+1, len(ScriptRanges)); i < j; {
		h := i + (j-i)/2
		entry := &ScriptRanges[h]
		if r < entry.Start {
			j = h
		} else if entry.End < r {
			i = h + 1
		} else {
			return entry.Script
		}
	}
	return Unknown
}

// okage: D72. latin1Scripts is the script of each rune below U+0100, from
// the general lookup.
var latin1Scripts = func() (t [0x100]Script) {
	for r := range t {
		t[r] = lookupScript(rune(r))
	}
	return t
}()

// lookupScript is the general lookup of [LookupScript]. okage: D72. Only
// latin1Scripts and the tests call it.
func lookupScript(r rune) Script {
	// binary search
	for i, j := 0, len(ScriptRanges); i < j; {
		h := i + (j-i)/2
		entry := ScriptRanges[h]
		if r < entry.Start {
			j = h
		} else if entry.End < r {
			i = h + 1
		} else {
			return entry.Script
		}
	}
	return Unknown
}

// String returns the ISO 4 lower letters code of the script
func (s Script) String() string {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(s))
	return string(buf[:])
}

// Strong returns true if the script is not Common or Inherited
func (s Script) Strong() bool {
	return s != Common && s != Inherited
}
