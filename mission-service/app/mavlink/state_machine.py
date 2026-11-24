"""Mission state machine coordinating MAVLink execution, adaptive search, and failsafes."""

from enum import Enum
import logging
import time
from typing import Dict, List, Optional
from app.mavlink.client import MAVLinkController
from app.mavlink.failsafes import FailsafeAction, FailsafeMonitor, FailsafeStatus
from app.planner.adaptive import AdaptiveSearchManager, SearchState
from app.planner.coverage import Waypoint

logger = logging.getLogger("mission.state_machine")


class VehicleMissionState(str, Enum):
    DISCONNECTED = "DISCONNECTED"
    STANDBY = "STANDBY"
    ARMING = "ARMING"
    TAKEOFF = "TAKEOFF"
    MISSION_RUNNING = "MISSION_RUNNING"
    MISSION_PAUSED = "MISSION_PAUSED"
    ADAPTIVE_ORBIT = "ADAPTIVE_ORBIT"
    FAILSAFE_RTL = "FAILSAFE_RTL"
    LANDED = "LANDED"


class MissionExecutionCoordinator:
    def __init__(
        self,
        mav_controller: MAVLinkController,
        failsafe_monitor: FailsafeMonitor,
        takeoff_alt_m: float = 20.0,
    ):
        self.mav = mav_controller
        self.failsafe_monitor = failsafe_monitor
        self.takeoff_alt_m = takeoff_alt_m

        self.state = VehicleMissionState.DISCONNECTED
        self.adaptive_mgr: Optional[AdaptiveSearchManager] = None
        self.planned_waypoints: List[Waypoint] = []
        self.current_waypoint_idx = 0

        # Register callback
        self.mav.on_telemetry_callbacks.append(self._on_telemetry)

    def set_mission(self, waypoints: List[Waypoint]):
        """Load planned coverage waypoints."""
        self.planned_waypoints = list(waypoints)
        self.adaptive_mgr = AdaptiveSearchManager(self.planned_waypoints)
        self.current_waypoint_idx = 0
        if self.mav.is_connected:
            self.state = VehicleMissionState.STANDBY

    def start_mission(self) -> bool:
        """Execute autonomous launch and mission sequence: Arm -> Takeoff -> Auto."""
        if not self.mav.is_connected:
            logger.error("Cannot start mission: MAVLink not connected")
            return False
        if not self.planned_waypoints:
            logger.error("Cannot start mission: No waypoints planned")
            return False

        logger.info("Starting mission sequence...")
        self.state = VehicleMissionState.ARMING

        # Upload waypoints to autopilot
        if not self.mav.upload_mission(self.planned_waypoints):
            logger.error("Failed to upload mission waypoints")
            return False

        # Arm
        if not self.mav.arm(True):
            logger.error("Arming failed")
            return False

        # Takeoff
        self.state = VehicleMissionState.TAKEOFF
        if not self.mav.takeoff(self.takeoff_alt_m):
            logger.error("Takeoff command failed")
            return False

        # Switch to AUTO
        time.sleep(1.0)
        self.mav.set_mode("AUTO")
        self.state = VehicleMissionState.MISSION_RUNNING
        logger.info("Mission is running in AUTO mode.")
        return True

    def pause_mission(self) -> bool:
        """Hold position in GUIDED mode."""
        if self.state != VehicleMissionState.MISSION_RUNNING:
            return False
        self.mav.set_mode("GUIDED")
        self.state = VehicleMissionState.MISSION_PAUSED
        logger.info("Mission paused (holding position).")
        return True

    def resume_mission(self) -> bool:
        """Resume coverage flight in AUTO mode."""
        if self.state != VehicleMissionState.MISSION_PAUSED:
            return False
        self.mav.set_mode("AUTO")
        self.state = VehicleMissionState.MISSION_RUNNING
        logger.info("Mission resumed.")
        return True

    def abort_mission(self) -> bool:
        """Abort mission and return to launch immediately."""
        self.mav.return_to_launch()
        self.state = VehicleMissionState.FAILSAFE_RTL
        logger.warning("Mission aborted: Commanded RTL.")
        return True

    def on_survivor_detected(self, lat: float, lon: float, conf: float):
        """Forward detection to adaptive search manager."""
        if self.state == VehicleMissionState.MISSION_RUNNING and self.adaptive_mgr:
            if self.adaptive_mgr.on_survivor_detected(lat, lon, conf):
                self.state = VehicleMissionState.ADAPTIVE_ORBIT
                logger.info("Adaptive search: Entered survivor confirmation orbit at (%.5f, %.5f)", lat, lon)

    def _on_telemetry(self, telemetry: Dict):
        """Evaluate vehicle state and failsafes on each telemetry packet."""
        if not self.mav.is_connected:
            self.state = VehicleMissionState.DISCONNECTED
            return

        if self.state == VehicleMissionState.DISCONNECTED:
            self.state = VehicleMissionState.STANDBY

        # Evaluate failsafes
        fs_status = self.failsafe_monitor.evaluate(
            battery_pct=self.mav.battery_pct,
            lat=self.mav.lat,
            lon=self.mav.lon,
            alt=self.mav.alt,
            gps_fix_type=self.mav.gps_fix_type,
        )

        if fs_status.action == FailsafeAction.RTL and self.state not in (VehicleMissionState.FAILSAFE_RTL, VehicleMissionState.LANDED):
            logger.critical("Failsafe triggered: %s -> Executing RTL", fs_status.reason)
            self.mav.return_to_launch()
            self.state = VehicleMissionState.FAILSAFE_RTL

        elif fs_status.action == FailsafeAction.LAND and self.state != VehicleMissionState.LANDED:
            logger.critical("Critical failsafe: %s -> Landing immediately", fs_status.reason)
            self.mav.set_mode("LAND")
            self.state = VehicleMissionState.LANDED

    def get_status(self) -> Dict:
        return {
            "state": self.state.value,
            "connected": self.mav.is_connected,
            "armed": self.mav.is_armed,
            "mode": self.mav.current_mode,
            "battery_pct": self.mav.battery_pct,
            "gps_fix_type": self.mav.gps_fix_type,
            "position": {
                "lat": self.mav.lat,
                "lon": self.mav.lon,
                "alt": self.mav.alt,
                "heading": self.mav.heading,
            },
            "total_waypoints": len(self.planned_waypoints),
            "current_waypoint_idx": self.current_waypoint_idx,
            "adaptive_state": self.adaptive_mgr.state.value if self.adaptive_mgr else None,
        }
