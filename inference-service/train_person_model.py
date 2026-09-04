#!/usr/bin/env python
"""Fine-tunes YOLOv8n on the c2a + SARD person-detection datasets and
exports the result to ONNX for PersonDetector (app/models/person_detector.py).

Usage:
    python train_person_model.py \
        --c2a-dir data/datasets/c2a \
        --sard-dir data/datasets/sard \
        --output data/models/person_yolov8.onnx \
        --epochs 50

Each dataset dir may be either:
  - already in YOLO format (images/{train,val}, labels/{train,val}), or
  - COCO format (an *annotations*.json / instances_*.json file) — this
    script converts it to YOLO format automatically (see training/yolo_common.py).
"""

import argparse
import logging
from pathlib import Path

from training.yolo_common import build_dataset_yaml, ensure_yolo_dataset, train_and_export

logger = logging.getLogger(__name__)

CLASS_NAMES = ["person"]


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--c2a-dir", type=Path, default=Path("data/datasets/c2a"))
    parser.add_argument("--sard-dir", type=Path, default=Path("data/datasets/sard"))
    parser.add_argument("--base-model", default="yolov8n.pt")
    parser.add_argument("--epochs", type=int, default=50)
    parser.add_argument("--imgsz", type=int, default=640)
    parser.add_argument("--output", type=Path, default=Path("data/models/person_yolov8.onnx"))
    parser.add_argument(
        "--dataset-yaml",
        type=Path,
        default=Path("data/datasets/person_dataset.yaml"),
        help="Where to write the merged dataset yaml before training.",
    )
    return parser


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    args = build_arg_parser().parse_args()

    dataset_dirs = [
        ensure_yolo_dataset(args.c2a_dir),
        ensure_yolo_dataset(args.sard_dir),
    ]
    dataset_yaml = build_dataset_yaml(dataset_dirs, CLASS_NAMES, args.dataset_yaml)

    train_and_export(
        base_model=args.base_model,
        dataset_yaml=dataset_yaml,
        output_onnx=args.output,
        epochs=args.epochs,
        imgsz=args.imgsz,
        run_name="person_yolov8",
    )


if __name__ == "__main__":
    main()
