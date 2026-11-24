package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"sar-drone-prototype/fusion-service/internal/coverage"
)

// CoverageGeoJSON handles GET /mission/coverage
func CoverageGeoJSON(grid *coverage.Grid) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		geojson, err := grid.ToGeoJSON(now)
		if err != nil {
			http.Error(w, "failed to generate coverage geojson", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(geojson)
	}
}

// CoverageSummary handles GET /mission/coverage/summary
func CoverageSummary(grid *coverage.Grid) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := time.Now()
		summary := grid.Summary(now)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(summary)
	}
}
