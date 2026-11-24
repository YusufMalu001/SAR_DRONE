from pathlib import Path
import pytest

from simulator.scenario import Scenario
from simulator.path_planner import TrajectorySynthesizer, haversine_distance_m, compute_bearing


def test_haversine_and_bearing():
    # Distance between (0, 0) and (0, 1) roughly 111.3 km
    d = haversine_distance_m(0.0, 0.0, 0.0, 1.0)
    assert 111000.0 < d < 112000.0

    # East bearing
    b_east = compute_bearing(0.0, 0.0, 0.0, 1.0)
    assert abs(b_east - 90.0) < 0.1

    # North bearing
    b_north = compute_bearing(0.0, 0.0, 1.0, 0.0)
    assert abs(b_north - 0.0) < 0.1 or abs(b_north - 360.0) < 0.1


def test_trajectory_synthesis_properties():
    scenario = Scenario.load("scenarios/flood.yaml")
    synth = TrajectorySynthesizer(
        polygon=scenario.area_polygon,
        home=scenario.home_point,
        flight=scenario.flight,
        hfov_deg=scenario.camera.hfov_deg,
    )

    outages = [(10.0, 13.0)]
    samples = synth.generate_trajectory(duration_s=15.0, gps_outages=outages)

    assert len(samples) == int(15.0 * scenario.flight.sensor_rate_hz)

    # Check timestamps and continuity
    for idx, s in enumerate(samples):
        expected_time = round(idx / scenario.flight.sensor_rate_hz, 4)
        assert abs(s.elapsed_s - expected_time) < 1e-4
        assert 0.0 <= s.heading <= 360.0
        assert -30.0 <= s.roll <= 30.0
        assert -10.0 <= s.pitch <= 5.0
        assert scenario.flight.altitude - 5.0 <= s.alt <= scenario.flight.altitude + 5.0

        # Outage check
        if 10.0 <= s.elapsed_s <= 13.0:
            assert s.is_gps_valid is False
        else:
            assert s.is_gps_valid is True
