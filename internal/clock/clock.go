package clock

func GetSyncedTime(serverURL string) (int64, error) {
	T1, T2, T3, err := RequestServerTime(serverURL)
	if err != nil {
		return 0, err
	}

	offset := CalculateOffset(T1, T2, T3)

	correctedTime := T3 + offset

	return correctedTime, nil
}
