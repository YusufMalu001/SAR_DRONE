package coverage

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCoverageGrid_StateTransitions(t *testing.T) {
	// 100m x 100m bounding box in lat/lon
	minLat, minLon := 20.000, 78.000
	maxLat, maxLon := 20.002, 78.002
	grid := NewGrid(minLat, minLon, maxLat, maxLon, 25.0)

	t0 := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	// Step 1: Initial state - all cells unobserved
	summary0 := grid.Summary(t0)
	if summary0.TotalCells == 0 {
		t.Fatal("expected non-zero total cells")
	}
	if summary0.SearchedCells != 0 {
		t.Fatalf("expected 0 searched cells initially, got %d", summary0.SearchedCells)
	}

	testCell := grid.Cells[0][0]
	if testCell.ObservationCount != 0 || testCell.Searched {
		t.Fatalf("cell should be unobserved, got count=%d searched=%v", testCell.ObservationCount, testCell.Searched)
	}

	// Step 2: First pass (altitude 50m, over cell 0,0)
	droneLat := (testCell.MinLat + testCell.MaxLat) / 2.0
	droneLon := (testCell.MinLon + testCell.MaxLon) / 2.0
	grid.RecordTelemetry(droneLat, droneLon, 50.0, 90.0, 60.0, t0)

	if testCell.ObservationCount != 1 {
		t.Fatalf("expected observation count 1, got %d", testCell.ObservationCount)
	}
	if testCell.Searched {
		t.Fatalf("expected searched to be false after only 1 observation (min=2), got true")
	}

	// Step 3: Second pass 10 seconds later -> State transitions to searched=true
	t1 := t0.Add(10 * time.Second)
	grid.RecordTelemetry(droneLat, droneLon, 50.0, 90.0, 60.0, t1)

	if testCell.ObservationCount != 2 {
		t.Fatalf("expected observation count 2, got %d", testCell.ObservationCount)
	}
	if !testCell.Searched {
		t.Fatalf("expected searched to be true after 2 observations, got false")
	}

	summary1 := grid.Summary(t1)
	if summary1.SearchedCells == 0 {
		t.Fatal("expected at least 1 searched cell in summary")
	}

	// Step 4: Record a detection in cell 0,0
	grid.RecordDetection(droneLat, droneLon, 0.94)
	if testCell.BestConfidence != 0.94 {
		t.Fatalf("expected best confidence 0.94, got %f", testCell.BestConfidence)
	}

	// Step 5: Advance time past staleness threshold (e.g. 650 seconds later)
	tStale := t1.Add(650 * time.Second)
	grid.EvaluateStaleness(tStale)

	if testCell.Searched {
		t.Fatalf("expected cell to transition to searched=false due to staleness, got true")
	}
	summaryStale := grid.Summary(tStale)
	if summaryStale.SearchedCells != 0 {
		t.Fatalf("expected 0 active searched cells after staleness expiration, got %d", summaryStale.SearchedCells)
	}

	// Step 6: Fresh observation re-activates cell
	tFresh := tStale.Add(5 * time.Second)
	grid.RecordTelemetry(droneLat, droneLon, 50.0, 90.0, 60.0, tFresh)
	if !testCell.Searched {
		t.Fatalf("expected re-observed cell to transition back to searched=true")
	}

	// Step 7: Verify GeoJSON export format
	geojsonBytes, err := grid.ToGeoJSON(tFresh)
	if err != nil {
		t.Fatalf("failed to export coverage GeoJSON: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(geojsonBytes, &parsed); err != nil {
		t.Fatalf("invalid GeoJSON JSON: %v", err)
	}
	if parsed["type"] != "FeatureCollection" {
		t.Fatalf("expected FeatureCollection, got %v", parsed["type"])
	}
}
