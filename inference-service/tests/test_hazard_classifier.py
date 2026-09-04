import jsonschema

from app.models.base import RawClassification
from app.models.hazard_classifier import HazardClassifier
from app.schemas import DetectionClass, Source


def test_hazard_classifier_output_matches_schema(sample_image, detection_schema, monkeypatch):
    classifier = HazardClassifier(model_path="unused-in-this-test.onnx", conf_threshold=0.5)
    monkeypatch.setattr(
        classifier, "_infer", lambda frame: RawClassification(class_name="fire", confidence=0.93)
    )

    detections = classifier.predict(sample_image, "frame_000001", "2026-09-04T12:00:00Z")

    assert len(detections) == 1
    detection = detections[0]
    assert detection.class_ == DetectionClass.fire
    assert detection.source == Source.rgb
    # Whole-frame placeholder bbox until segmentation replaces the classifier.
    assert detection.bbox.x == 0.0 and detection.bbox.y == 0.0
    assert detection.bbox.w == 1.0 and detection.bbox.h == 1.0

    payload = detection.model_dump(by_alias=True)
    jsonschema.validate(instance=payload, schema=detection_schema)


def test_hazard_classifier_maps_collapsed_building_to_structural_damage(sample_image, monkeypatch):
    classifier = HazardClassifier(model_path="unused-in-this-test.onnx", conf_threshold=0.5)
    monkeypatch.setattr(
        classifier,
        "_infer",
        lambda frame: RawClassification(class_name="collapsed_building", confidence=0.8),
    )

    detection = classifier.predict(sample_image, "frame_000002", "2026-09-04T12:00:01Z")[0]

    assert detection.class_ == DetectionClass.structural_damage


def test_hazard_classifier_normal_class_emits_no_detection(sample_image, monkeypatch):
    classifier = HazardClassifier(model_path="unused-in-this-test.onnx", conf_threshold=0.5)
    monkeypatch.setattr(
        classifier, "_infer", lambda frame: RawClassification(class_name="normal", confidence=0.99)
    )

    assert classifier.predict(sample_image, "frame_000003", "2026-09-04T12:00:02Z") == []


def test_hazard_classifier_below_threshold_emits_no_detection(sample_image, monkeypatch):
    classifier = HazardClassifier(model_path="unused-in-this-test.onnx", conf_threshold=0.5)
    monkeypatch.setattr(
        classifier, "_infer", lambda frame: RawClassification(class_name="flood", confidence=0.2)
    )

    assert classifier.predict(sample_image, "frame_000004", "2026-09-04T12:00:03Z") == []
