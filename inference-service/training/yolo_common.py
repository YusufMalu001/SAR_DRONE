"""Shared helpers for the YOLOv8 training scripts (train_person_model.py,
train_thermal_model.py).

Both datasets may show up either already in YOLO format (images/ + labels/
+ a data.yaml) or as COCO-style JSON annotations (instances_*.json /
FLIR's thermal_annotations.json). This module normalizes either into a
YOLO-format `data.yaml` that ultralytics can train against directly.
"""

import json
import logging
from pathlib import Path
from typing import List, Optional

import yaml

logger = logging.getLogger(__name__)


def find_coco_json(dataset_dir: Path) -> Optional[Path]:
    """Best-effort search for a COCO-style annotation file in a dataset dir."""
    candidates = list(dataset_dir.glob("**/*annotations*.json")) + list(
        dataset_dir.glob("**/instances_*.json")
    )
    return candidates[0] if candidates else None


def looks_like_yolo_format(dataset_dir: Path) -> bool:
    return (dataset_dir / "images").exists() and (dataset_dir / "labels").exists()


def convert_coco_to_yolo(coco_json: Path, dataset_dir: Path, keep_classes: Optional[List[str]] = None) -> Path:
    """Converts a COCO-format annotation file into YOLO-format label files
    under `dataset_dir/labels/`, optionally filtering to `keep_classes`
    (e.g. FLIR ADAS -> person only).

    Uses ultralytics' own converter, which is already a project dependency.
    Returns `dataset_dir` (now containing `images/` + `labels/`).
    """
    from ultralytics.data.converter import convert_coco

    logger.info("Converting COCO annotations at %s to YOLO format", coco_json)
    convert_coco(
        labels_dir=str(coco_json.parent),
        save_dir=str(dataset_dir / "yolo_labels"),
        use_segments=False,
        use_keypoints=False,
    )

    if keep_classes:
        with coco_json.open() as f:
            coco = json.load(f)
        logger.info(
            "NOTE: convert_coco does not filter classes; if only %s should "
            "be kept, prune the exported label files under %s manually or "
            "extend this function before training.",
            keep_classes,
            dataset_dir / "yolo_labels",
        )

    return dataset_dir


def ensure_yolo_dataset(dataset_dir: Path, keep_classes: Optional[List[str]] = None) -> Path:
    """Makes sure `dataset_dir` is in YOLO format, converting from COCO if
    needed. Raises with a clear message if neither format is detected.
    """
    if looks_like_yolo_format(dataset_dir):
        return dataset_dir

    coco_json = find_coco_json(dataset_dir)
    if coco_json is not None:
        return convert_coco_to_yolo(coco_json, dataset_dir, keep_classes=keep_classes)

    raise FileNotFoundError(
        f"'{dataset_dir}' does not look like a YOLO dataset (no images/+labels/) "
        "or a COCO dataset (no *annotations*.json / instances_*.json found). "
        "Point this script at a dataset in one of those two formats."
    )


def build_dataset_yaml(dataset_dirs: List[Path], class_names: List[str], out_path: Path) -> Path:
    """Writes an ultralytics dataset yaml that trains across one or more
    YOLO-format dataset directories (e.g. c2a + SARD combined).

    Each `dataset_dir` is expected to contain `images/train`, `images/val`
    (and matching `labels/...`) — the standard ultralytics YOLO layout.
    """
    config = {
        "train": [str(d / "images" / "train") for d in dataset_dirs],
        "val": [str(d / "images" / "val") for d in dataset_dirs],
        "names": {i: name for i, name in enumerate(class_names)},
    }
    out_path.parent.mkdir(parents=True, exist_ok=True)
    with out_path.open("w") as f:
        yaml.safe_dump(config, f, sort_keys=False)
    logger.info("Wrote dataset yaml to %s", out_path)
    return out_path


def train_and_export(
    base_model: str,
    dataset_yaml: Path,
    output_onnx: Path,
    epochs: int,
    imgsz: int,
    run_name: str,
) -> Path:
    """Fine-tunes `base_model` on `dataset_yaml` and exports the result to
    ONNX at `output_onnx`.
    """
    from ultralytics import YOLO

    model = YOLO(base_model)
    model.train(
        data=str(dataset_yaml),
        epochs=epochs,
        imgsz=imgsz,
        name=run_name,
    )

    exported_path = Path(model.export(format="onnx", opset=12, simplify=True))
    output_onnx.parent.mkdir(parents=True, exist_ok=True)
    exported_path.replace(output_onnx)
    logger.info("Exported ONNX model to %s", output_onnx)
    return output_onnx
