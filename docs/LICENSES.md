# Dataset, Model, and Footage Attributions & Licenses

This project incorporates and builds upon open benchmarks, datasets, and flight footage released under respective open-source and academic licenses:

---

## 1. Aerial Datasets & Annotations

| Asset / Dataset Name | Modality | Primary Use Case | Upstream License / Attribution |
| :--- | :--- | :--- | :--- |
| **HERIDAL Search and Rescue Dataset** | Visible RGB | Aerial survivor detection benchmark | Creative Commons Attribution-NonCommercial-ShareAlike 4.0 (CC-BY-NC-SA 4.0) |
| **VisDrone-DET** | Visible RGB | UAV object detection & small target scaling | AISKYEYE Team, Lab of Machine Learning and Data Mining, Tianjin University (Non-commercial research) |
| **HIT-UAV** | LWIR Infrared Thermal | High-altitude UAV thermal human detection | Creative Commons Attribution 4.0 International (CC-BY 4.0) |
| **FLIR Thermal Dataset** | Thermal | LWIR vehicle & pedestrian signatures | Teledyne FLIR (Non-commercial research license) |
| **D-Fire Dataset** | Visible RGB | Fire and smoke boundary detection | MIT License (Pedro Vinícius A. B. Venâncio) |
| **AIDER (Aerial Image Dataset for Emergency Response)** | Visible RGB | Frame-level disaster classification (Fire, Flood, Collapse) | Creative Commons Attribution 4.0 (Zenodo record 3888300) |
| **TTPLA (Transmission Line Dataset)** | Visible RGB | Power line and utility pole hazard mapping | Academic / Open Research Use |

---

## 2. Base Model Architectures & Frameworks

- **Ultralytics YOLOv8:** AGPL-3.0 License. Base architecture definition used for ONNX runtime graph exports.
- **PyTorch & Torchvision (ResNet18):** BSD-3-Clause License.
- **ONNX Runtime:** MIT License. High-performance cross-platform execution engine.
- **ArduPilot SITL:** GNU General Public License v3 (GPLv3).

---

## 3. Sample Footage & Map Tiles

- **Demo Video Footage:** Synthesized from disaster simulation benchmarks and open aerial footage under CC-BY licenses.
- **Offline Map Vector Tiles:** OpenStreetMap contributors (`ODbL` license) rendered via Carto / MapLibre rasterizer.
