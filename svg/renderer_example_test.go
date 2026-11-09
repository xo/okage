package svg_test

import (
	"fmt"
	"image/color"

	"github.com/xo/okage/svg"
)

// ExampleRenderer renders SVG data to a width of 50 pixels, on a white
// background.
func ExampleRenderer() {
	data := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="40">
  <rect x="10" y="10" width="80" height="20" fill="teal"/>
</svg>`)
	r := svg.New(
		svg.WithLoadSystemFonts(false),
		svg.WithBackground(color.White),
		svg.WithWidth(50),
		svg.WithScaleMode(svg.ScaleBestFit),
	)
	img, err := r.Render(data)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(img.Bounds().Dx(), img.Bounds().Dy())
	fmt.Println(img.RGBAAt(0, 0), img.RGBAAt(25, 10))
	// Output:
	// 50 20
	// {255 255 255 255} {0 128 128 255}
}
