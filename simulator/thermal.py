"""Thermal video reader and synthetic radiometric rendering pipeline."""

from pathlib import Path
from typing import Optional
import cv2
import numpy as np


class ThermalRenderingPipeline:
    """Transforms standard RGB imagery into simulated radiometric thermal/infrared frames.

    Used when physical LWIR thermal footage is unavailable, simulating human and flame hotspots.
    """

    def __init__(self, method: str = "radiometric_colormap"):
        self.method = method

    def render(self, rgb_frame: np.ndarray) -> np.ndarray:
        """Converts an RGB/BGR frame to thermal palette with simulated heat emission."""
        # Convert BGR to gray luminance as thermal base
        gray = cv2.cvtColor(rgb_frame, cv2.COLOR_BGR2GRAY).astype(np.float32)

        # Hotspot extraction: highlight areas with prominent red/warm saturation (skin tones, fire)
        b, g, r = cv2.split(rgb_frame.astype(np.float32))
        warmth = np.maximum(0.0, r - (b + g) / 2.0)
        # Scale warmth boost
        thermal_intensity = gray * 0.7 + warmth * 1.5
        thermal_intensity = np.clip(thermal_intensity, 0.0, 255.0).astype(np.uint8)

        if self.method == "white_hot":
            return cv2.cvtColor(thermal_intensity, cv2.COLOR_GRAY2BGR)
        elif self.method == "ironbow":
            return cv2.applyColorMap(thermal_intensity, cv2.COLORMAP_JET)
        else:
            # Default radiometric colormap (Inferno)
            return cv2.applyColorMap(thermal_intensity, cv2.COLORMAP_INFERNO)


class ThermalVideoSource:
    """Provides thermal frames either from a pre-recorded thermal clip or via real-time rendering."""

    def __init__(
        self,
        thermal_video_path: Optional[str] = None,
        use_pipeline: bool = True,
        pipeline_method: str = "radiometric_colormap",
    ):
        self.cap = None
        self.pipeline = ThermalRenderingPipeline(method=pipeline_method) if use_pipeline else None

        if thermal_video_path and Path(thermal_video_path).is_file():
            self.cap = cv2.VideoCapture(thermal_video_path)

    def get_frame(self, rgb_frame: np.ndarray) -> np.ndarray:
        if self.cap is not None and self.cap.isOpened():
            ok, frame = self.cap.read()
            if ok:
                return frame
            # Loop video if needed
            self.cap.set(cv2.CAP_PROP_POS_FRAMES, 0)
            ok, frame = self.cap.read()
            if ok:
                return frame

        # Fallback to thermal rendering pipeline
        if self.pipeline is not None:
            return self.pipeline.render(rgb_frame)

        return rgb_frame

    def release(self):
        if self.cap is not None:
            self.cap.release()
            self.cap = None
