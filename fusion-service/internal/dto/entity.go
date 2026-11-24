package dto

import "time"

type EntityRecord struct {
	ID                string     `json:"id"`
	Type              string     `json:"type"` // survivor | hazard
	Class             string     `json:"class"`
	Lat               float64    `json:"lat"`
	Lon               float64    `json:"lon"`
	UncertaintyRadius float64    `json:"uncertainty_radius"`
	TrackIDs          []string   `json:"track_ids"`
	ObservationCount  int        `json:"observation_count"`
	Confidence        float64    `json:"confidence"`
	Modalities        []string   `json:"modalities"`
	PriorityScore     float64    `json:"priority_score"`
	Status            string     `json:"status"` // active | rescued | cleared
	FirstSeen         time.Time  `json:"first_seen"`
	LastSeen          time.Time  `json:"last_seen"`
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	Notes             string     `json:"notes,omitempty"`
}
