package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// FusedDetection is a detection after telemetry matching, geo-projection,
// and priority scoring — ready to persist.
type FusedDetection struct {
	ID            string
	Class         string
	Confidence    float64
	BBoxX         float64
	BBoxY         float64
	BBoxW         float64
	BBoxH         float64
	FrameID       string
	Source        string
	DetectedAt    time.Time
	TelemetryAt   time.Time
	Lat           float64
	Lon           float64
	PriorityScore float64
	TrackID       string
	Modalities    []string
}

// InsertDetection persists a fused detection. The `id` column is the
// detection's own id (a UUID string minted by inference-service); a
// duplicate insert (e.g. after an at-least-once redelivery) is a silent
// no-op rather than an error.
func InsertDetection(ctx context.Context, pool *pgxpool.Pool, fd FusedDetection) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO detections
			(id, class, confidence, bbox_x, bbox_y, bbox_w, bbox_h, frame_id, source,
			 detected_at, telemetry_at, geom, priority_score, track_id, modalities)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9,
			 $10, $11, ST_SetSRID(ST_MakePoint($12, $13), 4326), $14, $15, $16)
		ON CONFLICT (id) DO UPDATE SET
			track_id = EXCLUDED.track_id,
			modalities = EXCLUDED.modalities`,
		fd.ID, fd.Class, fd.Confidence, fd.BBoxX, fd.BBoxY, fd.BBoxW, fd.BBoxH, fd.FrameID, fd.Source,
		fd.DetectedAt, fd.TelemetryAt, fd.Lon, fd.Lat, fd.PriorityScore, fd.TrackID, fd.Modalities,
	)
	if err != nil {
		return fmt.Errorf("insert detection %s: %w", fd.ID, err)
	}
	return nil
}

// DetectionRow is a detection as read back from Postgres, with the PostGIS
// point already unpacked into plain lat/lon.
type DetectionRow struct {
	ID            string
	Class         string
	Confidence    float64
	Lat           float64
	Lon           float64
	FrameID       string
	Source        string
	DetectedAt    time.Time
	PriorityScore float64
	CreatedAt     time.Time
	TrackID       string
	Modalities    []string
}

const detectionColumns = `id, class, confidence, ST_Y(geom), ST_X(geom), frame_id, source, detected_at, priority_score, created_at, COALESCE(track_id, ''), COALESCE(modalities, '{}')`

func scanDetectionRow(row pgx.Row) (DetectionRow, error) {
	var d DetectionRow
	err := row.Scan(&d.ID, &d.Class, &d.Confidence, &d.Lat, &d.Lon, &d.FrameID, &d.Source, &d.DetectedAt, &d.PriorityScore, &d.CreatedAt, &d.TrackID, &d.Modalities)
	return d, err
}

// ListDetections returns the most recent detections (newest first),
// optionally filtered by class.
func ListDetections(ctx context.Context, pool *pgxpool.Pool, limit int, class string) ([]DetectionRow, error) {
	query := `SELECT ` + detectionColumns + ` FROM detections`
	args := []any{}

	if class != "" {
		query += fmt.Sprintf(" WHERE class = $%d", len(args)+1)
		args = append(args, class)
	}
	query += fmt.Sprintf(" ORDER BY detected_at DESC LIMIT $%d", len(args)+1)
	args = append(args, limit)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list detections: %w", err)
	}
	defer rows.Close()

	var out []DetectionRow
	for rows.Next() {
		d, err := scanDetectionRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan detection row: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// GetDetection fetches a single detection by id, returning (nil, nil) if
// it doesn't exist.
func GetDetection(ctx context.Context, pool *pgxpool.Pool, id string) (*DetectionRow, error) {
	row, err := scanDetectionRow(pool.QueryRow(ctx, `SELECT `+detectionColumns+` FROM detections WHERE id = $1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get detection %s: %w", id, err)
	}
	return &row, nil
}

// MissionSummary aggregates the detections table for the mission dashboard.
type MissionSummary struct {
	TotalDetections   int            `json:"total_detections"`
	ByClass           map[string]int `json:"by_class"`
	HighPriorityCount int            `json:"high_priority_count"`
	AveragePriority   float64        `json:"average_priority"`
	LatestDetectionAt *time.Time     `json:"latest_detection_at,omitempty"`
}

func GetMissionSummary(ctx context.Context, pool *pgxpool.Pool, highPriorityThreshold float64) (*MissionSummary, error) {
	summary := &MissionSummary{ByClass: map[string]int{}}

	err := pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(AVG(priority_score), 0), MAX(detected_at)
		FROM detections`,
	).Scan(&summary.TotalDetections, &summary.AveragePriority, &summary.LatestDetectionAt)
	if err != nil {
		return nil, fmt.Errorf("mission summary totals: %w", err)
	}

	rows, err := pool.Query(ctx, `SELECT class, COUNT(*) FROM detections GROUP BY class`)
	if err != nil {
		return nil, fmt.Errorf("mission summary by class: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var class string
		var count int
		if err := rows.Scan(&class, &count); err != nil {
			return nil, fmt.Errorf("scan by-class row: %w", err)
		}
		summary.ByClass[class] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM detections WHERE priority_score >= $1`, highPriorityThreshold).
		Scan(&summary.HighPriorityCount)
	if err != nil {
		return nil, fmt.Errorf("mission summary high priority count: %w", err)
	}

	return summary, nil
}
