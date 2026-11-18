// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import "math"

// This file ports subsetter src/cff/subroutines.rs. A charstring of
// upstream, CharString, is a []byte.

// subroutineCollection is SubroutineCollection in subsetter
// src/cff/subroutines.rs. A wrapper over a vector of subroutine containers
// (for local subroutines, where we have a list of subroutines for each font
// dict).
type subroutineCollection struct {
	subroutines []subroutineContainer
}

// newSubroutineCollection is SubroutineCollection::new in subsetter
// src/cff/subroutines.rs.
func newSubroutineCollection(subroutines [][][]byte) subroutineCollection {
	c := subroutineCollection{subroutines: make([]subroutineContainer, 0, len(subroutines))}
	for _, s := range subroutines {
		c.subroutines = append(c.subroutines, newSubroutineContainer(s))
	}
	return c
}

// getHandler is SubroutineCollection::get_handler in subsetter
// src/cff/subroutines.rs.
func (c subroutineCollection) getHandler(fdIndex uint8) (subroutineHandler, bool) {
	if int(fdIndex) >= len(c.subroutines) {
		return subroutineHandler{}, false
	}
	return c.subroutines[fdIndex].getHandler(), true
}

// subroutineContainer is SubroutineContainer in subsetter
// src/cff/subroutines.rs. A wrapper over a vector of charstrings (for global
// subroutines).
type subroutineContainer struct {
	subroutines [][]byte
}

// newSubroutineContainer is SubroutineContainer::new in subsetter
// src/cff/subroutines.rs.
func newSubroutineContainer(subroutines [][]byte) subroutineContainer {
	return subroutineContainer{subroutines: subroutines}
}

// getHandler is SubroutineContainer::get_handler in subsetter
// src/cff/subroutines.rs.
func (c subroutineContainer) getHandler() subroutineHandler {
	return newSubroutineHandler(c.subroutines)
}

// subroutineHandler is SubroutineHandler in subsetter
// src/cff/subroutines.rs. A wrapper over a list of subroutines to allow for
// convenient access to subroutines given a biased or unbiased index.
type subroutineHandler struct {
	subroutines [][]byte
	bias        uint16
}

// newSubroutineHandler is SubroutineHandler::new in subsetter
// src/cff/subroutines.rs.
func newSubroutineHandler(charStrings [][]byte) subroutineHandler {
	return subroutineHandler{
		subroutines: charStrings,
		bias:        calcSubroutineBias(uint32(len(charStrings))),
	}
}

// getWithBiased is SubroutineHandler::get_with_biased in subsetter
// src/cff/subroutines.rs.
func (h subroutineHandler) getWithBiased(index int32) ([]byte, bool) {
	unbiased, ok := unapplyBias(index, h.bias)
	if !ok {
		return nil, false
	}
	return h.getWithUnbiased(unbiased)
}

// getWithUnbiased is SubroutineHandler::get_with_unbiased in subsetter
// src/cff/subroutines.rs.
func (h subroutineHandler) getWithUnbiased(index uint32) ([]byte, bool) {
	if uint64(index) >= uint64(len(h.subroutines)) {
		return nil, false
	}
	return h.subroutines[index], true
}

// calcSubroutineBias is calc_subroutine_bias in subsetter
// src/cff/subroutines.rs.
func calcSubroutineBias(n uint32) uint16 {
	switch {
	case n < 1240:
		return 107
	case n < 33900:
		return 1131
	}
	return 32768
}

// unapplyBias is unapply_bias in subsetter src/cff/subroutines.rs. It
// unapplies the bias from a biased subroutine offset.
func unapplyBias(index int32, bias uint16) (uint32, bool) {
	b := int32(bias)

	if index > math.MaxInt32-b {
		return 0, false
	}
	v := index + b
	if v < 0 {
		return 0, false
	}
	return uint32(v), true
}
