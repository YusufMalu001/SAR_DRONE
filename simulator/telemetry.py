"""Telemetry synthesis and serialization for the SAR Drone simulation layer."""

import hashlib
import json
from datetime import datetime, timezone
from typing import List, Optional

from simulator.path_planner import TelemetrySample, TrajectorySynthesizer
from simulator.scenario import Scenario


def format_iso_timestamp(dt: datetime) -> str:
    """Formats datetime as RFC3339 UTC string with Z suffix."""
    return dt.astimezone(timezone.utc).isoformat().replace("+00:00", "Z")


def compute_telemetry_digest(samples: List[TelemetrySample]) -> str:
    """Computes a SHA256 hex digest of the serialized telemetry samples to verify byte-identical reproducibility."""
    hasher = hashlib.sha256()
    for s in samples:
        line = f"{s.elapsed_s:.4f},{s.lat:.8f},{s.lon:.8f},{s.alt:.4f},{s.heading:.4f},{s.pitch:.4f},{s.roll:.4f},{s.is_gps_valid}\n"
        hasher.update(line.encode("utf-8"))
    return hasher.hexdigest()


class TelemetrySynthesizer:
    """Manages trajectory synthesis and telemetry sample stream generation for a scenario."""

    def __init__(self, scenario: Scenario):
        self.scenario = scenario
        self.planner = TrajectorySynthesizer(
            polygon=scenario.area_polygon,
            home=scenario.home_point,
            flight=scenario.flight,
            hfov_deg=scenario.camera.hfov_deg,
        )

    def generate(self, duration_s: float) -> List[TelemetrySample]:
        outages = [(w.t_start_s, w.t_end_s) for w in self.scenario.gps_outages]
        return self.planner.generate_trajectory(duration_s=duration_s, gps_outages=outages)
