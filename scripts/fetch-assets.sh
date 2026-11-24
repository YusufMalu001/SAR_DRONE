#!/usr/bin/env bash
# scripts/fetch-assets.sh
#
# Fetches, generates, and verifies required demonstration footage, simulation assets,
# and datasets for the SAR Drone system under data/ (gitignored).
#
# Usage:
#   ./scripts/fetch-assets.sh [--verify-only] [--force]
#
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

CHECKSUMS_FILE="scripts/checksums.sha256"
VERIFY_ONLY=false
FORCE=false

for arg in "$@"; do
  case "$arg" in
    --verify-only) VERIFY_ONLY=true ;;
    --force) FORCE=true ;;
    *) echo "Unknown flag: $arg" >&2; exit 1 ;;
  esac
done

echo "=========================================================="
echo "SAR Drone Asset Fetcher & Checksum Verifier"
echo "=========================================================="

# 1. Ensure required data directories exist
mkdir -p data/models data/videos data/datasets data/sample_footage

# 2. Check and generate/obtain video assets
VIDEOS=(
  "data/videos/flood_demo.mp4"
  "data/videos/flood_demo_thermal.mp4"
  "data/videos/fire_demo.mp4"
  "data/videos/fire_demo_thermal.mp4"
  "data/videos/collapse_demo.mp4"
  "data/videos/collapse_demo_thermal.mp4"
  "data/sample_footage/demo_flight.mp4"
  "data/sample_footage/demo_flight_thermal.mp4"
)

missing_videos=false
for v in "${VIDEOS[@]}"; do
  if [ ! -f "$v" ] || [ "$FORCE" = true ]; then
    missing_videos=true
    break
  fi
done

if [ "$missing_videos" = true ]; then
  if [ "$VERIFY_ONLY" = true ]; then
    echo "ERROR: Video assets are missing and --verify-only was set." >&2
    exit 1
  fi
  echo "[1/4] Generating deterministic scenario video assets..."
  python3 scripts/generate_demo_videos.py
else
  echo "[1/4] Scenario video assets already present."
fi

# 3. Checksum verification
echo "[2/4] Verifying SHA256 checksums of video assets..."
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c "$CHECKSUMS_FILE"
else
  shasum -a 256 -c "$CHECKSUMS_FILE"
fi
echo "  -> Checksum verification passed."

# 4. Check Public Datasets Status and Print Instructions
echo "[3/4] Inspecting public datasets in data/datasets/..."

check_dataset() {
  local name="$1"
  local path="$2"
  local license="$3"
  local url="$4"
  local instructions="$5"

  echo "----------------------------------------------------------"
  echo "Dataset: $name ($license)"
  if [ -d "$path" ] && [ "$(ls -A "$path" 2>/dev/null)" ]; then
    echo "  Status: PRESENT ($path)"
  else
    echo "  Status: NOT PRESENT ($path)"
    echo "  Download Source: $url"
    echo "  Instructions:"
    printf "%s\n" "$instructions"
  fi
}

check_dataset \
  "AIDER (Aerial Image Dataset for Emergency Response)" \
  "data/datasets/aider" \
  "Research / Academic Use (KIOS CoE)" \
  "https://doi.org/10.5281/zenodo.3888300" \
  "    1. Download AIDER.zip from Zenodo (DOI: 10.5281/zenodo.3888300).
    2. Extract images into data/datasets/aider with class folders:
       data/datasets/aider/{fire,flood,collapsed_building,traffic_accident,normal}/*.jpg
    3. Train hazard classifier:
       python inference-service/train_hazard_classifier.py"

check_dataset \
  "SARD (Search And Rescue Dataset - Drone)" \
  "data/datasets/sard" \
  "CC-BY-4.0" \
  "https://github.com/c2a-lab / Zenodo" \
  "    1. Download SARD aerial human search and rescue dataset.
    2. Place images and annotations under data/datasets/sard/
       (images/train, images/val, labels/train, labels/val or instances_*.json).
    3. Train person detector:
       python inference-service/train_person_model.py"

check_dataset \
  "FLIR ADAS Thermal Dataset" \
  "data/datasets/flir_thermal" \
  "Teledyne FLIR Research License" \
  "https://www.flir.com/oem/adas/adas-dataset-form/" \
  "    1. Register at FLIR ADAS portal (free academic/research access).
    2. Download thermal dataset archive (FLIR_ADAS_v2 or v1).
    3. Unpack into data/datasets/flir_thermal/ with thermal_annotations.json
    4. Train thermal detector:
       python inference-service/train_thermal_model.py"

check_dataset \
  "c2a (Aerial Human Detection)" \
  "data/datasets/c2a" \
  "Open Dataset / Academic" \
  "https://github.com/c2a-lab" \
  "    1. Download c2a dataset images and labels.
    2. Place under data/datasets/c2a/
    3. Used jointly with SARD by train_person_model.py."

# 5. Check & Verify ONNX Model Weights against models/registry.json
echo "----------------------------------------------------------"
echo "[4/4] Verifying ONNX model weights against models/registry.json..."

REGISTRY="models/registry.json"
if [ ! -f "$REGISTRY" ]; then
  echo "ERROR: Model registry $REGISTRY not found." >&2
  exit 1
fi

python3 -c '
import hashlib
import json
import os
import sys

with open("models/registry.json") as f:
    reg = json.load(f)

models = reg.get("models", {})
all_ok = True

for name, entry in models.items():
    for k, sha_k in [("onnx_path", "sha256"), ("int8_onnx_path", "int8_sha256")]:
        p = entry.get(k)
        expected = entry.get(sha_k)
        if not p or not expected:
            continue
        if not os.path.exists(p):
            print(f"  FAIL: Missing model file: {p}", file=sys.stderr)
            all_ok = False
            continue
        h = hashlib.sha256()
        with open(p, "rb") as mf:
            while chunk := mf.read(65536):
                h.update(chunk)
        actual = h.hexdigest()
        if actual != expected:
            print(f"  FAIL: Checksum mismatch for {p}!\n    Expected: {expected}\n    Actual:   {actual}", file=sys.stderr)
            all_ok = False
        else:
            print(f"  VERIFIED: {p} (SHA256: {actual[:12]}...)")

if not all_ok:
    sys.exit(1)
print("All registered ONNX models verified against models/registry.json.")
'

# Maintain backward-compatible links in data/models/ for legacy paths
mkdir -p data/models
[ -f models/person_yolov8n/person_yolov8n.onnx ] && cp -f models/person_yolov8n/person_yolov8n.onnx data/models/person_yolov8.onnx 2>/dev/null || true
[ -f models/thermal_yolov8n/thermal_yolov8n.onnx ] && cp -f models/thermal_yolov8n/thermal_yolov8n.onnx data/models/thermal_yolov8.onnx 2>/dev/null || true
[ -f models/hazard_classifier_resnet18/hazard_classifier_resnet18.onnx ] && cp -f models/hazard_classifier_resnet18/hazard_classifier_resnet18.onnx data/models/hazard_classifier.onnx 2>/dev/null || true

echo "=========================================================="
echo "Asset fetch and verification complete."
echo "All assets and models verified against registry."
echo "=========================================================="
