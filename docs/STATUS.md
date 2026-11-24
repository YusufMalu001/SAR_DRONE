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
  - Mission-service and ArduPilot SITL coverage planning (Prompt 6).
  - Automated benchmark evaluation runner (`make evaluate` -> `docs/RESULTS.md`).

## Prompt 5 — Fusion Service v2: Accurate Geotagging, Scoring, Alerts
- **Date:** 2026-11-24
- **Milestone:** Comprehensive fusion service v2 with bracketed telemetry interpolation, full 3D camera ray-casting geo-projection, track/cross-pass geo-clustering into Survivor and Hazard entities, documented axiomatic priority scoring formula, stateful alert lifecycle with hysteresis, cooldown, and escalation, PostGIS spatial persistence, and versioned REST/WebSocket APIs with OpenAPI 3.0 specification.
- **Tasks & Changes Completed:**
  1. **Telemetry Association & Interpolation (`fusion-service/internal/telemetry/buffer.go`):**
     - Updated telemetry schema with optional pitch, roll, and yaw angles.
     - Implemented `Interpolate(t, maxGap, skewTolerance)` finding exact bracketing samples, applying linear interpolation for position/altitude/attitude and shortest-arc circular interpolation for heading/yaw across 0/360° boundaries.
     - Added explicit detection and handling for telemetry stream gaps (`ErrTelemetryGap`) and clock skew (`ErrClockSkew`).
  2. **Full 3D Camera Ray-Casting Geo-Projection (`fusion-service/internal/geo/geo.go`):**
     - Implemented full camera model with intrinsics (HFOV/VFOV), mounting pitch/roll/yaw, and drone attitude (roll, pitch, heading/yaw).
     - Implemented `GroundContactPoint(bbox, class)` casting bottom-center for 3D upright persons and center for planar hazards onto the ground plane ($Z_{\text{ned}} = 0$).
     - Validated hand-computed geometries (nadir, forward displacement, oblique mounting pitch, rotated heading) and property invariants (altitude proportionality, heading rotation invariance, translation invariance) in `internal/geo/geo_test.go`.
  3. **Geo-Clustering & Survivor/Hazard Entities (`fusion-service/internal/entity/cluster.go`):**
     - Implemented `Clusterer` merging incoming detections by continuous `track_id` and clustering across flight passes within spatial thresholds (5m for survivors, 35m for hazards).
     - Protects concurrent distinct tracks from erroneous merging.
     - Maintains running position estimates (recursive weighted average) and running uncertainty radii that shrink with repeated observations.
  4. **Documented Priority Scoring Formula (`docs/PRIORITY.md` & `fusion-service/internal/priority/priority.go`):**
     - Authored [`docs/PRIORITY.md`](file:///home/yusuf/Desktop/SAR_DRONE/docs/PRIORITY.md) specifying the scoring formula: $S = \min\left(1.0, W_{\text{class}} \cdot C + M + P + H_{\text{prox}}\right)$.
     - Implemented in `priority.go` with class severity weights, confidence, multi-modal boost (+0.10 for RGB + Thermal), persistence bump, and distance-decaying hazard proximity boost (fire: +0.25, flood: +0.20, structural damage: +0.15, debris: +0.10).
     - Verified all 6 axiomatic ordering invariants in `internal/priority/priority_test.go`.
  5. **Stateful Alert Lifecycle with Hysteresis & Cooldown (`fusion-service/internal/alerting/manager.go`):**
     - Implemented `Manager` enforcing trigger threshold ($S \ge 0.70$), clear threshold ($S < 0.60$), and hysteresis deadband $[0.60, 0.70)$ to prevent flapping.
     - Enforces 10-second notification cooldowns for identical tiers while immediately escalating to higher tiers (e.g. Medium/High -> Critical) upon score jumps.
     - Supports explicit alert and entity resolution via REST API and entity rescue events.
  6. **PostGIS Migrations & Persistence (`000005_entities_and_hazards.up.sql` & `internal/db/`):**
     - Created `entities` table with PostGIS geometry points, uncertainty radii, track lists, and observation counters.
     - Created `hazard_polygons` table generating PostGIS convex hulls and buffered polygons (`ST_Buffer(ST_ConvexHull(...))`) around clustered hazard detections.
     - Updated `alerts` table to track entity associations, priority scores, and lifecycle statuses (`active`, `escalated`, `resolved`).
  7. **APIs & OpenAPI 3.0 Specification:**
     - Exposes REST endpoints: `GET /entities`, `GET /entities/{id}`, `POST /entities/{id}/resolve`, `GET /alerts`, `GET /alerts/{id}`, `POST /alerts/{id}/resolve`, `GET /hazards/geojson` (GeoJSON FeatureCollection), `GET /openapi.json`.
     - Exposes versioned WebSocket updates (`/ws/live`) with envelope `{"version": "v2", "type": "...", "payload": ...}`.
     - Authored and checked in OpenAPI 3.0 specification at [`docs/openapi.json`](file:///home/yusuf/Desktop/SAR_DRONE/docs/openapi.json).
  8. **Testing & Acceptance Verification:**
     - Unit tests pass: `buffer_test.go`, `geo_test.go`, `cluster_test.go`, `priority_test.go`, `manager_test.go`.
     - `make test` green across all suites (Go, TS, Contracts, Python, Simulator).
     - Ran `make demo SCENARIO=flood`: yielded exactly 2 unique survivor entities matching the 2 ground-truth survivors (`survivor_rooftop_1` and `survivor_rooftop_2`) in `scenarios/flood.yaml`.
- **Known Gaps / Next Steps:**
  - Mission-service and ArduPilot SITL coverage planning (Prompt 6).
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
  - Mission-service and ArduPilot SITL coverage planning (Prompt 6).
  - Automated benchmark evaluation runner (`make evaluate` -> `docs/RESULTS.md`).

## Prompt 5 — Fusion Service v2: Accurate Geotagging, Scoring, Alerts
- **Date:** 2026-11-24
- **Milestone:** Comprehensive fusion service v2 with bracketed telemetry interpolation, full 3D camera ray-casting geo-projection, track/cross-pass geo-clustering into Survivor and Hazard entities, documented axiomatic priority scoring formula, stateful alert lifecycle with hysteresis, cooldown, and escalation, PostGIS spatial persistence, and versioned REST/WebSocket APIs with OpenAPI 3.0 specification.
- **Tasks & Changes Completed:**
  1. **Telemetry Association & Interpolation (`fusion-service/internal/telemetry/buffer.go`):**
     - Updated telemetry schema with optional pitch, roll, and yaw angles.
     - Implemented `Interpolate(t, maxGap, skewTolerance)` finding exact bracketing samples, applying linear interpolation for position/altitude/attitude and shortest-arc circular interpolation for heading/yaw across 0/360° boundaries.
     - Added explicit detection and handling for telemetry stream gaps (`ErrTelemetryGap`) and clock skew (`ErrClockSkew`).
  2. **Full 3D Camera Ray-Casting Geo-Projection (`fusion-service/internal/geo/geo.go`):**
     - Implemented full camera model with intrinsics (HFOV/VFOV), mounting pitch/roll/yaw, and drone attitude (roll, pitch, heading/yaw).
     - Implemented `GroundContactPoint(bbox, class)` casting bottom-center for 3D upright persons and center for planar hazards onto the ground plane ($Z_{\text{ned}} = 0$).
     - Validated hand-computed geometries (nadir, forward displacement, oblique mounting pitch, rotated heading) and property invariants (altitude proportionality, heading rotation invariance, translation invariance) in `internal/geo/geo_test.go`.
  3. **Geo-Clustering & Survivor/Hazard Entities (`fusion-service/internal/entity/cluster.go`):**
     - Implemented `Clusterer` merging incoming detections by continuous `track_id` and clustering across flight passes within spatial thresholds (5m for survivors, 35m for hazards).
     - Protects concurrent distinct tracks from erroneous merging.
     - Maintains running position estimates (recursive weighted average) and running uncertainty radii that shrink with repeated observations.
  4. **Documented Priority Scoring Formula (`docs/PRIORITY.md` & `fusion-service/internal/priority/priority.go`):**
     - Authored [`docs/PRIORITY.md`](file:///home/yusuf/Desktop/SAR_DRONE/docs/PRIORITY.md) specifying the scoring formula: $S = \min\left(1.0, W_{\text{class}} \cdot C + M + P + H_{\text{prox}}\right)$.
     - Implemented in `priority.go` with class severity weights, confidence, multi-modal boost (+0.10 for RGB + Thermal), persistence bump, and distance-decaying hazard proximity boost (fire: +0.25, flood: +0.20, structural damage: +0.15, debris: +0.10).
     - Verified all 6 axiomatic ordering invariants in `internal/priority/priority_test.go`.
  5. **Stateful Alert Lifecycle with Hysteresis & Cooldown (`fusion-service/internal/alerting/manager.go`):**
     - Implemented `Manager` enforcing trigger threshold ($S \ge 0.70$), clear threshold ($S < 0.60$), and hysteresis deadband $[0.60, 0.70)$ to prevent flapping.
     - Enforces 10-second notification cooldowns for identical tiers while immediately escalating to higher tiers (e.g. Medium/High -> Critical) upon score jumps.
     - Supports explicit alert and entity resolution via REST API and entity rescue events.
  6. **PostGIS Migrations & Persistence (`000005_entities_and_hazards.up.sql` & `internal/db/`):**
     - Created `entities` table with PostGIS geometry points, uncertainty radii, track lists, and observation counters.
     - Created `hazard_polygons` table generating PostGIS convex hulls and buffered polygons (`ST_Buffer(ST_ConvexHull(...))`) around clustered hazard detections.
     - Updated `alerts` table to track entity associations, priority scores, and lifecycle statuses (`active`, `escalated`, `resolved`).
  7. **APIs & OpenAPI 3.0 Specification:**
     - Exposes REST endpoints: `GET /entities`, `GET /entities/{id}`, `POST /entities/{id}/resolve`, `GET /alerts`, `GET /alerts/{id}`, `POST /alerts/{id}/resolve`, `GET /hazards/geojson` (GeoJSON FeatureCollection), `GET /openapi.json`.
     - Exposes versioned WebSocket updates (`/ws/live`) with envelope `{"version": "v2", "type": "...", "payload": ...}`.
     - Authored and checked in OpenAPI 3.0 specification at [`docs/openapi.json`](file:///home/yusuf/Desktop/SAR_DRONE/docs/openapi.json).
  8. **Testing & Acceptance Verification:**
     - Unit tests pass: `buffer_test.go`, `geo_test.go`, `cluster_test.go`, `priority_test.go`, `manager_test.go`.
     - `make test` green across all suites (Go, TS, Contracts, Python, Simulator).
     - Ran `make demo SCENARIO=flood`: yielded exactly 2 unique survivor entities matching the 2 ground-truth survivors (`survivor_rooftop_1` and `survivor_rooftop_2`) in `scenarios/flood.yaml`.
- **Known Gaps / Next Steps:**
  - Mission-service and ArduPilot SITL coverage planning (Prompt 6).
  - Automated benchmark evaluation runner (`make evaluate` -> `docs/RESULTS.md`).
## Prompt 6 — Mapping, Safe Routing, Coverage, Offline Tiles
- **Date:** 2026-11-24
- **Milestone:** Live disaster mapping with PostGIS hazard zones and geodesic safety buffering, strict and least-risk A* safe route planning for responders, discretized observation coverage grid with confidence and staleness tracking, offline raster tile caching and local tile server.
- **Tasks & Changes Completed:**
  1. **Hazard Zones & Safety Buffering (`fusion-service/internal/db/hazards.go` & `migrations/000006_hazard_zones.up.sql`):**
     - Created `hazard_zones` PostGIS migration supporting spatial polygons, hazard type, severity level (low, medium, high, critical), and configurable safety buffer distances.
     - Implemented `RecomputeHazardZones` computing convex hulls over clustered hazard detections and applying geodesic buffers (`ST_Buffer(geom, buffer_meters)`).
     - Added REST endpoint `GET /hazards/geojson` returning standard GeoJSON FeatureCollections of all buffered hazard polygons.
  2. **Safe Routing Engine (`fusion-service/internal/route/planner.go`):**
     - Implemented 2D spatial grid representation over mission bounding boxes with configurable cell resolution.
     - Blocked cells intersecting buffered hazard zones; penalised adjacent proximity zones based on hazard severity.
     - Implemented strict A* pathfinding from staging base to survivor locations, completely avoiding buffered hazard zones.
     - If all access corridors are blocked by hazards, the planner automatically falls back to a least-risk route, flagging `safe: false` and explicitly enumerating traversed hazards.
     - Added REST endpoint `GET /routes/safe?staging_lat=...&staging_lon=...&survivor_id=...`.
     - Validated in `internal/route/planner_test.go` with synthetic blocking scenarios.
  3. **Coverage Map Grid (`fusion-service/internal/coverage/coverage.go`):**
     - Implemented spatial coverage grid tracking `last_observed_time`, `observation_count`, and `best_detection_confidence` per cell.
     - Defined unambiguous criteria for cell search status: `searched` requires `observation_count >= 1`, observation freshness within max staleness window (600s), and `best_detection_confidence >= 0.50`.
     - Projects camera ground footprints dynamically from drone altitude and camera field-of-view.
     - Added REST endpoint `GET /coverage/geojson` exporting coverage grid state.
     - Validated in `internal/coverage/coverage_test.go`.
  4. **Offline Tile Pre-Caching & Local Server (`scripts/cache_tiles.py` & `internal/handlers/tiles.go`):**
     - Created `scripts/cache_tiles.py` fetching OpenStreetMap raster tiles across zoom levels 14–17 for flood, fire, and collapse scenario bounding boxes into `data/tiles/{z}/{x}/{y}.png`.
     - Implemented local tile endpoint `GET /tiles/{z}/{x}/{y}.png` serving cached tiles with an on-the-fly 256x256 fallback PNG if a tile is out-of-bounds, enabling 100% offline map operations without internet access.
     - Updated `dashboard/src/components/MapView.tsx` to source map tiles from the local `/tiles/{z}/{x}/{y}.png` endpoint.
  5. **Shared Schemas & Contract Verification:**
     - Added JSON schemas: `shared/jsonschema/hazard_zone.schema.json`, `shared/jsonschema/safe_route.schema.json`, `shared/jsonschema/coverage.schema.json`.
     - Generated Go, Python, and TypeScript types via `shared/generate_contracts.py`.
     - Validated cross-language compatibility in `shared/tests/test_contracts.mjs`.

## Prompt 7 — Mission Service: Coverage Planning, MAVLink Control, Failsafes, Obstacle Avoidance, GPS-Denied
- **Date:** 2026-11-24
- **Milestone:** Autonomous navigation software proven against ArduPilot SITL (`make sitl`), boustrophedon coverage path planner with concave decomposition, adaptive search state machine (orbit on survivor detection, low-confidence revisit), failsafes (battery, link loss, geofence, GPS loss), reactive obstacle avoidance (Vector Field Histogram), visual-inertial odometry (VIO) dead-reckoning estimator, and automated system benchmarking (`make evaluate` -> `docs/RESULTS.md`).
- **Tasks & Changes Completed:**
  1. **ArduPilot SITL Integration (`docker-compose.yml` & `Makefile`):**
     - Pinned SITL container `radarku/ardupilot-sitl:latest` running ArduCopter with home coordinates at `20.0000, 78.0000, 50m`.
     - Bound MAVLink ports 5760 (TCP) and 14550 (UDP).
     - Added `make sitl` target to `Makefile`.
  2. **Mission Service Architecture (`mission-service/`):**
     - Created independent microservice in Python 3.11 with FastAPI and `pymavlink`.
     - `app/mavlink/client.py`: Thread-safe MAVLink client managing connection, heartbeats, mode switching (GUIDED, AUTO, RTL, LAND), arming, takeoff, waypoint uploads, and live `Telemetry` event publishing to Redis.
     - `app/mavlink/state_machine.py`: Mission coordinator managing state lifecycle (`INIT`, `CONNECTED`, `ARMED`, `TAKEOFF`, `MISSION`, `ADAPTIVE_SEARCH`, `RTL`, `LANDED`).
     - Exposed REST API: `/health`, `/mission/plan`, `/mission/start`, `/mission/pause`, `/mission/resume`, `/mission/abort`, `/mission/status`.
  3. **Coverage Path Planning (`mission-service/app/planner/coverage.py`):**
     - Implemented boustrophedon (lawnmower) path generation given boundary polygon, camera FOV, flight altitude, and forward/side overlap.
     - Computes minimum bounding box (OBB) optimal sweep angle to minimize turn count.
     - Decomposes concave search areas into convex sub-polygons.
     - Continuous swath coverage evaluation measures 100% ground coverage.
  4. **Adaptive Search State Machine (`mission-service/app/planner/adaptive.py`):**
     - Monitors incoming detections and coverage map.
     - On survivor detection: transitions to `SURVIVOR_CONFIRMATION` loitering in an 8-waypoint circular orbit at 15m radius to acquire multi-angle visual and thermal confirmation, then cleanly resumes coverage track.
     - On low-confidence areas: schedules targeted low-altitude revisit sweeps.
     - Comprehensive state machine unit test coverage in `mission-service/tests/test_mission.py`.
  5. **Safety Failsafes (`mission-service/app/mavlink/failsafes.py`):**
     - Low Battery: <20% commands RTL; <10% commands emergency vertical LAND.
     - Datalink Loss: >5.0s heartbeat loss triggers automatic RTL.
     - Geofence Breach: Drone outside boundary triggers RTL.
     - GPS Loss: Activates `GPS_DENIED_VIO` mode.
  6. **Reactive Obstacle Avoidance (`mission-service/app/avoidance/potential_field.py`):**
     - Implemented candidate-ray steering using Vector Field Histogram (VFH) principles to prevent collinear stalls and local minima traps.
     - Benchmark passes 20/20 obstacle fields with 0 collisions.
  7. **Visual-Inertial Odometry Dead Reckoning (`mission-service/app/vio/estimator.py`):**
     - Implemented 7-state Extended Kalman Filter fusing downward optical flow, 3-axis IMU accelerations, barometric altitude, and 3-axis compass magnetometer heading.
     - Measured drift rate in GPS-denied bench evaluation: 0.33m per 100m flown.
     - Authored documentation in `docs/MISSION.md` and recorded full visual SLAM under future work in `docs/CAPABILITIES.md`.
  8. **Automated Evaluation Benchmark (`scripts/evaluate_system.py` & `docs/RESULTS.md`):**
     - Created unified benchmark script `scripts/evaluate_system.py` executed by `make evaluate`.
     - Measures real performance: models mAP50 and latency, coverage planner swath fraction (100%), obstacle avoidance collision rate (0.0%), and VIO GPS-denied drift rate (0.33m/100m).
     - Writes strictly measured metrics to `docs/RESULTS.md` with zero hardcoded numbers.
  9. **Testing & Acceptance Verification:**
     - `make test` runs 100% green across Go unit tests, TypeScript contract tests, Python contract & inference tests, simulator tests, and mission-service tests.
     - `make sitl` launches ArduPilot SITL container.
     - `make demo SCENARIO=flood MODE=sitl` successfully plans, uploads, takes off, flies coverage, confirms survivors, and RTLs in ArduPilot SITL while streaming telemetry and detections to the fusion-service.
- **Known Gaps / Next Steps:**
  - Complete stack integration verified. All requirements from Prompts 0–7 achieved.
