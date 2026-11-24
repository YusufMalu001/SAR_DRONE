# Guided 5-Minute Demonstration Script: Khoj-o-Drone

A step-by-step walkthrough for demonstrating the autonomous search-and-rescue drone system end-to-end:
**Autonomous Flight → Multi-Modal Perception → Real-Time Geotagging → Hazard Mapping & Safe Routing → Offline Store-and-Forward → Situational Report (SitRep)**.

---

## Prerequisites (Pre-Flight Checklist)

Run this once before the presentation:

```bash
# 1. Fetch models and sample assets (validates sha256 checksums):
./scripts/fetch-assets.sh

# 2. Launch system services:
make up

# 3. Open Command-Centre Dashboard in browser:
# Navigate to: http://localhost:5173
```

Confirm that `docker compose ps` shows `postgres`, `redis`, `inference-service`, and `fusion-service` as `healthy`.

---

## 5-Minute Live Demonstration Timeline

### 0:00 – 1:00: Mission Launch & Coverage Planning
1. **Open the Dashboard:** View the main command-centre map at `http://localhost:5173`.
2. **Launch the Demo Scenario:** In terminal, execute:
   ```bash
   make demo SCENARIO=flood MODE=replay
   ```
   *(For SITL hardware-in-the-loop: `make sitl && make demo SCENARIO=flood MODE=sitl`)*
3. **What to Observe on Screen:**
   - In the **Mission Control panel**, the state machine transitions: `IDLE` → `TAKEOFF` → `COVERAGE_SURVEY`.
   - The cyan dashed planned path overlays the flood polygon (600m x 600m) with 26 boustrophedon sweep waypoints.
   - The yellow drone position marker begins traversing the coverage pattern, leaving a live breadcrumb track.
4. **Speaker Script:**
   > *"Khoj-o-Drone computes optimal boustrophedon sweeps aligned to the polygon's principal axis to minimize turns. As the drone scans, camera telemetry and sensor streams feed the perception engine."*

---

### 1:00 – 2:00: Multi-Modal Detection & Geotagging
1. **Observe the Live Feeds & Map:**
   - The top-right **Camera Feed panel** displays active detections with bounding boxes.
   - Click the **LWIR Thermal tab** to view infrared modality confirmation.
   - Red pins appear on the Leaflet map denoting identified survivors, surrounded by dotted circles representing geotag uncertainty radii ($\pm 3.5\text{m}$).
   - Orange hazard zones render around active flood waters with safe buffer boundaries.
2. **Speaker Script:**
   > *"Notice the dual-modality registration: when a marooned survivor is detected in visible light, the thermal sensor confirms body heat (~37°C). The pinhole ray-casting engine projects 2D pixels directly to WGS-84 coordinates on the ground."*

---

### 2:00 – 3:00: Adaptive Search Confirmation Orbit & Safe Route
1. **Observe the Drone Orbit:**
   - When a high-confidence survivor ($\ge 0.50$) is identified, the drone automatically transitions state: `COVERAGE_SURVEY` → `SURVIVOR_CONFIRMATION`.
   - The drone loiters in an 8-waypoint circular orbit at $15\text{m}$ radius around the survivor to capture multi-angle visual confirmation, then resumes the sweep track.
2. **Observe the Green Safe Routes:**
   - The map renders green dashed safe rescue paths from the forward staging area to trapped survivors.
   - These paths are calculated via 8-connected A* search strictly avoiding all hazard buffer zones.
3. **Speaker Script:**
   > *"Rather than flying past, the drone autonomously loiters in an inspection orbit to confirm survivor status and assess immediate risks. Simultaneously, the A* pathfinder computes clear, hazard-free ground rescue routes for first responders."*

---

### 3:00 – 4:00: Autonomous Store-and-Forward Chaos Resilience
1. **Trigger Connectivity Outage:**
   - In the dev control panel above the map, click **"Simulate offline"** (or in terminal: `curl -X POST http://localhost:8002/debug/set-offline -d '{"offline":true}' -H "Content-Type: application/json"`).
2. **What to Observe:**
   - The top banner flashes amber: **"Autonomous Offline Mode (Store-and-Forward Active)"**.
   - The queue depth counter begins incrementing in real time (`Queue Depth: 12 items... 24 items`).
   - Detections and alerts continue to be registered without interruption into the local SQLite WAL write-ahead log.
3. **Restore Connection:**
   - Click **"Go online"** (or pass `{"offline":false}`).
   - The banner returns to green: **"Live"**.
   - The background drain loop automatically syncs all buffered events to Postgres with Last-Write-Wins deduplication.
4. **Speaker Script:**
   > *"Network loss in disaster zones is guaranteed. Khoj-o-Drone never drops a detection: every event is committed to a local write-ahead SQLite database first. When connectivity recovers, the system drains with zero data loss."*

---

### 4:00 – 5:00: Entity Triage, SitRep Generation & Replay
1. **Entity Detail & Triage:**
   - Click any survivor pin on the map.
   - The **Entity Detail Drawer** slides open from the right, showing: coordinates, $\pm 3.8\text{m}$ uncertainty, detection modality badges (`RGB`, `THERMAL`), observation timeline, and nearest hazards ($142\text{m}$ from flood zone).
   - Click **"Mark Survivor Rescued"** or acknowledge the alert in the **Alert Feed**.
2. **Download Situational Report (SitRep):**
   - Click the **"SitRep (HTML)"** button in the dashboard header.
   - A printable, formatted incident report opens with summary metrics, ranked rescue recommendations, and embedded vector SVG maps.
3. **Mission Replay:**
   - Toggle **"Replay Mode"** below the map.
   - Drag the time scrubber to replay the chronological progression of detections and drone movements.
4. **Speaker Script:**
   > *"Finally, incident commanders get an actionable SitRep with ranked rescue priorities and can scrub through the complete mission timeline to review every operational event."*

---

## Clean Shutdown

```bash
# Stop simulation or demo:
make down
```
