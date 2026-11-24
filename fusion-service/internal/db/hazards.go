package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultBufferMetersPerClass provides sensible safety buffers per hazard type in meters.
var DefaultBufferMetersPerClass = map[string]float64{
	"fire":              50.0,
	"flood":             30.0,
	"structural_damage": 25.0,
	"debris":            20.0,
	"power_line":        25.0,
}

// SeverityPerClass maps hazard classes to their operational severity level.
var SeverityPerClass = map[string]string{
	"fire":              "critical",
	"flood":             "high",
	"structural_damage": "medium",
	"debris":            "medium",
	"power_line":        "high",
}

// RecomputeHazardPolygons aggregates active hazard entities into buffered hazard polygons with severity.
func RecomputeHazardPolygons(ctx context.Context, pool *pgxpool.Pool) error {
	return RecomputeHazardPolygonsWithBuffers(ctx, pool, DefaultBufferMetersPerClass)
}

// RecomputeHazardPolygonsWithBuffers aggregates active hazard entities into buffered polygons using custom buffers.
func RecomputeHazardPolygonsWithBuffers(ctx context.Context, pool *pgxpool.Pool, buffers map[string]float64) error {
	classes := []string{"fire", "flood", "debris", "structural_damage"}
	for _, class := range classes {
		buf := buffers[class]
		if buf <= 0 {
			buf = DefaultBufferMetersPerClass[class]
			if buf <= 0 {
				buf = 20.0
			}
		}
		sev := SeverityPerClass[class]
		if sev == "" {
			sev = "high"
		}

		query := `
			INSERT INTO hazard_polygons (id, class, severity, buffer_meters, geom, area_sq_m, detection_count, updated_at)
			SELECT
				'hazard_poly_' || class,
				class,
				$2,
				$3,
				ST_Buffer(ST_ConvexHull(ST_Collect(geom))::geography, $3)::geometry,
				ST_Area(ST_Buffer(ST_ConvexHull(ST_Collect(geom))::geography, $3)),
				count(*),
				now()
			FROM entities
			WHERE type = 'hazard' AND status = 'active' AND class = $1
			GROUP BY class
			ON CONFLICT (id) DO UPDATE SET
				severity = EXCLUDED.severity,
				buffer_meters = EXCLUDED.buffer_meters,
				geom = EXCLUDED.geom,
				area_sq_m = EXCLUDED.area_sq_m,
				detection_count = EXCLUDED.detection_count,
				updated_at = now()`

		_, err := pool.Exec(ctx, query, class, sev, buf)
		if err != nil {
			return fmt.Errorf("recompute hazard polygon for %s: %w", class, err)
		}
	}
	return nil
}

// ActiveHazardZoneRow represents a hazard zone read from PostGIS.
type ActiveHazardZoneRow struct {
	ID           string
	Class        string
	Severity     string
	BufferMeters float64
	GeoJSON      string
}

// ListActiveHazardPolygons loads all active hazard polygons from hazard_polygons table.
func ListActiveHazardPolygons(ctx context.Context, pool *pgxpool.Pool) ([]ActiveHazardZoneRow, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, class, severity, buffer_meters, ST_AsGeoJSON(geom)
		FROM hazard_polygons
		WHERE ST_GeometryType(geom) IN ('ST_Polygon', 'ST_MultiPolygon')`)
	if err != nil {
		return nil, fmt.Errorf("list active hazard polygons: %w", err)
	}
	defer rows.Close()

	var results []ActiveHazardZoneRow
	for rows.Next() {
		var r ActiveHazardZoneRow
		if err := rows.Scan(&r.ID, &r.Class, &r.Severity, &r.BufferMeters, &r.GeoJSON); err != nil {
			return nil, fmt.Errorf("scan hazard polygon: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

func GetHazardsGeoJSON(ctx context.Context, pool *pgxpool.Pool) ([]byte, error) {
	var jsonBytes []byte
	err := pool.QueryRow(ctx, `
		SELECT json_build_object(
			'type', 'FeatureCollection',
			'features', COALESCE(json_agg(
				json_build_object(
					'type', 'Feature',
					'id', t.id,
					'geometry', ST_AsGeoJSON(t.geom)::json,
					'properties', json_build_object(
						'id', t.id,
						'class', t.class,
						'severity', t.severity,
						'buffer_meters', t.buffer_meters,
						'area_sq_m', t.area_sq_m,
						'detection_count', t.detection_count,
						'feature_type', t.feature_type
					)
				)
			), '[]'::json)
		)::text
		FROM (
			SELECT
				id,
				class,
				severity,
				buffer_meters,
				area_sq_m,
				detection_count,
				'polygon' as feature_type,
				geom
			FROM hazard_polygons
			UNION ALL
			SELECT
				id,
				class,
				CASE class
					WHEN 'fire' THEN 'critical'
					WHEN 'flood' THEN 'high'
					ELSE 'medium'
				END as severity,
				0.0 as buffer_meters,
				0.0 as area_sq_m,
				observation_count as detection_count,
				'point' as feature_type,
				geom
			FROM entities
			WHERE type = 'hazard' AND status = 'active'
		) t`,
	).Scan(&jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("get hazards geojson: %w", err)
	}
	return jsonBytes, nil
}
