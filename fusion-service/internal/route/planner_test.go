package route

import (
	"encoding/json"
	"testing"

	"sar-drone-prototype/fusion-service/internal/geo"
)

func TestPlanSafeRoute_AvoidsSyntheticHazard(t *testing.T) {
	// Start at (20.000, 78.000), Dest at (20.004, 78.004)
	start := geo.Point{Lat: 20.000, Lon: 78.000}
	dest := geo.Point{Lat: 20.004, Lon: 78.004}

	// Hazard placed directly on the diagonal path: [20.0015, 78.0015] to [20.0025, 78.0025]
	hazardPoly := []geo.Point{
		{Lat: 20.0015, Lon: 78.0015},
		{Lat: 20.0025, Lon: 78.0015},
		{Lat: 20.0025, Lon: 78.0025},
		{Lat: 20.0015, Lon: 78.0025},
		{Lat: 20.0015, Lon: 78.0015},
	}

	hazards := []HazardZone{
		{
			ID:           "hazard_fire_1",
			Class:        "fire",
			Severity:     "critical",
			BufferMeters: 20.0,
			Polygon:      hazardPoly,
		},
	}

	result := PlanSafeRoute(start, dest, hazards, "route_test_avoid")

	if !result.Safe {
		t.Fatalf("expected safe route to be found, got unsafe: %v", result.Reason)
	}

	if result.LengthMeters <= 0 {
		t.Fatalf("expected positive length, got %f", result.LengthMeters)
	}

	// Verify that none of the smoothed waypoints are inside the hazard
	for _, wp := range result.Waypoints {
		if PointInPolygon(wp, hazardPoly) {
			t.Fatalf("waypoint %v is inside hazard polygon", wp)
		}
	}

	// Verify avoided hazards includes hazard_fire_1
	foundAvoided := false
	for _, id := range result.AvoidedHazards {
		if id == "hazard_fire_1" {
			foundAvoided = true
			break
		}
	}
	if !foundAvoided {
		t.Errorf("expected hazard_fire_1 in AvoidedHazards, got %v", result.AvoidedHazards)
	}

	// Verify GeoJSON feature structure
	feature := result.ToGeoJSONFeature()
	bytes, err := json.Marshal(feature)
	if err != nil {
		t.Fatalf("failed to marshal geojson: %v", err)
	}
	if len(bytes) == 0 {
		t.Fatal("empty geojson output")
	}
}

func TestPlanSafeRoute_ReportsNoSafeRouteWhenBlocked(t *testing.T) {
	start := geo.Point{Lat: 20.000, Lon: 78.000}
	dest := geo.Point{Lat: 20.004, Lon: 78.004}

	// A wall hazard that spans the entire width, leaving no passage
	wallPoly := []geo.Point{
		{Lat: 19.998, Lon: 77.995},
		{Lat: 20.006, Lon: 77.995},
		{Lat: 20.006, Lon: 78.008},
		{Lat: 19.998, Lon: 78.008},
		{Lat: 19.998, Lon: 77.995},
	}

	hazards := []HazardZone{
		{
			ID:           "hazard_flood_wall",
			Class:        "flood",
			Severity:     "high",
			BufferMeters: 30.0,
			Polygon:      wallPoly,
		},
	}

	result := PlanSafeRoute(start, dest, hazards, "route_test_blocked")

	if result.Safe {
		t.Fatalf("expected route to be flagged unsafe due to blocking wall, got safe")
	}

	if result.Reason == nil || *result.Reason == "" {
		t.Fatalf("expected non-empty reason when safe=false")
	}

	if result.RiskScore <= 0 {
		t.Fatalf("expected positive risk score for least-risk route, got %f", result.RiskScore)
	}

	// Intersected hazards must list the blocking wall
	foundIntersected := false
	for _, id := range result.IntersectedHazards {
		if id == "hazard_flood_wall" {
			foundIntersected = true
			break
		}
	}
	if !foundIntersected {
		t.Errorf("expected hazard_flood_wall in IntersectedHazards, got %v", result.IntersectedHazards)
	}
}
