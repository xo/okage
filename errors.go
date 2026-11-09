package okage

// This file ports no upstream file. It holds the sentinel errors of okage
// (D21).

// Error is an error of okage. Each sentinel error of okage is a constant of
// this type, so that a caller can compare it with errors.Is.
type Error string

// Error satisfies the error interface.
func (err Error) Error() string {
	return string(err)
}

// The sentinel errors of okage.
const (
	// ErrDecodingPNG is the error of DecodePNG, LoadPNG, DecodeMaskPNG and
	// LoadMaskPNG when the package image/png, or the file system, returns an
	// error. okage wraps that error with this one. Upstream returns the error
	// of the crate png.
	ErrDecodingPNG Error = "decoding png"

	// ErrEncodingPNG is the error of EncodePNG and SavePNG of a Pixmap and of
	// a Mask when the package image/png, or the file system, returns an
	// error. okage wraps that error with this one. Upstream returns the error
	// of the crate png.
	ErrEncodingPNG Error = "encoding png"

	// ErrUnsupportedPNG is the error of DecodePNG and DecodeMaskPNG for a PNG
	// image that tiny-skia does not support. Upstream gives "unsupported bit
	// depth", "indexed PNG is not supported" and "only grayscale masks are
	// supported".
	ErrUnsupportedPNG Error = "decoding png: unsupported image"

	// ErrInvalidImageSize is the error of DecodePNG and DecodeMaskPNG for an
	// image with a zero width or height. Upstream gives "invalid image size".
	ErrInvalidImageSize Error = "decoding png: invalid image size"

	// ErrImageTooBig is the error of DecodePNG for an image that is larger
	// than a Pixmap can hold. Upstream gives "image is too big".
	ErrImageTooBig Error = "decoding png: image is too big"

	// ErrCreatingPixmap is the error of DecodePNG when the decoded pixels do
	// not make a Pixmap. Upstream gives "failed to create a pixmap".
	ErrCreatingPixmap Error = "decoding png: creating a pixmap"

	// ErrCreatingMask is the error of DecodeMaskPNG when the decoded values do
	// not make a Mask. Upstream gives "failed to create a mask".
	ErrCreatingMask Error = "decoding png: creating a mask"
)
