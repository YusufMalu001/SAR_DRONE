// Package priority scores fused detections for triage: how urgently should
// a human responder look at this one?
package priority

import (
	"sync"
	"time"

	"sar-drone-prototype/fusion-service/internal/geo"
)

// Base priority by detection class. Person detections rank highest (a
// person found is the primary SAR objective); among hazards, fire and
// structural damage are the most immediately life-threatening, then flood,
// then debris.
var baseByClass = map[string]float64{
	"person":            1.0,
	"fire":              0.8,
	"structural_damage": 0.7,
	"flood":             0.6,
	"debris":            0.4,
}

const defaultBase = 0.5

// ProximityBoost is added to a detection's score when a hazard-class
// detection was recorded nearby recently — e.g. a person spotted close to
// a fire is more urgent than the same person detection in isolation.
const ProximityBoost = 0.2

// Base returns the base priority for a detection class, defaulting to 0.5
// for any class not in the table above (defensive — the shared schema only
// allows the five classes we know about).
func Base(class string) float64 {
	if v, ok := baseByClass[class]; ok {
		return v
	}
	return defaultBase
}

// Score combines class, confidence, and hazard proximity into a 0-1
// priority score.
func Score(class string, confidence float64, nearHazard bool) float64 {
	score := Base(class) * clamp01(confidence)
	if nearHazard {
		score += ProximityBoost
	}
	return clamp01(score)
}

// IsHazardClass reports whether class is one of the environmental hazard
// classes (as opposed to "person").
func IsHazardClass(class string) bool {
	switch class {
	case "fire", "flood", "debris", "structural_damage":
		return true
	default:
		return false
	}
}

type hazardRecord struct {
	class string
	point geo.Point
	at    time.Time
}

// HazardMatch is the closest recorded hazard to a query point, as returned
// by HazardTracker.Nearest — used both for the priority-boost check and to
// compose an alert message naming which hazard a survivor is near.
type HazardMatch struct {
	Class          string
	DistanceMeters float64
}

// HazardTracker remembers recent hazard-class detection locations so new
// detections can be scored with a proximity boost.
type HazardTracker struct {
	mu           sync.Mutex
	window       time.Duration
	radiusMeters float64
	recent       []hazardRecord
}

func NewHazardTracker(window time.Duration, radiusMeters float64) *HazardTracker {
	return &HazardTracker{window: window, radiusMeters: radiusMeters}
}

// Record stores a hazard-class detection's location for future proximity
// checks. No-op for non-hazard classes (e.g. "person").
func (t *HazardTracker) Record(class string, point geo.Point, at time.Time) {
	if !IsHazardClass(class) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.recent = append(t.recent, hazardRecord{class: class, point: point, at: at})
	t.pruneLocked(at)
}

// Nearest returns the closest recorded hazard to point (within the
// tracker's time window), regardless of the configured proximity radius —
// callers decide what "close enough" means for their own purpose (compare
// against HasNearby, which applies the tracker's own radius).
func (t *HazardTracker) Nearest(point geo.Point, at time.Time) (HazardMatch, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(at)

	var best HazardMatch
	found := false
	for _, r := range t.recent {
		d := geo.HaversineMeters(point, r.point)
		if !found || d < best.DistanceMeters {
			best = HazardMatch{Class: r.class, DistanceMeters: d}
			found = true
		}
	}
	return best, found
}

// HasNearby reports whether any recorded hazard is within the configured
// radius and time window of (point, at).
func (t *HazardTracker) HasNearby(point geo.Point, at time.Time) bool {
	match, found := t.Nearest(point, at)
	return found && match.DistanceMeters <= t.radiusMeters
}

// pruneLocked drops records older than the window, relative to `now`.
// Callers must hold t.mu.
func (t *HazardTracker) pruneLocked(now time.Time) {
	cutoff := now.Add(-t.window)
	kept := t.recent[:0]
	for _, r := range t.recent {
		if r.at.After(cutoff) {
			kept = append(kept, r)
		}
	}
	t.recent = kept
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
