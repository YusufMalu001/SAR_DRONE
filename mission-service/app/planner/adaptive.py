"""Adaptive search state machine for SAR drone missions.

Governs dynamic plan adaptation:
- Orbit/loiter around newly detected survivor candidates to confirm identification from multiple angles.
- Targeted localized re-planning when coverage map reports low-confidence or unsearched cells.
- Seamless resumption of main survey path upon confirmation.
"""

from enum import Enum
import math
from typing import Dict, List, Optional, Tuple
from app.planner.coverage import Waypoint, lat_lon_to_meters, meters_to_lat_lon


class SearchState(str, Enum):
    NORMAL_COVERAGE = "NORMAL_COVERAGE"
    SURVIVOR_CONFIRMATION = "SURVIVOR_CONFIRMATION"
    LOW_CONFIDENCE_REVISIT = "LOW_CONFIDENCE_REVISIT"
    RESUMING = "RESUMING"
    COMPLETED = "COMPLETED"


class AdaptiveSearchManager:
    def __init__(
        self,
        main_waypoints: List[Waypoint],
        orbit_radius_m: float = 15.0,
        orbit_altitude_m: float = 35.0,
        orbit_speed_mps: float = 3.0,
        orbit_points: int = 8,
        min_revisit_confidence: float = 0.40,
    ):
        self.main_waypoints = list(main_waypoints)
        self.current_main_idx = 0
        self.state = SearchState.NORMAL_COVERAGE
        self.orbit_radius_m = orbit_radius_m
        self.orbit_altitude_m = orbit_altitude_m
        self.orbit_speed_mps = orbit_speed_mps
        self.orbit_points = orbit_points
        self.min_revisit_confidence = min_revisit_confidence

        # Sub-mission queues
        self.active_sub_waypoints: List[Waypoint] = []
        self.confirmed_survivors: List[Tuple[float, float]] = []
        self.revisited_cells: List[str] = []

    def get_current_waypoint(self) -> Optional[Waypoint]:
        """Return next target waypoint in active state."""
        if self.state == SearchState.NORMAL_COVERAGE:
            if self.current_main_idx < len(self.main_waypoints):
                return self.main_waypoints[self.current_main_idx]
            return None
        elif self.state in (SearchState.SURVIVOR_CONFIRMATION, SearchState.LOW_CONFIDENCE_REVISIT):
            if len(self.active_sub_waypoints) > 0:
                return self.active_sub_waypoints[0]
            return None
        elif self.state == SearchState.RESUMING:
            if self.current_main_idx < len(self.main_waypoints):
                return self.main_waypoints[self.current_main_idx]
            return None
        return None

    def advance_waypoint(self):
        """Called when drone reaches current waypoint."""
        if self.state in (SearchState.SURVIVOR_CONFIRMATION, SearchState.LOW_CONFIDENCE_REVISIT):
            if len(self.active_sub_waypoints) > 0:
                self.active_sub_waypoints.pop(0)
            if len(self.active_sub_waypoints) == 0:
                # Sub-task finished -> resume main mission
                self.state = SearchState.RESUMING
        elif self.state in (SearchState.NORMAL_COVERAGE, SearchState.RESUMING):
            self.state = SearchState.NORMAL_COVERAGE
            self.current_main_idx += 1
            if self.current_main_idx >= len(self.main_waypoints):
                self.state = SearchState.COMPLETED

    def on_survivor_detected(self, lat: float, lon: float, confidence: float) -> bool:
        """Triggered upon detection of survivor candidate."""
        if confidence < 0.50:
            return False

        # Check if already confirmed or currently confirming
        for s_lat, s_lon in self.confirmed_survivors:
            d_lat = (lat - s_lat) * 111320.0
            d_lon = (lon - s_lon) * 111320.0 * math.cos(math.radians(lat))
            if math.hypot(d_lat, d_lon) < self.orbit_radius_m * 2.0:
                return False

        self.confirmed_survivors.append((lat, lon))
        self.state = SearchState.SURVIVOR_CONFIRMATION

        # Generate circular orbit waypoints around candidate
        self.active_sub_waypoints = self._generate_orbit_waypoints(lat, lon)
        return True

    def on_coverage_update(self, low_confidence_cells: List[Dict]) -> bool:
        """Triggered when coverage map reports unsearched/low-confidence cells."""
        # Only re-plan revisits if not currently in a survivor confirmation
        if self.state == SearchState.SURVIVOR_CONFIRMATION:
            return False

        candidates = [c for c in low_confidence_cells if c["cell_id"] not in self.revisited_cells and c.get("best_confidence", 0) < self.min_revisit_confidence]
        if not candidates:
            return False

        target_cell = candidates[0]
        self.revisited_cells.append(target_cell["cell_id"])
        self.state = SearchState.LOW_CONFIDENCE_REVISIT

        # Generate inspection cross/box over cell
        c_lat = (target_cell["min_lat"] + target_cell["max_lat"]) * 0.5
        c_lon = (target_cell["min_lon"] + target_cell["max_lon"]) * 0.5

        self.active_sub_waypoints = [
            Waypoint(lat=c_lat, lon=c_lon, alt=self.orbit_altitude_m, speed_mps=self.orbit_speed_mps),
            Waypoint(lat=c_lat + 0.0001, lon=c_lon, alt=self.orbit_altitude_m, speed_mps=self.orbit_speed_mps),
            Waypoint(lat=c_lat - 0.0001, lon=c_lon, alt=self.orbit_altitude_m, speed_mps=self.orbit_speed_mps),
        ]
        return True

    def _generate_orbit_waypoints(self, center_lat: float, center_lon: float) -> List[Waypoint]:
        """Generate N circle waypoints around center coordinates."""
        wps = []
        d_theta = (2.0 * math.pi) / self.orbit_points
        for i in range(self.orbit_points + 1):  # +1 to close circle
            theta = i * d_theta
            dx = self.orbit_radius_m * math.cos(theta)
            dy = self.orbit_radius_m * math.sin(theta)
            lat, lon = meters_to_lat_lon(dx, dy, center_lat, center_lon)
            wps.append(
                Waypoint(
                    lat=lat,
                    lon=lon,
                    alt=self.orbit_altitude_m,
                    is_turn=True,
                    speed_mps=self.orbit_speed_mps,
                )
            )
        return wps
