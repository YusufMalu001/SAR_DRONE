import pytest
from app.planner.coverage import CoveragePathPlanner, Waypoint


def test_coverage_planner_convex_polygon():
    # Flood scenario: 600m x 600m square polygon at (20.0, 78.0)
    polygon = [
        (20.0000, 78.0000),
        (20.0060, 78.0000),
        (20.0060, 78.0060),
        (20.0000, 78.0060),
    ]

    planner = CoveragePathPlanner(
        altitude_m=50.0,
        hfov_deg=90.0,
        vfov_deg=60.0,
        lateral_overlap=0.30,
        forward_overlap=0.40,
        flight_speed_mps=5.0,
    )

    waypoints = planner.plan_coverage(polygon)
    assert len(waypoints) > 5, f"Expected multiple waypoints, got {len(waypoints)}"

    # Verify all waypoints have correct altitude and speeds
    for wp in waypoints:
        assert wp.alt == 50.0
        assert wp.speed_mps > 0

    # Turn waypoints present
    turns = [wp for wp in waypoints if wp.is_turn]
    assert len(turns) > 0, "Expected turn waypoints to connect parallel tracks"

    # Verify coverage fraction exceeds 95%
    coverage_frac = planner.compute_coverage_fraction(polygon, waypoints)
    assert coverage_frac >= 0.95, f"Coverage fraction {coverage_frac:.4f} is below 95% requirement"


def test_coverage_planner_concave_polygon():
    # L-shaped concave polygon
    polygon = [
        (20.0000, 78.0000),
        (20.0060, 78.0000),
        (20.0060, 78.0030),
        (20.0030, 78.0030),
        (20.0030, 78.0060),
        (20.0000, 78.0060),
    ]

    planner = CoveragePathPlanner(
        altitude_m=50.0,
        hfov_deg=90.0,
        vfov_deg=60.0,
        lateral_overlap=0.30,
        forward_overlap=0.40,
        flight_speed_mps=5.0,
    )

    waypoints = planner.plan_coverage(polygon)
    assert len(waypoints) > 5

    coverage_frac = planner.compute_coverage_fraction(polygon, waypoints)
    assert coverage_frac >= 0.95, f"Concave coverage fraction {coverage_frac:.4f} is below 95% requirement"
