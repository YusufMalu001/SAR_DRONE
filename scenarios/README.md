# Simulation Scenarios (`scenarios/`)

This directory contains deterministic simulation scenario definitions for end-to-end evaluation and demonstration of Khoj-o-Drone without requiring a physical UAV.

## Scenario Schema

Each scenario is defined in YAML format with the following specification:

| Field | Description |
| :--- | :--- |
| `name` | Unique scenario identifier (e.g. `flood`, `fire`, `collapse`). |
| `description` | Situational summary of the disaster context and targets. |
| `area_polygon` | Bounding operational polygon as an ordered list of `{lat, lon}` vertices. |
| `home_point` | UAV launch and recovery coordinate `{lat, lon, alt}`. |
| `flight` | Flight configuration parameters: `altitude` (meters AGL), `speed` (m/s), `sensor_rate_hz` (telemetry frequency), `seed` (fixed PRNG seed for deterministic noise), and noise amplitudes (`pos_std_m`, `alt_std_m`, `heading_std_deg`, `attitude_std_deg`). |
| `camera` | Camera optical properties: `hfov_deg`, `vfov_deg`, and `fps`. |
| `video` | Video stream sources: `rgb_path`, `thermal_path`, and `thermal_pipeline` config. |
| `ground_truth_objects` | Catalog of ground-truth entities with true `{id, type, lat, lon}`, frame intervals `appearance_frames: [start, end]`, normalized bounding boxes `bbox: {x, y, w, h}`, and expected detection confidence. |
| `connectivity_events` | Scheduled communication link disruptions `{type: link_down, t_start_s, t_end_s, action}` for testing offline buffering and recovery. |
| `gps_outages` | Time windows `[t_start_s, t_end_s]` during which GPS signal acquisition is lost. |

## Available Scenarios

1. **`flood.yaml`**: Inundation scenario featuring submerged residential structures, flood water hazard zone, and stranded survivors stranded on a rooftop.
2. **`fire.yaml`**: Wildfire emergency scenario featuring active flame fronts, dense smoke plumes, and a civilian fleeing towards safety.
3. **`collapse.yaml`**: Urban structural failure with pancaked building ruins, road debris with downed high-voltage power lines, and a trapped survivor.

## Ground-Truth Artifacts

Each scenario pairs with a precomputed or exported ground-truth file:
- `flood_ground_truth.json`
- `fire_ground_truth.json`
- `collapse_ground_truth.json`

These files record exact target coordinates, expected frame detections, and projection parameters to allow deterministic geotagging error computation.
