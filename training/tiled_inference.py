# training/tiled_inference.py
"""Tiled (sliced) inference support for small object aerial detection.

High-altitude UAV imagery (50m-100m AGL) renders human survivors as small targets
(often 10x10 to 30x30 pixels in 4K or 1080p frames). Standard whole-frame resizing
down to 640x640 can discard subtle spatial details.

Tiled inference slides overlapping windows across the original high-resolution frame,
runs inference per tile, projects bounding boxes back into full-frame normalized
coordinates [0, 1], and fuses overlapping detections with NMS (Non-Maximum Suppression).
"""

from typing import List, Tuple
import numpy as np


def compute_tiles(
    img_h: int,
    img_w: int,
    tile_h: int = 640,
    tile_w: int = 640,
    overlap_ratio: float = 0.20,
) -> List[Tuple[int, int, int, int]]:
    """Generates bounding boxes (x1, y1, x2, y2) for overlapping tiles across an image."""
    step_x = max(1, int(tile_w * (1.0 - overlap_ratio)))
    step_y = max(1, int(tile_h * (1.0 - overlap_ratio)))

    tiles: List[Tuple[int, int, int, int]] = []
    y1 = 0
    while y1 < img_h:
        y2 = min(img_h, y1 + tile_h)
        if y2 - y1 < tile_h and y1 > 0:
            y1 = max(0, img_h - tile_h)
            y2 = img_h

        x1 = 0
        while x1 < img_w:
            x2 = min(img_w, x1 + tile_w)
            if x2 - x1 < tile_w and x1 > 0:
                x1 = max(0, img_w - tile_w)
                x2 = img_w

            tiles.append((x1, y1, x2, y2))
            if x2 == img_w:
                break
            x1 += step_x

        if y2 == img_h:
            break
        y1 += step_y

    return tiles


def nms_boxes(
    boxes: np.ndarray,
    scores: np.ndarray,
    classes: np.ndarray,
    iou_threshold: float = 0.45,
) -> List[int]:
    """Applies Non-Maximum Suppression (NMS) per class.
    
    boxes: [N, 4] in (x1, y1, x2, y2) format.
    scores: [N] confidence scores.
    classes: [N] class IDs.
    Returns: list of kept indices.
    """
    if len(boxes) == 0:
        return []

    kept_indices: List[int] = []
    unique_classes = np.unique(classes)

    for c in unique_classes:
        c_idxs = np.where(classes == c)[0]
        c_boxes = boxes[c_idxs]
        c_scores = scores[c_idxs]

        x1 = c_boxes[:, 0]
        y1 = c_boxes[:, 1]
        x2 = c_boxes[:, 2]
        y2 = c_boxes[:, 3]
        areas = np.maximum(0.0, x2 - x1) * np.maximum(0.0, y2 - y1)

        order = c_scores.argsort()[::-1]

        while order.size > 0:
            i = order[0]
            kept_indices.append(c_idxs[i])

            if order.size == 1:
                break

            xx1 = np.maximum(x1[i], x1[order[1:]])
            yy1 = np.maximum(y1[i], y1[order[1:]])
            xx2 = np.minimum(x2[i], x2[order[1:]])
            yy2 = np.minimum(y2[i], y2[order[1:]])

            w = np.maximum(0.0, xx2 - xx1)
            h = np.maximum(0.0, yy2 - yy1)
            inter = w * h

            union = areas[i] + areas[order[1:]] - inter
            iou = np.zeros_like(union)
            nonzero_mask = union > 0
            iou[nonzero_mask] = inter[nonzero_mask] / union[nonzero_mask]

            remaining = np.where(iou <= iou_threshold)[0]
            order = order[remaining + 1]

    return kept_indices
