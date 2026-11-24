// Package telemetry holds a small in-memory, time-ordered window of recent
// telemetry samples so incoming detections can be matched against interpolated
// drone telemetry without round-tripping to Postgres.
package telemetry

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

var (
	ErrBufferEmpty  = errors.New("telemetry buffer empty")
	ErrTelemetryGap = errors.New("telemetry gap exceeds maximum allowed threshold")
	ErrClockSkew    = errors.New("clock skew exceeds maximum tolerance")
)

// Sample is one drone telemetry reading.
type Sample struct {
	Lat     float64
	Lon     float64
	Alt     float64
	Heading float64
	Pitch   float64
	Roll    float64
	Yaw     float64
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
// timestamp order; Add still inserts at the correct sorted position if a
// sample arrives slightly out of order.
func (b *Buffer) Add(s Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if s.Yaw == 0 && s.Heading != 0 {
		s.Yaw = s.Heading
	} else if s.Heading == 0 && s.Yaw != 0 {
		s.Heading = s.Yaw
	}

	idx := sort.Search(len(b.samples), func(i int) bool { return b.samples[i].At.After(s.At) })
	b.samples = append(b.samples, Sample{})
	copy(b.samples[idx+1:], b.samples[idx:])
	b.samples[idx] = s

	b.evictLocked()
}

func (b *Buffer) evictLocked() {
	if len(b.samples) == 0 {
		return
	}
	latest := b.samples[len(b.samples)-1].At
	cutoff := latest.Add(-b.maxAge)
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

// Interpolate finds the two telemetry samples bracketing the target timestamp t
// and computes a linear (and shortest-arc angular) interpolation of position and attitude.
//
// Explicit error handling:
// - Returns ErrBufferEmpty if no telemetry samples exist.
// - Returns ErrClockSkew if t is ahead of newest or behind oldest sample by more than skewTolerance.
// - Returns ErrTelemetryGap if the temporal gap between the two bracketing samples exceeds maxGap.
func (b *Buffer) Interpolate(t time.Time, maxGap, skewTolerance time.Duration) (Sample, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	n := len(b.samples)
	if n == 0 {
		return Sample{}, ErrBufferEmpty
	}

	// 1. Clock skew check: target t is earlier than the earliest buffered sample
	if t.Before(b.samples[0].At) {
		diff := b.samples[0].At.Sub(t)
		if diff <= skewTolerance {
			res := b.samples[0]
			res.At = t
			return res, nil
		}
		return Sample{}, fmt.Errorf("%w: frame timestamp %v is %v earlier than oldest sample %v",
			ErrClockSkew, t, diff, b.samples[0].At)
	}

	// 2. Clock skew check: target t is later than the latest buffered sample
	if t.After(b.samples[n-1].At) {
		diff := t.Sub(b.samples[n-1].At)
		if diff <= skewTolerance {
			res := b.samples[n-1]
			res.At = t
			return res, nil
		}
		return Sample{}, fmt.Errorf("%w: frame timestamp %v is %v later than newest sample %v",
			ErrClockSkew, t, diff, b.samples[n-1].At)
	}

	// 3. Bracketing search: find index where samples[idx].At >= t
	idx := sort.Search(n, func(i int) bool { return !b.samples[i].At.Before(t) })

	if idx == 0 {
		res := b.samples[0]
		res.At = t
		return res, nil
	}

	s1 := b.samples[idx-1]
	s2 := b.samples[idx]

	gap := s2.At.Sub(s1.At)
	if gap > maxGap {
		return Sample{}, fmt.Errorf("%w: telemetry gap %v between %v and %v exceeds max %v",
			ErrTelemetryGap, gap, s1.At, s2.At, maxGap)
	}

	if gap <= 0 {
		res := s1
		res.At = t
		return res, nil
	}

	alpha := float64(t.Sub(s1.At)) / float64(gap)
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}

	// Linear interpolation for spatial coordinates and altitude
	lat := s1.Lat + alpha*(s2.Lat-s1.Lat)
	lon := s1.Lon + alpha*(s2.Lon-s1.Lon)
	alt := s1.Alt + alpha*(s2.Alt-s1.Alt)
	pitch := s1.Pitch + alpha*(s2.Pitch-s1.Pitch)
	roll := s1.Roll + alpha*(s2.Roll-s1.Roll)

	// Shortest-arc spherical interpolation for heading/yaw across 0/360 boundary
	heading := interpolateAngleDeg(s1.Heading, s2.Heading, alpha)
	yaw := interpolateAngleDeg(s1.Yaw, s2.Yaw, alpha)

	return Sample{
		Lat:     lat,
		Lon:     lon,
		Alt:     alt,
		Heading: heading,
		Pitch:   pitch,
		Roll:    roll,
		Yaw:     yaw,
		At:      t,
	}, nil
}

// interpolateAngleDeg interpolates between two angles in degrees [0, 360) along the shortest arc.
func interpolateAngleDeg(deg1, deg2, alpha float64) float64 {
	diff := math.Mod(deg2-deg1+540.0, 360.0) - 180.0
	res := math.Mod(deg1+alpha*diff+360.0, 360.0)
	if res < 0 {
		res += 360.0
	}
	return res
}

// Nearest returns the sample closest in time to t, provided it's within tolerance.
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
