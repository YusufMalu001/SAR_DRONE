CREATE TABLE IF NOT EXISTS alerts (
    id             BIGSERIAL PRIMARY KEY,
    -- One alert per detection: a redelivered/reprocessed detection must
    -- not create a duplicate alert (see db.InsertAlert's ON CONFLICT).
    detection_id   TEXT NOT NULL UNIQUE REFERENCES detections(id),
    priority_tier  TEXT NOT NULL CHECK (priority_tier IN ('critical', 'high', 'medium')),
    message        TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts (created_at);
