"""Failsafe manager for autonomous drone operations.

Monitors vehicle state against safety parameters and triggers autonomous failsafes:
- Battery Low (<20%) -> RTL (Return-to-Launch)
- Link Loss (>5s) -> RTL
- Geofence Breach -> RTL
- GPS Loss -> Switch to Visual-Inertial Dead-Reckoning (VIO)
"""

from enum import Enum
import math
import time
from typing import Dict, List, Optional, Tuple


class FailsafeAction(str, Enum):
    NONE = "NONE"
    RTL = "RTL"
    HOLD = "HOLD"
    GPS_DENIED_VIO = "GPS_DENIED_VIO"
    LAND = "LAND"


class FailsafeStatus:
    def __init__(self, action: FailsafeAction = FailsafeAction.NONE, reason: Optional[str] = None):
        self.action = action
        self.reason = reason


class FailsafeMonitor:
    def __init__(
        self,
        battery_low_threshold_pct: float = 20.0,
        battery_critical_threshold_pct: float = 10.0,
        link_loss_timeout_s: float = 5.0,
        geofence_poly: Optional[List[Tuple[float, float]]] = None,
        max_altitude_m: float = 120.0,
    ):
        self.battery_low_threshold = battery_low_threshold_pct
        self.battery_critical_threshold = battery_critical_threshold_pct
        self.link_loss_timeout_s = link_loss_timeout_s
        self.geofence_poly = geofence_poly or []
        self.max_altitude_m = max_altitude_m

        self.last_heartbeat_time = time.time()
        self.active_failsafe: FailsafeAction = FailsafeAction.NONE
        self.active_reason: Optional[str] = None

    def update_heartbeat(self, now: Optional[float] = None):
        self.last_heartbeat_time = now if now is not None else time.time()

    def evaluate(
        self,
        battery_pct: float,
        lat: float,
        lon: float,
        alt: float,
        gps_fix_type: int,  # 0/1=no fix, 2=2D, 3=3D fix
        now: Optional[float] = None,
    ) -> FailsafeStatus:
        """Evaluate vehicle state and return triggered failsafe action if any."""
        t = now if now is not None else time.time()

        # 1. Critical/Low Battery Failsafe
        if battery_pct <= self.battery_critical_threshold:
            self.active_failsafe = FailsafeAction.LAND
            self.active_reason = f"battery critical: {battery_pct:.1f}% <= {self.battery_critical_threshold}%"
            return FailsafeStatus(self.active_failsafe, self.active_reason)

        if battery_pct <= self.battery_low_threshold:
            self.active_failsafe = FailsafeAction.RTL
            self.active_reason = f"battery low: {battery_pct:.1f}% <= {self.battery_low_threshold}%"
            return FailsafeStatus(self.active_failsafe, self.active_reason)

        # 2. Link Loss Failsafe
        if (t - self.last_heartbeat_time) > self.link_loss_timeout_s:
            self.active_failsafe = FailsafeAction.RTL
            self.active_reason = f"link loss timeout: {t - self.last_heartbeat_time:.1f}s > {self.link_loss_timeout_s}s"
            return FailsafeStatus(self.active_failsafe, self.active_reason)

        # 3. Geofence / Altitude Breach
        if alt > self.max_altitude_m:
            self.active_failsafe = FailsafeAction.RTL
            self.active_reason = f"altitude geofence breach: {alt:.1f}m > {self.max_altitude_m}m"
            return FailsafeStatus(self.active_failsafe, self.active_reason)

        if self.geofence_poly and not self._is_inside_geofence(lat, lon):
            self.active_failsafe = FailsafeAction.RTL
            self.active_reason = f"spatial geofence breach at ({lat:.5f}, {lon:.5f})"
            return FailsafeStatus(self.active_failsafe, self.active_reason)

        # 4. GPS Loss Failsafe
        if gps_fix_type < 3:
            self.active_failsafe = FailsafeAction.GPS_DENIED_VIO
            self.active_reason = f"GPS fix lost (fix_type={gps_fix_type})"
            return FailsafeStatus(self.active_failsafe, self.active_reason)

        # Normal condition
        self.active_failsafe = FailsafeAction.NONE
        self.active_reason = None
        return FailsafeStatus(FailsafeAction.NONE, None)

    def _is_inside_geofence(self, lat: float, lon: float) -> bool:
        if len(self.geofence_poly) < 3:
            return True
        inside = False
        j = len(self.geofence_poly) - 1
        for i in range(len(self.geofence_poly)):
            xi, yi = self.geofence_poly[i][1], self.geofence_poly[i][0]
            xj, yj = self.geofence_poly[j][1], self.geofence_poly[j][0]
            intersect = ((yi > lat) != (yj > lat)) and (lon < (xj - xi) * (lat - yi) / (yj - yi) + xi)
            if intersect:
                inside = not inside
            j = i
        return inside
