import logging
from pathlib import Path
from typing import List, Optional, Protocol

import cv2
import numpy as np
import redis

from app.config import settings
from app.messaging import xadd_json
from app.pipeline.clock import PlaybackClock
from app.schemas import Detection

logger = logging.getLogger(__name__)


class Detector(Protocol):
    def predict(self, frame: np.ndarray, frame_id: str, timestamp: str) -> List[Detection]: ...


class VideoIngestionPipeline:
    """Reads frames from a local video file at a target FPS, runs every
    configured detector on each sampled frame, and publishes each resulting
    Detection to the `detections` Redis stream.
    """

    def __init__(
        self,
        redis_client: redis.Redis,
        detectors: List[Detector],
        video_path: Optional[str] = None,
        target_fps: Optional[float] = None,
        stream_name: Optional[str] = None,
        clock: Optional[PlaybackClock] = None,
    ):
        self.redis_client = redis_client
        self.detectors = detectors
        self.video_path = video_path or settings.video_path
        self.target_fps = target_fps or settings.target_fps
        self.stream_name = stream_name or settings.detections_channel
        self.clock = clock or PlaybackClock()

    def run(self, max_frames: Optional[int] = None, loop: bool = False) -> int:
        if not Path(self.video_path).exists():
            raise FileNotFoundError(
                f"Video file not found at '{self.video_path}'. Place a demo clip there "
                "(see inference-service/README.md) before running the ingestion pipeline."
            )

        cap = cv2.VideoCapture(self.video_path)
        if not cap.isOpened():
            raise RuntimeError(f"OpenCV failed to open video file '{self.video_path}'")

        source_fps = cap.get(cv2.CAP_PROP_FPS) or self.target_fps
        frame_interval = max(1, round(source_fps / self.target_fps))

        frame_idx = 0
        processed = 0
        try:
            while True:
                ok, frame = cap.read()
                if not ok:
                    if loop:
                        cap.set(cv2.CAP_PROP_POS_FRAMES, 0)
                        frame_idx = 0
                        continue
                    break

                if frame_idx % frame_interval == 0:
                    elapsed = frame_idx / source_fps
                    self.clock.sleep_until(elapsed)
                    timestamp = self.clock.timestamp_at(elapsed)
                    frame_id = f"frame_{frame_idx:08d}"

                    self._process_frame(frame, frame_id, timestamp)
                    processed += 1
                    if max_frames is not None and processed >= max_frames:
                        break

                frame_idx += 1
        finally:
            cap.release()

        logger.info("video ingestion finished: %d frames processed", processed)
        return processed

    def _process_frame(self, frame: np.ndarray, frame_id: str, timestamp: str) -> None:
        for detector in self.detectors:
            try:
                detections = detector.predict(frame, frame_id, timestamp)
            except Exception:
                logger.exception(
                    "detector %s failed on %s", type(detector).__name__, frame_id
                )
                continue

            for detection in detections:
                xadd_json(self.redis_client, self.stream_name, detection.model_dump(by_alias=True))
                logger.info(
                    "published detection %s class=%s",
                    detection.id,
                    detection.class_,
                    extra={"frame_id": frame_id},
                )
