"""Pure ONNX-Runtime YOLOv8 detector with configurable execution providers (CPU/CUDA/TensorRT)."""

import os
import uuid
from typing import List, Optional, Tuple

import cv2
import numpy as np

from app.config import settings
from app.models.base import RawDetection, clamp01, require_weights
from app.schemas import BBox, Detection, Source


def letterbox(
    im: np.ndarray,
    new_shape: Tuple[int, int] = (640, 640),
    color: Tuple[int, int, int] = (114, 114, 114),
    auto: bool = False,
    scaleFill: bool = False,
    scaleup: bool = True,
    stride: int = 32,
) -> Tuple[np.ndarray, float, Tuple[float, float]]:
    """Resizes and pads image while preserving aspect ratio, returning padded image, scale ratio, and pad offsets."""
    shape = im.shape[:2]  # current shape [height, width]
    if isinstance(new_shape, int):
        new_shape = (new_shape, new_shape)

    # Scale ratio (new / old)
    r = min(new_shape[0] / shape[0], new_shape[1] / shape[1])
    if not scaleup:  # only scale down, do not scale up (for better val mAP)
        r = min(r, 1.0)

    # Compute padding
    new_unpad = (int(round(shape[1] * r)), int(round(shape[0] * r)))
    dw, dh = new_shape[1] - new_unpad[0], new_shape[0] - new_unpad[1]  # wh padding
    if auto:  # minimum rectangle
        dw, dh = np.mod(dw, stride), np.mod(dh, stride)
    elif scaleFill:  # stretch
        dw, dh = 0.0, 0.0
        new_unpad = (new_shape[1], new_shape[0])
        r = new_shape[1] / shape[1], new_shape[0] / shape[0]

    dw /= 2  # divide padding into 2 sides
    dh /= 2

    if shape[::-1] != new_unpad:  # resize
        im = cv2.resize(im, new_unpad, interpolation=cv2.INTER_LINEAR)
    top, bottom = int(round(dh - 0.1)), int(round(dh + 0.1))
    left, right = int(round(dw - 0.1)), int(round(dw + 0.1))
    im = cv2.copyMakeBorder(im, top, bottom, left, right, cv2.BORDER_CONSTANT, value=color)
    return im, r, (dw, dh)


def nms_boxes_numpy(
    boxes: np.ndarray,
    scores: np.ndarray,
    classes: np.ndarray,
    iou_threshold: float = 0.45,
) -> List[int]:
    """Class-aware Non-Maximum Suppression (NMS) in pure NumPy.

    `boxes`: (N, 4) in [x1, y1, x2, y2] format
    `scores`: (N,)
    `classes`: (N,)
    """
    if len(boxes) == 0:
        return []

    # Offset boxes by class to perform class-specific NMS in a single vectorized pass
    max_coord = boxes.max() if len(boxes) > 0 else 0.0
    c_offset = classes * (max_coord + 1.0)
    b = boxes + c_offset[:, None]

    x1 = b[:, 0]
    y1 = b[:, 1]
    x2 = b[:, 2]
    y2 = b[:, 3]
    areas = np.maximum(0.0, x2 - x1) * np.maximum(0.0, y2 - y1)
    order = scores.argsort()[::-1]

    keep = []
    while order.size > 0:
        i = order[0]
        keep.append(int(i))
        if order.size == 1:
            break

        xx1 = np.maximum(x1[i], x1[order[1:]])
        yy1 = np.maximum(y1[i], y1[order[1:]])
        xx2 = np.minimum(x2[i], x2[order[1:]])
        yy2 = np.minimum(y2[i], y2[order[1:]])

        w = np.maximum(0.0, xx2 - xx1)
        h = np.maximum(0.0, yy2 - yy1)
        inter = w * h
        iou = inter / (areas[i] + areas[order[1:]] - inter + 1e-7)

        inds = np.where(iou <= iou_threshold)[0]
        order = order[inds + 1]

    return keep


class YoloOnnxDetector:
    """Production-grade YOLOv8 detector using pure ONNX Runtime.

    Supports CPU, CUDA, and TensorRT execution providers configurable at runtime.
    """

    def __init__(
        self,
        model_path: str,
        class_names: List[str],
        source: Source = Source.rgb,
        train_script: str = "train_person_model.py",
        conf_threshold: float = 0.4,
        iou_threshold: float = 0.45,
        input_size: Tuple[int, int] = (640, 640),
        model_version: str = "yolov8n-v2.0",
    ):
        self.model_path = model_path
        self.class_names = class_names
        self.source = source
        self.train_script = train_script
        self.conf_threshold = conf_threshold
        self.iou_threshold = iou_threshold
        self.input_size = input_size
        self.model_version = model_version
        self._session = None
        self._input_name = None
        self._output_name = None

    def _ensure_loaded(self):
        if self._session is None:
            require_weights(self.model_path, self.train_script)
            import onnxruntime as ort

            opts = ort.SessionOptions()
            opts.intra_op_num_threads = settings.ort_intra_op_threads
            opts.inter_op_num_threads = settings.ort_inter_op_threads
            opts.graph_optimization_level = ort.GraphOptimizationLevel.ORT_ENABLE_ALL

            providers = settings.get_onnx_providers()
            available = ort.get_available_providers()
            active_providers = [p for p in providers if p in available] or ["CPUExecutionProvider"]

            self._session = ort.InferenceSession(self.model_path, sess_options=opts, providers=active_providers)
            self._input_name = self._session.get_inputs()[0].name
            self._output_name = self._session.get_outputs()[0].name

        return self._session

    def preprocess(self, img: np.ndarray) -> Tuple[np.ndarray, float, Tuple[float, float]]:
        """Letterbox and normalize to [1, 3, H, W] NCHW float32 tensor."""
        padded, ratio, (dw, dh) = letterbox(img, new_shape=self.input_size, auto=False)
        # BGR -> RGB
        rgb = cv2.cvtColor(padded, cv2.COLOR_BGR2RGB)
        # HWC -> CHW, normalized [0, 1]
        blob = np.ascontiguousarray(rgb.transpose((2, 0, 1)), dtype=np.float32) / 255.0
        blob = np.expand_dims(blob, axis=0)
        return blob, ratio, (dw, dh)

    def postprocess(
        self,
        raw_output: np.ndarray,
        orig_shape: Tuple[int, int],
        ratio: float,
        pad: Tuple[float, float],
    ) -> List[RawDetection]:
        """Parses YOLOv8 raw output tensor and performs NMS."""
        # Output shape is typically (1, 4 + num_classes, 8400) or (1, 8400, 4 + num_classes)
        preds = raw_output[0]
        if preds.shape[0] < preds.shape[1]:
            # (4 + num_classes, 8400) -> (8400, 4 + num_classes)
            preds = preds.transpose((1, 0))

        boxes = preds[:, :4]  # cx, cy, w, h
        scores = preds[:, 4:]  # class scores

        # Find best class per candidate box
        class_ids = np.argmax(scores, axis=1)
        confidences = scores[np.arange(len(scores)), class_ids]

        # Filter by confidence threshold
        mask = confidences >= self.conf_threshold
        if not np.any(mask):
            return []

        boxes = boxes[mask]
        confidences = confidences[mask]
        class_ids = class_ids[mask]

        # Convert cx, cy, w, h to x1, y1, x2, y2 in padded coordinate system
        dw, dh = pad
        cx = boxes[:, 0]
        cy = boxes[:, 1]
        w = boxes[:, 2]
        h = boxes[:, 3]

        # Unpad and scale back to original image dimensions
        orig_h, orig_w = orig_shape
        x1 = np.clip((cx - w / 2.0 - dw) / ratio, 0.0, orig_w)
        y1 = np.clip((cy - h / 2.0 - dh) / ratio, 0.0, orig_h)
        x2 = np.clip((cx + w / 2.0 - dw) / ratio, 0.0, orig_w)
        y2 = np.clip((cy + h / 2.0 - dh) / ratio, 0.0, orig_h)

        xyxy = np.stack([x1, y1, x2, y2], axis=1)
        keep = nms_boxes_numpy(xyxy, confidences, class_ids, iou_threshold=self.iou_threshold)

        detections: List[RawDetection] = []
        for idx in keep:
            bx1, by1, bx2, by2 = xyxy[idx]
            bw = (bx2 - bx1) / orig_w
            bh = (by2 - by1) / orig_h
            if bw <= 0 or bh <= 0:
                continue

            detections.append(
                RawDetection(
                    class_id=int(class_ids[idx]),
                    confidence=float(confidences[idx]),
                    x=clamp01(float(bx1 / orig_w)),
                    y=clamp01(float(by1 / orig_h)),
                    w=clamp01(float(bw)),
                    h=clamp01(float(bh)),
                )
            )

        return detections

    def _infer(self, frame: np.ndarray) -> List[RawDetection]:
        session = self._ensure_loaded()
        blob, ratio, pad = self.preprocess(frame)
        raw_output = session.run([self._output_name], {self._input_name: blob})[0]
        return self.postprocess(raw_output, frame.shape[:2], ratio, pad)

    def predict(self, frame: np.ndarray, frame_id: str, timestamp: str) -> List[Detection]:
        raw_dets = self._infer(frame)
        out: List[Detection] = []
        for raw in raw_dets:
            if raw.class_id >= len(self.class_names):
                continue
            out.append(
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
                    model_version=self.model_version,
                )
            )
        return out
