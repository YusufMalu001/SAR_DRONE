// Mirrors ../../../shared/jsonschema/detection.schema.json and telemetry.schema.json,
// plus fusion-service's REST/WebSocket-only additions (status, alerts, route-status).

export type DetectionClass =
  | "person"
  | "fire"
  | "flood"
  | "debris"
  | "structural_damage";

export type Source = "rgb" | "thermal";

export interface BBox {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface StageTimings {
  decode?: number;
  preprocess?: number;
  inference?: number;
  postprocess?: number;
  publish?: number;
  total?: number;
}

export interface Detection {
  id: string;
  class: DetectionClass;
  confidence: number;
  bbox: BBox;
  frame_id: string;
  source: Source;
  timestamp: string;
  track_id?: string;
  modalities?: Source[];
  model_version?: string;
  timings_ms?: StageTimings;
}

export interface Telemetry {
  lat: number;
  lon: number;
  alt: number;
  heading: number;
  timestamp: string;
}

export interface ServiceHealth {
  status: string;
  service: string;
  mode?: string;
  queue_depth?: number;
  oldest_item_age_seconds?: number;
  last_successful_sync?: string;
  [key: string]: unknown;
}

// fusion-service's fused detection: GET /detections, GET /detections/{id},
// and the "detection" WS message payload.
export type DetectionStatus =
  | "live"
  | "queued"
  | "synced-from-offline-queue"
  | "synced";

export interface DetectionRecord {
  id: string;
  class: DetectionClass;
  confidence: number;
  lat: number;
  lon: number;
  frame_id: string;
  source: Source;
  detected_at: string;
  priority_score: number;
  status: DetectionStatus;
  track_id?: string;
  modalities?: Source[];
}

export type PriorityTier = "critical" | "high" | "medium";

export interface AlertRecord {
  id: number;
  detection_id: string;
  priority_tier: PriorityTier;
  message: string;
  created_at: string;
}

// /ws/live envelope: { "type": "detection" | "alert", "payload": ... }
export type WSMessageType = "detection" | "alert";

export interface WSMessage<T = unknown> {
  type: WSMessageType;
  payload: T;
}

export interface MissionSummary {
  total_detections: number;
  by_class: Record<string, number>;
  high_priority_count: number;
  average_priority: number;
  latest_detection_at?: string;
}

export interface RouteWaypoint {
  lat: number;
  lon: number;
  recorded_at: string;
}

export interface RouteSegment {
  index: number;
  start: RouteWaypoint;
  end: RouteWaypoint;
  status: "clear" | "obstructed";
  nearest_hazard_id?: string;
  nearest_hazard_class?: string;
  distance_meters?: number;
}

export interface RouteStatus {
  total_segments: number;
  obstructed_segments: number;
  segments: RouteSegment[];
}
