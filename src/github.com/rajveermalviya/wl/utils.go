package wl

import (
	"math"
)

func fixedToFloat64(fixed int32) float64 {
	dat := ((int64(1023 + 44)) << 52) + (1 << 51) + int64(fixed)
	return math.Float64frombits(uint64(dat)) - float64(3<<43)
}

func float64ToFixed(v float64) int32 {
	dat := v + float64(int64(3)<<(51-8))
	return int32(math.Float64bits(dat))
}
