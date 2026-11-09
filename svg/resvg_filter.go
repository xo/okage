// Copyright 2018 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"errors"
	"math"
	"slices"

	"github.com/xo/okage"
	"github.com/xo/okage/internal/wide"
	"github.com/xo/okage/path"
)

// This file ports resvg src/filter/mod.rs.
//
// Port: the private names of this module that collide with names of usvg
// take the name of the module as a suffix (D10): apply becomes applyFilter,
// f32_bound becomes f32BoundFilter, the enum Error becomes errorFilter and
// the struct Image becomes imageFilter. The function apply of each submodule
// takes the path of its module, such as applyFilterBoxBlur for
// filter/box_blur.rs, because a name such as applyColorMatrix is already the
// port of apply_color_matrix of this module.
//
// The type RGBA8 of the crate rgb is rgba8. Upstream views the bytes of a
// pixmap as a slice of RGBA8 with bytemuck. Go cannot do that without the
// package unsafe (D25), so imageRef and imageRefMut hold the bytes, and a
// loop over the pixels steps 4 bytes at a time.
//
// The methods of the traits PixmapExt and PixmapToImageRef, which upstream
// implements for tiny_skia::Pixmap, are functions that take the pixmap
// first, such as pixmapIntoSRGB, because Go cannot add a method to a type of
// another package.
//
// Each function that logs or makes a pixmap takes the options of Render, for
// the logger of D20.

// TODO: apply single primitive filters in-place

// rgba8 is RGBA8 of the crate rgb, which resvg uses for a pixel.
type rgba8 struct {
	r, g, b, a uint8
}

// rgba8At returns the pixel at index i of data, which holds RGBA bytes. It is
// data[i] of a Rust slice of RGBA8. It ports no upstream function.
func rgba8At(data []byte, i int) rgba8 {
	p := data[i*4 : i*4+4 : i*4+4]
	return rgba8{r: p[0], g: p[1], b: p[2], a: p[3]}
}

// setRGBA8At sets the pixel at index i of data to p. It is data[i] = p of a
// Rust slice of RGBA8. It ports no upstream function.
func setRGBA8At(data []byte, i int, p rgba8) {
	d := data[i*4 : i*4+4 : i*4+4]
	d[0], d[1], d[2], d[3] = p.r, p.g, p.b, p.a
}

// imageRef is ImageRef in resvg src/filter/mod.rs. It is an image reference.
//
// Image pixels should be stored in RGBA order.
//
// Some filters will require premultiplied channels, some not.
// See specific filter documentation for details.
type imageRef struct {
	data   []byte
	width  uint32
	height uint32
}

// newImageRef is ImageRef::new in resvg src/filter/mod.rs. It creates a new
// image reference.
//
// Doesn't clone the provided data.
func newImageRef(width, height uint32, data []byte) imageRef {
	return imageRef{
		data:   data,
		width:  width,
		height: height,
	}
}

// alphaAt is ImageRef::alpha_at in resvg src/filter/mod.rs.
func (r imageRef) alphaAt(x, y uint32) int16 {
	return int16(r.data[int(r.width*y+x)*4+3])
}

// imageRefMut is ImageRefMut in resvg src/filter/mod.rs. It is a mutable
// `ImageRef` variant.
type imageRefMut struct {
	data   []byte
	width  uint32
	height uint32
}

// newImageRefMut is ImageRefMut::new in resvg src/filter/mod.rs. It creates a
// new mutable image reference.
//
// Doesn't clone the provided data.
func newImageRefMut(width, height uint32, data []byte) imageRefMut {
	return imageRefMut{
		data:   data,
		width:  width,
		height: height,
	}
}

// pixelAt is ImageRefMut::pixel_at in resvg src/filter/mod.rs.
func (r imageRefMut) pixelAt(x, y uint32) rgba8 {
	return rgba8At(r.data, int(r.width*y+x))
}

// setPixelAt is ImageRefMut::pixel_at_mut in resvg src/filter/mod.rs, with a
// write through the reference that it returns.
func (r imageRefMut) setPixelAt(x, y uint32, p rgba8) {
	setRGBA8At(r.data, int(r.width*y+x), p)
}

// errorFilter is Error in resvg src/filter/mod.rs.
type errorFilter uint8 //nolint:errname // D10 names the enum Error of resvg src/filter/mod.rs after its module

// The values of errorFilter.
const (
	errorFilterInvalidRegion errorFilter = iota //nolint:errname // the case Error::InvalidRegion of upstream (D10)
	errorFilterNoResults                        //nolint:errname // the case Error::NoResults of upstream (D10)
)

// Error is the derived impl Debug for Error in resvg src/filter/mod.rs.
func (e errorFilter) Error() string {
	switch e {
	case errorFilterInvalidRegion:
		return "InvalidRegion"
	case errorFilterNoResults:
		return "NoResults"
	}
	return "Error(?)"
}

// pixmapTryCreate is PixmapExt::try_create in resvg src/filter/mod.rs.
func pixmapTryCreate(width, height uint32, opt *options) (*okage.Pixmap, error) {
	p, ok := newPixmap(width, height, opt)
	if !ok {
		return nil, errorFilterInvalidRegion
	}
	return p, nil
}

// pixmapCopyRegion is PixmapExt::copy_region in resvg src/filter/mod.rs.
func pixmapCopyRegion(p *okage.Pixmap, region path.IntRect) (*okage.Pixmap, error) {
	rect, ok := path.IntRectFromXYWH(region.X(), region.Y(), region.Width(), region.Height())
	if !ok {
		return nil, errorFilterInvalidRegion
	}
	r, ok := p.CloneRect(rect)
	if !ok {
		return nil, errorFilterInvalidRegion
	}
	return r, nil
}

// pixmapClear is PixmapExt::clear in resvg src/filter/mod.rs.
func pixmapClear(p *okage.Pixmap) {
	p.Fill(okage.ColorTransparent)
}

// pixmapIntoSRGB is PixmapExt::into_srgb in resvg src/filter/mod.rs.
func pixmapIntoSRGB(p *okage.Pixmap) {
	demultiplyAlpha(p.DataMut())
	fromLinearRGB(p.DataMut())
	multiplyAlpha(p.DataMut())
}

// pixmapIntoLinearRGB is PixmapExt::into_linear_rgb in resvg
// src/filter/mod.rs.
func pixmapIntoLinearRGB(p *okage.Pixmap) {
	demultiplyAlpha(p.DataMut())
	intoLinearRGB(p.DataMut())
	multiplyAlpha(p.DataMut())
}

// multiplyAlpha is multiply_alpha in resvg src/filter/mod.rs. It multiplies
// provided pixels alpha.
func multiplyAlpha(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		p := data[i : i+4 : i+4]
		a := float32(p[3]) / 255.0
		p[2] = wide.TruncU8(float32(float32(p[2])*a) + 0.5)
		p[1] = wide.TruncU8(float32(float32(p[1])*a) + 0.5)
		p[0] = wide.TruncU8(float32(float32(p[0])*a) + 0.5)
	}
}

// demultiplyAlpha is demultiply_alpha in resvg src/filter/mod.rs. It
// demultiplies provided pixels alpha.
func demultiplyAlpha(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		p := data[i : i+4 : i+4]
		a := float32(p[3]) / 255.0
		p[2] = wide.TruncU8(float32(float32(p[2])/a) + 0.5)
		p[1] = wide.TruncU8(float32(float32(p[1])/a) + 0.5)
		p[0] = wide.TruncU8(float32(float32(p[0])/a) + 0.5)
	}
}

// srgbToLinearRGBTable is SRGB_TO_LINEAR_RGB_TABLE in resvg
// src/filter/mod.rs. It is a precomputed sRGB to LinearRGB table.
//
// Since we are storing the result in `u8`, there is no need to compute those
// values each time. Mainly because it's very expensive.
//
//	if (C_srgb <= 0.04045)
//	    C_lin = C_srgb / 12.92;
//	 else
//	    C_lin = pow((C_srgb + 0.055) / 1.055, 2.4);
//
// Thanks to librsvg for the idea.
var srgbToLinearRGBTable = [256]uint8{ //nolint:dupl // the two tables of upstream
	0, 0, 0, 0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3,
	4, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 6, 6, 7, 7, 7,
	8, 8, 8, 8, 9, 9, 9, 10, 10, 10, 11, 11, 12, 12, 12, 13,
	13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 17, 18, 18, 19, 19, 20,
	20, 21, 22, 22, 23, 23, 24, 24, 25, 25, 26, 27, 27, 28, 29, 29,
	30, 30, 31, 32, 32, 33, 34, 35, 35, 36, 37, 37, 38, 39, 40, 41,
	41, 42, 43, 44, 45, 45, 46, 47, 48, 49, 50, 51, 51, 52, 53, 54,
	55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70,
	71, 72, 73, 74, 76, 77, 78, 79, 80, 81, 82, 84, 85, 86, 87, 88,
	90, 91, 92, 93, 95, 96, 97, 99, 100, 101, 103, 104, 105, 107, 108, 109,
	111, 112, 114, 115, 116, 118, 119, 121, 122, 124, 125, 127, 128, 130, 131, 133,
	134, 136, 138, 139, 141, 142, 144, 146, 147, 149, 151, 152, 154, 156, 157, 159,
	161, 163, 164, 166, 168, 170, 171, 173, 175, 177, 179, 181, 183, 184, 186, 188,
	190, 192, 194, 196, 198, 200, 202, 204, 206, 208, 210, 212, 214, 216, 218, 220,
	222, 224, 226, 229, 231, 233, 235, 237, 239, 242, 244, 246, 248, 250, 253, 255,
}

// linearRGBToSRGBTable is LINEAR_RGB_TO_SRGB_TABLE in resvg
// src/filter/mod.rs. It is a precomputed LinearRGB to sRGB table.
//
// Since we are storing the result in `u8`, there is no need to compute those
// values each time. Mainly because it's very expensive.
//
//	if (C_lin <= 0.0031308)
//	    C_srgb = C_lin * 12.92;
//	else
//	    C_srgb = 1.055 * pow(C_lin, 1.0 / 2.4) - 0.055;
//
// Thanks to librsvg for the idea.
var linearRGBToSRGBTable = [256]uint8{ //nolint:dupl // the two tables of upstream
	0, 13, 22, 28, 34, 38, 42, 46, 50, 53, 56, 59, 61, 64, 66, 69,
	71, 73, 75, 77, 79, 81, 83, 85, 86, 88, 90, 92, 93, 95, 96, 98,
	99, 101, 102, 104, 105, 106, 108, 109, 110, 112, 113, 114, 115, 117, 118, 119,
	120, 121, 122, 124, 125, 126, 127, 128, 129, 130, 131, 132, 133, 134, 135, 136,
	137, 138, 139, 140, 141, 142, 143, 144, 145, 146, 147, 148, 148, 149, 150, 151,
	152, 153, 154, 155, 155, 156, 157, 158, 159, 159, 160, 161, 162, 163, 163, 164,
	165, 166, 167, 167, 168, 169, 170, 170, 171, 172, 173, 173, 174, 175, 175, 176,
	177, 178, 178, 179, 180, 180, 181, 182, 182, 183, 184, 185, 185, 186, 187, 187,
	188, 189, 189, 190, 190, 191, 192, 192, 193, 194, 194, 195, 196, 196, 197, 197,
	198, 199, 199, 200, 200, 201, 202, 202, 203, 203, 204, 205, 205, 206, 206, 207,
	208, 208, 209, 209, 210, 210, 211, 212, 212, 213, 213, 214, 214, 215, 215, 216,
	216, 217, 218, 218, 219, 219, 220, 220, 221, 221, 222, 222, 223, 223, 224, 224,
	225, 226, 226, 227, 227, 228, 228, 229, 229, 230, 230, 231, 231, 232, 232, 233,
	233, 234, 234, 235, 235, 236, 236, 237, 237, 238, 238, 238, 239, 239, 240, 240,
	241, 241, 242, 242, 243, 243, 244, 244, 245, 245, 246, 246, 246, 247, 247, 248,
	248, 249, 249, 250, 250, 251, 251, 251, 252, 252, 253, 253, 254, 254, 255, 255,
}

// intoLinearRGB is into_linear_rgb in resvg src/filter/mod.rs. It converts
// input pixel from sRGB into LinearRGB.
//
// Provided pixels should have an **unpremultiplied alpha**.
//
// RGB channels order of the input image doesn't matter, but alpha channel must be the last one.
func intoLinearRGB(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		data[i+0] = srgbToLinearRGBTable[data[i+0]]
		data[i+1] = srgbToLinearRGBTable[data[i+1]]
		data[i+2] = srgbToLinearRGBTable[data[i+2]]
	}
}

// fromLinearRGB is from_linear_rgb in resvg src/filter/mod.rs. It converts
// input pixel from LinearRGB into sRGB.
//
// Provided pixels should have an **unpremultiplied alpha**.
//
// RGB channels order of the input image doesn't matter, but alpha channel must be the last one.
func fromLinearRGB(data []byte) {
	for i := 0; i+4 <= len(data); i += 4 {
		data[i+0] = linearRGBToSRGBTable[data[i+0]]
		data[i+1] = linearRGBToSRGBTable[data[i+1]]
		data[i+2] = linearRGBToSRGBTable[data[i+2]]
	}
}

// f32BoundFilter is f32_bound in resvg src/filter/mod.rs.
//
// TODO: https://github.com/rust-lang/rust/issues/44095
func f32BoundFilter(minimum, val, maximum float32) float32 { //nolint:unparam // the signature of upstream is kept
	switch {
	case val > maximum:
		return maximum
	case val >= minimum:
		return val
	}
	// Catches `val < min` as well as a NaN `val`.
	return minimum
}

// imageFilter is Image in resvg src/filter/mod.rs.
//
// Port: upstream shares the pixmap with Rc. Go shares the pointer, and no
// code changes a pixmap that an imageFilter holds. take returns a copy.
type imageFilter struct {
	// Filter primitive result.
	//
	// All images have the same size which is equal to the current filter region.
	image *okage.Pixmap

	// Image's region that has actual data.
	//
	// Region is in global coordinates and not in `image` one.
	//
	// Image's content outside this region will be transparent/cleared.
	//
	// Currently used only for `feTile`.
	region path.IntRect

	// The current color space.
	colorSpace ColorInterpolationFilter
}

// imageFilterFromImage is Image::from_image in resvg src/filter/mod.rs.
func imageFilterFromImage(img *okage.Pixmap, colorSpace ColorInterpolationFilter) imageFilter {
	w, h := img.Width(), img.Height()
	region, ok := path.IntRectFromXYWH(0, 0, w, h)
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
	return imageFilter{
		image:      img,
		region:     region,
		colorSpace: colorSpace,
	}
}

// intoColorSpace is Image::into_color_space in resvg src/filter/mod.rs.
func (i imageFilter) intoColorSpace(colorSpace ColorInterpolationFilter) (imageFilter, error) {
	if colorSpace != i.colorSpace {
		region := i.region

		img, err := i.take()
		if err != nil {
			return imageFilter{}, err
		}

		switch colorSpace {
		case ColorInterpolationFilterSRGB:
			pixmapIntoSRGB(img)
		case ColorInterpolationFilterLinearRGB:
			pixmapIntoLinearRGB(img)
		}

		return imageFilter{
			image:      img,
			region:     region,
			colorSpace: colorSpace,
		}, nil
	}
	return i, nil
}

// take is Image::take in resvg src/filter/mod.rs.
//
// Port: upstream moves the pixmap out of the Rc when no other Image shares
// it, and clones it if not. Go cannot know, so it always clones it. The
// pixels are the same.
func (i imageFilter) take() (*okage.Pixmap, error) { //nolint:unparam // upstream returns a Result
	return i.image.Clone(), nil
}

// width is Image::width in resvg src/filter/mod.rs.
func (i imageFilter) width() uint32 {
	return i.image.Width()
}

// height is Image::height in resvg src/filter/mod.rs.
func (i imageFilter) height() uint32 {
	return i.image.Height()
}

// asRef is Image::as_ref in resvg src/filter/mod.rs.
func (i imageFilter) asRef() *okage.Pixmap {
	return i.image
}

// filterResult is FilterResult in resvg src/filter/mod.rs.
type filterResult struct {
	name  string
	image imageFilter
}

// applyFilter is apply in resvg src/filter/mod.rs.
func applyFilter(
	filter *Filter,
	ts Transform,
	source *okage.Pixmap,
	opt *options,
) {
	img, region, err := applyInner(filter, ts, source, opt)
	if err == nil {
		err = applyToCanvas(img, region, source)
	}

	// Clear on error.
	if err != nil {
		source.Fill(okage.ColorTransparent)
	}

	switch {
	case err == nil:
	case errors.Is(err, errorFilterInvalidRegion):
		opt.warnf("Filter has an invalid region.")
	case errors.Is(err, errorFilterNoResults):
	}
}

// applyInner is apply_inner in resvg src/filter/mod.rs.
func applyInner(
	filter *Filter,
	ts Transform,
	source *okage.Pixmap,
	opt *options,
) (imageFilter, path.IntRect, error) {
	fr, ok := filter.Rect().Transform(ts)
	if !ok {
		return imageFilter{}, path.IntRect{}, errorFilterInvalidRegion
	}
	region := fr.ToIntRect()

	// The source pixmap is clamped to `max_bbox` in `render_group`, so the filter
	// region (derived directly from the unclamped filter rect) can be larger than
	// the buffer we actually render into.
	sourceRect, ok := path.IntRectFromXYWH(0, 0, source.Width(), source.Height())
	if !ok {
		return imageFilter{}, path.IntRect{}, errorFilterInvalidRegion
	}
	region, ok = fitToRect(region, sourceRect)
	if !ok {
		return imageFilter{}, path.IntRect{}, errorFilterInvalidRegion
	}

	if region != sourceRect {
		// TODO: Avoid copying?
		croppedSource, err := pixmapCopyRegion(source, region)
		if err != nil {
			return imageFilter{}, path.IntRect{}, err
		}
		source = croppedSource
	}
	outputRegion := region
	ts = ts.PostTranslate(float32(-region.X()), float32(-region.Y()))
	region, ok = region.TranslateTo(0, 0)
	if !ok {
		return imageFilter{}, path.IntRect{}, errorFilterInvalidRegion
	}

	var results []filterResult

	primitives := filter.Primitives()
	for pi := range primitives {
		primitive := &primitives[pi]
		pr, ok := primitive.Rect().Transform(ts)
		if !ok {
			return imageFilter{}, path.IntRect{}, errorFilterInvalidRegion
		}
		subregion := pr.ToIntRect()

		// `feOffset` inherits its region from the input.
		if fe, ok := primitive.Kind().(*OffsetFilter); ok {
			if fe.Input().Kind == InputFilterReference {
				if res, ok := findResult(results, fe.Input().Reference); ok {
					subregion = res.image.region
				}
			}
		}

		cs := primitive.ColorInterpolation()

		var result, input, input1, input2 imageFilter
		var err error
		switch fe := primitive.Kind().(type) {
		case *BlendFilter:
			input1, err = getInput(fe.Input1(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			input2, err = getInput(fe.Input2(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyBlend(fe, cs, region, input1, input2, opt)
		case *DropShadowFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyDropShadow(fe, cs, ts, input, opt)
		case *FloodFilter:
			result, err = applyFlood(fe, region, opt)
		case *GaussianBlurFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyBlur(fe, cs, ts, input)
		case *OffsetFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyOffset(fe, ts, input, opt)
		case *CompositeFilter:
			input1, err = getInput(fe.Input1(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			input2, err = getInput(fe.Input2(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyComposite(fe, cs, region, input1, input2, opt)
		case *MergeFilter:
			result, err = applyMerge(fe, cs, region, source, results, opt)
		case *TileFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyTile(input, region, opt)
		case *ImageFilter:
			result, err = applyImage(fe, region, subregion, ts, opt)
		case *ComponentTransferFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyComponentTransfer(fe, cs, input)
		case *ColorMatrixFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyColorMatrix(fe, cs, input)
		case *ConvolveMatrixFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyConvolveMatrix(fe, cs, input)
		case *MorphologyFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyMorphology(fe, cs, ts, input)
		case *DisplacementMapFilter:
			input1, err = getInput(fe.Input1(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			input2, err = getInput(fe.Input2(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyDisplacementMap(fe, region, cs, ts, input1, input2, opt)
		case *TurbulenceFilter:
			result, err = applyTurbulence(fe, region, cs, ts, opt)
		case *DiffuseLightingFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applyDiffuseLighting(fe, region, cs, ts, input, opt)
		case *SpecularLightingFilter:
			input, err = getInput(fe.Input(), region, source, results, opt)
			if err != nil {
				return imageFilter{}, path.IntRect{}, err
			}
			result, err = applySpecularLighting(fe, region, cs, ts, input, opt)
		}
		if err != nil {
			return imageFilter{}, path.IntRect{}, err
		}

		if region != subregion {
			// Clip result.

			// TODO: explain
			var subregion2 path.IntRect
			if _, ok := primitive.Kind().(*OffsetFilter); ok {
				// We do not support clipping on feOffset.
				subregion2, ok = region.TranslateTo(0, 0)
				mustOK(ok)
			} else {
				subregion2, ok = subregion.Translate(-region.X(), -region.Y())
				mustOK(ok)
			}

			colorSpace := result.colorSpace

			var pixmap *okage.Pixmap
			{
				// This is cropping by clearing the pixels outside the region.
				paint := okage.DefaultPaint()
				paint.SetColor(okage.ColorBlack)
				paint.BlendMode = okage.BlendModeClear

				pixmap, err = result.take()
				if err != nil {
					return imageFilter{}, path.IntRect{}, err
				}
				w := float32(pixmap.Width())
				h := float32(pixmap.Height())

				if rect, ok := path.RectFromXYWH(0.0, 0.0, w, float32(subregion2.Y())); ok {
					pixmap.FillRect(rect, &paint, path.IdentityTransform(), nil)
				}

				if rect, ok := path.RectFromXYWH(0.0, 0.0, float32(subregion2.X()), h); ok {
					pixmap.FillRect(rect, &paint, path.IdentityTransform(), nil)
				}

				if rect, ok := path.RectFromXYWH(float32(subregion2.Right()), 0.0, w, h); ok {
					pixmap.FillRect(rect, &paint, path.IdentityTransform(), nil)
				}

				if rect, ok := path.RectFromXYWH(0.0, float32(subregion2.Bottom()), w, h); ok {
					pixmap.FillRect(rect, &paint, path.IdentityTransform(), nil)
				}
			}

			result = imageFilter{
				image:      pixmap,
				region:     subregion,
				colorSpace: colorSpace,
			}
		}

		results = append(results, filterResult{
			name:  primitive.Result(),
			image: result,
		})
	}

	if len(results) != 0 {
		res := results[len(results)-1]
		return res.image, outputRegion, nil
	}
	return imageFilter{}, path.IntRect{}, errorFilterNoResults
}

// findResult is results.iter().rev().find(|v| v.name == *name) of resvg
// src/filter/mod.rs. It ports no upstream function.
func findResult(results []filterResult, name string) (filterResult, bool) {
	for _, v := range slices.Backward(results) {
		if v.name == name {
			return v, true
		}
	}
	return filterResult{}, false
}

// mustOK is the unwrap of an Option that upstream relies on. It panics when
// ok is false.
func mustOK(ok bool) {
	if !ok {
		panic("svg: called Option::unwrap() on a None value")
	}
}

// getInput is get_input in resvg src/filter/mod.rs.
func getInput(
	input InputFilter,
	region path.IntRect,
	source *okage.Pixmap,
	results []filterResult,
	opt *options,
) (imageFilter, error) {
	switch input.Kind {
	case InputFilterSourceGraphic:
		img := source.Clone()

		return imageFilter{
			image:      img,
			region:     region,
			colorSpace: ColorInterpolationFilterSRGB,
		}, nil
	case InputFilterSourceAlpha:
		img := source.Clone()
		// Set RGB to black. Keep alpha as is.
		data := img.DataMut()
		for i := 0; i+4 <= len(data); i += 4 {
			data[i+0] = 0
			data[i+1] = 0
			data[i+2] = 0
		}

		return imageFilter{
			image:      img,
			region:     region,
			colorSpace: ColorInterpolationFilterSRGB,
		}, nil
	case InputFilterReference:
		if v, ok := findResult(results, input.Reference); ok {
			return v.image, nil
		}
		// Technically unreachable.
		opt.warnf("Unknown filter primitive reference '%s'.", input.Reference)
		return getInput(InputFilter{Kind: InputFilterSourceGraphic}, region, source, results, opt)
	}
	return imageFilter{}, errorFilterInvalidRegion
}

// asImageRef is PixmapToImageRef::as_image_ref in resvg src/filter/mod.rs.
func asImageRef(p *okage.Pixmap) imageRef {
	return newImageRef(p.Width(), p.Height(), p.Data())
}

// asImageRefMut is PixmapToImageRef::as_image_ref_mut in resvg
// src/filter/mod.rs.
func asImageRefMut(p *okage.Pixmap) imageRefMut {
	return newImageRefMut(p.Width(), p.Height(), p.DataMut())
}

// applyDropShadow is apply_drop_shadow in resvg src/filter/mod.rs.
func applyDropShadow(
	fe *DropShadowFilter,
	cs ColorInterpolationFilter,
	ts Transform,
	input imageFilter,
	opt *options,
) (imageFilter, error) {
	dx, dy, ok := scaleCoordinates(fe.Dx(), fe.Dy(), ts)
	if !ok {
		return input, nil
	}

	pixmap, err := pixmapTryCreate(input.width(), input.height(), opt)
	if err != nil {
		return imageFilter{}, err
	}
	in, err := input.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	inputPixmap, err := in.take()
	if err != nil {
		return imageFilter{}, err
	}
	shadowPixmap := inputPixmap.Clone()

	if stdDx, stdDy, useBoxBlur, ok := resolveStdDev(fe.StdDevX().Get(), fe.StdDevY().Get(), ts); ok {
		if useBoxBlur {
			applyFilterBoxBlur(stdDx, stdDy, asImageRefMut(shadowPixmap))
		} else {
			applyFilterIIRBlur(stdDx, stdDy, asImageRefMut(shadowPixmap))
		}
	}

	// flood
	color := okage.ColorFromRGBA8(
		fe.Color().Red,
		fe.Color().Green,
		fe.Color().Blue,
		fe.Opacity().ToU8(),
	)
	pixels := shadowPixmap.PixelsMut()
	for i := range pixels.Len() {
		color := color
		color.ApplyOpacity(float32(pixels.At(i).Alpha()) / 255.0)
		pixels.Set(i, color.Premultiply().ToColorU8())
	}

	switch cs {
	case ColorInterpolationFilterSRGB:
		pixmapIntoSRGB(shadowPixmap)
	case ColorInterpolationFilterLinearRGB:
		pixmapIntoLinearRGB(shadowPixmap)
	}

	paint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(
		wide.TruncI32(dx),
		wide.TruncI32(dy),
		shadowPixmap.AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	pixmap.DrawPixmap(
		0,
		0,
		inputPixmap.AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	return imageFilterFromImage(pixmap, cs), nil
}

// applyBlur is apply_blur in resvg src/filter/mod.rs.
func applyBlur(
	fe *GaussianBlurFilter,
	cs ColorInterpolationFilter,
	ts Transform,
	input imageFilter,
) (imageFilter, error) {
	stdDx, stdDy, useBoxBlur, ok := resolveStdDev(fe.StdDevX().Get(), fe.StdDevY().Get(), ts)
	if !ok {
		return input, nil
	}

	in, err := input.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap, err := in.take()
	if err != nil {
		return imageFilter{}, err
	}

	if useBoxBlur {
		applyFilterBoxBlur(stdDx, stdDy, asImageRefMut(pixmap))
	} else {
		applyFilterIIRBlur(stdDx, stdDy, asImageRefMut(pixmap))
	}

	return imageFilterFromImage(pixmap, cs), nil
}

// applyOffset is apply_offset in resvg src/filter/mod.rs.
func applyOffset(
	fe *OffsetFilter,
	ts Transform,
	input imageFilter,
	opt *options,
) (imageFilter, error) {
	dx, dy, ok := scaleCoordinates(fe.Dx(), fe.Dy(), ts)
	if !ok {
		return input, nil
	}

	if ApproxZeroUlps32(dx, 4) && ApproxZeroUlps32(dy, 4) {
		return input, nil
	}

	pixmap, err := pixmapTryCreate(input.width(), input.height(), opt)
	if err != nil {
		return imageFilter{}, err
	}
	paint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(
		wide.TruncI32(dx),
		wide.TruncI32(dy),
		input.asRef().AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	return imageFilterFromImage(pixmap, input.colorSpace), nil
}

// applyBlend is apply_blend in resvg src/filter/mod.rs.
func applyBlend(
	fe *BlendFilter,
	cs ColorInterpolationFilter,
	region path.IntRect,
	input1 imageFilter,
	input2 imageFilter,
	opt *options,
) (imageFilter, error) {
	input1, err := input1.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	input2, err = input2.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}

	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	paint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(
		0,
		0,
		input2.asRef().AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	paint1 := okage.DefaultPixmapPaint()
	paint1.BlendMode = convertBlendMode(fe.Mode())
	pixmap.DrawPixmap(
		0,
		0,
		input1.asRef().AsRef(),
		&paint1,
		path.IdentityTransform(),
		nil,
	)

	return imageFilterFromImage(pixmap, cs), nil
}

// applyComposite is apply_composite in resvg src/filter/mod.rs.
func applyComposite(
	fe *CompositeFilter,
	cs ColorInterpolationFilter,
	region path.IntRect,
	input1 imageFilter,
	input2 imageFilter,
	opt *options,
) (imageFilter, error) {
	input1, err := input1.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	input2, err = input2.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}

	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	if op := fe.Operator(); op.Kind == CompositeOperatorFilterArithmetic {
		pixmap1, err := input1.take()
		if err != nil {
			return imageFilter{}, err
		}
		pixmap2, err := input2.take()
		if err != nil {
			return imageFilter{}, err
		}

		arithmetic(
			op.K1,
			op.K2,
			op.K3,
			op.K4,
			asImageRef(pixmap1),
			asImageRef(pixmap2),
			asImageRefMut(pixmap),
		)

		return imageFilterFromImage(pixmap, cs), nil
	}

	paint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(
		0,
		0,
		input2.asRef().AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	var blendMode okage.BlendMode
	switch fe.Operator().Kind {
	case CompositeOperatorFilterOver:
		blendMode = okage.BlendModeSourceOver
	case CompositeOperatorFilterIn:
		blendMode = okage.BlendModeSourceIn
	case CompositeOperatorFilterOut:
		blendMode = okage.BlendModeSourceOut
	case CompositeOperatorFilterAtop:
		blendMode = okage.BlendModeSourceAtop
	case CompositeOperatorFilterXor:
		blendMode = okage.BlendModeXor
	case CompositeOperatorFilterLighter:
		blendMode = okage.BlendModePlus
	case CompositeOperatorFilterArithmetic:
		blendMode = okage.BlendModeSourceOver
	}

	paint1 := okage.DefaultPixmapPaint()
	paint1.BlendMode = blendMode
	pixmap.DrawPixmap(
		0,
		0,
		input1.asRef().AsRef(),
		&paint1,
		path.IdentityTransform(),
		nil,
	)

	return imageFilterFromImage(pixmap, cs), nil
}

// applyMerge is apply_merge in resvg src/filter/mod.rs.
func applyMerge(
	fe *MergeFilter,
	cs ColorInterpolationFilter,
	region path.IntRect,
	source *okage.Pixmap,
	results []filterResult,
	opt *options,
) (imageFilter, error) {
	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	for _, input := range fe.Inputs() {
		input, err := getInput(input, region, source, results, opt)
		if err != nil {
			return imageFilter{}, err
		}
		input, err = input.intoColorSpace(cs)
		if err != nil {
			return imageFilter{}, err
		}
		paint := okage.DefaultPixmapPaint()
		pixmap.DrawPixmap(
			0,
			0,
			input.asRef().AsRef(),
			&paint,
			path.IdentityTransform(),
			nil,
		)
	}

	return imageFilterFromImage(pixmap, cs), nil
}

// applyFlood is apply_flood in resvg src/filter/mod.rs.
func applyFlood(fe *FloodFilter, region path.IntRect, opt *options) (imageFilter, error) {
	c := fe.Color()

	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap.Fill(okage.ColorFromRGBA8(
		c.Red,
		c.Green,
		c.Blue,
		fe.Opacity().ToU8(),
	))

	return imageFilterFromImage(
		pixmap,
		ColorInterpolationFilterSRGB,
	), nil
}

// applyTile is apply_tile in resvg src/filter/mod.rs.
func applyTile(input imageFilter, region path.IntRect, opt *options) (imageFilter, error) {
	subregion, ok := input.region.Translate(-region.X(), -region.Y())
	mustOK(ok)

	tilePixmap, err := pixmapCopyRegion(input.image, subregion)
	if err != nil {
		return imageFilter{}, err
	}
	paint := okage.DefaultPaint()
	paint.Shader = okage.NewPattern(
		tilePixmap.AsRef(),
		okage.SpreadModeRepeat,
		okage.FilterQualityBicubic,
		1.0,
		path.TransformFromTranslate(float32(subregion.X()), float32(subregion.Y())),
	)

	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}
	rect, ok := path.RectFromXYWH(0.0, 0.0, float32(region.Width()), float32(region.Height()))
	mustOK(ok)
	pixmap.FillRect(rect, &paint, path.IdentityTransform(), nil)

	return imageFilterFromImage(
		pixmap,
		ColorInterpolationFilterSRGB,
	), nil
}

// applyImage is apply_image in resvg src/filter/mod.rs.
func applyImage(
	fe *ImageFilter,
	region path.IntRect,
	subregion path.IntRect,
	ts Transform,
	opt *options,
) (imageFilter, error) {
	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	sx, sy := ts.GetScale()
	transform := path.TransformFromRow(
		sx,
		0.0,
		0.0,
		sy,
		float32(subregion.X()),
		float32(subregion.Y()),
	)

	maxBBox, ok := path.IntRectFromXYWH(0, 0, region.Width(), region.Height())
	mustOK(ok)
	ctx := &context{
		maxBBox: maxBBox,
		opt:     opt,
	}

	m := pixmap.AsMut()
	renderNodes(fe.Root(), ctx, transform, &m)

	return imageFilterFromImage(
		pixmap,
		ColorInterpolationFilterSRGB,
	), nil
}

// applyComponentTransfer is apply_component_transfer in resvg
// src/filter/mod.rs.
func applyComponentTransfer(
	fe *ComponentTransferFilter,
	cs ColorInterpolationFilter,
	input imageFilter,
) (imageFilter, error) {
	in, err := input.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap, err := in.take()
	if err != nil {
		return imageFilter{}, err
	}

	demultiplyAlpha(pixmap.DataMut())
	applyFilterComponentTransfer(fe, asImageRefMut(pixmap))
	multiplyAlpha(pixmap.DataMut())

	return imageFilterFromImage(pixmap, cs), nil
}

// applyColorMatrix is apply_color_matrix in resvg src/filter/mod.rs.
func applyColorMatrix(
	fe *ColorMatrixFilter,
	cs ColorInterpolationFilter,
	input imageFilter,
) (imageFilter, error) {
	in, err := input.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap, err := in.take()
	if err != nil {
		return imageFilter{}, err
	}

	demultiplyAlpha(pixmap.DataMut())
	applyFilterColorMatrix(fe.Kind(), asImageRefMut(pixmap))
	multiplyAlpha(pixmap.DataMut())

	return imageFilterFromImage(pixmap, cs), nil
}

// applyConvolveMatrix is apply_convolve_matrix in resvg src/filter/mod.rs.
func applyConvolveMatrix(
	fe *ConvolveMatrixFilter,
	cs ColorInterpolationFilter,
	input imageFilter,
) (imageFilter, error) {
	in, err := input.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap, err := in.take()
	if err != nil {
		return imageFilter{}, err
	}

	if fe.PreserveAlpha() {
		demultiplyAlpha(pixmap.DataMut())
	}

	applyFilterConvolveMatrix(fe, asImageRefMut(pixmap))

	return imageFilterFromImage(pixmap, cs), nil
}

// applyMorphology is apply_morphology in resvg src/filter/mod.rs.
func applyMorphology(
	fe *MorphologyFilter,
	cs ColorInterpolationFilter,
	ts Transform,
	input imageFilter,
) (imageFilter, error) {
	in, err := input.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap, err := in.take()
	if err != nil {
		return imageFilter{}, err
	}

	rx, ry, ok := scaleCoordinates(fe.RadiusX().Get(), fe.RadiusY().Get(), ts)
	if !ok {
		return imageFilterFromImage(pixmap, cs), nil
	}

	if !(rx > 0.0 && ry > 0.0) {
		pixmapClear(pixmap)
		return imageFilterFromImage(pixmap, cs), nil
	}

	applyFilterMorphology(fe.Operator(), rx, ry, asImageRefMut(pixmap))

	return imageFilterFromImage(pixmap, cs), nil
}

// applyDisplacementMap is apply_displacement_map in resvg src/filter/mod.rs.
func applyDisplacementMap(
	fe *DisplacementMapFilter,
	region path.IntRect,
	cs ColorInterpolationFilter,
	ts Transform,
	input1 imageFilter,
	input2 imageFilter,
	opt *options,
) (imageFilter, error) {
	in1, err := input1.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap1, err := in1.take()
	if err != nil {
		return imageFilter{}, err
	}
	in2, err := input2.intoColorSpace(cs)
	if err != nil {
		return imageFilter{}, err
	}
	pixmap2, err := in2.take()
	if err != nil {
		return imageFilter{}, err
	}

	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	sx, sy, ok := scaleCoordinates(fe.Scale(), fe.Scale(), ts)
	if !ok {
		return imageFilterFromImage(pixmap1, cs), nil
	}

	applyFilterDisplacementMap(
		fe,
		sx,
		sy,
		asImageRef(pixmap1),
		asImageRef(pixmap2),
		asImageRefMut(pixmap),
	)

	return imageFilterFromImage(pixmap, cs), nil
}

// applyTurbulence is apply_turbulence in resvg src/filter/mod.rs.
func applyTurbulence(
	fe *TurbulenceFilter,
	region path.IntRect,
	cs ColorInterpolationFilter,
	ts Transform,
	opt *options,
) (imageFilter, error) {
	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	sx, sy := ts.GetScale()
	if ApproxZeroUlps32(sx, 4) || ApproxZeroUlps32(sy, 4) {
		return imageFilterFromImage(pixmap, cs), nil
	}

	applyFilterTurbulence(
		float64(region.X())-float64(ts.TX),
		float64(region.Y())-float64(ts.TY),
		float64(sx),
		float64(sy),
		float64(fe.BaseFrequencyX().Get()),
		float64(fe.BaseFrequencyY().Get()),
		fe.NumOctaves(),
		fe.Seed(),
		fe.StitchTiles(),
		fe.Kind() == TurbulenceKindFilterFractalNoise,
		asImageRefMut(pixmap),
	)

	multiplyAlpha(pixmap.DataMut())

	return imageFilterFromImage(pixmap, cs), nil
}

// applyDiffuseLighting is apply_diffuse_lighting in resvg src/filter/mod.rs.
func applyDiffuseLighting(
	fe *DiffuseLightingFilter,
	region path.IntRect,
	cs ColorInterpolationFilter,
	ts Transform,
	input imageFilter,
	opt *options,
) (imageFilter, error) {
	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	lightSource := transformLightSource(fe.LightSource(), region, ts)

	diffuseLighting(
		fe,
		lightSource,
		asImageRef(input.asRef()),
		asImageRefMut(pixmap),
	)

	return imageFilterFromImage(pixmap, cs), nil
}

// applySpecularLighting is apply_specular_lighting in resvg
// src/filter/mod.rs.
func applySpecularLighting(
	fe *SpecularLightingFilter,
	region path.IntRect,
	cs ColorInterpolationFilter,
	ts Transform,
	input imageFilter,
	opt *options,
) (imageFilter, error) {
	pixmap, err := pixmapTryCreate(region.Width(), region.Height(), opt)
	if err != nil {
		return imageFilter{}, err
	}

	lightSource := transformLightSource(fe.LightSource(), region, ts)

	specularLighting(
		fe,
		lightSource,
		asImageRef(input.asRef()),
		asImageRefMut(pixmap),
	)

	return imageFilterFromImage(pixmap, cs), nil
}

// sqrt2F32 is std::f32::consts::SQRT_2.
const sqrt2F32 = float32(math.Sqrt2)

// transformLightSource is transform_light_source in resvg src/filter/mod.rs.
//
// TODO: do not modify LightSource.
func transformLightSource(
	source LightSourceFilter,
	region path.IntRect,
	ts Transform,
) LightSourceFilter {
	switch light := source.(type) {
	case DistantLightFilter:
	case PointLightFilter:
		point := path.Point{X: light.X, Y: light.Y}
		ts.MapPoint(&point)
		light.X = point.X - float32(region.X())
		light.Y = point.Y - float32(region.Y())
		light.Z = light.Z * sqrtF32(float32(ts.SX*ts.SX)+float32(ts.SY*ts.SY)) / sqrt2F32
		source = light
	case SpotLightFilter:
		sz := sqrtF32(float32(ts.SX*ts.SX)+float32(ts.SY*ts.SY)) / sqrt2F32

		point := path.Point{X: light.X, Y: light.Y}
		ts.MapPoint(&point)
		light.X = point.X - float32(region.X())
		light.Y = point.Y - float32(region.Y())
		light.Z *= sz

		point = path.Point{X: light.PointsAtX, Y: light.PointsAtY}
		ts.MapPoint(&point)
		light.PointsAtX = point.X - float32(region.X())
		light.PointsAtY = point.Y - float32(region.Y())
		light.PointsAtZ *= sz
		source = light
	}

	return source
}

// applyToCanvas is apply_to_canvas in resvg src/filter/mod.rs.
func applyToCanvas(
	input imageFilter,
	region path.IntRect,
	pixmap *okage.Pixmap,
) error {
	input, err := input.intoColorSpace(ColorInterpolationFilterSRGB)
	if err != nil {
		return err
	}

	pixmap.Fill(okage.ColorTransparent)
	paint := okage.DefaultPixmapPaint()
	pixmap.DrawPixmap(
		region.X(),
		region.Y(),
		input.asRef().AsRef(),
		&paint,
		path.IdentityTransform(),
		nil,
	)

	return nil
}

// resolveStdDev is resolve_std_dev in resvg src/filter/mod.rs. It calculates
// Gaussian blur sigmas for the current world transform.
//
// If the last flag is set, then a box blur should be used. Or IIR otherwise.
//
// Port: the last result is false where upstream returns None.
func resolveStdDev(stdDx, stdDy float32, ts Transform) (float64, float64, bool, bool) {
	stdDx, stdDy, ok := scaleCoordinates(stdDx, stdDy, ts)
	if !ok {
		return 0, 0, false, false
	}

	// 'A negative value or a value of zero disables the effect of the given filter primitive
	// (i.e., the result is the filter input image).'
	if ApproxEqUlps32(stdDx, 0.0, 4) && ApproxEqUlps32(stdDy, 0.0, 4) {
		return 0, 0, false, false
	}

	// Ignore tiny sigmas. In case of IIR blur it can lead to a transparent image.
	if stdDx < 0.05 {
		stdDx = 0.0
	}

	if stdDy < 0.05 {
		stdDy = 0.0
	}

	const blurSigmaThreshold float32 = 2.0
	// Check that the current feGaussianBlur filter can be applied using a box blur.
	boxBlur := stdDx >= blurSigmaThreshold || stdDy >= blurSigmaThreshold

	return float64(stdDx), float64(stdDy), boxBlur, true
}

// scaleCoordinates is scale_coordinates in resvg src/filter/mod.rs.
func scaleCoordinates(x, y float32, ts Transform) (float32, float32, bool) {
	sx, sy := ts.GetScale()
	return x * sx, y * sy, true
}

// sqrtF32 is f32::sqrt. The square root in float64, rounded to float32, is
// the correctly rounded square root in float32.
func sqrtF32(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
