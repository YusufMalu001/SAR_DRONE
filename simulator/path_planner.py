"""Coverage path planner and trajectory synthesizer for SAR drone simulation."""

import math
from dataclasses import dataclass
from typing import List, Tuple
import numpy as np

from simulator.scenario import FlightConfig, HomePoint, LatLonPoint

METERS_PER_DEG_LAT = 111320.0
GRAVITY_MPS2 = 9.80665


@dataclass(frozen=True)
class TelemetrySample:
    elapsed_s: float
    lat: float
    lon: float
    alt: float
    heading: float
    pitch: float
    roll: float
    is_gps_valid: bool = True

    def to_stream_dict(self, timestamp: str) -> dict:
        """Production Redis stream payload matching telemetry.schema.json."""
        return {
            "lat": round(self.lat, 7),
            "lon": round(self.lon, 7),
            "alt": round(self.alt, 2),
            "heading": round(self.heading % 360.0, 2),
            "pitch": round(self.pitch, 2),
            "roll": round(self.roll, 2),
            "timestamp": timestamp,
        }


def meters_per_deg_lon(lat_deg: float) -> float:
    return METERS_PER_DEG_LAT * math.cos(math.radians(lat_deg))


def compute_bearing(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """Computes initial bearing from (lat1, lon1) to (lat2, lon2) in degrees [0, 360)."""
    phi1, phi2 = math.radians(lat1), math.radians(lat2)
    delta_lambda = math.radians(lon2 - lon1)

    y = math.sin(delta_lambda) * math.cos(phi2)
    x = math.cos(phi1) * math.sin(phi2) - math.sin(phi1) * math.cos(phi2) * math.cos(delta_lambda)
    bearing = math.degrees(math.atan2(y, x))
    return (bearing + 360.0) % 360.0


def haversine_distance_m(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """Great circle distance between two points in meters."""
    r = 6371000.0
    phi1, phi2 = math.radians(lat1), math.radians(lat2)
    dphi = math.radians(lat2 - lat1)
    dlam = math.radians(lon2 - lon1)

    a = math.sin(dphi / 2.0) ** 2 + math.cos(phi1) * math.cos(phi2) * math.sin(dlam / 2.0) ** 2
    c = 2.0 * math.atan2(math.sqrt(a), math.sqrt(1.0 - a))
    return r * c


class TrajectorySynthesizer:
    """Generates deterministic, physically grounded UAV flight trajectories across an area polygon."""

    def __init__(
        self,
        polygon: List[LatLonPoint],
        home: HomePoint,
        flight: FlightConfig,
        hfov_deg: float = 90.0,
    ):
        self.polygon = polygon
        self.home = home
        self.flight = flight
        self.hfov_deg = hfov_deg
        self.waypoints = self._generate_search_grid()

    def _generate_search_grid(self) -> List[Tuple[float, float, float]]:
        """Generates a boustrophedon (lawnmower) survey grid over polygon bounds."""
        lats = [p.lat for p in self.polygon]
        lons = [p.lon for p in self.polygon]
        min_lat, max_lat = min(lats), max(lats)
        min_lon, max_lon = min(lons), max(lons)

        center_lat = (min_lat + max_lat) / 2.0
        m_lon = meters_per_deg_lon(center_lat)

        # Ground swath width from altitude & HFOV (with 40% lateral overlap)
        ground_width_m = 2.0 * self.flight.altitude * math.tan(math.radians(self.hfov_deg / 2.0))
        swath_spacing_m = max(15.0, ground_width_m * 0.60)
        swath_spacing_lat = swath_spacing_m / METERS_PER_DEG_LAT

        waypoints: List[Tuple[float, float, float]] = []
        # Start at home point
        waypoints.append((self.home.lat, self.home.lon, self.flight.altitude))

        current_lat = min_lat
        sweep_east = True

        while current_lat <= max_lat + 1e-6:
            if sweep_east:
                waypoints.append((current_lat, min_lon, self.flight.altitude))
                waypoints.append((current_lat, max_lon, self.flight.altitude))
            else:
                waypoints.append((current_lat, max_lon, self.flight.altitude))
                waypoints.append((current_lat, min_lon, self.flight.altitude))

            sweep_east = not sweep_east
            current_lat += swath_spacing_lat

        # Return to home
        waypoints.append((self.home.lat, self.home.lon, self.flight.altitude))
        return waypoints

    def generate_trajectory(
        self,
        duration_s: float,
        gps_outages: List[Tuple[float, float]] | None = None,
    ) -> List[TelemetrySample]:
        """Synthesizes deterministic telemetry samples at sensor_rate_hz with realistic noise."""
        rate = self.flight.sensor_rate_hz
        dt = 1.0 / rate
        total_steps = int(math.ceil(duration_s * rate))
        outages = gps_outages or []

        # Deterministic PRNG seeded from scenario flight config
        rng = np.random.default_rng(seed=self.flight.seed)
        noise = self.flight.noise

        # Precompute segment distances along waypoints
        seg_dists: List[float] = []
        for i in range(len(self.waypoints) - 1):
            w1 = self.waypoints[i]
            w2 = self.waypoints[i + 1]
            seg_dists.append(haversine_distance_m(w1[0], w1[1], w2[0], w2[1]))

        total_path_m = sum(seg_dists)
        speed = self.flight.speed

        samples: List[TelemetrySample] = []
        prev_heading = 0.0

        for step in range(total_steps):
            elapsed_s = step * dt
            distance_flown = (elapsed_s * speed) % total_path_m if total_path_m > 0 else 0.0

            # Find active segment along waypoints
            cum_dist = 0.0
            seg_idx = 0
            seg_progress = 0.0

            for idx, d in enumerate(seg_dists):
                if cum_dist + d >= distance_flown:
                    seg_idx = idx
                    seg_progress = (distance_flown - cum_dist) / d if d > 0 else 0.0
                    break
                cum_dist += d

            w_start = self.waypoints[seg_idx]
            w_end = self.waypoints[seg_idx + 1]

            # Interpolate ideal position
            ideal_lat = w_start[0] + seg_progress * (w_end[0] - w_start[0])
            ideal_lon = w_start[1] + seg_progress * (w_end[1] - w_start[1])
            ideal_alt = self.flight.altitude

            # Heading along current segment
            ideal_heading = compute_bearing(w_start[0], w_start[1], w_end[0], w_end[1])

            # Rate of turn & bank angle (roll)
            heading_diff = (ideal_heading - prev_heading + 180.0) % 360.0 - 180.0
            turn_rate_rad_s = math.radians(heading_diff) / dt if step > 0 else 0.0
            # Coordinated turn: tan(roll) = (v * omega) / g
            ideal_roll_deg = math.degrees(math.atan((speed * turn_rate_rad_s) / GRAVITY_MPS2))
            ideal_roll_deg = max(-25.0, min(25.0, ideal_roll_deg))

            # Pitch: slight nose-down in forward flight
            ideal_pitch_deg = -2.5

            # Inject calibrated Gaussian noise using the fixed seed
            lat_noise_m = rng.normal(0.0, noise.pos_std_m)
            lon_noise_m = rng.normal(0.0, noise.pos_std_m)
            alt_noise_m = rng.normal(0.0, noise.alt_std_m)
            heading_noise = rng.normal(0.0, noise.heading_std_deg)
            roll_noise = rng.normal(0.0, noise.attitude_std_deg)
            pitch_noise = rng.normal(0.0, noise.attitude_std_deg)

            noisy_lat = ideal_lat + (lat_noise_m / METERS_PER_DEG_LAT)
            noisy_lon = ideal_lon + (lon_noise_m / meters_per_deg_lon(ideal_lat))
            noisy_alt = ideal_alt + alt_noise_m
            noisy_heading = (ideal_heading + heading_noise) % 360.0
            noisy_roll = ideal_roll_deg + roll_noise
            noisy_pitch = ideal_pitch_deg + pitch_noise

            # Check if current time falls into a GPS outage window
            is_gps_valid = not any(start <= elapsed_s <= end for start, end in outages)

            samples.append(
                TelemetrySample(
                    elapsed_s=round(elapsed_s, 4),
                    lat=noisy_lat,
                    lon=noisy_lon,
                    alt=noisy_alt,
                    heading=noisy_heading,
                    pitch=noisy_pitch,
                    roll=noisy_roll,
                    is_gps_valid=is_gps_valid,
                )
            )
            prev_heading = ideal_heading

        return samples
