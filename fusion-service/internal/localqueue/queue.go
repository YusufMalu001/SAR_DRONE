// Package localqueue is fusion-service's offline write-ahead buffer: every
// fused detection is written here first, before Postgres or any external
// sync is attempted, so nothing is lost if the database (or the network to
// it) is unavailable. Backed by SQLite (pure-Go driver, no CGO) so it needs
// no external service of its own.
package localqueue

import (
	"context"
	"database/sql"
	"fmt"
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
	queued_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_queued_detections_detected_at ON queued_detections (detected_at);
`

// Queue is a durable FIFO (ordered by detected_at, not insertion order) of
// fused detections awaiting a Postgres write.
type Queue struct {
	conn *sql.DB
}

// Open opens (creating if needed) the SQLite file at path and ensures its
// schema exists.
func Open(path string) (*Queue, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite queue at %s: %w", path, err)
	}
	// SQLite handles one writer at a time; forcing a single connection
	// avoids "database is locked" errors between the consumer and drain
	// loop instead of relying on busy-timeout retries for this prototype.
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("init sqlite queue schema: %w", err)
	}
	return &Queue{conn: conn}, nil
}

func (q *Queue) Close() error {
	return q.conn.Close()
}

// Enqueue durably records fd. A duplicate id (e.g. re-processing after a
// crash) is a no-op.
func (q *Queue) Enqueue(ctx context.Context, fd db.FusedDetection) error {
	_, err := q.conn.ExecContext(ctx, `
		INSERT INTO queued_detections
			(id, class, confidence, bbox_x, bbox_y, bbox_w, bbox_h, frame_id, source,
			 detected_at, telemetry_at, lat, lon, priority_score, queued_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING`,
		fd.ID, fd.Class, fd.Confidence, fd.BBoxX, fd.BBoxY, fd.BBoxW, fd.BBoxH, fd.FrameID, fd.Source,
		formatTime(fd.DetectedAt), formatTime(fd.TelemetryAt), fd.Lat, fd.Lon, fd.PriorityScore,
		formatTime(time.Now().UTC()),
	)
	if err != nil {
		return fmt.Errorf("enqueue detection %s: %w", fd.ID, err)
	}
	return nil
}

// Delete removes a detection from the queue, typically after it's been
// successfully written to Postgres.
func (q *Queue) Delete(ctx context.Context, id string) error {
	if _, err := q.conn.ExecContext(ctx, `DELETE FROM queued_detections WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete queued detection %s: %w", id, err)
	}
	return nil
}

// Count returns how many detections are currently queued.
func (q *Queue) Count(ctx context.Context) (int, error) {
	var n int
	if err := q.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM queued_detections`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count queued detections: %w", err)
	}
	return n, nil
}

// IDs returns the ids currently queued, oldest (by detected_at) first —
// mainly for the /debug/queue introspection endpoint.
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

// ListOrdered returns every queued detection ordered by its own
// detected_at ascending (oldest first) — the order they should be drained
// into Postgres in.
func (q *Queue) ListOrdered(ctx context.Context) ([]db.FusedDetection, error) {
	rows, err := q.conn.QueryContext(ctx, `
		SELECT id, class, confidence, bbox_x, bbox_y, bbox_w, bbox_h, frame_id, source,
		       detected_at, telemetry_at, lat, lon, priority_score
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
		err := rows.Scan(
			&fd.ID, &fd.Class, &fd.Confidence, &fd.BBoxX, &fd.BBoxY, &fd.BBoxW, &fd.BBoxH,
			&fd.FrameID, &fd.Source, &detectedAt, &telemetryAt, &fd.Lat, &fd.Lon, &fd.PriorityScore,
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
		out = append(out, fd)
	}
	return out, rows.Err()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}
