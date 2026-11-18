// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import "iter"

// This file ports subsetter src/cff/remapper.rs.

// fontDictRemapper is FontDictRemapper in subsetter src/cff/remapper.rs.
type fontDictRemapper = remapper[uint8, uint8]

// sidRemapper is SidRemapper in subsetter src/cff/remapper.rs. It remaps old
// SIDs to new SIDs, and also allows the insertion of new strings. Make one
// with newSIDRemapper.
//
// Upstream keeps the strings in a BTreeMap from SIDs to strings. The SIDs
// that remap gives follow each other from 391, so the port keeps the strings
// in a slice, in the order of their SIDs.
type sidRemapper struct {
	// counter is the next SID to be assigned.
	counter stringID
	// strings holds the string of each SID from 391, in the order of the
	// SIDs.
	strings [][]byte
	// stringToSID maps strings to their corresponding SID (so the reverse
	// of strings).
	stringToSID map[string]stringID
	// oldSIDToNewSID maps old SIDs to new SIDs.
	oldSIDToNewSID map[stringID]stringID
}

// newSIDRemapper is SidRemapper::new in subsetter src/cff/remapper.rs.
func newSIDRemapper() *sidRemapper {
	return &sidRemapper{
		counter:        standardStringLen,
		stringToSID:    map[string]stringID{},
		oldSIDToNewSID: map[stringID]stringID{},
	}
}

// get is SidRemapper::get in subsetter src/cff/remapper.rs.
func (s *sidRemapper) get(str []byte) (stringID, bool) {
	sid, ok := s.stringToSID[string(str)]
	return sid, ok
}

// getNewSID is SidRemapper::get_new_sid in subsetter src/cff/remapper.rs. It
// gets the new SID to a corresponding old SID.
func (s *sidRemapper) getNewSID(sid stringID) (stringID, bool) {
	v, ok := s.oldSIDToNewSID[sid]
	return v, ok
}

// remap is SidRemapper::remap in subsetter src/cff/remapper.rs. It remaps a
// string.
//
// As upstream, it panics when the counter overflows. The subset of a CFF
// table remaps at most 261 strings, so it cannot overflow.
func (s *sidRemapper) remap(str []byte) stringID {
	if sid, ok := s.stringToSID[string(str)]; ok {
		return sid
	}
	value := s.counter
	s.strings = append(s.strings, str)
	if s.counter == 0xFFFF {
		panic("sid remapper overflowed")
	}
	s.counter++
	s.stringToSID[string(str)] = value
	return value
}

// remapWithOldSID is SidRemapper::remap_with_old_sid in subsetter
// src/cff/remapper.rs. It remaps an old SID and its corresponding string.
func (s *sidRemapper) remapWithOldSID(sid stringID, str []byte) stringID {
	if newSID, ok := s.oldSIDToNewSID[sid]; ok {
		return newSID
	}
	newSID := s.remap(str)
	s.oldSIDToNewSID[sid] = newSID
	return newSID
}

// sortedStrings is SidRemapper::sorted_strings in subsetter
// src/cff/remapper.rs. It returns an iterator over the strings, ordered by
// their new SID.
func (s *sidRemapper) sortedStrings() iter.Seq[[]byte] {
	return func(yield func([]byte) bool) {
		for _, str := range s.strings {
			if !yield(str) {
				return
			}
		}
	}
}
