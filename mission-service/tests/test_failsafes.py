import time
import pytest
from app.mavlink.failsafes import FailsafeAction, FailsafeMonitor


def test_failsafe_battery_low_triggers_rtl():
    monitor = FailsafeMonitor(battery_low_threshold_pct=20.0, battery_critical_threshold_pct=10.0)

    # Normal battery
    status = monitor.evaluate(battery_pct=50.0, lat=20.0, lon=78.0, alt=40.0, gps_fix_type=3)
    assert status.action == FailsafeAction.NONE

    # Battery drops to 18% -> triggers RTL
    status = monitor.evaluate(battery_pct=18.0, lat=20.0, lon=78.0, alt=40.0, gps_fix_type=3)
    assert status.action == FailsafeAction.RTL
    assert "battery low" in status.reason


def test_failsafe_battery_critical_triggers_land():
    monitor = FailsafeMonitor(battery_critical_threshold_pct=10.0)
    status = monitor.evaluate(battery_pct=8.0, lat=20.0, lon=78.0, alt=40.0, gps_fix_type=3)
    assert status.action == FailsafeAction.LAND
    assert "battery critical" in status.reason


def test_failsafe_link_loss_timeout_triggers_rtl():
    monitor = FailsafeMonitor(link_loss_timeout_s=5.0)
    now = 1000.0
    monitor.update_heartbeat(now)

    # 4 seconds elapsed -> still valid
    status = monitor.evaluate(battery_pct=80.0, lat=20.0, lon=78.0, alt=40.0, gps_fix_type=3, now=now + 4.0)
    assert status.action == FailsafeAction.NONE

    # 6 seconds elapsed -> link loss timeout -> triggers RTL
    status = monitor.evaluate(battery_pct=80.0, lat=20.0, lon=78.0, alt=40.0, gps_fix_type=3, now=now + 6.0)
    assert status.action == FailsafeAction.RTL
    assert "link loss timeout" in status.reason


def test_failsafe_geofence_breach_triggers_rtl():
    geofence = [
        (20.000, 78.000),
        (20.010, 78.000),
        (20.010, 78.010),
        (20.000, 78.010),
    ]
    monitor = FailsafeMonitor(geofence_poly=geofence)

    # Inside
    status = monitor.evaluate(battery_pct=80.0, lat=20.005, lon=78.005, alt=40.0, gps_fix_type=3)
    assert status.action == FailsafeAction.NONE

    # Outside (breach)
    status = monitor.evaluate(battery_pct=80.0, lat=20.015, lon=78.005, alt=40.0, gps_fix_type=3)
    assert status.action == FailsafeAction.RTL
    assert "geofence breach" in status.reason


def test_failsafe_gps_loss_triggers_vio_mode():
    monitor = FailsafeMonitor()
    # GPS fix lost (fix_type < 3)
    status = monitor.evaluate(battery_pct=80.0, lat=20.0, lon=78.0, alt=40.0, gps_fix_type=1)
    assert status.action == FailsafeAction.GPS_DENIED_VIO
    assert "GPS fix lost" in status.reason
