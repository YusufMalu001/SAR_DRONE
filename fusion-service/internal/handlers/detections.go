package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
)

const defaultListLimit = 100
const maxListLimit = 1000

// ListDetections handles GET /detections?limit=&class=
func ListDetections(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultListLimit
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxListLimit {
				limit = n
			}
		}

		rows, err := db.ListDetections(r.Context(), pool, limit, r.URL.Query().Get("class"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, toRecords(rows))
	}
}

// GetDetectionByID handles GET /detections/{id}
func GetDetectionByID(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		row, err := db.GetDetection(r.Context(), pool, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if row == nil {
			http.Error(w, "detection not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, toRecord(*row))
	}
}

// MissionSummary handles GET /mission/summary
func MissionSummary(pool *pgxpool.Pool, highPriorityThreshold float64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		summary, err := db.GetMissionSummary(r.Context(), pool, highPriorityThreshold)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}
}

func toRecord(row db.DetectionRow) dto.DetectionRecord {
	return dto.DetectionRecord{
		ID:            row.ID,
		Class:         row.Class,
		Confidence:    row.Confidence,
		Lat:           row.Lat,
		Lon:           row.Lon,
		FrameID:       row.FrameID,
		Source:        row.Source,
		DetectedAt:    row.DetectedAt,
		PriorityScore: row.PriorityScore,
		Status:        dto.StatusSynced,
	}
}

func toRecords(rows []db.DetectionRow) []dto.DetectionRecord {
	out := make([]dto.DetectionRecord, len(rows))
	for i, row := range rows {
		out[i] = toRecord(row)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
