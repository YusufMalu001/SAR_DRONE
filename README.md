# sar-drone-prototype

Prototype search-and-rescue drone software system: video/thermal frames flow
into a Python inference service, detections get geotagged, fused with
telemetry, scored for priority, and alerted on by a Go service, and results
are persisted to PostGIS and shown live on a React operations dashboard.

**Want to run the 5-minute live demo? Start with [`DEMO.md`](DEMO.md).**

## Structure

```
sar-drone-prototype/
  inference-service/   Python (FastAPI) — YOLOv8/ResNet inference, video ingestion, publishes Detection/Telemetry
  fusion-service/       Go — geotagging, priority scoring, alerting, offline buffering, REST + WebSocket API
  dashboard/             React (Vite + TS) — live operations dashboard (map, alerts, connectivity, health)
  shared/                 Protobuf + JSON Schema definitions shared across services
  data/                   sample footage/datasets/model weights (gitignored except small samples)
  scripts/                cross-service scripts (seed_demo.sh)
  docker-compose.yml
  DEMO.md                 step-by-step live demo script
```

See each service's own README for full details:
[inference-service](inference-service/README.md) ·
[fusion-service](fusion-service/README.md) ·
[dashboard](dashboard/README.md) ·
[shared](shared/README.md)

## What's implemented

- **inference-service**: three ONNX models (person detector, thermal
  detector, hazard classifier) run per video frame; a video ingestion loop
  and telemetry replay publish `Detection`/`Telemetry` events to Redis
  streams.
- **fusion-service**: consumes both streams, matches each detection to the
  nearest telemetry sample, geo-projects it to a lat/lon (pinhole camera
  model), scores its priority, generates a rule-based alert above a
  threshold, computes a safe-route heuristic, and serves it all over a REST
  API and a `/ws/live` WebSocket. Every write goes through a local SQLite
  offline queue first, so a Postgres/network outage doesn't lose data —
  toggle it live via `POST /debug/set-offline`.
- **dashboard**: a dark live-operations UI — Leaflet map with
  class-colored markers, a priority-badged alert feed, a connectivity
  status pill, a throughput/latency health strip, and a detection detail
  panel.

## Message contracts

Two event types flow through Redis **streams** between services — schemas
live in [`shared/`](shared/):

- **Detection** — `{id, class, confidence, bbox, frame_id, source, timestamp}`
  (`class`: person/fire/flood/debris/structural_damage, `source`: rgb/thermal)
- **Telemetry** — `{lat, lon, alt, heading, timestamp}`

fusion-service's own REST/WebSocket API adds `DetectionRecord` (fused,
geotagged, with a `status`), `AlertRecord`, and `RouteStatus` — see
[`fusion-service/README.md`](fusion-service/README.md).

## Run everything locally with Docker Compose

Requires Docker Desktop (or Docker Engine + Compose v2).

```bash
docker compose up -d --build
docker compose ps   # wait until every service's STATUS shows "healthy"
```

| service            | URL                    | notes                          |
|---------------------|-------------------------|---------------------------------|
| postgres (PostGIS)  | localhost:5432          | schema applied by fusion-service on startup (golang-migrate) |
| redis               | localhost:6379          |                                  |
| inference-service   | http://localhost:8001   | `GET /health`; run the pipeline with `docker compose exec inference-service python -m app.pipeline.run_demo` |
| fusion-service      | http://localhost:8002   | `GET /health`, `GET /detections`, `GET /alerts`, `GET /mission/route-status`, `/ws/live` |
| dashboard           | http://localhost:5173   | live operations dashboard       |

The pipeline needs trained model weights and demo footage that aren't
checked into git — see [`DEMO.md`](DEMO.md)'s prerequisites section before
expecting to see detections. Once the stack is healthy and those assets are
in place, verify the whole thing works end to end with:

```bash
./scripts/seed_demo.sh
```

> **Note:** `fusion-service/go.mod` was hand-written without network access
> and has no `go.sum` yet — run `cd fusion-service && go mod tidy` once
> before the first build, or the Go build step will fail.

## Run everything locally without Docker

Start Postgres (with PostGIS) and Redis yourself, then in three terminals:

```bash
# 1. inference-service
cd inference-service
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --reload --port 8001

# 2. fusion-service
cd fusion-service
go mod tidy
go run ./cmd/fusion-service

# 3. dashboard
cd dashboard
cp .env.example .env
npm install
npm run dev
```

Each service's README has full config/env var details.

## Health checks

- `inference-service`: `GET http://localhost:8001/health` → `{status, service, redis}`
- `fusion-service`: `GET http://localhost:8002/health` → `{status, service, postgres, redis}`
- `dashboard`: visit `http://localhost:5173/status` to see both, polled every 5s
- All five docker-compose services (postgres, redis, inference-service,
  fusion-service, dashboard) have container healthchecks — `docker compose
  ps` shows each one's status directly.

## Known gaps

- No trained model weights or demo footage are checked in (too large) —
  see `DEMO.md` for what to prepare before a demo, and each service's
  README for training scripts / sample data notes.
- `fusion-service/go.mod` has no committed `go.sum` (see note above).
- Alerts generated from a detection drained out of the offline queue use a
  best-effort (not perfectly historical) hazard-proximity context — see
  the doc comment on `fusion-service/internal/fusion`.
