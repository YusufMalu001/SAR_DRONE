package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"sar-drone-prototype/fusion-service/internal/alerting"
	"sar-drone-prototype/fusion-service/internal/db"
	"sar-drone-prototype/fusion-service/internal/dto"
	"sar-drone-prototype/fusion-service/internal/entity"
)

// ListEntities handles GET /entities?type=&status=
func ListEntities(pool *pgxpool.Pool, clusterer *entity.Clusterer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entityType := r.URL.Query().Get("type")
		status := r.URL.Query().Get("status")

		rows, err := db.ListEntities(r.Context(), pool, entityType, status)
		if err != nil {
			// Fallback to in-memory clusterer if DB read fails (e.g. offline)
			entities := clusterer.ListEntities(entity.EntityType(entityType), entity.EntityStatus(status))
			out := make([]dto.EntityRecord, len(entities))
			for i, e := range entities {
				out[i] = entityToDTO(e)
			}
			writeJSON(w, http.StatusOK, out)
			return
		}

		out := make([]dto.EntityRecord, len(rows))
		for i, r := range rows {
			out[i] = entityRowToDTO(r)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GetEntityByID handles GET /entities/{id}
func GetEntityByID(pool *pgxpool.Pool, clusterer *entity.Clusterer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		row, err := db.GetEntity(r.Context(), pool, id)
		if err != nil || row == nil {
			if e, ok := clusterer.GetEntity(id); ok {
				writeJSON(w, http.StatusOK, entityToDTO(*e))
				return
			}
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, entityRowToDTO(*row))
	}
}

type ResolveEntityRequest struct {
	Status string `json:"status"` // rescued | cleared
	Notes  string `json:"notes"`
}

// ResolveEntity handles POST /entities/{id}/resolve
func ResolveEntity(pool *pgxpool.Pool, clusterer *entity.Clusterer, alertMgr *alerting.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req ResolveEntityRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		newStatus := entity.StatusRescued
		if req.Status == "cleared" {
			newStatus = entity.StatusCleared
		}

		// Update in-memory clusterer
		e, ok := clusterer.ResolveEntity(id, newStatus, req.Notes)
		if !ok {
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}

		// Update in Postgres
		_ = db.ResolveEntityInDB(r.Context(), pool, id, string(newStatus), req.Notes)

		// Resolve alerts for this entity
		for _, a := range alertMgr.ListAlerts(alerting.StatusActive) {
			if a.EntityID == id {
				alertMgr.ResolveAlertByID(a.ID)
				_ = db.ResolveAlertInDB(r.Context(), pool, a.ID)
			}
		}

		writeJSON(w, http.StatusOK, entityToDTO(*e))
	}
}

func entityRowToDTO(r db.EntityRow) dto.EntityRecord {
	return dto.EntityRecord{
		ID:                r.ID,
		Type:              r.Type,
		Class:             r.Class,
		Lat:               r.Lat,
		Lon:               r.Lon,
		UncertaintyRadius: r.UncertaintyRadius,
		TrackIDs:          r.TrackIDs,
		ObservationCount:  r.ObservationCount,
		Confidence:        r.Confidence,
		Modalities:        r.Modalities,
		PriorityScore:     r.PriorityScore,
		Status:            r.Status,
		FirstSeen:         r.FirstSeen,
		LastSeen:          r.LastSeen,
		ResolvedAt:        r.ResolvedAt,
		Notes:             r.Notes,
	}
}

func entityToDTO(e entity.Entity) dto.EntityRecord {
	return dto.EntityRecord{
		ID:                e.ID,
		Type:              string(e.Type),
		Class:             e.Class,
		Lat:               e.Lat,
		Lon:               e.Lon,
		UncertaintyRadius: e.UncertaintyRadius,
		TrackIDs:          e.TrackIDs,
		ObservationCount:  e.ObservationCount,
		Confidence:        e.Confidence,
		Modalities:        e.Modalities,
		PriorityScore:     e.PriorityScore,
		Status:            string(e.Status),
		FirstSeen:         e.FirstSeen,
		LastSeen:          e.LastSeen,
		ResolvedAt:        e.ResolvedAt,
		Notes:             e.Notes,
	}
}
