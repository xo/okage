// Copyright 2022 The subsetter Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package subsetter

import (
	"fmt"
	"strings"
)

// This file ports subsetter src/cff/charstring.rs. A charstring of
// upstream, CharString, is a []byte.

// decompiler is Decompiler in subsetter src/cff/charstring.rs, adapted from
// fonttools. A charstring decompiler.
type decompiler struct {
	gsubrHandler  subroutineHandler
	lsubrHandler  subroutineHandler
	stack         argumentsStack
	hintCount     uint16
	hintMaskBytes uint16
	// operators counts the operators that the decompiler runs, through all
	// the calls of subroutines. Upstream has no such count.
	operators int
}

// The limits of the work of the decompiler for one glyph. Upstream has no
// limit other than a depth of 64 calls of subroutines, so 64 nested
// subroutines that each call the next one twice run 2^64 subroutines. The
// maintainers chose these limits as a difference from upstream. A glyph of
// a font has some hundreds of operators, far below them.
const (
	// maxDecompiledOperators is the largest number of operators that the
	// decompiler runs for one glyph.
	maxDecompiledOperators = 1_000_000
	// maxProgramLen is the largest number of instructions of the program
	// of one glyph.
	maxProgramLen = 1_000_000
)

// errCharStringTooLarge is the error of a glyph past a limit of the
// decompiler.
func errCharStringTooLarge() error {
	return fmt.Errorf("%w: a charstring of the CFF table runs more than %d operators or gives more than %d instructions",
		errMalformedFont, maxDecompiledOperators, maxProgramLen)
}

// charStringEnd is CharStringEnd in subsetter src/cff/charstring.rs.
type charStringEnd int

const (
	charStringEndReturn charStringEnd = iota
	charStringEndEndChar
)

// newDecompiler is Decompiler::new in subsetter src/cff/charstring.rs. It
// creates a new charstring decompiler.
func newDecompiler(gsubrHandler, lsubrHandler subroutineHandler) *decompiler {
	return &decompiler{
		gsubrHandler: gsubrHandler,
		lsubrHandler: lsubrHandler,
	}
}

// decompile is Decompiler::decompile in subsetter src/cff/charstring.rs. It
// decompiles a charstring with the given decompiler.
func (d *decompiler) decompile(charstring []byte) (program, error) {
	var p program
	if _, err := d.decompileInner(charstring, &p, 1); err != nil {
		return program{}, err
	}
	return p, nil
}

// decompileInner is Decompiler::decompile_inner in subsetter
// src/cff/charstring.rs.
func (d *decompiler) decompileInner(charstring []byte, p *program, depth uint8) (charStringEnd, error) {
	if depth > 64 {
		return 0, errCFF
	}

	r := newReader(charstring)

	for !r.atEnd() {
		// We need to peak instead of read because parsing a number requires
		// access to the whole buffer.
		peak := r
		op, ok := peak.readU8()
		if !ok {
			return 0, errCFF
		}

		// Numbers
		if op == 28 || op >= 32 {
			number, ok := parseCharStringNumber(&r)
			if !ok {
				return 0, errCFF
			}
			if err := d.stack.push(number); err != nil {
				return 0, err
			}
			if err := p.push(instruction{kind: instructionOperand, operand: number}); err != nil {
				return 0, err
			}
			continue
		}

		// No numbers can appear now, so now we can actually read it.
		op, ok = r.readU8()
		if !ok {
			return 0, errCFF
		}
		d.operators++
		if d.operators > maxDecompiledOperators {
			return 0, errCharStringTooLarge()
		}
		var operator operator
		if op == twoByteOperatorMark {
			b, ok := r.readU8()
			if !ok {
				return 0, errCFF
			}
			operator = operatorFromTwoByte(b)
		} else {
			operator = operatorFromOneByte(op)
		}

		switch operator {
		case opHFlex, opFlex, opHFlex1, opFlex1:
			d.stack.popAll()
			if err := p.push(instruction{kind: instructionOperator, operator: operator}); err != nil {
				return 0, err
			}
		case opHorizontalStem, opVerticalStem, opHorizontalStemHintMask, opVerticalStemHintMask:
			d.countHints()
			if err := p.push(instruction{kind: instructionOperator, operator: operator}); err != nil {
				return 0, err
			}
		case opVerticalMoveTo, opHorizontalMoveTo, opLineTo, opVerticalLineTo,
			opHorizontalLineTo, opMoveTo, opCurveLine, opLineCurve,
			opVVCurveTo, opVHCurveTo, opHHCurveTo, opHVCurveTo, opCurveTo:
			d.stack.popAll()
			if err := p.push(instruction{kind: instructionOperator, operator: operator}); err != nil {
				return 0, err
			}
		case opReturn:
			return charStringEndReturn, nil
		case opCallGlobalSubroutine, opCallLocalSubroutine:
			// Pop the subroutine index from the program.
			p.pop()

			n, ok := d.stack.pop()
			if !ok {
				return 0, errCFF
			}
			biasedIndex, ok := n.asI32()
			if !ok {
				return 0, errCFF
			}
			handler := d.gsubrHandler
			if operator == opCallLocalSubroutine {
				handler = d.lsubrHandler
			}
			subr, ok := handler.getWithBiased(biasedIndex)
			if !ok {
				return 0, errCFF
			}
			end, err := d.decompileInner(subr, p, depth+1)
			if err != nil {
				return 0, err
			}
			if end == charStringEndEndChar {
				return charStringEndEndChar, nil
			}
		case opHintMask, opCounterMask:
			if err := p.push(instruction{kind: instructionOperator, operator: operator}); err != nil {
				return 0, err
			}
			if d.hintMaskBytes == 0 {
				// Hintmask can contain implicit stems.
				d.countHints()
				d.hintMaskBytes = d.hintCount / 8
				if d.hintCount%8 != 0 {
					d.hintMaskBytes++
				}
			}

			hintBytes, ok := r.readBytes(int(d.hintMaskBytes))
			if !ok {
				return 0, errCFF
			}
			if err := p.push(instruction{kind: instructionHintMask, hintMask: hintBytes}); err != nil {
				return 0, err
			}
		case opEndChar:
			// We don't support seac for now. It's a deprecated feature and
			// Typst for some reason does not support it anyway.
			if d.stack.len() >= 4 {
				return 0, fmt.Errorf("%w: the CFF table has an endchar with the arguments of seac", errUnimplemented)
			}

			if err := p.push(instruction{kind: instructionOperator, operator: operator}); err != nil {
				return 0, err
			}

			return charStringEndEndChar, nil
		default:
			return 0, errCFF
		}
	}

	return charStringEndReturn, nil
}

// countHints is Decompiler::count_hints in subsetter src/cff/charstring.rs.
// Rust adds in a u16 that wraps in a release build, and so does the port.
func (d *decompiler) countHints() {
	elements := d.stack.popAll()
	d.hintCount += uint16(elements) / 2
}

// instructionKind is the case of the enum Instruction in subsetter
// src/cff/charstring.rs.
type instructionKind uint8

const (
	// instructionOperand is Instruction::Operand.
	instructionOperand instructionKind = iota
	// instructionOperator is Instruction::Operator.
	instructionOperator
	// instructionHintMask is Instruction::HintMask.
	instructionHintMask
)

// instruction is Instruction in subsetter src/cff/charstring.rs. A type of
// instruction in a charstring program. The field of its kind holds the
// value.
type instruction struct {
	kind     instructionKind
	operator operator
	operand  number
	hintMask []byte
}

// program is Program in subsetter src/cff/charstring.rs. A charstring
// program, decompiled into its constituent instructions.
type program struct {
	instructions []instruction
}

// String is the impl Debug of Program in subsetter src/cff/charstring.rs.
func (p program) String() string {
	var formattedStrings []string
	var strBuffer []string

	for _, instr := range p.instructions {
		switch instr.kind {
		case instructionOperand:
			strBuffer = append(strBuffer, formatF64(instr.operand.asF64()))
		case instructionOperator:
			strBuffer = append(strBuffer, "op("+instr.operator.String()+")")

			if instr.operator != opHintMask && instr.operator != opCounterMask {
				formattedStrings = append(formattedStrings, strings.Join(strBuffer, " "))
				strBuffer = strBuffer[:0]
			}
		case instructionHintMask:
			var byteString strings.Builder

			for _, b := range instr.hintMask {
				fmt.Fprintf(&byteString, "%08b", b)
			}

			strBuffer = append(strBuffer, byteString.String())
			formattedStrings = append(formattedStrings, strings.Join(strBuffer, " "))
			strBuffer = strBuffer[:0]
		}
	}

	return strings.Join(formattedStrings, "\n")
}

// push is Program::push in subsetter src/cff/charstring.rs. It pushes a new
// instruction to the program. It returns an error that wraps
// okage.ErrSubsetMalformedFont when the program holds maxProgramLen
// instructions, where upstream has no limit.
func (p *program) push(instruction instruction) error {
	if len(p.instructions) >= maxProgramLen {
		return errCharStringTooLarge()
	}
	p.instructions = append(p.instructions, instruction)
	return nil
}

// pop is program.0.pop() in subsetter src/cff/charstring.rs. It removes the
// last instruction, if there is one.
func (p *program) pop() {
	if len(p.instructions) > 0 {
		p.instructions = p.instructions[:len(p.instructions)-1]
	}
}

// compile is Program::compile in subsetter src/cff/charstring.rs. It
// compiles the program. A charstring holds no real number, so compile does
// not test the numbers with writable.
func (p program) compile() []byte {
	w := newWriter()

	for _, instr := range p.instructions {
		switch instr.kind {
		case instructionOperand:
			instr.operand.write(w)
		case instructionOperator:
			instr.operator.write(w)
		case instructionHintMask:
			w.extend(instr.hintMask)
		}
	}

	return w.finish()
}

// The operators of charstrings, the module operators in subsetter
// src/cff/charstring.rs. Upstream also declares SHORT_INT and FIXED_16_16,
// and uses neither, so the port leaves them out.
const (
	opHorizontalStem         operator = 1
	opVerticalStem           operator = 3
	opVerticalMoveTo         operator = 4
	opLineTo                 operator = 5
	opHorizontalLineTo       operator = 6
	opVerticalLineTo         operator = 7
	opCurveTo                operator = 8
	opCallLocalSubroutine    operator = 10
	opReturn                 operator = 11
	opEndChar                operator = 14
	opHorizontalStemHintMask operator = 18
	opHintMask               operator = 19
	opCounterMask            operator = 20
	opMoveTo                 operator = 21
	opHorizontalMoveTo       operator = 22
	opVerticalStemHintMask   operator = 23
	opCurveLine              operator = 24
	opLineCurve              operator = 25
	opVVCurveTo              operator = 26
	opHHCurveTo              operator = 27
	opCallGlobalSubroutine   operator = 29
	opVHCurveTo              operator = 30
	opHVCurveTo              operator = 31
	opHFlex                  operator = twoByteOperatorMark<<8 | 34
	opFlex                   operator = twoByteOperatorMark<<8 | 35
	opHFlex1                 operator = twoByteOperatorMark<<8 | 36
	opFlex1                  operator = twoByteOperatorMark<<8 | 37
)
