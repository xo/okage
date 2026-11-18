// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/argstack.rs, which was taken from
// ttf-parser.

// maxOperandsLen is MAX_OPERANDS_LEN in subsetter src/cff/argstack.rs.
const maxOperandsLen = 48

// argumentsStack is ArgumentsStack in subsetter src/cff/argstack.rs.
type argumentsStack struct {
	data []number
}

// len is ArgumentsStack::len in subsetter src/cff/argstack.rs.
func (s *argumentsStack) len() int {
	return len(s.data)
}

// push is ArgumentsStack::push in subsetter src/cff/argstack.rs.
func (s *argumentsStack) push(n number) error {
	if s.len() == maxOperandsLen {
		return errCFF
	}
	s.data = append(s.data, n)
	return nil
}

// pop is ArgumentsStack::pop in subsetter src/cff/argstack.rs.
func (s *argumentsStack) pop() (number, bool) {
	if len(s.data) == 0 {
		return number{}, false
	}
	n := s.data[len(s.data)-1]
	s.data = s.data[:len(s.data)-1]
	return n, true
}

// popAll is ArgumentsStack::pop_all in subsetter src/cff/argstack.rs. It
// returns the number of the values that it removes, which is all that the
// callers of upstream use of the vector that it returns. The port keeps the
// memory of the stack for the next values.
func (s *argumentsStack) popAll() int {
	n := len(s.data)
	s.data = s.data[:0]
	return n
}
