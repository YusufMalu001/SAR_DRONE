"""Scenario parser and Pydantic models for SAR Drone simulation."""

from pathlib import Path
from typing import Any, Dict, List, Literal, Optional
from pydantic import BaseModel, Field
import yaml


class LatLonPoint(BaseModel):
    lat: float = Field(ge=-90.0, le=90.0)
    lon: float = Field(ge=-180.0, le=180.0)


class HomePoint(LatLonPoint):
    alt: float = Field(default=0.0, ge=0.0)


class NoiseConfig(BaseModel):
    pos_std_m: float = Field(default=0.2, ge=0.0)
    alt_std_m: float = Field(default=0.1, ge=0.0)
    heading_std_deg: float = Field(default=0.5, ge=0.0)
    attitude_std_deg: float = Field(default=0.5, ge=0.0)


class FlightConfig(BaseModel):
    altitude: float = Field(default=50.0, gt=0.0)
    speed: float = Field(default=5.0, gt=0.0)
    sensor_rate_hz: float = Field(default=5.0, gt=0.0)
    seed: int = Field(default=42)
    noise: NoiseConfig = Field(default_factory=NoiseConfig)


class CameraConfig(BaseModel):
    hfov_deg: float = Field(default=90.0, gt=0.0, lt=180.0)
    vfov_deg: float = Field(default=60.0, gt=0.0, lt=180.0)
    fps: float = Field(default=5.0, gt=0.0)


class ThermalPipelineConfig(BaseModel):
    enabled: bool = True
    method: Literal["radiometric_colormap", "white_hot", "ironbow"] = "radiometric_colormap"


class VideoConfig(BaseModel):
    rgb_path: str
    thermal_path: Optional[str] = None
    thermal_pipeline: ThermalPipelineConfig = Field(default_factory=ThermalPipelineConfig)


class BBoxConfig(BaseModel):
    x: float = Field(ge=0.0, le=1.0)
    y: float = Field(ge=0.0, le=1.0)
    w: float = Field(gt=0.0, le=1.0)
    h: float = Field(gt=0.0, le=1.0)


class GroundTruthObject(BaseModel):
    id: str
    type: Literal["person", "fire", "flood", "debris", "structural_damage"]
    lat: float = Field(ge=-90.0, le=90.0)
    lon: float = Field(ge=-180.0, le=180.0)
    appearance_frames: List[int] = Field(min_length=2, max_length=2)
    bbox: BBoxConfig
    confidence: float = Field(default=0.90, ge=0.0, le=1.0)
    description: str = ""


class ConnectivityEvent(BaseModel):
    type: Literal["link_down", "latency_spike"] = "link_down"
    t_start_s: float = Field(ge=0.0)
    t_end_s: float = Field(ge=0.0)
    action: Literal["set_offline", "pause_postgres"] = "set_offline"
    description: str = ""


class GPSOutageWindow(BaseModel):
    t_start_s: float = Field(ge=0.0)
    t_end_s: float = Field(ge=0.0)
    description: str = ""


class Scenario(BaseModel):
    name: str
    description: str = ""
    area_polygon: List[LatLonPoint] = Field(min_length=3)
    home_point: HomePoint
    flight: FlightConfig = Field(default_factory=FlightConfig)
    camera: CameraConfig = Field(default_factory=CameraConfig)
    video: VideoConfig
    ground_truth_objects: List[GroundTruthObject] = Field(default_factory=list)
    connectivity_events: List[ConnectivityEvent] = Field(default_factory=list)
    gps_outages: List[GPSOutageWindow] = Field(default_factory=list)

    @classmethod
    def load(cls, path: str | Path) -> "Scenario":
        p = Path(path)
        if not p.is_file():
            raise FileNotFoundError(f"Scenario YAML not found at: {p}")
        with p.open("r", encoding="utf-8") as f:
            data = yaml.safe_load(f)
        return cls(**data)
