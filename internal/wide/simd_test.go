//go:build goexperiment.simd && !okage_scalar && !tinygo && (amd64 || arm64 || wasm)

package wide

import (
	"fmt"
	"os"
	"testing"
)

// TestMain runs each test twice in the SIMD build: first with the SIMD code,
// and then with useSIMD set to false, so that the same tables test the
// scalar code of the SIMD build. If the CPU has no SIMD code, the first run
// is already the scalar code, and the tests run once.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && useSIMD {
		restore := setUseSIMD(false)
		if testing.Verbose() {
			fmt.Println("wide: the second run tests the scalar code of the SIMD build")
		}
		code = m.Run()
		restore()
	}
	os.Exit(code)
}

// setUseSIMD sets useSIMD to on, and gives a function that sets it back. It
// is the hook that lets a test run the scalar code in the SIMD build.
func setUseSIMD(on bool) (restore func()) {
	old := useSIMD
	useSIMD = on
	return func() { useSIMD = old }
}

// implName names the code that the methods run, for the names of the
// benchmarks: "simd" for the SIMD code, and "fallback" for the scalar code
// in the SIMD build.
func implName() string {
	if useSIMD {
		return "simd"
	}
	return "fallback"
}
