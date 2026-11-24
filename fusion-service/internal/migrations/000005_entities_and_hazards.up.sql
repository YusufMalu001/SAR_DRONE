CREATE TABLE IF NOT EXISTS entities (
    id                 TEXT PRIMARY KEY,
    type               TEXT NOT NULL CHECK (type IN ('survivor', 'hazard')),
    class              TEXT NOT NULL CHECK (class IN ('person', 'fire', 'flood', 'debris', 'structural_damage')),
    geom               geometry(Point, 4326) NOT NULL,
    uncertainty_radius REAL NOT NULL DEFAULT 6.0,
    track_ids          TEXT[] NOT NULL DEFAULT '{}',
    observation_count  INT NOT NULL DEFAULT 1,
    confidence         REAL NOT NULL,
    modalities         TEXT[] NOT NULL DEFAULT '{}',
    priority_score     REAL NOT NULL DEFAULT 0.0,
    status             TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'rescued', 'cleared')),
    first_seen         TIMESTAMPTZ NOT NULL,
    last_seen          TIMESTAMPTZ NOT NULL,
    resolved_at        TIMESTAMPTZ,
    notes              TEXT NOT NULL DEFAULT '',
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_entities_geom ON entities USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_entities_type ON entities (type);
CREATE INDEX IF NOT EXISTS idx_entities_status ON entities (status);
CREATE INDEX IF NOT EXISTS idx_entities_class ON entities (class);

CREATE TABLE IF NOT EXISTS hazard_polygons (
    id                 TEXT PRIMARY KEY,
    class              TEXT NOT NULL CHECK (class IN ('fire', 'flood', 'debris', 'structural_damage')),
    geom               geometry(Geometry, 4326) NOT NULL,
    area_sq_m          REAL NOT NULL DEFAULT 0.0,
    detection_count    INT NOT NULL DEFAULT 1,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hazard_polygons_geom ON hazard_polygons USING GIST (geom);
CREATE INDEX IF NOT EXISTS idx_hazard_polygons_class ON hazard_polygons (class);

ALTER TABLE alerts ADD COLUMN IF NOT EXISTS alert_id TEXT;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS entity_id TEXT;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS priority_score REAL DEFAULT 0.0;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'active';
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS last_notified_at TIMESTAMPTZ DEFAULT now();
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;

ALTER TABLE alerts ALTER COLUMN detection_id DROP NOT NULL;
ALTER TABLE alerts DROP CONSTRAINT IF EXISTS alerts_detection_id_key;

CREATE INDEX IF NOT EXISTS idx_alerts_entity_id ON alerts (entity_id);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts (status);
