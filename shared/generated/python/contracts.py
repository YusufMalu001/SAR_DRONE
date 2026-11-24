"""Generated from shared/jsonschema — DO NOT EDIT MANUALLY."""
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
