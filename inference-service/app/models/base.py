import os
from dataclasses import dataclass


class ModelNotFoundError(FileNotFoundError):
    """Raised when a model's ONNX weights file is missing.

    Carries the training script the caller should run to produce it.
    """

    def __init__(self, model_path: str, train_script: str):
        super().__init__(
            f"Model weights not found at '{model_path}'. "
            f"Run `python {train_script}` to train and export it first."
        )
        self.model_path = model_path
        self.train_script = train_script


def require_weights(model_path: str, train_script: str) -> None:
    if not os.path.exists(model_path):
        raise ModelNotFoundError(model_path, train_script)


def clamp01(value: float) -> float:
    return max(0.0, min(1.0, value))


@dataclass
class RawDetection:
    """A single decoded detection, independent of any inference-engine API.

    Kept separate from the pydantic `Detection` schema so detector classes
    can be unit-tested by mocking `_infer()` without needing real model
    weights or ultralytics/onnxruntime internals.
    """

    class_id: int
    confidence: float
    x: float  # normalized top-left x, [0, 1]
    y: float  # normalized top-left y, [0, 1]
    w: float  # normalized width, [0, 1]
    h: float  # normalized height, [0, 1]


@dataclass
class RawClassification:
    class_name: str
    confidence: float
