"""MAVLink Controller and Telemetry Streamer for ArduPilot SITL / Copter.

Uses pymavlink to:
- Establish and monitor MAVLink connection and heartbeats.
- Arm, takeoff, set flight modes (GUIDED, AUTO, RTL, LAND).
- Upload coverage waypoints via MAVLink mission protocol (MISSION_ITEM_INT).
- Stream high-frequency vehicle telemetry (lat, lon, alt, heading, attitude) to Redis.
"""

import json
import logging
import math
import threading
import time
from datetime import datetime, timezone
from typing import Callable, Dict, List, Optional, Tuple

import redis
from pymavlink import mavutil
from app.planner.coverage import Waypoint

logger = logging.getLogger("mission.mavlink")


class MAVLinkController:
    def __init__(
        self,
        connection_str: str = "udpin:0.0.0.0:14550",
        redis_url: str = "redis://localhost:6379/0",
        telemetry_channel: str = "telemetry",
        telemetry_rate_hz: float = 5.0,
    ):
        self.connection_str = connection_str
        self.redis_url = redis_url
        self.telemetry_channel = telemetry_channel
        self.telemetry_rate_hz = telemetry_rate_hz

        self.mav: Optional[mavutil.mavlink_connection] = None
        self.redis_client: Optional[redis.Redis] = None

        self.is_connected = False
        self.is_armed = False
        self.current_mode = "UNKNOWN"
        self.battery_pct = 100.0
        self.gps_fix_type = 3

        # Vehicle kinematics
        self.lat = 0.0
        self.lon = 0.0
        self.alt = 0.0
        self.heading = 0.0
        self.pitch = 0.0
        self.roll = 0.0
        self.yaw = 0.0

        self._running = False
        self._thread: Optional[threading.Thread] = None
        self.on_telemetry_callbacks: List[Callable[[Dict], None]] = []

    def connect(self, timeout_s: float = 10.0) -> bool:
        """Establish MAVLink connection and wait for first heartbeat."""
        try:
            logger.info("Connecting to MAVLink endpoint: %s", self.connection_str)
            self.mav = mavutil.mavlink_connection(self.connection_str)
            self.redis_client = redis.Redis.from_url(self.redis_url)

            # Wait for heartbeat
            msg = self.mav.wait_heartbeat(timeout=timeout_s)
            if msg:
                self.is_connected = True
                self.current_mode = self.mav.flightmode
                logger.info("Connected to system %d component %d in mode %s", self.mav.target_system, self.mav.target_component, self.current_mode)
                self.start_streaming()
                return True
            else:
                logger.warning("Heartbeat timeout connecting to MAVLink")
                return False
        except Exception as e:
            logger.error("Failed to connect to MAVLink: %v", e)
            return False

    def start_streaming(self):
        """Start background message listener and telemetry publisher thread."""
        if self._running:
            return
        self._running = True
        self._thread = threading.Thread(target=self._telemetry_loop, daemon=True)
        self._thread.start()

    def stop(self):
        self._running = False
        if self._thread and self._thread.is_alive():
            self._thread.join(timeout=2.0)
        if self.mav:
            self.mav.close()

    def set_mode(self, mode_name: str) -> bool:
        """Switch ArduPilot flight mode (e.g. 'GUIDED', 'AUTO', 'RTL', 'LAND')."""
        if not self.mav:
            return False
        mode_id = self.mav.mode_mapping().get(mode_name)
        if mode_id is None:
            logger.error("Unknown flight mode: %s", mode_name)
            return False

        self.mav.mav.set_mode_send(
            self.mav.target_system,
            mavutil.mavlink.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED,
            mode_id,
        )
        self.current_mode = mode_name
        logger.info("Set mode commanded: %s (id=%d)", mode_name, mode_id)
        return True

    def arm(self, arm_state: bool = True) -> bool:
        """Arm or disarm vehicle motors."""
        if not self.mav:
            return False
        self.mav.mav.command_long_send(
            self.mav.target_system,
            self.mav.target_component,
            mavutil.mavlink.MAV_CMD_COMPONENT_ARM_DISARM,
            0,
            1 if arm_state else 0,
            0, 0, 0, 0, 0, 0,
        )
        self.is_armed = arm_state
        logger.info("Arm vehicle: %s", arm_state)
        return True

    def takeoff(self, altitude_m: float = 20.0) -> bool:
        """Command vertical takeoff to altitude_m in GUIDED mode."""
        if not self.mav:
            return False
        self.set_mode("GUIDED")
        time.sleep(0.5)
        self.mav.mav.command_long_send(
            self.mav.target_system,
            self.mav.target_component,
            mavutil.mavlink.MAV_CMD_NAV_TAKEOFF,
            0,
            0, 0, 0, 0, 0, 0,
            altitude_m,
        )
        logger.info("Takeoff commanded to %.1f meters", altitude_m)
        return True

    def return_to_launch(self) -> bool:
        """Command Return-to-Launch (RTL)."""
        logger.warning("Failsafe/Command: Return to Launch (RTL) triggered")
        return self.set_mode("RTL")

    def upload_mission(self, waypoints: List[Waypoint]) -> bool:
        """Upload waypoints using MAVLink MISSION_ITEM_INT protocol."""
        if not self.mav or not waypoints:
            return False

        logger.info("Uploading %d waypoints to autopilot...", len(waypoints))
        # Clear existing mission
        self.mav.mav.mission_clear_all_send(self.mav.target_system, self.mav.target_component)
        time.sleep(0.2)

        # Announce mission count (+1 for home)
        total_items = len(waypoints)
        self.mav.mav.mission_count_send(self.mav.target_system, self.mav.target_component, total_items)

        for seq, wp in enumerate(waypoints):
            # Send MISSION_ITEM_INT
            lat_int = int(wp.lat * 1e7)
            lon_int = int(wp.lon * 1e7)
            self.mav.mav.mission_item_int_send(
                self.mav.target_system,
                self.mav.target_component,
                seq,
                mavutil.mavlink.MAV_FRAME_GLOBAL_RELATIVE_ALT_INT,
                mavutil.mavlink.MAV_CMD_NAV_WAYPOINT,
                0,  # current
                1,  # autocontinue
                0.0, 0.0, 0.0, 0.0,  # params
                lat_int,
                lon_int,
                wp.alt,
            )
            time.sleep(0.02)

        logger.info("Mission upload complete.")
        return True

    def _telemetry_loop(self):
        """Read incoming MAVLink packets and publish Telemetry events to Redis."""
        last_pub_time = 0.0
        pub_interval = 1.0 / self.telemetry_rate_hz

        while self._running and self.mav:
            try:
                msg = self.mav.recv_match(blocking=True, timeout=0.5)
                if not msg:
                    continue

                msg_type = msg.get_type()

                if msg_type == "HEARTBEAT":
                    self.is_connected = True
                    self.current_mode = self.mav.flightmode
                    self.is_armed = bool(msg.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED)

                elif msg_type == "GLOBAL_POSITION_INT":
                    self.lat = msg.lat / 1e7
                    self.lon = msg.lon / 1e7
                    self.alt = msg.relative_alt / 1000.0  # mm to meters
                    self.heading = msg.hdg / 100.0 if msg.hdg != 65535 else 0.0

                elif msg_type == "ATTITUDE":
                    self.roll = math.degrees(msg.roll)
                    self.pitch = math.degrees(msg.pitch)
                    self.yaw = math.degrees(msg.yaw)

                elif msg_type == "SYS_STATUS":
                    if msg.battery_remaining != -1:
                        self.battery_pct = float(msg.battery_remaining)

                elif msg_type == "GPS_RAW_INT":
                    self.gps_fix_type = msg.fix_type

                # Publish to Redis at telemetry_rate_hz
                now = time.time()
                if (now - last_pub_time) >= pub_interval and self.lat != 0.0 and self.lon != 0.0:
                    last_pub_time = now
                    telemetry_payload = {
                        "lat": round(self.lat, 6),
                        "lon": round(self.lon, 6),
                        "alt": round(self.alt, 2),
                        "heading": round(self.heading, 1),
                        "pitch": round(self.pitch, 2),
                        "roll": round(self.roll, 2),
                        "yaw": round(self.yaw, 2),
                        "timestamp": datetime.now(timezone.utc).isoformat(),
                    }

                    if self.redis_client:
                        try:
                            self.redis_client.xadd(self.telemetry_channel, {"data": json.dumps(telemetry_payload)})
                        except Exception as e:
                            logger.error("Failed to publish telemetry to Redis: %v", e)

                    for cb in self.on_telemetry_callbacks:
                        try:
                            cb(telemetry_payload)
                        except Exception:
                            pass

            except Exception as e:
                if self._running:
                    logger.error("Error in MAVLink receive loop: %v", e)
                time.sleep(0.1)
