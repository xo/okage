// Copyright 2026 Kenneth Shaw
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package filter_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/xo/okage"
	"github.com/xo/okage/filter"
	"github.com/xo/okage/path"
	"github.com/xo/okage/svg"
)

// renderSVG renders the SVG file of 40 by 30 pixels that draws shapes with
// the filter primitive fe, or with no filter when fe is empty.
func renderSVG(t *testing.T, fe, colorSpace string) *okage.Pixmap {
	t.Helper()
	attr := ""
	if fe != "" {
		attr = ` filter="url(#f)"`
	}
	src := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30">
<filter id="f" filterUnits="userSpaceOnUse" x="0" y="0" width="40" height="30" color-interpolation-filters="%s">%s</filter>
<g%s>
<rect x="6" y="5" width="20" height="12" fill="rgb(200,100,50)" fill-opacity="0.6"/>
<circle cx="25" cy="18" r="8" fill="rgb(20,180,240)"/>
</g>
</svg>`, colorSpace, fe, attr)
	tree, err := svg.FromData([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	p, ok := okage.NewPixmap(40, 30)
	if !ok {
		t.Fatal("pixmap")
	}
	m := p.AsMut()
	svg.Render(tree, path.IdentityTransform(), &m)
	return p
}

// TestSameAsSVG makes sure that each function gives the bytes of the same
// filter primitive in an SVG file that svg renders.
func TestSameAsSVG(t *testing.T) {
	tests := []struct {
		fe    string
		apply func(p *okage.Pixmap, opts ...filter.Option)
	}{
		{`<feGaussianBlur stdDeviation="3 1.5"/>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.Blur(p, 3, 1.5, opts...)
		}},
		{`<feGaussianBlur stdDeviation="0.8"/>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.Blur(p, 0.8, 0.8, opts...)
		}},
		{`<feColorMatrix type="matrix" values="0.2 0.3 0.5 0 0.1  0 1 0 0 0  0.7 0 0.1 0 -0.1  0 0 0 0.8 0"/>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.ApplyColorMatrix(p, filter.NewColorMatrix([20]float32{0.2, 0.3, 0.5, 0, 0.1, 0, 1, 0, 0, 0, 0.7, 0, 0.1, 0, -0.1, 0, 0, 0, 0.8, 0}), opts...)
		}},
		{`<feColorMatrix type="saturate" values="0.3"/>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.ApplyColorMatrix(p, filter.SaturateMatrix(0.3), opts...)
		}},
		{`<feColorMatrix type="hueRotate" values="73"/>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.ApplyColorMatrix(p, filter.HueRotateMatrix(73), opts...)
		}},
		{`<feColorMatrix type="luminanceToAlpha"/>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.ApplyColorMatrix(p, filter.LuminanceToAlphaMatrix(), opts...)
		}},
		{`<feComponentTransfer>
<feFuncR type="table" tableValues="0 0.8 0.2"/>
<feFuncG type="discrete" tableValues="1 0 0.5"/>
<feFuncB type="linear" slope="1.5" intercept="-0.2"/>
<feFuncA type="gamma" amplitude="2" exponent="0.5" offset="0.1"/>
</feComponentTransfer>`, func(p *okage.Pixmap, opts ...filter.Option) {
			filter.ApplyComponentTransfer(p,
				filter.TransferTable([]float32{0, 0.8, 0.2}),
				filter.TransferDiscrete([]float32{1, 0, 0.5}),
				filter.TransferLinear(1.5, -0.2),
				filter.TransferGamma(2, 0.5, 0.1),
				opts...)
		}},
	}
	for i, tt := range tests {
		for _, cs := range []string{"sRGB", "linearRGB"} {
			want := renderSVG(t, tt.fe, cs)
			got := renderSVG(t, "", cs)
			var opts []filter.Option
			if cs == "linearRGB" {
				opts = append(opts, filter.WithLinearRGB())
			}
			tt.apply(got, opts...)
			if !bytes.Equal(got.Data(), want.Data()) {
				t.Errorf("test %d in %s: the bytes differ", i, cs)
			}
		}
	}
}
