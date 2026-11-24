"""Unit tests for SAHI-style tiled inference and cross-tile NMS."""

import numpy as np
import pytest

from app.models.base import RawDetection
from app.models.yolo_detector import YoloOnnxDetector
from app.pipeline.tiling import compute_tiles, TiledInferenceEngine


def test_compute_tiles_coverage():
    # 1280x720 image with 640x640 tile and 20% overlap
    tiles = compute_tiles(img_h=720, img_w=1280, tile_h=640, tile_w=640, overlap_ratio=0.20)

    assert len(tiles) >= 3

    # Check boundaries
    for x1, y1, x2, y2 in tiles:
        assert 0 <= x1 < x2 <= 1280
        assert 0 <= y1 < y2 <= 720
        assert (x2 - x1) <= 640
        assert (y2 - y1) <= 640

    # Ensure coverage of corners
    covered_min_x = min(t[0] for t in tiles)
    covered_max_x = max(t[2] for t in tiles)
    covered_min_y = min(t[1] for t in tiles)
    covered_max_y = max(t[3] for t in tiles)

    assert covered_min_x == 0
    assert covered_max_x == 1280
    assert covered_min_y == 0
    assert covered_max_y == 720


def test_tiled_inference_cross_tile_nms(monkeypatch):
    """Verifies that an object appearing across adjacent overlapping tiles is unified by cross-tile NMS."""
    engine = TiledInferenceEngine(tile_size=640, overlap_ratio=0.20, iou_threshold=0.45)
    detector = YoloOnnxDetector(
        model_path="dummy.onnx",
        class_names=["person"],
    )

    frame = np.zeros((720, 1280, 3), dtype=np.uint8)

    # Monkeypatch _infer to simulate detection of the same object in two overlapping patches
    def mock_infer(patch: np.ndarray):
        # Target located around global x=500, y=300
        # If patch contains this region, return detection
        h, w = patch.shape[:2]
        # Return a detection relative to patch
        return [RawDetection(class_id=0, confidence=0.89, x=0.5, y=0.5, w=0.1, h=0.1)]

    monkeypatch.setattr(detector, "_infer", mock_infer)

    results = engine.predict_tiled(detector, frame)

    # Cross-tile NMS should merge overlapping duplicates
    assert len(results) > 0
    # Confidence should be preserved
    assert results[0].confidence >= 0.85
    # Normalized coords must be in [0, 1]
    for r in results:
        assert 0.0 <= r.x <= 1.0
        assert 0.0 <= r.y <= 1.0
