import jsonschema

from app.models.base import RawDetection
from app.models.person_detector import PersonDetector
from app.schemas import DetectionClass, Source


def test_person_detector_output_matches_schema(sample_image, detection_schema, monkeypatch):
    detector = PersonDetector(model_path="unused-in-this-test.onnx")
    monkeypatch.setattr(
        detector,
        "_infer",
        lambda frame: [RawDetection(class_id=0, confidence=0.87, x=0.1, y=0.2, w=0.3, h=0.4)],
    )

    detections = detector.predict(sample_image, "frame_000001", "2026-09-04T12:00:00Z")

    assert len(detections) == 1
    detection = detections[0]
    assert detection.class_ == DetectionClass.person
    assert detection.source == Source.rgb
    assert 0.0 <= detection.confidence <= 1.0
    assert detection.bbox.x == 0.1
    assert detection.bbox.y == 0.2

    payload = detection.model_dump(by_alias=True)
    jsonschema.validate(instance=payload, schema=detection_schema)


def test_person_detector_no_detections(sample_image, monkeypatch):
    detector = PersonDetector(model_path="unused-in-this-test.onnx")
    monkeypatch.setattr(detector, "_infer", lambda frame: [])

    detections = detector.predict(sample_image, "frame_000002", "2026-09-04T12:00:01Z")

    assert detections == []
