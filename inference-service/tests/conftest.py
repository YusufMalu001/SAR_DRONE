import json
from pathlib import Path

import numpy as np
import pytest

REPO_ROOT = Path(__file__).resolve().parents[2]
DETECTION_SCHEMA_PATH = REPO_ROOT / "shared" / "jsonschema" / "detection.schema.json"


@pytest.fixture
def sample_image() -> np.ndarray:
    """A single synthetic BGR frame, standing in for a real video/thermal
    frame — the detectors under test only need a correctly-shaped array
    since `_infer()` is mocked in these tests.
    """
    rng = np.random.default_rng(0)
    return rng.integers(0, 255, size=(480, 640, 3), dtype=np.uint8)


@pytest.fixture
def detection_schema() -> dict:
    with DETECTION_SCHEMA_PATH.open() as f:
        return json.load(f)
