"""Deterministic Scenario Runner for SAR Drone simulation.

Replays scenario video frames and synchronised telemetry synthesised from planned flight paths,
publishing directly to production Redis streams ('telemetry' and 'detections').
"""

import argparse
import json
import logging
import os
import sys
import time
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import List, Optional

import cv2
import redis

from simulator.chaos import ChaosScheduler, ConnectivityChaosHook
from simulator.path_planner import TelemetrySample, TrajectorySynthesizer
from simulator.scenario import Scenario
from simulator.telemetry import compute_telemetry_digest, format_iso_timestamp
from simulator.thermal import ThermalVideoSource

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s [%(levelname)s] [%(name)s] %(message)s',
    datefmt='%H:%M:%S',
)
logger = logging.getLogger("simulator.runner")


class PlaybackClock:
    """Paces playback relative to an origin at given speed multiplier."""

    def __init__(self, start_time: Optional[datetime] = None, speed: float = 1.0):
        self.start_time = start_time or datetime.now(timezone.utc)
        self.speed = speed
        self._t0 = time.monotonic()

    def timestamp_at(self, elapsed_s: float) -> str:
        moment = self.start_time + timedelta(seconds=elapsed_s)
        return format_iso_timestamp(moment)

    def sleep_until(self, elapsed_s: float) -> None:
        if self.speed <= 0:
            return
        target = self._t0 + (elapsed_s / self.speed)
        delay = target - time.monotonic()
        if delay > 0:
            time.sleep(delay)


def publish_redis_stream(client: redis.Redis, stream: str, payload: dict, maxlen: int = 1000) -> str:
    """Publishes a JSON payload under the 'data' field matching production convention."""
    return client.xadd(
        stream,
        {"data": json.dumps(payload)},
        maxlen=maxlen,
        approximate=True,
    )


class ScenarioRunner:
    """Executes a simulation scenario against Redis streams."""

    def __init__(
        self,
        scenario: Scenario,
        redis_client: Optional[redis.Redis] = None,
        speed: float = 1.0,
        detections_stream: str = "detections",
        telemetry_stream: str = "telemetry",
        start_time: Optional[datetime] = None,
    ):
        self.scenario = scenario
        self.redis_client = redis_client
        self.speed = speed
        self.detections_stream = detections_stream
        self.telemetry_stream = telemetry_stream
        self.clock = PlaybackClock(start_time=start_time, speed=speed)

        # Synthesizer
        self.synthesizer = TrajectorySynthesizer(
            polygon=scenario.area_polygon,
            home=scenario.home_point,
            flight=scenario.flight,
            hfov_deg=scenario.camera.hfov_deg,
        )

        # Chaos scheduler
        self.chaos = ChaosScheduler(events=scenario.connectivity_events)

        # Thermal source
        self.thermal_source = ThermalVideoSource(
            thermal_video_path=scenario.video.thermal_path,
            use_pipeline=scenario.video.thermal_pipeline.enabled,
            pipeline_method=scenario.video.thermal_pipeline.method,
        )

    def run(self, max_frames: Optional[int] = None) -> dict:
        fps = self.scenario.camera.fps
        video_path = Path(self.scenario.video.rgb_path)

        cap = None
        total_video_frames = 60
        if video_path.is_file():
            cap = cv2.VideoCapture(str(video_path))
            if cap.isOpened():
                total_video_frames = int(cap.get(cv2.CAP_PROP_FRAME_COUNT)) or 60

        run_frames = min(total_video_frames, max_frames) if max_frames else total_video_frames
        duration_s = run_frames / fps

        # Synthesize trajectory
        gps_outages = [(w.t_start_s, w.t_end_s) for w in self.scenario.gps_outages]
        telemetry_samples = self.synthesizer.generate_trajectory(
            duration_s=duration_s,
            gps_outages=gps_outages,
        )

        logger.info(
            "Starting scenario '%s': %d frames at %.1f fps (speed=%.1fx, seed=%d)",
            self.scenario.name,
            run_frames,
            fps,
            self.speed,
            self.scenario.flight.seed,
        )

        telemetry_digest = compute_telemetry_digest(telemetry_samples)
        logger.info("Telemetry SHA256 digest: %s", telemetry_digest)

        published_telemetry = 0
        published_detections = 0
        suppressed_gps_outages = 0
        telem_idx = 0
        num_telem = len(telemetry_samples)

        # Stable track IDs per ground-truth entity
        track_id_map = {obj.id: f"track_{i+1:03d}" for i, obj in enumerate(self.scenario.ground_truth_objects)}

        try:
            for frame_idx in range(run_frames):
                elapsed_s = frame_idx / fps
                self.clock.sleep_until(elapsed_s)
                timestamp_str = self.clock.timestamp_at(elapsed_s)
                frame_id = f"frame_{frame_idx:08d}"

                # 1. Update Chaos scheduler
                self.chaos.update(elapsed_s)

                # 2. Replay video frame
                rgb_frame = None
                if cap is not None and cap.isOpened():
                    ok, frame = cap.read()
                    if ok:
                        rgb_frame = frame

                # 3. Publish Telemetry up to current elapsed time
                target_telem_time = elapsed_s + (1.0 / fps)
                while telem_idx < num_telem and telemetry_samples[telem_idx].elapsed_s <= target_telem_time:
                    sample = telemetry_samples[telem_idx]
                    sample_ts = self.clock.timestamp_at(sample.elapsed_s)

                    if sample.is_gps_valid:
                        if self.redis_client is not None:
                            payload = sample.to_stream_dict(sample_ts)
                            publish_redis_stream(self.redis_client, self.telemetry_stream, payload)
                        published_telemetry += 1
                    else:
                        suppressed_gps_outages += 1

                    telem_idx += 1

                # 4. Check Ground Truth Objects appearing in this frame
                for obj in self.scenario.ground_truth_objects:
                    start_f, end_f = obj.appearance_frames
                    if start_f <= frame_idx <= end_f:
                        # Produce deterministic detection with persistent track identity
                        det_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, f"{self.scenario.name}:{obj.id}:{frame_idx}"))
                        persistent_track_id = track_id_map.get(obj.id, f"track_{obj.id}")
                        det_payload = {
                            "id": det_id,
                            "class": obj.type,
                            "confidence": obj.confidence,
                            "bbox": obj.bbox.model_dump(),
                            "frame_id": frame_id,
                            "source": "rgb",
                            "timestamp": timestamp_str,
                            "track_id": persistent_track_id,
                            "modalities": ["rgb", "thermal"] if (self.scenario.video.thermal_pipeline.enabled or self.scenario.video.thermal_path) else ["rgb"],
                            "model_version": "v2.0-sim-tracker",
                        }
                        if self.redis_client is not None:
                            publish_redis_stream(self.redis_client, self.detections_stream, det_payload)
                        published_detections += 1
                        logger.debug("Published detection %s class=%s frame=%s track=%s", det_id, obj.type, frame_id, persistent_track_id)

                if (frame_idx + 1) % 15 == 0 or frame_idx == run_frames - 1:
                    logger.info(
                        "Progress: frame %d/%d (telem=%d, dets=%d)",
                        frame_idx + 1,
                        run_frames,
                        published_telemetry,
                        published_detections,
                    )

        finally:
            if cap is not None:
                cap.release()
            self.thermal_source.release()
            # Clean up chaos hooks
            self.chaos.reset()

        logger.info(
            "Scenario '%s' completed: frames=%d, telemetry=%d (gps_outages=%d), detections=%d",
            self.scenario.name,
            run_frames,
            published_telemetry,
            suppressed_gps_outages,
            published_detections,
        )

        return {
            "scenario": self.scenario.name,
            "frames_replayed": run_frames,
            "telemetry_published": published_telemetry,
            "telemetry_suppressed_gps_outage": suppressed_gps_outages,
            "detections_published": published_detections,
            "telemetry_digest": telemetry_digest,
        }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--scenario", default="scenarios/flood.yaml", help="Path or name of scenario YAML")
    parser.add_argument("--speed", type=float, default=2.0, help="Playback speed multiplier (e.g. 1.0 or 2.0)")
    parser.add_argument("--max-frames", type=int, default=None, help="Max frames to replay")
    parser.add_argument("--redis-url", default=os.environ.get("REDIS_URL", "redis://localhost:6379/0"))
    parser.add_argument("--detections-stream", default="detections")
    parser.add_argument("--telemetry-stream", default="telemetry")
    parser.add_argument("--verify-fusion", action="store_true", help="Poll fusion-service after completion")
    args = parser.parse_args()

    scenario_path = Path(args.scenario)
    if not scenario_path.is_file():
        # Check if shorthand was passed, e.g. "flood" -> scenarios/flood.yaml
        alt_path = Path(f"scenarios/{args.scenario}.yaml")
        if alt_path.is_file():
            scenario_path = alt_path
        else:
            logger.error("Scenario file not found: %s", args.scenario)
            sys.exit(1)

    scenario = Scenario.load(scenario_path)

    # Connect to Redis
    redis_client = None
    try:
        redis_client = redis.from_url(args.redis_url, decode_responses=True)
        redis_client.ping()
        logger.info("Connected to Redis at %s", args.redis_url)
    except Exception as e:
        logger.warning("Could not connect to Redis (%s). Running in dry-run mode.", e)

    runner = ScenarioRunner(
        scenario=scenario,
        redis_client=redis_client,
        speed=args.speed,
        detections_stream=args.detections_stream,
        telemetry_stream=args.telemetry_stream,
    )

    result = runner.run(max_frames=args.max_frames)

    # Verification if requested
    if args.verify_fusion:
        time.sleep(2.0)
        from simulator.annotation import fetch_detections_from_fusion
        dets = fetch_detections_from_fusion()
        logger.info("Verification: fusion-service holds %d persisted detections.", len(dets))

    print(json.dumps(result, indent=2))


if __name__ == "__main__":
    main()
