//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
)

// TestOfflineBuffering_QueuesThenDrainsInOrder publishes 5 detections while
// the connectivity monitor is forced offline, confirms they land in the
// local SQLite queue and NOT in Postgres, then flips connectivity back on
// and confirms all 5 are drained into Postgres in timestamp order within a
// few seconds.
func TestOfflineBuffering_QueuesThenDrainsInOrder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	wsURL := "ws" + strings.TrimPrefix(h.httpServer.URL, "http") + "/ws/live"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	defer wsConn.Close()

	setOffline(t, h.httpServer.URL, true)

	droneLat, droneLon, droneAlt, droneHeading := 20.0, 78.0, 50.0, 90.0
	baseTime := time.Now().UTC().Truncate(time.Millisecond)
	publishTelemetry(t, h.redisClient, h.cfg.TelemetryCh, droneLat, droneLon, droneAlt, droneHeading, baseTime)

	const n = 5
	ids := make([]string, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("offline-test-%s-%d", baseTime.Format("150405.000000000"), i)
		ids[i] = id
		publishDetection(t, h.redisClient, h.cfg.DetectionsCh, detectionPayload{
			id: id, class: "person", confidence: 0.8,
			bboxX: 0.45, bboxY: 0.45, bboxW: 0.1, bboxH: 0.1, // centered bbox: simplest to reason about
			frameID: fmt.Sprintf("frame_%d", i), source: "rgb",
			// Ascending timestamps, all still within the 200ms match
			// tolerance of the single telemetry sample above.
			timestamp: baseTime.Add(time.Duration(i*10) * time.Millisecond),
		})
	}

	// --- confirm all 5 are queued (SQLite) and NOT yet in Postgres ---

	waitForQueueCount(t, h, n, 5*time.Second)

	for _, id := range ids {
		row, err := db.GetDetection(ctx, h.pool, id)
		if err != nil {
			t.Fatalf("query detection %s: %v", id, err)
		}
		if row != nil {
			t.Errorf("detection %s already in postgres while offline", id)
		}
	}

	// Each should also have been broadcast immediately as "queued" so the
	// dashboard can show its offline status right away.
	queuedByID := waitForWebSocketRecords(t, wsConn, ids, dto.StatusQueued, 5*time.Second)
	for _, id := range ids {
		if _, ok := queuedByID[id]; !ok {
			t.Errorf("no %q websocket broadcast seen for %s", dto.StatusQueued, id)
		}
	}

	// --- connectivity returns: drain should apply them in timestamp order ---

	setOffline(t, h.httpServer.URL, false)

	syncedByID := waitForWebSocketRecords(t, wsConn, ids, dto.StatusSyncedFromOfflineQueue, 5*time.Second)

	var previousCreatedAt time.Time
	for i, id := range ids {
		row := waitForDetection(t, ctx, h.pool, id, 5*time.Second)
		if row.PriorityScore <= 0 {
			t.Errorf("detection %s: priority_score = %v, want > 0", id, row.PriorityScore)
		}

		if i > 0 && row.CreatedAt.Before(previousCreatedAt) {
			t.Errorf(
				"detection %s (timestamp order index %d) was persisted before the previous one (created_at %s < %s) — drain did not apply timestamp order",
				id, i, row.CreatedAt, previousCreatedAt,
			)
		}
		previousCreatedAt = row.CreatedAt

		if _, ok := syncedByID[id]; !ok {
			t.Errorf("no %q websocket broadcast seen for %s", dto.StatusSyncedFromOfflineQueue, id)
		}
	}

	if remaining, err := h.queue.Count(ctx); err != nil {
		t.Fatalf("count local queue: %v", err)
	} else if remaining != 0 {
		t.Errorf("local queue still has %d entries after drain, want 0", remaining)
	}
}

func setOffline(t *testing.T, baseURL string, offline bool) {
	t.Helper()

	body, err := json.Marshal(map[string]bool{"offline": offline})
	if err != nil {
		t.Fatalf("marshal set-offline request: %v", err)
	}

	resp, err := http.Post(baseURL+"/debug/set-offline", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /debug/set-offline: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /debug/set-offline: status %d", resp.StatusCode)
	}
}

func waitForQueueCount(t *testing.T, h *harness, want int, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := h.queue.Count(ctx)
		if err != nil {
			t.Fatalf("count local queue: %v", err)
		}
		if n >= want {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("local queue did not reach %d entries within %s", want, timeout)
}

// waitForWebSocketRecords collects incoming records off conn until every id
// in wantIDs has been seen with the given status (or timeout elapses),
// ignoring messages for other ids/statuses (e.g. the "queued" broadcasts
// that precede a "synced-from-offline-queue" one for the same id).
func waitForWebSocketRecords(t *testing.T, conn *websocket.Conn, wantIDs []string, wantStatus string, timeout time.Duration) map[string]dto.DetectionRecord {
	t.Helper()

	remaining := make(map[string]struct{}, len(wantIDs))
	for _, id := range wantIDs {
		remaining[id] = struct{}{}
	}
	found := make(map[string]dto.DetectionRecord, len(wantIDs))

	deadline := time.Now().Add(timeout)
	for len(remaining) > 0 && time.Now().Before(deadline) {
		envelope := readWSEnvelope(t, conn, timeout)
		if envelope.Type != dto.WSMessageDetection {
			continue
		}

		var record dto.DetectionRecord
		if err := json.Unmarshal(envelope.Payload, &record); err != nil {
			t.Fatalf("unmarshal detection payload: %v", err)
		}

		if _, want := remaining[record.ID]; want && record.Status == wantStatus {
			found[record.ID] = record
			delete(remaining, record.ID)
		}
	}

	if len(remaining) > 0 {
		t.Fatalf("timed out waiting for %d websocket records with status %q", len(remaining), wantStatus)
	}
	return found
}
