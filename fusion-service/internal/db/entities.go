package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EntityRow struct {
	ID                string
	Type              string
	Class             string
	Lat               float64
	Lon               float64
	UncertaintyRadius float64
	TrackIDs          []string
	ObservationCount  int
	Confidence        float64
	Modalities        []string
	PriorityScore     float64
	Status            string
	FirstSeen         time.Time
	LastSeen          time.Time
	ResolvedAt        *time.Time
	Notes             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

const entityColumns = `id, type, class, ST_Y(geom), ST_X(geom), uncertainty_radius, track_ids, observation_count, confidence, modalities, priority_score, status, first_seen, last_seen, resolved_at, notes, created_at, updated_at`

func scanEntityRow(row pgx.Row) (EntityRow, error) {
	var e EntityRow
	err := row.Scan(
		&e.ID, &e.Type, &e.Class, &e.Lat, &e.Lon, &e.UncertaintyRadius,
		&e.TrackIDs, &e.ObservationCount, &e.Confidence, &e.Modalities,
		&e.PriorityScore, &e.Status, &e.FirstSeen, &e.LastSeen, &e.ResolvedAt,
		&e.Notes, &e.CreatedAt, &e.UpdatedAt,
	)
	return e, err
}

func UpsertEntity(ctx context.Context, pool *pgxpool.Pool, e EntityRow) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO entities
			(id, type, class, geom, uncertainty_radius, track_ids, observation_count,
			 confidence, modalities, priority_score, status, first_seen, last_seen, resolved_at, notes, updated_at)
		VALUES
			($1, $2, $3, ST_SetSRID(ST_MakePoint($4, $5), 4326), $6, $7, $8,
			 $9, $10, $11, $12, $13, $14, $15, $16, now())
		ON CONFLICT (id) DO UPDATE SET
			geom = EXCLUDED.geom,
			uncertainty_radius = EXCLUDED.uncertainty_radius,
			track_ids = EXCLUDED.track_ids,
			observation_count = EXCLUDED.observation_count,
			confidence = EXCLUDED.confidence,
			modalities = EXCLUDED.modalities,
			priority_score = EXCLUDED.priority_score,
			status = EXCLUDED.status,
			last_seen = EXCLUDED.last_seen,
			resolved_at = EXCLUDED.resolved_at,
			notes = EXCLUDED.notes,
			updated_at = now()`,
		e.ID, e.Type, e.Class, e.Lon, e.Lat, e.UncertaintyRadius, e.TrackIDs, e.ObservationCount,
		e.Confidence, e.Modalities, e.PriorityScore, e.Status, e.FirstSeen, e.LastSeen, e.ResolvedAt, e.Notes,
	)
	if err != nil {
		return fmt.Errorf("upsert entity %s: %w", e.ID, err)
	}
	return nil
}

func GetEntity(ctx context.Context, pool *pgxpool.Pool, id string) (*EntityRow, error) {
	row := pool.QueryRow(ctx, `SELECT `+entityColumns+` FROM entities WHERE id = $1`, id)
	e, err := scanEntityRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get entity %s: %w", id, err)
	}
	return &e, nil
}

// ListActiveSurvivors returns all active survivor entities.
func ListActiveSurvivors(ctx context.Context, pool *pgxpool.Pool) ([]EntityRow, error) {
	return ListEntities(ctx, pool, "survivor", "active")
}

func ListEntities(ctx context.Context, pool *pgxpool.Pool, entityType, status string) ([]EntityRow, error) {
	query := `SELECT ` + entityColumns + ` FROM entities WHERE 1=1`
	var args []any

	if entityType != "" {
		args = append(args, entityType)
		query += fmt.Sprintf(` AND type = $%d`, len(args))
	}
	if status != "" {
		args = append(args, status)
		query += fmt.Sprintf(` AND status = $%d`, len(args))
	}
	query += ` ORDER BY priority_score DESC, last_seen DESC`

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close()

	var out []EntityRow
	for rows.Next() {
		e, err := scanEntityRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func ResolveEntityInDB(ctx context.Context, pool *pgxpool.Pool, id, status, notes string) error {
	_, err := pool.Exec(ctx, `
		UPDATE entities
		SET status = $2, notes = $3, resolved_at = now(), updated_at = now()
		WHERE id = $1`, id, status, notes,
	)
	if err != nil {
		return fmt.Errorf("resolve entity %s in db: %w", id, err)
	}
	return nil
}
