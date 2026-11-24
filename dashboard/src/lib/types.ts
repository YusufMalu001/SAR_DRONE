// Mirrors shared/jsonschema contracts + fusion-service & mission-service additions

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
  state?: "active" | "acknowledged" | "resolved";
  created_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
}

export interface EntityRecord {
  id: string;
  type: "survivor" | "hazard";
  class: DetectionClass | string;
  lat: number;
  lon: number;
  uncertainty_radius: number;
  track_ids?: string[];
  observation_count: number;
  confidence: number;
  modalities?: string[];
  priority_score: number;
  status: "active" | "rescued" | "cleared";
  first_seen: string;
  last_seen: string;
  resolved_at?: string;
  notes?: string;
}

export interface GeoJSONGeometry {
  type: string;
  coordinates: any;
}

export interface GeoJSONFeature {
  type: "Feature";
  geometry: GeoJSONGeometry;
  properties: Record<string, any>;
}

export interface GeoJSONFeatureCollection {
  type: "FeatureCollection";
  features: GeoJSONFeature[];
}

export interface MissionStatusResponse {
  state: string;
  target_waypoints_count: number;
  current_waypoint_idx: number;
  armed: boolean;
  battery_pct: number;
  telemetry?: Telemetry;
}

export interface CoveragePlanWaypoint {
  index: number;
  lat: number;
  lon: number;
  alt_m: number;
  action?: string;
}

export interface CoveragePlanResponse {
  status: string;
  total_waypoints: number;
  coverage_fraction: number;
  waypoints: CoveragePlanWaypoint[];
}

export type WSMessageType = "detection" | "alert" | "telemetry";

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
