package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/coverage"
	"sar-drone-prototype/fusion-service/internal/entity"
	"sar-drone-prototype/fusion-service/internal/report"
)

// MissionReport handles GET /mission/report and GET /missions/{id}/report.
// Returns JSON by default or formatted printable HTML when format=html or Accept: text/html.
func MissionReport(
	pool *pgxpool.Pool,
	clusterer *entity.Clusterer,
	covGrid *coverage.Grid,
	monitor *connectivity.Monitor,
	obstructionMeters float64,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		missionID := r.PathValue("id")
		if missionID == "" {
			missionID = r.URL.Query().Get("mission_id")
		}

		sitrep, err := report.GenerateSitRep(r.Context(), pool, clusterer, covGrid, monitor, missionID, obstructionMeters)
		if err != nil {
			http.Error(w, "generate report: "+err.Error(), http.StatusInternalServerError)
			return
		}

		format := strings.ToLower(r.URL.Query().Get("format"))
		accept := r.Header.Get("Accept")

		if format == "html" || (format == "" && strings.Contains(accept, "text/html")) {
			htmlContent := report.RenderHTML(sitrep)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(htmlContent))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(sitrep)
	}
}
