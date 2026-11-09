// Copyright (c) 2019 Reizner Evgeniy
//
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package xmlwriter_test

import (
	"fmt"

	"github.com/xo/okage/internal/xmlwriter"
)

// This example ports the doc test of the crate in xmlwriter src/lib.rs.
func Example() {
	w := xmlwriter.New(xmlwriter.WithUseSingleQuote(true))
	w.StartElement("svg")
	w.WriteAttribute("xmlns", "http://www.w3.org/2000/svg")
	w.WriteAttributeFmt("viewBox", "%d %d %d %d", 0, 0, 128, 128)
	w.StartElement("text")
	// We can write any value that the package fmt formats.
	w.WriteAttribute("x", 10)
	w.WriteAttribute("y", 20)
	w.WriteTextFmt("length is %d", 5)

	fmt.Print(w.EndDocument())
	// Output:
	// <svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 128 128'>
	//     <text x='10' y='20'>
	//         length is 5
	//     </text>
	// </svg>
}

// This example ports the doc test of XmlWriter::write_attribute in xmlwriter
// src/lib.rs.
func ExampleXMLWriter_WriteAttribute() {
	w := xmlwriter.New()
	w.StartElement("svg")
	w.WriteAttribute("x", "5")
	w.WriteAttribute("y", 5)
	fmt.Print(w.EndDocument())
	// Output:
	// <svg x="5" y="5"/>
}

// This example ports the doc test of XmlWriter::write_attribute_fmt in
// xmlwriter src/lib.rs.
func ExampleXMLWriter_WriteAttributeFmt() {
	w := xmlwriter.New()
	w.StartElement("rect")
	w.WriteAttributeFmt("fill", "url(#%s)", "gradient")
	fmt.Print(w.EndDocument())
	// Output:
	// <rect fill="url(#gradient)"/>
}

// This example ports the doc test of XmlWriter::write_attribute_raw in
// xmlwriter src/lib.rs.
func ExampleXMLWriter_WriteAttributeRaw() {
	w := xmlwriter.New()
	w.StartElement("path")
	w.WriteAttributeRaw("d", func(buf []byte) []byte { return append(buf, "M 10 20 L 30 40"...) })
	fmt.Print(w.EndDocument())
	// Output:
	// <path d="M 10 20 L 30 40"/>
}

// This example ports the doc test of XmlWriter::set_preserve_whitespaces in
// xmlwriter src/lib.rs.
func ExampleXMLWriter_SetPreserveWhitespaces() {
	w := xmlwriter.New()
	w.StartElement("html")
	w.StartElement("p")
	w.WriteText("text")
	w.EndElement()
	w.StartElement("p")
	w.SetPreserveWhitespaces(true)
	w.WriteText("text")
	w.EndElement()
	w.SetPreserveWhitespaces(false)
	fmt.Print(w.EndDocument())
	// Output:
	// <html>
	//     <p>
	//         text
	//     </p>
	//     <p>text</p>
	// </html>
}

// This example ports the doc test of XmlWriter::end_document in xmlwriter
// src/lib.rs.
func ExampleXMLWriter_EndDocument() {
	w := xmlwriter.New()
	w.StartElement("svg")
	w.StartElement("g")
	w.StartElement("rect")
	fmt.Print(w.EndDocument())
	// Output:
	// <svg>
	//     <g>
	//         <rect/>
	//     </g>
	// </svg>
}
