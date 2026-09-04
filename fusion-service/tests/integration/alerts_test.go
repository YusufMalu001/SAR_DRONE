//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
)

// TestAlertGeneration_SurvivorNearFire publishes a fire detection followed
// by a high-confidence person detection at the same location and verifies
// an Alert is generated: persisted in Postgres, listed via GET /alerts, and
// pushed over /ws/live as an "alert" envelope naming the fire and its
// distance.
func TestAlertGeneration_SurvivorNearFire(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	wsURL := "ws" + strings.TrimPrefix(h.httpServer.URL, "http") + "/ws/live"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	defer wsConn.Close()

	baseTime := time.Now().UTC().Truncate(time.Millisecond)
	droneLat, droneLon, droneAlt, droneHeading := 20.0, 78.0, 50.0, 90.0
	publishTelemetry(t, h.redisClient, h.cfg.TelemetryCh, droneLat, droneLon, droneAlt, droneHeading, baseTime)

	suffix := baseTime.Format("150405.000000000")

	// Fire detection first, at the drone's exact position (centered bbox),
	// so it's recorded as a nearby hazard before the survivor is processed.
	fireID := "alert-test-fire-" + suffix
	publishDetection(t, h.redisClient, h.cfg.DetectionsCh, detectionPayload{
		id: fireID, class: "fire", confidence: 0.9,
		bboxX: 0.45, bboxY: 0.45, bboxW: 0.1, bboxH: 0.1,
		frameID: "frame_fire", source: "rgb",
		timestamp: baseTime.Add(10 * time.Millisecond),
	})
	waitForDetection(t, ctx, h.pool, fireID, 5*time.Second)

	// Survivor at (effectively) the same position, high confidence so the
	// resulting priority score clears both the alert threshold (0.7) and
	// the "critical" tier (0.9): base 1.0 * confidence 0.95 + 0.2 proximity
	// boost, clamped to 1.0.
	personID := "alert-test-person-" + suffix
	publishDetection(t, h.redisClient, h.cfg.DetectionsCh, detectionPayload{
		id: personID, class: "person", confidence: 0.95,
		bboxX: 0.45, bboxY: 0.45, bboxW: 0.1, bboxH: 0.1,
		frameID: "frame_person", source: "rgb",
		timestamp: baseTime.Add(20 * time.Millisecond),
	})
	waitForDetection(t, ctx, h.pool, personID, 5*time.Second)

	// --- persisted in Postgres ---

	alert := waitForAlert(t, ctx, h.pool, personID, 5*time.Second)
	if alert.PriorityTier != "critical" {
		t.Errorf("priority_tier = %q, want %q", alert.PriorityTier, "critical")
	}
	if !strings.Contains(alert.Message, "fire") {
		t.Errorf("message = %q, want it to mention fire", alert.Message)
	}

	// --- reachable via GET /alerts ---

	resp, err := http.Get(h.httpServer.URL + "/alerts")
	if err != nil {
		t.Fatalf("GET /alerts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /alerts: status %d", resp.StatusCode)
	}
	var alerts []dto.AlertRecord
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		t.Fatalf("decode GET /alerts response: %v", err)
	}
	found := false
	for _, a := range alerts {
		if a.DetectionID == personID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GET /alerts did not include an alert for detection %s", personID)
	}

	// --- pushed over /ws/live as a distinct "alert" message type ---

	wsAlert := waitForWebSocketAlert(t, wsConn, personID, 5*time.Second)
	if wsAlert.PriorityTier != "critical" {
		t.Errorf("ws alert priority_tier = %q, want %q", wsAlert.PriorityTier, "critical")
	}
	if !strings.Contains(wsAlert.Message, "fire") {
		t.Errorf("ws alert message = %q, want it to mention fire", wsAlert.Message)
	}
}

// waitForAlert polls Postgres for the alert generated from detectionID to
// appear.
func waitForAlert(t *testing.T, ctx context.Context, pool *pgxpool.Pool, detectionID string, timeout time.Duration) db.Alert {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alert, err := db.GetAlertByDetectionID(ctx, pool, detectionID)
		if err != nil {
			t.Fatalf("query alert for detection %s: %v", detectionID, err)
		}
		if alert != nil {
			return *alert
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("alert for detection %s did not appear in postgres within %s", detectionID, timeout)
	return db.Alert{}
}

func waitForWebSocketAlert(t *testing.T, conn *websocket.Conn, wantDetectionID string, timeout time.Duration) dto.AlertRecord {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		envelope := readWSEnvelope(t, conn, timeout)
		if envelope.Type != dto.WSMessageAlert {
			continue
		}

		var record dto.AlertRecord
		if err := json.Unmarshal(envelope.Payload, &record); err != nil {
			t.Fatalf("unmarshal alert payload: %v", err)
		}
		if record.DetectionID == wantDetectionID {
			return record
		}
	}

	t.Fatalf("did not receive websocket alert for %s within %s", wantDetectionID, timeout)
	return dto.AlertRecord{}
}
