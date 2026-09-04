package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Alert is a rule-based, human-readable flag raised for a high-priority
// fused detection (see internal/alerting).
type Alert struct {
	ID           int64
	DetectionID  string
	PriorityTier string
	Message      string
	CreatedAt    time.Time
}

// InsertAlert persists a new alert. At most one alert exists per detection
// (UNIQUE constraint on detection_id): if one already exists — e.g. the
// same detection was reprocessed after an at-least-once redelivery —
// InsertAlert returns (nil, nil) rather than creating a duplicate.
func InsertAlert(ctx context.Context, pool *pgxpool.Pool, a Alert) (*Alert, error) {
	row := pool.QueryRow(ctx, `
		INSERT INTO alerts (detection_id, priority_tier, message)
		VALUES ($1, $2, $3)
		ON CONFLICT (detection_id) DO NOTHING
		RETURNING id, created_at`,
		a.DetectionID, a.PriorityTier, a.Message,
	)

	if err := row.Scan(&a.ID, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("insert alert for detection %s: %w", a.DetectionID, err)
	}
	return &a, nil
}

// GetAlertByDetectionID fetches the alert for a given detection, if one
// exists (returns (nil, nil) if not — at most one alert exists per
// detection).
func GetAlertByDetectionID(ctx context.Context, pool *pgxpool.Pool, detectionID string) (*Alert, error) {
	var a Alert
	err := pool.QueryRow(ctx, `
		SELECT id, detection_id, priority_tier, message, created_at
		FROM alerts
		WHERE detection_id = $1`, detectionID,
	).Scan(&a.ID, &a.DetectionID, &a.PriorityTier, &a.Message, &a.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get alert for detection %s: %w", detectionID, err)
	}
	return &a, nil
}

// ListAlerts returns the most recent alerts, newest first.
func ListAlerts(ctx context.Context, pool *pgxpool.Pool, limit int) ([]Alert, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, detection_id, priority_tier, message, created_at
		FROM alerts
		ORDER BY created_at DESC
		LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.DetectionID, &a.PriorityTier, &a.Message, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
