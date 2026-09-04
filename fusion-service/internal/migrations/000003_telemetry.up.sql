-- Recorded flight path, used by GET /mission/route-status to check for
-- segments passing close to a hazard-class detection.
CREATE TABLE IF NOT EXISTS telemetry (
    id             BIGSERIAL PRIMARY KEY,
    lat            DOUBLE PRECISION NOT NULL,
    lon            DOUBLE PRECISION NOT NULL,
    alt            DOUBLE PRECISION NOT NULL,
    heading        DOUBLE PRECISION NOT NULL,
    recorded_at    TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_telemetry_recorded_at ON telemetry (recorded_at);
