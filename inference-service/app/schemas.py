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
    x: float = Field(ge=0, le=1)
    y: float = Field(ge=0, le=1)
    w: float = Field(ge=0, le=1)
    h: float = Field(ge=0, le=1)


class StageTimings(BaseModel):
    decode: Optional[float] = None
    preprocess: Optional[float] = None
    inference: Optional[float] = None
    postprocess: Optional[float] = None
    publish: Optional[float] = None
    total: Optional[float] = None


class Detection(BaseModel):
    """Mirrors shared/jsonschema/detection.schema.json."""

    id: str
    class_: DetectionClass = Field(alias="class")
    confidence: float = Field(ge=0, le=1)
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
    """Mirrors shared/jsonschema/telemetry.schema.json."""

    lat: float = Field(ge=-90, le=90)
    lon: float = Field(ge=-180, le=180)
    alt: float
    heading: float = Field(ge=0, le=360)
    timestamp: str
