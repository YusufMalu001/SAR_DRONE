from app.config import settings
from app.models.yolo_detector import YoloOnnxDetector
from app.schemas import Source


class ThermalDetector(YoloOnnxDetector):
    """YOLOv8n thermal hotspot (person) detector, trained on HIT-UAV / FLIR Thermal.
    
    Loads and validates weights via models/registry.json.
    """

    def __init__(self, model_path: str | None = None, conf_threshold: float | None = None):
        super().__init__(
            model_name="thermal_yolov8n",
            class_names=["person"],
            source=Source.thermal,
            train_script="training/train_thermal.py",
            conf_threshold=conf_threshold if conf_threshold is not None else settings.thermal_conf_threshold,
            iou_threshold=settings.iou_threshold,
            model_path=model_path or settings.thermal_model_path,
            use_tiled=False,
        )
