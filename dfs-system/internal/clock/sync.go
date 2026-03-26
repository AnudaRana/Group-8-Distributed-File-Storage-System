package clock

import "sort"

// CalculateOffset computes the estimated clock offset using the Cristian
func CalculateOffset(T1, T2, T3 int64) int64 {
	return T2 - (T1+T3)/2
}

// CalculateRTT returns the network round-trip time for one sample.
func CalculateRTT(T1, T3 int64) int64 {
	return T3 - T1
}

func medianInt64(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]int64, len(vals))
	copy(cp, vals)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

func meanInt64(vals []int64) int64 {
	if len(vals) == 0 {
		return 0
	}
	var sum int64
	for _, v := range vals {
		sum += v
	}
	return sum / int64(len(vals))
}