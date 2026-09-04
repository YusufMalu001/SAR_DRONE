package alerting

import (
	"strings"
	"testing"

	"sar-drone-prototype/fusion-service/internal/priority"
)

// Scenario: survivor near fire.
func TestMessage_SurvivorNearFire(t *testing.T) {
	hazard := &priority.HazardMatch{Class: "fire", DistanceMeters: 12}

	got := Message("person", hazard)

	if !strings.Contains(got, "fire") {
		t.Errorf("Message() = %q, want it to mention fire", got)
	}
	if !strings.Contains(got, "12m") {
		t.Errorf("Message() = %q, want it to mention the 12m distance", got)
	}
}

// Scenario: survivor with no nearby hazard.
func TestMessage_SurvivorNoNearbyHazard(t *testing.T) {
	got := Message("person", nil)

	want := "Isolated survivor detected with no nearby hazards."
	if got != want {
		t.Errorf("Message() = %q, want %q", got, want)
	}
}

func TestMessage_SurvivorNearFlood(t *testing.T) {
	got := Message("person", &priority.HazardMatch{Class: "flood", DistanceMeters: 8})

	if !strings.Contains(got, "flood") {
		t.Errorf("Message() = %q, want it to mention flood", got)
	}
}

func TestMessage_SurvivorInStructurallyDamagedZone(t *testing.T) {
	got := Message("person", &priority.HazardMatch{Class: "structural_damage", DistanceMeters: 3})

	if !strings.Contains(got, "structurally damaged zone") {
		t.Errorf("Message() = %q, want it to mention the structurally damaged zone", got)
	}
}

func TestMessage_SurvivorNearDebris(t *testing.T) {
	got := Message("person", &priority.HazardMatch{Class: "debris", DistanceMeters: 5})

	if !strings.Contains(got, "debris field") {
		t.Errorf("Message() = %q, want it to mention the debris field", got)
	}
}

func TestMessage_HazardClassDetection(t *testing.T) {
	cases := map[string]string{
		"fire":              "Active fire detected.",
		"flood":             "Flooding detected.",
		"structural_damage": "Structural damage detected.",
		"debris":            "Debris field detected.",
	}
	for class, want := range cases {
		if got := Message(class, nil); got != want {
			t.Errorf("Message(%q, nil) = %q, want %q", class, got, want)
		}
	}
}

func TestTier(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{0.95, "critical"},
		{0.9, "critical"},
		{0.85, "high"},
		{0.8, "high"},
		{0.75, "medium"},
		{0.71, "medium"},
	}
	for _, c := range cases {
		if got := Tier(c.score); got != c.want {
			t.Errorf("Tier(%v) = %q, want %q", c.score, got, c.want)
		}
	}
}
