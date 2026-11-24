"""Ground-truth annotation tooling and geotagging error evaluation.

Given scenario ground truth and drone telemetry, projects bounding box detections
to ground coordinates and computes metric geolocation error against true target positions.
"""

import argparse
import json
import math
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
import urllib.request

from simulator.scenario import Scenario

METERS_PER_DEG_LAT = 111320.0


def haversine_distance_m(lat1: float, lon1: float, lat2: float, lon2: float) -> float:
    """Computes great-circle distance between two WGS84 points in meters."""
    r = 6371000.0
    phi1, phi2 = math.radians(lat1), math.radians(lat2)
    dphi = math.radians(lat2 - lat1)
    dlam = math.radians(lon2 - lon1)

    a = math.sin(dphi / 2.0) ** 2 + math.cos(phi1) * math.cos(phi2) * math.sin(dlam / 2.0) ** 2
    c = 2.0 * math.atan2(math.sqrt(a), math.sqrt(1.0 - a))
    return r * c


def project_detection(
    bbox_cx: float,
    bbox_cy: float,
    hfov_deg: float,
    vfov_deg: float,
    alt_m: float,
    heading_deg: float,
    drone_lat: float,
    drone_lon: float,
) -> Tuple[float, float]:
    """Pinhole camera projection matching fusion-service/internal/geo/geo.go."""
    if alt_m <= 0:
        return drone_lat, drone_lon

    angle_x = (bbox_cx - 0.5) * hfov_deg
    angle_y = (bbox_cy - 0.5) * vfov_deg

    right = alt_m * math.tan(math.radians(angle_x))
    forward = alt_m * math.tan(math.radians(-angle_y))

    heading_rad = math.radians(heading_deg)
    north = forward * math.cos(heading_rad) - right * math.sin(heading_rad)
    east = forward * math.sin(heading_rad) + right * math.cos(heading_rad)

    delta_lat = north / METERS_PER_DEG_LAT
    delta_lon = east / (METERS_PER_DEG_LAT * math.cos(math.radians(drone_lat)))

    return drone_lat + delta_lat, drone_lon + delta_lon


def export_ground_truth_file(scenario: Scenario, output_path: str | Path) -> dict:
    """Exports structured ground truth JSON file for a scenario."""
    gt_data = {
        "scenario_name": scenario.name,
        "description": scenario.description,
        "home_point": scenario.home_point.model_dump(),
        "area_polygon": [p.model_dump() for p in scenario.area_polygon],
        "flight": scenario.flight.model_dump(),
        "camera": scenario.camera.model_dump(),
        "ground_truth_objects": [obj.model_dump() for obj in scenario.ground_truth_objects],
        "connectivity_events": [ev.model_dump() for ev in scenario.connectivity_events],
        "gps_outages": [w.model_dump() for w in scenario.gps_outages],
    }

    p = Path(output_path)
    p.parent.mkdir(parents=True, exist_ok=True)
    with p.open("w", encoding="utf-8") as f:
        json.dump(gt_data, f, indent=2)

    return gt_data


class GeotagEvaluator:
    """Matches emitted detections against scenario ground truth and computes metric error."""

    def __init__(self, scenario: Scenario):
        self.scenario = scenario
        self.gt_objects = scenario.ground_truth_objects

    def match_and_evaluate(self, detection: dict) -> Optional[dict]:
        """Finds closest ground-truth object of matching class and calculates geotag error in meters."""
        det_class = detection.get("class")
        det_lat = detection.get("lat")
        det_lon = detection.get("lon")

        if det_lat is None or det_lon is None:
            return None

        candidates = [gt for gt in self.gt_objects if gt.type == det_class]
        if not candidates:
            return None

        # Find closest ground truth object
        best_gt = None
        min_dist = float("inf")
        for gt in candidates:
            dist = haversine_distance_m(det_lat, det_lon, gt.lat, gt.lon)
            if dist < min_dist:
                min_dist = dist
                best_gt = gt

        return {
            "detection_id": detection.get("id"),
            "class": det_class,
            "detected_lat": det_lat,
            "detected_lon": det_lon,
            "true_lat": best_gt.lat,
            "true_lon": best_gt.lon,
            "ground_truth_id": best_gt.id,
            "error_meters": round(min_dist, 3),
        }

    def evaluate_batch(self, detections: List[dict]) -> dict:
        """Evaluates a batch of detections, returning comprehensive error statistics."""
        results = []
        errors = []

        for d in detections:
            eval_res = self.match_and_evaluate(d)
            if eval_res:
                results.append(eval_res)
                errors.append(eval_res["error_meters"])

        if not errors:
            return {
                "matched_count": 0,
                "mean_error_m": 0.0,
                "median_error_m": 0.0,
                "max_error_m": 0.0,
                "rmse_m": 0.0,
                "evaluations": [],
            }

        mean_err = sum(errors) / len(errors)
        sorted_errs = sorted(errors)
        median_err = sorted_errs[len(sorted_errs) // 2]
        max_err = max(errors)
        rmse = math.sqrt(sum(e * e for e in errors) / len(errors))

        return {
            "matched_count": len(results),
            "mean_error_m": round(mean_err, 3),
            "median_error_m": round(median_err, 3),
            "max_error_m": round(max_err, 3),
            "rmse_m": round(rmse, 3),
            "evaluations": results,
        }


def fetch_detections_from_fusion(fusion_url: str = "http://localhost:8002", limit: int = 200) -> List[dict]:
    """Retrieves live detections persisted in fusion-service."""
    url = f"{fusion_url.rstrip('/')}/detections?limit={limit}"
    req = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=5.0) as resp:
            data = json.loads(resp.read().decode("utf-8"))
            return data if isinstance(data, list) else []
    except Exception as e:
        print(f"Error fetching detections from fusion-service: {e}")
        return []


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scenario", required=True, help="Path to scenario YAML file")
    parser.add_argument("--export-gt", help="Export scenario ground-truth to JSON file path")
    parser.add_argument("--evaluate", action="store_true", help="Evaluate live detections from fusion-service")
    parser.add_argument("--fusion-url", default="http://localhost:8002", help="Fusion service base URL")
    args = parser.parse_args()

    scenario = Scenario.load(args.scenario)

    if args.export_gt:
        export_ground_truth_file(scenario, args.export_gt)
        print(f"Exported ground truth to: {args.export_gt}")

    if args.evaluate:
        print(f"Evaluating live detections against scenario '{scenario.name}' ground truth...")
        detections = fetch_detections_from_fusion(args.fusion_url)
        evaluator = GeotagEvaluator(scenario)
        stats = evaluator.evaluate_batch(detections)
        print(f"Evaluation Summary:")
        print(f"  Matched Detections: {stats['matched_count']}")
        print(f"  Mean Error:         {stats['mean_error_m']} meters")
        print(f"  Median Error:       {stats['median_error_m']} meters")
        print(f"  Max Error:          {stats['max_error_m']} meters")
        print(f"  RMSE:               {stats['rmse_m']} meters")


if __name__ == "__main__":
    main()
