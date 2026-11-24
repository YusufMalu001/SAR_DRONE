"""Video ingestion pipeline reading RGB and Thermal streams and running multi-sensor inference."""

import logging
import time
from pathlib import Path
from typing import Optional

import cv2
import numpy as np
import redis

from app.config import settings
from app.pipeline.clock import PlaybackClock
from app.pipeline.metrics import metrics
from app.pipeline.multisensor_pipeline import MultiSensorPipeline
from app.pipeline.queue import BoundedFrameQueue

logger = logging.getLogger("inference.ingest")


class VideoIngestionPipeline:
    """Reads synchronized video frames at a target FPS, buffers them through a bounded queue,
    runs tiled multi-sensor inference, tracking, and fusion, and publishes to Redis.
    """

    def __init__(
        self,
        redis_client: Optional[redis.Redis] = None,
        video_path: Optional[str] = None,
        thermal_path: Optional[str] = None,
        target_fps: Optional[float] = None,
        stream_name: Optional[str] = None,
        clock: Optional[PlaybackClock] = None,
        pipeline: Optional[MultiSensorPipeline] = None,
    ):
        self.redis_client = redis_client
        self.video_path = video_path or settings.video_path
        self.thermal_path = thermal_path
        self.target_fps = target_fps or settings.target_fps
        self.stream_name = stream_name or settings.detections_channel
        self.clock = clock or PlaybackClock()

        self.multisensor_pipeline = pipeline or MultiSensorPipeline(
            redis_client=redis_client,
            stream_name=self.stream_name,
            clock=self.clock,
        )
        self.frame_queue = BoundedFrameQueue(maxsize=settings.queue_max_size)

    def run(self, max_frames: Optional[int] = None, loop: bool = False) -> int:
        if not Path(self.video_path).exists():
            raise FileNotFoundError(
                f"Video file not found at '{self.video_path}'. Place a demo clip there "
                "(see inference-service/README.md) before running the ingestion pipeline."
            )

        cap_rgb = cv2.VideoCapture(self.video_path)
        if not cap_rgb.isOpened():
            raise RuntimeError(f"OpenCV failed to open video file '{self.video_path}'")

        cap_thermal = None
        if self.thermal_path and Path(self.thermal_path).exists():
            cap_thermal = cv2.VideoCapture(self.thermal_path)

        source_fps = cap_rgb.get(cv2.CAP_PROP_FPS) or self.target_fps
        frame_interval = max(1, round(source_fps / self.target_fps))

        frame_idx = 0
        processed = 0

        try:
            while True:
                t_decode_start = time.perf_counter()
                ok_rgb, frame_rgb = cap_rgb.read()
                frame_thermal = None
                if cap_thermal is not None and cap_thermal.isOpened():
                    ok_th, th_frame = cap_thermal.read()
                    if ok_th:
                        frame_thermal = th_frame

                t_decode = (time.perf_counter() - t_decode_start) * 1000.0
                metrics.stage_latency.observe({"stage": "decode"}, t_decode / 1000.0)

                if not ok_rgb:
                    if loop:
                        cap_rgb.set(cv2.CAP_PROP_POS_FRAMES, 0)
                        if cap_thermal:
                            cap_thermal.set(cv2.CAP_PROP_POS_FRAMES, 0)
                        frame_idx = 0
                        continue
                    break

                if frame_idx % frame_interval == 0:
                    elapsed = frame_idx / source_fps
                    self.clock.sleep_until(elapsed)
                    timestamp = self.clock.timestamp_at(elapsed)
                    frame_id = f"frame_{frame_idx:08d}"

                    # Enqueue with bounded drop-oldest backpressure
                    self.frame_queue.put((frame_rgb, frame_thermal, frame_id, timestamp))

                    # Process frame
                    rgb_in, th_in, fid, ts = self.frame_queue.get()
                    self.multisensor_pipeline.process_frame_pair(rgb_in, th_in, fid, ts)

                    processed += 1
                    if max_frames is not None and processed >= max_frames:
                        break

                frame_idx += 1
        finally:
            cap_rgb.release()
            if cap_thermal:
                cap_thermal.release()

        logger.info("Video ingestion finished: %d frames processed", processed)
        return processed
