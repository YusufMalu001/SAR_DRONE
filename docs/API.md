# Khoj-o-Drone API Reference

This document provides a comprehensive REST and WebSocket API specification for the Khoj-o-Drone autonomous search-and-rescue platform (`fusion-service`, `mission-service`, and `inference-service`).

---

## 1. Fusion Service API (`http://localhost:8002`)

### 1.1 Health & Connectivity
- **`GET /health`**
  - **Summary:** Returns operational mode (`online`, `offline`, `degraded`), database/redis status, local queue depth, and last sync timestamp.
  - **Response (200 OK):**
    ```json
    {
      "status": "ok",
      "service": "fusion-service",
      "mode": "online",
      "postgres": "connected",
      "redis": "connected",
      "queue_depth": 0,
      "oldest_item_age_seconds": 0.0,
      "last_successful_sync": "2026-11-24T12:00:00Z"
    }
    ```

### 1.2 Entities (Survivors & Hazards)
- **`GET /entities`**
  - **Query Params:** `type` (`survivor` | `hazard`), `status` (`active` | `rescued` | `cleared`)
  - **Summary:** Lists spatially clustered entities with priority scores, uncertainty radii, and observation counts.
  - **Response (200 OK):** Array of `EntityRecord`.
- **`GET /entities/{id}`**
  - **Summary:** Retrieves details and track history for a specific entity.
- **`POST /entities/{id}/resolve`**
  - **Body:** `{"status": "rescued" | "cleared"}`
  - **Summary:** Transitions entity lifecycle status and deactivates linked active alerts.

### 1.3 Hazard Polygons & Safe Routes
- **`GET /hazards/geojson`** (also `GET /hazards/zones`)
  - **Query Params:** `buffer_meters` (optional override)
  - **Summary:** Returns GeoJSON FeatureCollection of hazard polygon boundaries with safety buffers.
- **`GET /mission/routes/safe`**
  - **Query Params:** `start_lat`, `start_lon`, `dest_lat`, `dest_lon`
  - **Summary:** Computes an 8-connected A* obstacle-free path avoiding hazard buffer polygons.
- **`GET /mission/routes/survivors`**
  - **Summary:** Returns pre-planned safe rescue routes to all active survivors.

### 1.4 Coverage Map & Offline Tiles
- **`GET /mission/coverage`**
  - **Summary:** Returns GeoJSON FeatureCollection of discrete spatial grid cells colored by coverage pass counts.
- **`GET /mission/coverage/summary`**
  - **Summary:** Returns overall coverage metrics: total cells, visited cells, coverage fraction, and area ($m^2$).
- **`GET /tiles/{z}/{x}/{y}.png`**
  - **Summary:** Serves pre-cached offline raster map tiles directly from local disk storage.

### 1.5 Situational Reports (SitRep)
- **`GET /mission/report`** (also `GET /missions/{id}/report`)
  - **Query Params:** `format` (`html` | `json`)
  - **Summary:** Generates printable HTML or raw JSON situational report featuring summary statistics, priority-ranked rescue recommendations, and self-contained vector maps.

### 1.6 Alerts
- **`GET /alerts`**
  - **Query Params:** `limit`, `state` (`active` | `acknowledged` | `resolved`)
  - **Summary:** Returns chronologically ordered emergency dispatch alerts.
- **`POST /alerts/{id}/ack`**
  - **Summary:** Acknowledges an active alert.
- **`POST /alerts/{id}/resolve`**
  - **Summary:** Resolves an alert.

### 1.7 Live WebSocket Feed
- **`WS /ws/live`**
  - **Summary:** Real-time bidirectional event stream pushing `detection`, `alert`, and `telemetry` JSON envelopes.
  - **Message Envelope:**
    ```json
    {
      "type": "detection",
      "payload": {
        "id": "det-1234",
        "class": "person",
        "confidence": 0.94,
        "lat": 20.0015,
        "lon": 78.0025,
        "priority_score": 0.88,
        "status": "live",
        "modalities": ["rgb", "thermal"]
      }
    }
    ```

---

## 2. Mission Control Service API (`http://localhost:8003`)

- **`GET /health`**
  - **Summary:** Verifies service health and MAVLink connection to flight controller or SITL.
- **`GET /mission/status`**
  - **Summary:** Returns current flight state machine status (`IDLE`, `TAKEOFF`, `COVERAGE_SURVEY`, `SURVIVOR_CONFIRMATION`, `PAUSED`, `RTL`, `LANDED`), armed state, battery percentage, and active waypoint index.
- **`POST /mission/plan`**
  - **Body:**
    ```json
    {
      "polygon": [{"lat": 20.0, "lon": 78.0}, {"lat": 20.005, "lon": 78.0}, {"lat": 20.005, "lon": 78.005}, {"lat": 20.0, "lon": 78.005}],
      "altitude_m": 50.0,
      "speed_mps": 5.0,
      "overlap_lateral": 0.30,
      "overlap_forward": 0.40
    }
    ```
  - **Summary:** Generates optimal boustrophedon sweep waypoints for the search polygon and calculates area coverage fraction.
- **`POST /mission/start`**: Starts takeoff and autonomous coverage execution.
- **`POST /mission/pause`**: Loiters in place at current waypoint.
- **`POST /mission/resume`**: Resumes execution of coverage path.
- **`POST /mission/abort`**: Commands immediate Return-to-Launch (RTL).

---

## 3. Inference Service API (`http://localhost:8001`)

- **`GET /health`**
  - **Summary:** Reports model load status, ONNX Runtime session provider (CPU/CUDA), and Redis stream connectivity.
- **`POST /detect`**
  - **Body:** Multipart image or base64 frame payload.
  - **Summary:** Executes sliding-window tiled detection and returns bounding boxes, class labels, confidences, and stage timings.
