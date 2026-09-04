//go:build integration

// Package integration exercises the full fusion pipeline against real
// Redis and Postgres (with PostGIS) instances.
//
// Run it with:
//
//	docker compose up -d postgres redis
//	go test -tags=integration ./tests/integration/...
//
// DATABASE_URL / REDIS_ADDR env vars override the docker-compose defaults
// (useful in CI where the services might not be on localhost).
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
	"github.com/redis/go-redis/v9"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
	"sar-drone-prototype/fusion-service/internal/geo"
)

const latLonTolerance = 1e-5 // ~1 meter at these latitudes

// TestFusionPipeline_EndToEnd publishes a synthetic telemetry + detection
// pair to Redis and verifies the resulting fused detection lands in
// Postgres with the expected geotag, is reachable via the REST API, and is
// broadcast over /ws/live.
func TestFusionPipeline_EndToEnd(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	wsURL := "ws" + strings.TrimPrefix(h.httpServer.URL, "http") + "/ws/live"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", wsURL, err)
	}
	defer wsConn.Close()

	// --- publish a synthetic telemetry + detection pair ---

	detectionID := "integration-test-" + time.Now().UTC().Format("20060102T150405.000000000")
	baseTime := time.Now().UTC().Truncate(time.Millisecond)

	droneLat, droneLon, droneAlt, droneHeading := 20.0, 78.0, 50.0, 90.0
	publishTelemetry(t, h.redisClient, h.cfg.TelemetryCh, droneLat, droneLon, droneAlt, droneHeading, baseTime)

	// Off-center bbox so the test also exercises the projection math, not
	// just message plumbing. Detection timestamp is 50ms after telemetry —
	// well within the default 200ms match tolerance.
	bboxX, bboxY, bboxW, bboxH := 0.7, 0.6, 0.1, 0.1
	publishDetection(t, h.redisClient, h.cfg.DetectionsCh, detectionPayload{
		id: detectionID, class: "person", confidence: 0.91,
		bboxX: bboxX, bboxY: bboxY, bboxW: bboxW, bboxH: bboxH,
		frameID: "frame_000123", source: "rgb",
		timestamp: baseTime.Add(50 * time.Millisecond),
	})

	expected := geo.ProjectDetection(
		bboxX+bboxW/2, bboxY+bboxH/2,
		h.cfg.CameraHFOVDeg, h.cfg.CameraVFOVDeg,
		droneAlt, droneHeading, droneLat, droneLon,
	)

	// --- assert it landed in Postgres ---

	row := waitForDetection(t, ctx, h.pool, detectionID, 10*time.Second)
	if row.Class != "person" {
		t.Errorf("class = %q, want %q", row.Class, "person")
	}
	if row.Source != "rgb" {
		t.Errorf("source = %q, want %q", row.Source, "rgb")
	}
	assertNear(t, "lat", row.Lat, expected.Lat, latLonTolerance)
	assertNear(t, "lon", row.Lon, expected.Lon, latLonTolerance)
	if row.PriorityScore <= 0 {
		t.Errorf("priority_score = %v, want > 0", row.PriorityScore)
	}

	// --- assert it's reachable via the REST API too ---

	resp, err := http.Get(h.httpServer.URL + "/detections/" + detectionID)
	if err != nil {
		t.Fatalf("GET /detections/%s: %v", detectionID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /detections/%s: status %d", detectionID, resp.StatusCode)
	}
	var apiRecord dto.DetectionRecord
	if err := json.NewDecoder(resp.Body).Decode(&apiRecord); err != nil {
		t.Fatalf("decode API response: %v", err)
	}
	if apiRecord.ID != detectionID {
		t.Errorf("API record id = %q, want %q", apiRecord.ID, detectionID)
	}
	if apiRecord.Status != dto.StatusSynced {
		t.Errorf("API record status = %q, want %q", apiRecord.Status, dto.StatusSynced)
	}

	// --- assert it arrived over the WebSocket, tagged "live" ---

	record := waitForWebSocketRecord(t, wsConn, detectionID, 10*time.Second)
	assertNear(t, "ws lat", record.Lat, expected.Lat, latLonTolerance)
	assertNear(t, "ws lon", record.Lon, expected.Lon, latLonTolerance)
	if record.Status != dto.StatusLive {
		t.Errorf("ws record status = %q, want %q", record.Status, dto.StatusLive)
	}
}

// waitForDetection polls Postgres for the fused detection to appear,
// tolerating the small delay between publishing to Redis and the consumer
// processing + persisting it.
func waitForDetection(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id string, timeout time.Duration) db.DetectionRow {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		row, err := db.GetDetection(ctx, pool, id)
		if err != nil {
			t.Fatalf("query detection %s: %v", id, err)
		}
		if row != nil {
			return *row
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("detection %s did not appear in postgres within %s", id, timeout)
	return db.DetectionRow{}
}

// readWSEnvelope reads one /ws/live message and unwraps its dto.WSMessage
// envelope.
func readWSEnvelope(t *testing.T, conn *websocket.Conn, timeout time.Duration) dto.WSMessage {
	t.Helper()

	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read websocket message: %v", err)
	}

	var envelope dto.WSMessage
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal websocket envelope: %v", err)
	}
	return envelope
}

// waitForWebSocketRecord reads /ws/live messages until a "detection"
// envelope for wantID arrives, ignoring anything else (other ids, or
// "alert" envelopes).
func waitForWebSocketRecord(t *testing.T, conn *websocket.Conn, wantID string, timeout time.Duration) dto.DetectionRecord {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		envelope := readWSEnvelope(t, conn, timeout)
		if envelope.Type != dto.WSMessageDetection {
			continue
		}

		var record dto.DetectionRecord
		if err := json.Unmarshal(envelope.Payload, &record); err != nil {
			t.Fatalf("unmarshal detection payload: %v", err)
		}
		if record.ID == wantID {
			return record
		}
		// Not our message (another test/process's detection) — keep waiting.
	}

	t.Fatalf("did not receive websocket record for %s within %s", wantID, timeout)
	return dto.DetectionRecord{}
}

func assertNear(t *testing.T, label string, got, want, tolerance float64) {
	t.Helper()
	if diff := got - want; diff < -tolerance || diff > tolerance {
		t.Errorf("%s = %v, want %v (±%v)", label, got, want, tolerance)
	}
}

type detectionPayload struct {
	id                         string
	class                      string
	confidence                 float64
	bboxX, bboxY, bboxW, bboxH float64
	frameID                    string
	source                     string
	timestamp                  time.Time
}

func publishTelemetry(t *testing.T, redisClient *redis.Client, stream string, lat, lon, alt, heading float64, at time.Time) {
	t.Helper()
	ctx := context.Background()

	payload, err := json.Marshal(map[string]any{
		"lat": lat, "lon": lon, "alt": alt, "heading": heading,
		"timestamp": at.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal telemetry: %v", err)
	}

	if err := redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"data": payload},
	}).Err(); err != nil {
		t.Fatalf("publish telemetry: %v", err)
	}
}

func publishDetection(t *testing.T, redisClient *redis.Client, stream string, p detectionPayload) {
	t.Helper()
	ctx := context.Background()

	payload, err := json.Marshal(map[string]any{
		"id":         p.id,
		"class":      p.class,
		"confidence": p.confidence,
		"bbox":       map[string]float64{"x": p.bboxX, "y": p.bboxY, "w": p.bboxW, "h": p.bboxH},
		"frame_id":   p.frameID,
		"source":     p.source,
		"timestamp":  p.timestamp.Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal detection: %v", err)
	}

	if err := redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"data": payload},
	}).Err(); err != nil {
		t.Fatalf("publish detection: %v", err)
	}
}
