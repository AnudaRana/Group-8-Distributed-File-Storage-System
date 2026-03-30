package clock

import "time"

type SyncSample struct {
	T1     int64
	T2     int64
	T3     int64
	Offset int64
	RTT    int64
}

type SyncResult struct {
	Offset       int64
	MeanRTT      int64
	SamplesUsed  int
	SamplesTotal int
	SkewNs       int64
}

// SkewRecord is one historical skew measurement stored for analysis.
type SkewRecord struct {
	TimestampNs int64
	SkewNs      int64
}

// TimeResponse is the JSON payload the time server returns.
type TimeResponse struct {
	ServerTime int64 `json:"server_time"`
}

// OverheadResult holds the result of one overhead benchmark run.
type OverheadResult struct {
	PollInterval   time.Duration
	SyncDurationNs int64
	SamplesUsed    int
	MeanRTTNs      int64
	EstimatedCPUNs int64
}
