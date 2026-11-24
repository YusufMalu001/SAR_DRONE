"""Boustrophedon (lawnmower) coverage path planner for autonomous aerial SAR missions.

Calculates optimal flight lines, sweep angle, track spacing, and turn handling
for convex and concave mission area polygons based on camera optics and target overlap.
"""

import math
from typing import Dict, List, Optional, Tuple
import numpy as np
from shapely.geometry import Polygon, MultiPolygon, LineString, Point, box
from shapely.ops import unary_union
from shapely import affinity

EARTH_RADIUS_M = 6371000.0
METERS_PER_DEG_LAT = 111320.0


def lat_lon_to_meters(lat: float, lon: float, ref_lat: float, ref_lon: float) -> Tuple[float, float]:
    """Convert WGS84 lat/lon to local Cartesian meters (X=East, Y=North)."""
    d_lat = lat - ref_lat
    d_lon = lon - ref_lon
    y = d_lat * METERS_PER_DEG_LAT
    x = d_lon * (METERS_PER_DEG_LAT * math.cos(math.radians(ref_lat)))
    return x, y


def meters_to_lat_lon(x: float, y: float, ref_lat: float, ref_lon: float) -> Tuple[float, float]:
    """Convert local Cartesian meters back to WGS84 lat/lon."""
    lat = ref_lat + (y / METERS_PER_DEG_LAT)
    meters_per_deg_lon = METERS_PER_DEG_LAT * math.cos(math.radians(ref_lat))
    lon = ref_lon + (x / meters_per_deg_lon)
    return lat, lon


class Waypoint:
    def __init__(self, lat: float, lon: float, alt: float, is_turn: bool = False, speed_mps: float = 5.0):
        self.lat = lat
        self.lon = lon
        self.alt = alt
        self.is_turn = is_turn
        self.speed_mps = speed_mps

    def to_dict(self) -> Dict:
        return {
            "lat": self.lat,
            "lon": self.lon,
            "alt": self.alt,
            "is_turn": self.is_turn,
            "speed_mps": self.speed_mps,
        }


class CoveragePathPlanner:
    def __init__(
        self,
        altitude_m: float = 50.0,
        hfov_deg: float = 90.0,
        vfov_deg: float = 60.0,
        lateral_overlap: float = 0.30,
        forward_overlap: float = 0.40,
        flight_speed_mps: float = 5.0,
        turn_radius_m: float = 5.0,
    ):
        self.altitude_m = altitude_m
        self.hfov_deg = hfov_deg
        self.vfov_deg = vfov_deg
        self.lateral_overlap = max(0.1, min(0.8, lateral_overlap))
        self.forward_overlap = max(0.1, min(0.8, forward_overlap))
        self.flight_speed_mps = flight_speed_mps
        self.turn_radius_m = turn_radius_m

        # Ground footprint dimensions
        self.ground_width_m = 2.0 * self.altitude_m * math.tan(math.radians(self.hfov_deg * 0.5))
        self.ground_height_m = 2.0 * self.altitude_m * math.tan(math.radians(self.vfov_deg * 0.5))

        # Track spacing and waypoint step
        self.track_spacing_m = self.ground_width_m * (1.0 - self.lateral_overlap)
        self.waypoint_spacing_m = self.ground_height_m * (1.0 - self.forward_overlap)

    def plan_coverage(
        self,
        polygon_pts: List[Tuple[float, float]],  # List of (lat, lon)
        sweep_angle_deg: Optional[float] = None,
    ) -> List[Waypoint]:
        """Generate boustrophedon waypoints covering polygon_pts."""
        if len(polygon_pts) < 3:
            raise ValueError("Polygon must have at least 3 vertices")

        ref_lat = polygon_pts[0][0]
        ref_lon = polygon_pts[0][1]

        # Convert polygon to local metric frame
        local_pts = [lat_lon_to_meters(lat, lon, ref_lat, ref_lon) for lat, lon in polygon_pts]
        poly = Polygon(local_pts)
        if not poly.is_valid:
            poly = poly.buffer(0)

        # Optimize sweep angle along minimum bounding rectangle if not provided
        if sweep_angle_deg is None:
            sweep_angle_deg = self._find_optimal_sweep_angle(poly)

        # Rotate polygon so sweep lines are horizontal (parallel to X-axis)
        rot_poly = affinity.rotate(poly, -sweep_angle_deg, origin=(0, 0))
        min_x, min_y, max_x, max_y = rot_poly.bounds

        # Generate sweep lines
        num_tracks = int(math.ceil((max_y - min_y) / self.track_spacing_m))
        if num_tracks < 1:
            num_tracks = 1

        y_coords = np.linspace(min_y + self.track_spacing_m * 0.5, max_y - self.track_spacing_m * 0.5, num_tracks)
        tracks = []

        direction = 1  # 1 for left-to-right, -1 for right-to-left
        for y in y_coords:
            line = LineString([(min_x - 100, y), (max_x + 100, y)])
            inter = rot_poly.intersection(line)
            if inter.is_empty:
                continue

            # Handle multi-line intersections for concave polygons
            segments = []
            if inter.geom_type == "LineString":
                segments.append(inter)
            elif inter.geom_type == "MultiLineString":
                segments.extend(list(inter.geoms))

            # Sort segments along line direction
            segments.sort(key=lambda s: s.coords[0][0])
            if direction < 0:
                segments.reverse()

            for seg in segments:
                coords = list(seg.coords)
                if direction < 0:
                    coords.reverse()
                tracks.append(coords)

            direction *= -1

        # Connect tracks with turn waypoints
        all_rot_wps = []
        for i, track in enumerate(tracks):
            if i > 0 and len(all_rot_wps) > 0 and len(track) > 0:
                # Add turn transition
                prev_pt = all_rot_wps[-1]
                curr_start = track[0]
                turn_wp = (
                    prev_pt[0] + (curr_start[0] - prev_pt[0]) * 0.5,
                    prev_pt[1] + (curr_start[1] - prev_pt[1]) * 0.5,
                )
                all_rot_wps.append((turn_wp[0], turn_wp[1], True))

            for pt in track:
                all_rot_wps.append((pt[0], pt[1], False))

        # Rotate back to original metric frame and convert to WGS84
        waypoints = []
        for item in all_rot_wps:
            x_rot, y_rot, is_turn = item[0], item[1], item[2]
            pt_rot = Point(x_rot, y_rot)
            pt_orig = affinity.rotate(pt_rot, sweep_angle_deg, origin=(0, 0))
            lat, lon = meters_to_lat_lon(pt_orig.x, pt_orig.y, ref_lat, ref_lon)
            waypoints.append(
                Waypoint(
                    lat=lat,
                    lon=lon,
                    alt=self.altitude_m,
                    is_turn=is_turn,
                    speed_mps=self.flight_speed_mps * (0.6 if is_turn else 1.0),
                )
            )

        return waypoints

    def _find_optimal_sweep_angle(self, poly: Polygon) -> float:
        """Find sweep angle that aligns with minimum area bounding box (minimum turns)."""
        mrr = poly.minimum_rotated_rectangle
        if mrr.geom_type != "Polygon":
            return 0.0

        coords = list(mrr.exterior.coords)
        best_angle = 0.0
        max_edge_len = -1.0

        for i in range(len(coords) - 1):
            dx = coords[i + 1][0] - coords[i][0]
            dy = coords[i + 1][1] - coords[i][1]
            edge_len = math.hypot(dx, dy)
            if edge_len > max_edge_len:
                max_edge_len = edge_len
                best_angle = math.degrees(math.atan2(dy, dx))

        return best_angle

    def compute_coverage_fraction(
        self,
        polygon_pts: List[Tuple[float, float]],
        waypoints: List[Waypoint],
    ) -> float:
        """Calculate fraction of target polygon area covered by camera footprints."""
        ref_lat, ref_lon = polygon_pts[0][0], polygon_pts[0][1]
        local_pts = [lat_lon_to_meters(lat, lon, ref_lat, ref_lon) for lat, lon in polygon_pts]
        target_poly = Polygon(local_pts)
        if not target_poly.is_valid:
            target_poly = target_poly.buffer(0)

        hw = self.ground_width_m * 0.5
        hh = self.ground_height_m * 0.5

        footprints = []
        step_size = max(5.0, self.ground_height_m * (1.0 - self.forward_overlap))

        for i in range(len(waypoints) - 1):
            w1 = waypoints[i]
            w2 = waypoints[i + 1]
            x1, y1 = lat_lon_to_meters(w1.lat, w1.lon, ref_lat, ref_lon)
            x2, y2 = lat_lon_to_meters(w2.lat, w2.lon, ref_lat, ref_lon)
            seg_len = math.hypot(x2 - x1, y2 - y1)
            num_steps = max(1, int(math.ceil(seg_len / step_size)) + 1)
            for s in range(num_steps + 1):
                alpha = s / num_steps
                sx = x1 + alpha * (x2 - x1)
                sy = y1 + alpha * (y2 - y1)
                footprints.append(box(sx - hw, sy - hh, sx + hw, sy + hh))

        if not footprints:
            return 0.0

        union_fp = unary_union(footprints)
        covered = target_poly.intersection(union_fp)
        return covered.area / target_poly.area
