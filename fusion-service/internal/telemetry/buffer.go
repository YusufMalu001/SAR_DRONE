// Package telemetry holds a small in-memory, time-ordered window of recent
// telemetry samples so incoming detections can be matched against the
// nearest-in-time sample without round-tripping to Postgres.
package telemetry

import (
	"sort"
	"sync"
	"time"
)

// Sample is one drone telemetry reading.
type Sample struct {
	Lat     float64
	Lon     float64
	Alt     float64
	Heading float64
	At      time.Time
}

// Buffer keeps recent samples sorted by time, evicting anything older than
// maxAge or beyond maxSize (whichever trims first) on every Add.
type Buffer struct {
	mu      sync.RWMutex
	samples []Sample
	maxAge  time.Duration
	maxSize int
}

func NewBuffer(maxAge time.Duration, maxSize int) *Buffer {
	return &Buffer{maxAge: maxAge, maxSize: maxSize}
}

// Add records a new sample. Samples are expected to arrive roughly in
// timestamp order (as published by the telemetry stream); Add still inserts
// at the correct sorted position if a sample arrives slightly out of order.
func (b *Buffer) Add(s Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()

	idx := sort.Search(len(b.samples), func(i int) bool { return b.samples[i].At.After(s.At) })
	b.samples = append(b.samples, Sample{})
	copy(b.samples[idx+1:], b.samples[idx:])
	b.samples[idx] = s

	b.evictLocked()
}

func (b *Buffer) evictLocked() {
	cutoff := time.Now().Add(-b.maxAge)
	start := 0
	for start < len(b.samples) && b.samples[start].At.Before(cutoff) {
		start++
	}
	if start > 0 {
		b.samples = append([]Sample(nil), b.samples[start:]...)
	}

	if b.maxSize > 0 && len(b.samples) > b.maxSize {
		b.samples = append([]Sample(nil), b.samples[len(b.samples)-b.maxSize:]...)
	}
}

// Nearest returns the sample closest in time to t, provided it's within
// tolerance. Returns (zero, false) if the buffer is empty or nothing
// qualifies.
func (b *Buffer) Nearest(t time.Time, tolerance time.Duration) (Sample, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.samples) == 0 {
		return Sample{}, false
	}

	idx := sort.Search(len(b.samples), func(i int) bool { return !b.samples[i].At.Before(t) })

	var (
		best     Sample
		bestDiff time.Duration
		found    bool
	)

	for _, i := range [2]int{idx - 1, idx} {
		if i < 0 || i >= len(b.samples) {
			continue
		}
		diff := absDuration(b.samples[i].At.Sub(t))
		if diff > tolerance {
			continue
		}
		if !found || diff < bestDiff {
			best, bestDiff, found = b.samples[i], diff, true
		}
	}

	return best, found
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
