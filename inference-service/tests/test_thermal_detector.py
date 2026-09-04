import jsonschema

from app.models.base import RawDetection
from app.models.thermal_detector import ThermalDetector
from app.schemas import DetectionClass, Source


def test_thermal_detector_output_matches_schema(sample_image, detection_schema, monkeypatch):
    detector = ThermalDetector(model_path="unused-in-this-test.onnx")
    monkeypatch.setattr(
        detector,
        "_infer",
        lambda frame: [RawDetection(class_id=0, confidence=0.62, x=0.5, y=0.5, w=0.1, h=0.1)],
    )

    detections = detector.predict(sample_image, "frame_000001", "2026-09-04T12:00:00Z")

    assert len(detections) == 1
    detection = detections[0]
    assert detection.class_ == DetectionClass.person
    assert detection.source == Source.thermal

    payload = detection.model_dump(by_alias=True)
    jsonschema.validate(instance=payload, schema=detection_schema)


def test_thermal_detector_bbox_clamped_to_unit_range(sample_image, monkeypatch):
    detector = ThermalDetector(model_path="unused-in-this-test.onnx")
    # A box near the frame edge can decode to a slightly out-of-range top-left
    # corner (x_center - w/2 < 0); predict() must clamp it into [0, 1].
    monkeypatch.setattr(
        detector,
        "_infer",
        lambda frame: [RawDetection(class_id=0, confidence=0.9, x=-0.05, y=1.02, w=0.2, h=0.2)],
    )

    detection = detector.predict(sample_image, "frame_000003", "2026-09-04T12:00:02Z")[0]

    assert 0.0 <= detection.bbox.x <= 1.0
    assert 0.0 <= detection.bbox.y <= 1.0
