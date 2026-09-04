# Live demo script (~5 minutes)

A step-by-step run sheet for demoing the SAR drone prototype end to end:
video → detections → geotags → alerts → live dashboard, including the
offline-buffering catch-up behavior.

## Before the demo (do this well before showtime)

The pipeline needs real assets that aren't checked into git:

1. **Trained model weights** at `data/models/person_yolov8.onnx`,
   `data/models/thermal_yolov8.onnx`, `data/models/hazard_classifier.onnx`.
   If you don't have these yet, train them first — see
   [`inference-service/README.md`](inference-service/README.md)'s
   **Training** section (`train_person_model.py`, `train_thermal_model.py`,
   `train_hazard_classifier.py`). Without these, `inference-service` logs
   `ModelNotFoundError` per frame and never publishes a detection — nothing
   in this script will work.
2. **Demo footage** at `data/sample_footage/demo_flight.mp4` (a short clip;
   gitignored, see [`data/sample_footage/README.md`](data/sample_footage/README.md)).
   `demo_telemetry.csv` is already checked in and will pace itself to
   whatever clip you use — for lat/lon to land near real-world detections,
   ideally re-record telemetry matching your clip, but the sample data is
   fine for showing the pipeline mechanics.
3. Docker Desktop (or an equivalent Docker Engine + Compose v2) running.

Run this once, ahead of time, so the audience doesn't watch a build:

```bash
docker compose up -d --build
docker compose ps   # wait until every service's STATUS shows "healthy"
```

Then verify the pipeline actually works before anyone's watching:

```bash
./scripts/seed_demo.sh
```

This starts the ingestion pipeline for up to two minutes and confirms
detections, fused geotags, and at least one alert all show up — exactly
the thing you're about to demo live. If it fails, fix that first (see its
output for which piece didn't show up, and `docker compose logs
inference-service fusion-service`).

Once `seed_demo.sh` passes, restart the stack fresh so the demo starts from
an empty dashboard:

```bash
docker compose restart fusion-service dashboard
```

## The demo (~5 minutes)

Open http://localhost:5173 in a browser **before** you start narrating —
first paint can take a couple seconds.

### 0:00 – 0:30 — Orient

Point out, without clicking anything yet:

- The **status pill** (top left) — should read **Live**.
- The **health strip** (top right) — throughput and avg fusion latency,
  both currently near zero (nothing's flowing yet).
- The empty map and empty alert feed.

Say: *"This is the live operations dashboard — everything you're about to
see is coming off a real detection pipeline over Redis and Postgres, not
canned data."*

### 0:30 – 2:00 — Start the pipeline, watch detections populate live

In a terminal (can be visible to the audience, or narrate over it):

```bash
docker compose exec inference-service \
  python -m app.pipeline.run_demo --fps 5 --loop
```

Within a few seconds, on the dashboard:

- Colored circle markers start appearing on the map (person = red,
  fire = orange, flood = blue, structural_damage = amber).
- The **health strip** throughput ticks up from 0/min; avg fusion latency
  settles to a small, real number.
- The **alert feed** on the right starts filling in, newest first, with
  priority-tier badges (critical / high / medium).

Say: *"Each of these markers is a real YOLOv8/ResNet inference result,
geotagged by fusion-service using the drone's telemetry and a pinhole
camera projection, scored for priority, and pushed here over a
WebSocket the instant it's written."*

### 2:00 – 3:00 — Trigger the offline toggle

The dashboard container runs Vite's dev server, so the dev-only demo
control panel is visible right on the page — look for the amber-outlined
**"Dev only — demo controls"** box above the map, with a **"Simulate
offline"** button. Click it.

(No UI handy, or want it scripted? Same effect either way:

```bash
curl -X POST http://localhost:8002/debug/set-offline \
  -H "Content-Type: application/json" \
  -d '{"offline": true}'
```

— that's exactly the request the button makes.)

On the dashboard, within a couple seconds:

- The status pill flips to **Offline — Queueing** (amber), and the
  button's own label flips to **"Go online"**.
- New markers/alerts keep appearing — say: *"Detections aren't being
  lost — fusion-service is writing every one of these to a local SQLite
  queue first, before it ever touches Postgres. If the network or the
  database drops, nothing here changes for the operator except this
  banner."*

### 3:00 – 4:00 — Reconnect and show the catch-up

Click **"Go online"** in the same debug panel (or re-run the curl above
with `{"offline": false}`).

Within a couple seconds:

- The status pill returns to **Live**.
- Every detection that queued while offline lands in Postgres in the
  order it was originally detected, each one pushed to the dashboard
  again — same markers/alerts, no duplicates, no gaps.

Say: *"That's the whole point of the local queue: the operator never sees
a hole in the timeline, they just see a brief 'offline' banner and then
everything catches up in order."*

### 4:00 – 5:00 — Open an alert end to end

1. Click any marker on the map whose color matches a "survivor" (red).
2. In the **Detection Detail** panel, read off: class, confidence,
   timestamp, coordinates.
3. Scroll the **alert feed** to find the alert generated from that same
   detection (matching detection id/time) and read its message — e.g.
   *"Survivor detected 12m from active fire zone."*
4. Say: *"The alert text isn't generic — it's rule-based on this
   detection's class and its distance to the nearest recorded hazard, so
   a responder immediately knows why this one matters."*

Stop the pipeline when you're done:

```bash
# Ctrl+C the `run_demo` terminal, or:
docker compose exec inference-service pkill -f app.pipeline.run_demo
```

## Fallback / troubleshooting

- **Nothing appears on the map**: check
  `docker compose logs inference-service` for `ModelNotFoundError` — the
  model weights aren't in place (see prerequisites above).
- **Status pill stuck on "Disconnected"**: `fusion-service` isn't
  reachable from the browser — confirm `docker compose ps` shows it
  healthy and `http://localhost:8002/health` responds.
- **Offline toggle doesn't change anything**: confirm the curl command
  actually hit fusion-service (`{"offline":true}` in the response), and
  that new detections are still being published (the toggle only affects
  *newly processed* detections, not ones already on screen).
- **Full reset** between rehearsals:
  `docker compose down -v && docker compose up -d --build` (the `-v`
  drops the Postgres and offline-queue volumes for a clean slate).
