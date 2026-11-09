package okage

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/xo/okage/path"
)

// The tests of this file port no upstream test. Upstream has no unit test
// for the scan converters. The expected output came from tiny-skia 0.12.0,
// built without default features and in release mode, with a blitter that
// writes each call as a line of text. The scan converters of okage must
// write the same lines.

// recordingBlitter is a blitter that writes each call as a line of text.
type recordingBlitter struct {
	sb strings.Builder
}

func (r *recordingBlitter) blitH(x, y uint32, w lengthU32) {
	fmt.Fprintf(&r.sb, "h %d %d %d\n", x, y, w)
}

func (r *recordingBlitter) blitAntiH(x, y uint32, aa []uint8, runs []alphaRun) {
	fmt.Fprintf(&r.sb, "ah %d %d", x, y)
	i := 0
	for runs[i] != 0 {
		fmt.Fprintf(&r.sb, " %d:%d", runs[i], aa[i])
		i += int(runs[i])
	}
	r.sb.WriteString("\n")
}

func (r *recordingBlitter) blitV(x, y uint32, h lengthU32, a uint8) {
	fmt.Fprintf(&r.sb, "v %d %d %d %d\n", x, y, h, a)
}

func (r *recordingBlitter) blitAntiH2(x, y uint32, a0, a1 uint8) {
	fmt.Fprintf(&r.sb, "ah2 %d %d %d %d\n", x, y, a0, a1)
}

func (r *recordingBlitter) blitAntiV2(x, y uint32, a0, a1 uint8) {
	fmt.Fprintf(&r.sb, "av2 %d %d %d %d\n", x, y, a0, a1)
}

func (r *recordingBlitter) blitRect(rc *screenIntRect) {
	fmt.Fprintf(&r.sb, "r %d %d %d %d\n", rc.X(), rc.Y(), rc.Width(), rc.Height())
}

func (r *recordingBlitter) blitMask(_ *maskBlitter, _ *screenIntRect) {
	r.sb.WriteString("m\n")
}

// f32FromHex returns the float32 with the bits of the hexadecimal string s.
func f32FromHex(t *testing.T, s string) float32 {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	return math.Float32frombits(uint32(v))
}

// buildTestPath builds a path from its description. M, L, Q and C are the
// commands of a path builder, V is a vertical line to the given y, and Z
// closes the contour. Each number is the bits of a float32 in hexadecimal.
func buildTestPath(t *testing.T, desc string) *Path {
	t.Helper()
	f := strings.Fields(desc)
	h := func(i int) float32 { return f32FromHex(t, f[i]) }
	pb := path.NewPathBuilder()
	for i := 0; i < len(f); {
		switch f[i] {
		case "M":
			pb.MoveTo(h(i+1), h(i+2))
			i += 3
		case "L":
			pb.LineTo(h(i+1), h(i+2))
			i += 3
		case "Q":
			pb.QuadTo(h(i+1), h(i+2), h(i+3), h(i+4))
			i += 5
		case "C":
			pb.CubicTo(h(i+1), h(i+2), h(i+3), h(i+4), h(i+5), h(i+6))
			i += 7
		case "V":
			lp, _ := pb.LastPoint()
			pb.LineTo(lp.X, h(i+1))
			i += 2
		case "Z":
			pb.Close()
			i++
		default:
			t.Fatalf("bad command %q", f[i])
		}
	}
	p, ok := pb.Finish()
	if !ok {
		t.Fatal("the path is empty")
	}
	return p
}

// scanCases holds the cases. A case starts with a line that has the kind of
// scan conversion, the fill rule, the line cap and the clip, then the path
// after "|". The lines that follow, up to END, are the expected calls.
const scanCases = `
CASE 5 0 2 0 0 5000 3000 | M c1b32bce 41d54154 L 42fae248 c1f822ac
r 0 0 125 26
v 125 0 26 113
ah 0 26 100:168
ah 100 26 25:168
v 125 26 1 74
END
CASE 0 1 2 0 0 5000 3000 | M 43607462 40530de0 Q 433a9617 4117e868 43633a6a c09988f0 Z
h 213 0 12
h 211 1 14
h 209 2 16
h 207 3 16
h 206 4 10
END
CASE 1 1 0 0 0 5000 3000 | M 43607462 40530de0 Q 433a9617 4117e868 43633a6a c09988f0 Z
ah 186 0 26:0 1:48 1:159 11:255 1:96 2:0
ah 186 1 24:0 1:95 1:191 13:255 1:16 2:0
ah 186 2 22:0 1:95 1:223 14:255 1:176 3:0
ah 186 3 20:0 1:16 1:191 12:255 1:240 1:192 1:144 1:128 1:32 3:0
ah 186 4 20:0 1:144 6:255 2:192 1:144 1:128 1:80 1:64 9:0
ah 186 5 22:0 1:48 1:48 18:0
END
CASE 3 0 1 0 0 5000 3000 | M 42f4845e 433d6a3c V 4318231d Z
v 122 151 1 51
v 121 151 1 16
v 122 152 37 192
v 121 152 37 63
v 122 189 1 153
v 121 189 1 50
v 122 152 1 168
v 121 152 1 55
v 122 153 36 192
v 121 153 36 63
v 122 189 1 153
v 121 189 1 50
END
CASE 2 0 1 13 7 150 170 | M 42090d54 435df74a L c0b7b648 435b7f92 V 42de66ee Z
h 13 163 1
h 13 164 1
h 14 165 1
h 14 166 1
h 14 167 1
h 15 168 1
h 15 169 1
h 15 170 1
h 16 171 1
h 16 172 1
h 16 173 1
h 17 174 1
h 17 175 1
h 18 176 1
END
CASE 1 1 2 0 0 5000 3000 | M 43247d2b 430db494 C 431c14bc c1deccc7 425cddac 436f298e 42359ac6 42be5dea Z
ah 45 83 92:0 1:16 1:95 1:143 2:191 1:255 2:191 1:127 1:48 18:0
ah 45 84 90:0 1:95 1:175 10:255 1:191 1:64 16:0
ah 45 85 87:0 1:16 1:127 1:223 14:255 1:143 1:16 14:0
ah 45 86 86:0 1:95 1:223 17:255 1:207 1:16 13:0
ah 45 87 84:0 1:48 1:207 20:255 1:207 1:16 12:0
ah 45 88 83:0 1:143 23:255 1:175 12:0
ah 45 89 81:0 1:48 1:207 25:255 1:128 11:0
ah 45 90 80:0 1:96 28:255 1:32 10:0
ah 45 91 78:0 1:16 1:159 29:255 1:191 10:0
ah 45 92 77:0 1:32 1:207 31:255 1:80 9:0
ah 45 93 76:0 1:64 1:239 32:255 1:223 9:0
ah 45 94 75:0 1:96 35:255 1:64 8:0
ah 45 95 1:96 1:95 1:16 71:0 1:143 36:255 1:191 8:0
ah 45 96 1:128 1:255 1:239 1:159 1:48 68:0 1:159 38:255 1:32 7:0
ah 45 97 1:128 4:255 1:207 1:111 1:16 63:0 1:16 1:191 39:255 1:144 7:0
ah 45 98 1:64 7:255 1:159 1:64 60:0 1:16 1:207 40:255 1:223 7:0
ah 45 99 1:64 9:255 1:223 1:127 1:32 56:0 1:16 1:207 42:255 1:80 6:0
ah 45 100 1:64 12:255 1:175 1:79 53:0 1:48 1:223 43:255 1:144 6:0
ah 45 101 1:0 14:255 1:223 1:143 1:32 49:0 1:48 1:239 44:255 1:223 6:0
ah 45 102 1:0 17:255 1:191 1:95 46:0 1:48 1:239 46:255 1:48 5:0
ah 45 103 1:0 1:224 18:255 1:239 1:143 1:48 42:0 1:48 1:239 47:255 1:112 5:0
ah 45 104 1:0 1:192 21:255 1:207 1:95 1:16 38:0 1:48 1:239 48:255 1:192 5:0
ah 45 105 1:0 1:176 23:255 1:239 1:159 1:64 35:0 1:48 1:239 50:255 5:0
ah 45 106 1:0 1:128 26:255 1:223 1:111 1:16 31:0 1:48 1:239 51:255 1:48 4:0
ah 45 107 1:0 1:112 29:255 1:159 1:79 28:0 1:48 1:239 52:255 1:112 4:0
ah 45 108 1:0 1:64 31:255 1:223 1:127 1:32 24:0 1:48 1:239 53:255 1:176 4:0
ah 45 109 1:0 1:32 34:255 1:175 1:79 21:0 1:48 1:239 54:255 1:223 4:0
ah 45 110 2:0 36:255 1:239 1:143 1:32 17:0 1:48 1:239 56:255 1:16 3:0
ah 45 111 2:0 1:208 38:255 1:191 1:95 1:16 13:0 1:48 1:239 57:255 1:64 3:0
ah 45 112 2:0 1:176 40:255 1:239 1:159 1:48 10:0 1:48 1:239 58:255 1:128 3:0
ah 45 113 2:0 1:128 43:255 1:207 1:95 1:16 6:0 1:48 1:239 59:255 1:160 3:0
ah 45 114 2:0 1:80 46:255 1:159 1:64 3:0 1:48 1:239 60:255 1:192 3:0
ah 45 115 2:0 1:48 48:255 1:223 1:111 1:64 1:239 62:255 3:0
ah 45 116 3:0 1:240 48:255 1:208 1:16 1:80 1:176 61:255 1:16 2:0
ah 45 117 3:0 1:192 47:255 1:208 1:16 3:0 1:32 1:128 1:224 58:255 1:64 2:0
ah 45 118 3:0 1:128 46:255 1:208 1:16 7:0 1:64 1:160 56:255 1:112 2:0
ah 45 119 3:0 1:80 45:255 1:208 1:16 10:0 1:16 1:112 1:224 53:255 1:128 2:0
ah 45 120 3:0 1:16 44:255 1:208 1:16 14:0 1:48 1:160 1:240 50:255 1:192 2:0
ah 45 121 4:0 1:208 42:255 1:176 1:16 17:0 1:16 1:96 1:208 48:255 1:192 2:0
ah 45 122 4:0 1:128 41:255 1:160 22:0 1:48 1:144 1:240 45:255 1:239 2:0
ah 45 123 4:0 1:48 40:255 1:160 26:0 1:96 1:192 44:255 2:0
ah 45 124 5:0 1:224 38:255 1:160 29:0 1:32 1:128 1:224 41:255 1:48 1:0
ah 45 125 5:0 1:144 37:255 1:128 33:0 1:80 1:176 39:255 1:64 1:0
ah 45 126 5:0 1:48 36:255 1:96 36:0 1:32 1:112 1:224 36:255 1:96 1:0
ah 45 127 6:0 1:224 33:255 1:240 1:80 40:0 1:64 1:160 34:255 1:128 1:0
ah 45 128 6:0 1:112 32:255 1:240 1:48 43:0 1:16 1:112 1:208 31:255 1:128 1:0
ah 45 129 6:0 1:16 1:240 30:255 1:208 1:32 47:0 1:48 1:160 1:240 28:255 1:160 1:0
ah 45 130 7:0 1:128 29:255 1:176 1:16 50:0 1:16 1:96 1:192 26:255 1:192 1:0
ah 45 131 7:0 1:16 1:240 27:255 1:128 55:0 1:32 1:144 1:240 23:255 1:192 1:0
ah 45 132 8:0 1:128 25:255 1:240 1:64 59:0 1:96 1:176 21:255 1:239 1:0
ah 45 133 9:0 1:208 23:255 1:192 1:16 62:0 1:32 1:128 1:224 19:255 1:0
ah 45 134 9:0 1:48 1:240 20:255 1:240 1:112 67:0 1:80 1:176 17:255 1:0
ah 45 135 10:0 1:64 19:255 1:192 1:32 70:0 1:16 1:112 1:224 14:255 1:16
ah 45 136 11:0 1:96 1:240 15:255 1:224 1:64 75:0 1:64 1:160 12:255 1:64
ah 45 137 12:0 1:48 1:224 12:255 1:224 1:96 79:0 1:16 1:96 1:208 9:255 1:64
ah 45 138 13:0 1:16 1:128 1:224 7:255 1:240 1:160 1:64 84:0 1:48 1:160 1:240 6:255 1:64
ah 45 139 16:0 1:64 1:112 3:128 1:112 1:48 90:0 1:96 1:192 4:255 1:80
ah 45 140 115:0 1:32 1:144 1:240 1:255 1:128
ah 45 141 118:0 1:80 1:80
END
CASE 0 1 2 13 7 150 170 | M 436e00a4 c155a8f8 C 432a8c39 43176f1c 42dc2ff4 42f769e8 43612804 c194eaa0 Q 4285a982 c19ba6d3 43464305 c186ec3a V 431de25d Q 434e387d c00c9470 43523002 42aecd4f Q c21984f4 42b2505e 435ff1b4 430b5cde Z
h 162 82 1
h 162 83 1
h 162 84 1
h 162 85 1
h 161 86 2
h 161 87 2
h 158 88 3
h 139 89 22
h 126 90 35
h 116 91 45
h 109 92 52
h 103 93 58
h 99 94 62
h 96 95 65
h 93 96 68
h 91 97 71
h 90 98 72
h 90 99 73
h 89 100 74
h 89 101 74
h 90 102 73
h 91 103 72
h 92 104 71
h 93 105 70
h 95 106 68
h 97 107 66
h 99 108 64
h 101 109 62
h 104 110 59
h 107 111 56
h 109 112 54
h 112 113 51
h 115 114 48
h 119 115 44
h 122 116 41
h 126 117 37
h 129 118 34
h 133 119 30
h 137 120 26
h 141 121 22
h 145 122 18
h 149 123 14
h 153 124 10
h 157 125 6
h 162 126 1
END
CASE 3 0 2 13 7 150 170 | M c72b3608 c742a060 V 47674c50 Q c6f5d81c c7617555 c761af9a c5bca120 Q c787b222 466900c0 464cba40 475a5874 Z M 46abc9f0 c76f1052 L 467de500 46328b78 L c7863384 c6aadb42 V 47728270 L c60ed560 c66b22d4 Z M c4561900 c5119d40 L 45ab0d80 476e6440 V 46bd2e40 V 4752d512 C 46a1ea58 c71bf7ae c6d2e4d6 46b75784 469c2804 475d6160 Z
ah 13 129 1:13
ah 13 130 1:103
ah 13 131 1:193
ah 13 132 1:227 1:28
ah 13 133 1:137 1:118
ah 13 134 1:46 1:209
ah 14 135 1:212 1:43
ah 14 136 1:122 1:133
ah 14 137 1:31 1:224
ah 15 138 1:197 1:58
ah 15 139 1:107 1:148
ah 15 140 1:16 1:239
ah 16 141 1:182 1:73
ah 16 142 1:92 1:163
ah 16 143 1:1 1:254
ah 17 144 1:167 1:88
ah 17 145 1:76 1:179
ah 18 146 1:242 1:13
ah 18 147 1:152 1:103
ah 18 148 1:61 1:194
ah 19 149 1:227 1:28
ah 19 150 1:137 1:118
ah 19 151 1:46 1:209
ah 20 152 1:212 1:43
ah 20 153 1:122 1:133
ah 20 154 1:31 1:224
ah 21 155 1:197 1:58
ah 21 156 1:106 1:149
ah 21 157 1:16 1:239
ah 22 158 1:182 1:73
ah 22 159 1:91 1:164
ah 22 160 1:1 1:254
ah 23 161 1:167 1:88
ah 23 162 1:76 1:179
ah 24 163 1:242 1:13
ah 24 164 1:152 1:103
ah 24 165 1:61 1:194
ah 25 166 1:227 1:28
ah 25 167 1:137 1:118
ah 25 168 1:46 1:209
ah 26 169 1:212 1:43
ah 26 170 1:121 1:134
ah 26 171 1:31 1:224
ah 27 172 1:197 1:58
ah 27 173 1:106 1:149
ah 27 174 1:16 1:239
ah 28 175 1:182 1:73
ah 28 176 1:91 1:164
END
`

// TestScanGolden runs each case of scanCases.
func TestScanGolden(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(scanCases), "\n")
	for i := 0; i < len(lines); i++ {
		head, desc, _ := strings.Cut(lines[i], "|")
		hf := strings.Fields(head)
		var v [7]int
		for j := range v {
			n, err := strconv.Atoi(hf[1+j])
			if err != nil {
				t.Fatal(err)
			}
			v[j] = n
		}
		var want strings.Builder
		for i++; lines[i] != "END"; i++ {
			want.WriteString(lines[i] + "\n")
		}

		p := buildTestPath(t, desc)
		clip, ok := screenIntRectFromXYWH(uint32(v[3]), uint32(v[4]), uint32(v[5]), uint32(v[6]))
		if !ok {
			t.Fatal("bad clip")
		}
		fillRule := FillRule(v[1])
		lineCap := LineCap(v[2])
		rec := &recordingBlitter{}
		switch v[0] {
		case 0:
			fillPathPath(p, fillRule, &clip, rec, nil)
		case 1:
			fillPathPathAA(p, fillRule, &clip, rec, nil)
		case 2:
			strokePathHairline(p, lineCap, &clip, rec)
		case 3:
			strokePathHairlineAA(p, lineCap, &clip, rec)
		case 4:
			b := p.Bounds()
			fillRect(&b, &clip, rec)
		case 5:
			b := p.Bounds()
			fillRectAA(&b, &clip, rec)
		}
		if got := rec.sb.String(); got != want.String() {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", head, got, want.String())
		}
	}
}
