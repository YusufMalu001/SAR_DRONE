package handlers

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
)

// HazardsGeoJSON handles GET /hazards/geojson and GET /hazards/zones
func HazardsGeoJSON(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Optional query param: custom buffer_meters
		if bufStr := r.URL.Query().Get("buffer_meters"); bufStr != "" {
			if buf, err := strconv.ParseFloat(bufStr, 64); err == nil && buf > 0 {
				customBuffers := make(map[string]float64)
				for k := range db.DefaultBufferMetersPerClass {
					customBuffers[k] = buf
				}
				_ = db.RecomputeHazardPolygonsWithBuffers(r.Context(), pool, customBuffers)
			}
		}

		geojson, err := db.GetHazardsGeoJSON(r.Context(), pool)
		if err != nil {
			// Return valid empty FeatureCollection on empty or error
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(geojson)
	}
}
