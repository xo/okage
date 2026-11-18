// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import "github.com/xo/okage/internal/pdfwriter"

// BlendMode is BlendMode in krilla src/graphics/blend.rs. The zero value is
// BlendModeNormal.
//
// How to blend source and backdrop.
type BlendMode uint8

// The values of BlendMode.
const (
	BlendModeNormal BlendMode = iota
	BlendModeMultiply
	BlendModeScreen
	BlendModeOverlay
	BlendModeDarken
	BlendModeLighten
	BlendModeColorDodge
	BlendModeColorBurn
	BlendModeHardLight
	BlendModeSoftLight
	BlendModeDifference
	BlendModeExclusion
	BlendModeHue
	BlendModeSaturation
	BlendModeColor
	BlendModeLuminosity
)

// toPDF is BlendMode::to_pdf in krilla src/graphics/blend.rs.
func (m BlendMode) toPDF() pdfwriter.BlendMode {
	switch m {
	case BlendModeNormal:
		return pdfwriter.BlendModeNormal
	case BlendModeMultiply:
		return pdfwriter.BlendModeMultiply
	case BlendModeScreen:
		return pdfwriter.BlendModeScreen
	case BlendModeOverlay:
		return pdfwriter.BlendModeOverlay
	case BlendModeDarken:
		return pdfwriter.BlendModeDarken
	case BlendModeLighten:
		return pdfwriter.BlendModeLighten
	case BlendModeColorDodge:
		return pdfwriter.BlendModeColorDodge
	case BlendModeColorBurn:
		return pdfwriter.BlendModeColorBurn
	case BlendModeHardLight:
		return pdfwriter.BlendModeHardLight
	case BlendModeSoftLight:
		return pdfwriter.BlendModeSoftLight
	case BlendModeDifference:
		return pdfwriter.BlendModeDifference
	case BlendModeExclusion:
		return pdfwriter.BlendModeExclusion
	case BlendModeHue:
		return pdfwriter.BlendModeHue
	case BlendModeSaturation:
		return pdfwriter.BlendModeSaturation
	case BlendModeColor:
		return pdfwriter.BlendModeColor
	default:
		return pdfwriter.BlendModeLuminosity
	}
}
