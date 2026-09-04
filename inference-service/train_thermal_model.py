#!/usr/bin/env python
"""Fine-tunes YOLOv8n on the FLIR ADAS thermal dataset (person class only)
and exports the result to ONNX for ThermalDetector (app/models/thermal_detector.py).

Usage:
    python train_thermal_model.py \
        --data-dir data/datasets/flir_thermal \
        --output data/models/thermal_yolov8.onnx \
        --epochs 50

FLIR ADAS ships COCO-style JSON annotations (thermal_annotations.json)
covering multiple classes (person, bike, car, ...); this script converts
them to YOLO format and trains a single-class ("person") detector — see
training/yolo_common.py's `keep_classes` note for where to prune the other
classes out of the converted labels if you need a stricter filter than
just restricting the training class list below.
"""

import argparse
import logging
from pathlib import Path

from training.yolo_common import build_dataset_yaml, ensure_yolo_dataset, train_and_export

logger = logging.getLogger(__name__)

CLASS_NAMES = ["person"]


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data-dir", type=Path, default=Path("data/datasets/flir_thermal"))
    parser.add_argument("--base-model", default="yolov8n.pt")
    parser.add_argument("--epochs", type=int, default=50)
    parser.add_argument("--imgsz", type=int, default=640)
    parser.add_argument("--output", type=Path, default=Path("data/models/thermal_yolov8.onnx"))
    parser.add_argument(
        "--dataset-yaml",
        type=Path,
        default=Path("data/datasets/thermal_dataset.yaml"),
    )
    return parser


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    args = build_arg_parser().parse_args()

    dataset_dir = ensure_yolo_dataset(args.data_dir, keep_classes=CLASS_NAMES)
    dataset_yaml = build_dataset_yaml([dataset_dir], CLASS_NAMES, args.dataset_yaml)

    train_and_export(
        base_model=args.base_model,
        dataset_yaml=dataset_yaml,
        output_onnx=args.output,
        epochs=args.epochs,
        imgsz=args.imgsz,
        run_name="thermal_yolov8",
    )


if __name__ == "__main__":
    main()
