// Package route computes a safe-route heuristic: which segments of the
// drone's recorded flight path pass close enough to a hazard-class
// detection to be flagged "obstructed".
package route

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/geo"
)

const (
	StatusClear      = "clear"
	StatusObstructed = "obstructed"
)

// Waypoint is one point on the flight path, as reported in a Status.
type Waypoint struct {
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Segment is the leg of the flight path between two consecutive recorded
// telemetry points.
type Segment struct {
	Index              int       `json:"index"`
	Start              Waypoint  `json:"start"`
	End                Waypoint  `json:"end"`
	Status             string    `json:"status"` // "clear" | "obstructed"
	NearestHazardID    string    `json:"nearest_hazard_id,omitempty"`
	NearestHazardClass string    `json:"nearest_hazard_class,omitempty"`
	DistanceMeters     *float64  `json:"distance_meters,omitempty"`
}

// Status is the full GET /mission/route-status response.
type Status struct {
	TotalSegments      int       `json:"total_segments"`
	ObstructedSegments int       `json:"obstructed_segments"`
	Segments           []Segment `json:"segments"`
}

// Compute builds the route status from the recorded flight path (up to
// telemetryLimit points, oldest first) and every persisted hazard-class
// detection: any segment passing within obstructionRadiusMeters of a
// hazard is flagged "obstructed".
func Compute(ctx context.Context, pool *pgxpool.Pool, obstructionRadiusMeters float64, telemetryLimit int) (*Status, error) {
	path, err := db.ListTelemetry(ctx, pool, telemetryLimit)
	if err != nil {
		return nil, fmt.Errorf("route status: %w", err)
	}

	hazards, err := db.ListHazardDetections(ctx, pool)
	if err != nil {
		return nil, fmt.Errorf("route status: %w", err)
	}

	status := &Status{}
	if len(path) < 2 {
		return status, nil // need at least two points to form a segment
	}

	for i := 0; i < len(path)-1; i++ {
		a := geo.Point{Lat: path[i].Lat, Lon: path[i].Lon}
		b := geo.Point{Lat: path[i+1].Lat, Lon: path[i+1].Lon}

		seg := Segment{
			Index:  i,
			Start:  Waypoint{Lat: a.Lat, Lon: a.Lon, RecordedAt: path[i].RecordedAt},
			End:    Waypoint{Lat: b.Lat, Lon: b.Lon, RecordedAt: path[i+1].RecordedAt},
			Status: StatusClear,
		}

		nearestIdx := -1
		nearestDist := 0.0
		for hi, h := range hazards {
			d := geo.PointToSegmentMeters(geo.Point{Lat: h.Lat, Lon: h.Lon}, a, b)
			if nearestIdx == -1 || d < nearestDist {
				nearestIdx, nearestDist = hi, d
			}
		}

		if nearestIdx != -1 {
			dist := nearestDist
			seg.DistanceMeters = &dist
			seg.NearestHazardID = hazards[nearestIdx].ID
			seg.NearestHazardClass = hazards[nearestIdx].Class
			if nearestDist <= obstructionRadiusMeters {
				seg.Status = StatusObstructed
				status.ObstructedSegments++
			}
		}

		status.Segments = append(status.Segments, seg)
	}

	status.TotalSegments = len(status.Segments)
	return status, nil
}
