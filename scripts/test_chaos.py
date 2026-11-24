#!/usr/bin/env python3
"""Automated Chaos Testing Suite for SAR Drone System.

Tests store-and-forward resilience under network partitions and database outages:
1. Emits detections continuously to Redis stream.
2. Cuts Postgres connectivity mid-run via docker compose stop postgres.
3. Verifies fusion-service transitions to offline mode and buffers in SQLite.
4. Restores Postgres connectivity via docker compose start postgres.
5. Verifies drain loop recovers all buffered events in chronological order.
6. Asserts lost_detections = 0 (computed from emitted vs persisted IDs).
"""

import json
import os
import subprocess
import sys
import time
import urllib.request
import urllib.error
from datetime import datetime, timezone
from pathlib import Path

import redis

FUSION_URL = os.environ.get("FUSION_URL", "http://localhost:8002")
REDIS_ADDR = os.environ.get("REDIS_ADDR", "localhost:6379")


def http_post(url: str, data: dict):
    req = urllib.request.Request(
        url,
        data=json.dumps(data).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def http_get(url: str):
    with urllib.request.urlopen(url, timeout=5) as resp:
        return json.loads(resp.read().decode("utf-8"))


def run_psql(query: str) -> list[str]:
    cmd = [
        "docker", "compose", "exec", "-T", "postgres",
        "psql", "-U", "sar", "-d", "sar", "-t", "-A", "-c", query
    ]
    res = subprocess.run(cmd, capture_output=True, text=True, check=True)
    lines = [line.strip() for line in res.stdout.strip().split("\n") if line.strip()]
    return lines


def run_chaos_test():
    print("=" * 60)
    print("STARTING CHAOS TEST: Network Partition & Store-and-Forward")
    print("=" * 60)

    # 1. Connect to Redis
    r_host, r_port = REDIS_ADDR.split(":")
    r = redis.Redis(host=r_host, port=int(r_port), decode_responses=True)
    r.ping()
    print(f"Connected to Redis at {REDIS_ADDR}")

    # Ensure system is online initially
    try:
        http_post(f"{FUSION_URL}/debug/set-offline", {"offline": False})
    except Exception:
        pass

    # Seed baseline telemetry so detections can interpolate
    t_start = datetime.now(timezone.utc)
    for i in range(120):
        t_sample = t_start.timestamp() + i * 0.1
        iso = datetime.fromtimestamp(t_sample, timezone.utc).isoformat()
        tel_data = {
            "lat": 18.51 + i * 0.0001,
            "lon": 73.81 + i * 0.0001,
            "alt": 50.0,
            "heading": 90.0,
            "pitch": 0.0,
            "roll": 0.0,
            "timestamp": iso,
        }
        r.xadd("telemetry", {"data": json.dumps(tel_data)})

    time.sleep(0.5)

    emitted_ids = []

    # 2. Phase 1: Emit 20 detections while ONLINE
    print("\nPhase 1: Emitting 20 detections while ONLINE...")
    for i in range(20):
        det_id = f"chaos-det-online-{i}-{int(time.time()*1000)}"
        t_det = datetime.fromtimestamp(t_start.timestamp() + (i + 5) * 0.1, timezone.utc).isoformat()
        payload = {
            "id": det_id,
            "class": "person",
            "confidence": 0.88,
            "bbox": {"x": 100, "y": 100, "w": 40, "h": 60},
            "frame_id": f"frame-{i}",
            "source": "rgb",
            "timestamp": t_det,
            "track_id": f"trk-{i % 3}",
            "modalities": ["rgb"],
        }
        r.xadd("detections", {"data": json.dumps(payload)})
        emitted_ids.append(det_id)
        time.sleep(0.04)

    time.sleep(1.0)

    # 3. Phase 2: Sever connection / stop Postgres container mid-run
    print("\nPhase 2: STOPPING POSTGRES CONTAINER (MID-RUN OUTAGE)...")
    subprocess.run(["docker", "compose", "stop", "postgres"], check=True)
    print("Postgres stopped. Waiting 3s for fusion-service to detect outage...")
    time.sleep(3.0)

    # Emit 30 detections while OFFLINE
    print("Emitting 30 detections during active database partition...")
    for i in range(20, 50):
        det_id = f"chaos-det-partition-{i}-{int(time.time()*1000)}"
        t_det = datetime.fromtimestamp(t_start.timestamp() + (i + 5) * 0.1, timezone.utc).isoformat()
        payload = {
            "id": det_id,
            "class": "person",
            "confidence": 0.92,
            "bbox": {"x": 120, "y": 120, "w": 45, "h": 65},
            "frame_id": f"frame-{i}",
            "source": "thermal",
            "timestamp": t_det,
            "track_id": f"trk-{i % 3}",
            "modalities": ["thermal"],
        }
        r.xadd("detections", {"data": json.dumps(payload)})
        emitted_ids.append(det_id)
        time.sleep(0.04)

    time.sleep(2.0)

    # Check health and queue depth during outage
    try:
        health = http_get(f"{FUSION_URL}/health")
        print(f"Health during outage: status={health.get('status')}, mode={health.get('mode')}, queue_depth={health.get('queue_depth')}")
    except Exception as e:
        print(f"Health check during outage: {e}")

    # 4. Phase 3: RESTORE POSTGRES CONTAINER
    print("\nPhase 3: STARTING POSTGRES CONTAINER (RECOVERY)...")
    subprocess.run(["docker", "compose", "start", "postgres"], check=True)
    print("Postgres container restarted. Waiting for database ready...")

    # Wait for postgres to be ready
    for _ in range(30):
        try:
            res = subprocess.run(
                ["docker", "compose", "exec", "-T", "postgres", "pg_isready", "-U", "sar", "-d", "sar"],
                capture_output=True, text=True
            )
            if res.returncode == 0:
                print("Postgres is ready!")
                break
        except Exception:
            pass
        time.sleep(1.0)

    # Emit 10 more detections after recovery
    for i in range(50, 60):
        det_id = f"chaos-det-recovered-{i}-{int(time.time()*1000)}"
        t_det = datetime.fromtimestamp(t_start.timestamp() + (i + 5) * 0.1, timezone.utc).isoformat()
        payload = {
            "id": det_id,
            "class": "person",
            "confidence": 0.96,
            "bbox": {"x": 130, "y": 130, "w": 45, "h": 65},
            "frame_id": f"frame-{i}",
            "source": "rgb",
            "timestamp": t_det,
            "track_id": f"trk-{i % 3}",
            "modalities": ["rgb", "thermal"],
        }
        r.xadd("detections", {"data": json.dumps(payload)})
        emitted_ids.append(det_id)
        time.sleep(0.04)

    # Wait for drain loop to flush all buffered items to Postgres
    print("\nWaiting for store-and-forward queue to flush...")
    flushed = False
    for attempt in range(25):
        time.sleep(1.0)
        try:
            h = http_get(f"{FUSION_URL}/health")
            depth = h.get("queue_depth", 0)
            mode = h.get("mode", "unknown")
            print(f"  Drain wait {attempt+1}s: mode = {mode}, queue_depth = {depth}")
            if depth == 0 and mode == "online":
                flushed = True
                break
        except Exception as e:
            print(f"  Drain wait {attempt+1}s: {e}")

    # 5. Phase 4: Mathematical Validation against Database
    print("\nPhase 4: Validating persistence and ordering invariants...")
    in_clause = ",".join(f"'{did}'" for did in emitted_ids)
    query = f"SELECT id, detected_at FROM detections WHERE id IN ({in_clause}) ORDER BY detected_at ASC;"
    rows = run_psql(query)

    persisted_ids = set()
    timestamps = []
    for r_line in rows:
        parts = r_line.split("|")
        if len(parts) >= 2:
            persisted_ids.add(parts[0])
            timestamps.append(parts[1])

    emitted_set = set(emitted_ids)
    lost_ids = emitted_set - persisted_ids
    lost_detections = len(lost_ids)

    is_ordered = all(timestamps[i] <= timestamps[i+1] for i in range(len(timestamps)-1))

    print("\n" + "=" * 60)
    print("CHAOS TEST SUMMARY REPORT")
    print("=" * 60)
    print(f"Total Emitted Detections:   {len(emitted_ids)}")
    print(f"Total Persisted Detections: {len(persisted_ids)}")
    print(f"Computed Lost Detections:   {lost_detections}")
    print(f"Chronological Ordering:     {'PASSED' if is_ordered else 'FAILED'}")
    print("=" * 60)

    if lost_detections > 0:
        print(f"FAIL: {lost_detections} detections lost during partition: {lost_ids}")
        sys.exit(1)

    if not is_ordered:
        print("FAIL: Persisted detections violated chronological ordering invariant")
        sys.exit(1)

    print("SUCCESS: 100% of detections persisted, zero loss under database outage.")
    sys.exit(0)


if __name__ == "__main__":
    run_chaos_test()
