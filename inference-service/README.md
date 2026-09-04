# inference-service

Python model-inference service. Runs three models over video frames and
publishes `Detection` events (see
[`../shared/jsonschema/detection.schema.json`](../shared/jsonschema/detection.schema.json))
to a Redis stream for `fusion-service` to consume, plus a synthetic
telemetry replay publishing `Telemetry` events on a second stream.

The FastAPI app (`app/main.py`) is just the health-check surface. The
detection pipeline itself runs as a separate process (see below), not
through an HTTP endpoint.

## Models

| model              | class(es)                          | architecture                | weights                              | trained by                  |
|--------------------|-------------------------------------|-------------------------------|----------------------------------------|------------------------------|
| `PersonDetector`   | person (rgb)                        | YOLOv8n (ONNX)                 | `data/models/person_yolov8.onnx`       | `train_person_model.py`     |
| `ThermalDetector`  | person (thermal)                    | YOLOv8n (ONNX)                 | `data/models/thermal_yolov8.onnx`      | `train_thermal_model.py`    |
| `HazardClassifier` | fire / flood / structural_damage / debris | ResNet18 or EfficientNet-B0 (ONNX) — whole-frame classifier | `data/models/hazard_classifier.onnx` | `train_hazard_classifier.py` |

`HazardClassifier` is a classifier, not a segmenter — it labels the whole
frame and every `Detection` it emits carries a full-frame bbox
(`{x:0,y:0,w:1,h:1}`) as a placeholder. See the `TODO(segmentation)` note in
`app/models/hazard_classifier.py` for where a DeepLabV3-style segmentation
model would replace it later to actually localize the hazard.

None of the three `.onnx` weights files are checked into git (see
`../data/.gitignore`). If a model's weights file is missing, the detector
raises `ModelNotFoundError` naming the training script to run — models are
loaded lazily on first `predict()` call, not at import time.

## Training

```bash
# Person detector — fine-tunes yolov8n.pt on c2a + SARD, exports to ONNX
python train_person_model.py --c2a-dir data/datasets/c2a --sard-dir data/datasets/sard

# Thermal hotspot detector — fine-tunes yolov8n.pt on FLIR ADAS (person only)
python train_thermal_model.py --data-dir data/datasets/flir_thermal

# Hazard classifier — fine-tunes resnet18 (or --arch efficientnet_b0) on AIDER
python train_hazard_classifier.py --data-dir data/datasets/aider
```

Each script accepts `--epochs`, `--imgsz`/`--input-size`, and `--output` to
override the default `data/models/*.onnx` destination — run `--help` for the
full list. The two YOLO scripts accept either COCO-format datasets
(auto-converted via `training/yolo_common.py`) or datasets already in YOLO
format.

## Running the pipeline

```bash
python -m app.pipeline.run_demo --fps 5 --max-frames 200
```

Starts `VideoIngestionPipeline` (reads `data/sample_footage/demo_flight.mp4`,
runs all three models per sampled frame, XADDs each `Detection` to the
`detections` Redis stream) and `TelemetryPublisher` (replays
`data/sample_footage/demo_telemetry.csv` onto the `telemetry` stream) on a
shared `PlaybackClock`, so entries on both streams land on the same
timeline and can be joined by `timestamp` downstream. Pass `--loop` to
replay continuously. See `data/sample_footage/README.md` — the demo video
itself isn't checked in; only the small telemetry CSV is.

## Benchmarking

```bash
python -m scripts.benchmark --iterations 50
```

Reports per-model mean/p50/p95 latency and FPS, plus an end-to-end
single-core pipeline FPS estimate (sequential person + thermal + hazard
inference per frame) — use this to demonstrate edge-feasible inference
speed. Falls back to synthetic frames if no video is present yet, and skips
any model whose weights haven't been trained yet rather than failing.

## Tests

```bash
pip install -r requirements-dev.txt
pytest
```

`tests/` feeds a synthetic sample image through each of the three models
(with the underlying inference call mocked, so no trained weights are
needed) and asserts the resulting `Detection` validates against
`shared/jsonschema/detection.schema.json`.

## Local setup (no Docker)

```bash
cd inference-service
python -m venv .venv
source .venv/bin/activate   # Windows: .venv\Scripts\activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8001
```

## Endpoints

- `GET /` — service info
- `GET /health` — `{ status, service, redis }`

## Config (env vars)

| var                        | default                                  |
|-----------------------------|--------------------------------------------|
| `HOST`                      | `0.0.0.0`                                   |
| `PORT`                      | `8001`                                      |
| `REDIS_URL`                 | `redis://localhost:6379/0`                  |
| `DETECTIONS_CHANNEL`        | `detections`                                |
| `TELEMETRY_CHANNEL`         | `telemetry`                                 |
| `STREAM_MAXLEN`             | `10000`                                     |
| `MODEL_PATH`                | `/data/models/placeholder.onnx`             |
| `PERSON_MODEL_PATH`         | `/data/models/person_yolov8.onnx`           |
| `THERMAL_MODEL_PATH`        | `/data/models/thermal_yolov8.onnx`          |
| `HAZARD_MODEL_PATH`         | `/data/models/hazard_classifier.onnx`       |
| `PERSON_CONF_THRESHOLD`     | `0.4`                                        |
| `THERMAL_CONF_THRESHOLD`    | `0.4`                                        |
| `HAZARD_CONF_THRESHOLD`     | `0.5`                                        |
| `IOU_THRESHOLD`             | `0.45`                                       |
| `VIDEO_PATH`                | `/data/sample_footage/demo_flight.mp4`      |
| `TELEMETRY_CSV_PATH`        | `/data/sample_footage/demo_telemetry.csv`   |
| `TARGET_FPS`                | `5`                                          |
| `TELEMETRY_SAMPLE_RATE_HZ`  | `5`                                          |
| `PLAYBACK_SPEED`            | `1.0`                                        |

The `/data/...` defaults assume the docker-compose `./data:/data` mount —
override these for non-Docker local runs, e.g. `VIDEO_PATH=../data/sample_footage/demo_flight.mp4`.
