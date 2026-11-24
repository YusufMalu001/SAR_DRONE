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
from typing import List, Optional
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
'''
    (GEN_GO / "contracts.go").write_text(code)


if __name__ == "__main__":
    gen_python()
    gen_typescript()
    gen_go()
    print("Successfully generated contracts for Python, TypeScript, and Go in shared/generated/")
