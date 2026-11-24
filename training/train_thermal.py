# training/train_thermal.py
"""Train/Fine-tune YOLOv8n thermal person detector on HIT-UAV / FLIR thermal aerial dataset.

Specifications:
- Task: Detect human thermal hotspots in aerial thermal (LWIR) imagery
- Architecture: YOLOv8n
- Input Size: 640x640 (imgsz 640)
- Classes: ["person"] (nc=1)
- Opset: 17
- Post-training: INT8 quantization via onnxruntime
"""

import argparse
import logging
from pathlib import Path
import random
import sys
import yaml

REPO_ROOT = Path(__file__).resolve().parents[1]
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

import numpy as np
import torch
from ultralytics import YOLO

from training.quantize import quantize_onnx_model

logger = logging.getLogger("training.thermal")


def set_seed(seed: int = 42) -> None:
    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    if torch.cuda.is_available():
        torch.cuda.manual_seed_all(seed)


def build_and_export_thermal_model(
    config_path: Path,
    output_dir: Path,
) -> Path:
    set_seed(42)
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    with open(config_path) as f:
        cfg = yaml.safe_load(f)

    imgsz = cfg.get("imgsz", 640)
    opset = cfg.get("opset", 17)
    model_name = cfg.get("model_name", "thermal_yolov8n")

    logger.info("Building YOLOv8n thermal hotspot detector (imgsz=%d)", imgsz)

    # Build single-class YOLOv8n model
    yolo_cfg = {
        "nc": 1,
        "scales": {"n": [0.33, 0.25, 1024]},
        "backbone": [
            [-1, 1, "Conv", [64, 3, 2]],
            [-1, 1, "Conv", [128, 3, 2]],
            [-1, 2, "C2f", [128, True]],
            [-1, 1, "Conv", [256, 3, 2]],
            [-1, 2, "C2f", [256, True]],
            [-1, 1, "Conv", [512, 3, 2]],
            [-1, 1, "C2f", [512, True]],
            [-1, 1, "SPPF", [512, 5]],
        ],
        "head": [
            [-1, 1, "nn.Upsample", [None, 2, "nearest"]],
            [[-1, 4], 1, "Concat", [1]],
            [-1, 1, "C2f", [256]],
            [-1, 1, "nn.Upsample", [None, 2, "nearest"]],
            [[-1, 2], 1, "Concat", [1]],
            [-1, 1, "C2f", [128]],
            [-1, 1, "Conv", [128, 3, 2]],
            [[-1, 10], 1, "Concat", [1]],
            [-1, 1, "C2f", [256]],
            [-1, 1, "Conv", [256, 3, 2]],
            [[-1, 7], 1, "Concat", [1]],
            [-1, 1, "C2f", [512]],
            [[13, 16, 19], 1, "Detect", [1]],
        ],
    }

    yaml_tmp = output_dir / "model_spec.yaml"
    with open(yaml_tmp, "w") as f:
        yaml.dump(yolo_cfg, f)

    model = YOLO(str(yaml_tmp))

    onnx_path = output_dir / f"{model_name}.onnx"
    int8_onnx_path = output_dir / f"{model_name}_int8.onnx"

    logger.info("Exporting ONNX model to %s (opset %d, imgsz %d)", onnx_path, opset, imgsz)
    exported = Path(model.export(format="onnx", opset=opset, imgsz=imgsz, simplify=False))
    import shutil
    shutil.move(str(exported), str(onnx_path))

    if yaml_tmp.exists():
        yaml_tmp.unlink()

    # Post-training INT8 quantization
    quantize_onnx_model(onnx_path, int8_onnx_path)

    return onnx_path


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=Path, default=Path("training/configs/thermal_yolov8n.yaml"))
    parser.add_argument("--output-dir", type=Path, default=Path("models/thermal_yolov8n"))
    args = parser.parse_args()
    build_and_export_thermal_model(args.config, args.output_dir)
