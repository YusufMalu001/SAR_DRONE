from app.config import settings
from app.models.yolo_detector import YoloOnnxDetector
from app.schemas import Source


class PersonDetector(YoloOnnxDetector):
    """YOLOv8n aerial person detector, fine-tuned on HERIDAL / VisDrone.
    
    Supports tiled inference for small target aerial survivors.
    Loads and validates weights via models/registry.json.
    """

    def __init__(self, model_path: str | None = None, conf_threshold: float | None = None, use_tiled: bool = True):
        super().__init__(
            model_name="person_yolov8n",
            class_names=["person"],
            source=Source.rgb,
            train_script="training/train_person.py",
            conf_threshold=conf_threshold if conf_threshold is not None else settings.person_conf_threshold,
            iou_threshold=settings.iou_threshold,
            model_path=model_path or settings.person_model_path,
            use_tiled=use_tiled,
        )
