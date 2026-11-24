/** Generated from shared/jsonschema — DO NOT EDIT MANUALLY. */

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
  pitch?: number;
  roll?: number;
  yaw?: number;
  timestamp: string;
}

export type HazardSeverity = "critical" | "high" | "medium" | "low";

export interface HazardZoneProperties {
  id: string;
  class: string;
  severity: HazardSeverity;
  buffer_meters: number;
  area_sq_m: number;
  detection_count: number;
  feature_type?: string;
}

export interface HazardZoneFeature {
  type: "Feature";
  id: string;
  geometry: {
    type: "Polygon" | "MultiPolygon" | "Point";
    coordinates: any;
  };
  properties: HazardZoneProperties;
}

export interface HazardZonesGeoJSON {
  type: "FeatureCollection";
  features: HazardZoneFeature[];
}

export interface SafeRouteProperties {
  route_id: string;
  safe: boolean;
  reason?: string | null;
  length_meters: number;
  risk_score: number;
  avoided_hazards: string[];
  intersected_hazards: string[];
  start: [number, number];
  destination: [number, number];
}

export interface SafeRouteFeature {
  type: "Feature";
  geometry: {
    type: "LineString";
    coordinates: [number, number][];
  };
  properties: SafeRouteProperties;
}

export interface SafeRouteGeoJSON {
  type: "FeatureCollection";
  features: SafeRouteFeature[];
}

export interface CoverageCellProperties {
  cell_id: string;
  observation_count: number;
  last_observed_at?: string | null;
  best_confidence: number;
  searched: boolean;
  staleness_seconds?: number | null;
}

export interface CoverageCellFeature {
  type: "Feature";
  geometry: {
    type: "Polygon";
    coordinates: number[][][];
  };
  properties: CoverageCellProperties;
}

export interface CoverageGeoJSON {
  type: "FeatureCollection";
  features: CoverageCellFeature[];
}
