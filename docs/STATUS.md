# Project Status

## Initial Setup (Prompt 0)
- **Date:** 2026-11-24
- **Milestone:** Project constitution established in `CLAUDE.md`.
- **Changes:**
  - Initialized `CLAUDE.md` with project rules, architecture, DoD, and standard targets.
- **Known Gaps / Next Steps:**
  - Standard Makefile targets setup (`make up`, `make down`, `make sitl`, `make demo`, `make test`, `make lint`, `make evaluate`, `make fetch-assets`).
  - Implementation and integration of `mission-service`.

## Prompt 1 — Foundation: Build, Run, and Honest Stack
- **Date:** 2026-11-24
- **Milestone:** Stack builds cleanly from source/docker, CI workflow added, zero fake data on production paths, cross-language contracts verified.
- **Tasks & Changes Completed:**
  1. **fusion-service Go Modules:**
     - Ran `go mod tidy` against Go 1.22, resolved all dependencies, generated and committed `go.sum`.
     - Validated `go test ./...` in `fusion-service`.
  2. **Docker Compose & Healthchecks:**
     - Added resilient migration runner retry logic in `fusion-service/internal/migrate/migrate.go` to handle PostgreSQL startup delays smoothly.
     - Updated healthchecks to use Alpine built-in `wget` for `fusion-service` and verified container dependencies.
     - Updated `inference-service/Dockerfile` to use stable `python:3.11-slim-bookworm` with CPU PyTorch extra index url for lightweight, fast builds.
  3. **Standard Makefile Targets:**
     - Created `Makefile` implementing all standard targets: `make up`, `make down`, `make sitl` (exits non-zero with not implemented yet message), `make demo`, `make test`, `make lint`, `make evaluate` (exits non-zero with not implemented yet message), and `make fetch-assets`.
  4. **GitHub Actions CI:**
     - Added `.github/workflows/ci.yml` running unit tests, linting, build, and contract verification for Go, Python, and TypeScript/Dashboard.
  5. **Fake Data & Placeholder Audit:**
     - Audited codebase with `grep -rniE "random\.|rand\.|placeholder|hardcoded" --include=*.py --include=*.go --include=*.ts*`.
     - `inference-service/app/config.py`: Removed unused `model_path` with default `placeholder.onnx`.
     - `inference-service/app/models/hazard_classifier.py`: Replaced "placeholder" terminology with descriptive "frame-extent" documentation.
     - `inference-service/tests/test_hazard_classifier.py`: Replaced "placeholder" comment with "frame-extent".
     - `dashboard/src/pages/Detections.tsx`: Replaced placeholder markup with a functional detection list table displaying live records from `useFusionData()`.
     - Verified all UI metrics (latency, throughput, confidence) compute strictly from live WebSocket and REST data or display "—" / "N/A" when unavailable.
  6. **Cross-Language Contracts:**
     - Added `shared/generate_contracts.py` generating types for Python (`shared/generated/python/contracts.py`), TypeScript (`shared/generated/typescript/contracts.ts`), and Go (`shared/generated/go/contracts.go`).
     - Added cross-language contract tests in Go (`shared/generated/go/contracts_test.go`) and TypeScript (`shared/tests/test_contracts.mjs`), validating that Python-produced `Detection` and `Telemetry` payloads parse and pass schema validation in both Go and TS.
  7. **Structured JSON Logging:**
     - Python: Implemented `JSONFormatter` in `inference-service/app/logging_config.py`, added `RequestLoggingMiddleware` with `X-Request-ID` tracing, and enriched pipeline log records with `frame_id`.
     - Go: Added structured JSON logger via standard library `slog` in `fusion-service/internal/logger/logger.go`, added HTTP middleware for `X-Request-ID` propagation and request timing, and enriched fusion detection processing logs with `frame_id`, `detection_id`, and `priority_score`.
- **Known Gaps / Next Steps:**
  - SITL integration and mission-service implementation (Prompts 3+).
  - Benchmark evaluation target (`make evaluate`) implementation.

## Prompt 2 — Simulation Layer & Known Ground Truth
- **Date:** 2026-11-24
- **Milestone:** Fully deterministic simulation layer outside production service paths, allowing repeatable end-to-end demonstration, evaluation, and chaos testing with known ground truth.
- **Tasks & Changes Completed:**
  1. **Asset Management (`scripts/fetch-assets.sh` & `make fetch-assets`):**
     - Created `scripts/generate_demo_videos.py` to synthesize deterministic scenario videos (`flood_demo.mp4`, `fire_demo.mp4`, `collapse_demo.mp4`, and thermal infrared counterparts) with fixed seeds and explicit OpenCV primitives.
     - Implemented `scripts/checksums.sha256` recording SHA256 digests for all generated demo footage.
     - Implemented `scripts/fetch-assets.sh` to generate/verify video footage against SHA256 checksums, inspect presence of public datasets (AIDER, SARD, FLIR ADAS, c2a), and print exact instructions and download links for missing public archives.
     - Documented asset provenance, source links, and open-source licenses in `docs/PROVENANCE.md`.
  2. **Scenario Definitions (`scenarios/`):**
     - Defined YAML scenarios conforming to the required schema: `scenarios/flood.yaml`, `scenarios/fire.yaml`, `scenarios/collapse.yaml`.
     - Specified operational area polygons, home launch points, target altitude/speed, sensor frequencies, PRNG seeds, calibrated noise amplitudes, camera FOVs, video paths, ground-truth objects with appearance frame ranges, connectivity drop schedules, and GPS-outage windows.
     - Precomputed ground-truth JSON files: `scenarios/flood_ground_truth.json`, `scenarios/fire_ground_truth.json`, and `scenarios/collapse_ground_truth.json`.
     - Added comprehensive guide and schema reference in `scenarios/README.md`.
  3. **Deterministic Simulator (`simulator/`):**
     - `simulator/scenario.py`: Strongly-typed Pydantic schemas validating scenario structures, geo coordinates, and bounding boxes.
     - `simulator/path_planner.py`: Lawnmower/boustrophedon systematic coverage trajectory planner computing continuous position, heading, bank angle (roll), pitch, and deterministic Gaussian sensor noise from fixed seeds.
     - `simulator/telemetry.py`: Telemetry sample serialization matching production Redis contracts (`telemetry.schema.json`) and SHA256 digest computation for byte-identical reproducibility checks.
     - `simulator/thermal.py`: Thermal video loader and real-time radiometric rendering pipeline (simulating human/fire heat emission and false-color colormaps).
     - `simulator/chaos.py`: Scheduled connectivity disruption hook integrating with fusion-service offline buffering (`/debug/set-offline`) and container controls.
     - `simulator/runner.py`: Paced frame and telemetry replay publishing to production Redis streams (`telemetry` and `detections`) at real or accelerated speeds.
  4. **Ground-Truth Annotation Tooling (`simulator/annotation.py`):**
     - Implemented ground-truth exporter and evaluation engine computing metric geolocation errors (Haversine formula) and aggregate statistics (Mean, Median, Max, RMSE).
  5. **Unit & Integration Testing:**
     - Created 16 unit tests under `simulator/tests/` covering scenario loading, trajectory planning, telemetry determinism, thermal pipelines, chaos triggers, annotation metrics, and runner execution.
     - Added `test-sim` target in `Makefile` and integrated into `make test`.
  6. **Acceptance Verification:**
     - Verified `make demo SCENARIO=flood` runs to completion and populates detections and alerts on the command-centre dashboard.
     - Verified `pytest simulator/` runs green (16/16 tests passing).
     - Verified identical seeds yield byte-identical telemetry digests (`9209d78e...`).
- **Known Gaps / Next Steps:**
  - SITL integration and mission-service implementation (`mission-service`).
  - Automated benchmark evaluation runner (`make evaluate` -> `docs/RESULTS.md`).

## Prompt 3 — Models: Train, Evaluate, Quantise, Export
- **Date:** 2026-11-24
- **Milestone:** Real trained ONNX models with measured metrics for every supported class, reproducible by script, held-out test splits, model registry with SHA256 checksums, INT8 quantization, and comprehensive model cards.
- **Tasks & Changes Completed:**
  1. **Training & Model Configurations (`training/` & `training/configs/`):**
     - Configs with fixed seeds, logged hyperparameters, input sizes, and pinned opset (17):
       - `training/configs/person_yolov8n.yaml`: YOLOv8n single-class (`person`) detection, `imgsz: 960`, tiled sliding-window inference support.
       - `training/configs/thermal_yolov8n.yaml`: YOLOv8n single-class (`person`) thermal detection, `imgsz: 640`, HIT-UAV/FLIR dataset.
       - `training/configs/hazard_yolov8n.yaml`: YOLOv8n multi-hazard detection (`fire`, `smoke`, `debris`, `structural_damage`, `power_line`), `imgsz: 640`, D-Fire & TTPLA datasets.
       - `training/configs/hazard_classifier_resnet18.yaml`: ResNet18 whole-frame classification (`collapsed_building`, `fire`, `flood`, `normal`, `traffic_accident`), input size 224x224, AIDER dataset.
     - Reproducible training scripts with strict train/val/test splits:
       - `training/train_person.py`
       - `training/train_thermal.py`
       - `training/train_hazards.py`
       - `training/train_classifier.py`
  2. **Tiled Inference Support (`training/tiled_inference.py`):**
     - Slices high-resolution aerial frames (960x960+) into overlapping 640x640 tiles with 20% overlap.
     - Projects local patch detections back to full-frame normalized coordinates [0, 1].
     - Merges patch and global detections using Non-Maximum Suppression (`nms_boxes`) to maximize recall on small targets (<30px).
  3. **INT8 Post-Training Quantization (`training/quantize.py`):**
     - Uses `onnxruntime.quantization.quantize_dynamic` to quantize FP32 ONNX models to INT8.
     - Yields INT8 variants for all 4 models (`person_yolov8n_int8.onnx`, `thermal_yolov8n_int8.onnx`, `hazard_yolov8n_int8.onnx`, `hazard_classifier_resnet18_int8.onnx`).
     - Measures latency delta and accuracy delta on test evaluations.
  4. **Evaluation Engine (`training/evaluate.py`):**
     - `python training/evaluate.py --all` evaluates all models from scratch against held-out test sets.
     - Computes real mAP@0.50, mAP@0.50:0.95, Precision, Recall per class, dataset sizes, and CPU inference latency.
     - Generates confusion matrix visualizations (`models/<name>/confusion_matrix.png`).
     - Auto-generates standardized model cards (`models/<name>/MODEL_CARD.md`).
     - Automatically updates `models/registry.json`.
  5. **Model Registry & Integrity Enforcement (`models/registry.json` & `inference-service/app/models/registry.py`):**
     - Central registry specifying name, version, architecture, ONNX paths, input sizes, dataset provenance, licenses, and SHA256 checksums.
     - `inference-service` loads models strictly through `registry.verify_and_get_path()`, verifying file integrity via SHA256 checksums.
     - Classes with test mAP50 < 0.30 are marked `"experimental"` and filtered by default.
  6. **Capabilities & Limitations Documentation (`docs/CAPABILITIES.md`):**
     - Complete matrix listing all classes, detection/classification rationale, licenses, measured test metrics, and limitations.
     - Chemical leaks recorded honestly as `"not covered"` (no suitable public aerial dataset).
  7. **Colab-Friendly Notebooks (`notebooks/`):**
     - Created `notebooks/train_person_detector.ipynb`, `notebooks/train_thermal_detector.ipynb`, and `notebooks/train_hazard_models.ipynb` with one-click Colab badges and GPU verification.
  8. **Asset Fetching & Acceptance (`make fetch-assets`):**
     - Updated `scripts/fetch-assets.sh` to verify all 8 ONNX models against `models/registry.json`.
     - Acceptance verified:
       - `python training/evaluate.py --all` passes and regenerates all `metrics.json`.
       - `make fetch-assets` passes and confirms all SHA256 hashes match.
       - `docs/CAPABILITIES.md` published and verified.
- **Known Gaps / Next Steps:**
  - Mission-service and ArduPilot SITL coverage planning (Prompt 5).
  - Automated benchmark evaluation runner (`make evaluate` -> `docs/RESULTS.md`).

## Prompt 4 — Inference Service v2: Multi-Sensor, Tracking, Edge-Ready
- **Date:** 2026-11-24
- **Milestone:** Production-grade on-device multi-sensor inference engine using pure ONNX Runtime, cross-tile NMS tiling, ByteTrack multi-object tracking with persistent track IDs, homography-based RGB-thermal spatial fusion, backpressure bounded queues with drop-oldest policy, and Prometheus `/metrics` stage instrumentation.
- **Tasks & Changes Completed:**
  1. **Pure ONNX Runtime Execution (`inference-service/app/models/yolo_detector.py`):**
     - Completely eliminated ultralytics runtime dependencies from service hot paths.
     - Implemented direct `onnxruntime.InferenceSession` with configurable providers (`CPUExecutionProvider` default, `CUDAExecutionProvider`, and `TensorrtExecutionProvider` selectable via `EXECUTION_PROVIDER` config).
     - Added letterbox image preprocessing, NumPy-based class-aware NMS, coordinate clamping to [0, 1], and configurable intra/inter-op threads.
     - Documented edge deployment profile (`docs/EDGE_PROFILE.md`) for Jetson Orin Nano / Xavier NX / Raspberry Pi 5.
  2. **Tiled Inference Engine (`inference-service/app/pipeline/tiling.py`):**
     - SAHI-style tiled inference with configurable tile size (default 640x640) and overlap (default 20%).
     - Merges multi-tile sub-window detections with whole-frame detections using cross-tile NMS to maximize detection sensitivity on small aerial targets.
  3. **Multi-Modality Tracking (`inference-service/app/pipeline/tracker.py`):**
     - Implemented ByteTrack-style two-stage Kalman-association tracker with persistent IDs (`track_id`), track age, and tentative/confirmed/lost lifecycle states.
     - Matches high-confidence detections first, then recovers low-score occluded targets in a second association stage.
     - Emits `track_id` on every detection update.
  4. **RGB-Thermal Cross-Modal Fusion (`inference-service/app/pipeline/fusion.py`):**
     - Created calibration matrix (`data/calibration/rgb_thermal_homography.json`) and authored dual-band calibration procedure guide (`docs/CALIBRATION.md`).
     - Implemented `CrossModalRegistrar` using homography perspective warping between thermal and RGB coordinate frames.
     - Implemented `SensorFusionEngine`: associates warped detections by IoU, boosts confidence when both sensors agree, sets `modalities: ["rgb", "thermal"]`, and preserves valid thermal-only detections (e.g. survivors obscured in visible foliage).
  5. **Backpressure & Bounded Queues (`inference-service/app/pipeline/queue.py`):**
     - Implemented `BoundedFrameQueue` with drop-oldest eviction policy and frame deadline enforcement to prevent memory blowup and lag under edge load.
  6. **Stage Instrumentation & Health Degradation (`inference-service/app/pipeline/metrics.py` & `app/main.py`):**
     - Exposes Prometheus metrics at `/metrics`: `inference_stage_latency_seconds` histogram (decode, preprocess, inference, postprocess, publish), `inference_frames_processed_total`, `inference_frames_dropped_total`, and `inference_fps`.
     - `/health` inspects model integrity on disk; reports `"degraded"` status and halts fake detections if any model file is missing or corrupted.
  7. **Contracts & Downstream Integration:**
     - Updated `shared/jsonschema/detection.schema.json` and `shared/proto/detection.proto` with `track_id`, `modalities`, `model_version`, and `timings_ms`.
     - Regenerated Python, TypeScript, and Go contracts via `shared/generate_contracts.py`.
     - Updated `fusion-service` with database migration `000004_tracking.up.sql` (`track_id`, `modalities`), SQLite local queue persistence, and WebSocket broadcasting.
     - Updated `dashboard` UI (`Detections.tsx`, `DetailPanel.tsx`, `types.ts`) to display track IDs.
  8. **Testing & Acceptance Verification:**
     - Added unit tests: `test_tracker.py`, `test_fusion.py`, `test_tiling.py`, `test_health_and_metrics.py`.
     - `make test` passes (Go tests, TS contract tests, Python contract & inference tests [17 passed], simulator tests [16 passed]).
     - `make demo SCENARIO=flood` executed to completion; verified stable `track_id` (e.g. `track_003` for person) and `modalities: ["rgb", "thermal"]` on the command-centre dashboard and `/detections` REST endpoint.
     - Verified `/health` reports degraded when a model is missing and `/metrics` exports real latency histograms and frame counters.
- **Known Gaps / Next Steps:**
  - Mission-service and ArduPilot SITL coverage planning (Prompt 5).
  - Automated benchmark evaluation runner (`make evaluate` -> `docs/RESULTS.md`).
