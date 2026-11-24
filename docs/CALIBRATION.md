# Dual-Sensor (RGB & Thermal LWIR) Camera Calibration Procedure

This document specifies the offline and on-field geometric registration procedure to calibrate the spatial homography between the drone's visible-light (RGB) camera and long-wave infrared (LWIR) thermal camera.

---

## 1. Physical Calibration Target

Because standard printed checkerboards do not provide adequate thermal contrast, use one of the following dual-spectrum targets:
1. **Heated Checkerboard (Recommended):** A matte-black and polished aluminum checkerboard heated from the rear by a thermal heating pad (maintaining target temperature at ~45°C, yielding high emissivity difference $\Delta\epsilon > 0.85$).
2. **Backlit Cutout Mask:** Laser-cut acrylic matrix with resistive heating wires embedded behind.
3. **Grid Dimensions:** 9x6 inner corners with 30mm square edge size.

---

## 2. Data Acquisition
1. Fix both RGB and LWIR cameras on the drone payload gimbal at operating standoff distance (1.5m to 4.0m).
2. Ensure both cameras are focused at infinity (or SAR operating hyperfocal distance).
3. Capture at least 25 synchronized image pairs across diverse tilt angles ($\pm 30^\circ$), yaw rotations, and frame positions.

---

## 3. Mathematical Registration & Homography Estimation

The spatial transformation from thermal coordinates $\mathbf{x}_{\text{thermal}} = (x_t, y_t, 1)^T$ to RGB coordinates $\mathbf{x}_{\text{rgb}} = (x_{rgb}, y_{rgb}, 1)^T$ is modeled by a planar projective homography $\mathbf{H} \in \mathbb{R}^{3\times 3}$:

$$\mathbf{x}_{\text{rgb}} \sim \mathbf{H} \, \mathbf{x}_{\text{thermal}}$$

### OpenCV Calibration Script
```python
import cv2
import json
import numpy as np

# Detect checkerboard corners in both modalities
ret_rgb, corners_rgb = cv2.findChessboardCorners(rgb_img, (9, 6), None)
ret_th, corners_th = cv2.findChessboardCorners(thermal_img, (9, 6), None)

if ret_rgb and ret_th:
    # Sub-pixel corner refinement
    corners_rgb = cv2.cornerSubPix(rgb_gray, corners_rgb, (11, 11), (-1, -1), criteria)
    corners_th = cv2.cornerSubPix(thermal_gray, corners_th, (11, 11), (-1, -1), criteria)

    # Compute planar homography with RANSAC outlier rejection
    H, mask = cv2.findHomography(corners_th, corners_rgb, cv2.RANSAC, 3.0)

    # Save to calibration config
    calibration_data = {
        "homography_matrix": H.tolist(),
        "mean_reprojection_error_px": float(mean_error),
    }
    with open("data/calibration/rgb_thermal_homography.json", "w") as f:
        json.dump(calibration_data, f, indent=2)
```

---

## 4. Bounding Box Warping & Cross-Modal Fusion
During real-time inference:
1. For every thermal detection bounding box $[x, y, w, h]$, the 4 corners are transformed through $\mathbf{H}$.
2. The axis-aligned bounding box enclosing the warped corners is evaluated against RGB bounding boxes via Intersection-over-Union (IoU).
3. If $\text{IoU} \ge 0.30$, both modalities agree:
   - Confidence is boosted: $c_{\text{fused}} = \min(0.99, \max(c_{\text{rgb}}, c_{\text{thermal}}) + 0.15 \cdot \min(c_{\text{rgb}}, c_{\text{thermal}}))$.
   - Modalities array is stamped: `["rgb", "thermal"]`.
4. Thermal-only detections (e.g. survivors concealed in smoke or dense foliage) remain valid without matching RGB counterparts and emit `modalities: ["thermal"]`.
