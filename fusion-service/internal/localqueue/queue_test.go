package localqueue

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sar-drone-prototype/fusion-service/internal/db"
)

func TestLocalQueueDetections(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "queue_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "queue.db")
	q, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer q.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	fd1 := db.FusedDetection{
		ID:            "det-1",
		Class:         "person",
		Confidence:    0.95,
		BBoxX:         100,
		BBoxY:         100,
		BBoxW:         50,
		BBoxH:         80,
		FrameID:       "frame-1",
		Source:        "rgb",
		DetectedAt:    now.Add(-2 * time.Second),
		TelemetryAt:   now.Add(-2 * time.Second),
		Lat:           18.5,
		Lon:           73.8,
		PriorityScore: 0.85,
		TrackID:       "trk-1",
		Modalities:    []string{"rgb", "thermal"},
	}

	fd2 := db.FusedDetection{
		ID:            "det-2",
		Class:         "fire",
		Confidence:    0.88,
		BBoxX:         200,
		BBoxY:         200,
		BBoxW:         60,
		BBoxH:         60,
		FrameID:       "frame-2",
		Source:        "thermal",
		DetectedAt:    now.Add(-1 * time.Second),
		TelemetryAt:   now.Add(-1 * time.Second),
		Lat:           18.5001,
		Lon:           73.8001,
		PriorityScore: 0.9,
		TrackID:       "trk-2",
		Modalities:    []string{"thermal"},
	}

	if err := q.EnqueueDetection(ctx, fd2); err != nil {
		t.Fatalf("EnqueueDetection fd2: %v", err)
	}
	if err := q.EnqueueDetection(ctx, fd1); err != nil {
		t.Fatalf("EnqueueDetection fd1: %v", err)
	}

	count, err := q.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}

	// List ordered should return fd1 first (older detected_at)
	list, err := q.ListOrdered(ctx)
	if err != nil {
		t.Fatalf("ListOrdered: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 items, got %d", len(list))
	}
	if list[0].ID != "det-1" || list[1].ID != "det-2" {
		t.Fatalf("expected [det-1, det-2], got [%s, %s]", list[0].ID, list[1].ID)
	}

	// Delete fd1
	if err := q.DeleteDetection(ctx, "det-1"); err != nil {
		t.Fatalf("DeleteDetection: %v", err)
	}
	count, _ = q.Count(ctx)
	if count != 1 {
		t.Fatalf("expected count 1 after delete, got %d", count)
	}
}

func TestLocalQueueEntitiesAndLWW(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "queue_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "queue.db")
	q, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer q.Close()

	ctx := context.Background()
	t0 := time.Now().UTC().Add(-10 * time.Second)
	t1 := t0.Add(5 * time.Second)

	e1 := db.EntityRow{
		ID:                "ent-1",
		Type:              "survivor",
		Class:             "person",
		Lat:               18.5,
		Lon:               73.8,
		UncertaintyRadius: 5.0,
		TrackIDs:          []string{"trk-1"},
		ObservationCount:  1,
		Confidence:        0.8,
		Modalities:        []string{"rgb"},
		PriorityScore:     0.7,
		Status:            "active",
		FirstSeen:         t0,
		LastSeen:          t0,
	}

	if err := q.EnqueueEntity(ctx, e1); err != nil {
		t.Fatalf("EnqueueEntity: %v", err)
	}

	// Update with newer last_seen (should overwrite)
	e2 := e1
	e2.ObservationCount = 2
	e2.Confidence = 0.92
	e2.Modalities = []string{"rgb", "thermal"}
	e2.LastSeen = t1

	if err := q.EnqueueEntity(ctx, e2); err != nil {
		t.Fatalf("EnqueueEntity update: %v", err)
	}

	entities, err := q.ListOrderedEntities(ctx)
	if err != nil {
		t.Fatalf("ListOrderedEntities: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if entities[0].ObservationCount != 2 || entities[0].Confidence != 0.92 {
		t.Fatalf("expected updated entity, got count=%d conf=%f", entities[0].ObservationCount, entities[0].Confidence)
	}

	// Update with older last_seen (should NOT overwrite due to LWW condition)
	eOlder := e1
	eOlder.ObservationCount = 99
	eOlder.LastSeen = t0.Add(-2 * time.Second)
	_ = q.EnqueueEntity(ctx, eOlder)

	entitiesAfterOlder, _ := q.ListOrderedEntities(ctx)
	if entitiesAfterOlder[0].ObservationCount != 2 {
		t.Fatalf("stale update should have been ignored by LWW")
	}

	// Delete
	if err := q.DeleteEntity(ctx, "ent-1"); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	rem, _ := q.ListOrderedEntities(ctx)
	if len(rem) != 0 {
		t.Fatalf("expected 0 entities, got %d", len(rem))
	}
}

func TestLocalQueueAlertsAndMetrics(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "queue_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "queue.db")
	q, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer q.Close()

	ctx := context.Background()
	now := time.Now().UTC()

	// 1. Alert queue
	alert := db.Alert{
		AlertID:        "alt-1",
		EntityID:       "ent-1",
		DetectionID:    "det-1",
		PriorityTier:   "critical",
		Message:        "Person in hazard zone",
		PriorityScore:  0.95,
		Status:         "active",
		LastNotifiedAt: now,
		CreatedAt:      now,
	}
	if err := q.EnqueueAlert(ctx, alert); err != nil {
		t.Fatalf("EnqueueAlert: %v", err)
	}

	alerts, err := q.ListOrderedAlerts(ctx)
	if err != nil {
		t.Fatalf("ListOrderedAlerts: %v", err)
	}
	if len(alerts) != 1 || alerts[0].AlertID != "alt-1" {
		t.Fatalf("expected 1 alert with ID alt-1")
	}

	// 2. Outbound queue
	outbound := OutboundItem{
		ID:        "out-1",
		AlertID:   "alt-1",
		Adapter:   "webhook",
		Payload:   `{"msg":"test"}`,
		Status:    "pending",
		CreatedAt: now,
	}
	if err := q.EnqueueOutbound(ctx, outbound); err != nil {
		t.Fatalf("EnqueueOutbound: %v", err)
	}

	pending, err := q.ListPendingOutbound(ctx)
	if err != nil {
		t.Fatalf("ListPendingOutbound: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "out-1" {
		t.Fatalf("expected 1 pending outbound")
	}

	// 3. Metrics verification
	metrics, err := q.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	// 1 alert + 1 outbound = 2
	if metrics.QueueDepth != 2 {
		t.Fatalf("expected queue depth 2, got %d", metrics.QueueDepth)
	}
	if metrics.OldestItemAgeSeconds < 0 {
		t.Fatalf("expected oldest item age >= 0, got %f", metrics.OldestItemAgeSeconds)
	}
	if metrics.LastSuccessfulSync != nil {
		t.Fatalf("expected nil last successful sync before sync")
	}

	// Record sync success
	if err := q.RecordSyncSuccess(ctx); err != nil {
		t.Fatalf("RecordSyncSuccess: %v", err)
	}
	metricsAfterSync, _ := q.GetMetrics(ctx)
	if metricsAfterSync.LastSuccessfulSync == nil {
		t.Fatalf("expected non-nil last successful sync after record")
	}

	// Mark outbound sent
	if err := q.MarkOutboundSent(ctx, "out-1"); err != nil {
		t.Fatalf("MarkOutboundSent: %v", err)
	}
	pendingAfterSent, _ := q.ListPendingOutbound(ctx)
	if len(pendingAfterSent) != 0 {
		t.Fatalf("expected 0 pending outbound after sent")
	}
}
