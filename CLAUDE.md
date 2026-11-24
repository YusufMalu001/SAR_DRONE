# Project Constitution: sar-drone-prototype

## Project Overview
- **Project Name:** sar-drone-prototype
- **Purpose:** Software for an autonomous search-and-rescue drone
- **Context:** Smart India Hackathon 2026 (PS SIH26177, Team NIRIKSH, Solution "Khoj-o-Drone")
- **Problem Statement:** After disasters (floods, cyclones, earthquakes, landslides) responders need fast situational awareness. The system must, with on-device AI and without depending on network connectivity:
  - Detect survivors (RGB + thermal) and hazards (fire, smoke, flood, debris, damaged structures, landslide, exposed power lines)
  - Geolocate detected survivors and hazards
  - Map hazard zones and safe routes
  - Raise prioritised alerts
  - Produce situational reports
  - Navigate / cover an area autonomously
  - Present all situational intelligence on a command-centre dashboard

---

## Architecture (Existing, Keep and Extend)
- **`inference-service`** (Python, FastAPI, ONNX Runtime)
  - Publishes `Detection` events to Redis Streams.
- **`fusion-service`** (Go)
  - Consumes `Detection` + `Telemetry`.
  - Performs geo-projection, scoring, alerting, hazard zone mapping, safe routing, and report generation.
  - Offline queue via SQLite; persistent storage via Postgres/PostGIS; exposes REST + WebSocket APIs.
- **`mission-service`** (Python, pymavlink) — *NEW*
  - Coverage planning, MAVLink mission control against ArduPilot SITL, failsafes, obstacle avoidance, GPS-denied estimator.
- **`dashboard`** (React, Vite, TypeScript, Leaflet)
  - Command-centre user interface displaying maps, detections, telemetry, routes, alerts, and reports.
- **Support & Shared Modules:**
  - `shared/`: Protobuf + JSON Schema contracts.
  - `scripts/`: Operational, setup, and evaluation scripts.
  - `data/`: Datasets, calibration, and test fixtures.
  - `docs/`: Project documentation, status, capabilities, and benchmark results.
  - `docker-compose.yml`: Multi-container service composition.
  - `Makefile`: Standard development and build targets.

---

## Non-Negotiable Rules

1. **TRUTHFULNESS**
   - Never fabricate, hardcode, randomise, or "scale up" any metric, telemetry value, status flag, accuracy, latency, or result.
   - Every number shown in the UI, API, or docs must be measured or computed from real data.
   - If something is not implemented, the UI/API must say so (e.g. `"N/A"`) and docs must list it under limitations.
   - Fake data is a defect: remove it on sight.

2. **NO PLACEHOLDERS**
   - No `TODO` stubs presented as features.
   - No mocked model outputs in production code paths.
   - Test doubles are allowed only inside tests and the clearly named simulator.

3. **REPRODUCIBLE**
   - Seeds fixed, scripts idempotent.
   - Assets fetched by scripts with checksums.
   - A fresh clone must reach a working demo with documented commands.

4. **TESTED**
   - Every module gets unit tests.
   - Cross-service behaviour gets integration tests.
   - `make test` must pass and run in CI (GitHub Actions).

5. **MEASURED**
   - Performance and accuracy claims come only from files produced by `make evaluate` (`docs/RESULTS.md`), never typed by hand.

6. **CONTRACTS FIRST**
   - Message schemas live in `shared/` and are validated in all three languages (Python, Go, TypeScript).
   - Change the schema, regenerate contracts, and update tests together.

7. **SMALL, REVIEWABLE COMMITS**
   - Small, reviewable commits with clear messages.
   - After each task, append what changed and any known gap to `docs/STATUS.md`.

8. **HONEST REDUCTION ON INFEASIBILITY**
   - When a requirement is infeasible, do the honest reduced version.
   - Document it in `docs/STATUS.md` and `docs/CAPABILITIES.md`, and inform the user.
   - Do not paper over it.

---

## Standard Make Targets (Create / Maintain)
- `make up`: Start all services via docker-compose or local daemon.
- `make down`: Stop all running services.
- `make sitl`: Run ArduPilot SITL simulator instance.
- `make demo`: Run full end-to-end demonstration.
- `make test`: Run all unit and integration test suites.
- `make lint`: Run code formatting and lint checks across services.
- `make evaluate`: Run performance and accuracy evaluation benchmarks and output to `docs/RESULTS.md`.
- `make fetch-assets`: Download models and test assets with checksum validation.

---

## Definition of Done (DoD)
For any task:
1. Code written + Unit / Integration Tests passing.
2. Documentation updated.
3. `make test` is green.
4. Acceptance commands in the prompt pass.
5. `docs/STATUS.md` updated with changes and known gaps.
