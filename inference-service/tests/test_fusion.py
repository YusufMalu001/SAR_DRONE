"""Unit tests for cross-modal RGB-Thermal sensor registration and fusion."""

import pytest

from app.pipeline.fusion import CrossModalRegistrar, SensorFusionEngine
from app.schemas import BBox, Detection, DetectionClass, Source


def test_fusion_agreement_raises_confidence():
    """When both RGB and Thermal detect an overlapping target, confidence is boosted and modalities stamped."""
    engine = SensorFusionEngine(homography_path=None, iou_threshold=0.30, boost_factor=0.15)

    rgb_det = Detection(
        id="det-rgb-1",
        **{"class": DetectionClass.person},
        confidence=0.80,
        bbox=BBox(x=0.20, y=0.20, w=0.10, h=0.10),
        frame_id="frame_00000001",
        source=Source.rgb,
        timestamp="2026-11-24T12:00:00Z",
        track_id="track_1",
    )

    thermal_det = Detection(
        id="det-th-1",
        **{"class": DetectionClass.person},
        confidence=0.75,
        bbox=BBox(x=0.21, y=0.21, w=0.10, h=0.10),  # overlapping
        frame_id="frame_00000001",
        source=Source.thermal,
        timestamp="2026-11-24T12:00:00Z",
        track_id="th_track_1",
    )

    fused = engine.fuse([rgb_det], [thermal_det])

    assert len(fused) == 1
    d = fused[0]
    # Boosted confidence: > max(0.80, 0.75) = 0.80
    assert d.confidence > 0.80
    assert d.modalities == [Source.rgb, Source.thermal]
    assert d.class_ == DetectionClass.person
    assert d.track_id == "track_1"


def test_fusion_thermal_only_remains_valid():
    """Thermal detections without visible light match (e.g. concealed survivor) remain valid."""
    engine = SensorFusionEngine(homography_path=None, iou_threshold=0.30)

    # RGB sees a person at x=0.1
    rgb_det = Detection(
        id="det-rgb-1",
        **{"class": DetectionClass.person},
        confidence=0.85,
        bbox=BBox(x=0.10, y=0.10, w=0.05, h=0.05),
        frame_id="frame_00000001",
        source=Source.rgb,
        timestamp="2026-11-24T12:00:00Z",
        track_id="track_1",
    )

    # Thermal sees a survivor at x=0.8 (hidden in smoke/darkness, no RGB match)
    thermal_det = Detection(
        id="det-th-hidden",
        **{"class": DetectionClass.person},
        confidence=0.78,
        bbox=BBox(x=0.80, y=0.80, w=0.06, h=0.06),
        frame_id="frame_00000001",
        source=Source.thermal,
        timestamp="2026-11-24T12:00:00Z",
        track_id="track_hidden",
    )

    fused = engine.fuse([rgb_det], [thermal_det])

    assert len(fused) == 2
    classes = [d.class_ for d in fused]
    assert classes == [DetectionClass.person, DetectionClass.person]

    thermal_only = next(d for d in fused if d.source == Source.thermal)
    assert thermal_only.modalities == [Source.thermal]
    assert thermal_only.confidence == 0.78
    assert abs(thermal_only.bbox.x - 0.80) < 1e-4
