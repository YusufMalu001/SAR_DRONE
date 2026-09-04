from app.config import settings
from app.models.yolo_detector import YoloOnnxDetector
from app.schemas import Source


class PersonDetector(YoloOnnxDetector):
    """YOLOv8n person detector, fine-tuned on the c2a + SARD datasets.

    Weights: data/models/person_yolov8.onnx (see ../../train_person_model.py).
    """

    def __init__(self, model_path: str | None = None, conf_threshold: float | None = None):
        super().__init__(
            model_path=model_path or settings.person_model_path,
            class_names=["person"],
            source=Source.rgb,
            train_script="train_person_model.py",
            conf_threshold=conf_threshold if conf_threshold is not None else settings.person_conf_threshold,
            iou_threshold=settings.iou_threshold,
        )
