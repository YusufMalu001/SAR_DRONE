import os
from typing import List


class Settings:
    service_name: str = "inference-service"
    host: str = os.getenv("HOST", "0.0.0.0")
    port: int = int(os.getenv("PORT", "8001"))
    redis_url: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")

    # Redis streams (XADD/XREAD) carrying Detection / Telemetry JSON payloads,
    # matching shared/jsonschema/{detection,telemetry}.schema.json.
    detections_channel: str = os.getenv("DETECTIONS_CHANNEL", "detections")
    telemetry_channel: str = os.getenv("TELEMETRY_CHANNEL", "telemetry")
    stream_maxlen: int = int(os.getenv("STREAM_MAXLEN", "10000"))

    # ONNX Runtime Execution Provider: 'cpu' | 'cuda' | 'tensorrt'
    execution_provider: str = os.getenv("EXECUTION_PROVIDER", "cpu").lower()
    ort_intra_op_threads: int = int(os.getenv("ORT_INTRA_OP_THREADS", "4"))
    ort_inter_op_threads: int = int(os.getenv("ORT_INTER_OP_THREADS", "1"))

    # Model weights. Defaults assume the docker-compose `./data:/data` mount;
    # override via env vars for non-Docker local runs (e.g. ../data/models/...).
    person_model_path: str = os.getenv("PERSON_MODEL_PATH", "/data/models/person_yolov8.onnx")
    thermal_model_path: str = os.getenv("THERMAL_MODEL_PATH", "/data/models/thermal_yolov8.onnx")
    hazard_model_path: str = os.getenv("HAZARD_MODEL_PATH", "/data/models/hazard_classifier.onnx")

    person_conf_threshold: float = float(os.getenv("PERSON_CONF_THRESHOLD", "0.4"))
    thermal_conf_threshold: float = float(os.getenv("THERMAL_CONF_THRESHOLD", "0.4"))
    hazard_conf_threshold: float = float(os.getenv("HAZARD_CONF_THRESHOLD", "0.5"))
    iou_threshold: float = float(os.getenv("IOU_THRESHOLD", "0.45"))

    # SAHI-style tiled inference configuration
    enable_tiling: bool = os.getenv("ENABLE_TILING", "true").lower() in ("true", "1", "yes")
    tile_size: int = int(os.getenv("TILE_SIZE", "640"))
    tile_overlap: float = float(os.getenv("TILE_OVERLAP", "0.20"))

    # Tracking parameters
    track_high_thresh: float = float(os.getenv("TRACK_HIGH_THRESH", "0.5"))
    track_low_thresh: float = float(os.getenv("TRACK_LOW_THRESH", "0.2"))
    track_max_age: int = int(os.getenv("TRACK_MAX_AGE", "15"))
    track_min_hits: int = int(os.getenv("TRACK_MIN_HITS", "2"))

    # Cross-modal RGB-Thermal fusion
    fusion_iou_threshold: float = float(os.getenv("FUSION_IOU_THRESHOLD", "0.30"))
    rgb_thermal_homography_path: str = os.getenv(
        "RGB_THERMAL_HOMOGRAPHY_PATH", "data/calibration/rgb_thermal_homography.json"
    )

    # Robustness and bounded queue backpressure
    queue_max_size: int = int(os.getenv("QUEUE_MAX_SIZE", "10"))
    frame_deadline_ms: float = float(os.getenv("FRAME_DEADLINE_MS", "200.0"))

    # Video ingestion / telemetry replay.
    video_path: str = os.getenv("VIDEO_PATH", "/data/sample_footage/demo_flight.mp4")
    telemetry_csv_path: str = os.getenv("TELEMETRY_CSV_PATH", "/data/sample_footage/demo_telemetry.csv")
    target_fps: float = float(os.getenv("TARGET_FPS", "5"))
    telemetry_sample_rate_hz: float = float(os.getenv("TELEMETRY_SAMPLE_RATE_HZ", "5"))
    playback_speed: float = float(os.getenv("PLAYBACK_SPEED", "1.0"))

    def get_onnx_providers(self) -> List[str]:
        ep = self.execution_provider
        if ep == "cuda":
            return ["CUDAExecutionProvider", "CPUExecutionProvider"]
        elif ep == "tensorrt":
            return ["TensorrtExecutionProvider", "CUDAExecutionProvider", "CPUExecutionProvider"]
        return ["CPUExecutionProvider"]


settings = Settings()
