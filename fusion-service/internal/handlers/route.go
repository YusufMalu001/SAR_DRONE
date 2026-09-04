package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/route"
)

// RouteStatus handles GET /mission/route-status.
func RouteStatus(pool *pgxpool.Pool, obstructionMeters float64, telemetryLimit int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, err := route.Compute(r.Context(), pool, obstructionMeters, telemetryLimit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, status)
	}
}
