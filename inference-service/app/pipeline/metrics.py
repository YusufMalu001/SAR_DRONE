"""Prometheus-style metrics registry for inference service stages and performance."""

import threading
import time
from typing import Dict, List


class Histogram:
    """Thread-safe Prometheus-style latency histogram."""

    DEFAULT_BUCKETS = (0.002, 0.005, 0.010, 0.025, 0.050, 0.100, 0.200, 0.500, 1.0, 2.5)

    def __init__(self, name: str, help_text: str, label_names: List[str], buckets=DEFAULT_BUCKETS):
        self.name = name
        self.help_text = help_text
        self.label_names = label_names
        self.buckets = sorted(buckets)
        self._lock = threading.Lock()
        # key: tuple of label values -> {"buckets": {le: count}, "sum": float, "count": int}
        self._data: Dict[tuple, dict] = {}

    def observe(self, labels: dict, value: float):
        key = tuple(str(labels.get(k, "")) for k in self.label_names)
        with self._lock:
            if key not in self._data:
                self._data[key] = {
                    "buckets": {le: 0 for le in self.buckets},
                    "sum": 0.0,
                    "count": 0,
                }
            entry = self._data[key]
            entry["sum"] += value
            entry["count"] += 1
            for le in self.buckets:
                if value <= le:
                    entry["buckets"][le] += 1

    def render(self) -> str:
        lines = [f"# HELP {self.name} {self.help_text}", f"# TYPE {self.name} histogram"]
        with self._lock:
            for key, entry in self._data.items():
                label_pairs = [f'{k}="{v}"' for k, v in zip(self.label_names, key)]
                base_labels = ", ".join(label_pairs)

                cum = 0
                for le in self.buckets:
                    cum = entry["buckets"][le]
                    lbl = f'{base_labels}, le="{le}"' if base_labels else f'le="{le}"'
                    lines.append(f"{self.name}_bucket{{{lbl}}} {cum}")

                inf_lbl = f'{base_labels}, le="+Inf"' if base_labels else 'le="+Inf"'
                lines.append(f"{self.name}_bucket{{{inf_lbl}}} {entry['count']}")
                sum_lbl = f"{{{base_labels}}}" if base_labels else ""
                lines.append(f"{self.name}_sum{sum_lbl} {entry['sum']:.6f}")
                lines.append(f"{self.name}_count{sum_lbl} {entry['count']}")
        return "\n".join(lines)


class Counter:
    """Thread-safe Prometheus counter."""

    def __init__(self, name: str, help_text: str, label_names: List[str] = None):
        self.name = name
        self.help_text = help_text
        self.label_names = label_names or []
        self._lock = threading.Lock()
        self._data: Dict[tuple, float] = {}

    def inc(self, labels: dict = None, amount: float = 1.0):
        key = tuple(str((labels or {}).get(k, "")) for k in self.label_names)
        with self._lock:
            self._data[key] = self._data.get(key, 0.0) + amount

    def render(self) -> str:
        lines = [f"# HELP {self.name} {self.help_text}", f"# TYPE {self.name} counter"]
        with self._lock:
            if not self._data and not self.label_names:
                lines.append(f"{self.name} 0")
            for key, val in self._data.items():
                if self.label_names:
                    label_pairs = [f'{k}="{v}"' for k, v in zip(self.label_names, key)]
                    lines.append(f"{self.name}{{{', '.join(label_pairs)}}} {val}")
                else:
                    lines.append(f"{self.name} {val}")
        return "\n".join(lines)


class Gauge:
    """Thread-safe Prometheus gauge."""

    def __init__(self, name: str, help_text: str, label_names: List[str] = None):
        self.name = name
        self.help_text = help_text
        self.label_names = label_names or []
        self._lock = threading.Lock()
        self._data: Dict[tuple, float] = {}

    def set(self, value: float, labels: dict = None):
        key = tuple(str((labels or {}).get(k, "")) for k in self.label_names)
        with self._lock:
            self._data[key] = float(value)

    def render(self) -> str:
        lines = [f"# HELP {self.name} {self.help_text}", f"# TYPE {self.name} gauge"]
        with self._lock:
            if not self._data and not self.label_names:
                lines.append(f"{self.name} 0.0")
            for key, val in self._data.items():
                if self.label_names:
                    label_pairs = [f'{k}="{v}"' for k, v in zip(self.label_names, key)]
                    lines.append(f"{self.name}{{{', '.join(label_pairs)}}} {val:.4f}")
                else:
                    lines.append(f"{self.name} {val:.4f}")
        return "\n".join(lines)


class MetricsRegistry:
    """Central metrics collector for inference pipeline instrumentation."""

    def __init__(self):
        self.stage_latency = Histogram(
            name="inference_stage_latency_seconds",
            help_text="Per-stage execution latency in seconds (decode, preprocess, inference, postprocess, publish).",
            label_names=["stage"],
        )
        self.frames_processed = Counter(
            name="inference_frames_processed_total",
            help_text="Total number of video frames successfully processed through the pipeline.",
        )
        self.frames_dropped = Counter(
            name="inference_frames_dropped_total",
            help_text="Total number of video frames dropped due to backpressure / bounded queue saturation.",
        )
        self.fps_gauge = Gauge(
            name="inference_fps",
            help_text="Instantaneous measured frames per second.",
        )
        self.active_tracks_gauge = Gauge(
            name="inference_tracks_active",
            help_text="Current number of active confirmed object tracks.",
        )
        self.model_status_gauge = Gauge(
            name="inference_model_status",
            help_text="Model load status (1 = loaded and healthy, 0 = degraded/missing).",
            label_names=["model"],
        )

        # FPS calculation window
        self._fps_lock = threading.Lock()
        self._recent_timestamps: List[float] = []

    def record_frame_processed(self):
        self.frames_processed.inc()
        now = time.monotonic()
        with self._fps_lock:
            self._recent_timestamps.append(now)
            # Retain timestamps from last 5 seconds
            self._recent_timestamps = [t for t in self._recent_timestamps if now - t <= 5.0]
            if len(self._recent_timestamps) > 1:
                dt = self._recent_timestamps[-1] - self._recent_timestamps[0]
                fps = (len(self._recent_timestamps) - 1) / dt if dt > 0 else 0.0
                self.fps_gauge.set(fps)

    def record_frame_dropped(self):
        self.frames_dropped.inc()

    def render_prometheus(self) -> str:
        sections = [
            self.stage_latency.render(),
            self.frames_processed.render(),
            self.frames_dropped.render(),
            self.fps_gauge.render(),
            self.active_tracks_gauge.render(),
            self.model_status_gauge.render(),
        ]
        return "\n\n".join(sections) + "\n"


# Global singleton instance
metrics = MetricsRegistry()
