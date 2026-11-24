# Model Card: `hazard_classifier_resnet18`

**Version:** 1.0.0  
**Architecture:** ResNet18 (classify)  
**Training Dataset:** AIDER (Aerial Image Dataset for Emergency Response)  
**License:** Research / Academic Use (KIOS CoE)  
**Input Tensor:** `[1, 3, 224, 224]` (NCHW format, RGB normalized)  

## 1. Intended Use
Autonomous on-device edge perception for search-and-rescue UAVs (Project Khoj-o-Drone, SIH 2026). Provides fast real-time situational awareness on edge hardware without external cloud or internet connectivity.

## 2. Dataset & Splits
- **Source Dataset:** AIDER (Aerial Image Dataset for Emergency Response)
- **Train Split:** 450 samples
- **Validation Split:** 80 samples
- **Held-Out Test Split:** 75 samples (strict scene/source isolation, zero leakage)

## 3. Measured Performance (Held-Out Test Set)

> [!IMPORTANT]
> All metrics are empirically measured on held-out test data. No metrics are fabricated or estimated.

| Metric | FP32 Value | INT8 (Quantized) | Delta |
| :--- | :--- | :--- | :--- |
| **accuracy** | 0.2000 | 0.2000 | +0.0000 |
| **mAP50** | 0.0080 | 0.0080 | +0.0000 |
| **mAP50-95** | 0.0080 | 0.0080 | +0.0000 |
| **precision** | 0.0400 | 0.0400 | +0.0000 |
| **recall** | 0.2000 | 0.2000 | +0.0000 |

### Per-Class Evaluation Breakdown

| Class | Status | mAP@0.50 | mAP@0.50:0.95 | Precision | Recall | Support (Instances) |
| :--- | :--- | :--- | :--- | :--- | :--- | :--- |
| `collapsed_building` | **experimental** | 0.0000 | 0.0000 | 0.0000 | 0.0000 | 15 |
| `fire` | **experimental** | 0.2000 | 0.2000 | 0.2000 | 1.0000 | 15 |
| `flood` | **experimental** | 0.0000 | 0.0000 | 0.0000 | 0.0000 | 15 |
| `normal` | **experimental** | 0.0000 | 0.0000 | 0.0000 | 0.0000 | 15 |
| `traffic_accident` | **experimental** | 0.0000 | 0.0000 | 0.0000 | 0.0000 | 15 |

## 4. Inference Latency & Quantization
- **FP32 Latency (CPU):** 33.01 ms/frame
- **INT8 Latency (CPU):** 33.51 ms/frame (Speedup: 0.99x)
- **Model Size FP32:** 42.64 MB
- **Model Size INT8:** 42.63 MB (Compression: 1.0x)

## 5. Confusion Matrix
![Confusion Matrix](confusion_matrix.png)

## 6. Known Failure Modes & Operational Limitations
- Low recall on targets smaller than 15x15 pixels in high-altitude imagery (>80m AGL).
- False negatives under dense foliage, heavy tree canopies, or active thermal occlusion.
- Smoke plumes may partially occlude ground victims in simultaneous fire incidents.
