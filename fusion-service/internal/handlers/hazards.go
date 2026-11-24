package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
)

// HazardsGeoJSON handles GET /hazards/geojson
func HazardsGeoJSON(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
