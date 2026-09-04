CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS detections (
    -- Detection's own id, minted by inference-service (a UUID string) —
    -- kept as TEXT rather than the native UUID type to avoid driver-level
    -- type-mapping friction for this prototype.
    id              TEXT PRIMARY KEY,
    class           TEXT NOT NULL CHECK (class IN ('person', 'fire', 'flood', 'debris', 'structural_damage')),
    confidence      REAL NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    bbox_x          REAL NOT NULL,
    bbox_y          REAL NOT NULL,
    bbox_w          REAL NOT NULL,
    bbox_h          REAL NOT NULL,
    frame_id        TEXT NOT NULL,
    source          TEXT NOT NULL CHECK (source IN ('rgb', 'thermal')),
    detected_at     TIMESTAMPTZ NOT NULL,
    telemetry_at    TIMESTAMPTZ NOT NULL,
    -- Planar geometry (not geography) point, as requested: fusion-service
    -- computes lat/lon directly via a flat-earth pinhole projection, so a
    -- geometry column matches the precision it's actually storing.
    geom            geometry(Point, 4326) NOT NULL,
    priority_score  REAL NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_detections_geom ON detections USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_detections_detected_at ON detections (detected_at);
CREATE INDEX IF NOT EXISTS idx_detections_class ON detections (class);
