package lottiecompat

import "testing"

// TestVelatoIsOff checks that the switch is off unless a test sets it. The
// file also gives the package a test program, which the build of each test
// program for wasip1 with TinyGo needs.
func TestVelatoIsOff(t *testing.T) {
	if Velato {
		t.Error("expected Velato to be false")
	}
}
