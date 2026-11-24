package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RecomputeHazardPolygons aggregates active hazard entities into convex-hull hazard polygons.
func RecomputeHazardPolygons(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO hazard_polygons (id, class, geom, area_sq_m, detection_count, updated_at)
		SELECT
			'hazard_poly_' || class,
			class,
			ST_Buffer(ST_ConvexHull(ST_Collect(geom)), 0.00015),
			ST_Area(ST_Buffer(ST_ConvexHull(ST_Collect(geom)), 0.00015)::geography),
			count(*),
			now()
		FROM entities
		WHERE type = 'hazard' AND status = 'active'
		GROUP BY class
		ON CONFLICT (id) DO UPDATE SET
			geom = EXCLUDED.geom,
			area_sq_m = EXCLUDED.area_sq_m,
			detection_count = EXCLUDED.detection_count,
			updated_at = now()`,
	)
	if err != nil {
		return fmt.Errorf("recompute hazard polygons: %w", err)
	}
	return nil
}

// GetHazardsGeoJSON returns a GeoJSON FeatureCollection of all active hazard polygons and points.
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
						'area_sq_m', t.area_sq_m,
						'detection_count', t.detection_count,
						'feature_type', t.feature_type
					)
				)
			), '[]'::json)
		)::text
		FROM (
			SELECT id, class, area_sq_m, detection_count, 'polygon' as feature_type, geom
			FROM hazard_polygons
			UNION ALL
			SELECT id, class, 0.0 as area_sq_m, observation_count as detection_count, 'point' as feature_type, geom
			FROM entities
			WHERE type = 'hazard' AND status = 'active'
		) t`,
	).Scan(&jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("get hazards geojson: %w", err)
	}
	return jsonBytes, nil
}
