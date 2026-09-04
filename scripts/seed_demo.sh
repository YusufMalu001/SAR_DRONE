#!/usr/bin/env bash
# Seeds the running docker-compose stack with the demo footage/telemetry and
# confirms the full pipeline produces detections, fused geotags, and at
# least one alert within the first two minutes of playback.
#
# Usage:
#   docker compose up -d --build
#   ./scripts/seed_demo.sh
#
# Requires: the stack already running (docker compose up), and
# data/sample_footage/demo_flight.mp4 present (gitignored — not shipped in
# the repo; see data/sample_footage/README.md). demo_telemetry.csv is
# already checked in.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

FUSION_URL="${FUSION_URL:-http://localhost:8002}"
DEADLINE_SECONDS="${DEADLINE_SECONDS:-120}"
PIPELINE_PID=""

log() { printf '[seed_demo] %s\n' "$1"; }
fail() { printf '[seed_demo] FAIL: %s\n' "$1" >&2; exit 1; }

cleanup() {
  if [ -n "$PIPELINE_PID" ] && kill -0 "$PIPELINE_PID" 2>/dev/null; then
    log "stopping ingestion pipeline (pid $PIPELINE_PID)"
    docker compose exec -T inference-service pkill -f app.pipeline.run_demo 2>/dev/null || true
    kill "$PIPELINE_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# --- preconditions ---

[ -f "data/sample_footage/demo_flight.mp4" ] || fail \
  "data/sample_footage/demo_flight.mp4 not found. It's gitignored (too large) — drop a short demo clip there first. See data/sample_footage/README.md."
[ -f "data/sample_footage/demo_telemetry.csv" ] || fail \
  "data/sample_footage/demo_telemetry.csv not found (should be checked into the repo)."

for svc in postgres redis inference-service fusion-service; do
  status="$(docker compose ps --format json "$svc" 2>/dev/null \
    | grep -o '"Health":"[a-z]*"' | head -1 | cut -d'"' -f4 || true)"
  [ "$status" = "healthy" ] || fail "$svc is not healthy (docker compose ps reports Health='$status'). Run 'docker compose up -d --build' first."
done

# --- start the demo pipeline (video ingestion + telemetry replay) ---

log "starting video ingestion + telemetry replay inside inference-service..."
docker compose exec -T inference-service \
  python -m app.pipeline.run_demo --fps 5 --max-frames 600 &
PIPELINE_PID=$!

# --- poll for detections, geotags, and an alert within the deadline ---

have_detection=""
have_alert=""
start_ts=$(date +%s)

while :; do
  now_ts=$(date +%s)
  elapsed=$((now_ts - start_ts))
  if [ "$elapsed" -ge "$DEADLINE_SECONDS" ]; then
    break
  fi

  if [ -z "$have_detection" ]; then
    body="$(curl -sf "$FUSION_URL/detections?limit=1" || true)"
    if [ -n "$body" ] && [ "$body" != "[]" ] && [ "$body" != "null" ]; then
      have_detection="$body"
      log "detection + fused geotag confirmed after ${elapsed}s:"
      printf '  %s\n' "$have_detection"
    fi
  fi

  if [ -z "$have_alert" ]; then
    body="$(curl -sf "$FUSION_URL/alerts?limit=1" || true)"
    if [ -n "$body" ] && [ "$body" != "[]" ] && [ "$body" != "null" ]; then
      have_alert="$body"
      log "alert confirmed after ${elapsed}s:"
      printf '  %s\n' "$have_alert"
    fi
  fi

  if [ -n "$have_detection" ] && [ -n "$have_alert" ]; then
    log "PASS: detections, fused geotags, and an alert all confirmed within ${elapsed}s (limit ${DEADLINE_SECONDS}s)."
    exit 0
  fi

  sleep 3
done

[ -n "$have_detection" ] || log "no detections appeared within ${DEADLINE_SECONDS}s"
[ -n "$have_alert" ] || log "no alert appeared within ${DEADLINE_SECONDS}s"
fail "pipeline did not produce the expected output within ${DEADLINE_SECONDS}s. Check 'docker compose logs inference-service fusion-service'."
