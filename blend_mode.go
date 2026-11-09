package okage

// BlendMode is BlendMode in tiny-skia src/blend_mode.rs. It is a blending
// mode.
type BlendMode uint8

// The values of BlendMode, in the order of the Rust enum.
const (
	// BlendModeClear is BlendMode::Clear. It replaces destination with zero:
	// fully transparent.
	BlendModeClear BlendMode = iota
	// BlendModeSource is BlendMode::Source. It replaces destination.
	BlendModeSource
	// BlendModeDestination is BlendMode::Destination. It preserves
	// destination.
	BlendModeDestination
	// BlendModeSourceOver is BlendMode::SourceOver, the default. It draws
	// source over destination.
	BlendModeSourceOver
	// BlendModeDestinationOver is BlendMode::DestinationOver. It draws
	// destination over source.
	BlendModeDestinationOver
	// BlendModeSourceIn is BlendMode::SourceIn. It keeps the source inside
	// destination.
	BlendModeSourceIn
	// BlendModeDestinationIn is BlendMode::DestinationIn. It keeps the
	// destination inside source.
	BlendModeDestinationIn
	// BlendModeSourceOut is BlendMode::SourceOut. It keeps the source
	// outside destination.
	BlendModeSourceOut
	// BlendModeDestinationOut is BlendMode::DestinationOut. It keeps the
	// destination outside source.
	BlendModeDestinationOut
	// BlendModeSourceAtop is BlendMode::SourceAtop. It blends the source
	// inside destination with destination.
	BlendModeSourceAtop
	// BlendModeDestinationAtop is BlendMode::DestinationAtop. It blends the
	// destination inside source with source.
	BlendModeDestinationAtop
	// BlendModeXor is BlendMode::Xor. It keeps each of source and
	// destination outside the other.
	BlendModeXor
	// BlendModePlus is BlendMode::Plus. It is the sum of colors.
	BlendModePlus
	// BlendModeModulate is BlendMode::Modulate. It is the product of
	// premultiplied colors. It darkens destination.
	BlendModeModulate
	// BlendModeScreen is BlendMode::Screen. It multiplies the inverse of the
	// pixels and inverts the result. It brightens destination.
	BlendModeScreen
	// BlendModeOverlay is BlendMode::Overlay. It multiplies or screens,
	// depending on destination.
	BlendModeOverlay
	// BlendModeDarken is BlendMode::Darken. It keeps the darker of source
	// and destination.
	BlendModeDarken
	// BlendModeLighten is BlendMode::Lighten. It keeps the lighter of source
	// and destination.
	BlendModeLighten
	// BlendModeColorDodge is BlendMode::ColorDodge. It brightens destination
	// to reflect source.
	BlendModeColorDodge
	// BlendModeColorBurn is BlendMode::ColorBurn. It darkens destination to
	// reflect source.
	BlendModeColorBurn
	// BlendModeHardLight is BlendMode::HardLight. It multiplies or screens,
	// depending on source.
	BlendModeHardLight
	// BlendModeSoftLight is BlendMode::SoftLight. It lightens or darkens,
	// depending on source.
	BlendModeSoftLight
	// BlendModeDifference is BlendMode::Difference. It subtracts the darker
	// from the lighter with higher contrast.
	BlendModeDifference
	// BlendModeExclusion is BlendMode::Exclusion. It subtracts the darker
	// from the lighter with lower contrast.
	BlendModeExclusion
	// BlendModeMultiply is BlendMode::Multiply. It multiplies source with
	// destination, which darkens the image.
	BlendModeMultiply
	// BlendModeHue is BlendMode::Hue. It takes the hue of source with the
	// saturation and luminosity of destination.
	BlendModeHue
	// BlendModeSaturation is BlendMode::Saturation. It takes the saturation
	// of source with the hue and luminosity of destination.
	BlendModeSaturation
	// BlendModeColor is BlendMode::Color. It takes the hue and saturation of
	// source with the luminosity of destination.
	BlendModeColor
	// BlendModeLuminosity is BlendMode::Luminosity. It takes the luminosity
	// of source with the hue and saturation of destination.
	BlendModeLuminosity
)

// String satisfies the fmt.Stringer interface. It returns the name of the
// Rust variant.
func (m BlendMode) String() string {
	switch m {
	case BlendModeClear:
		return "Clear"
	case BlendModeSource:
		return "Source"
	case BlendModeDestination:
		return "Destination"
	case BlendModeSourceOver:
		return "SourceOver"
	case BlendModeDestinationOver:
		return "DestinationOver"
	case BlendModeSourceIn:
		return "SourceIn"
	case BlendModeDestinationIn:
		return "DestinationIn"
	case BlendModeSourceOut:
		return "SourceOut"
	case BlendModeDestinationOut:
		return "DestinationOut"
	case BlendModeSourceAtop:
		return "SourceAtop"
	case BlendModeDestinationAtop:
		return "DestinationAtop"
	case BlendModeXor:
		return "Xor"
	case BlendModePlus:
		return "Plus"
	case BlendModeModulate:
		return "Modulate"
	case BlendModeScreen:
		return "Screen"
	case BlendModeOverlay:
		return "Overlay"
	case BlendModeDarken:
		return "Darken"
	case BlendModeLighten:
		return "Lighten"
	case BlendModeColorDodge:
		return "ColorDodge"
	case BlendModeColorBurn:
		return "ColorBurn"
	case BlendModeHardLight:
		return "HardLight"
	case BlendModeSoftLight:
		return "SoftLight"
	case BlendModeDifference:
		return "Difference"
	case BlendModeExclusion:
		return "Exclusion"
	case BlendModeMultiply:
		return "Multiply"
	case BlendModeHue:
		return "Hue"
	case BlendModeSaturation:
		return "Saturation"
	case BlendModeColor:
		return "Color"
	case BlendModeLuminosity:
		return "Luminosity"
	}
	return "BlendMode(?)"
}

// shouldPreScaleCoverage is BlendMode::should_pre_scale_coverage in tiny-skia
// src/blend_mode.rs.
func (m BlendMode) shouldPreScaleCoverage() bool {
	// The most important things we do here are:
	//   1) never pre-scale with rgb coverage if the blend mode involves a source-alpha term,
	//   2) always pre-scale Plus.
	//
	// When we pre-scale with rgb coverage, we scale each of source r,g,b, with a distinct value,
	// and source alpha with one of those three values. This process destructively updates the
	// source-alpha term, so we cannot evaluate blend modes that need its original value.
	//
	// Plus always requires pre-scaling as a specific quirk of its implementation in
	// RasterPipeline. This lets us put the clamp inside the blend mode itself rather
	// than as a separate stage that comes after the lerp.
	//
	// This function is a finer-grained breakdown of SkBlendMode_SupportsCoverageAsAlpha().
	switch m {
	case BlendModeDestination, // d              --> no sa term, ok!
		BlendModeDestinationOver, // d + s*inv(da)  --> no sa term, ok!
		BlendModePlus,            // clamp(s+d)     --> no sa term, ok!
		BlendModeDestinationOut,  // d * inv(sa)
		BlendModeSourceAtop,      // s*da + d*inv(sa)
		BlendModeSourceOver,      // s + d*inv(sa)
		BlendModeXor:             // s*inv(da) + d*inv(sa)
		return true
	}
	return false
}

// toStage is BlendMode::to_stage in tiny-skia src/blend_mode.rs.
func (m BlendMode) toStage() (stage, bool) {
	switch m {
	case BlendModeClear:
		return stageClear, true
	case BlendModeSource:
		return 0, false // This stage is a no-op.
	case BlendModeDestination:
		return stageMoveDestinationToSource, true
	case BlendModeSourceOver:
		return stageSourceOver, true
	case BlendModeDestinationOver:
		return stageDestinationOver, true
	case BlendModeSourceIn:
		return stageSourceIn, true
	case BlendModeDestinationIn:
		return stageDestinationIn, true
	case BlendModeSourceOut:
		return stageSourceOut, true
	case BlendModeDestinationOut:
		return stageDestinationOut, true
	case BlendModeSourceAtop:
		return stageSourceAtop, true
	case BlendModeDestinationAtop:
		return stageDestinationAtop, true
	case BlendModeXor:
		return stageXor, true
	case BlendModePlus:
		return stagePlus, true
	case BlendModeModulate:
		return stageModulate, true
	case BlendModeScreen:
		return stageScreen, true
	case BlendModeOverlay:
		return stageOverlay, true
	case BlendModeDarken:
		return stageDarken, true
	case BlendModeLighten:
		return stageLighten, true
	case BlendModeColorDodge:
		return stageColorDodge, true
	case BlendModeColorBurn:
		return stageColorBurn, true
	case BlendModeHardLight:
		return stageHardLight, true
	case BlendModeSoftLight:
		return stageSoftLight, true
	case BlendModeDifference:
		return stageDifference, true
	case BlendModeExclusion:
		return stageExclusion, true
	case BlendModeMultiply:
		return stageMultiply, true
	case BlendModeHue:
		return stageHue, true
	case BlendModeSaturation:
		return stageSaturation, true
	case BlendModeColor:
		return stageColor, true
	case BlendModeLuminosity:
		return stageLuminosity, true
	}
	return 0, false
}
