import logging
from contextlib import asynccontextmanager
from typing import Dict, List, Optional
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from app.config import MissionConfig
from app.mavlink.client import MAVLinkController
from app.mavlink.failsafes import FailsafeMonitor
from app.mavlink.state_machine import MissionExecutionCoordinator
from app.planner.coverage import CoveragePathPlanner, Waypoint

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(name)s: %(message)s")
logger = logging.getLogger("mission.main")

mav_controller = MAVLinkController(
    connection_str=MissionConfig.MAVLINK_CONNECTION,
    redis_url=MissionConfig.REDIS_URL,
    telemetry_channel=MissionConfig.TELEMETRY_CHANNEL,
)

failsafe_monitor = FailsafeMonitor(
    battery_low_threshold_pct=MissionConfig.BATTERY_LOW_THRESHOLD,
    battery_critical_threshold_pct=MissionConfig.BATTERY_CRITICAL_THRESHOLD,
    link_loss_timeout_s=MissionConfig.LINK_LOSS_TIMEOUT_S,
)

coordinator = MissionExecutionCoordinator(
    mav_controller=mav_controller,
    failsafe_monitor=failsafe_monitor,
    takeoff_alt_m=MissionConfig.DEFAULT_ALTITUDE_M,
)


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("Initializing mission-service...")
    # Attempt MAVLink connection asynchronously
    mav_controller.connect(timeout_s=2.0)
    yield
    logger.info("Shutting down mission-service...")
    mav_controller.stop()


app = FastAPI(title="SAR Mission Control Service", version="1.0.0", lifespan=lifespan)


class Point2D(BaseModel):
    lat: float = Field(ge=-90.0, le=90.0)
    lon: float = Field(ge=-180.0, le=180.0)


class PlanMissionRequest(BaseModel):
    polygon: List[Point2D]
    altitude_m: float = Field(default=50.0, ge=10.0, le=150.0)
    speed_mps: float = Field(default=5.0, ge=1.0, le=15.0)
    overlap_lateral: float = Field(default=0.30, ge=0.1, le=0.8)
    overlap_forward: float = Field(default=0.40, ge=0.1, le=0.8)


@app.get("/health")
def health():
    return {
        "status": "ok",
        "service": "mission-service",
        "mavlink_connected": mav_controller.is_connected,
    }


@app.get("/mission/status")
def get_mission_status():
    return coordinator.get_status()


@app.post("/mission/plan")
def plan_mission(req: PlanMissionRequest):
    if len(req.polygon) < 3:
        raise HTTPException(status_code=400, detail="Polygon must have at least 3 vertices")

    pts = [(p.lat, p.lon) for p in req.polygon]
    planner = CoveragePathPlanner(
        altitude_m=req.altitude_m,
        hfov_deg=MissionConfig.CAMERA_HFOV_DEG,
        vfov_deg=MissionConfig.CAMERA_VFOV_DEG,
        lateral_overlap=req.overlap_lateral,
        forward_overlap=req.overlap_forward,
        flight_speed_mps=req.speed_mps,
    )

    try:
        waypoints = planner.plan_coverage(pts)
        coverage_frac = planner.compute_coverage_fraction(pts, waypoints)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Planning failed: {str(e)}")

    coordinator.set_mission(waypoints)

    return {
        "status": "planned",
        "total_waypoints": len(waypoints),
        "coverage_fraction": round(coverage_frac, 4),
        "waypoints": [wp.to_dict() for wp in waypoints],
    }


@app.post("/mission/start")
def start_mission():
    success = coordinator.start_mission()
    if not success:
        raise HTTPException(status_code=400, detail="Failed to start mission; check connection and waypoints")
    return {"status": "started", "mission_state": coordinator.state.value}


@app.post("/mission/pause")
def pause_mission():
    success = coordinator.pause_mission()
    if not success:
        raise HTTPException(status_code=400, detail="Cannot pause mission in current state")
    return {"status": "paused", "mission_state": coordinator.state.value}


@app.post("/mission/resume")
def resume_mission():
    success = coordinator.resume_mission()
    if not success:
        raise HTTPException(status_code=400, detail="Cannot resume mission in current state")
    return {"status": "resumed", "mission_state": coordinator.state.value}


@app.post("/mission/abort")
def abort_mission():
    coordinator.abort_mission()
    return {"status": "aborted", "mission_state": coordinator.state.value}
