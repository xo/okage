// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import "strconv"

// This file ports subsetter src/cff/operator.rs.

// twoByteOperatorMark is TWO_BYTE_OPERATOR_MARK in subsetter
// src/cff/operator.rs.
const twoByteOperatorMark = 12

// operator is Operator and OperatorType in subsetter src/cff/operator.rs.
// A one-byte operator b is the value b. A two-byte operator 12 b is the
// value 0x0C00 + b. The byte 12 marks a two-byte operator, so no one-byte
// operator has the value 12, and the two forms do not overlap.
type operator uint16

// operatorFromOneByte is Operator::from_one_byte in subsetter
// src/cff/operator.rs.
func operatorFromOneByte(b uint8) operator {
	return operator(b)
}

// operatorFromTwoByte is Operator::from_two_byte in subsetter
// src/cff/operator.rs.
func operatorFromTwoByte(b uint8) operator {
	return operator(twoByteOperatorMark)<<8 | operator(b)
}

// isTwoByte tells whether the operator is OperatorType::TwoByteOperator.
func (o operator) isTwoByte() bool {
	return o > 0xFF
}

// String is the impl Display of Operator in subsetter src/cff/operator.rs.
func (o operator) String() string {
	if o.isTwoByte() {
		return strconv.Itoa(twoByteOperatorMark) + strconv.Itoa(int(uint8(o)))
	}
	return strconv.Itoa(int(o))
}

// write is the impl Writeable of Operator in subsetter src/cff/operator.rs.
func (o operator) write(w *writer) {
	if o.isTwoByte() {
		w.writeU8(twoByteOperatorMark)
	}
	w.writeU8(uint8(o))
}
