package dto

import "time"

type AlertRecord struct {
	ID             string     `json:"id"`
	NumericID      int64      `json:"numeric_id,omitempty"`
	EntityID       string     `json:"entity_id,omitempty"`
	DetectionID    string     `json:"detection_id,omitempty"`
	PriorityTier   string     `json:"priority_tier"`
	Message        string     `json:"message"`
	PriorityScore  float64    `json:"priority_score"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	LastNotifiedAt time.Time  `json:"last_notified_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}
