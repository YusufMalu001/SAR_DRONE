package report

import (
	"context"
	"strings"
	"testing"
	"time"

	"sar-drone-prototype/fusion-service/internal/coverage"
	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/geo"
)

func TestSitRepAndRecommendationOrdering(t *testing.T) {
	clusterer := entity.NewClusterer(entity.DefaultClusterConfig())
	covGrid := coverage.NewGrid(18.50, 73.80, 18.52, 73.82, 20.0)

	t0 := time.Now().UTC().Add(-5 * time.Minute)

	// 1. Ingest two survivors and one hazard
	// Survivor 1: lower priority
	_, _ = clusterer.IngestDetection("det-1", "person", 0.75, geo.Point{Lat: 18.505, Lon: 73.805}, "trk-1", []string{"rgb"}, t0, "", 0.0)
	// Survivor 2: higher priority (near fire hazard)
	_, _ = clusterer.IngestDetection("det-2", "person", 0.95, geo.Point{Lat: 18.510, Lon: 73.810}, "trk-2", []string{"rgb", "thermal"}, t0, "fire", 30.0)
	// Hazard: fire
	_, _ = clusterer.IngestDetection("det-3", "fire", 0.90, geo.Point{Lat: 18.5102, Lon: 73.8102}, "trk-3", []string{"thermal"}, t0, "", 0.0)

	ctx := context.Background()
	sitrep, err := GenerateSitRep(ctx, nil, clusterer, covGrid, nil, "test-mission", 50.0)
	if err != nil {
		t.Fatalf("GenerateSitRep: %v", err)
	}

	if sitrep.TotalSurvivors != 2 {
		t.Fatalf("expected 2 survivors, got %d", sitrep.TotalSurvivors)
	}
	if sitrep.TotalHazards != 1 {
		t.Fatalf("expected 1 hazard, got %d", sitrep.TotalHazards)
	}

	// Recommendations check
	if len(sitrep.Recommendations) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(sitrep.Recommendations))
	}

	// First recommendation must have higher priority score than second
	first := sitrep.Recommendations[0]
	second := sitrep.Recommendations[1]

	if first.PriorityScore < second.PriorityScore {
		t.Fatalf("expected recommendations ordered by priority score descending: first=%.2f second=%.2f", first.PriorityScore, second.PriorityScore)
	}

	// Verify machine-generated rationale template
	if !strings.Contains(first.Rationale, "RGB+thermal") && !strings.Contains(first.Rationale, "fire zone") {
		t.Fatalf("expected rationale to mention cross-modal or hazard zone, got: %s", first.Rationale)
	}

	// Render HTML and assert key elements
	htmlOut := RenderHTML(sitrep)
	if !strings.Contains(htmlOut, "<svg") || !strings.Contains(htmlOut, "</svg>") {
		t.Fatalf("expected HTML to contain static SVG map")
	}
	if !strings.Contains(htmlOut, first.SurvivorID) {
		t.Fatalf("expected HTML to contain survivor ID %s", first.SurvivorID)
	}
	if !strings.Contains(htmlOut, "FIRE ZONE") {
		t.Fatalf("expected HTML to contain FIRE ZONE")
	}
}
