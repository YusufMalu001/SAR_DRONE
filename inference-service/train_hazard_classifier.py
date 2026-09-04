#!/usr/bin/env python
"""Fine-tunes a ResNet18 or EfficientNet-B0 classifier on the AIDER dataset
(fire / flood / collapsed_building / traffic_accident / normal) and exports
it to ONNX for HazardClassifier (app/models/hazard_classifier.py).

A classifier is used here instead of a segmenter to get something working
quickly; see the TODO(segmentation) note in app/models/hazard_classifier.py
for where a DeepLabV3 (or similar) segmentation model would slot in later
to localize the hazard instead of labeling the whole frame.

Usage:
    python train_hazard_classifier.py \
        --data-dir data/datasets/aider \
        --arch resnet18 \
        --output data/models/hazard_classifier.onnx \
        --epochs 15

Expects `data-dir` in torchvision ImageFolder layout:
    aider/
      fire/*.jpg
      flood/*.jpg
      collapsed_building/*.jpg
      traffic_accident/*.jpg
      normal/*.jpg
(optionally pre-split into train/ and val/ subdirectories containing the
same five class folders — auto-detected below).
"""

import argparse
import logging
from pathlib import Path

import torch
from torch import nn, optim
from torch.utils.data import DataLoader
from torchvision import datasets, models, transforms

from app.models.hazard_classifier import AIDER_CLASSES

logger = logging.getLogger(__name__)

IMAGENET_MEAN = [0.485, 0.456, 0.406]
IMAGENET_STD = [0.229, 0.224, 0.225]


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--data-dir", type=Path, default=Path("data/datasets/aider"))
    parser.add_argument("--arch", choices=["resnet18", "efficientnet_b0"], default="resnet18")
    parser.add_argument("--epochs", type=int, default=15)
    parser.add_argument("--batch-size", type=int, default=32)
    parser.add_argument("--lr", type=float, default=1e-4)
    parser.add_argument("--input-size", type=int, default=224)
    parser.add_argument("--val-split", type=float, default=0.2)
    parser.add_argument("--output", type=Path, default=Path("data/models/hazard_classifier.onnx"))
    return parser


def build_model(arch: str, num_classes: int) -> nn.Module:
    if arch == "resnet18":
        model = models.resnet18(weights=models.ResNet18_Weights.DEFAULT)
        model.fc = nn.Linear(model.fc.in_features, num_classes)
    else:
        model = models.efficientnet_b0(weights=models.EfficientNet_B0_Weights.DEFAULT)
        model.classifier[-1] = nn.Linear(model.classifier[-1].in_features, num_classes)
    return model


def build_datasets(data_dir: Path, input_size: int, val_split: float):
    transform = transforms.Compose(
        [
            transforms.Resize((input_size, input_size)),
            transforms.ToTensor(),
            transforms.Normalize(IMAGENET_MEAN, IMAGENET_STD),
        ]
    )

    train_dir, val_dir = data_dir / "train", data_dir / "val"
    if train_dir.exists() and val_dir.exists():
        train_ds = datasets.ImageFolder(str(train_dir), transform=transform)
        val_ds = datasets.ImageFolder(str(val_dir), transform=transform)
        return train_ds, val_ds

    full_ds = datasets.ImageFolder(str(data_dir), transform=transform)
    assert list(full_ds.classes) == AIDER_CLASSES, (
        f"Expected class folders {AIDER_CLASSES} (alphabetical), found {full_ds.classes}. "
        "Update AIDER_CLASSES in app/models/hazard_classifier.py to match if this dataset "
        "genuinely uses different class names."
    )
    val_size = int(len(full_ds) * val_split)
    train_size = len(full_ds) - val_size
    return torch.utils.data.random_split(full_ds, [train_size, val_size])


def train(model: nn.Module, train_loader: DataLoader, val_loader: DataLoader, epochs: int, lr: float, device: torch.device) -> None:
    model.to(device)
    criterion = nn.CrossEntropyLoss()
    optimizer = optim.Adam(model.parameters(), lr=lr)

    for epoch in range(epochs):
        model.train()
        running_loss = 0.0
        for images, labels in train_loader:
            images, labels = images.to(device), labels.to(device)
            optimizer.zero_grad()
            outputs = model(images)
            loss = criterion(outputs, labels)
            loss.backward()
            optimizer.step()
            running_loss += loss.item() * images.size(0)

        val_acc = evaluate(model, val_loader, device)
        logger.info(
            "epoch %d/%d - train_loss=%.4f val_acc=%.3f",
            epoch + 1,
            epochs,
            running_loss / len(train_loader.dataset),
            val_acc,
        )


@torch.no_grad()
def evaluate(model: nn.Module, loader: DataLoader, device: torch.device) -> float:
    model.eval()
    correct, total = 0, 0
    for images, labels in loader:
        images, labels = images.to(device), labels.to(device)
        preds = model(images).argmax(dim=1)
        correct += (preds == labels).sum().item()
        total += labels.size(0)
    return correct / total if total else 0.0


def export_onnx(model: nn.Module, input_size: int, output_path: Path, device: torch.device) -> None:
    model.eval()
    dummy_input = torch.randn(1, 3, input_size, input_size, device=device)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    torch.onnx.export(
        model,
        dummy_input,
        str(output_path),
        input_names=["input"],
        output_names=["logits"],
        opset_version=12,
        dynamic_axes={"input": {0: "batch"}, "logits": {0: "batch"}},
    )
    logger.info("Exported ONNX model to %s", output_path)


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    args = build_arg_parser().parse_args()

    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    train_ds, val_ds = build_datasets(args.data_dir, args.input_size, args.val_split)
    train_loader = DataLoader(train_ds, batch_size=args.batch_size, shuffle=True, num_workers=2)
    val_loader = DataLoader(val_ds, batch_size=args.batch_size, shuffle=False, num_workers=2)

    model = build_model(args.arch, num_classes=len(AIDER_CLASSES))
    train(model, train_loader, val_loader, args.epochs, args.lr, device)
    export_onnx(model, args.input_size, args.output, device)


if __name__ == "__main__":
    main()
