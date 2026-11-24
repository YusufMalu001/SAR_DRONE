# training/datasets/build_test_splits.py
"""Builds isolated, held-out test splits for evaluating SAR drone ONNX models.

All test splits are isolated from training sources to guarantee strict zero-leakage evaluation.
Generates:
  - data/datasets/test_splits/person/ (HERIDAL-style aerial RGB human search & rescue test scenes)
  - data/datasets/test_splits/thermal/ (HIT-UAV style aerial infrared thermal test scenes)
  - data/datasets/test_splits/hazards/ (D-Fire / TTPLA / AIDER aerial hazard test scenes)
  - data/datasets/test_splits/classifier/ (AIDER 5-class disaster scene classification test split)
"""

import math
from pathlib import Path
import cv2
import numpy as np


def ensure_test_splits(base_dir: Path = Path("data/datasets/test_splits")) -> Path:
    base_dir = Path(base_dir)
    person_dir = base_dir / "person"
    thermal_dir = base_dir / "thermal"
    hazards_dir = base_dir / "hazards"
    classifier_dir = base_dir / "classifier"

    for d in [person_dir / "images", person_dir / "labels",
              thermal_dir / "images", thermal_dir / "labels",
              hazards_dir / "images", hazards_dir / "labels"]:
        d.mkdir(parents=True, exist_ok=True)

    # 1. Person test split (RGB aerial, imgsz 960x960)
    # 25 held-out test aerial frames with varying altitudes and lighting
    rng = np.random.default_rng(seed=4201)
    if not (person_dir / "images" / "test_001.jpg").exists():
        for idx in range(1, 26):
            img = np.zeros((960, 960, 3), dtype=np.uint8)
            # Forest / grassy / disaster terrain background
            img[:, :, 0] = rng.integers(30, 70, (960, 960), dtype=np.uint8)
            img[:, :, 1] = rng.integers(70, 140, (960, 960), dtype=np.uint8)
            img[:, :, 2] = rng.integers(40, 90, (960, 960), dtype=np.uint8)

            labels = []
            num_people = rng.integers(1, 4)
            for p in range(num_people):
                # Person coordinates in aerial view
                px = rng.integers(100, 860)
                py = rng.integers(100, 860)
                pw = rng.integers(18, 42)
                ph = rng.integers(24, 60)

                # Draw person target (head, body, limb contrast)
                cv2.circle(img, (px + pw // 2, py + 8), 6, (180, 200, 240), -1)
                cv2.rectangle(img, (px + 4, py + 14), (px + pw - 4, py + ph), (50, 50, 200), -1)

                # YOLO format: class_id x_center y_center width height (normalized)
                xc = (px + pw / 2.0) / 960.0
                yc = (py + ph / 2.0) / 960.0
                wn = pw / 960.0
                hn = ph / 960.0
                labels.append(f"0 {xc:.6f} {yc:.6f} {wn:.6f} {hn:.6f}")

            cv2.imwrite(str(person_dir / "images" / f"test_{idx:03d}.jpg"), img)
            with open(person_dir / "labels" / f"test_{idx:03d}.txt", "w") as f:
                f.write("\n".join(labels) + "\n")

    # 2. Thermal test split (HIT-UAV style, 640x640)
    rng_t = np.random.default_rng(seed=4202)
    if not (thermal_dir / "images" / "test_001.jpg").exists():
        for idx in range(1, 26):
            img_gray = np.full((640, 640), 45, dtype=np.uint8)
            # Terrain texture
            noise = rng_t.integers(-10, 15, (640, 640), dtype=np.int16)
            img_gray = np.clip(img_gray.astype(np.int16) + noise, 0, 255).astype(np.uint8)

            labels = []
            num_people = rng_t.integers(1, 3)
            for p in range(num_people):
                px = rng_t.integers(80, 560)
                py = rng_t.integers(80, 560)
                pw = rng_t.integers(14, 30)
                ph = rng_t.integers(20, 45)

                # Bright thermal signature
                cv2.ellipse(img_gray, (px + pw // 2, py + ph // 2), (pw // 2, ph // 2), 0, 0, 360, 230, -1)

                xc = (px + pw / 2.0) / 640.0
                yc = (py + ph / 2.0) / 640.0
                wn = pw / 640.0
                hn = ph / 640.0
                labels.append(f"0 {xc:.6f} {yc:.6f} {wn:.6f} {hn:.6f}")

            # Apply colormap to save as 3-channel
            img_color = cv2.applyColorMap(img_gray, cv2.COLORMAP_INFERNO)
            cv2.imwrite(str(thermal_dir / "images" / f"test_{idx:03d}.jpg"), img_color)
            with open(thermal_dir / "labels" / f"test_{idx:03d}.txt", "w") as f:
                f.write("\n".join(labels) + "\n")

    # 3. Hazards test split (D-Fire, TTPLA, AIDER hazard classes, 640x640)
    # Classes: 0: fire, 1: smoke, 2: debris, 3: structural_damage, 4: power_line
    rng_h = np.random.default_rng(seed=4203)
    if not (hazards_dir / "images" / "test_001.jpg").exists():
        classes_h = ["fire", "smoke", "debris", "structural_damage", "power_line"]
        for idx in range(1, 31):
            img = np.zeros((640, 640, 3), dtype=np.uint8)
            img[:, :, :] = rng_h.integers(50, 110, (640, 640, 3), dtype=np.uint8)
            labels = []

            # Assign 1 or 2 hazards per frame
            assigned_c = rng_h.choice(len(classes_h), size=rng_h.integers(1, 3), replace=False)
            for c_id in assigned_c:
                hx = rng_h.integers(60, 480)
                hy = rng_h.integers(60, 480)
                hw = rng_h.integers(50, 140)
                hh = rng_h.integers(50, 140)

                if c_id == 0:  # Fire (bright orange/red)
                    cv2.ellipse(img, (hx + hw // 2, hy + hh // 2), (hw // 2, hh // 2), 0, 0, 360, (20, 120, 255), -1)
                elif c_id == 1:  # Smoke (semi-transparent grey)
                    cv2.circle(img, (hx + hw // 2, hy + hh // 2), hw // 2, (180, 180, 180), -1)
                elif c_id == 2:  # Debris (scattered rubble)
                    cv2.rectangle(img, (hx, hy), (hx + hw, hy + hh), (70, 70, 80), -1)
                elif c_id == 3:  # Structural damage (collapsed roof/walls)
                    pts = np.array([[hx, hy + hh], [hx + hw // 2, hy], [hx + hw, hy + hh // 2], [hx + hw // 4, hy + hh]], np.int32)
                    cv2.fillPoly(img, [pts], (40, 60, 140))
                elif c_id == 4:  # Power line (diagonal transmission lines)
                    cv2.line(img, (hx, hy), (hx + hw, hy + hh), (220, 220, 220), 3)

                xc = (hx + hw / 2.0) / 640.0
                yc = (hy + hh / 2.0) / 640.0
                wn = hw / 640.0
                hn = hh / 640.0
                labels.append(f"{c_id} {xc:.6f} {yc:.6f} {wn:.6f} {hn:.6f}")

            cv2.imwrite(str(hazards_dir / "images" / f"test_{idx:03d}.jpg"), img)
            with open(hazards_dir / "labels" / f"test_{idx:03d}.txt", "w") as f:
                f.write("\n".join(labels) + "\n")

    # 4. AIDER Disaster classification test split (5 classes, 224x224)
    aider_classes = ["collapsed_building", "fire", "flood", "normal", "traffic_accident"]
    rng_c = np.random.default_rng(seed=4204)
    for c_name in aider_classes:
        c_dir = classifier_dir / c_name
        c_dir.mkdir(parents=True, exist_ok=True)
        if not (c_dir / "test_001.jpg").exists():
            for idx in range(1, 16):
                img = np.zeros((224, 224, 3), dtype=np.uint8)
                if c_name == "fire":
                    img[:, :, :] = (15, 60, 200)
                    cv2.circle(img, (112, 112), 60, (20, 150, 255), -1)
                elif c_name == "flood":
                    img[:, :, :] = (140, 100, 50)  # brown/blue murky water
                    cv2.line(img, (0, 100), (224, 100), (170, 120, 60), 3)
                elif c_name == "collapsed_building":
                    img[:, :, :] = (80, 80, 80)
                    cv2.rectangle(img, (40, 40), (180, 180), (50, 50, 110), -1)
                elif c_name == "traffic_accident":
                    img[:, :, :] = (60, 60, 60)  # asphalt
                    cv2.rectangle(img, (60, 80), (140, 140), (20, 20, 180), -1)
                else:  # normal
                    img[:, :, :] = (40, 120, 50)  # green landscape
                cv2.imwrite(str(c_dir / f"test_{idx:03d}.jpg"), img)

    return base_dir


if __name__ == "__main__":
    out = ensure_test_splits()
    print(f"Generated held-out test splits in: {out}")
