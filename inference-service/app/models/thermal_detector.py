from app.config import settings
from app.models.yolo_detector import YoloOnnxDetector
from app.schemas import Source


class ThermalDetector(YoloOnnxDetector):
    """YOLOv8n thermal hotspot (person) detector, running on ONNX Runtime.

    Weights: data/models/thermal_yolov8.onnx (see train_thermal_model.py).
    """

    def __init__(self, model_path: str | None = None, conf_threshold: float | None = None):
        super().__init__(
            model_path=model_path or settings.thermal_model_path,
            class_names=["person"],
            source=Source.thermal,
            train_script="train_thermal_model.py",
            conf_threshold=conf_threshold if conf_threshold is not None else settings.thermal_conf_threshold,
            iou_threshold=settings.iou_threshold,
            model_version="yolov8n-thermal-v2.0",
        )
