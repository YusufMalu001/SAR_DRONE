// Package alerting turns a high-priority fused detection into a short,
// rule-based, human-readable alert.
package alerting

import (
	"fmt"

	"sar-drone-prototype/fusion-service/internal/priority"
)

// AlertThreshold: an alert is generated only for detections scoring
// strictly above this.
const AlertThreshold = 0.7

// Tier buckets a priority score into a human-facing urgency tier. Callers
// should only invoke this for scores > AlertThreshold.
func Tier(score float64) string {
	switch {
	case score >= 0.9:
		return "critical"
	case score >= 0.8:
		return "high"
	default:
		return "medium"
	}
}

// Message composes a short, templated description of the situation: the
// detection's own class, and — for a survivor (person) detection — the
// nearest hazard recorded close enough to matter (nil if there isn't one).
func Message(detectionClass string, hazard *priority.HazardMatch) string {
	if detectionClass == "person" {
		return survivorMessage(hazard)
	}
	return hazardMessage(detectionClass)
}

func survivorMessage(hazard *priority.HazardMatch) string {
	if hazard == nil {
		return "Isolated survivor detected with no nearby hazards."
	}

	distance := fmt.Sprintf("%.0fm", hazard.DistanceMeters)
	switch hazard.Class {
	case "fire":
		return fmt.Sprintf("Survivor detected %s from active fire zone.", distance)
	case "flood":
		return fmt.Sprintf("Survivor detected %s from flooded area.", distance)
	case "structural_damage":
		return fmt.Sprintf("Survivor detected in structurally damaged zone, %s from collapse site.", distance)
	case "debris":
		return fmt.Sprintf("Survivor detected %s from debris field.", distance)
	default:
		return "Isolated survivor detected with no nearby hazards."
	}
}

func hazardMessage(class string) string {
	switch class {
	case "fire":
		return "Active fire detected."
	case "flood":
		return "Flooding detected."
	case "structural_damage":
		return "Structural damage detected."
	case "debris":
		return "Debris field detected."
	default:
		return "High-priority detection recorded."
	}
}
