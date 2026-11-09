package wide

import (
	"flag"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// exhaustive turns on the tests that check each of the 2^32 values of a lane.
var exhaustive = flag.Bool("wide.exhaustive", false, "check each of the 2^32 values of a lane")

// roundF32x4Upstream is the literal port of the scalar code of f32x4::round
// in tiny-skia src/wide/f32x4_t.rs. roundF32x4 gives the same bits.
func roundF32x4Upstream(v F32x4) F32x4 {
	toInt := SplatF32x4(1.0 / epsilon)
	u := v.toU32x4Bitcast()
	e := andU32x4(shrU32x4(u, 23), SplatU32x4(0xff)).toI32x4Bitcast()
	var y F32x4

	noOpMagic := SplatI32x4(0x7f + 23)
	noOpMask := toF32x4BitcastI32x4(orI32x4(cmpGtI32x4(e, noOpMagic), cmpEqI32x4(e, noOpMagic)))
	noOpVal := v

	zeroMagic := SplatI32x4(0x7f - 1)
	zeroMask := toF32x4BitcastI32x4(cmpLtI32x4(e, zeroMagic))
	zeroVal := mulF32x4(v, SplatF32x4(0.0))

	negBit := toF32x4BitcastI32x4(cmpLtI32x4(u.toI32x4Bitcast(), I32x4{}))
	x := blendF32x4(negBit, negF32x4(v), v)
	y = subF32x4(subF32x4(addF32x4(x, toInt), toInt), x)
	y = blendF32x4(cmpGtF32x4(y, SplatF32x4(0.5)),
		subF32x4(addF32x4(y, x), SplatF32x4(-1.0)),
		blendF32x4(cmpLtF32x4(y, SplatF32x4(-0.5)), addF32x4(addF32x4(y, x), SplatF32x4(1.0)), addF32x4(y, x)),
	)
	y = blendF32x4(negBit, negF32x4(y), y)

	return blendF32x4(noOpMask, noOpVal, blendF32x4(zeroMask, zeroVal, y))
}

// floorF32x4Upstream is the literal port of the scalar code of f32x4::floor
// in tiny-skia src/wide/f32x4_t.rs. floorF32x4 gives the same bits.
func floorF32x4Upstream(v F32x4) F32x4 {
	roundtrip := toF32x4I32x4(truncIntF32x4(v))
	return subF32x4(roundtrip, blendF32x4(cmpGtF32x4(roundtrip, v), SplatF32x4(1.0), F32x4{}))
}

// TestRoundExhaustive checks that roundF32x4 gives the bits of the literal
// port for each of the 2^32 values of a lane. It runs with -wide.exhaustive,
// and it takes about 7 seconds on 32 cores.
func TestRoundExhaustive(t *testing.T) {
	if !*exhaustive {
		t.Skip("run with -wide.exhaustive")
	}
	var bad atomic.Int64
	var wg sync.WaitGroup
	n := runtime.NumCPU()
	step := uint64(1<<32) / uint64(n)
	for w := range n {
		wg.Add(1)
		go func(lo, hi uint64) {
			defer wg.Done()
			for i := lo; i < hi; i += 4 {
				var v F32x4
				for k := range 4 {
					v[k] = math.Float32frombits(uint32(i) + uint32(k))
				}
				got, want := roundF32x4(v), roundF32x4Upstream(v)
				for k := range 4 {
					if math.Float32bits(got[k]) != math.Float32bits(want[k]) && bad.Add(1) < 5 {
						t.Errorf("Round of the bits %08x gives %08x, want %08x",
							math.Float32bits(v[k]), math.Float32bits(got[k]), math.Float32bits(want[k]))
					}
				}
			}
		}(uint64(w)*step, uint64(w+1)*step)
	}
	wg.Wait()
}

// TestFloorExhaustive checks that floorF32x4 gives the bits of the literal
// port for each of the 2^32 values of a lane. It runs with -wide.exhaustive.
func TestFloorExhaustive(t *testing.T) {
	if !*exhaustive {
		t.Skip("run with -wide.exhaustive")
	}
	var bad atomic.Int64
	var wg sync.WaitGroup
	n := runtime.NumCPU()
	step := uint64(1<<32) / uint64(n)
	for w := range n {
		wg.Add(1)
		go func(lo, hi uint64) {
			defer wg.Done()
			for i := lo; i < hi; i += 4 {
				var v F32x4
				for k := range 4 {
					v[k] = math.Float32frombits(uint32(i) + uint32(k))
				}
				got, want := floorF32x4(v), floorF32x4Upstream(v)
				for k := range 4 {
					if math.Float32bits(got[k]) != math.Float32bits(want[k]) && bad.Add(1) < 5 {
						t.Errorf("Floor of the bits %08x gives %08x, want %08x",
							math.Float32bits(v[k]), math.Float32bits(got[k]), math.Float32bits(want[k]))
					}
				}
			}
		}(uint64(w)*step, uint64(w+1)*step)
	}
	wg.Wait()
}

// TestRoundIntExhaustive checks that LaneRoundInt gives TruncI32 of
// roundLaneF32, the code of RoundInt before its fast path, for each of the
// 2^32 values of a lane. It runs with -wide.exhaustive.
func TestRoundIntExhaustive(t *testing.T) {
	if !*exhaustive {
		t.Skip("run with -wide.exhaustive")
	}
	exhaustiveLanes(t, "RoundInt", func(x float32) (uint32, uint32) {
		return uint32(LaneRoundInt(x)), uint32(TruncI32(roundLaneF32(x)))
	})
}

// TestFloor16Exhaustive checks that LaneFloor16 gives the bits of the vector
// code of f32x16::floor for each of the 2^32 values of a lane. It runs with
// -wide.exhaustive.
func TestFloor16Exhaustive(t *testing.T) {
	if !*exhaustive {
		t.Skip("run with -wide.exhaustive")
	}
	exhaustiveLanes(t, "Floor16", func(x float32) (uint32, uint32) {
		v := SplatF32x8(x)
		roundtrip := roundF32x8(v)
		want := subF32x8(roundtrip, blendF32x8(cmpGtF32x8(roundtrip, v), SplatF32x8(1.0), SplatF32x8(0.0)))
		return math.Float32bits(LaneFloor16(x)), math.Float32bits(want[0])
	})
}

// exhaustiveLanes calls f on each of the 2^32 values of a lane, on all the
// cores, and reports each value where f gives two different results.
func exhaustiveLanes(t *testing.T, name string, f func(x float32) (got, want uint32)) {
	t.Helper()
	var bad atomic.Int64
	var wg sync.WaitGroup
	n := runtime.NumCPU()
	step := uint64(1<<32) / uint64(n)
	for w := range n {
		wg.Add(1)
		go func(lo, hi uint64) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				x := math.Float32frombits(uint32(i))
				if got, want := f(x); got != want && bad.Add(1) < 5 {
					t.Errorf("%s of the bits %08x gives %08x, want %08x", name, uint32(i), got, want)
				}
			}
		}(uint64(w)*step, uint64(w+1)*step)
	}
	wg.Wait()
}
