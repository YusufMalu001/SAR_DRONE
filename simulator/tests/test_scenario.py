from pathlib import Path
import pytest
from pydantic import ValidationError

from simulator.scenario import Scenario, LatLonPoint, BBoxConfig


def test_load_all_provided_scenarios():
    for name in ["flood", "fire", "collapse"]:
        path = Path(f"scenarios/{name}.yaml")
        assert path.exists(), f"Scenario file {path} missing"
        scenario = Scenario.load(path)
        assert scenario.name == name
        assert len(scenario.area_polygon) >= 3
        assert scenario.flight.altitude > 0
        assert scenario.flight.speed > 0
        assert scenario.camera.fps > 0
        assert len(scenario.ground_truth_objects) >= 2
        assert len(scenario.connectivity_events) >= 1
        assert len(scenario.gps_outages) >= 1

        for gt in scenario.ground_truth_objects:
            assert gt.type in {"person", "fire", "flood", "debris", "structural_damage"}
            assert -90.0 <= gt.lat <= 90.0
            assert -180.0 <= gt.lon <= 180.0
            assert gt.appearance_frames[0] <= gt.appearance_frames[1]
            assert 0.0 <= gt.bbox.x <= 1.0
            assert 0.0 <= gt.bbox.y <= 1.0
            assert 0.0 < gt.bbox.w <= 1.0
            assert 0.0 < gt.bbox.h <= 1.0


def test_invalid_polygon_rejected():
    invalid_data = {
        "name": "invalid",
        "area_polygon": [{"lat": 20.0, "lon": 78.0}],  # < 3 points
        "home_point": {"lat": 20.0, "lon": 78.0, "alt": 50.0},
        "video": {"rgb_path": "dummy.mp4"},
    }
    with pytest.raises(ValidationError):
        Scenario(**invalid_data)


def test_invalid_bbox_rejected():
    with pytest.raises(ValidationError):
        BBoxConfig(x=1.5, y=0.5, w=0.5, h=0.5)

    with pytest.raises(ValidationError):
        BBoxConfig(x=0.5, y=0.5, w=-0.1, h=0.5)
