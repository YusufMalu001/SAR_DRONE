import uuid
from typing import List, Optional

import cv2
import numpy as np

from app.config import settings
from app.models.base import RawClassification, require_weights
from app.schemas import BBox, Detection, DetectionClass, Source

# AIDER dataset classes, in the order the classifier's output layer was
# trained with. Alphabetical to match torchvision.datasets.ImageFolder's
# default class_to_idx ordering (see ../../train_hazard_classifier.py) —
# keep this in sync with that script if the dataset's folder names change.
AIDER_CLASSES = ["collapsed_building", "fire", "flood", "normal", "traffic_accident"]

# Maps AIDER classes onto the shared Detection.class enum. "normal" has no
# corresponding hazard and never emits a detection. "traffic_accident" is
# mapped to "debris" as the closest available hazard class for this
# prototype's taxonomy.
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
    """Whole-frame hazard classifier (fire/flood/debris/structural_damage).

    Backed by a ResNet18 (or EfficientNet-B0) fine-tuned on AIDER and
    exported to ONNX — see ../../train_hazard_classifier.py.

    This is a classifier, not a segmenter: it labels the whole frame rather
    than localizing the hazard, so every emitted Detection carries a
    full-frame bbox ({x:0, y:0, w:1, h:1}) covering the entire frame extent.

    TODO(segmentation): once time allows, replace this with a DeepLabV3
    (or similar) segmentation model so `bbox` reflects the actual hazard
    extent instead of the whole frame. The `predict()` signature and the
    Detection this emits would stay the same — only `_infer()` and bbox
    computation change.
    """

    def __init__(
        self,
        model_path: Optional[str] = None,
        input_size: int = 224,
        conf_threshold: Optional[float] = None,
    ):
        self.model_path = model_path or settings.hazard_model_path
        self.input_size = input_size
        self.conf_threshold = (
            conf_threshold if conf_threshold is not None else settings.hazard_conf_threshold
        )
        self.source = Source.rgb
        self.train_script = "train_hazard_classifier.py"
        self._session = None

    def _ensure_loaded(self):
        if self._session is None:
            require_weights(self.model_path, self.train_script)
            import onnxruntime as ort

            self._session = ort.InferenceSession(
                self.model_path, providers=["CPUExecutionProvider"]
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

        detection_class = AIDER_TO_DETECTION_CLASS.get(raw.class_name)
        if detection_class is None:
            return []

        return [
            Detection(
                id=str(uuid.uuid4()),
                **{"class": detection_class},
                confidence=raw.confidence,
                bbox=BBox(x=0.0, y=0.0, w=1.0, h=1.0),
                frame_id=frame_id,
                source=self.source,
                timestamp=timestamp,
            )
        ]
