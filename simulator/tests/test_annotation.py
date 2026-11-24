from pathlib import Path
import json
from simulator.scenario import Scenario
from simulator.annotation import (
    project_detection,
    haversine_distance_m,
    GeotagEvaluator,
    export_ground_truth_file,
)


def test_pinhole_projection_nadir():
    # Nadir center of frame (0.5, 0.5) should project to drone location
    lat, lon = project_detection(
        bbox_cx=0.5,
        bbox_cy=0.5,
        hfov_deg=90.0,
        vfov_deg=60.0,
        alt_m=50.0,
        heading_deg=0.0,
        drone_lat=20.0,
        drone_lon=78.0,
    )
    assert abs(lat - 20.0) < 1e-7
    assert abs(lon - 78.0) < 1e-7


def test_ground_truth_files_exist_and_match():
    for sc_name in ["flood", "fire", "collapse"]:
        gt_path = Path(f"scenarios/{sc_name}_ground_truth.json")
        assert gt_path.is_file(), f"Ground-truth file {gt_path} must exist"

        with gt_path.open() as f:
            data = json.load(f)

        assert data["scenario_name"] == sc_name
        assert len(data["ground_truth_objects"]) >= 2
        for obj in data["ground_truth_objects"]:
            assert "id" in obj
            assert "type" in obj
            assert "lat" in obj
            assert "lon" in obj
            assert "appearance_frames" in obj


def test_geotag_evaluator_statistics():
    scenario = Scenario.load("scenarios/flood.yaml")
    evaluator = GeotagEvaluator(scenario)

    # Test detections close to true coordinates
    target = scenario.ground_truth_objects[0]
    detections = [
        {
            "id": "det-1",
            "class": target.type,
            "lat": target.lat + 0.00001,  # ~1.1 meters offset
            "lon": target.lon,
        },
        {
            "id": "det-2",
            "class": target.type,
            "lat": target.lat + 0.00002,  # ~2.2 meters offset
            "lon": target.lon,
        },
    ]

    stats = evaluator.evaluate_batch(detections)
    assert stats["matched_count"] == 2
    assert 1.0 < stats["mean_error_m"] < 2.5
    assert stats["rmse_m"] > 0.0
    assert stats["max_error_m"] >= stats["median_error_m"]
