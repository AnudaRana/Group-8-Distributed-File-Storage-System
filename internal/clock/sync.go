package clock

import "sort"

// CalculateOffset computes the local clock skew against a server clock
// using standard Cristian's algorithm timing formulas.
func CalculateOffset(T1, T2, T3 int64) int64 {
	return T2 - (T1+T3)/2
}

// CalculateRTT gauges the total network latency experienced
// during the duration of a round-trip synchronization ping.
func CalculateRTT(T1, T3 int64) int64 {
	return T3 - T1
}

// medianInt64 extracts the median value from a slice of integers 
// to naturally filter out extreme network jitter outliers during clock sampling.
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

// meanInt64 computes the average of a set of 64-bit integer values mathematically.
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
