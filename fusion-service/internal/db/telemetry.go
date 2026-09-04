package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TelemetryRow is one recorded flight-path point.
type TelemetryRow struct {
	Lat        float64
	Lon        float64
	Alt        float64
	Heading    float64
	RecordedAt time.Time
}

// InsertTelemetry durably records a telemetry sample — the recorded flight
// path GET /mission/route-status is computed from.
func InsertTelemetry(ctx context.Context, pool *pgxpool.Pool, t TelemetryRow) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO telemetry (lat, lon, alt, heading, recorded_at)
		VALUES ($1, $2, $3, $4, $5)`,
		t.Lat, t.Lon, t.Alt, t.Heading, t.RecordedAt,
	)
	if err != nil {
		return fmt.Errorf("insert telemetry: %w", err)
	}
	return nil
}

// ListTelemetry returns up to `limit` recorded telemetry samples, ordered
// oldest first (i.e. in flight-path order).
func ListTelemetry(ctx context.Context, pool *pgxpool.Pool, limit int) ([]TelemetryRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT lat, lon, alt, heading, recorded_at
		FROM telemetry
		ORDER BY recorded_at ASC
		LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list telemetry: %w", err)
	}
	defer rows.Close()

	var out []TelemetryRow
	for rows.Next() {
		var t TelemetryRow
		if err := rows.Scan(&t.Lat, &t.Lon, &t.Alt, &t.Heading, &t.RecordedAt); err != nil {
			return nil, fmt.Errorf("scan telemetry row: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// HazardDetectionRow is a hazard-class detection's identity and location,
// used by the route-status obstruction check.
type HazardDetectionRow struct {
	ID    string
	Class string
	Lat   float64
	Lon   float64
}

// ListHazardDetections returns every persisted hazard-class detection
// (fire/flood/debris/structural_damage) — "person" is never a hazard.
func ListHazardDetections(ctx context.Context, pool *pgxpool.Pool) ([]HazardDetectionRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, class, ST_Y(geom), ST_X(geom)
		FROM detections
		WHERE class IN ('fire', 'flood', 'debris', 'structural_damage')`,
	)
	if err != nil {
		return nil, fmt.Errorf("list hazard detections: %w", err)
	}
	defer rows.Close()

	var out []HazardDetectionRow
	for rows.Next() {
		var h HazardDetectionRow
		if err := rows.Scan(&h.ID, &h.Class, &h.Lat, &h.Lon); err != nil {
			return nil, fmt.Errorf("scan hazard detection row: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
