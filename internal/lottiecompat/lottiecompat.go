// Package lottiecompat holds the switch that makes the package lottie draw
// as velato does, without the differences of D42 and D43. Only the test of
// lottie against velato in the test module sets it, so that it compares the
// port itself on every file (D41). A program never sets it.
package lottiecompat

// Velato is true while the test against velato runs. lottie reads it when it
// parses a file and when it renders a frame.
var Velato bool
