"""Cross-modal RGB-Thermal sensor registration and detection fusion."""

import json
from pathlib import Path
from typing import List, Optional, Tuple
import numpy as np

from app.models.base import RawDetection, clamp01
from app.pipeline.tracker import compute_iou_matrix
from app.schemas import BBox, Detection, DetectionClass, Source


class CrossModalRegistrar:
    """Manages geometric homography warping between thermal and RGB coordinate frames."""

    def __init__(self, homography_path: Optional[str] = None):
        self.H = np.eye(3, dtype=np.float32)

        if homography_path and Path(homography_path).is_file():
            try:
                with open(homography_path) as f:
                    data = json.load(f)
                    if "homography_matrix" in data:
                        self.H = np.array(data["homography_matrix"], dtype=np.float32)
            except Exception:
                pass

    def warp_bbox(self, bbox: Tuple[float, float, float, float]) -> Tuple[float, float, float, float]:
        """Warps a normalized [x, y, w, h] bounding box through homography H into RGB frame."""
        x, y, w, h = bbox
        corners = np.array(
            [
                [x, y, 1.0],
                [x + w, y, 1.0],
                [x + w, y + h, 1.0],
                [x, y + h, 1.0],
            ],
            dtype=np.float32,
        ).T

        warped = self.H @ corners
        warped_x = warped[0] / (warped[2] + 1e-7)
        warped_y = warped[1] / (warped[2] + 1e-7)

        min_x = clamp01(float(np.min(warped_x)))
        min_y = clamp01(float(np.min(warped_y)))
        max_x = clamp01(float(np.max(warped_x)))
        max_y = clamp01(float(np.max(warped_y)))

        return (min_x, min_y, max(1e-4, max_x - min_x), max(1e-4, max_y - min_y))


class SensorFusionEngine:
    """Associates cross-modal detections by IoU after spatial warping, raising confidence on agreement."""

    def __init__(
        self,
        homography_path: Optional[str] = None,
        iou_threshold: float = 0.30,
        boost_factor: float = 0.15,
    ):
        self.registrar = CrossModalRegistrar(homography_path)
        self.iou_threshold = iou_threshold
        self.boost_factor = boost_factor

    def fuse(
        self,
        rgb_detections: List[Detection],
        thermal_detections: List[Detection],
    ) -> List[Detection]:
        """Fuses RGB and thermal detections for a synchronized frame pair.

        - When modalities agree on a target (IoU >= threshold), confidence is boosted and
          modalities is set to [rgb, thermal].
        - Thermal-only detections remain valid (e.g. victims obscured by smoke/darkness).
        - RGB-only detections remain valid.
        """
        if not thermal_detections:
            for d in rgb_detections:
                if not d.modalities:
                    d.modalities = [Source.rgb]
            return rgb_detections

        if not rgb_detections:
            for d in thermal_detections:
                if not d.modalities:
                    d.modalities = [Source.thermal]
            return thermal_detections

        # Warp thermal bounding boxes into RGB coordinate system
        warped_thermal_boxes = []
        for td in thermal_detections:
            orig_box = (td.bbox.x, td.bbox.y, td.bbox.w, td.bbox.h)
            warped_thermal_boxes.append(self.registrar.warp_bbox(orig_box))

        rgb_boxes = np.array([[d.bbox.x, d.bbox.y, d.bbox.w, d.bbox.h] for d in rgb_detections], dtype=np.float32)
        th_boxes = np.array(warped_thermal_boxes, dtype=np.float32)

        iou_matrix = compute_iou_matrix(rgb_boxes, th_boxes)

        matched_rgb = set()
        matched_thermal = set()
        fused_results: List[Detection] = []

        # Find greedy matches above IoU threshold
        while True:
            max_iou = 0.0
            best_r, best_t = -1, -1
            for r_idx in range(len(rgb_detections)):
                if r_idx in matched_rgb:
                    continue
                for t_idx in range(len(thermal_detections)):
                    if t_idx in matched_thermal:
                        continue
                    if rgb_detections[r_idx].class_ != thermal_detections[t_idx].class_:
                        continue
                    if iou_matrix[r_idx, t_idx] > max_iou:
                        max_iou = iou_matrix[r_idx, t_idx]
                        best_r, best_t = r_idx, t_idx

            if max_iou < self.iou_threshold or best_r < 0:
                break

            matched_rgb.add(best_r)
            matched_thermal.add(best_t)

            rd = rgb_detections[best_r]
            td = thermal_detections[best_t]

            # Boost confidence upon multi-sensor agreement
            boosted_conf = min(
                0.99,
                max(rd.confidence, td.confidence) + self.boost_factor * min(rd.confidence, td.confidence),
            )

            # Refined weighted bounding box
            w_r = rd.confidence
            w_t = td.confidence
            total_w = w_r + w_t
            fused_x = (rd.bbox.x * w_r + th_boxes[best_t][0] * w_t) / total_w
            fused_y = (rd.bbox.y * w_r + th_boxes[best_t][1] * w_t) / total_w
            fused_w = (rd.bbox.w * w_r + th_boxes[best_t][2] * w_t) / total_w
            fused_h = (rd.bbox.h * w_r + th_boxes[best_t][3] * w_t) / total_w

            fused_det = Detection(
                id=rd.id,
                **{"class": rd.class_},
                confidence=round(boosted_conf, 3),
                bbox=BBox(
                    x=clamp01(float(fused_x)),
                    y=clamp01(float(fused_y)),
                    w=clamp01(float(fused_w)),
                    h=clamp01(float(fused_h)),
                ),
                frame_id=rd.frame_id,
                source=Source.rgb,
                timestamp=rd.timestamp,
                track_id=rd.track_id or td.track_id,
                modalities=[Source.rgb, Source.thermal],
                model_version=rd.model_version,
                timings_ms=rd.timings_ms,
            )
            fused_results.append(fused_det)

        # Append unmatched RGB detections
        for r_idx in range(len(rgb_detections)):
            if r_idx not in matched_rgb:
                rd = rgb_detections[r_idx]
                if not rd.modalities:
                    rd.modalities = [Source.rgb]
                fused_results.append(rd)

        # Append unmatched Thermal-only detections (valid survivors in darkness/smoke)
        for t_idx in range(len(thermal_detections)):
            if t_idx not in matched_thermal:
                td = thermal_detections[t_idx]
                tb = th_boxes[t_idx]
                thermal_only_det = Detection(
                    id=td.id,
                    **{"class": td.class_},
                    confidence=td.confidence,
                    bbox=BBox(x=tb[0], y=tb[1], w=tb[2], h=tb[3]),
                    frame_id=td.frame_id,
                    source=Source.thermal,
                    timestamp=td.timestamp,
                    track_id=td.track_id,
                    modalities=[Source.thermal],
                    model_version=td.model_version,
                    timings_ms=td.timings_ms,
                )
                fused_results.append(thermal_only_det)

        return fused_results
