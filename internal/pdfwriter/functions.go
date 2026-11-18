// Copyright 2020 The pdf-writer Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdfwriter

import "fmt"

// This file ports pdf-writer src/functions.rs, with the methods of Chunk for
// functions of src/chunk.rs. The macro common_func_methods! becomes the
// methods Domain and Range of each writer.

// functionType is FunctionType in pdf-writer src/functions.rs.
//
// Way the function is defined in.
type functionType uint8

// The values of functionType.
const (
	// functionTypeSampled is a function that is derived from a set of
	// sampled data.
	functionTypeSampled functionType = iota
	// functionTypeExponential is a exponential function.
	functionTypeExponential
	// functionTypeStitching is a composite function made up of multiple
	// other functions.
	functionTypeStitching
	// functionTypePostScript is a postscript function.
	functionTypePostScript
)

// toInt is FunctionType::to_int in pdf-writer src/functions.rs.
func (t functionType) toInt() int32 {
	switch t {
	case functionTypeSampled:
		return 0
	case functionTypeExponential:
		return 2
	case functionTypeStitching:
		return 3
	case functionTypePostScript:
		return 4
	}
	panic(fmt.Sprintf("pdfwriter: the function type %d is not known", uint8(t)))
}

// SampledFunction is SampledFunction in pdf-writer src/functions.rs.
//
// Writer for a sampled function stream. PDF 1.2+.
//
// This struct is created by Chunk.SampledFunction.
type SampledFunction struct {
	*Stream
}

// startSampledFunction is SampledFunction::start in pdf-writer
// src/functions.rs.
//
// Create a new sampled function writer.
func startSampledFunction(stream *Stream) *SampledFunction {
	stream.Pair(Name("FunctionType"), Int(functionTypeSampled.toInt()))
	return &SampledFunction{Stream: stream}
}

// Domain is the method domain of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Domain attribute to set where the function is defined.
// Required.
func (f *SampledFunction) Domain(domain ...float32) *SampledFunction {
	itemsF32(f.Insert(Name("Domain")).Array(), domain)
	return f
}

// Range is the method range of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Range attribute.
//
// Required for sampled and PostScript functions.
func (f *SampledFunction) Range(rng ...float32) *SampledFunction {
	itemsF32(f.Insert(Name("Range")).Array(), rng)
	return f
}

// Size is SampledFunction::size in pdf-writer src/functions.rs.
//
// Write the /Size attribute.
//
// Sets the number of input samples per dimension. Required.
func (f *SampledFunction) Size(size ...int32) *SampledFunction {
	a := f.Insert(Name("Size")).Array()
	for _, v := range size {
		a.Item(Int(v))
	}
	return f
}

// BitsPerSample is SampledFunction::bits_per_sample in pdf-writer
// src/functions.rs.
//
// Write the /BitsPerSample attribute.
//
// Sets the number of bits per input sample. Required.
func (f *SampledFunction) BitsPerSample(bits int32) *SampledFunction {
	f.Pair(Name("BitsPerSample"), Int(bits))
	return f
}

// Order is SampledFunction::order in pdf-writer src/functions.rs.
//
// Write the /Order attribute.
//
// Choose the implementation kind.
func (f *SampledFunction) Order(order InterpolationOrder) *SampledFunction {
	f.Pair(Name("Order"), Int(order.toInt()))
	return f
}

// Encode is SampledFunction::encode in pdf-writer src/functions.rs.
//
// Write the /Encode attribute.
//
// For each sample, define how the input is mapped to the domain range.
func (f *SampledFunction) Encode(encode ...float32) *SampledFunction {
	itemsF32(f.Insert(Name("Encode")).Array(), encode)
	return f
}

// Decode is SampledFunction::decode in pdf-writer src/functions.rs.
//
// Write the /Decode attribute.
//
// For each sample, define how the output is mapped to the output range.
func (f *SampledFunction) Decode(decode ...float32) *SampledFunction {
	itemsF32(f.Insert(Name("Decode")).Array(), decode)
	return f
}

// InterpolationOrder is InterpolationOrder in pdf-writer src/functions.rs.
// Its zero value is the default of upstream, InterpolationOrderLinear.
//
// How to interpolate between the samples in a function of the sampled type.
type InterpolationOrder uint8

// The values of InterpolationOrder.
const (
	// InterpolationOrderLinear is linear spline interpolation.
	InterpolationOrderLinear InterpolationOrder = iota
	// InterpolationOrderCubic is cubic spline interpolation.
	InterpolationOrderCubic
)

// interpolationOrderNames holds the Rust name of each value of
// InterpolationOrder.
var interpolationOrderNames = [...]string{
	InterpolationOrderLinear: "Linear",
	InterpolationOrderCubic:  "Cubic",
}

// String returns the Rust name of the interpolation order.
func (o InterpolationOrder) String() string {
	if int(o) < len(interpolationOrderNames) {
		return interpolationOrderNames[o]
	}
	return "InterpolationOrder(?)"
}

// toInt is InterpolationOrder::to_int in pdf-writer src/functions.rs.
func (o InterpolationOrder) toInt() int32 {
	switch o {
	case InterpolationOrderLinear:
		return 1
	case InterpolationOrderCubic:
		return 3
	}
	panic(fmt.Sprintf("pdfwriter: the interpolation order %d is not known", uint8(o)))
}

// ExponentialFunction is ExponentialFunction in pdf-writer
// src/functions.rs.
//
// Writer for an exponential function dictionary. PDF 1.3+.
//
// The function result is y_i = C0_i + x^N * (C1_i - C0_i) where i is the
// current dimension.
//
// This struct is created by Chunk.ExponentialFunction and
// Separation.TintExponential.
type ExponentialFunction struct {
	*Dict
}

// start is the macro writer! for ExponentialFunction in pdf-writer
// src/functions.rs.
func (f *ExponentialFunction) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("FunctionType"), Int(functionTypeExponential.toInt()))
}

// Domain is the method domain of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Domain attribute to set where the function is defined.
// Required.
func (f *ExponentialFunction) Domain(domain ...float32) *ExponentialFunction {
	itemsF32(f.Insert(Name("Domain")).Array(), domain)
	return f
}

// Range is the method range of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Range attribute.
//
// Required for sampled and PostScript functions.
func (f *ExponentialFunction) Range(rng ...float32) *ExponentialFunction {
	itemsF32(f.Insert(Name("Range")).Array(), rng)
	return f
}

// C0 is ExponentialFunction::c0 in pdf-writer src/functions.rs.
//
// Write the /C0 array.
//
// Function result when input is zero. Default is 0.0.
func (f *ExponentialFunction) C0(c0 ...float32) *ExponentialFunction {
	itemsF32(f.Insert(Name("C0")).Array(), c0)
	return f
}

// C1 is ExponentialFunction::c1 in pdf-writer src/functions.rs.
//
// Write the /C1 array.
//
// Function result when input is one. Default is 1.0.
func (f *ExponentialFunction) C1(c1 ...float32) *ExponentialFunction {
	itemsF32(f.Insert(Name("C1")).Array(), c1)
	return f
}

// N is ExponentialFunction::n in pdf-writer src/functions.rs.
//
// Write the /N attribute.
//
// The interpolation exponent. Required.
func (f *ExponentialFunction) N(n float32) *ExponentialFunction {
	f.Pair(Name("N"), Real(n))
	return f
}

// StitchingFunction is StitchingFunction in pdf-writer src/functions.rs.
//
// Writer for a stitching function dictionary. PDF 1.3+.
//
// The function result is y_i = C0_i + x^N * (C1_i - C0_i) where i is the
// current dimension.
//
// This struct is created by Chunk.StitchingFunction and
// Separation.TintStitching.
type StitchingFunction struct {
	*Dict
}

// start is the macro writer! for StitchingFunction in pdf-writer
// src/functions.rs.
func (f *StitchingFunction) start(obj Obj) {
	f.Dict = obj.Dict()
	f.Pair(Name("FunctionType"), Int(functionTypeStitching.toInt()))
}

// Domain is the method domain of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Domain attribute to set where the function is defined.
// Required.
func (f *StitchingFunction) Domain(domain ...float32) *StitchingFunction {
	itemsF32(f.Insert(Name("Domain")).Array(), domain)
	return f
}

// Range is the method range of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Range attribute.
//
// Required for sampled and PostScript functions.
func (f *StitchingFunction) Range(rng ...float32) *StitchingFunction {
	itemsF32(f.Insert(Name("Range")).Array(), rng)
	return f
}

// Functions is StitchingFunction::functions in pdf-writer src/functions.rs.
//
// Write the /Functions array.
//
// The functions to be stitched. Required.
func (f *StitchingFunction) Functions(functions ...Ref) *StitchingFunction {
	NewTypedArray[Ref](f.Insert(Name("Functions")).Array()).Items(functions...)
	return f
}

// Bounds is StitchingFunction::bounds in pdf-writer src/functions.rs.
//
// Write the /Bounds array.
//
// The boundaries of the intervals that each function is called in. The
// array has one less entry than there are stiched functions. Required.
func (f *StitchingFunction) Bounds(bounds ...float32) *StitchingFunction {
	itemsF32(f.Insert(Name("Bounds")).Array(), bounds)
	return f
}

// Encode is StitchingFunction::encode in pdf-writer src/functions.rs.
//
// Write the /Encode array.
//
// Pair of values for each function that maps the stitching domain subsets
// to the function domain. Required.
func (f *StitchingFunction) Encode(encode ...float32) *StitchingFunction {
	itemsF32(f.Insert(Name("Encode")).Array(), encode)
	return f
}

// PostScriptFunction is PostScriptFunction in pdf-writer src/functions.rs.
//
// Writer for a PostScript function stream. PDF 1.3+.
//
// This struct is created by Chunk.PostScriptFunction.
type PostScriptFunction struct {
	*Stream
}

// startPostScriptFunction is PostScriptFunction::start in pdf-writer
// src/functions.rs.
//
// Create a new postscript function writer.
func startPostScriptFunction(stream *Stream) *PostScriptFunction {
	stream.Pair(Name("FunctionType"), Int(functionTypePostScript.toInt()))
	return &PostScriptFunction{Stream: stream}
}

// Domain is the method domain of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Domain attribute to set where the function is defined.
// Required.
func (f *PostScriptFunction) Domain(domain ...float32) *PostScriptFunction {
	itemsF32(f.Insert(Name("Domain")).Array(), domain)
	return f
}

// Range is the method range of the macro common_func_methods! in
// pdf-writer src/functions.rs.
//
// Write the /Range attribute.
//
// Required for sampled and PostScript functions.
func (f *PostScriptFunction) Range(rng ...float32) *PostScriptFunction {
	itemsF32(f.Insert(Name("Range")).Array(), rng)
	return f
}

// PostScriptOp is PostScriptOp in pdf-writer src/functions.rs. It is an enum
// whose cases hold a number, one or two lists of operators, or nothing. The
// cases with data are the functions PostScriptOpReal, PostScriptOpInteger,
// PostScriptOpIf and PostScriptOpIfElse, and the other cases are the variables
// PostScriptOpAbs to PostScriptOpRoll (D27).
//
// PostScript operators for use in Type 4 functions.
type PostScriptOp struct {
	kind    postScriptOpKind
	real    float32
	integer int32
	ops1    []PostScriptOp
	ops2    []PostScriptOp
}

// postScriptOpKind is the case of a PostScriptOp.
type postScriptOpKind uint8

// The cases of PostScriptOp.
const (
	postScriptOpReal postScriptOpKind = iota
	postScriptOpInteger
	postScriptOpAbs
	postScriptOpAdd
	postScriptOpAtan
	postScriptOpCeiling
	postScriptOpCos
	postScriptOpCvi
	postScriptOpCvr
	postScriptOpDiv
	postScriptOpExp
	postScriptOpFloor
	postScriptOpIdiv
	postScriptOpLn
	postScriptOpLog
	postScriptOpMod
	postScriptOpMul
	postScriptOpNeg
	postScriptOpRound
	postScriptOpSin
	postScriptOpSqrt
	postScriptOpSub
	postScriptOpTruncate
	postScriptOpAnd
	postScriptOpBitshift
	postScriptOpEq
	postScriptOpFalse
	postScriptOpGe
	postScriptOpGt
	postScriptOpLe
	postScriptOpLt
	postScriptOpNe
	postScriptOpNot
	postScriptOpOr
	postScriptOpTrue
	postScriptOpXor
	postScriptOpIf
	postScriptOpIfElse
	postScriptOpCopy
	postScriptOpDup
	postScriptOpExch
	postScriptOpIndex
	postScriptOpPop
	postScriptOpRoll
)

// PostScriptOpReal is PostScriptOp::Real in pdf-writer src/functions.rs.
//
// Push a real number.
func PostScriptOpReal(r float32) PostScriptOp {
	return PostScriptOp{kind: postScriptOpReal, real: r}
}

// PostScriptOpInteger is PostScriptOp::Integer in pdf-writer
// src/functions.rs.
//
// Push an integer number.
func PostScriptOpInteger(i int32) PostScriptOp {
	return PostScriptOp{kind: postScriptOpInteger, integer: i}
}

// PostScriptOpIf is PostScriptOp::If in pdf-writer src/functions.rs.
//
// Conditional. Runs if boolean argument is true.
func PostScriptOpIf(ops []PostScriptOp) PostScriptOp {
	return PostScriptOp{kind: postScriptOpIf, ops1: ops}
}

// PostScriptOpIfElse is PostScriptOp::IfElse in pdf-writer src/functions.rs.
//
// Conditional. Decides which branch to run depending on boolean argument.
func PostScriptOpIfElse(ops1, ops2 []PostScriptOp) PostScriptOp {
	return PostScriptOp{kind: postScriptOpIfElse, ops1: ops1, ops2: ops2}
}

// The cases of PostScriptOp without data.
var (
	// PostScriptOpAbs is absolute value. One number argument.
	PostScriptOpAbs = PostScriptOp{kind: postScriptOpAbs}
	// PostScriptOpAdd is addition. Two number arguments.
	PostScriptOpAdd = PostScriptOp{kind: postScriptOpAdd}
	// PostScriptOpAtan is arc tangent. One number argument.
	PostScriptOpAtan = PostScriptOp{kind: postScriptOpAtan}
	// PostScriptOpCeiling rounds up to the nearest integer. One number
	// argument.
	PostScriptOpCeiling = PostScriptOp{kind: postScriptOpCeiling}
	// PostScriptOpCos is cosine. One number argument.
	PostScriptOpCos = PostScriptOp{kind: postScriptOpCos}
	// PostScriptOpCvi converts to integer. One real number argument.
	PostScriptOpCvi = PostScriptOp{kind: postScriptOpCvi}
	// PostScriptOpCvr converts to real. One integer argument.
	PostScriptOpCvr = PostScriptOp{kind: postScriptOpCvr}
	// PostScriptOpDiv divides. Two number arguments.
	PostScriptOpDiv = PostScriptOp{kind: postScriptOpDiv}
	// PostScriptOpExp raises the base to the exponent. Two number arguments.
	PostScriptOpExp = PostScriptOp{kind: postScriptOpExp}
	// PostScriptOpFloor rounds down to the nearest integer. One number
	// argument.
	PostScriptOpFloor = PostScriptOp{kind: postScriptOpFloor}
	// PostScriptOpIdiv is integer division. Two integer arguments.
	PostScriptOpIdiv = PostScriptOp{kind: postScriptOpIdiv}
	// PostScriptOpLn is the natural logarithm. One number argument.
	PostScriptOpLn = PostScriptOp{kind: postScriptOpLn}
	// PostScriptOpLog is the logarithm base 10. One number argument.
	PostScriptOpLog = PostScriptOp{kind: postScriptOpLog}
	// PostScriptOpMod is modulo. Two integer arguments.
	PostScriptOpMod = PostScriptOp{kind: postScriptOpMod}
	// PostScriptOpMul multiplies. Two number arguments.
	PostScriptOpMul = PostScriptOp{kind: postScriptOpMul}
	// PostScriptOpNeg negates. One number argument.
	PostScriptOpNeg = PostScriptOp{kind: postScriptOpNeg}
	// PostScriptOpRound rounds to the nearest integer. One number argument.
	PostScriptOpRound = PostScriptOp{kind: postScriptOpRound}
	// PostScriptOpSin is sine. One number argument.
	PostScriptOpSin = PostScriptOp{kind: postScriptOpSin}
	// PostScriptOpSqrt is the square root. One number argument.
	PostScriptOpSqrt = PostScriptOp{kind: postScriptOpSqrt}
	// PostScriptOpSub subtracts. Two number arguments.
	PostScriptOpSub = PostScriptOp{kind: postScriptOpSub}
	// PostScriptOpTruncate removes the fractional part. One number argument.
	PostScriptOpTruncate = PostScriptOp{kind: postScriptOpTruncate}

	// PostScriptOpAnd is logical bitwise and. Two integer or boolean
	// arguments.
	PostScriptOpAnd = PostScriptOp{kind: postScriptOpAnd}
	// PostScriptOpBitshift is a bitwise shift left. Negative shifts possible.
	// Two integer arguments.
	PostScriptOpBitshift = PostScriptOp{kind: postScriptOpBitshift}
	// PostScriptOpEq is equals. Any two arguments of the same type.
	PostScriptOpEq = PostScriptOp{kind: postScriptOpEq}
	// PostScriptOpFalse is the constant false.
	PostScriptOpFalse = PostScriptOp{kind: postScriptOpFalse}
	// PostScriptOpGe is greater than or equal. Two number arguments.
	PostScriptOpGe = PostScriptOp{kind: postScriptOpGe}
	// PostScriptOpGt is greater than. Two number arguments.
	PostScriptOpGt = PostScriptOp{kind: postScriptOpGt}
	// PostScriptOpLe is less than or equal. Two number arguments.
	PostScriptOpLe = PostScriptOp{kind: postScriptOpLe}
	// PostScriptOpLt is less than. Two number arguments.
	PostScriptOpLt = PostScriptOp{kind: postScriptOpLt}
	// PostScriptOpNe is not equals. Any two arguments of the same type.
	PostScriptOpNe = PostScriptOp{kind: postScriptOpNe}
	// PostScriptOpNot is bitwise logical not. One integer or boolean
	// argument.
	PostScriptOpNot = PostScriptOp{kind: postScriptOpNot}
	// PostScriptOpOr is bitwise logical or. Two integer or boolean arguments.
	PostScriptOpOr = PostScriptOp{kind: postScriptOpOr}
	// PostScriptOpTrue is the constant true.
	PostScriptOpTrue = PostScriptOp{kind: postScriptOpTrue}
	// PostScriptOpXor is bitwise logical exclusive or. Two integer or boolean
	// arguments.
	PostScriptOpXor = PostScriptOp{kind: postScriptOpXor}

	// PostScriptOpCopy copies the top elements. One integer argument.
	PostScriptOpCopy = PostScriptOp{kind: postScriptOpCopy}
	// PostScriptOpDup duplicates the top element.
	PostScriptOpDup = PostScriptOp{kind: postScriptOpDup}
	// PostScriptOpExch exchanges the two top elements.
	PostScriptOpExch = PostScriptOp{kind: postScriptOpExch}
	// PostScriptOpIndex duplicates any element. One integer argument.
	PostScriptOpIndex = PostScriptOp{kind: postScriptOpIndex}
	// PostScriptOpPop discards the top element.
	PostScriptOpPop = PostScriptOp{kind: postScriptOpPop}
	// PostScriptOpRoll rolls n elements up j times. Two integer arguments.
	PostScriptOpRoll = PostScriptOp{kind: postScriptOpRoll}
)

// PostScriptOpEncode is PostScriptOp::encode in pdf-writer
// src/functions.rs.
//
// Encode a slice of operations into a byte stream.
func PostScriptOpEncode(ops []PostScriptOp) *Buf {
	buf := newBuf()
	writePostScriptSlice(ops, &buf)
	return &buf
}

// writePostScriptSlice is PostScriptOp::write_slice in pdf-writer
// src/functions.rs.
func writePostScriptSlice(ops []PostScriptOp, buf *Buf) {
	buf.push('{')
	if len(ops) > 1 {
		buf.push(' ')
	}
	for _, op := range ops {
		op.write(buf)
		buf.push(' ')
	}
	if len(ops) == 1 {
		buf.inner = buf.inner[:len(buf.inner)-1]
	}
	buf.push('}')
}

// write is PostScriptOp::write in pdf-writer src/functions.rs.
func (op PostScriptOp) write(buf *Buf) {
	switch op.kind {
	case postScriptOpReal:
		buf.pushDecimal(op.real)
	case postScriptOpInteger:
		buf.pushVal(Int(op.integer))
	case postScriptOpIf:
		writePostScriptSlice(op.ops1, buf)
		buf.push(' ')
		buf.extendString(op.operator())
	case postScriptOpIfElse:
		writePostScriptSlice(op.ops1, buf)
		buf.push(' ')
		writePostScriptSlice(op.ops2, buf)
		buf.push(' ')
		buf.extendString(op.operator())
	default:
		buf.extendString(op.operator())
	}
}

// postScriptOperators holds the operator of each case of PostScriptOp.
var postScriptOperators = [...]string{
	postScriptOpReal:     "",
	postScriptOpInteger:  "",
	postScriptOpAbs:      "abs",
	postScriptOpAdd:      "add",
	postScriptOpAtan:     "atan",
	postScriptOpCeiling:  "ceiling",
	postScriptOpCos:      "cos",
	postScriptOpCvi:      "cvi",
	postScriptOpCvr:      "cvr",
	postScriptOpDiv:      "div",
	postScriptOpExp:      "exp",
	postScriptOpFloor:    "floor",
	postScriptOpIdiv:     "idiv",
	postScriptOpLn:       "ln",
	postScriptOpLog:      "log",
	postScriptOpMod:      "mod",
	postScriptOpMul:      "mul",
	postScriptOpNeg:      "neg",
	postScriptOpRound:    "round",
	postScriptOpSin:      "sin",
	postScriptOpSqrt:     "sqrt",
	postScriptOpSub:      "sub",
	postScriptOpTruncate: "truncate",
	postScriptOpAnd:      "and",
	postScriptOpBitshift: "bitshift",
	postScriptOpEq:       "eq",
	postScriptOpFalse:    "false",
	postScriptOpGe:       "ge",
	postScriptOpGt:       "gt",
	postScriptOpLe:       "le",
	postScriptOpLt:       "lt",
	postScriptOpNe:       "ne",
	postScriptOpNot:      "not",
	postScriptOpOr:       "or",
	postScriptOpTrue:     "true",
	postScriptOpXor:      "xor",
	postScriptOpIf:       "if",
	postScriptOpIfElse:   "ifelse",
	postScriptOpCopy:     "copy",
	postScriptOpDup:      "dup",
	postScriptOpExch:     "exch",
	postScriptOpIndex:    "index",
	postScriptOpPop:      "pop",
	postScriptOpRoll:     "roll",
}

// operator is PostScriptOp::operator in pdf-writer src/functions.rs.
func (op PostScriptOp) operator() string {
	if int(op.kind) < len(postScriptOperators) {
		return postScriptOperators[op.kind]
	}
	panic(fmt.Sprintf("pdfwriter: the PostScript operator %d is not known", uint8(op.kind)))
}

// Functions.

// SampledFunction is Chunk::sampled_function in pdf-writer src/chunk.rs.
//
// Start writing a sampled function stream.
func (c *Chunk) SampledFunction(id Ref, samples []byte) *SampledFunction {
	return startSampledFunction(c.Stream(id, samples))
}

// ExponentialFunction is Chunk::exponential_function in pdf-writer
// src/chunk.rs.
//
// Start writing an exponential function.
func (c *Chunk) ExponentialFunction(id Ref) *ExponentialFunction {
	return Start[ExponentialFunction](c.Indirect(id))
}

// StitchingFunction is Chunk::stitching_function in pdf-writer
// src/chunk.rs.
//
// Start writing a stitching function.
func (c *Chunk) StitchingFunction(id Ref) *StitchingFunction {
	return Start[StitchingFunction](c.Indirect(id))
}

// PostScriptFunction is Chunk::post_script_function in pdf-writer
// src/chunk.rs.
//
// Start writing a PostScript function stream.
//
// You can create the code bytes using PostScriptOpEncode.
func (c *Chunk) PostScriptFunction(id Ref, code []byte) *PostScriptFunction {
	return startPostScriptFunction(c.Stream(id, code))
}
