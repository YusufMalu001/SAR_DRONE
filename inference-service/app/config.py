import os


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

    # Model weights. Defaults assume the docker-compose `./data:/data` mount;
    # override via env vars for non-Docker local runs (e.g. ../data/models/...).
    model_path: str = os.getenv("MODEL_PATH", "/data/models/placeholder.onnx")
    person_model_path: str = os.getenv("PERSON_MODEL_PATH", "/data/models/person_yolov8.onnx")
    thermal_model_path: str = os.getenv("THERMAL_MODEL_PATH", "/data/models/thermal_yolov8.onnx")
    hazard_model_path: str = os.getenv("HAZARD_MODEL_PATH", "/data/models/hazard_classifier.onnx")

    person_conf_threshold: float = float(os.getenv("PERSON_CONF_THRESHOLD", "0.4"))
    thermal_conf_threshold: float = float(os.getenv("THERMAL_CONF_THRESHOLD", "0.4"))
    hazard_conf_threshold: float = float(os.getenv("HAZARD_CONF_THRESHOLD", "0.5"))
    iou_threshold: float = float(os.getenv("IOU_THRESHOLD", "0.45"))

    # Video ingestion / telemetry replay.
    video_path: str = os.getenv("VIDEO_PATH", "/data/sample_footage/demo_flight.mp4")
    telemetry_csv_path: str = os.getenv("TELEMETRY_CSV_PATH", "/data/sample_footage/demo_telemetry.csv")
    target_fps: float = float(os.getenv("TARGET_FPS", "5"))
    telemetry_sample_rate_hz: float = float(os.getenv("TELEMETRY_SAMPLE_RATE_HZ", "5"))
    playback_speed: float = float(os.getenv("PLAYBACK_SPEED", "1.0"))


settings = Settings()
