// Package dto holds the JSON-facing shapes shared between the REST API and
// the /ws/live broadcast, so both surfaces serialize a fused detection
// identically.
package dto

import "time"

// Status values for DetectionRecord.Status. REST responses (always read
// from Postgres) are always StatusSynced; the /ws/live broadcast uses all
// three depending on how the detection was (or wasn't yet) persisted.
const (
	// StatusLive: freshly fused and written straight to Postgres.
	StatusLive = "live"
	// StatusQueued: written to the local offline queue only — Postgres is
	// (or was, at write time) unreachable.
	StatusQueued = "queued"
	// StatusSyncedFromOfflineQueue: drained from the local offline queue
	// into Postgres now that connectivity has returned.
	StatusSyncedFromOfflineQueue = "synced-from-offline-queue"
	// StatusSynced: already durably in Postgres (how REST reads report it,
	// regardless of whether it originally arrived live or via the queue).
	StatusSynced = "synced"
)

type DetectionRecord struct {
	ID            string    `json:"id"`
	Class         string    `json:"class"`
	Confidence    float64   `json:"confidence"`
	Lat           float64   `json:"lat"`
	Lon           float64   `json:"lon"`
	FrameID       string    `json:"frame_id"`
	Source        string    `json:"source"`
	DetectedAt    time.Time `json:"detected_at"`
	PriorityScore float64   `json:"priority_score"`
	Status        string    `json:"status"`
}
