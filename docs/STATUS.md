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
  - SITL integration and mission-service implementation (Prompts 2+).
  - Benchmark evaluation target (`make evaluate`) implementation.
