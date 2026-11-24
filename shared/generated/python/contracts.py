"""Generated from shared/jsonschema — DO NOT EDIT MANUALLY."""
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
