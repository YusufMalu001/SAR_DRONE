ALTER TABLE detections ADD COLUMN IF NOT EXISTS track_id TEXT;
ALTER TABLE detections ADD COLUMN IF NOT EXISTS modalities TEXT[];
CREATE INDEX IF NOT EXISTS idx_detections_track_id ON detections (track_id);
