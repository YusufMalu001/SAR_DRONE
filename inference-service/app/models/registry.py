# inference-service/app/models/registry.py
"""Central Model Registry loader and integrity verifier.

Loads model metadata from models/registry.json and verifies SHA256 checksums
before permitting inference-service to load ONNX weights.
"""

import hashlib
import json
import logging
import os
from pathlib import Path
from typing import Dict, Any, List, Optional

logger = logging.getLogger("inference.registry")


class ModelIntegrityError(ValueError):
    """Raised when on-disk ONNX weights fail SHA256 checksum verification."""
    pass


class ModelRegistry:
    def __init__(self, registry_path: Optional[str] = None):
        if registry_path:
            self.registry_path = Path(registry_path)
        else:
            candidates = [
                Path("models/registry.json"),
                Path("/models/registry.json"),
                Path(__file__).resolve().parents[3] / "models/registry.json",
                Path(__file__).resolve().parents[2] / "models/registry.json",
            ]
            self.registry_path = next((p for p in candidates if p.exists()), Path("models/registry.json"))

        self._data: Optional[Dict[str, Any]] = None

    def _load(self) -> Dict[str, Any]:
        if self._data is None:
            if not self.registry_path.exists():
                raise FileNotFoundError(f"Model registry not found at: {self.registry_path}")
            with open(self.registry_path) as f:
                self._data = json.load(f)
        return self._data

    def get_model_entry(self, model_name: str) -> Dict[str, Any]:
        data = self._load()
        models = data.get("models", {})
        if model_name not in models:
            raise KeyError(f"Model '{model_name}' not registered in {self.registry_path}")
        return models[model_name]

    def verify_and_get_path(self, model_name: str, prefer_int8: bool = False) -> str:
        entry = self.get_model_entry(model_name)
        target_key = "int8_onnx_path" if prefer_int8 and entry.get("int8_onnx_path") else "onnx_path"
        expected_sha = entry.get("int8_sha256") if prefer_int8 and entry.get("int8_sha256") else entry.get("sha256")

        rel_path = entry[target_key]
        candidates = [
            Path(rel_path),
            Path("/") / rel_path,
            Path("/models") / Path(rel_path).name,
            self.registry_path.parent / Path(rel_path).relative_to("models") if rel_path.startswith("models/") else self.registry_path.parent / rel_path,
        ]

        resolved_path = next((p for p in candidates if p.exists()), None)
        if resolved_path is None:
            raise FileNotFoundError(f"ONNX weights file '{rel_path}' for model '{model_name}' not found on disk.")

        # Compute SHA256 checksum
        hasher = hashlib.sha256()
        with open(resolved_path, "rb") as f:
            while chunk := f.read(65536):
                hasher.update(chunk)
        actual_sha = hasher.hexdigest()

        if expected_sha and actual_sha != expected_sha:
            raise ModelIntegrityError(
                f"Checksum mismatch for model '{model_name}' at {resolved_path}!\n"
                f"Expected SHA256: {expected_sha}\n"
                f"Actual SHA256:   {actual_sha}"
            )

        logger.info("Verified model '%s' integrity (SHA256: %s...)", model_name, actual_sha[:12])
        return str(resolved_path)

    def get_supported_classes(self, model_name: str) -> List[str]:
        entry = self.get_model_entry(model_name)
        return [c["name"] for c in entry.get("classes", []) if c.get("status") == "supported"]

    def get_all_classes(self, model_name: str) -> List[str]:
        entry = self.get_model_entry(model_name)
        return [c["name"] for c in entry.get("classes", [])]


# Global singleton instance
registry = ModelRegistry()
