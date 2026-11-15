package brotli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/xo/okage"
)

// TestDictionary checks the dictionary against the SHA-256 sum of the
// dictionary of RFC 7932.
func TestDictionary(t *testing.T) {
	const exp = "20e42eb1b511c21806d4d227d07e5dd06877d8ce7b3a817f378f313653f35c70"
	sum := sha256.Sum256(dictLUT)
	if got := hex.EncodeToString(sum[:]); got != exp {
		t.Errorf("expected %s, got %s", exp, got)
	}
}

// TestReader decodes each .br file of testdata, from the tests of
// github.com/dsnet/compress, and compares it with the file without .br.
func TestReader(t *testing.T) {
	names, err := filepath.Glob(filepath.Join("testdata", "*.br"))
	if err != nil || len(names) == 0 {
		t.Fatalf("no test data: %v", err)
	}
	for _, name := range names {
		t.Run(filepath.Base(name), func(t *testing.T) {
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			exp, err := os.ReadFile(name[:len(name)-len(".br")])
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(NewReader(bytes.NewReader(src)))
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !bytes.Equal(got, exp) {
				t.Errorf("expected %d bytes, got %d bytes that differ", len(exp), len(got))
			}
		})
	}
}

// TestReaderCorrupted checks that a corrupted stream gives an error that
// wraps okage.ErrFontData, and that a stream cut short gives
// io.ErrUnexpectedEOF.
func TestReaderCorrupted(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "monkey.br"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(NewReader(bytes.NewReader(src[:len(src)/2]))); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("cut short: expected io.ErrUnexpectedEOF, got %v", err)
	}
	// The window bits 1000100 are reserved (RFC 7932 section 9.1).
	if _, err := io.ReadAll(NewReader(bytes.NewReader([]byte{0x11, 0xff}))); !errors.Is(err, okage.ErrFontData) {
		t.Errorf("reserved window: expected okage.ErrFontData, got %v", err)
	}
}
