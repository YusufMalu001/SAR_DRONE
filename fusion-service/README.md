# fusion-service

Go service that consumes `Detection` and `Telemetry` events (see
[`../shared/jsonschema`](../shared/jsonschema)) from Redis streams, matches
each detection to the nearest-in-time telemetry sample, geo-projects it into
a lat/lon, scores its priority, and persists the fused result into PostGIS.
Also serves a REST API and a live WebSocket feed over the same data.

## Pipeline

1. **Consume** — `internal/fusion.Consumer` reads the `detections` and
   `telemetry` Redis streams via a consumer group (`XREADGROUP` / `XACK`, so
   processing survives a restart without dropping or double-processing
   in-flight messages under normal operation).
2. **Match** — `internal/telemetry.Buffer` keeps a small time-ordered window
   of recent telemetry; each detection is matched to the nearest sample
   within `TELEMETRY_TOLERANCE_MS` (default 200ms). No match within
   tolerance ⇒ the detection is dropped (logged), since it can't be
   geotagged.
3. **Geo-project** — `internal/geo.ProjectDetection` converts the
   detection's bbox center into a ground lat/lon using a pinhole camera
   model: pixel offset from center → angle (via `CAMERA_HFOV_DEG`/
   `CAMERA_VFOV_DEG`) → ground offset in meters (via matched altitude) →
   rotated into north/east by heading → applied to the drone's lat/lon.
   Assumes a nadir-mounted camera; see the doc comment on `ProjectDetection`
   for the full set of assumptions.
4. **Score priority** — `internal/priority.Score` combines a per-class base
   priority (person highest), confidence, and a proximity boost if a
   hazard-class detection was recorded within `HAZARD_PROXIMITY_METERS` in
   the last `HAZARD_WINDOW_SECONDS`.
5. **Persist + broadcast** — the fused detection is written to the local
   offline queue first (see below), then to the `detections` table
   (`internal/db`) if online, and pushed to every `/ws/live` client
   (`internal/ws`) either way.
6. **Alert** — if the detection's priority score clears `AlertThreshold`
   (0.7), `internal/alerting` composes a rule-based message and an Alert is
   persisted and broadcast (see below).

## Alerting

Any detection persisted with `priority_score > 0.7` generates an Alert:
`{ detection_id, priority_tier, message, created_at }`, stored in the
`alerts` table (one per detection — a redelivered/reprocessed detection
never creates a duplicate) and pushed over `/ws/live` as a distinct message
type so the dashboard doesn't have to guess.

- **priority_tier**: `critical` (score ≥ 0.9), `high` (≥ 0.8), or `medium`
  (> 0.7) — see `alerting.Tier`.
- **message**: rule-based, built from the detection's class and (for a
  survivor) the nearest hazard-class detection recorded within
  `HAZARD_PROXIMITY_METERS` at the time — the same proximity check used for
  the priority boost, so the two agree. Templates: survivor near fire /
  flood / debris, survivor in a structurally damaged zone, isolated
  survivor (no nearby hazard), or a generic message for a hazard-class
  detection itself. See `internal/alerting/alerting.go`.

Note: for a detection drained from the offline queue, the hazard-proximity
context used for its alert is re-evaluated against whatever's in the
tracker *at drain time* — the original context isn't preserved through the
SQLite queue (see the doc comment on `internal/fusion`).

- `GET /alerts?limit=` — most recent alerts, newest first

## Safe-route heuristic

`GET /mission/route-status` reconstructs the drone's recorded flight path
(every persisted `telemetry` row, oldest first — see below) and checks each
consecutive segment against every persisted hazard-class detection: if a
segment passes within `ROUTE_OBSTRUCTION_METERS` (default 20m) of a hazard,
it's flagged `"obstructed"` (otherwise `"clear"`), naming the nearest hazard
and the distance either way. Distance uses `geo.PointToSegmentMeters` — a
local flat-earth point-to-segment calculation, not just distance to the
segment's endpoints.

Telemetry is now persisted (`internal/db.InsertTelemetry`, table added in
migration `000003_telemetry`) in addition to the existing in-memory
matching buffer — best-effort (logged, not offline-queued) since it's a
secondary feature, not the primary detection pipeline.

## Offline buffering

Every fused detection is written to a local SQLite file (`internal/localqueue`)
**before** Postgres is ever attempted — so nothing is lost if Postgres (or
the network to it) is unreachable.

- `internal/connectivity.Monitor` decides whether the service is "offline":
  either a real periodic Postgres ping fails, or an operator forces it via
  `POST /debug/set-offline {"offline": true|false}`.
- **Online**: enqueue to SQLite, then immediately insert into Postgres; on
  success the SQLite row is deleted and the WebSocket broadcast is tagged
  `"status": "live"`. If the Postgres insert fails, the row stays queued
  (broadcast as `"queued"`) for the drain loop to retry.
- **Offline**: enqueue to SQLite only; broadcast immediately as
  `"queued"`, so the dashboard can show it right away even though it isn't
  durable in Postgres yet.
- **Recovery**: `Consumer.RunDrainLoop` drains the queue into Postgres in
  `detected_at` order as soon as connectivity returns (triggered instantly
  by the monitor's offline→online transition, with a periodic fallback
  check too). Each drained row is broadcast as
  `"synced-from-offline-queue"`.

Debug/demo endpoints:

- `POST /debug/set-offline {"offline": true|false}` — force offline mode
- `GET /debug/offline-status` — `{ "offline": bool }`
- `GET /debug/queue` — `{ "count": N, "ids": [...] }`, the local queue's
  current contents

`DetectionRecord.status` (on every `/ws/live` message) is one of `"live"`,
`"queued"`, or `"synced-from-offline-queue"` — REST reads (always sourced
from Postgres) report `"synced"`. Note that a detection sitting only in the
local queue isn't yet retrievable via `GET /detections/{id}` — only the
WebSocket push and `GET /debug/queue` see it before it's drained.

## Migrations

golang-migrate-style `.up`/`.down.sql` pairs under `internal/migrations/`,
embedded into the binary (`migrations.go`, `//go:embed`) and applied
automatically on every startup via `internal/migrate` — `m.Up()` is a
no-op once the schema is current, so this is safe to run every time,
including in tests.

- `000001_init` — `detections` (`geometry(Point, 4326)` column + GIST index)
- `000002_alerts` — `alerts` (one per detection, FK + UNIQUE on `detection_id`)
- `000003_telemetry` — `telemetry` (the recorded flight path)

To apply/roll back manually instead (e.g. while iterating on the SQL):

```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate -source file://internal/migrations -database "$DATABASE_URL" up
```

## Local setup (no Docker)

```bash
cd fusion-service
go mod tidy   # resolves go.sum (network access required, not run in this scaffold)
go run ./cmd/fusion-service
```

Requires a reachable Postgres (with PostGIS) and Redis — see
[`../docker-compose.yml`](../docker-compose.yml). The demo data pipeline in
`inference-service` (`python -m app.pipeline.run_demo`) is what actually
publishes to the `detections`/`telemetry` streams for this service to
consume.

## REST API

- `GET /detections?limit=&class=` — most recent fused detections (newest
  first)
- `GET /detections/{id}` — a single fused detection, 404 if unknown
- `GET /mission/summary` — aggregate counts: total, by class, high-priority
  count (`priority_score >= HIGH_PRIORITY_THRESHOLD`), average priority,
  latest detection time
- `GET /alerts?limit=` — most recent alerts, newest first
- `GET /mission/route-status` — flight-path segments with obstruction status

## WebSocket

- `GET /ws/live` — upgrades to a WebSocket. Every message is a
  `{"type": ..., "payload": ...}` envelope (`internal/dto.WSMessage`):
  - `"type": "detection"` — a fused detection, `payload` is a
    `DetectionRecord` (same shape as the REST endpoints above)
  - `"type": "alert"` — a new alert, `payload` is an `AlertRecord`

## Health

- `GET /` — service info
- `GET /health` — `{ status, service, postgres, redis }`

## Tests

```bash
# unit tests (none require external services)
go test ./...

# integration tests — need real Postgres + Redis
docker compose up -d postgres redis
go test -tags=integration ./tests/integration/...
```

`internal/alerting/alerting_test.go` — pure-function unit tests for the
message/tier rules, including the survivor-near-fire and
survivor-with-no-nearby-hazard scenarios (no external services needed).

`tests/integration/pipeline_test.go` publishes a synthetic telemetry +
detection pair directly to Redis and asserts the fused result appears in
Postgres with the expected lat/lon (computed independently via
`geo.ProjectDetection` in the test), is reachable via `GET /detections/{id}`,
and arrives over `/ws/live` tagged `"live"`.

`tests/integration/offline_test.go` forces offline mode via
`POST /debug/set-offline`, publishes 5 detections, confirms they're in the
local SQLite queue and NOT yet in Postgres (and broadcast as `"queued"`),
then flips connectivity back on and confirms all 5 land in Postgres — in
`detected_at` order, verified via each row's `created_at` — within a few
seconds, broadcast as `"synced-from-offline-queue"`.

`tests/integration/alerts_test.go` publishes a fire detection then a
high-confidence survivor at the same spot and asserts the resulting alert
(critical tier, message mentioning fire) is in Postgres, listed via
`GET /alerts`, and pushed over `/ws/live` as an `"alert"`-typed message.

`tests/integration/route_test.go` builds a 3-point flight path with a fire
detection on top of the first waypoint and asserts
`GET /mission/route-status` flags that leg `"obstructed"` (fire is well
within 20m) while the second leg (~55m from the fire) stays `"clear"`.

## Config (env vars)

| var                        | default                                                  |
|-----------------------------|------------------------------------------------------------|
| `PORT`                      | `8002`                                                       |
| `REDIS_ADDR`                | `localhost:6379`                                             |
| `DATABASE_URL`              | `postgres://sar:sar@localhost:5432/sar?sslmode=disable`      |
| `DETECTIONS_CHANNEL`        | `detections`                                                 |
| `TELEMETRY_CHANNEL`         | `telemetry`                                                  |
| `CONSUMER_GROUP`            | `fusion-service`                                             |
| `CONSUMER_NAME`             | `fusion-service-1`                                           |
| `TELEMETRY_TOLERANCE_MS`    | `200`                                                        |
| `CAMERA_HFOV_DEG`           | `90`                                                         |
| `CAMERA_VFOV_DEG`           | `60`                                                         |
| `HAZARD_PROXIMITY_METERS`   | `50`                                                         |
| `HAZARD_WINDOW_SECONDS`     | `60`                                                         |
| `HIGH_PRIORITY_THRESHOLD`   | `0.7`                                                        |
| `LOCAL_QUEUE_PATH`          | `/data/fusion-queue.db`                                      |
| `CONNECTIVITY_CHECK_INTERVAL_MS` | `5000`                                                  |
| `DRAIN_INTERVAL_MS`         | `2000`                                                       |
| `ROUTE_OBSTRUCTION_METERS`  | `20`                                                         |
| `ROUTE_STATUS_TELEMETRY_LIMIT` | `5000`                                                    |

## Known limitations (prototype)

- `go.mod` was hand-written without network access — there's no `go.sum`
  yet. Run `go mod tidy` before building/testing.
- The camera projection assumes a nadir-mounted, non-gimbal-stabilized(-ish)
  camera whose image "up" faces the drone's heading — a real gimbal would
  need its own pitch/roll/yaw in the telemetry stream and a fuller
  projection model.
- If no telemetry sample lands within tolerance, the detection is dropped
  rather than queued/retried (this happens before the offline queue, since
  it hasn't been geotagged yet).
- The local SQLite queue uses `modernc.org/sqlite` (pure Go, no CGO) so
  cross-compiling into the Alpine Docker image needs no C toolchain. It's
  a single file (`LOCAL_QUEUE_PATH`); in docker-compose it's given its own
  named volume (`fusion-queue-data`) so it survives container restarts.
