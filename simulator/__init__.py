"""Deterministic simulation layer for SAR drone prototype."""

from simulator.scenario import Scenario
from simulator.path_planner import TrajectorySynthesizer, TelemetrySample
from simulator.telemetry import TelemetrySynthesizer, compute_telemetry_digest
from simulator.annotation import GeotagEvaluator, project_detection

__all__ = [
    "Scenario",
    "TrajectorySynthesizer",
    "TelemetrySample",
    "TelemetrySynthesizer",
    "compute_telemetry_digest",
    "GeotagEvaluator",
    "project_detection",
]
