package lottie

// This file ports no file of velato. It holds the detection of a Lottie
// document (D44).

import (
	"bytes"
	"encoding/json"
	"slices"
)

// markers are the top-level keys that identify a Lottie document: the frame
// rate, the in point, the out point, the width and the height.
var markers = [...]string{"fr", "ip", "op", "w", "h"}

// allMarkers is the set of seen markers when each marker was seen.
const allMarkers = 1<<len(markers) - 1

// Detect reports whether head is the start of a Lottie document. head can be
// cut short at any byte. Detect reads only the top level of the object, and
// returns true when each of the keys fr, ip, op, w and h holds a number
// there. No one of these keys is rare, but a JSON document that has all five
// at its top level, each with a number, is a Lottie document.
//
// A writer such as bodymovin puts these keys before the assets and the layers.
// In the 879 Noto animated emoji, the last of them ends at byte 52 or before.
// So a few kilobytes of the start of a file are enough.
func Detect(head []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(head))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return false
	}
	var seen int
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return false
		}
		key, ok := t.(string)
		if !ok {
			return false
		}
		i := slices.Index(markers[:], key)
		if i == -1 {
			if err := skipValue(dec); err != nil {
				return false
			}
			continue
		}
		// A key of another format can have the same name, so the value
		// must be a number.
		if t, err = dec.Token(); err != nil {
			return false
		}
		if _, ok := t.(float64); !ok {
			return false
		}
		if seen |= 1 << i; seen == allMarkers {
			return true
		}
	}
	return false
}

// skipValue reads past the next value of dec, with the objects and the arrays
// in it.
func skipValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if _, ok := t.(json.Delim); !ok {
		return nil
	}
	for depth := 1; depth > 0; {
		switch t, err := dec.Token(); {
		case err != nil:
			return err
		case t == json.Delim('{'), t == json.Delim('['):
			depth++
		case t == json.Delim('}'), t == json.Delim(']'):
			depth--
		}
	}
	return nil
}
