"""Production multi-sensor inference pipeline: Tiled Detection -> Hazard Model -> ByteTrack -> Cross-Modal Fusion."""

import logging
import time
import uuid
from typing import List, Optional, Tuple
import numpy as np
import redis

from app.config import settings
from app.messaging import xadd_json
from app.models.hazard_classifier import HazardClassifier
from app.models.person_detector import PersonDetector
from app.models.thermal_detector import ThermalDetector
from app.pipeline.clock import PlaybackClock
from app.pipeline.fusion import SensorFusionEngine
from app.pipeline.metrics import metrics
from app.pipeline.tiling import TiledInferenceEngine
from app.pipeline.tracker import ByteTracker
from app.schemas import BBox, Detection, DetectionClass, Source, StageTimings

logger = logging.getLogger("inference.pipeline")


class MultiSensorPipeline:
    """Processes synchronized RGB + Thermal frame pairs with tiling, tracking, and fusion."""

    def __init__(
        self,
        redis_client: Optional[redis.Redis] = None,
        stream_name: Optional[str] = None,
        clock: Optional[PlaybackClock] = None,
        person_detector: Optional[PersonDetector] = None,
        thermal_detector: Optional[ThermalDetector] = None,
        hazard_classifier: Optional[HazardClassifier] = None,
    ):
        self.redis_client = redis_client
        self.stream_name = stream_name or settings.detections_channel
        self.clock = clock or PlaybackClock()

        # Models
        self.person_detector = person_detector or PersonDetector()
        self.thermal_detector = thermal_detector or ThermalDetector()
        self.hazard_classifier = hazard_classifier or HazardClassifier()

        # Subsystems
        self.tiling_engine = TiledInferenceEngine(
            tile_size=settings.tile_size,
            overlap_ratio=settings.tile_overlap,
            iou_threshold=settings.iou_threshold,
        )
        self.rgb_tracker = ByteTracker(
            high_thresh=settings.track_high_thresh,
            low_thresh=settings.track_low_thresh,
            max_age=settings.track_max_age,
            min_hits=settings.track_min_hits,
        )
        self.thermal_tracker = ByteTracker(
            high_thresh=settings.track_high_thresh,
            low_thresh=settings.track_low_thresh,
            max_age=settings.track_max_age,
            min_hits=settings.track_min_hits,
        )
        self.fusion_engine = SensorFusionEngine(
            homography_path=settings.rgb_thermal_homography_path,
            iou_threshold=settings.fusion_iou_threshold,
        )

    def process_frame_pair(
        self,
        rgb_frame: np.ndarray,
        thermal_frame: Optional[np.ndarray],
        frame_id: str,
        timestamp: str,
    ) -> List[Detection]:
        """Runs tiled inference, hazard classification, tracking, and cross-modal fusion on a frame pair."""
        t_start = time.perf_counter()

        # Stage 1: Preprocess
        t_prep_start = time.perf_counter()
        # Ensure dimensions match expected formats
        h_rgb, w_rgb = rgb_frame.shape[:2]
        t_prep = (time.perf_counter() - t_prep_start) * 1000.0

        # Stage 2: Inference (Tiled Person Detection + Hazard Classification)
        t_infer_start = time.perf_counter()

        # 2a. RGB Person detection (Tiled or full-frame)
        if settings.enable_tiling:
            raw_rgb = self.tiling_engine.predict_tiled(self.person_detector, rgb_frame)
        else:
            raw_rgb = self.person_detector._infer(rgb_frame)

        # 2b. Thermal Person detection if thermal frame provided
        raw_thermal = []
        if thermal_frame is not None:
            if settings.enable_tiling:
                raw_thermal = self.tiling_engine.predict_tiled(self.thermal_detector, thermal_frame)
            else:
                raw_thermal = self.thermal_detector._infer(thermal_frame)

        # 2c. Hazard Whole-frame classification on RGB
        hazard_dets = self.hazard_classifier.predict(rgb_frame, frame_id, timestamp)

        t_infer = (time.perf_counter() - t_infer_start) * 1000.0

        # Stage 3: Postprocess & Tracking & Fusion
        t_post_start = time.perf_counter()

        # 3a. Update ByteTrack on RGB detections
        active_rgb_tracks = self.rgb_tracker.update(
            raw_rgb,
            self.person_detector.class_names,
            modality="rgb",
        )
        rgb_detections: List[Detection] = []
        for track in active_rgb_tracks:
            rgb_detections.append(
                Detection(
                    id=str(uuid.uuid4()),
                    **{"class": track.class_name},
                    confidence=track.score,
                    bbox=BBox(x=track.bbox[0], y=track.bbox[1], w=track.bbox[2], h=track.bbox[3]),
                    frame_id=frame_id,
                    source=Source.rgb,
                    timestamp=timestamp,
                    track_id=str(track.track_id),
                    modalities=[Source.rgb],
                    model_version=self.person_detector.model_version,
                )
            )

        # 3b. Update ByteTrack on Thermal detections
        thermal_detections: List[Detection] = []
        if thermal_frame is not None:
            active_thermal_tracks = self.thermal_tracker.update(
                raw_thermal,
                self.thermal_detector.class_names,
                modality="thermal",
            )
            for track in active_thermal_tracks:
                thermal_detections.append(
                    Detection(
                        id=str(uuid.uuid4()),
                        **{"class": track.class_name},
                        confidence=track.score,
                        bbox=BBox(x=track.bbox[0], y=track.bbox[1], w=track.bbox[2], h=track.bbox[3]),
                        frame_id=frame_id,
                        source=Source.thermal,
                        timestamp=timestamp,
                        track_id=str(track.track_id),
                        modalities=[Source.thermal],
                        model_version=self.thermal_detector.model_version,
                    )
                )

        # 3c. Cross-modal sensor fusion
        fused_person_detections = self.fusion_engine.fuse(rgb_detections, thermal_detections)

        # 3d. Combine object detections and hazard classifications
        all_detections = fused_person_detections + hazard_dets
        t_post = (time.perf_counter() - t_post_start) * 1000.0

        # Stage 4: Publish
        t_pub_start = time.perf_counter()
        t_total_before_pub = (time.perf_counter() - t_start) * 1000.0

        # Attach per-stage timings
        stage_timings = StageTimings(
            decode=0.0,
            preprocess=round(t_prep, 2),
            inference=round(t_infer, 2),
            postprocess=round(t_post, 2),
            publish=0.0,
            total=round(t_total_before_pub, 2),
        )

        for d in all_detections:
            d.timings_ms = stage_timings
            if self.redis_client is not None:
                xadd_json(self.redis_client, self.stream_name, d.model_dump(by_alias=True))

        t_pub = (time.perf_counter() - t_pub_start) * 1000.0
        t_total = (time.perf_counter() - t_start) * 1000.0
        stage_timings.publish = round(t_pub, 2)
        stage_timings.total = round(t_total, 2)

        # Record metrics
        metrics.stage_latency.observe({"stage": "preprocess"}, t_prep / 1000.0)
        metrics.stage_latency.observe({"stage": "inference"}, t_infer / 1000.0)
        metrics.stage_latency.observe({"stage": "postprocess"}, t_post / 1000.0)
        metrics.stage_latency.observe({"stage": "publish"}, t_pub / 1000.0)
        metrics.stage_latency.observe({"stage": "total"}, t_total / 1000.0)
        metrics.record_frame_processed()
        metrics.active_tracks_gauge.set(len(active_rgb_tracks))

        if t_total > settings.frame_deadline_ms:
            logger.warning(
                "Frame %s exceeded processing deadline (%.1fms > %.1fms)",
                frame_id,
                t_total,
                settings.frame_deadline_ms,
            )

        return all_detections
