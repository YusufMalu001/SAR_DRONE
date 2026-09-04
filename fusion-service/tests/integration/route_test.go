//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/route"
)

// TestRouteStatus_FlagsObstructedSegment builds a 3-point flight path
// (two ~55m legs) and a fire detection sitting essentially on top of the
// first waypoint, and verifies GET /mission/route-status flags the first
// leg "obstructed" (fire is well within the 20m radius) and leaves the
// second leg "clear" (fire is ~55m away from it).
func TestRouteStatus_FlagsObstructedSegment(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	baseline, err := db.ListTelemetry(ctx, h.pool, 1_000_000)
	if err != nil {
		t.Fatalf("list telemetry (baseline): %v", err)
	}
	baselineCount := len(baseline)

	baseTime := time.Now().UTC().Truncate(time.Millisecond)
	const alt, heading = 50.0, 0.0

	// ~0.0005 degrees latitude is ~55.7m; distinct decimal values keep
	// these waypoints unambiguous against whatever other tests have left
	// in the (shared, never-cleared) telemetry table.
	p0Lat, p0Lon := 20.11000, 78.11000
	p1Lat, p1Lon := 20.11050, 78.11000
	p2Lat, p2Lon := 20.11100, 78.11000

	publishTelemetry(t, h.redisClient, h.cfg.TelemetryCh, p0Lat, p0Lon, alt, heading, baseTime)
	publishTelemetry(t, h.redisClient, h.cfg.TelemetryCh, p1Lat, p1Lon, alt, heading, baseTime.Add(2*time.Second))
	publishTelemetry(t, h.redisClient, h.cfg.TelemetryCh, p2Lat, p2Lon, alt, heading, baseTime.Add(4*time.Second))

	waitForTelemetryCount(t, ctx, h.pool, baselineCount+3, 5*time.Second)

	// Fire detection matched to p0's telemetry sample (its timestamp is far
	// closer to p0's than to p1's/p2's) with a centered bbox, so it
	// projects to exactly p0's lat/lon — on top of segment 0 (p0->p1) and
	// ~55m from segment 1 (p1->p2).
	fireID := "route-test-fire-" + baseTime.Format("150405.000000000")
	publishDetection(t, h.redisClient, h.cfg.DetectionsCh, detectionPayload{
		id: fireID, class: "fire", confidence: 0.9,
		bboxX: 0.45, bboxY: 0.45, bboxW: 0.1, bboxH: 0.1,
		frameID: "frame_fire", source: "rgb",
		timestamp: baseTime.Add(10 * time.Millisecond),
	})
	waitForDetection(t, ctx, h.pool, fireID, 5*time.Second)

	status := waitForRouteStatusSegments(t, h.httpServer.URL, p0Lat, p1Lat, p2Lat, 5*time.Second)

	seg0, seg1 := findSegments(status, p0Lat, p1Lat, p2Lat)
	if seg0 == nil {
		t.Fatalf("could not find segment %v->%v in route status", p0Lat, p1Lat)
	}
	if seg1 == nil {
		t.Fatalf("could not find segment %v->%v in route status", p1Lat, p2Lat)
	}

	if seg0.Status != route.StatusObstructed {
		t.Errorf("segment p0->p1 status = %q, want %q (distance %s)", seg0.Status, route.StatusObstructed, formatDistance(seg0.DistanceMeters))
	}
	if seg0.NearestHazardID != fireID {
		t.Errorf("segment p0->p1 nearest_hazard_id = %q, want %q", seg0.NearestHazardID, fireID)
	}
	if seg0.NearestHazardClass != "fire" {
		t.Errorf("segment p0->p1 nearest_hazard_class = %q, want %q", seg0.NearestHazardClass, "fire")
	}

	if seg1.Status != route.StatusClear {
		t.Errorf("segment p1->p2 status = %q, want %q (distance %s)", seg1.Status, route.StatusClear, formatDistance(seg1.DistanceMeters))
	}
}

func waitForTelemetryCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rows, err := db.ListTelemetry(ctx, pool, 1_000_000)
		if err != nil {
			t.Fatalf("list telemetry: %v", err)
		}
		if len(rows) >= want {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("telemetry did not reach %d rows within %s", want, timeout)
}

// waitForRouteStatusSegments polls GET /mission/route-status until both
// expected segments (identified by matching start/end latitudes) show up —
// the fire detection lands in Postgres slightly before route-status
// reflects it, since they're independent reads.
func waitForRouteStatusSegments(t *testing.T, baseURL string, p0Lat, p1Lat, p2Lat float64, timeout time.Duration) route.Status {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := fetchRouteStatus(t, baseURL)
		seg0, seg1 := findSegments(status, p0Lat, p1Lat, p2Lat)
		if seg0 != nil && seg1 != nil {
			return status
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("route status did not include the expected segments within %s", timeout)
	return route.Status{}
}

func fetchRouteStatus(t *testing.T, baseURL string) route.Status {
	t.Helper()

	resp, err := http.Get(baseURL + "/mission/route-status")
	if err != nil {
		t.Fatalf("GET /mission/route-status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /mission/route-status: status %d", resp.StatusCode)
	}

	var status route.Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode route status: %v", err)
	}
	return status
}

func findSegments(status route.Status, p0Lat, p1Lat, p2Lat float64) (seg0, seg1 *route.Segment) {
	for i := range status.Segments {
		s := &status.Segments[i]
		if almostEqual(s.Start.Lat, p0Lat) && almostEqual(s.End.Lat, p1Lat) {
			seg0 = s
		}
		if almostEqual(s.Start.Lat, p1Lat) && almostEqual(s.End.Lat, p2Lat) {
			seg1 = s
		}
	}
	return seg0, seg1
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func formatDistance(d *float64) string {
	if d == nil {
		return "none"
	}
	return strconv.FormatFloat(*d, 'f', 1, 64) + "m"
}
