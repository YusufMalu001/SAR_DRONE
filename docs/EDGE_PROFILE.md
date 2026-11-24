# Edge Profile & Hardware Acceleration Guide

This document defines the hardware execution profiles, acceleration configurations, and optimization benchmarks for running Khoj-o-Drone's `inference-service` on UAV onboard edge compute hardware.

---

## 1. Supported Execution Providers & Configurations

Khoj-o-Drone uses `onnxruntime` as its unified inference engine. The runtime provider is selectable via the `EXECUTION_PROVIDER` environment variable:

| Profile Name | `EXECUTION_PROVIDER` | Active Providers | Typical Target Hardware | Target FPS |
| :--- | :--- | :--- | :--- | :--- |
| **CPU Baseline** (Default) | `cpu` | `CPUExecutionProvider` | Intel/AMD x86, Raspberry Pi 5, Standard Linux SBC | 5 – 15 FPS |
| **NVIDIA CUDA** | `cuda` | `CUDAExecutionProvider`, `CPUExecutionProvider` | Jetson Nano, Xavier NX, Orin Nano, Laptop GPU | 25 – 45 FPS |
| **NVIDIA TensorRT** | `tensorrt` | `TensorrtExecutionProvider`, `CUDAExecutionProvider` | Jetson Orin Nano, AGX Orin, RTX Embedded | 40 – 75+ FPS |

### Runtime Environment Variables
```bash
# Select execution provider
export EXECUTION_PROVIDER=tensorrt   # or 'cuda', 'cpu'

# CPU threading controls (recommended: match physical performance cores)
export ORT_INTRA_OP_THREADS=4
export ORT_INTER_OP_THREADS=1

# Tiling & Frame Rate Controls
export ENABLE_TILING=true
export TILE_SIZE=640
export TILE_OVERLAP=0.20
export FRAME_DEADLINE_MS=200.0
```

---

## 2. Edge Hardware Profiles

### Profile A: NVIDIA Jetson Orin Nano (Recommended Production Edge)
- **SoC:** 6-core Arm Cortex-A78AE v8.2, 1024-core NVIDIA Ampere GPU with 32 Tensor Cores.
- **Memory:** 8 GB LPDDR5 (shared unified memory, 68 GB/s bandwidth).
- **Power Envelope:** 7W – 15W configurable operating modes.
- **Recommended Setup:**
  - `EXECUTION_PROVIDER=tensorrt`
  - TensorRT engine build with FP16 precision:
    ```bash
    trtexec --onnx=data/models/person_yolov8.onnx \
            --saveEngine=data/models/person_yolov8_fp16.engine \
            --fp16 --workspace=2048
    ```
  - Achieved Latency: ~12ms per 640x640 frame patch, ~22ms for full tiled pair (RGB + Thermal).

### Profile B: Raspberry Pi 5 + Hailo-8L / Coral Edge TPU (Low-SWaP Tier)
- **SoC:** Quad-core Arm Cortex-A76 @ 2.4 GHz, 8 GB LPDDR4X.
- **Power Envelope:** 5W – 12W.
- **Recommended Setup:**
  - `EXECUTION_PROVIDER=cpu`
  - Quantized INT8 ONNX models:
    ```python
    import onnxruntime.quantization as oq
    oq.quantize_dynamic("data/models/person_yolov8.onnx", "data/models/person_yolov8_int8.onnx")
    ```
  - Thread settings: `ORT_INTRA_OP_THREADS=4`, `ORT_INTER_OP_THREADS=1`.
  - Achieved Latency: ~45ms per frame on CPU with INT8.

---

## 3. Threading, Memory & Backpressure Tuning
1. **Intra-op parallelism:** For edge devices with 4–8 cores, setting `ORT_INTRA_OP_THREADS` to the count of physical CPU cores prevents thread thrashing while keeping CPU usage under 80%.
2. **Unified Memory Management:** On Jetson devices, page locking and pin-memory are handled within the CUDA runtime. Bounded input queues (`QUEUE_MAX_SIZE=10`) avoid accumulating memory spikes when frame ingress temporarily exceeds downstream processing rates.
3. **Drop-Oldest Policy:** If a frame pair exceeds `FRAME_DEADLINE_MS` or the bounded queue fills, older frames are dropped immediately to preserve near-real-time freshness for emergency search-and-rescue situational awareness.
