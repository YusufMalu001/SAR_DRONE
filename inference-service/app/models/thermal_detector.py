from app.config import settings
from app.models.yolo_detector import YoloOnnxDetector
from app.schemas import Source


class ThermalDetector(YoloOnnxDetector):
    """YOLOv8n thermal hotspot (person) detector, trained on FLIR ADAS.

    Weights: data/models/thermal_yolov8.onnx (see ../../train_thermal_model.py).

    NOTE: `video_ingest.py` currently feeds this detector the same RGB frame
    as `PersonDetector` because the demo footage only has one (visible-light)
    stream. In a real deployment this receives frames from a separate
    thermal camera synced to the same frame clock.
    """

    def __init__(self, model_path: str | None = None, conf_threshold: float | None = None):
        super().__init__(
            model_path=model_path or settings.thermal_model_path,
            class_names=["person"],
            source=Source.thermal,
            train_script="train_thermal_model.py",
            conf_threshold=conf_threshold if conf_threshold is not None else settings.thermal_conf_threshold,
            iou_threshold=settings.iou_threshold,
        )
