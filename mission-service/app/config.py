import os

class MissionConfig:
    PORT: int = int(os.getenv("PORT", "8003"))
    REDIS_URL: str = os.getenv("REDIS_URL", "redis://localhost:6379/0")
    TELEMETRY_CHANNEL: str = os.getenv("TELEMETRY_CHANNEL", "telemetry")
    MAVLINK_CONNECTION: str = os.getenv("MAVLINK_CONNECTION", "udpin:0.0.0.0:14550")
    DEFAULT_ALTITUDE_M: float = float(os.getenv("DEFAULT_ALTITUDE_M", "50.0"))
    DEFAULT_SPEED_MPS: float = float(os.getenv("DEFAULT_SPEED_MPS", "5.0"))
    CAMERA_HFOV_DEG: float = float(os.getenv("CAMERA_HFOV_DEG", "90.0"))
    CAMERA_VFOV_DEG: float = float(os.getenv("CAMERA_VFOV_DEG", "60.0"))
    BATTERY_LOW_THRESHOLD: float = float(os.getenv("BATTERY_LOW_THRESHOLD", "20.0"))
    BATTERY_CRITICAL_THRESHOLD: float = float(os.getenv("BATTERY_CRITICAL_THRESHOLD", "10.0"))
    LINK_LOSS_TIMEOUT_S: float = float(os.getenv("LINK_LOSS_TIMEOUT_S", "5.0"))
