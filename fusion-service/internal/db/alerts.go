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
// fused detection or entity (see internal/alerting).
type Alert struct {
	ID             int64
	AlertID        string
	EntityID       string
	DetectionID    string
	PriorityTier   string
	Message        string
	PriorityScore  float64
	Status         string
	LastNotifiedAt time.Time
	CreatedAt      time.Time
	ResolvedAt     *time.Time
}

// InsertAlert persists a new detection alert (backward compatibility).
func InsertAlert(ctx context.Context, pool *pgxpool.Pool, a Alert) (*Alert, error) {
	row := pool.QueryRow(ctx, `
		INSERT INTO alerts (detection_id, priority_tier, message, alert_id, entity_id, priority_score, status, last_notified_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (id) DO UPDATE SET
			priority_tier = EXCLUDED.priority_tier,
			message = EXCLUDED.message,
			priority_score = EXCLUDED.priority_score,
			status = EXCLUDED.status,
			last_notified_at = now()
		RETURNING id, created_at`,
		a.DetectionID, a.PriorityTier, a.Message, a.AlertID, a.EntityID, a.PriorityScore, a.Status,
	)

	if err := row.Scan(&a.ID, &a.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("insert alert for detection %s: %w", a.DetectionID, err)
	}
	return &a, nil
}

// UpsertAlertRecord stores or updates an entity-centric alert.
func UpsertAlertRecord(ctx context.Context, pool *pgxpool.Pool, a Alert) (*Alert, error) {
	row := pool.QueryRow(ctx, `
		INSERT INTO alerts
			(alert_id, entity_id, detection_id, priority_tier, message, priority_score, status, last_notified_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
		RETURNING id, created_at`,
		a.AlertID, a.EntityID, a.DetectionID, a.PriorityTier, a.Message, a.PriorityScore, a.Status, a.LastNotifiedAt,
	)
	if err := row.Scan(&a.ID, &a.CreatedAt); err != nil {
		return nil, fmt.Errorf("upsert alert %s: %w", a.AlertID, err)
	}
	return &a, nil
}

// AcknowledgeAlertInDB sets an alert's status to acknowledged.
func AcknowledgeAlertInDB(ctx context.Context, pool *pgxpool.Pool, alertID string) error {
	_, err := pool.Exec(ctx, `
		UPDATE alerts
		SET status = 'acknowledged'
		WHERE alert_id = $1 OR id::text = $1`, alertID,
	)
	if err != nil {
		return fmt.Errorf("acknowledge alert %s in db: %w", alertID, err)
	}
	return nil
}

// ResolveAlertInDB sets an alert's status to resolved.
func ResolveAlertInDB(ctx context.Context, pool *pgxpool.Pool, alertID string) error {
	_, err := pool.Exec(ctx, `
		UPDATE alerts
		SET status = 'resolved', resolved_at = now()
		WHERE alert_id = $1 OR id::text = $1`, alertID,
	)
	if err != nil {
		return fmt.Errorf("resolve alert %s in db: %w", alertID, err)
	}
	return nil
}

// GetAlertByDetectionID fetches the alert for a given detection, if one exists.
func GetAlertByDetectionID(ctx context.Context, pool *pgxpool.Pool, detectionID string) (*Alert, error) {
	var a Alert
	err := pool.QueryRow(ctx, `
		SELECT id, COALESCE(alert_id, ''), COALESCE(entity_id, ''), COALESCE(detection_id, ''),
		       priority_tier, message, COALESCE(priority_score, 0), COALESCE(status, 'active'),
		       COALESCE(last_notified_at, created_at), created_at, resolved_at
		FROM alerts
		WHERE detection_id = $1
		ORDER BY created_at DESC LIMIT 1`, detectionID,
	).Scan(&a.ID, &a.AlertID, &a.EntityID, &a.DetectionID, &a.PriorityTier, &a.Message, &a.PriorityScore, &a.Status, &a.LastNotifiedAt, &a.CreatedAt, &a.ResolvedAt)
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
	return ListAlertsWithFilter(ctx, pool, limit, "")
}

// ListAlertsWithFilter returns alerts optionally filtered by status.
func ListAlertsWithFilter(ctx context.Context, pool *pgxpool.Pool, limit int, status string) ([]Alert, error) {
	query := `
		SELECT id, COALESCE(alert_id, ''), COALESCE(entity_id, ''), COALESCE(detection_id, ''),
		       priority_tier, message, COALESCE(priority_score, 0), COALESCE(status, 'active'),
		       COALESCE(last_notified_at, created_at), created_at, resolved_at
		FROM alerts WHERE 1=1`
	var args []any

	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d`, len(args))

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	var out []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.AlertID, &a.EntityID, &a.DetectionID, &a.PriorityTier, &a.Message, &a.PriorityScore, &a.Status, &a.LastNotifiedAt, &a.CreatedAt, &a.ResolvedAt); err != nil {
			return nil, fmt.Errorf("scan alert row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
