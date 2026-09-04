package handlers

import (
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
)

// ListAlerts handles GET /alerts?limit= — most recent first.
func ListAlerts(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultListLimit
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxListLimit {
				limit = n
			}
		}

		rows, err := db.ListAlerts(r.Context(), pool, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := make([]dto.AlertRecord, len(rows))
		for i, a := range rows {
			out[i] = dto.AlertRecord{
				ID:           a.ID,
				DetectionID:  a.DetectionID,
				PriorityTier: a.PriorityTier,
				Message:      a.Message,
				CreatedAt:    a.CreatedAt,
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}
