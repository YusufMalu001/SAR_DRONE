# Operational Runbook & Edge Deployment Guide

This runbook describes operating Khoj-o-Drone on constrained UAV hardware, handling offline network-denied states, and troubleshooting common operational issues.

---

## 1. Edge Drone Deployment Profile

For deployment on edge drone companion computers (such as NVIDIA Jetson Orin Nano, Raspberry Pi 5, or Intel N100 SBC):

### 1.1 Resource Envelope Constraints
The edge deployment profile enforces strict hardware resource caps:
- **CPU Allocation:** 4.0 CPU Cores
- **RAM Budget:** 8.0 GB RAM
- **Storage Mode:** Local eMMC / NVMe SSD with SQLite WAL mode

### 1.2 Running the Edge Profile
Launch services using the capped compose configuration:

```bash
docker compose -f docker-compose.edge.yml up -d --build
```

### 1.3 Measuring Edge Performance
To benchmark sustained throughput and memory utilization under edge constraints:

```bash
python3 scripts/measure_edge_profile.py
```
*Current benchmark results are recorded in `data/edge_benchmark.json` and mirrored in `docs/RESULTS.md`.*

---

## 2. Network-Denied & Offline Autonomous Behavior

In disaster areas, cellular towers and RF datalinks are frequently damaged or absent. Khoj-o-Drone functions fully autonomously without external network connections.

### 2.1 Store-and-Forward Lifecycle
1. **Local Write-Ahead Queue:** Detections, entities, alerts, and outbound messages are committed immediately to `/data/fusion-queue.db` (SQLite in WAL mode with `synchronous=NORMAL`).
2. **Degraded Mode Indication:**
   - When the PostgreSQL container or telemetry radio drops, `/health` reports `"mode": "offline"`.
   - The command-centre dashboard activates the **Autonomous Offline Mode** banner with live queue depth.
3. **Automatic Drain & Synchronization:**
   - The background drain loop continuously monitors upstream database reachability.
   - Upon reconnection, pending records are drained in chronological order using deterministic Last-Write-Wins (LWW) deduplication.
   - Guaranteed metric: **$0$ lost detections**.

### 2.2 Offline Map Tiles
To provide responsive geospatial mapping without Internet access:
1. Pre-render offline raster tiles:
   ```bash
   python3 scripts/cache_tiles.py --bbox 19.9 77.9 20.1 78.1 --min-zoom 12 --max-zoom 17 --output /data/tiles
   ```
2. `fusion-service` serves tiles directly at `GET /tiles/{z}/{x}/{y}.png`.
3. The dashboard map seamlessly renders offline tiles with automatic fallback indicators.

---

## 3. Operational Troubleshooting

| Symptom | Probable Cause | Diagnostic Command & Fix |
| :--- | :--- | :--- |
| **`Dashboard shows Disconnected`** | `fusion-service` is unreachable or starting up | Run `docker compose ps`. Check logs: `docker compose logs fusion-service`. Verify `http://localhost:8002/health`. |
| **`No detections appear on map`** | Model ONNX files missing from `models/` | Run `./scripts/fetch-assets.sh` to download models and verify sha256 checksums. Check `docker compose logs inference-service`. |
| **`Queue Depth is growing continuously`** | PostgreSQL is partitioned or paused | Check PostgreSQL container: `docker compose ps postgres`. If offline testing is active, toggle online via `curl -X POST http://localhost:8002/debug/set-offline -d '{"offline":false}' -H "Content-Type: application/json"`. |
| **`SITL drone does not arm`** | SITL initialization timeout or pre-arm check | Verify SITL heartbeat: `nc -zv 127.0.0.1 5760`. Check `docker compose logs sitl`. Ensure battery $\ge 20\%$. |
| **`MAVLink link loss triggers RTL`** | Heartbeat lost for $> 5.0\text{s}$ | Failsafe is functioning correctly. Check physical telemetry link or SITL connection string in `mission-service`. |
| **`Clean Slate Reset`** | Resetting all runtime state and databases | Run `make down && docker compose down -v && make up`. |
