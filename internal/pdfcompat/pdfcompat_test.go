package pdfcompat

import "testing"

// TestKrillaIsOff checks that the switch is off unless a test sets it. The
// file also gives the package a test program, which the build of each test
// program for wasip1 with TinyGo needs.
func TestKrillaIsOff(t *testing.T) {
	if Krilla {
		t.Error("expected Krilla to be false")
	}
}
