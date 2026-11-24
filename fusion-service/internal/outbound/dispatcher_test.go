package outbound

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/config"
	"sar-drone-prototype/fusion-service/internal/localqueue"
)

func TestOutboundDispatcherOfflineAndRecovery(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "outbound_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "queue.db")
	q, err := localqueue.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer q.Close()

	mockAdapter := NewMockAdapter()
	cfg := config.Config{
		OutboundAdapters: []string{"mock"},
	}
	dispatcher := NewDispatcher(q, cfg)
	dispatcher.Register(mockAdapter)

	ctx := context.Background()
	now := time.Now().UTC()

	alert := alerting.Alert{
		ID:             "alt-999",
		EntityID:       "ent-100",
		DetectionID:    "det-100",
		Tier:           "critical",
		PriorityScore:  0.96,
		Message:        "CRITICAL: person near fire",
		Status:         alerting.StatusActive,
		CreatedAt:      now,
		LastNotifiedAt: now,
	}

	// 1. Enqueue alert while network partition is simulated
	mockAdapter.SetSimulateFailure(true)

	if err := dispatcher.EnqueueAlert(ctx, alert); err != nil {
		t.Fatalf("EnqueueAlert: %v", err)
	}

	// Attempt drain while partition active
	if err := dispatcher.Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	// Should not be delivered yet
	if len(mockAdapter.Delivered()) != 0 {
		t.Fatalf("expected 0 delivered items during partition, got %d", len(mockAdapter.Delivered()))
	}

	// Metrics should show queue_depth >= 1
	metrics, err := q.GetMetrics(ctx)
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if metrics.QueueDepth != 1 {
		t.Fatalf("expected queue depth 1, got %d", metrics.QueueDepth)
	}

	// 2. Network restored: set simulation failure to false, reset next_retry_at so it's eligible
	mockAdapter.SetSimulateFailure(false)

	// Update next_retry_at to past so Drain picks it up immediately
	_, _ = q.ListPendingOutbound(ctx)
	// Force retry time to past
	_ = q.MarkOutboundFailed(ctx, "alt-999-mock", "retry now", time.Now().UTC().Add(-1*time.Second))

	if err := dispatcher.Drain(ctx); err != nil {
		t.Fatalf("Drain after recovery: %v", err)
	}

	// Should now be delivered
	delivered := mockAdapter.Delivered()
	if len(delivered) != 1 {
		t.Fatalf("expected 1 delivered item after recovery, got %d", len(delivered))
	}
	if delivered[0].AlertID != "alt-999" {
		t.Fatalf("expected delivered alert ID alt-999, got %s", delivered[0].AlertID)
	}

	// Queue depth should now be 0
	metricsAfter, _ := q.GetMetrics(ctx)
	if metricsAfter.QueueDepth != 0 {
		t.Fatalf("expected queue depth 0 after recovery, got %d", metricsAfter.QueueDepth)
	}
	if metricsAfter.LastSuccessfulSync == nil {
		t.Fatalf("expected non-nil last successful sync after recovery")
	}
}
