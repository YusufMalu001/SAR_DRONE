// Package priority scores fused detections and clustered entities for triage:
// how urgently should human incident commanders and rescue teams act on this finding?
package priority

import (
	"math"
	"sync"
	"time"

	"sar-drone-prototype/fusion-service/internal/geo"
)

// Weights and configuration for priority scoring.
type Config struct {
	BaseWeights map[string]float64
	ModalBoost  float64
	MaxPersistN int
	PersistBump float64
	HazardRadius float64
	HazardBeta  map[string]float64
}

// DefaultConfig returns the standard tuned weights specified in docs/PRIORITY.md.
func DefaultConfig() Config {
	return Config{
		BaseWeights: map[string]float64{
			"person":            1.00,
			"fire":              0.85,
			"structural_damage": 0.75,
			"flood":             0.65,
			"debris":            0.45,
		},
		ModalBoost:   0.10,
		MaxPersistN:  5,
		PersistBump:  0.10,
		HazardRadius: 50.0,
		HazardBeta: map[string]float64{
			"fire":              0.25,
			"flood":             0.20,
			"structural_damage": 0.15,
			"debris":            0.10,
		},
	}
}

var globalConfig = DefaultConfig()

// ScoringParams contains all input parameters needed to evaluate priority.
type ScoringParams struct {
	Class            string
	Confidence       float64
	Modalities       []string
	ObservationCount int
	NearHazardClass  string
	HazardDistanceM  float64
}

// Score computes the standardized priority score S in [0.0, 1.0] as specified in docs/PRIORITY.md.
func Score(params ScoringParams, cfg ...Config) float64 {
	c := globalConfig
	if len(cfg) > 0 {
		c = cfg[0]
	}

	baseWeight, ok := c.BaseWeights[params.Class]
	if !ok {
		baseWeight = 0.50
	}

	conf := clamp01(params.Confidence)
	score := baseWeight * conf

	// 1. Modality agreement boost
	hasRGB, hasThermal := false, false
	for _, m := range params.Modalities {
		if m == "rgb" {
			hasRGB = true
		}
		if m == "thermal" {
			hasThermal = true
		}
	}
	if hasRGB && hasThermal {
		score += c.ModalBoost
	}

	// 2. Persistence / Track Age bump
	if params.ObservationCount > 1 && c.MaxPersistN > 0 {
		persistRatio := math.Min(1.0, float64(params.ObservationCount)/float64(c.MaxPersistN))
		score += persistRatio * c.PersistBump
	}

	// 3. Hazard proximity boost (applies primarily to survivors)
	if params.Class == "person" && params.NearHazardClass != "" && params.HazardDistanceM <= c.HazardRadius {
		beta, exists := c.HazardBeta[params.NearHazardClass]
		if !exists {
			beta = 0.10
		}
		distRatio := math.Max(0.0, 1.0-(params.HazardDistanceM/c.HazardRadius))
		score += beta * distRatio
	}

	return clamp01(score)
}

// SimpleScore provides backward-compatible scoring combining class, confidence, and nearHazard bool.
func SimpleScore(class string, confidence float64, nearHazard bool) float64 {
	params := ScoringParams{
		Class:      class,
		Confidence: confidence,
	}
	if nearHazard {
		params.NearHazardClass = "fire"
		params.HazardDistanceM = 10.0
	}
	return Score(params)
}

// Base returns the configured base weight for a class.
func Base(class string) float64 {
	if v, ok := globalConfig.BaseWeights[class]; ok {
		return v
	}
	return 0.50
}

// IsHazardClass reports whether a class represents an environmental danger.
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

// HazardMatch contains the nearest recorded hazard and distance.
type HazardMatch struct {
	Class          string
	DistanceMeters float64
}

// HazardTracker maintains a sliding-window spatial index of recent hazards.
type HazardTracker struct {
	mu           sync.Mutex
	window       time.Duration
	radiusMeters float64
	recent       []hazardRecord
}

func NewHazardTracker(window time.Duration, radiusMeters float64) *HazardTracker {
	return &HazardTracker{window: window, radiusMeters: radiusMeters}
}

func (t *HazardTracker) Record(class string, point geo.Point, at time.Time) {
	if !IsHazardClass(class) {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.recent = append(t.recent, hazardRecord{class: class, point: point, at: at})
	t.pruneLocked(at)
}

func (t *HazardTracker) Nearest(point geo.Point, at time.Time) (HazardMatch, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(at)

	var (
		bestDist  = math.MaxFloat64
		bestClass string
		found     bool
	)
	for _, h := range t.recent {
		d := geo.HaversineMeters(point, h.point)
		if d <= t.radiusMeters && d < bestDist {
			bestDist = d
			bestClass = h.class
			found = true
		}
	}
	if !found {
		return HazardMatch{}, false
	}
	return HazardMatch{Class: bestClass, DistanceMeters: bestDist}, true
}

func (t *HazardTracker) pruneLocked(now time.Time) {
	cutoff := now.Add(-t.window)
	start := 0
	for start < len(t.recent) && t.recent[start].at.Before(cutoff) {
		start++
	}
	if start > 0 {
		t.recent = append([]hazardRecord(nil), t.recent[start:]...)
	}
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
