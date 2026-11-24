#!/usr/bin/env python3
"""Generates typed contracts for Python, Go, and TypeScript from shared/jsonschema."""

import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
SCHEMAS_DIR = ROOT / "jsonschema"
GEN_DIR = ROOT / "generated"

GEN_PY = GEN_DIR / "python"
GEN_GO = GEN_DIR / "go"
GEN_TS = GEN_DIR / "typescript"

for d in (GEN_PY, GEN_GO, GEN_TS):
    d.mkdir(parents=True, exist_ok=True)


def gen_python():
    code = '''"""Generated from shared/jsonschema — DO NOT EDIT MANUALLY."""
from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field


class DetectionClass(str, Enum):
    person = "person"
    fire = "fire"
    flood = "flood"
    debris = "debris"
    structural_damage = "structural_damage"


class Source(str, Enum):
    rgb = "rgb"
    thermal = "thermal"


class BBox(BaseModel):
    x: float = Field(ge=0.0, le=1.0)
    y: float = Field(ge=0.0, le=1.0)
    w: float = Field(ge=0.0, le=1.0)
    h: float = Field(ge=0.0, le=1.0)


class StageTimings(BaseModel):
    decode: Optional[float] = None
    preprocess: Optional[float] = None
    inference: Optional[float] = None
    postprocess: Optional[float] = None
    publish: Optional[float] = None
    total: Optional[float] = None


class Detection(BaseModel):
    id: str
    class_: DetectionClass = Field(alias="class")
    confidence: float = Field(ge=0.0, le=1.0)
    bbox: BBox
    frame_id: str
    source: Source
    timestamp: str
    track_id: Optional[str] = None
    modalities: Optional[List[Source]] = None
    model_version: Optional[str] = None
    timings_ms: Optional[StageTimings] = None

    model_config = {"populate_by_name": True, "protected_namespaces": ()}


class Telemetry(BaseModel):
    lat: float = Field(ge=-90.0, le=90.0)
    lon: float = Field(ge=-180.0, le=180.0)
    alt: float
    heading: float = Field(ge=0.0, le=360.0)
    pitch: Optional[float] = None
    roll: Optional[float] = None
    yaw: Optional[float] = None
    timestamp: str

    model_config = {"populate_by_name": True, "protected_namespaces": ()}


class HazardZoneProperties(BaseModel):
    id: str
    class_: str = Field(alias="class")
    severity: str
    buffer_meters: float = Field(ge=0.0)
    area_sq_m: float = Field(ge=0.0)
    detection_count: int = Field(ge=0)
    feature_type: Optional[str] = None

    model_config = {"populate_by_name": True, "protected_namespaces": ()}


class HazardZoneFeature(BaseModel):
    type: str = "Feature"
    id: str
    geometry: Dict[str, Any]
    properties: HazardZoneProperties


class HazardZonesGeoJSON(BaseModel):
    type: str = "FeatureCollection"
    features: List[HazardZoneFeature]


class SafeRouteProperties(BaseModel):
    route_id: str
    safe: bool
    reason: Optional[str] = None
    length_meters: float = Field(ge=0.0)
    risk_score: float = Field(ge=0.0)
    avoided_hazards: List[str]
    intersected_hazards: List[str]
    start: List[float]
    destination: List[float]


class SafeRouteFeature(BaseModel):
    type: str = "Feature"
    geometry: Dict[str, Any]
    properties: SafeRouteProperties


class SafeRouteGeoJSON(BaseModel):
    type: str = "FeatureCollection"
    features: List[SafeRouteFeature]


class CoverageCellProperties(BaseModel):
    cell_id: str
    observation_count: int = Field(ge=0)
    last_observed_at: Optional[str] = None
    best_confidence: float = Field(ge=0.0, le=1.0)
    searched: bool
    staleness_seconds: Optional[float] = None


class CoverageCellFeature(BaseModel):
    type: str = "Feature"
    geometry: Dict[str, Any]
    properties: CoverageCellProperties


class CoverageGeoJSON(BaseModel):
    type: str = "FeatureCollection"
    features: List[CoverageCellFeature]
'''
    (GEN_PY / "contracts.py").write_text(code)
    (GEN_PY / "__init__.py").write_text("from .contracts import *\n")


def gen_typescript():
    code = '''/** Generated from shared/jsonschema — DO NOT EDIT MANUALLY. */

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
'''
    (GEN_TS / "contracts.ts").write_text(code)
    (GEN_TS / "index.ts").write_text("export * from './contracts';\n")


def gen_go():
    tag_x = '`json:"x"`'
    tag_y = '`json:"y"`'
    tag_w = '`json:"w"`'
    tag_h = '`json:"h"`'
    tag_id = '`json:"id"`'
    tag_class = '`json:"class"`'
    tag_conf = '`json:"confidence"`'
    tag_bbox = '`json:"bbox"`'
    tag_frame = '`json:"frame_id"`'
    tag_source = '`json:"source"`'
    tag_ts = '`json:"timestamp"`'
    tag_track_id = '`json:"track_id,omitempty"`'
    tag_modalities = '`json:"modalities,omitempty"`'
    tag_version = '`json:"model_version,omitempty"`'
    tag_timings = '`json:"timings_ms,omitempty"`'
    tag_lat = '`json:"lat"`'
    tag_lon = '`json:"lon"`'
    tag_alt = '`json:"alt"`'
    tag_heading = '`json:"heading"`'
    tag_pitch = '`json:"pitch,omitempty"`'
    tag_roll = '`json:"roll,omitempty"`'
    tag_yaw = '`json:"yaw,omitempty"`'

    code = f'''// Package contracts contains types generated from shared/jsonschema -- DO NOT EDIT MANUALLY.
package contracts

import (
\t"fmt"
\ttime "time"
)

type DetectionClass string

const (
\tDetectionClassPerson           DetectionClass = "person"
\tDetectionClassFire             DetectionClass = "fire"
\tDetectionClassFlood            DetectionClass = "flood"
\tDetectionClassDebris           DetectionClass = "debris"
\tDetectionClassStructuralDamage DetectionClass = "structural_damage"
)

type Source string

const (
\tSourceRGB     Source = "rgb"
\tSourceThermal Source = "thermal"
)

type BBox struct {{
\tX float64 {tag_x}
\tY float64 {tag_y}
\tW float64 {tag_w}
\tH float64 {tag_h}
}}

type StageTimings struct {{
\tDecode      float64 `json:"decode,omitempty"`
\tPreprocess  float64 `json:"preprocess,omitempty"`
\tInference   float64 `json:"inference,omitempty"`
\tPostprocess float64 `json:"postprocess,omitempty"`
\tPublish     float64 `json:"publish,omitempty"`
\tTotal       float64 `json:"total,omitempty"`
}}

type Detection struct {{
\tID           string        {tag_id}
\tClass        DetectionClass {tag_class}
\tConfidence   float64       {tag_conf}
\tBBox         BBox          {tag_bbox}
\tFrameID      string        {tag_frame}
\tSource       Source        {tag_source}
\tTimestamp    string        {tag_ts}
\tTrackID      string        {tag_track_id}
\tModalities   []Source      {tag_modalities}
\tModelVersion string        {tag_version}
\tTimingsMS    *StageTimings {tag_timings}
}}

func (d *Detection) Validate() error {{
\tif d.ID == "" {{
\t\treturn fmt.Errorf("id is required")
\t}}
\tswitch d.Class {{
\tcase DetectionClassPerson, DetectionClassFire, DetectionClassFlood, DetectionClassDebris, DetectionClassStructuralDamage:
\tdefault:
\t\treturn fmt.Errorf("invalid class %q", d.Class)
\t}}
\tif d.Confidence < 0 || d.Confidence > 1 {{
\t\treturn fmt.Errorf("confidence out of range [0, 1]: %f", d.Confidence)
\t}}
\tif d.BBox.X < 0 || d.BBox.X > 1 || d.BBox.Y < 0 || d.BBox.Y > 1 || d.BBox.W < 0 || d.BBox.W > 1 || d.BBox.H < 0 || d.BBox.H > 1 {{
\t\treturn fmt.Errorf("bbox coords out of range [0, 1]")
\t}}
\tif d.FrameID == "" {{
\t\treturn fmt.Errorf("frame_id is required")
\t}}
\tswitch d.Source {{
\tcase SourceRGB, SourceThermal:
\tdefault:
\t\treturn fmt.Errorf("invalid source %q", d.Source)
\t}}
\tif _, err := time.Parse(time.RFC3339, d.Timestamp); err != nil {{
\t\tif _, err := time.Parse(time.RFC3339Nano, d.Timestamp); err != nil {{
\t\t\treturn fmt.Errorf("timestamp is not valid RFC3339: %w", err)
\t\t}}
\t}}
\tfor _, m := range d.Modalities {{
\t\tswitch m {{
\t\tcase SourceRGB, SourceThermal:
\t\tdefault:
\t\t\treturn fmt.Errorf("invalid modality %q", m)
\t\t}}
\t}}
\treturn nil
}}

type Telemetry struct {{
\tLat       float64 {tag_lat}
\tLon       float64 {tag_lon}
\tAlt       float64 {tag_alt}
\tHeading   float64 {tag_heading}
\tPitch     float64 {tag_pitch}
\tRoll      float64 {tag_roll}
\tYaw       float64 {tag_yaw}
\tTimestamp string  {tag_ts}
}}

func (t *Telemetry) Validate() error {{
\tif t.Lat < -90 || t.Lat > 90 {{
\t\treturn fmt.Errorf("lat out of range [-90, 90]: %f", t.Lat)
\t}}
\tif t.Lon < -180 || t.Lon > 180 {{
\t\treturn fmt.Errorf("lon out of range [-180, 180]: %f", t.Lon)
\t}}
\tif t.Heading < 0 || t.Heading > 360 {{
\t\treturn fmt.Errorf("heading out of range [0, 360]: %f", t.Heading)
\t}}
\tif _, err := time.Parse(time.RFC3339, t.Timestamp); err != nil {{
\t\tif _, err := time.Parse(time.RFC3339Nano, t.Timestamp); err != nil {{
\t\t\treturn fmt.Errorf("timestamp is not valid RFC3339: %w", err)
\t\t}}
\t}}
\treturn nil
}}

type HazardZoneProperties struct {{
\tID             string  `json:"id"`
\tClass          string  `json:"class"`
\tSeverity       string  `json:"severity"`
\tBufferMeters   float64 `json:"buffer_meters"`
\tAreaSqM        float64 `json:"area_sq_m"`
\tDetectionCount int     `json:"detection_count"`
\tFeatureType    string  `json:"feature_type,omitempty"`
}}

type HazardZoneFeature struct {{
\tType       string                 `json:"type"`
\tID         string                 `json:"id"`
\tGeometry   map[string]interface{{}} `json:"geometry"`
\tProperties HazardZoneProperties   `json:"properties"`
}}

type HazardZonesGeoJSON struct {{
\tType     string              `json:"type"`
\tFeatures []HazardZoneFeature `json:"features"`
}}

func (h *HazardZonesGeoJSON) Validate() error {{
\tif h.Type != "FeatureCollection" {{
\t\treturn fmt.Errorf("invalid type %q", h.Type)
\t}}
\tfor _, f := range h.Features {{
\t\tif f.Type != "Feature" {{
\t\t\treturn fmt.Errorf("invalid feature type %q", f.Type)
\t\t}}
\t\tif f.Properties.ID == "" {{
\t\t\treturn fmt.Errorf("hazard zone ID is required")
\t\t}}
\t\tif f.Properties.BufferMeters < 0 {{
\t\t\treturn fmt.Errorf("buffer meters must be >= 0")
\t\t}}
\t}}
\treturn nil
}}

type SafeRouteProperties struct {{
\tRouteID            string     `json:"route_id"`
\tSafe               bool       `json:"safe"`
\tReason             *string    `json:"reason,omitempty"`
\tLengthMeters       float64    `json:"length_meters"`
\tRiskScore          float64    `json:"risk_score"`
\tAvoidedHazards     []string   `json:"avoided_hazards"`
\tIntersectedHazards []string   `json:"intersected_hazards"`
\tStart              [2]float64 `json:"start"`
\tDestination        [2]float64 `json:"destination"`
}}

type SafeRouteFeature struct {{
\tType       string                 `json:"type"`
\tGeometry   map[string]interface{{}} `json:"geometry"`
\tProperties SafeRouteProperties    `json:"properties"`
}}

type SafeRouteGeoJSON struct {{
\tType     string             `json:"type"`
\tFeatures []SafeRouteFeature `json:"features"`
}}

func (r *SafeRouteGeoJSON) Validate() error {{
\tif r.Type != "FeatureCollection" {{
\t\treturn fmt.Errorf("invalid type %q", r.Type)
\t}}
\tfor _, f := range r.Features {{
\t\tif f.Type != "Feature" {{
\t\t\treturn fmt.Errorf("invalid feature type %q", f.Type)
\t\t}}
\t\tif f.Properties.LengthMeters < 0 {{
\t\t\treturn fmt.Errorf("length meters must be >= 0")
\t\t}}
\t}}
\treturn nil
}}

type CoverageCellProperties struct {{
\tCellID           string   `json:"cell_id"`
\tObservationCount int      `json:"observation_count"`
\tLastObservedAt   *string  `json:"last_observed_at,omitempty"`
\tBestConfidence   float64  `json:"best_confidence"`
\tSearched         bool     `json:"searched"`
\tStalenessSeconds *float64 `json:"staleness_seconds,omitempty"`
}}

type CoverageCellFeature struct {{
\tType       string                 `json:"type"`
\tGeometry   map[string]interface{{}} `json:"geometry"`
\tProperties CoverageCellProperties `json:"properties"`
}}

type CoverageGeoJSON struct {{
\tType     string                `json:"type"`
\tFeatures []CoverageCellFeature `json:"features"`
}}

func (c *CoverageGeoJSON) Validate() error {{
\tif c.Type != "FeatureCollection" {{
\t\treturn fmt.Errorf("invalid type %q", c.Type)
\t}}
\tfor _, f := range c.Features {{
\t\tif f.Type != "Feature" {{
\t\t\treturn fmt.Errorf("invalid feature type %q", f.Type)
\t\t}}
\t\tif f.Properties.CellID == "" {{
\t\t\treturn fmt.Errorf("cell ID is required")
\t\t}}
\t\tif f.Properties.BestConfidence < 0 || f.Properties.BestConfidence > 1 {{
\t\t\treturn fmt.Errorf("best confidence out of range [0, 1]")
\t\t}}
\t}}
\treturn nil
}}
'''
    (GEN_GO / "contracts.go").write_text(code)


if __name__ == "__main__":
    gen_python()
    gen_typescript()
    gen_go()
    print("Successfully generated contracts for Python, TypeScript, and Go in shared/generated/")
