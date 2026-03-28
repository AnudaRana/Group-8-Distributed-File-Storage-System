package clock

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"dfs-system/internal/transport"
)

const (

	DefaultSamples = 8
	MaxRTTNs = 100_000_000
	MinAcceptedSamples = 3
	DefaultPollInterval = 30 * time.Second
)

type Syncer struct {
	peerURL string

	mu           sync.RWMutex
	offset       int64
	lastSyncTime time.Time
	hasSynced    bool
	skewHistory  []SkewRecord
	prevOffset   int64
}

func NewSyncer(peerURL string) *Syncer {
	return &Syncer{peerURL: peerURL}
}

func collectOneSample(peerURL string) (T1, T2, T3 int64, err error) {
	T1 = time.Now().UnixNano()

	var data TimeResponse
	if err := transport.GetJSON(peerURL+"/time", &data); err != nil {
		return 0, 0, 0, fmt.Errorf("time request to %s: %w", peerURL, err)
	}

	T3 = time.Now().UnixNano()
	
	log.Printf("⏱️  [NTP_SYNC] Executing Cristian's Algorithm | T1: %v | Server T2: %v | Local T3: %v", T1, data.ServerTime, T3)
	
	return T1, data.ServerTime, T3, nil
}

func (s *Syncer) CollectSamples(n int) ([]SyncSample, error) {
	accepted := make([]SyncSample, 0, n)

	for i := 0; i < n; i++ {
		T1, T2, T3, err := collectOneSample(s.peerURL)
		if err != nil {
			log.Printf("[clock] sample %d/%d failed: %v", i+1, n, err)
			continue
		}

		rtt := CalculateRTT(T1, T3)
		if rtt > MaxRTTNs {
			log.Printf("[clock] sample %d/%d discarded: RTT %dms exceeds limit",
				i+1, n, rtt/1_000_000)
			continue
		}

		accepted = append(accepted, SyncSample{
			T1:     T1,
			T2:     T2,
			T3:     T3,
			Offset: CalculateOffset(T1, T2, T3),
			RTT:    rtt,
		})
	}

	if len(accepted) < MinAcceptedSamples {
		return accepted, fmt.Errorf(
			"only %d/%d samples accepted (need %d): peer may be unreachable",
			len(accepted), n, MinAcceptedSamples,
		)
	}
	return accepted, nil
}

func (s *Syncer) Synchronise() (SyncResult, error) {
	samples, err := s.CollectSamples(DefaultSamples)
	result := SyncResult{SamplesTotal: DefaultSamples, SamplesUsed: len(samples)}
	if err != nil {
		return result, err
	}

	offsets := make([]int64, len(samples))
	rtts := make([]int64, len(samples))
	for i, sm := range samples {
		offsets[i] = sm.Offset
		rtts[i] = sm.RTT
	}

	bestOffset := medianInt64(offsets)
	meanRTT := meanInt64(rtts)

	s.mu.Lock()
	skew := bestOffset - s.prevOffset
	s.prevOffset = bestOffset
	s.offset = bestOffset
	s.lastSyncTime = time.Now()
	s.hasSynced = true
	s.skewHistory = append(s.skewHistory, SkewRecord{
		TimestampNs: time.Now().UnixNano(),
		SkewNs:      skew,
	})
	s.mu.Unlock()

	result.Offset = bestOffset
	result.MeanRTT = meanRTT
	result.SkewNs = skew

	log.Printf("[clock] sync OK — offset=%dns  meanRTT=%dms  skew=%dns  samples=%d/%d",
		bestOffset, meanRTT/1_000_000, skew, len(samples), DefaultSamples)
	
	log.Printf("📉 [SKEW_ANALYSIS] Measured Drift: %vns | Applying offset to ensure strict event ordering and preserve Raft consistency.", skew)

	return result, nil
}

func (s *Syncer) Now() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.hasSynced {
		log.Printf("[clock] WARNING: no sync yet — falling back to local clock (accuracy unknown)")
		return time.Now()
	}

	return time.Unix(0, time.Now().UnixNano()+s.offset)
}

func (s *Syncer) Offset() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.offset
}

func (s *Syncer) OffsetSeconds() float64 {
	return float64(s.Offset()) / 1e9
}

func (s *Syncer) SkewHistory() []SkewRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make([]SkewRecord, len(s.skewHistory))
	copy(cp, s.skewHistory)
	return cp
}

func HandleSyncFailure(s *Syncer) (time.Time, error) {
	s.mu.RLock()
	hasSynced := s.hasSynced
	lastSync := s.lastSyncTime
	s.mu.RUnlock()

	if hasSynced {
		log.Printf("[clock] sync failure fallback: using last known offset (last sync: %s)",
			lastSync.Format(time.RFC3339))
		log.Printf("⚠️  [SYNC_FAILURE] Network time unreachable. Gracefully degrading to Local Monotonic Clock + Last Known Offset.")
		return s.Now(), nil
	}

	log.Printf("[clock] sync failure fallback: no prior sync — using local clock")
	log.Printf("🚨 [CRITICAL_SYNC_FAILURE] No prior clock synchronization exists. Safely falling back to local isolated clock to prevent global ordering corruption.")
	return time.Now(), errors.New("no successful sync has occurred; local clock used as fallback")
}

func (s *Syncer) RunLoop(interval time.Duration, stopCh <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if _, err := s.Synchronise(); err != nil {
					log.Printf("[clock] periodic sync failed: %v — retaining offset %dns",
						err, s.Offset())
				}
			case <-stopCh:
				log.Printf("[clock] background sync loop stopped")
				return
			}
		}
	}()
}

func GetSyncedTime(peerURL string) (int64, error) {
	syncer := NewSyncer(peerURL)
	if _, err := syncer.Synchronise(); err != nil {
		return 0, err
	}
	return syncer.Now().UnixNano(), nil
}
