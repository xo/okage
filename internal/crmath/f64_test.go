package crmath

import (
	"math"
	"testing"
)

// mpfrValues holds results of MPFR 4 for random inputs, which are correctly
// rounded. The comment of a row gives the result of glibc 2.44 where it
// differs. The inputs are x, then y, as bits. Only atan2 and pow read y, and
// atan2 computes atan2(y, x).
//
// The rows come from a C program that calls MPFR and glibc on 30,000 random
// inputs of each function, from tiny values to values near 2^1000. Every one
// of the 210,000 results of this package matched MPFR on 2026-11-09, except
// one result of pow in the range of subnormal values, where the program
// rounded the result of MPFR twice. math/big at 2,000 bits showed that the
// result of this package was the nearest one.
var mpfrValues = []struct {
	fn         string
	x, y, want uint64
}{
	{"sin", 0x401ecd3e83229167, 0x400cf91edd645f98, 0x3fef9f9e512245cc},
	{"sin", 0x4068df226732e5f5, 0x4013fcfbf2bef14e, 0xbfebcc8e28b3b40f},
	{"sin", 0xc0cc1ffd5a85eeb8, 0x3ffeefcca4039b00, 0x3fec3e649d00cfed},
	{"sin", 0xd485c308459d479e, 0x4010700dafd91c9a, 0x3fe54553d0c1f0b9},
	{"sin", 0xd2b25597ab3666a8, 0xc067d5553aff2ebf, 0xbfd972b26fbf0879}, // glibc gives 0xbfd972b26fbf0878
	{"sin", 0x414f053c6a0a4ea0, 0xc5a1a3a3c8d99ee0, 0x3fe69a50b6fad1fd}, // glibc gives 0x3fe69a50b6fad1fe
	{"sin", 0xc022374211a9bbb1, 0xbc66fdab1349fe14, 0xbfd3f0dff41559ed}, // glibc gives 0xbfd3f0dff41559ec
	{"sin", 0xdf67f61bbe976a5c, 0xc5e69b70d0221c60, 0x3fe575b980e98e31}, // glibc gives 0x3fe575b980e98e30
	{"sin", 0x40b9d724a0307b4c, 0xc2c57c4abac1d29a, 0xbfebc63e8cf752cd}, // glibc gives 0xbfebc63e8cf752cc
	{"sin", 0x401899adddb673ca, 0xc085c0b574fbe4ff, 0xbfc0fcd07eab2993}, // glibc gives 0xbfc0fcd07eab2992
	{"cos", 0x63d5bb9a2114270c, 0x43498aca451ce89c, 0xbfcc6fc3e968dc13}, // glibc gives 0xbfcc6fc3e968dc12
	{"cos", 0xc08a98c0888ef620, 0x408a97c1fb859b55, 0xbfeec6613e4841cd},
	{"cos", 0x442948787d7c8e4e, 0x408b43268cb61ce9, 0x3fca3675e8f8ec1e},
	{"cos", 0x40377653dcd8da84, 0xc07d81372790a8b7, 0xbfb97cee1a9842fb},
	{"cos", 0x4021651ed4aae280, 0xc589f109e06e6480, 0xbfe7e7523799d42d},
	{"cos", 0x403b1256f28f2522, 0x6f0af54a04ebe0f8, 0xbfd707b2ecf71807}, // glibc gives 0xbfd707b2ecf71806
	{"cos", 0xc0145c0aaddbf1b6, 0xc031cadc7ff55ede, 0x3fd7970915e1c6c5}, // glibc gives 0x3fd7970915e1c6c6
	{"cos", 0x44f2967e015228f0, 0x58fe03f269f3ab68, 0x3fdfc314a396b7d5}, // glibc gives 0x3fdfc314a396b7d4
	{"cos", 0x40799cdcf9dfc7f6, 0x40056a81bf2a7009, 0x3fc6256be0d86daf}, // glibc gives 0x3fc6256be0d86dae
	{"cos", 0x419e9f2384145446, 0xbe2b9b4b7b07ba2c, 0xbfb89abe893d216d}, // glibc gives 0xbfb89abe893d216e
	{"tan", 0x769108129416e0c8, 0x40718c4e00756494, 0xc0193dd99a873f0c}, // glibc gives 0xc0193dd99a873f0b
	{"tan", 0xc598724594e10380, 0xbca3cf5735b5b67c, 0x3ffb0372d3cecabc},
	{"tan", 0xe927018d5e200718, 0x44a16b78a55bee28, 0xc03e5ef70a4a3128},
	{"tan", 0xc0065989a3e5f3d4, 0xb856b24047ff5ac0, 0x3fd7352abe7d7629},
	{"tan", 0xc0882b9421559026, 0x89a6c672dafee140, 0xbfe6a150cfdcaa0c},
	{"tan", 0xc51de72b5b3bb4e2, 0xc017b011b55f4308, 0xbfe6add4cff32f1f}, // glibc gives 0xbfe6add4cff32f1e
	{"tan", 0x4018b3b8b4fcbcbc, 0xeba5e41e053f4ea0, 0xbfbbac0df6062119}, // glibc gives 0xbfbbac0df606211a
	{"tan", 0x4016834db818c124, 0x408c5c548fa32f10, 0xbfe893f0cf83e737}, // glibc gives 0xbfe893f0cf83e738
	{"tan", 0x408df48a5047a1b9, 0xef5a3adff8910970, 0x3fd9b1c0d35297a0}, // glibc gives 0x3fd9b1c0d35297a1
	{"tan", 0xc088b85069b39f4f, 0xa2998d8df9258930, 0x3fe7ede34a22d093}, // glibc gives 0x3fe7ede34a22d094
	{"atan2", 0x4793dd10a7c9c570, 0x49a04eac456e5414, 0x3ff921fb543a6e7a},
	{"atan2", 0xbff60eea1da4efbc, 0xc013b1884c2484f9, 0xbffd804ce7491245},
	{"atan2", 0x405455ce6cf817fd, 0x4016d07c7da8e048, 0x3fb1ebdd94052df6},
	{"atan2", 0xbd7f026ff3871c10, 0x3ff52d6a5c828ee0, 0x3ff921fb54444486},
	{"atan2", 0x404f741846313bd4, 0xc082589040a2bf17, 0xbff76cbf3ba2cc4f}, // glibc gives 0xbff76cbf3ba2cc50
	{"atan2", 0x4022c7da54bd5ad6, 0xc0000d4b24be325a, 0xbfcaf2018842b457}, // glibc gives 0xbfcaf2018842b458
	{"atan2", 0x40225ad0a6a420b5, 0x3fee77ed4cc543dc, 0x3fba770757b3faab}, // glibc gives 0x3fba770757b3faaa
	{"atan2", 0xbff97065159ed794, 0x4017d77cd0384774, 0x3ffd4db943a6115f}, // glibc gives 0x3ffd4db943a61160
	{"atan2", 0x3ffe17384e142d08, 0xbfe906494f5e16c4, 0xbfd9386ab0a1b3a3}, // glibc gives 0xbfd9386ab0a1b3a2
	{"pow", 0x3fd9e2ce257426b0, 0xc019e60449d743f8, 0x4075ef9d101d25af},
	{"pow", 0x40071d37e73b2d4e, 0xbe210266e35a6116, 0x3feffffffedf3f7b},
	{"pow", 0x406b62704704eb70, 0x3eda926263be1ffe, 0x3ff00023cd61e4ff},
	{"pow", 0x400db9bd0bd5cadc, 0xc01311beb63b1f78, 0x3f5f64f9fa64d323},
	{"pow", 0x400535c377848d04, 0xc01c000000000000, 0x3f51cb27a7f58487}, // glibc gives 0x3f51cb27a7f58486
	{"pow", 0x3fecec3c21d700bc, 0xc033000000000000, 0x401b4fbda24f8e6a}, // glibc gives 0x401b4fbda24f8e69
	{"pow", 0x3ffc55d22a23249e, 0xc0176713c255813e, 0x3fa213623b05fd3f}, // glibc gives 0x3fa213623b05fd40
	{"pow", 0x400676a8f57f758d, 0xbff5d6d3d2541ed0, 0x3fcf46276f424536}, // glibc gives 0x3fcf46276f424537
	{"pow", 0x3ff8eecd990c3116, 0x400da2edcd141698, 0x4014b07d247998a6}, // glibc gives 0x4014b07d247998a5
	{"pow", 0x3ffec28ad20005f4, 0x401909cccb3db892, 0x404de977e3cf2393}, // glibc gives 0x404de977e3cf2394
	{"ln", 0x3f906c8af1e591da, 0xbddf791673b8cc40, 0xc01087e9f5a4b943},
	{"ln", 0x3fefffffeeb7d508, 0x404b14badcabc0f0, 0xbe61482afcaaa836},
	{"ln", 0x401386dd4198871d, 0xfa289c9b1691c252, 0x3ff95e2f2efa4834},
	{"ln", 0x40744036281266cb, 0x583d60280c670042, 0x40171f85ecf2481a}, // glibc gives 0x40171f85ecf24819
	{"ln", 0x4080988ccf56d25d, 0xafae72604a1a0840, 0x4019197d27ee4910},
	{"ln", 0x3ff5a9befaf42aae, 0xc2f2bc2fabea4892, 0x3fd364af1987ff08}, // glibc gives 0x3fd364af1987ff07
	{"ln", 0x3fe645bea6cd77b8, 0xc079cd65394d1877, 0xbfd73154682c8e17}, // glibc gives 0xbfd73154682c8e16
	{"ln", 0x3ff9c255f2ff9636, 0x3eb94e1db5ad8dc0, 0x3fde7a1048b2d5da}, // glibc gives 0x3fde7a1048b2d5d9
	{"ln", 0x4019c96bccc501af, 0xc5754393c7e7bb10, 0x3ffdd12d1da4e3bc}, // glibc gives 0x3ffdd12d1da4e3bb
	{"ln", 0x45c881782d3d8b20, 0xbfb77224063cd580, 0x405038e57b4a905a}, // glibc gives 0x405038e57b4a905b
	{"log2", 0x41829cf6b5e64e00, 0x8aa14af52e2a7aac, 0x403937df33353b1f},
	{"log2", 0x4023fd209cf1b80e, 0x3fa32378d29df540, 0x400a91a68ad31e49},
	{"log2", 0x3fe3e60971a690fc, 0x401d4027f89da640, 0xbfe5eed93597344d},
	{"log2", 0x3a5c06763a0428c8, 0xc015852283351c7a, 0xc0564c3eff84f229},
	{"log2", 0x4010cc38f373fe78, 0x89f1fd9e20e78cc8, 0x40008fc2b381cecb}, // glibc gives 0x40008fc2b381cecc
	{"log2", 0x3ff1515bfceac938, 0xbab52a51458e6cc0, 0x3fbd3b336cb23cd4}, // glibc gives 0x3fbd3b336cb23cd3
	{"log2", 0x3ff134411175cdb6, 0x278b29d1054cfdea, 0x3fbacc70c69eaf53}, // glibc gives 0x3fbacc70c69eaf52
	{"log2", 0x3ffb3442d008a446, 0x455b6abb04f4abb4, 0x3fe8810fb1b8ab13}, // glibc gives 0x3fe8810fb1b8ab12
	{"log2", 0x3ff909b73c450fa4, 0x8297c2c4297e5218, 0x3fe4ac65dda0b91e}, // glibc gives 0x3fe4ac65dda0b91d
	{"log2", 0x3ff037b0c88f2875, 0x400f174459284a0a, 0x3f93f367a5187acf}, // glibc gives 0x3f93f367a5187ad0
}

// eval64 calls the function fn of this package.
func eval64(fn string, x, y float64) float64 {
	switch fn {
	case "sin":
		return Sin64(x)
	case "cos":
		return Cos64(x)
	case "tan":
		return Tan64(x)
	case "atan2":
		return Atan2_64(y, x)
	case "pow":
		return Pow64(x, y)
	case "ln":
		return Ln64(x)
	case "log2":
		return Log2_64(x)
	}
	panic(fn)
}

// TestF64MPFR checks the f64 functions against the results of MPFR.
func TestF64MPFR(t *testing.T) {
	for _, v := range mpfrValues {
		x, y := math.Float64frombits(v.x), math.Float64frombits(v.y)
		want := math.Float64frombits(v.want)
		if got := eval64(v.fn, x, y); math.Float64bits(got) != v.want {
			t.Errorf("%s(%v, %v) = %v, want %v", v.fn, x, y, got, want)
		}
	}
}

// TestF64Special checks the special values of the f64 functions.
func TestF64Special(t *testing.T) {
	inf, nan := math.Inf(1), math.NaN()
	negZero := math.Copysign(0, -1)
	tests := []struct {
		fn         string
		x, y, want float64
	}{
		{"sin", 0, 0, 0},
		{"sin", negZero, 0, negZero},
		{"sin", inf, 0, nan},
		{"sin", nan, 0, nan},
		{"cos", 0, 0, 1},
		{"cos", -inf, 0, nan},
		{"tan", negZero, 0, negZero},
		{"tan", inf, 0, nan},
		{"atan2", 1, 0, 0},
		{"atan2", -1, 0, math.Pi},
		{"atan2", 0, 1, math.Pi / 2},
		{"atan2", 0, -1, -math.Pi / 2},
		{"atan2", -inf, inf, 3 * math.Pi / 4},
		{"atan2", 1, 1, math.Pi / 4},
		{"atan2", -1, 1, 3 * math.Pi / 4},
		{"atan2", nan, 1, nan},
		{"pow", 2, 10, 1024},
		{"pow", -2, 3, -8},
		{"pow", -2, 0.5, nan},
		{"pow", 0, -1, inf},
		{"pow", 4, 0.5, 2},
		{"pow", 1, nan, 1},
		{"pow", nan, 0, 1},
		{"pow", 10, 400, inf},
		{"pow", 10, -400, 0},
		{"pow", 2, -1074, 5e-324},
		{"ln", 1, 0, 0},
		{"ln", 0, 0, -inf},
		{"ln", -1, 0, nan},
		{"ln", inf, 0, inf},
		{"ln", math.E, 0, 1},
		{"log2", 1024, 0, 10},
		{"log2", 0.125, 0, -3},
		{"log2", 5e-324, 0, -1074},
		{"log2", 0, 0, -inf},
		{"log2", -1, 0, nan},
	}
	for _, tt := range tests {
		got := eval64(tt.fn, tt.x, tt.y)
		if math.Float64bits(got) != math.Float64bits(tt.want) && (got == got || tt.want == tt.want) {
			t.Errorf("%s(%v, %v) = %v, want %v", tt.fn, tt.x, tt.y, got, tt.want)
		}
	}
}

// TestCos64AgreesWithCosBig checks that Cos64 gives the result of cosBig, the
// computation that Cos64 used before it reduced its argument with more bits
// of pi, on the inputs where both are exact enough.
func TestCos64AgreesWithCosBig(t *testing.T) {
	for i := range 2000 {
		x := float64(i-1000) * 0.0123456789
		if want, _ := cosBig(newFloat(x)).Float64(); Cos64(x) != want {
			t.Errorf("Cos64(%v) = %v, want %v", x, Cos64(x), want)
		}
	}
}

func BenchmarkSin64(b *testing.B) {
	for i := range b.N {
		Sin64(float64(i) * 0.001)
	}
}

func BenchmarkAtan2_64(b *testing.B) {
	for i := range b.N {
		Atan2_64(float64(i)*0.001, 1.5)
	}
}
