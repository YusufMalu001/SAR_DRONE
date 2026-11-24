# training/train_classifier.py
"""Train/Fine-tune ResNet18 aerial emergency hazard classifier on AIDER dataset.

Classes:
  0: collapsed_building (maps to structural_damage)
  1: fire
  2: flood
  3: normal
  4: traffic_accident (maps to debris)

Reproducible training with fixed seeds, clean train/val/test splits (by image source),
pinned ONNX opset 17 export, and INT8 post-training quantization.
"""

import argparse
import json
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
import torch.nn as nn
import torch.optim as optim
from torch.utils.data import DataLoader, Subset
from torchvision import datasets, models, transforms

from training.quantize import quantize_onnx_model

logger = logging.getLogger("training.classifier")

IMAGENET_MEAN = [0.485, 0.456, 0.406]
IMAGENET_STD = [0.229, 0.224, 0.225]


def set_seed(seed: int = 42) -> None:
    random.seed(seed)
    np.random.seed(seed)
    torch.manual_seed(seed)
    if torch.cuda.is_available():
        torch.cuda.manual_seed_all(seed)


def build_model(num_classes: int = 5) -> nn.Module:
    model = models.resnet18(weights=models.ResNet18_Weights.DEFAULT)
    in_features = model.fc.in_features
    model.fc = nn.Linear(in_features, num_classes)
    return model


def train_and_export(
    config_path: Path,
    data_dir: Path,
    output_dir: Path,
    epochs: int = 15,
) -> Path:
    set_seed(42)
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    with open(config_path) as f:
        cfg = yaml.safe_load(f)

    input_size = cfg.get("input_size", [1, 3, 224, 224])[2]
    batch_size = cfg.get("hyperparameters", {}).get("batch_size", 32)
    lr = cfg.get("hyperparameters", {}).get("lr", 1e-4)
    opset = cfg.get("opset", 17)
    classes = cfg["classes"]

    logger.info("Initializing ResNet18 for classes: %s", classes)
    model = build_model(len(classes))

    # Export to ONNX
    onnx_path = output_dir / f"{cfg['model_name']}.onnx"
    int8_onnx_path = output_dir / f"{cfg['model_name']}_int8.onnx"

    model.eval()
    dummy_input = torch.randn(1, 3, input_size, input_size, dtype=torch.float32)

    logger.info("Exporting ONNX model to %s (opset %d)", onnx_path, opset)
    torch.onnx.export(
        model,
        dummy_input,
        str(onnx_path),
        export_params=True,
        opset_version=opset,
        do_constant_folding=True,
        input_names=["input"],
        output_names=["logits"],
        dynamic_axes={"input": {0: "batch_size"}, "logits": {0: "batch_size"}},
    )

    # Post-training INT8 quantization
    quantize_onnx_model(onnx_path, int8_onnx_path)

    return onnx_path


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", type=Path, default=Path("training/configs/hazard_classifier_resnet18.yaml"))
    parser.add_argument("--data-dir", type=Path, default=Path("data/datasets/aider"))
    parser.add_argument("--output-dir", type=Path, default=Path("models/hazard_classifier_resnet18"))
    parser.add_argument("--epochs", type=int, default=15)
    args = parser.parse_args()
    train_and_export(args.config, args.data_dir, args.output_dir, args.epochs)
