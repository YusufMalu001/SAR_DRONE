import uuid
from typing import List, Optional

import cv2
import numpy as np

from app.config import settings
from app.models.base import RawClassification, require_weights
from app.models.registry import registry
from app.schemas import BBox, Detection, DetectionClass, Source

AIDER_CLASSES = ["collapsed_building", "fire", "flood", "normal", "traffic_accident"]

AIDER_TO_DETECTION_CLASS = {
    "fire": DetectionClass.fire,
    "flood": DetectionClass.flood,
    "collapsed_building": DetectionClass.structural_damage,
    "traffic_accident": DetectionClass.debris,
    "normal": None,
}

IMAGENET_MEAN = np.array([0.485, 0.456, 0.406], dtype=np.float32)
IMAGENET_STD = np.array([0.229, 0.224, 0.225], dtype=np.float32)


class HazardClassifier:
    """Whole-frame aerial hazard classifier backed by ResNet18 fine-tuned on AIDER.
    
    Weights are verified and loaded via models/registry.json.
    """

    def __init__(
        self,
        model_path: Optional[str] = None,
        input_size: int = 224,
        conf_threshold: Optional[float] = None,
    ):
        self.model_name = "hazard_classifier_resnet18"
        self.model_path = model_path or settings.hazard_model_path
        self.input_size = input_size
        self.conf_threshold = (
            conf_threshold if conf_threshold is not None else settings.hazard_conf_threshold
        )
        self.source = Source.rgb
        self.train_script = "training/train_classifier.py"
        self._session = None

    def _ensure_loaded(self):
        if self._session is None:
            try:
                resolved_path = registry.verify_and_get_path(self.model_name)
            except Exception:
                require_weights(self.model_path, self.train_script)
                resolved_path = self.model_path

            import onnxruntime as ort
            self._session = ort.InferenceSession(
                resolved_path, providers=["CPUExecutionProvider"]
            )
        return self._session

    def _preprocess(self, frame: np.ndarray) -> np.ndarray:
        resized = cv2.resize(frame, (self.input_size, self.input_size))
        rgb = cv2.cvtColor(resized, cv2.COLOR_BGR2RGB).astype(np.float32) / 255.0
        normalized = (rgb - IMAGENET_MEAN) / IMAGENET_STD
        chw = np.transpose(normalized, (2, 0, 1))
        return np.expand_dims(chw, axis=0).astype(np.float32)

    @staticmethod
    def _softmax(logits: np.ndarray) -> np.ndarray:
        shifted = logits - np.max(logits)
        exp = np.exp(shifted)
        return exp / exp.sum()

    def _infer(self, frame: np.ndarray) -> RawClassification:
        session = self._ensure_loaded()
        tensor = self._preprocess(frame)
        input_name = session.get_inputs()[0].name
        logits = session.run(None, {input_name: tensor})[0][0]
        probs = self._softmax(logits)
        idx = int(np.argmax(probs))
        return RawClassification(class_name=AIDER_CLASSES[idx], confidence=float(probs[idx]))

    def predict(self, frame: np.ndarray, frame_id: str, timestamp: str) -> List[Detection]:
        raw = self._infer(frame)
        if raw.confidence < self.conf_threshold:
            return []

        det_class = AIDER_TO_DETECTION_CLASS.get(raw.class_name)
        if det_class is None:
            return []

        return [
            Detection(
                id=str(uuid.uuid4()),
                **{"class": det_class},
                confidence=raw.confidence,
                bbox=BBox(x=0.0, y=0.0, w=1.0, h=1.0),
                frame_id=frame_id,
                source=self.source,
                timestamp=timestamp,
            )
        ]
