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
  timestamp: string;
}
