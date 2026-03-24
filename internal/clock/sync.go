package clock

func CalculateOffset(T1, T2, T3 int64) int64 {
	return T2 - ((T1 + T3) / 2)
}
