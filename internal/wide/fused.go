package wide

// This file holds the types of the fused programs of D98. A fused program
// is a copy of a program of stages of the raster pipeline that keeps its
// values in registers through the whole program, in place of the channels
// of the pipeline in memory. The tool fusegen in internal/fusegen writes
// the programs from its table into fused_programs.go, fused_scalar.go and
// fused_simd_amd64.go. This file is written by hand.

// FusedCtx holds the values that the fused programs read. The pipeline
// fills it from its context when it compiles a program that matches one of
// the fused programs, and sets Coverage before each run.
type FusedCtx struct {
	// RGBA is the color of the stage uniform_color of lowp. Each channel
	// must be from 0 to 255.
	RGBA [4]uint16
	// Coverage is from_float of the current coverage, for the stage
	// scale_1_float of lowp. It must be from 0 to 255.
	Coverage uint16
	// Transform holds sx, kx and tx of the stage transform: the values that
	// give the x of a pixel.
	Transform [3]float32
	// Factor and Bias hold the colors of the stage
	// evenly_spaced_2_stop_gradient, in the order r, g, b and a.
	Factor [4]float32
	Bias   [4]float32
}

// fusedUnit is the factor of load_8888 of highp: a channel of a pixel is
// float32(v) * fusedUnit.
const fusedUnit float32 = 1.0 / 255.0
