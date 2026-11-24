package handlers

import (
	"encoding/json"
	"net/http"

	"sar-drone-prototype/fusion-service/internal/connectivity"
	"sar-drone-prototype/fusion-service/internal/localqueue"
)

type setOfflineRequest struct {
	Offline bool `json:"offline"`
}

// SetOffline handles POST /debug/set-offline {"offline": true|false}.
// Guarded behind allowToggle flag so production builds reject it with 403 Forbidden.
func SetOffline(monitor *connectivity.Monitor, allowToggle bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !allowToggle {
			http.Error(w, "debug endpoints disabled in production", http.StatusForbidden)
			return
		}

		var req setOfflineRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		monitor.SetForcedOffline(req.Offline)
		writeJSON(w, http.StatusOK, map[string]bool{"offline": monitor.IsOffline()})
	}
}

// OfflineStatus handles GET /debug/offline-status.
func OfflineStatus(monitor *connectivity.Monitor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"offline": monitor.IsOffline()})
	}
}

// QueueStatus handles GET /debug/queue — introspection into the local offline queue.
func QueueStatus(queue *localqueue.Queue) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids, err := queue.IDs(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"count": len(ids), "ids": ids})
	}
}
