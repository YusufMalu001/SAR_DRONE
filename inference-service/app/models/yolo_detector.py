import uuid
from typing import List

import numpy as np

from app.models.base import RawDetection, clamp01, require_weights
from app.schemas import BBox, Detection, Source


class YoloOnnxDetector:
    """Generic YOLOv8 object detector backed by an exported ONNX model.

    Both the person detector and the thermal hotspot detector are the same
    architecture (YOLOv8n, single "person" class) trained on different
    datasets, so they share this implementation and only differ in which
    weights file and `Source` they're configured with.

    Ultralytics can load an `.onnx` file directly and runs it through
    onnxruntime internally, which keeps this class simple while still being
    edge-deployable (no PyTorch runtime required at inference time).
    """

    def __init__(
        self,
        model_path: str,
        class_names: List[str],
        source: Source,
        train_script: str,
        conf_threshold: float = 0.4,
        iou_threshold: float = 0.45,
    ):
        self.model_path = model_path
        self.class_names = class_names
        self.source = source
        self.train_script = train_script
        self.conf_threshold = conf_threshold
        self.iou_threshold = iou_threshold
        self._model = None

    def _ensure_loaded(self):
        if self._model is None:
            require_weights(self.model_path, self.train_script)
            from ultralytics import YOLO

            self._model = YOLO(self.model_path, task="detect")
        return self._model

    def _infer(self, frame: np.ndarray) -> List[RawDetection]:
        model = self._ensure_loaded()
        results = model.predict(
            frame,
            conf=self.conf_threshold,
            iou=self.iou_threshold,
            verbose=False,
        )[0]

        raw: List[RawDetection] = []
        boxes = results.boxes
        if boxes is None:
            return raw

        for box in boxes:
            class_id = int(box.cls[0])
            if class_id >= len(self.class_names):
                # Model reports a class outside what this detector expects
                # (e.g. base COCO weights before fine-tuning) — skip it.
                continue
            confidence = float(box.conf[0])
            x_center, y_center, w, h = box.xywhn[0].tolist()
            raw.append(
                RawDetection(
                    class_id=class_id,
                    confidence=confidence,
                    x=x_center - w / 2,
                    y=y_center - h / 2,
                    w=w,
                    h=h,
                )
            )
        return raw

    def predict(self, frame: np.ndarray, frame_id: str, timestamp: str) -> List[Detection]:
        detections: List[Detection] = []
        for raw in self._infer(frame):
            detections.append(
                Detection(
                    id=str(uuid.uuid4()),
                    **{"class": self.class_names[raw.class_id]},
                    confidence=raw.confidence,
                    bbox=BBox(
                        x=clamp01(raw.x),
                        y=clamp01(raw.y),
                        w=clamp01(raw.w),
                        h=clamp01(raw.h),
                    ),
                    frame_id=frame_id,
                    source=self.source,
                    timestamp=timestamp,
                )
            )
        return detections
