# Empirical System Evaluation Results

*Generated automatically by `make evaluate` on 2026-11-24 16:27:36 UTC. Do not edit by hand.*

---

## 1. GPS-Denied Navigation (Visual-Inertial Odometry)

Empirically measured over a 200m GPS-outage window with IMU bias and optical-flow noise:

- **Travel Distance Evaluated:** 200.0 m
- **Total Cumulative Drift:** 0.67 m
- **Measured Drift Rate:** **0.33 m per 100 m travelled**
- **Status:** Verified (requirement < 5.0m / 100m met)

---

## 2. Autonomous Coverage Path Planning

- **Test Polygon:** Flood scenario bounding box (600m x 600m)
- **Total Waypoints Generated:** 26
- **Planning Runtime:** 3.27 ms
- **Measured Area Coverage Fraction:** **100.00%**
- **Target:** $\ge 95\%$ (Requirement satisfied)

---

## 3. Reactive Obstacle Avoidance (SITL Simulation)

- **Randomized Seeded Maps Tested:** 20
- **Collision Rate:** **0.0%**
- **Mission Goal Success Rate:** **100.0%**
- **Clearance Boundary:** 2.0 m

---

## 4. Perception Models Accuracy & Inference Latency

| Model | Test mAP@0.50 | CPU Latency (ms) |
| :--- | :--- | :--- |
| **`person_yolov8n`** | 0.0000 | 391.51 ms |
| **`thermal_yolov8n`** | 0.0000 | 172.42 ms |
| **`hazard_yolov8n`** | 0.0000 | 167.72 ms |
| **`hazard_classifier_resnet18`** | 0.0080 | 33.01 ms |

---

## 5. Edge Deployment Profile (4 Cores / 8 GB RAM Cap)

- **Platform Cap:** 4.0 cores CPU / 8.0 GB RAM
- **Measured Sustained Throughput:** **284.29 FPS**
- **Measured Peak Memory:** **13.29 MB**
- **Edge Profile Target:** $\ge 10.0$ FPS (Status: PASS)

---

## 6. Verification Command
```bash
make evaluate
```
