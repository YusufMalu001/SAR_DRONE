DROP INDEX IF EXISTS idx_detections_track_id;
ALTER TABLE detections DROP COLUMN IF EXISTS modalities;
ALTER TABLE detections DROP COLUMN IF EXISTS track_id;
