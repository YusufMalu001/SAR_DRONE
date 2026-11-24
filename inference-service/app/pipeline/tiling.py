"""SAHI-style sliced/tiled inference for high-altitude UAV detection of small targets."""

import math
from typing import List, Tuple
import numpy as np

from app.models.base import RawDetection, clamp01
from app.models.yolo_detector import YoloOnnxDetector, nms_boxes_numpy


def compute_tiles(
    img_h: int,
    img_w: int,
    tile_h: int = 640,
    tile_w: int = 640,
    overlap_ratio: float = 0.20,
) -> List[Tuple[int, int, int, int]]:
    """Generates overlapping sliding window tile slices covering the full image.

    Returns a list of (x1, y1, x2, y2) pixel bounding boxes.
    """
    if img_h <= tile_h and img_w <= tile_w:
        return [(0, 0, img_w, img_h)]

    step_x = max(1, int(tile_w * (1.0 - overlap_ratio)))
    step_y = max(1, int(tile_h * (1.0 - overlap_ratio)))

    x_starts = list(range(0, max(1, img_w - tile_w + 1), step_x))
    if not x_starts or x_starts[-1] + tile_w < img_w:
        x_starts.append(max(0, img_w - tile_w))

    y_starts = list(range(0, max(1, img_h - tile_h + 1), step_y))
    if not y_starts or y_starts[-1] + tile_h < img_h:
        y_starts.append(max(0, img_h - tile_h))

    tiles: List[Tuple[int, int, int, int]] = []
    for y1 in y_starts:
        y2 = min(img_h, y1 + tile_h)
        for x1 in x_starts:
            x2 = min(img_w, x1 + tile_w)
            tiles.append((x1, y1, x2, y2))

    return tiles


class TiledInferenceEngine:
    """Orchestrates tiled patch inference and global frame inference with cross-tile NMS."""

    def __init__(
        self,
        tile_size: int = 640,
        overlap_ratio: float = 0.20,
        iou_threshold: float = 0.45,
    ):
        self.tile_size = tile_size
        self.overlap_ratio = overlap_ratio
        self.iou_threshold = iou_threshold

    def predict_tiled(
        self,
        detector: YoloOnnxDetector,
        frame: np.ndarray,
    ) -> List[RawDetection]:
        h, w = frame.shape[:2]

        # If image fits in one tile, single-pass inference is sufficient
        if h <= self.tile_size and w <= self.tile_size:
            return detector._infer(frame)

        tiles = compute_tiles(h, w, self.tile_size, self.tile_size, self.overlap_ratio)

        all_boxes = []
        all_scores = []
        all_classes = []

        # 1. Tiled local patch inference
        for x1, y1, x2, y2 in tiles:
            patch = frame[y1:y2, x1:x2]
            patch_h, patch_w = patch.shape[:2]
            patch_dets = detector._infer(patch)

            for d in patch_dets:
                fx1 = x1 + (d.x * patch_w)
                fy1 = y1 + (d.y * patch_h)
                fx2 = fx1 + (d.w * patch_w)
                fy2 = fy1 + (d.h * patch_h)
                all_boxes.append([fx1, fy1, fx2, fy2])
                all_scores.append(d.confidence)
                all_classes.append(d.class_id)

        # 2. Global full-frame inference (for larger context / larger objects)
        global_dets = detector._infer(frame)
        for d in global_dets:
            fx1 = d.x * w
            fy1 = d.y * h
            fx2 = (d.x + d.w) * w
            fy2 = (d.y + d.h) * h
            all_boxes.append([fx1, fy1, fx2, fy2])
            all_scores.append(d.confidence)
            all_classes.append(d.class_id)

        if not all_boxes:
            return []

        boxes_arr = np.array(all_boxes, dtype=np.float32)
        scores_arr = np.array(all_scores, dtype=np.float32)
        classes_arr = np.array(all_classes, dtype=np.int32)

        # 3. Cross-tile NMS
        keep_indices = nms_boxes_numpy(
            boxes_arr,
            scores_arr,
            classes_arr,
            iou_threshold=self.iou_threshold,
        )

        merged: List[RawDetection] = []
        for idx in keep_indices:
            bx1, by1, bx2, by2 = boxes_arr[idx]
            bw = (bx2 - bx1) / w
            bh = (by2 - by1) / h
            if bw <= 0 or bh <= 0:
                continue

            merged.append(
                RawDetection(
                    class_id=int(classes_arr[idx]),
                    confidence=float(scores_arr[idx]),
                    x=clamp01(float(bx1 / w)),
                    y=clamp01(float(by1 / h)),
                    w=clamp01(float(bw)),
                    h=clamp01(float(bh)),
                )
            )

        return merged
