// Package localqueue is fusion-service's offline write-ahead buffer: every
// fused detection, entity update, alert state, and outbound dispatch is
// written here first, before Postgres or any external sync is attempted,
// so nothing is lost if the database or external network is unavailable.
// Backed by SQLite (pure-Go driver, no CGO).
package localqueue

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"sar-drone-prototype/fusion-service/internal/db"
)

const schema = `
CREATE TABLE IF NOT EXISTS queued_detections (
	id             TEXT PRIMARY KEY,
	class          TEXT NOT NULL,
	confidence     REAL NOT NULL,
	bbox_x         REAL NOT NULL,
	bbox_y         REAL NOT NULL,
	bbox_w         REAL NOT NULL,
	bbox_h         REAL NOT NULL,
	frame_id       TEXT NOT NULL,
	source         TEXT NOT NULL,
	detected_at    TEXT NOT NULL,
	telemetry_at   TEXT NOT NULL,
	lat            REAL NOT NULL,
	lon            REAL NOT NULL,
	priority_score REAL NOT NULL,
	queued_at      TEXT NOT NULL,
	track_id       TEXT,
	modalities     TEXT
);
CREATE INDEX IF NOT EXISTS idx_queued_detections_detected_at ON queued_detections (detected_at);

CREATE TABLE IF NOT EXISTS queued_entities (
	id                 TEXT PRIMARY KEY,
	type               TEXT NOT NULL,
	class              TEXT NOT NULL,
	lat                REAL NOT NULL,
	lon                REAL NOT NULL,
	uncertainty_radius REAL NOT NULL,
	track_ids          TEXT,
	observation_count  INTEGER NOT NULL,
	confidence         REAL NOT NULL,
	modalities         TEXT,
	priority_score     REAL NOT NULL,
	status             TEXT NOT NULL,
	first_seen         TEXT NOT NULL,
	last_seen          TEXT NOT NULL,
	notes              TEXT,
	queued_at          TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_queued_entities_last_seen ON queued_entities (last_seen);

CREATE TABLE IF NOT EXISTS queued_alerts (
	id               TEXT PRIMARY KEY,
	entity_id        TEXT NOT NULL,
	detection_id     TEXT NOT NULL,
	priority_tier    TEXT NOT NULL,
	message          TEXT NOT NULL,
	priority_score   REAL NOT NULL,
	status           TEXT NOT NULL,
	last_notified_at TEXT NOT NULL,
	created_at       TEXT NOT NULL,
	resolved_at      TEXT,
	queued_at        TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_queued_alerts_created_at ON queued_alerts (created_at);

CREATE TABLE IF NOT EXISTS queued_outbound (
	id            TEXT PRIMARY KEY,
	alert_id      TEXT NOT NULL,
	adapter       TEXT NOT NULL,
	payload       TEXT NOT NULL,
	status        TEXT NOT NULL,
	attempts      INTEGER NOT NULL DEFAULT 0,
	next_retry_at TEXT,
	last_error    TEXT,
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_queued_outbound_status ON queued_outbound (status, next_retry_at);

CREATE TABLE IF NOT EXISTS sync_meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
`

// Queue is a durable local buffer for detections, entities, alerts, and outbound messages.
type Queue struct {
	conn *sql.DB
}

// OutboundItem represents an outbound notification queued for external dispatch.
type OutboundItem struct {
	ID          string    `json:"id"`
	AlertID     string    `json:"alert_id"`
	Adapter     string    `json:"adapter"`
	Payload     string    `json:"payload"`
	Status      string    `json:"status"` // "pending", "sent", "failed"
	Attempts    int       `json:"attempts"`
	NextRetryAt time.Time `json:"next_retry_at"`
	LastError   string    `json:"last_error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// QueueMetrics exposes sync and queue health for dashboard and health status.
type QueueMetrics struct {
	QueueDepth           int        `json:"queue_depth"`
	OldestItemAgeSeconds float64    `json:"oldest_item_age_seconds"`
	LastSuccessfulSync   *time.Time `json:"last_successful_sync"`
}

// Open opens (creating if needed) the SQLite file at path and ensures its schema exists.
func Open(path string) (*Queue, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite queue at %s: %w", path, err)
	}
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("init sqlite queue schema: %w", err)
	}

	// Migrations for existing database files
	_, _ = conn.Exec(`ALTER TABLE queued_detections ADD COLUMN track_id TEXT;`)
	_, _ = conn.Exec(`ALTER TABLE queued_detections ADD COLUMN modalities TEXT;`)

	return &Queue{conn: conn}, nil
}

func (q *Queue) Close() error {
	return q.conn.Close()
}

// EnqueueDetection durably records a fused detection (backward compatible with Enqueue).
func (q *Queue) EnqueueDetection(ctx context.Context, fd db.FusedDetection) error {
	mods := strings.Join(fd.Modalities, ",")
	_, err := q.conn.ExecContext(ctx, `
		INSERT INTO queued_detections
			(id, class, confidence, bbox_x, bbox_y, bbox_w, bbox_h, frame_id, source,
			 detected_at, telemetry_at, lat, lon, priority_score, queued_at, track_id, modalities)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING`,
		fd.ID, fd.Class, fd.Confidence, fd.BBoxX, fd.BBoxY, fd.BBoxW, fd.BBoxH, fd.FrameID, fd.Source,
		formatTime(fd.DetectedAt), formatTime(fd.TelemetryAt), fd.Lat, fd.Lon, fd.PriorityScore,
		formatTime(time.Now().UTC()), fd.TrackID, mods,
	)
	if err != nil {
		return fmt.Errorf("enqueue detection %s: %w", fd.ID, err)
	}
	return nil
}

func (q *Queue) Enqueue(ctx context.Context, fd db.FusedDetection) error {
	return q.EnqueueDetection(ctx, fd)
}

// DeleteDetection removes a detection from the queue.
func (q *Queue) DeleteDetection(ctx context.Context, id string) error {
	if _, err := q.conn.ExecContext(ctx, `DELETE FROM queued_detections WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete queued detection %s: %w", id, err)
	}
	return nil
}

func (q *Queue) Delete(ctx context.Context, id string) error {
	return q.DeleteDetection(ctx, id)
}

// Count returns the number of queued detections.
func (q *Queue) Count(ctx context.Context) (int, error) {
	var n int
	if err := q.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM queued_detections`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count queued detections: %w", err)
	}
	return n, nil
}

// IDs returns the detection ids currently queued, oldest first.
func (q *Queue) IDs(ctx context.Context) ([]string, error) {
	rows, err := q.conn.QueryContext(ctx, `SELECT id FROM queued_detections ORDER BY detected_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list queued ids: %w", err)
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan queued id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListOrdered returns every queued detection ordered by detected_at ascending.
func (q *Queue) ListOrdered(ctx context.Context) ([]db.FusedDetection, error) {
	rows, err := q.conn.QueryContext(ctx, `
		SELECT id, class, confidence, bbox_x, bbox_y, bbox_w, bbox_h, frame_id, source,
		       detected_at, telemetry_at, lat, lon, priority_score, COALESCE(track_id, ''), COALESCE(modalities, '')
		FROM queued_detections
		ORDER BY detected_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list queued detections: %w", err)
	}
	defer rows.Close()

	var out []db.FusedDetection
	for rows.Next() {
		var fd db.FusedDetection
		var detectedAt, telemetryAt string
		var trackID, mods string
		err := rows.Scan(
			&fd.ID, &fd.Class, &fd.Confidence, &fd.BBoxX, &fd.BBoxY, &fd.BBoxW, &fd.BBoxH,
			&fd.FrameID, &fd.Source, &detectedAt, &telemetryAt, &fd.Lat, &fd.Lon, &fd.PriorityScore,
			&trackID, &mods,
		)
		if err != nil {
			return nil, fmt.Errorf("scan queued detection: %w", err)
		}

		if fd.DetectedAt, err = time.Parse(time.RFC3339Nano, detectedAt); err != nil {
			return nil, fmt.Errorf("parse queued detected_at: %w", err)
		}
		if fd.TelemetryAt, err = time.Parse(time.RFC3339Nano, telemetryAt); err != nil {
			return nil, fmt.Errorf("parse queued telemetry_at: %w", err)
		}
		fd.TrackID = trackID
		if mods != "" {
			fd.Modalities = strings.Split(mods, ",")
		}
		out = append(out, fd)
	}
	return out, rows.Err()
}

// EnqueueEntity buffers an entity update with Last-Write-Wins (LWW) conflict resolution.
func (q *Queue) EnqueueEntity(ctx context.Context, e db.EntityRow) error {
	trackIDs := strings.Join(e.TrackIDs, ",")
	mods := strings.Join(e.Modalities, ",")
	_, err := q.conn.ExecContext(ctx, `
		INSERT INTO queued_entities
			(id, type, class, lat, lon, uncertainty_radius, track_ids, observation_count,
			 confidence, modalities, priority_score, status, first_seen, last_seen, notes, queued_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = EXCLUDED.type,
			class = EXCLUDED.class,
			lat = EXCLUDED.lat,
			lon = EXCLUDED.lon,
			uncertainty_radius = EXCLUDED.uncertainty_radius,
			track_ids = EXCLUDED.track_ids,
			observation_count = EXCLUDED.observation_count,
			confidence = EXCLUDED.confidence,
			modalities = EXCLUDED.modalities,
			priority_score = EXCLUDED.priority_score,
			status = EXCLUDED.status,
			last_seen = EXCLUDED.last_seen,
			notes = EXCLUDED.notes,
			queued_at = EXCLUDED.queued_at
		WHERE queued_entities.last_seen <= EXCLUDED.last_seen`,
		e.ID, e.Type, e.Class, e.Lat, e.Lon, e.UncertaintyRadius, trackIDs, e.ObservationCount,
		e.Confidence, mods, e.PriorityScore, e.Status, formatTime(e.FirstSeen), formatTime(e.LastSeen),
		e.Notes, formatTime(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("enqueue entity %s: %w", e.ID, err)
	}
	return nil
}

// DeleteEntity removes an entity from the local queue after Postgres commit.
func (q *Queue) DeleteEntity(ctx context.Context, id string) error {
	if _, err := q.conn.ExecContext(ctx, `DELETE FROM queued_entities WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete queued entity %s: %w", id, err)
	}
	return nil
}

// ListOrderedEntities returns buffered entities ordered by last_seen ascending.
func (q *Queue) ListOrderedEntities(ctx context.Context) ([]db.EntityRow, error) {
	rows, err := q.conn.QueryContext(ctx, `
		SELECT id, type, class, lat, lon, uncertainty_radius, track_ids, observation_count,
		       confidence, modalities, priority_score, status, first_seen, last_seen, COALESCE(notes, '')
		FROM queued_entities
		ORDER BY last_seen ASC`)
	if err != nil {
		return nil, fmt.Errorf("list queued entities: %w", err)
	}
	defer rows.Close()

	var out []db.EntityRow
	for rows.Next() {
		var e db.EntityRow
		var trackIDs, mods, firstSeen, lastSeen string
		err := rows.Scan(
			&e.ID, &e.Type, &e.Class, &e.Lat, &e.Lon, &e.UncertaintyRadius, &trackIDs,
			&e.ObservationCount, &e.Confidence, &mods, &e.PriorityScore, &e.Status,
			&firstSeen, &lastSeen, &e.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("scan queued entity: %w", err)
		}
		if e.FirstSeen, err = time.Parse(time.RFC3339Nano, firstSeen); err != nil {
			return nil, fmt.Errorf("parse queued first_seen: %w", err)
		}
		if e.LastSeen, err = time.Parse(time.RFC3339Nano, lastSeen); err != nil {
			return nil, fmt.Errorf("parse queued last_seen: %w", err)
		}
		if trackIDs != "" {
			e.TrackIDs = strings.Split(trackIDs, ",")
		}
		if mods != "" {
			e.Modalities = strings.Split(mods, ",")
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EnqueueAlert buffers an alert update with LWW semantics.
func (q *Queue) EnqueueAlert(ctx context.Context, a db.Alert) error {
	resolvedAtStr := ""
	if a.ResolvedAt != nil {
		resolvedAtStr = formatTime(*a.ResolvedAt)
	}
	alertID := a.AlertID
	if alertID == "" {
		alertID = fmt.Sprintf("%d", a.ID)
	}

	_, err := q.conn.ExecContext(ctx, `
		INSERT INTO queued_alerts
			(id, entity_id, detection_id, priority_tier, message, priority_score, status,
			 last_notified_at, created_at, resolved_at, queued_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			priority_tier = EXCLUDED.priority_tier,
			message = EXCLUDED.message,
			priority_score = EXCLUDED.priority_score,
			status = EXCLUDED.status,
			last_notified_at = EXCLUDED.last_notified_at,
			resolved_at = EXCLUDED.resolved_at,
			queued_at = EXCLUDED.queued_at
		WHERE queued_alerts.last_notified_at <= EXCLUDED.last_notified_at`,
		alertID, a.EntityID, a.DetectionID, a.PriorityTier, a.Message, a.PriorityScore,
		a.Status, formatTime(a.LastNotifiedAt), formatTime(a.CreatedAt),
		sql.NullString{String: resolvedAtStr, Valid: resolvedAtStr != ""},
		formatTime(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("enqueue alert %s: %w", alertID, err)
	}
	return nil
}

// DeleteAlert removes an alert from the local queue after Postgres commit.
func (q *Queue) DeleteAlert(ctx context.Context, alertID string) error {
	if _, err := q.conn.ExecContext(ctx, `DELETE FROM queued_alerts WHERE id = ?`, alertID); err != nil {
		return fmt.Errorf("delete queued alert %s: %w", alertID, err)
	}
	return nil
}

// ListOrderedAlerts returns buffered alerts ordered by created_at ascending.
func (q *Queue) ListOrderedAlerts(ctx context.Context) ([]db.Alert, error) {
	rows, err := q.conn.QueryContext(ctx, `
		SELECT id, entity_id, detection_id, priority_tier, message, priority_score, status,
		       last_notified_at, created_at, COALESCE(resolved_at, '')
		FROM queued_alerts
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list queued alerts: %w", err)
	}
	defer rows.Close()

	var out []db.Alert
	for rows.Next() {
		var a db.Alert
		var lastNotified, createdAt, resolvedAt string
		err := rows.Scan(
			&a.AlertID, &a.EntityID, &a.DetectionID, &a.PriorityTier, &a.Message,
			&a.PriorityScore, &a.Status, &lastNotified, &createdAt, &resolvedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan queued alert: %w", err)
		}
		if a.LastNotifiedAt, err = time.Parse(time.RFC3339Nano, lastNotified); err != nil {
			return nil, fmt.Errorf("parse queued last_notified_at: %w", err)
		}
		if a.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
			return nil, fmt.Errorf("parse queued created_at: %w", err)
		}
		if resolvedAt != "" {
			if t, err := time.Parse(time.RFC3339Nano, resolvedAt); err == nil {
				a.ResolvedAt = &t
			}
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// EnqueueOutbound buffers an outbound notification.
func (q *Queue) EnqueueOutbound(ctx context.Context, item OutboundItem) error {
	now := time.Now().UTC()
	nextRetry := now
	if !item.NextRetryAt.IsZero() {
		nextRetry = item.NextRetryAt
	}
	_, err := q.conn.ExecContext(ctx, `
		INSERT INTO queued_outbound
			(id, alert_id, adapter, payload, status, attempts, next_retry_at, last_error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = EXCLUDED.status,
			attempts = EXCLUDED.attempts,
			next_retry_at = EXCLUDED.next_retry_at,
			last_error = EXCLUDED.last_error,
			updated_at = EXCLUDED.updated_at`,
		item.ID, item.AlertID, item.Adapter, item.Payload, item.Status, item.Attempts,
		formatTime(nextRetry), item.LastError, formatTime(now), formatTime(now),
	)
	if err != nil {
		return fmt.Errorf("enqueue outbound %s: %w", item.ID, err)
	}
	return nil
}

// ListPendingOutbound returns outbound tasks ready to be sent (status != 'sent' and next_retry_at <= now).
func (q *Queue) ListPendingOutbound(ctx context.Context) ([]OutboundItem, error) {
	nowStr := formatTime(time.Now().UTC())
	rows, err := q.conn.QueryContext(ctx, `
		SELECT id, alert_id, adapter, payload, status, attempts, COALESCE(next_retry_at, ''),
		       COALESCE(last_error, ''), created_at, updated_at
		FROM queued_outbound
		WHERE status != 'sent' AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY created_at ASC`, nowStr)
	if err != nil {
		return nil, fmt.Errorf("list pending outbound: %w", err)
	}
	defer rows.Close()

	var out []OutboundItem
	for rows.Next() {
		var item OutboundItem
		var nextRetry, createdAt, updatedAt string
		err := rows.Scan(
			&item.ID, &item.AlertID, &item.Adapter, &item.Payload, &item.Status,
			&item.Attempts, &nextRetry, &item.LastError, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan pending outbound: %w", err)
		}
		if nextRetry != "" {
			item.NextRetryAt, _ = time.Parse(time.RFC3339Nano, nextRetry)
		}
		item.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}

// MarkOutboundSent marks an outbound notification successfully delivered.
func (q *Queue) MarkOutboundSent(ctx context.Context, id string) error {
	nowStr := formatTime(time.Now().UTC())
	_, err := q.conn.ExecContext(ctx, `
		UPDATE queued_outbound
		SET status = 'sent', updated_at = ?
		WHERE id = ?`, nowStr, id)
	if err != nil {
		return fmt.Errorf("mark outbound sent %s: %w", id, err)
	}
	return nil
}

// MarkOutboundFailed records failure and schedules next retry with backoff.
func (q *Queue) MarkOutboundFailed(ctx context.Context, id string, errStr string, nextRetry time.Time) error {
	nowStr := formatTime(time.Now().UTC())
	_, err := q.conn.ExecContext(ctx, `
		UPDATE queued_outbound
		SET status = 'failed', attempts = attempts + 1, last_error = ?, next_retry_at = ?, updated_at = ?
		WHERE id = ?`, errStr, formatTime(nextRetry), nowStr, id)
	if err != nil {
		return fmt.Errorf("mark outbound failed %s: %w", id, err)
	}
	return nil
}

// RecordSyncSuccess records the wall-clock time of a successful sync to Postgres or downstream.
func (q *Queue) RecordSyncSuccess(ctx context.Context) error {
	nowStr := formatTime(time.Now().UTC())
	_, err := q.conn.ExecContext(ctx, `
		INSERT INTO sync_meta (key, value)
		VALUES ('last_successful_sync', ?)
		ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value`,
		nowStr,
	)
	if err != nil {
		return fmt.Errorf("record sync success: %w", err)
	}
	return nil
}

// GetMetrics computes the queue depth, age of the oldest queued item, and last successful sync.
func (q *Queue) GetMetrics(ctx context.Context) (QueueMetrics, error) {
	var metrics QueueMetrics

	// Queue depth: total across detections, entities, alerts, pending outbound
	var dCount, eCount, aCount, oCount int
	_ = q.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM queued_detections`).Scan(&dCount)
	_ = q.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM queued_entities`).Scan(&eCount)
	_ = q.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM queued_alerts`).Scan(&aCount)
	_ = q.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM queued_outbound WHERE status != 'sent'`).Scan(&oCount)
	metrics.QueueDepth = dCount + eCount + aCount + oCount

	// Find the oldest item across all queues
	var oldestTS sql.NullString
	_ = q.conn.QueryRowContext(ctx, `
		SELECT MIN(ts) FROM (
			SELECT MIN(queued_at) AS ts FROM queued_detections
			UNION ALL
			SELECT MIN(queued_at) AS ts FROM queued_entities
			UNION ALL
			SELECT MIN(queued_at) AS ts FROM queued_alerts
			UNION ALL
			SELECT MIN(created_at) AS ts FROM queued_outbound WHERE status != 'sent'
		) WHERE ts IS NOT NULL AND ts != ''`).Scan(&oldestTS)

	if oldestTS.Valid && oldestTS.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, oldestTS.String); err == nil {
			age := time.Since(t).Seconds()
			if age < 0 {
				age = 0
			}
			metrics.OldestItemAgeSeconds = age
		}
	}

	// Last successful sync
	var lastSync sql.NullString
	_ = q.conn.QueryRowContext(ctx, `SELECT value FROM sync_meta WHERE key = 'last_successful_sync'`).Scan(&lastSync)
	if lastSync.Valid && lastSync.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, lastSync.String); err == nil {
			metrics.LastSuccessfulSync = &t
		}
	}

	return metrics, nil
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
