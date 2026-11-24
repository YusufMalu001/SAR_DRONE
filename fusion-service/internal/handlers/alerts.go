package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
)

// ListAlerts handles GET /alerts?limit=&status=&state= — most recent first.
func ListAlerts(pool *pgxpool.Pool, alertMgr *alerting.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultListLimit
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxListLimit {
				limit = n
			}
		}
		status := r.URL.Query().Get("status")
		if status == "" {
			status = r.URL.Query().Get("state")
		}

		rows, err := db.ListAlertsWithFilter(r.Context(), pool, limit, status)
		if err != nil {
			// Fallback to in-memory alerts if DB query fails
			alerts := alertMgr.ListAlerts(alerting.AlertStatus(status))
			out := make([]dto.AlertRecord, len(alerts))
			for i, a := range alerts {
				out[i] = alertToDTO(a)
			}
			writeJSON(w, http.StatusOK, out)
			return
		}

		out := make([]dto.AlertRecord, len(rows))
		for i, a := range rows {
			alertID := a.AlertID
			if alertID == "" {
				alertID = fmt.Sprintf("%d", a.ID)
			}
			out[i] = dto.AlertRecord{
				ID:             alertID,
				NumericID:      a.ID,
				EntityID:       a.EntityID,
				DetectionID:    a.DetectionID,
				PriorityTier:   a.PriorityTier,
				Message:        a.Message,
				PriorityScore:  a.PriorityScore,
				Status:         a.Status,
				CreatedAt:      a.CreatedAt,
				LastNotifiedAt: a.LastNotifiedAt,
				ResolvedAt:     a.ResolvedAt,
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GetAlertByID handles GET /alerts/{id}
func GetAlertByID(pool *pgxpool.Pool, alertMgr *alerting.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if a, ok := alertMgr.GetAlert(id); ok {
			writeJSON(w, http.StatusOK, alertToDTO(*a))
			return
		}

		// Fallback to searching DB
		alerts, err := db.ListAlertsWithFilter(r.Context(), pool, 100, "")
		if err == nil {
			for _, a := range alerts {
				if a.AlertID == id || fmt.Sprintf("%d", a.ID) == id {
					alertID := a.AlertID
					if alertID == "" {
						alertID = fmt.Sprintf("%d", a.ID)
					}
					writeJSON(w, http.StatusOK, dto.AlertRecord{
						ID:             alertID,
						NumericID:      a.ID,
						EntityID:       a.EntityID,
						DetectionID:    a.DetectionID,
						PriorityTier:   a.PriorityTier,
						Message:        a.Message,
						PriorityScore:  a.PriorityScore,
						Status:         a.Status,
						CreatedAt:      a.CreatedAt,
						LastNotifiedAt: a.LastNotifiedAt,
						ResolvedAt:     a.ResolvedAt,
					})
					return
				}
			}
		}

		http.Error(w, "alert not found", http.StatusNotFound)
	}
}

// AckAlert handles POST /alerts/{id}/ack
func AckAlert(pool *pgxpool.Pool, alertMgr *alerting.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		a, ok := alertMgr.AcknowledgeAlertByID(id)
		_ = db.AcknowledgeAlertInDB(r.Context(), pool, id)

		if !ok {
			writeJSON(w, http.StatusOK, map[string]string{"status": "acknowledged", "id": id})
			return
		}

		writeJSON(w, http.StatusOK, alertToDTO(*a))
	}
}

// ResolveAlert handles POST /alerts/{id}/resolve
func ResolveAlert(pool *pgxpool.Pool, alertMgr *alerting.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		a, ok := alertMgr.ResolveAlertByID(id)
		_ = db.ResolveAlertInDB(r.Context(), pool, id)

		if !ok {
			// Still acknowledge if resolved in DB
			writeJSON(w, http.StatusOK, map[string]string{"status": "resolved", "id": id})
			return
		}

		writeJSON(w, http.StatusOK, alertToDTO(*a))
	}
}

func alertToDTO(a alerting.Alert) dto.AlertRecord {
	return dto.AlertRecord{
		ID:             a.ID,
		EntityID:       a.EntityID,
		DetectionID:    a.DetectionID,
		PriorityTier:   a.Tier,
		Message:        a.Message,
		PriorityScore:  a.PriorityScore,
		Status:         string(a.Status),
		CreatedAt:      a.CreatedAt,
		LastNotifiedAt: a.LastNotifiedAt,
		ResolvedAt:     a.ResolvedAt,
	}
}
