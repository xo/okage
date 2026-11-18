// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"slices"
	"testing"
)

// This file ports the tests of subsetter src/cff/remapper.rs.

func TestRemap1(t *testing.T) {
	sidRemapper := newSIDRemapper()
	for _, tc := range []struct {
		s    string
		want stringID
	}{
		{"hi", 391},
		{"there", 392},
		{"hi", 391},
		{"test", 393},
	} {
		if got := sidRemapper.remap([]byte(tc.s)); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.s, got, tc.want)
		}
	}

	var got []string
	for s := range sidRemapper.sortedStrings() {
		got = append(got, string(s))
	}
	if want := []string{"hi", "there", "test"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemapWithOldSID(t *testing.T) {
	s := newSIDRemapper()
	s.remap([]byte("Adobe"))
	if got := s.remapWithOldSID(400, []byte("Adobe")); got != 391 {
		t.Errorf("got %d", got)
	}
	if got := s.remapWithOldSID(401, []byte("x")); got != 392 {
		t.Errorf("got %d", got)
	}
	if got := s.remapWithOldSID(401, []byte("y")); got != 392 {
		t.Errorf("got %d", got)
	}
	if got, ok := s.getNewSID(400); !ok || got != 391 {
		t.Errorf("getNewSID(400) = %d, %v", got, ok)
	}
	if _, ok := s.getNewSID(402); ok {
		t.Error("getNewSID(402) finds a SID")
	}
}
