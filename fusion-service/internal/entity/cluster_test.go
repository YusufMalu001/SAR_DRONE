package entity

import (
	"testing"
	"time"

	"sar-drone-prototype/fusion-service/internal/geo"
)

func TestClusterer_SameTrackAssociation(t *testing.T) {
	c := NewClusterer(DefaultClusterConfig())
	now := time.Now()

	pt1 := geo.Point{Lat: 20.0020, Lon: 78.0020}
	e1, isNew := c.IngestDetection("d1", "person", 0.85, pt1, "track_101", []string{"rgb"}, now, "", 0)
	if !isNew {
		t.Fatalf("first detection must create new entity")
	}
	if e1.ObservationCount != 1 || e1.Type != TypeSurvivor {
		t.Errorf("unexpected entity: %+v", e1)
	}

	// Second detection with same track_id, slight position shift
	pt2 := geo.Point{Lat: 20.00201, Lon: 78.00201}
	e2, isNew := c.IngestDetection("d2", "person", 0.90, pt2, "track_101", []string{"thermal"}, now.Add(200*time.Millisecond), "", 0)
	if isNew {
		t.Fatalf("second detection with same track_id must merge, not create new entity")
	}
	if e2.ID != e1.ID {
		t.Errorf("expected entity ID %s, got %s", e1.ID, e2.ID)
	}
	if e2.ObservationCount != 2 {
		t.Errorf("expected observation count 2, got %d", e2.ObservationCount)
	}
	if len(e2.Modalities) != 2 {
		t.Errorf("expected 2 merged modalities, got %v", e2.Modalities)
	}
	if e2.UncertaintyRadius >= e1.UncertaintyRadius {
		t.Errorf("uncertainty radius must decrease with more observations (was %f, now %f)",
			e1.UncertaintyRadius, e2.UncertaintyRadius)
	}
}

func TestClusterer_CrossPassSpatialClustering(t *testing.T) {
	c := NewClusterer(DefaultClusterConfig())
	now := time.Now()

	// Pass 1: Survivor seen with track_A
	pt1 := geo.Point{Lat: 20.00250, Lon: 78.00250}
	e1, isNew := c.IngestDetection("d1", "person", 0.80, pt1, "track_A", []string{"rgb"}, now, "", 0)
	if !isNew {
		t.Fatal("expected new entity")
	}

	// Pass 2 (30 seconds later, drone returns): new track_B assigned, 2.5 meters away
	pt2 := geo.Point{Lat: 20.00252, Lon: 78.00250}
	dist := geo.HaversineMeters(pt1, pt2)
	if dist > 4.0 {
		t.Fatalf("setup error: dist should be < 4m, got %f", dist)
	}

	e2, isNew := c.IngestDetection("d2", "person", 0.88, pt2, "track_B", []string{"thermal"}, now.Add(30*time.Second), "", 0)
	if isNew {
		t.Fatalf("expected cross-pass reacquisition within 15m to merge, but created new entity")
	}
	if e2.ID != e1.ID {
		t.Errorf("expected same entity ID, got %s != %s", e2.ID, e1.ID)
	}
	if len(e2.TrackIDs) != 2 {
		t.Errorf("expected entity to store both track IDs [track_A, track_B], got %v", e2.TrackIDs)
	}
}

func TestClusterer_DistinctSurvivorsKeptSeparate(t *testing.T) {
	c := NewClusterer(DefaultClusterConfig())
	now := time.Now()

	// Rooftop survivor 1
	pt1 := geo.Point{Lat: 20.0025, Lon: 78.0025}
	e1, _ := c.IngestDetection("d1", "person", 0.90, pt1, "track_1", []string{"rgb"}, now, "", 0)

	// Rooftop survivor 2 (~75 meters away, as in flood scenario)
	pt2 := geo.Point{Lat: 20.0030, Lon: 78.0030}
	dist := geo.HaversineMeters(pt1, pt2)
	if dist < 50.0 {
		t.Fatalf("setup error: distance should be > 50m, got %f", dist)
	}

	e2, isNew := c.IngestDetection("d2", "person", 0.85, pt2, "track_2", []string{"rgb"}, now, "", 0)
	if !isNew {
		t.Fatalf("distinct survivor 75m away must NOT be merged!")
	}
	if e1.ID == e2.ID {
		t.Errorf("survivors should have distinct IDs")
	}

	survivors := c.ListEntities(TypeSurvivor, StatusActive)
	if len(survivors) != 2 {
		t.Errorf("expected exactly 2 active survivors, got %d", len(survivors))
	}
}

func TestClusterer_Resolution(t *testing.T) {
	c := NewClusterer(DefaultClusterConfig())
	pt := geo.Point{Lat: 20.0020, Lon: 78.0020}
	e, _ := c.IngestDetection("d1", "person", 0.9, pt, "track_1", []string{"rgb"}, time.Now(), "", 0)

	resolved, ok := c.ResolveEntity(e.ID, StatusRescued, "Evacuated via helicopter hoist")
	if !ok {
		t.Fatalf("failed to resolve entity")
	}
	if resolved.Status != StatusRescued || resolved.Notes != "Evacuated via helicopter hoist" {
		t.Errorf("unexpected resolved entity: %+v", resolved)
	}
	if resolved.ResolvedAt == nil {
		t.Errorf("resolved_at timestamp must be set")
	}

	// Active survivors query should now return 0
	active := c.ListEntities(TypeSurvivor, StatusActive)
	if len(active) != 0 {
		t.Errorf("expected 0 active survivors after resolution, got %d", len(active))
	}
}
