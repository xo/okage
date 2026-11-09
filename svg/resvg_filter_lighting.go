// Copyright 2020 the Resvg Authors
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package svg

import (
	"github.com/xo/okage/internal/crmath"
	"github.com/xo/okage/internal/wide"
)

// This file ports resvg src/filter/lighting.rs.
//
// Port: the function apply of upstream takes the name of its module path,
// applyFilterLighting (D10). The operators of Vector2 and Vector3 are the
// methods mul, add and sub. Each sum of products converts each product to
// float32, so that the Go compiler cannot fuse a multiply and an add (D16).

// The factors of the normals in resvg src/filter/lighting.rs.
const (
	factor12 float32 = 1.0 / 2.0
	factor13 float32 = 1.0 / 3.0
	factor14 float32 = 1.0 / 4.0
	factor23 float32 = 2.0 / 3.0
)

// vector2 is Vector2 in resvg src/filter/lighting.rs.
type vector2 struct {
	x float32
	y float32
}

// newVector2 is Vector2::new in resvg src/filter/lighting.rs.
func newVector2(x, y float32) vector2 {
	return vector2{x: x, y: y}
}

// approxZero is Vector2::approx_zero in resvg src/filter/lighting.rs.
func (v vector2) approxZero() bool {
	return ApproxZeroUlps32(v.x, 4) && ApproxZeroUlps32(v.y, 4)
}

// mul is the impl Mul<f32> for Vector2 in resvg src/filter/lighting.rs.
func (v vector2) mul(c float32) vector2 {
	return vector2{
		x: v.x * c,
		y: v.y * c,
	}
}

// vector3 is Vector3 in resvg src/filter/lighting.rs.
type vector3 struct {
	x float32
	y float32
	z float32
}

// newVector3 is Vector3::new in resvg src/filter/lighting.rs.
func newVector3(x, y, z float32) vector3 {
	return vector3{x: x, y: y, z: z}
}

// dot is Vector3::dot in resvg src/filter/lighting.rs.
func (v vector3) dot(other vector3) float32 {
	return float32(v.x*other.x) + float32(v.y*other.y) + float32(v.z*other.z)
}

// length is Vector3::length in resvg src/filter/lighting.rs.
func (v vector3) length() float32 {
	return sqrtF32(float32(v.x*v.x) + float32(v.y*v.y) + float32(v.z*v.z))
}

// normalized is Vector3::normalized in resvg src/filter/lighting.rs.
func (v vector3) normalized() (vector3, bool) {
	length := v.length()
	if !ApproxZeroUlps32(length, 4) {
		return vector3{
			x: v.x / length,
			y: v.y / length,
			z: v.z / length,
		}, true
	}
	return vector3{}, false
}

// add is the impl Add<Vector3> for Vector3 in resvg src/filter/lighting.rs.
func (v vector3) add(rhs vector3) vector3 {
	return vector3{
		x: v.x + rhs.x,
		y: v.y + rhs.y,
		z: v.z + rhs.z,
	}
}

// sub is the impl Sub<Vector3> for Vector3 in resvg src/filter/lighting.rs.
func (v vector3) sub(rhs vector3) vector3 {
	return vector3{
		x: v.x - rhs.x,
		y: v.y - rhs.y,
		z: v.z - rhs.z,
	}
}

// normal is Normal in resvg src/filter/lighting.rs.
type normal struct {
	factor vector2
	normal vector2
}

// newNormal is Normal::new in resvg src/filter/lighting.rs.
func newNormal(factorX, factorY float32, nx, ny int16) normal {
	return normal{
		factor: newVector2(factorX, factorY),
		normal: newVector2(float32(-nx), float32(-ny)),
	}
}

// diffuseLighting is diffuse_lighting in resvg src/filter/lighting.rs. It
// renders a diffuse lighting.
//
//   - `src` pixels can have any alpha method, since only the alpha channel is used.
//   - `dest` will have an **unpremultiplied alpha**.
//
// Does nothing when `src` is less than 3x3.
//
// `src` and `dest` dimensions must match.
func diffuseLighting(
	fe *DiffuseLightingFilter,
	lightSource LightSourceFilter,
	src imageRef,
	dest imageRefMut,
) {
	lightFactor := func(nrm normal, lightVector vector3) float32 {
		var k float32
		if nrm.normal.approxZero() {
			k = lightVector.z
		} else {
			n := nrm.normal.mul(fe.SurfaceScale() / 255.0)
			n.x *= nrm.factor.x
			n.y *= nrm.factor.y

			normal := newVector3(n.x, n.y, 1.0)

			k = normal.dot(lightVector) / normal.length()
		}

		return fe.DiffuseConstant() * k
	}

	applyFilterLighting(
		lightSource,
		fe.SurfaceScale(),
		fe.LightingColor(),
		lightFactor,
		calcDiffuseAlpha,
		src,
		dest,
	)
}

// specularLighting is specular_lighting in resvg src/filter/lighting.rs. It
// renders a specular lighting.
//
//   - `src` pixels can have any alpha method, since only the alpha channel is used.
//   - `dest` will have a **premultiplied alpha**.
//
// Does nothing when `src` is less than 3x3.
//
// `src` and `dest` dimensions must match.
func specularLighting(
	fe *SpecularLightingFilter,
	lightSource LightSourceFilter,
	src imageRef,
	dest imageRefMut,
) {
	lightFactor := func(nrm normal, lightVector vector3) float32 {
		h := lightVector.add(newVector3(0.0, 0.0, 1.0))
		hLength := h.length()

		if ApproxZeroUlps32(hLength, 4) {
			return 0.0
		}

		var k float32
		if nrm.normal.approxZero() {
			nDotH := h.z / hLength
			if ApproxEqUlps32(fe.SpecularExponent(), 1.0, 4) {
				k = nDotH
			} else {
				k = crmath.Pow32(nDotH, fe.SpecularExponent())
			}
		} else {
			n := nrm.normal.mul(fe.SurfaceScale() / 255.0)
			n.x *= nrm.factor.x
			n.y *= nrm.factor.y

			normal := newVector3(n.x, n.y, 1.0)

			nDotH := normal.dot(h) / normal.length() / hLength
			if ApproxEqUlps32(fe.SpecularExponent(), 1.0, 4) {
				k = nDotH
			} else {
				k = crmath.Pow32(nDotH, fe.SpecularExponent())
			}
		}

		return fe.SpecularConstant() * k
	}

	applyFilterLighting(
		lightSource,
		fe.SurfaceScale(),
		fe.LightingColor(),
		lightFactor,
		calcSpecularAlpha,
		src,
		dest,
	)
}

// applyFilterLighting is apply in resvg src/filter/lighting.rs.
func applyFilterLighting(
	lightSource LightSourceFilter,
	surfaceScale float32,
	lightingColor Color,
	lightFactor func(normal, vector3) float32,
	calcAlpha func(uint8, uint8, uint8) uint8,
	src imageRef,
	dest imageRefMut,
) {
	if src.width < 3 || src.height < 3 {
		return
	}

	width := src.width
	height := src.height

	// `feDistantLight` has a fixed vector, so calculate it beforehand.
	var lightVector vector3
	if light, ok := lightSource.(DistantLightFilter); ok {
		azimuth := toRadians(light.Azimuth)
		elevation := toRadians(light.Elevation)
		lightVector = newVector3(
			crmath.Cos32(azimuth)*crmath.Cos32(elevation),
			crmath.Sin32(azimuth)*crmath.Cos32(elevation),
			crmath.Sin32(elevation),
		)
	} else {
		lightVector = newVector3(1.0, 1.0, 1.0)
	}

	calc := func(nx, ny uint32, nrm normal) {
		switch light := lightSource.(type) {
		case DistantLightFilter:
		case PointLightFilter:
			nz := float32(src.alphaAt(nx, ny)) / 255.0 * surfaceScale
			origin := newVector3(light.X, light.Y, light.Z)
			v := origin.sub(newVector3(float32(nx), float32(ny), nz))
			if n, ok := v.normalized(); ok {
				lightVector = n
			} else {
				lightVector = v
			}
		case SpotLightFilter:
			nz := float32(src.alphaAt(nx, ny)) / 255.0 * surfaceScale
			origin := newVector3(light.X, light.Y, light.Z)
			v := origin.sub(newVector3(float32(nx), float32(ny), nz))
			if n, ok := v.normalized(); ok {
				lightVector = n
			} else {
				lightVector = v
			}
		}

		lightColor := lightColor(lightSource, lightingColor, lightVector)
		factor := lightFactor(nrm, lightVector)

		compute := func(x uint8) uint8 {
			return wide.TruncU8(f32BoundFilter(0.0, float32(float32(x)*factor), 255.0) + 0.5)
		}

		r := compute(lightColor.Red)
		g := compute(lightColor.Green)
		b := compute(lightColor.Blue)
		a := calcAlpha(r, g, b)

		dest.setPixelAt(nx, ny, rgba8{b: b, g: g, r: r, a: a})
	}

	calc(0, 0, topLeftNormal(src))
	calc(width-1, 0, topRightNormal(src))
	calc(0, height-1, bottomLeftNormal(src))
	calc(width-1, height-1, bottomRightNormal(src))

	for x := uint32(1); x < width-1; x++ {
		calc(x, 0, topRowNormal(src, x))
		calc(x, height-1, bottomRowNormal(src, x))
	}

	for y := uint32(1); y < height-1; y++ {
		calc(0, y, leftColumnNormal(src, y))
		calc(width-1, y, rightColumnNormal(src, y))
	}

	for y := uint32(1); y < height-1; y++ {
		for x := uint32(1); x < width-1; x++ {
			calc(x, y, interiorNormal(src, x, y))
		}
	}
}

// lightColor is light_color in resvg src/filter/lighting.rs.
func lightColor(light LightSourceFilter, lightingColor Color, lightVector vector3) Color {
	switch light := light.(type) {
	case DistantLightFilter, PointLightFilter:
		return lightingColor
	case SpotLightFilter:
		origin := newVector3(light.X, light.Y, light.Z)
		direction := newVector3(light.PointsAtX, light.PointsAtY, light.PointsAtZ)
		direction = direction.sub(origin)
		if n, ok := direction.normalized(); ok {
			direction = n
		}
		minusLDotS := -lightVector.dot(direction)
		if minusLDotS <= 0.0 {
			return ColorBlack()
		}

		if light.LimitingConeAngle != nil {
			limitingConeAngle := *light.LimitingConeAngle
			if minusLDotS < crmath.Cos32(toRadians(limitingConeAngle)) {
				return ColorBlack()
			}
		}

		factor := crmath.Pow32(minusLDotS, light.SpecularExponent.Get())
		compute := func(x uint8) uint8 {
			return wide.TruncU8(f32BoundFilter(0.0, float32(float32(x)*factor), 255.0) + 0.5)
		}

		return NewColorRGB(
			compute(lightingColor.Red),
			compute(lightingColor.Green),
			compute(lightingColor.Blue),
		)
	}
	return lightingColor
}

// topLeftNormal is top_left_normal in resvg src/filter/lighting.rs.
func topLeftNormal(img imageRef) normal {
	center := img.alphaAt(0, 0)
	right := img.alphaAt(1, 0)
	bottom := img.alphaAt(0, 1)
	bottomRight := img.alphaAt(1, 1)

	return newNormal(
		factor23,
		factor23,
		-2*center+2*right-bottom+bottomRight,
		-2*center-right+2*bottom+bottomRight,
	)
}

// topRightNormal is top_right_normal in resvg src/filter/lighting.rs.
func topRightNormal(img imageRef) normal {
	left := img.alphaAt(img.width-2, 0)
	center := img.alphaAt(img.width-1, 0)
	bottomLeft := img.alphaAt(img.width-2, 1)
	bottom := img.alphaAt(img.width-1, 1)

	return newNormal(
		factor23,
		factor23,
		-2*left+2*center-bottomLeft+bottom,
		-left-2*center+bottomLeft+2*bottom,
	)
}

// bottomLeftNormal is bottom_left_normal in resvg src/filter/lighting.rs.
func bottomLeftNormal(img imageRef) normal {
	top := img.alphaAt(0, img.height-2)
	topRight := img.alphaAt(1, img.height-2)
	center := img.alphaAt(0, img.height-1)
	right := img.alphaAt(1, img.height-1)

	return newNormal(
		factor23,
		factor23,
		-top+topRight-2*center+2*right,
		-2*top-topRight+2*center+right,
	)
}

// bottomRightNormal is bottom_right_normal in resvg src/filter/lighting.rs.
func bottomRightNormal(img imageRef) normal {
	topLeft := img.alphaAt(img.width-2, img.height-2)
	top := img.alphaAt(img.width-1, img.height-2)
	left := img.alphaAt(img.width-2, img.height-1)
	center := img.alphaAt(img.width-1, img.height-1)

	return newNormal(
		factor23,
		factor23,
		-topLeft+top-2*left+2*center,
		-topLeft-2*top+left+2*center,
	)
}

// topRowNormal is top_row_normal in resvg src/filter/lighting.rs.
func topRowNormal(img imageRef, x uint32) normal {
	left := img.alphaAt(x-1, 0)
	center := img.alphaAt(x, 0)
	right := img.alphaAt(x+1, 0)
	bottomLeft := img.alphaAt(x-1, 1)
	bottom := img.alphaAt(x, 1)
	bottomRight := img.alphaAt(x+1, 1)

	return newNormal(
		factor13,
		factor12,
		-2*left+2*right-bottomLeft+bottomRight,
		-left-2*center-right+bottomLeft+2*bottom+bottomRight,
	)
}

// bottomRowNormal is bottom_row_normal in resvg src/filter/lighting.rs.
func bottomRowNormal(img imageRef, x uint32) normal {
	topLeft := img.alphaAt(x-1, img.height-2)
	top := img.alphaAt(x, img.height-2)
	topRight := img.alphaAt(x+1, img.height-2)
	left := img.alphaAt(x-1, img.height-1)
	center := img.alphaAt(x, img.height-1)
	right := img.alphaAt(x+1, img.height-1)

	return newNormal(
		factor13,
		factor12,
		-topLeft+topRight-2*left+2*right,
		-topLeft-2*top-topRight+left+2*center+right,
	)
}

// leftColumnNormal is left_column_normal in resvg src/filter/lighting.rs.
func leftColumnNormal(img imageRef, y uint32) normal {
	top := img.alphaAt(0, y-1)
	topRight := img.alphaAt(1, y-1)
	center := img.alphaAt(0, y)
	right := img.alphaAt(1, y)
	bottom := img.alphaAt(0, y+1)
	bottomRight := img.alphaAt(1, y+1)

	return newNormal(
		factor12,
		factor13,
		-top+topRight-2*center+2*right-bottom+bottomRight,
		-2*top-topRight+2*bottom+bottomRight,
	)
}

// rightColumnNormal is right_column_normal in resvg src/filter/lighting.rs.
func rightColumnNormal(img imageRef, y uint32) normal {
	topLeft := img.alphaAt(img.width-2, y-1)
	top := img.alphaAt(img.width-1, y-1)
	left := img.alphaAt(img.width-2, y)
	center := img.alphaAt(img.width-1, y)
	bottomLeft := img.alphaAt(img.width-2, y+1)
	bottom := img.alphaAt(img.width-1, y+1)

	return newNormal(
		factor12,
		factor13,
		-topLeft+top-2*left+2*center-bottomLeft+bottom,
		-topLeft-2*top+bottomLeft+2*bottom,
	)
}

// interiorNormal is interior_normal in resvg src/filter/lighting.rs.
func interiorNormal(img imageRef, x, y uint32) normal {
	topLeft := img.alphaAt(x-1, y-1)
	top := img.alphaAt(x, y-1)
	topRight := img.alphaAt(x+1, y-1)
	left := img.alphaAt(x-1, y)
	right := img.alphaAt(x+1, y)
	bottomLeft := img.alphaAt(x-1, y+1)
	bottom := img.alphaAt(x, y+1)
	bottomRight := img.alphaAt(x+1, y+1)

	return newNormal(
		factor14,
		factor14,
		-topLeft+topRight-2*left+2*right-bottomLeft+bottomRight,
		-topLeft-2*top-topRight+bottomLeft+2*bottom+bottomRight,
	)
}

// calcDiffuseAlpha is calc_diffuse_alpha in resvg src/filter/lighting.rs.
func calcDiffuseAlpha(_, _, _ uint8) uint8 {
	return 255
}

// calcSpecularAlpha is calc_specular_alpha in resvg src/filter/lighting.rs.
func calcSpecularAlpha(r, g, b uint8) uint8 {
	return max(max(r, g), b)
}
