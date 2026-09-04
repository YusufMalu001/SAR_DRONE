package dto

import "time"

type AlertRecord struct {
	ID           int64     `json:"id"`
	DetectionID  string    `json:"detection_id"`
	PriorityTier string    `json:"priority_tier"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}
