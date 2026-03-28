package clock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)


func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/time", TimeHandler)
	return httptest.NewServer(mux)
}

// --- Offset Calculation Tests ---
// These tests verify the SNTP formula: offset = T2 - ((T1 + T3) / 2)
// TestCalculateOffset_InSync validates the formula when clocks match precisely.
func TestCalculateOffset_InSync(t *testing.T) {
	if got := CalculateOffset(1000, 2000, 3000); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCalculateOffset_ServerAhead(t *testing.T) {
	if got := CalculateOffset(1000, 2500, 3000); got != 500 {
		t.Errorf("expected 500, got %d", got)
	}
}

func TestCalculateOffset_ServerBehind(t *testing.T) {
	if got := CalculateOffset(1000, 1500, 3000); got != -500 {
		t.Errorf("expected -500, got %d", got)
	}
}

func TestCalculateRTT(t *testing.T) {
	if got := CalculateRTT(1_000_000, 3_000_000); got != 2_000_000 {
		t.Errorf("expected 2000000, got %d", got)
	}
}

func TestMedianInt64_Odd(t *testing.T) {
	if got := medianInt64([]int64{5, 1, 3}); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

func TestMedianInt64_Even(t *testing.T) {
	if got := medianInt64([]int64{4, 2, 6, 8}); got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
}

func TestMeanInt64(t *testing.T) {
	if got := meanInt64([]int64{10, 20, 30}); got != 20 {
		t.Errorf("expected 20, got %d", got)
	}
}


// --- HTTP Handler Tests ---
// These tests confirm that the server endpoint serving time correctly replies with valid data.
func TestTimeHandler_StatusOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/time", nil)
	w := httptest.NewRecorder()
	TimeHandler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status %d, want 200", w.Code)
	}
}

func TestTimeHandler_ReturnsTimeInRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/time", nil)
	w := httptest.NewRecorder()

	before := time.Now().UnixNano()
	TimeHandler(w, req)
	after := time.Now().UnixNano()

	var resp TimeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ServerTime < before || resp.ServerTime > after {
		t.Errorf("ServerTime %d outside [%d, %d]", resp.ServerTime, before, after)
	}
}

// --- Sample Collection Tests ---
// Ensures that polling multiple timestamps (T1, T2, T3) yields correct metrics (RTT) and handles peer failures.
func TestCollectSamples_RTTAndTimingValid(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	samples, err := syncer.CollectSamples(DefaultSamples)
	if err != nil {
		t.Fatalf("CollectSamples: %v", err)
	}
	for i, s := range samples {
		if s.RTT < 0 {
			t.Errorf("sample %d: RTT %d is negative", i, s.RTT)
		}
		if s.RTT > MaxRTTNs {
			t.Errorf("sample %d: RTT %d exceeds MaxRTTNs", i, s.RTT)
		}
		if s.T3 < s.T1 {
			t.Errorf("sample %d: T3 (%d) < T1 (%d)", i, s.T3, s.T1)
		}
	}
}

func TestCollectSamples_FailsOnUnreachablePeer(t *testing.T) {
	syncer := NewSyncer("http://127.0.0.1:1")
	if _, err := syncer.CollectSamples(DefaultSamples); err == nil {
		t.Error("expected error for unreachable peer")
	}
}

// --- Synchronization Logic Tests ---
// Validates the main synchronization loop that computes the final offset using the sampled data.
func TestSynchronise_SetsOffset(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	result, err := syncer.Synchronise()
	if err != nil {
		t.Fatalf("Synchronise: %v", err)
	}
	if result.SamplesUsed < MinAcceptedSamples {
		t.Errorf("samples used %d < min %d", result.SamplesUsed, MinAcceptedSamples)
	}
}

func TestSynchronise_FailsOnUnreachablePeer(t *testing.T) {
	syncer := NewSyncer("http://127.0.0.1:1")
	if _, err := syncer.Synchronise(); err == nil {
		t.Error("expected sync failure")
	}
}
// --- Clock Interface Tests ---
// Confirms that Now() utilizes local time combined with calculated offsets correctly.
func TestNow_FallsBackToLocalBeforeSync(t *testing.T) {
	syncer := NewSyncer("http://127.0.0.1:1")
	before := time.Now()
	got := syncer.Now()
	after := time.Now()
	if got.Before(before) || got.After(after) {
		t.Errorf("fallback Now() %v outside [%v, %v]", got, before, after)
	}
}

func TestNow_ReturnsValidTimeAfterSync(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	if _, err := syncer.Synchronise(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if syncer.Now().IsZero() {
		t.Error("Now() is zero after sync")
	}
}

func TestOffsetSeconds_MatchesNanoseconds(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	if _, err := syncer.Synchronise(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	want := float64(syncer.Offset()) / 1e9
	if got := syncer.OffsetSeconds(); got != want {
		t.Errorf("OffsetSeconds %f != %f", got, want)
	}
}

func TestGetSyncedTime_ReturnsNonZero(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	ts, err := GetSyncedTime(srv.URL)
	if err != nil {
		t.Fatalf("GetSyncedTime: %v", err)
	}
	if ts == 0 {
		t.Error("returned 0")
	}
}

// --- Failure Handling & Mechanism Tests ---
// Validates fallback strategies when time servers are unreachable (as required by the assignment).
func TestHandleSyncFailure_NoPriorSync_ReturnsError(t *testing.T) {
	syncer := NewSyncer("http://127.0.0.1:1")
	got, err := HandleSyncFailure(syncer)
	if err == nil {
		t.Error("expected error when no prior sync")
	}
	if got.IsZero() {
		t.Error("fallback time must not be zero")
	}
}

func TestHandleSyncFailure_WithPriorSync_NoError(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	if _, err := syncer.Synchronise(); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	offline := &Syncer{
		peerURL:      "http://127.0.0.1:1",
		offset:       syncer.Offset(),
		lastSyncTime: time.Now(),
		hasSynced:    true,
	}
	got, err := HandleSyncFailure(offline)
	if err != nil {
		t.Errorf("unexpected error when prior sync exists: %v", err)
	}
	if got.IsZero() {
		t.Error("fallback time is zero")
	}
}

// --- Background Process Tests ---
// Tests the RunLoop ensuring the system continuously synchronizes in the background over time.
func TestRunLoop_TicksAndStopsCleanly(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	if _, err := syncer.Synchronise(); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	stop := make(chan struct{})
	syncer.RunLoop(150*time.Millisecond, stop)
	time.Sleep(400 * time.Millisecond) // allow at least two ticks
	close(stop)
	time.Sleep(50 * time.Millisecond) // let goroutine exit
}

func TestSkewHistory_GrowsWithEachSync(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	syncer := NewSyncer(srv.URL)
	for i := 0; i < 3; i++ {
		if _, err := syncer.Synchronise(); err != nil {
			t.Fatalf("sync %d: %v", i+1, err)
		}
	}
	if got := len(syncer.SkewHistory()); got != 3 {
		t.Errorf("expected 3 skew records, got %d", got)
	}
}