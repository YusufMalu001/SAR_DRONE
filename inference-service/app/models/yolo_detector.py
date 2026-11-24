import uuid
from typing import List, Optional

import cv2
import numpy as np

from app.models.base import RawDetection, clamp01, require_weights
from app.models.registry import registry
from app.schemas import BBox, Detection, Source
from training.tiled_inference import compute_tiles, nms_boxes


class YoloOnnxDetector:
    """Generic YOLOv8 object detector backed by an exported ONNX model,
    verified and loaded via the central model registry.
    """

    def __init__(
        self,
        model_name: str,
        class_names: List[str],
        source: Source,
        train_script: str,
        conf_threshold: float = 0.4,
        iou_threshold: float = 0.45,
        model_path: Optional[str] = None,
        use_tiled: bool = False,
    ):
        self.model_name = model_name
        self.class_names = class_names
        self.source = source
        self.train_script = train_script
        self.conf_threshold = conf_threshold
        self.iou_threshold = iou_threshold
        self.model_path = model_path
        self.use_tiled = use_tiled
        self._model = None

    def _ensure_loaded(self):
        if self._model is None:
            if self.model_path and os.path.exists(self.model_path):
                path = self.model_path
            else:
                try:
                    path = registry.verify_and_get_path(self.model_name)
                except Exception:
                    # Fallback to configured path with clear error
                    require_weights(self.model_path or f"models/{self.model_name}/{self.model_name}.onnx", self.train_script)
                    path = self.model_path

            from ultralytics import YOLO
            self._model = YOLO(path, task="detect")
        return self._model

    def _infer_patch(self, img_patch: np.ndarray) -> List[RawDetection]:
        model = self._ensure_loaded()
        results = model.predict(
            img_patch,
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

    def _infer(self, frame: np.ndarray) -> List[RawDetection]:
        if not self.use_tiled:
            return self._infer_patch(frame)

        # Tiled sliding window inference for small targets
        h, w = frame.shape[:2]
        tiles = compute_tiles(h, w, tile_h=640, tile_w=640, overlap_ratio=0.20)
        all_boxes = []
        all_scores = []
        all_classes = []

        for x1, y1, x2, y2 in tiles:
            patch = frame[y1:y2, x1:x2]
            patch_h, patch_w = patch.shape[:2]
            patch_dets = self._infer_patch(patch)
            for d in patch_dets:
                # Map patch relative coordinates to full-frame pixels
                fx1 = x1 + d.x * patch_w
                fy1 = y1 + d.y * patch_h
                fx2 = fx1 + d.w * patch_w
                fy2 = fy1 + d.h * patch_h
                all_boxes.append([fx1, fy1, fx2, fy2])
                all_scores.append(d.confidence)
                all_classes.append(d.class_id)

        # Full-frame global inference
        full_dets = self._infer_patch(frame)
        for d in full_dets:
            all_boxes.append([d.x * w, d.y * h, (d.x + d.w) * w, (d.y + d.h) * h])
            all_scores.append(d.confidence)
            all_classes.append(d.class_id)

        if not all_boxes:
            return []

        boxes_arr = np.array(all_boxes)
        scores_arr = np.array(all_scores)
        classes_arr = np.array(all_classes)

        kept_indices = nms_boxes(boxes_arr, scores_arr, classes_arr, iou_threshold=self.iou_threshold)

        merged: List[RawDetection] = []
        for idx in kept_indices:
            bx1, by1, bx2, by2 = boxes_arr[idx]
            bw = (bx2 - bx1) / w
            bh = (by2 - by1) / h
            merged.append(
                RawDetection(
                    class_id=int(classes_arr[idx]),
                    confidence=float(scores_arr[idx]),
                    x=bx1 / w,
                    y=by1 / h,
                    w=bw,
                    h=bh,
                )
            )
        return merged

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
