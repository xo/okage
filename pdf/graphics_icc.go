// Copyright 2024 The krilla Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package pdf

import (
	_ "embed"
	"encoding/binary"
	"sync"

	"github.com/xo/okage/internal/pdfwriter"
)

// ICCProfile is ICCProfile<C> in krilla src/graphics/icc.rs. The constant C
// of upstream is the number of components of the color space of the
// profile. Go has no constant type parameter, so the profile holds the
// number. A *ICCProfile is cheap to share, as the Arc of upstream.
//
// An ICC profile.
type ICCProfile struct {
	// components is the constant C of upstream.
	components uint8
	repr       prehashed[iccRepr]
}

// NewICCProfile is ICCProfile::new in krilla src/graphics/icc.rs, with C in
// components. It deflates the data at once, as upstream does.
//
// Create a new ICC profile.
//
// Returns false if the metadata of the profile could not be read or if the
// number of channels of the underlying data does not correspond to
// components.
func NewICCProfile(data []byte, components uint8) (*ICCProfile, bool) {
	metadata, ok := iccMetadataFromData(data)
	if !ok {
		return nil, false
	}
	if metadata.colorSpace.numComponents() != components {
		return nil, false
	}
	return &ICCProfile{
		components: components,
		repr: newPrehashed(iccRepr{
			data:     deflateEncode(data),
			metadata: metadata,
		}),
	}, true
}

// metadata is ICCProfile::metadata in krilla src/graphics/icc.rs.
func (p *ICCProfile) metadata() iccMetadata {
	return p.repr.value.metadata
}

// typeID names the type for sipHash.
func (p *ICCProfile) typeID() string { return "ICCProfile" }

// hash is the derived Hash of ICCProfile in krilla src/graphics/icc.rs. The
// type_id of upstream holds the constant C, so the hash holds the number of
// components.
func (p *ICCProfile) hash(h *hasher) {
	h.u8(p.components)
	p.repr.write(h)
}

// serialize is Cacheable::serialize for ICCProfile in krilla
// src/graphics/icc.rs. It writes the deflated data as it is.
func (p *ICCProfile) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	chunk := sc.newChunk()
	iccStream := newFilterStreamFromDeflated(p.repr.value.data).finish(sc.serializeSettings())

	iccProfile := chunk.ICCProfile(rootRef, iccStream.encodedData())
	iccProfile.N(int32(p.components)).Range(repeatUnitRange(int(p.components))...)
	iccStream.writeFilters(iccProfile.Dict)
	iccProfile.Finish()
	cc.streams.iccProfiles = append(cc.streams.iccProfiles, chunk)
}

// genericICCProfile is GenericICCProfile in krilla src/graphics/icc.rs, the
// profile of an image. The cases Luma, Rgb and Cmyk of upstream are the
// profiles of 1, 3 and 4 components, which the profile holds. A nil profile
// is the None of an Option<GenericICCProfile>.
//
// upstream also implements Cacheable for GenericICCProfile, but no code of
// krilla registers one. An image registers the ICCBasedColorSpace of its
// profile. So the port leaves that impl out.
type genericICCProfile struct {
	profile *ICCProfile
}

// metadata is GenericICCProfile::metadata in krilla src/graphics/icc.rs.
func (g genericICCProfile) metadata() iccMetadata {
	return g.profile.metadata()
}

// iccBasedColorSpace is ICCBasedColorSpace<C> in krilla
// src/graphics/icc.rs.
type iccBasedColorSpace struct {
	profile *ICCProfile
}

// typeID names the type for sipHash.
func (iccBasedColorSpace) typeID() string { return "iccBasedColorSpace" }

// hash is the derived Hash of ICCBasedColorSpace in krilla
// src/graphics/icc.rs.
func (cs iccBasedColorSpace) hash(h *hasher) {
	cs.profile.hash(h)
}

// serialize is Cacheable::serialize for ICCBasedColorSpace in krilla
// src/graphics/icc.rs. It writes the profile once, and the array of the
// color space.
func (cs iccBasedColorSpace) serialize(sc *serializeContext, cc *chunkContainer, rootRef pdfwriter.Ref) {
	iccRef := sc.registerCacheable(cc, cs.profile)

	chunk := cc.nonStream.colorSpaces

	array := chunk.Indirect(rootRef).Array()
	array.Item(pdfwriter.Name("ICCBased"))
	array.Item(iccRef)
	array.Finish()
}

// newResource is Resourceable for ICCBasedColorSpace in krilla
// src/graphics/icc.rs.
func (iccBasedColorSpace) newResource(ref pdfwriter.Ref) colorSpaceResource {
	return colorSpaceResource{ref}
}

// iccColorSpace is ICCColorSpace in krilla src/graphics/icc.rs. There are
// more, but those are the most important ones.
type iccColorSpace uint8

// The cases of ICCColorSpace.
const (
	iccColorSpaceXYZ iccColorSpace = iota
	iccColorSpaceLab
	iccColorSpaceLuv
	iccColorSpaceYCbr
	iccColorSpaceYxy
	iccColorSpaceLMS
	iccColorSpaceRGB
	iccColorSpaceGray
	iccColorSpaceHSV
	iccColorSpaceHLS
	iccColorSpaceCMYK
	iccColorSpaceCMY
	iccColorSpaceOneClr
	iccColorSpaceThreeClr
	iccColorSpaceFourClr
)

// numComponents is ICCColorSpace::num_components in krilla
// src/graphics/icc.rs.
func (cs iccColorSpace) numComponents() uint8 {
	switch cs {
	case iccColorSpaceGray, iccColorSpaceOneClr:
		return 1
	case iccColorSpaceCMYK, iccColorSpaceFourClr:
		return 4
	default:
		return 3
	}
}

// iccColorSpaceFromU32 is the impl TryFrom<u32> for ICCColorSpace in krilla
// src/graphics/icc.rs. It reads the signature of the color space of the
// header of a profile.
func iccColorSpaceFromU32(value uint32) (iccColorSpace, bool) {
	switch value {
	case 0x58595A20:
		return iccColorSpaceXYZ, true
	case 0x4C616220:
		return iccColorSpaceLab, true
	case 0x4C757620:
		return iccColorSpaceLuv, true
	case 0x59436272:
		return iccColorSpaceYCbr, true
	case 0x59787920:
		return iccColorSpaceYxy, true
	case 0x4C4D5320:
		return iccColorSpaceLMS, true
	case 0x52474220:
		return iccColorSpaceRGB, true
	case 0x47524159:
		return iccColorSpaceGray, true
	case 0x48535620:
		return iccColorSpaceHSV, true
	case 0x484C5320:
		return iccColorSpaceHLS, true
	case 0x434D594B:
		return iccColorSpaceCMYK, true
	case 0x434D5920:
		return iccColorSpaceCMY, true
	case 0x31434C52:
		return iccColorSpaceOneClr, true
	case 0x33434C52:
		return iccColorSpaceThreeClr, true
	case 0x34434C52:
		return iccColorSpaceFourClr, true
	}
	return 0, false
}

// iccMetadata is ICCMetadata in krilla src/graphics/icc.rs: the version of
// a profile and its color space.
type iccMetadata struct {
	major      uint8
	minor      uint8
	colorSpace iccColorSpace
}

// iccMetadataFromData is ICCMetadata::from_data in krilla
// src/graphics/icc.rs. It reads the header of a profile, and returns false
// when the data is too short or the color space is not known.
func iccMetadataFromData(data []byte) (iccMetadata, bool) {
	if len(data) < 20 {
		return iccMetadata{}, false
	}
	major := data[8]
	minor := data[9] >> 4
	colorSpace, ok := iccColorSpaceFromU32(binary.BigEndian.Uint32(data[16:20]))
	if !ok {
		return iccMetadata{}, false
	}
	return iccMetadata{major: major, minor: minor, colorSpace: colorSpace}, true
}

// hash is the derived Hash of ICCMetadata in krilla src/graphics/icc.rs.
func (m iccMetadata) hash(h *hasher) {
	h.u8(m.major)
	h.u8(m.minor)
	h.usize(int(m.colorSpace))
}

// iccRepr is Repr in krilla src/graphics/icc.rs.
type iccRepr struct {
	data     []byte
	metadata iccMetadata
}

// typeID names the type for sipHash.
func (iccRepr) typeID() string { return "iccRepr" }

// hash is the derived Hash of Repr in krilla src/graphics/icc.rs.
func (r iccRepr) hash(h *hasher) {
	h.bytes(r.data)
	r.metadata.hash(h)
}

// The ICC profiles that krilla embeds, in krilla src/configure/version.rs.
// The files in the folder icc are the files of upstream, byte for byte, with
// their license, CC0 1.0.
var (
	//go:embed icc/sRGB-v4.icc
	srgbV4ICCData []byte
	//go:embed icc/sRGB-v2-magic.icc
	srgbV2ICCData []byte
	//go:embed icc/sGrey-v4.icc
	greyV4ICCData []byte
	//go:embed icc/sGrey-v2-magic.icc
	greyV2ICCData []byte

	// srgbV4ICC is SRGB_V4_ICC, the ICC v4 profile for the sRGB color
	// space.
	srgbV4ICC = sync.OnceValue(func() *ICCProfile { return mustICCProfile(srgbV4ICCData, 3) })
	// srgbV2ICC is SRGB_V2_ICC, the ICC v2 profile for the sRGB color
	// space.
	srgbV2ICC = sync.OnceValue(func() *ICCProfile { return mustICCProfile(srgbV2ICCData, 3) })
	// greyV4ICC is GREY_V4_ICC, the ICC v4 profile for the sGrey color
	// space.
	greyV4ICC = sync.OnceValue(func() *ICCProfile { return mustICCProfile(greyV4ICCData, 1) })
	// greyV2ICC is GREY_V2_ICC, the ICC v2 profile for the sGrey color
	// space.
	greyV2ICC = sync.OnceValue(func() *ICCProfile { return mustICCProfile(greyV2ICCData, 1) })
)

// mustICCProfile is the unwrap of the profiles of krilla
// src/configure/version.rs. The data is embedded, so a failure is a fault of
// the package.
func mustICCProfile(data []byte, components uint8) *ICCProfile {
	p, ok := NewICCProfile(data, components)
	if !ok {
		panic("pdf: an embedded ICC profile is not valid")
	}
	return p
}

// rgbICC is PdfVersion::rgb_icc in krilla src/configure/version.rs. PDF 1.4
// to 1.6 take the profile of ICC v2.
func (v PDFVersion) rgbICC() *ICCProfile {
	switch v {
	case PDF14, PDF15, PDF16:
		return srgbV2ICC()
	default:
		return srgbV4ICC()
	}
}

// greyICC is PdfVersion::grey_icc in krilla src/configure/version.rs. PDF 1.4
// to 1.6 take the profile of ICC v2.
func (v PDFVersion) greyICC() *ICCProfile {
	switch v {
	case PDF14, PDF15, PDF16:
		return greyV2ICC()
	default:
		return greyV4ICC()
	}
}
