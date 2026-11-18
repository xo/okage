// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

// This file ports subsetter src/cff/dict/mod.rs. The DictionaryParser was
// taken from ttf-parser.

// dictionaryParser is DictionaryParser in subsetter src/cff/dict/mod.rs.
type dictionaryParser struct {
	data           []byte
	offset         int
	operandsOffset int
	operands       []number
	operandsLen    uint16
}

// newDictionaryParser is DictionaryParser::new in subsetter
// src/cff/dict/mod.rs. Upstream takes the buffer of the operands from the
// caller, and each caller gives a buffer of 48 numbers. The port makes that
// buffer.
func newDictionaryParser(data []byte) *dictionaryParser {
	return &dictionaryParser{
		data:     data,
		operands: make([]number, maxOperandsLen),
	}
}

// parseNext is DictionaryParser::parse_next in subsetter
// src/cff/dict/mod.rs.
func (p *dictionaryParser) parseNext() (operator, bool) {
	r := newReaderAt(p.data, p.offset)
	p.operandsOffset = p.offset
	for !r.atEnd() {
		peak := r
		b, ok := peak.readU8()
		if !ok {
			return 0, false
		}
		// 0..=21 bytes are operators.
		if isDictOneByteOp(b) {
			b, _ = r.readU8()
			operator := operatorFromOneByte(b)

			if b == twoByteOperatorMark {
				b2, ok := r.readU8()
				if !ok {
					return 0, false
				}
				operator = operatorFromTwoByte(b2)
			}

			p.offset = r.offset()
			return operator, true
		}
		if _, ok := parseCFFNumber(&r); !ok {
			return 0, false
		}
	}

	return 0, false
}

// parseOperands is DictionaryParser::parse_operands in subsetter
// src/cff/dict/mod.rs.
func (p *dictionaryParser) parseOperands() bool {
	r := newReaderAt(p.data, p.operandsOffset)
	p.operandsLen = 0
	for !r.atEnd() {
		peak := r
		b, ok := peak.readU8()
		if !ok {
			return false
		}
		// 0..=21 bytes are operators.
		if isDictOneByteOp(b) {
			r.readU8()
			break
		}
		op, ok := parseCFFNumber(&r)
		if !ok {
			return false
		}
		p.operands[p.operandsLen] = op
		p.operandsLen++

		if int(p.operandsLen) >= len(p.operands) {
			break
		}
	}

	return true
}

// operandsSlice is DictionaryParser::operands in subsetter
// src/cff/dict/mod.rs. The field of the buffer has the name operands.
func (p *dictionaryParser) operandsSlice() []number {
	return p.operands[:p.operandsLen]
}

// parseSID is DictionaryParser::parse_sid in subsetter src/cff/dict/mod.rs.
func (p *dictionaryParser) parseSID() (stringID, bool) {
	if !p.parseOperands() {
		return 0, false
	}
	operands := p.operandsSlice()
	if len(operands) != 1 {
		return 0, false
	}
	v, ok := operands[0].asI32()
	if !ok || v < 0 || v > 0xFFFF {
		return 0, false
	}
	return stringID(v), true
}

// parseOffset is DictionaryParser::parse_offset in subsetter
// src/cff/dict/mod.rs. The offset is a usize in Rust, which holds each u32
// on a machine of 64 bits. The port gives it as a uint64, so that it is the
// same on a machine of 32 bits.
func (p *dictionaryParser) parseOffset() (uint64, bool) {
	if !p.parseOperands() {
		return 0, false
	}
	operands := p.operandsSlice()
	if len(operands) != 1 {
		return 0, false
	}
	v, ok := operands[0].asU32()
	return uint64(v), ok
}

// parseFontBBox is DictionaryParser::parse_font_bbox in subsetter
// src/cff/dict/mod.rs.
func (p *dictionaryParser) parseFontBBox() ([4]number, bool) {
	if !p.parseOperands() {
		return [4]number{}, false
	}
	operands := p.operandsSlice()
	if len(operands) != 4 {
		return [4]number{}, false
	}
	return [4]number(operands), true
}

// parseFontMatrix is DictionaryParser::parse_font_matrix in subsetter
// src/cff/dict/mod.rs.
func (p *dictionaryParser) parseFontMatrix() ([6]number, bool) {
	if !p.parseOperands() {
		return [6]number{}, false
	}
	operands := p.operandsSlice()
	if len(operands) != 6 {
		return [6]number{}, false
	}
	return [6]number(operands), true
}

// parseRange is DictionaryParser::parse_range in subsetter
// src/cff/dict/mod.rs. The bounds of the range are a usize in Rust, and a
// uint64 in the port, as parseOffset says. The sum of two u32 values does
// not overflow either type, so the port has no test for it where Rust calls
// checked_add.
func (p *dictionaryParser) parseRange() (dataRange, bool) {
	if !p.parseOperands() {
		return dataRange{}, false
	}
	operands := p.operandsSlice()
	if len(operands) != 2 {
		return dataRange{}, false
	}
	n, ok := operands[0].asU32()
	if !ok {
		return dataRange{}, false
	}
	start, ok := operands[1].asU32()
	if !ok {
		return dataRange{}, false
	}
	return dataRange{start: uint64(start), end: uint64(start) + uint64(n)}, true
}

// dataRange is a Range<usize> of Rust, as parseRange gives it.
type dataRange struct {
	start, end uint64
}

// sliceAt is data.get(start..) of Rust. It returns false when start is
// after the end of data.
func sliceAt(data []byte, start uint64) ([]byte, bool) {
	if start > uint64(len(data)) {
		return nil, false
	}
	return data[start:], true
}

// sliceRange is data.get(start..end) of Rust. It returns false when the
// range is not in data, or when start is after end.
func sliceRange(data []byte, r dataRange) ([]byte, bool) {
	if r.start > r.end || r.end > uint64(len(data)) {
		return nil, false
	}
	return data[r.start:r.end], true
}

// newReaderAtOffset is Reader::new_at in subsetter src/read.rs for an
// offset that parseOffset gives. An offset after the end of data gives a
// reader that reads nothing, as in Rust.
func newReaderAtOffset(data []byte, offset uint64) reader {
	if offset > uint64(len(data)) {
		return newReaderAt(data, len(data)+1)
	}
	return newReaderAt(data, int(offset))
}

// isDictOneByteOp is is_dict_one_byte_op in subsetter src/cff/dict/mod.rs.
func isDictOneByteOp(b uint8) bool {
	switch {
	case b <= 27:
		return true
	case b <= 30:
		// numbers
		return false
	case b == 31:
		// Reserved
		return true
	case b <= 254:
		// numbers
		return false
	}
	// Reserved
	return true
}

// The operators of the DICTs, the module operators in subsetter
// src/cff/dict/mod.rs. A subset of the operators for DICT's we care about.
const (
	// TOP DICT OPERATORS.
	opNotice      operator = 1
	opFontBBox    operator = 5
	opCopyright   operator = twoByteOperatorMark << 8
	opFontMatrix  operator = twoByteOperatorMark<<8 | 7
	opCharset     operator = 15
	opEncoding    operator = 16
	opCharStrings operator = 17
	opPrivate     operator = 18

	// TOP DICT OPERATORS (CID FONTS).
	opROS      operator = twoByteOperatorMark<<8 | 30
	opCIDCount operator = twoByteOperatorMark<<8 | 34
	opFDArray  operator = twoByteOperatorMark<<8 | 36
	opFDSelect operator = twoByteOperatorMark<<8 | 37
	opFontName operator = twoByteOperatorMark<<8 | 38

	// PRIVATE DICT OPERATORS.
	opSubrs operator = 19
)
