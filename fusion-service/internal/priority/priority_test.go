package priority

import (
	"testing"
)

func TestPriority_OrderingInvariants(t *testing.T) {
	cfg := DefaultConfig()

	// Invariant 1: Hazard Proximity Invariant
	// A survivor near fire/flood scores strictly higher than the identical survivor in isolation
	scoreIsolated := Score(ScoringParams{
		Class:      "person",
		Confidence: 0.85,
		Modalities: []string{"rgb"},
	}, cfg)

	scoreNearFire := Score(ScoringParams{
		Class:           "person",
		Confidence:      0.85,
		Modalities:      []string{"rgb"},
		NearHazardClass: "fire",
		HazardDistanceM: 10.0,
	}, cfg)

	scoreNearFlood := Score(ScoringParams{
		Class:           "person",
		Confidence:      0.85,
		Modalities:      []string{"rgb"},
		NearHazardClass: "flood",
		HazardDistanceM: 10.0,
	}, cfg)

	if scoreNearFire <= scoreIsolated {
		t.Errorf("invariant 1 failed: survivor near fire (%f) must score > isolated (%f)", scoreNearFire, scoreIsolated)
	}
	if scoreNearFlood <= scoreIsolated {
		t.Errorf("invariant 1 failed: survivor near flood (%f) must score > isolated (%f)", scoreNearFlood, scoreIsolated)
	}

	// Invariant 2: Modal Agreement Invariant
	// Fused [rgb, thermal] scores strictly higher than [rgb] only at equal confidence
	scoreSingleModal := Score(ScoringParams{
		Class:      "person",
		Confidence: 0.70,
		Modalities: []string{"rgb"},
	}, cfg)

	scoreDualModal := Score(ScoringParams{
		Class:      "person",
		Confidence: 0.70,
		Modalities: []string{"rgb", "thermal"},
	}, cfg)

	if scoreDualModal <= scoreSingleModal {
		t.Errorf("invariant 2 failed: dual modality (%f) must score > single modality (%f)", scoreDualModal, scoreSingleModal)
	}

	// Invariant 3: Class Severity Invariant
	// person > fire > structural_damage > flood > debris
	conf := 0.80
	sPerson := Score(ScoringParams{Class: "person", Confidence: conf}, cfg)
	sFire := Score(ScoringParams{Class: "fire", Confidence: conf}, cfg)
	sStruct := Score(ScoringParams{Class: "structural_damage", Confidence: conf}, cfg)
	sFlood := Score(ScoringParams{Class: "flood", Confidence: conf}, cfg)
	sDebris := Score(ScoringParams{Class: "debris", Confidence: conf}, cfg)

	if !(sPerson > sFire && sFire > sStruct && sStruct > sFlood && sFlood > sDebris) {
		t.Errorf("invariant 3 failed: class hierarchy violated: person(%f) > fire(%f) > struct(%f) > flood(%f) > debris(%f)",
			sPerson, sFire, sStruct, sFlood, sDebris)
	}

	// Invariant 4: Confidence Monotonicity
	sConfLow := Score(ScoringParams{Class: "person", Confidence: 0.50}, cfg)
	sConfHigh := Score(ScoringParams{Class: "person", Confidence: 0.90}, cfg)
	if sConfHigh <= sConfLow {
		t.Errorf("invariant 4 failed: higher confidence (%f) must yield >= score than lower (%f)", sConfHigh, sConfLow)
	}

	// Invariant 5: Persistence Monotonicity
	sObs1 := Score(ScoringParams{Class: "person", Confidence: 0.70, ObservationCount: 1}, cfg)
	sObs5 := Score(ScoringParams{Class: "person", Confidence: 0.70, ObservationCount: 5}, cfg)
	if sObs5 <= sObs1 {
		t.Errorf("invariant 5 failed: persistent entity (%f) must score > single observation (%f)", sObs5, sObs1)
	}

	// Invariant 6: Range Boundaries [0, 1]
	scores := []float64{scoreIsolated, scoreNearFire, scoreDualModal, sPerson, sFire, sStruct, sFlood, sDebris, sObs5}
	for _, s := range scores {
		if s < 0.0 || s > 1.0 {
			t.Errorf("invariant 6 failed: score %f outside [0.0, 1.0]", s)
		}
	}
}
