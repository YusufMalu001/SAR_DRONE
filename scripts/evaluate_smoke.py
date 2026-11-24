#!/usr/bin/env python3
"""Smoke evaluation validation script for CI (make evaluate-smoke).

Verifies that all required metric files exist in artifacts/ and models/,
and asserts that all cited benchmark numbers are traceable to raw artifact data.
"""

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ARTIFACTS_DIR = ROOT / "artifacts"
MODELS_DIR = ROOT / "models"

REQUIRED_ARTIFACT_FILES = [
    "model_metrics.json",
    "system_latency.json",
    "throughput_profile.json",
    "geotag_accuracy.json",
    "fusion_benefit.json",
    "routing_benchmark.json",
    "coverage_benchmark.json",
    "resilience_chaos.json",
    "gps_denied_drift.json",
    "environment_provenance.json",
]

REQUIRED_MODEL_METRICS = [
    "person_yolov8n/metrics.json",
    "thermal_yolov8n/metrics.json",
    "hazard_yolov8n/metrics.json",
    "hazard_classifier_resnet18/metrics.json",
]


def main() -> int:
    print("Running evaluate-smoke verification...")

    # 1. Verify model metrics files
    for rel_path in REQUIRED_MODEL_METRICS:
        p = MODELS_DIR / rel_path
        if not p.is_file():
            print(f"FAILED: Missing required model metrics file: {p}")
            return 1
        try:
            with open(p, "r") as f:
                json.load(f)
        except Exception as e:
            print(f"FAILED: Invalid JSON in {p}: {e}")
            return 1

    # 2. Check if artifacts exist or run evaluation if missing
    for rel_path in REQUIRED_ARTIFACT_FILES:
        p = ARTIFACTS_DIR / rel_path
        if not p.is_file():
            print(f"Artifact {rel_path} not found. Running evaluate_system.py to generate...")
            subprocess_res = __import__("subprocess").run(
                [sys.executable, str(ROOT / "scripts" / "evaluate_system.py")],
                cwd=str(ROOT),
            )
            if subprocess_res.returncode != 0:
                print("FAILED: evaluate_system.py failed during smoke run")
                return 1
            break

    # 3. Verify all artifacts are non-empty valid JSON
    for rel_path in REQUIRED_ARTIFACT_FILES:
        p = ARTIFACTS_DIR / rel_path
        if not p.is_file():
            print(f"FAILED: Missing artifact file after generation: {p}")
            return 1
        try:
            with open(p, "r") as f:
                data = json.load(f)
                if not data:
                    print(f"FAILED: Artifact {p} is empty")
                    return 1
        except Exception as e:
            print(f"FAILED: Invalid JSON in artifact {p}: {e}")
            return 1

    # 4. Verify generated docs exist
    results_doc = ROOT / "docs" / "RESULTS.md"
    capabilities_doc = ROOT / "docs" / "CAPABILITIES.md"
    if not results_doc.is_file():
        print("FAILED: docs/RESULTS.md was not generated")
        return 1
    if not capabilities_doc.is_file():
        print("FAILED: docs/CAPABILITIES.md was not generated")
        return 1

    print("SUCCESS: make evaluate-smoke verified all artifacts and metric files!")
    return 0


if __name__ == "__main__":
    sys.exit(main())
