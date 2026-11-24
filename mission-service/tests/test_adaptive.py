import pytest
from app.planner.adaptive import AdaptiveSearchManager, SearchState
from app.planner.coverage import Waypoint


def test_adaptive_search_survivor_orbit_and_resume():
    main_wps = [
        Waypoint(lat=20.000, lon=78.000, alt=50.0),
        Waypoint(lat=20.002, lon=78.002, alt=50.0),
        Waypoint(lat=20.004, lon=78.004, alt=50.0),
    ]

    mgr = AdaptiveSearchManager(main_wps, orbit_radius_m=15.0)
    assert mgr.state == SearchState.NORMAL_COVERAGE
    assert mgr.get_current_waypoint().lat == 20.000

    # Advance to next waypoint
    mgr.advance_waypoint()
    assert mgr.current_main_idx == 1
    assert mgr.get_current_waypoint().lat == 20.002

    # Survivor detected nearby with confidence 0.88 -> enters SURVIVOR_CONFIRMATION
    triggered = mgr.on_survivor_detected(lat=20.0025, lon=78.0025, confidence=0.88)
    assert triggered is True
    assert mgr.state == SearchState.SURVIVOR_CONFIRMATION
    assert len(mgr.active_sub_waypoints) == 9  # 8 points + 1 to close

    # Advance through orbit waypoints
    for _ in range(9):
        assert mgr.state == SearchState.SURVIVOR_CONFIRMATION
        mgr.advance_waypoint()

    # Once orbit completes, transitions to RESUMING and returns to main survey path
    assert mgr.state == SearchState.RESUMING
    assert mgr.get_current_waypoint().lat == 20.002

    # Advance resumes NORMAL_COVERAGE
    mgr.advance_waypoint()
    assert mgr.state == SearchState.NORMAL_COVERAGE
    assert mgr.current_main_idx == 2


def test_adaptive_search_low_confidence_revisit():
    main_wps = [Waypoint(lat=20.000, lon=78.000, alt=50.0)]
    mgr = AdaptiveSearchManager(main_wps)

    # Coverage map reports low confidence cell
    low_cells = [
        {
            "cell_id": "cell_1_2",
            "min_lat": 20.001,
            "max_lat": 20.002,
            "min_lon": 78.001,
            "max_lon": 78.002,
            "best_confidence": 0.20,
        }
    ]

    triggered = mgr.on_coverage_update(low_cells)
    assert triggered is True
    assert mgr.state == SearchState.LOW_CONFIDENCE_REVISIT
    assert len(mgr.active_sub_waypoints) == 3

    for _ in range(3):
        mgr.advance_waypoint()

    assert mgr.state == SearchState.RESUMING
