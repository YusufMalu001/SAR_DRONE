#!/usr/bin/env python
"""Benchmarks per-model latency and end-to-end pipeline FPS, to demonstrate
edge-feasible inference speed.

    python -m scripts.benchmark [--iterations 50] [--warmup 5] [--video PATH] [--json out.json]

Uses real frames from `--video` (default: settings.video_path) if it
exists, otherwise falls back to synthetic 640x480 noise frames so the
benchmark still runs before any sample footage is in place.

Any model whose ONNX weights aren't trained yet is skipped with a clear
message rather than failing the whole run — see train_*.py.
"""

import argparse
import json
import logging
import statistics
import sys
import time
from pathlib import Path
from typing import Dict, List, Optional

import numpy as np

from app.config import settings
from app.models.base import ModelNotFoundError
from app.models.hazard_classifier import HazardClassifier
from app.models.person_detector import PersonDetector
from app.models.thermal_detector import ThermalDetector

logger = logging.getLogger(__name__)


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--iterations", type=int, default=50)
    parser.add_argument("--warmup", type=int, default=5)
    parser.add_argument("--video", default=settings.video_path)
    parser.add_argument("--json", type=Path, default=None, help="Optional path to write results as JSON")
    return parser


def load_sample_frames(video_path: str, count: int) -> List[np.ndarray]:
    path = Path(video_path)
    if path.exists():
        import cv2

        cap = cv2.VideoCapture(str(path))
        frames = []
        while cap.isOpened() and len(frames) < count:
            ok, frame = cap.read()
            if not ok:
                break
            frames.append(frame)
        cap.release()
        if frames:
            # Cycle frames if the video is shorter than requested count.
            return [frames[i % len(frames)] for i in range(count)]

    logger.warning(
        "No usable video at '%s' — benchmarking against synthetic 640x480 noise frames instead.",
        video_path,
    )
    rng = np.random.default_rng(0)
    return [rng.integers(0, 255, size=(480, 640, 3), dtype=np.uint8) for _ in range(count)]


def time_model(
    name: str, predict_fn, warmup_frames: List[np.ndarray], timed_frames: List[np.ndarray]
) -> Optional[Dict]:
    # Trigger lazy weight loading / raise ModelNotFoundError up front, using
    # whichever frame pool is non-empty (warmup may be 0).
    probe_frame = warmup_frames[0] if warmup_frames else timed_frames[0]
    try:
        predict_fn(probe_frame, "frame_warmup", "1970-01-01T00:00:00Z")
    except ModelNotFoundError as exc:
        print(f"[skip] {name}: {exc}")
        return None

    for i, frame in enumerate(warmup_frames[1:]):
        predict_fn(frame, f"frame_warmup_{i}", "1970-01-01T00:00:00Z")

    latencies_ms = []
    for i, frame in enumerate(timed_frames):
        start = time.perf_counter()
        predict_fn(frame, f"frame_{i:06d}", "1970-01-01T00:00:00Z")
        latencies_ms.append((time.perf_counter() - start) * 1000)

    latencies_ms.sort()
    mean_ms = statistics.mean(latencies_ms)
    return {
        "model": name,
        "mean_ms": round(mean_ms, 2),
        "p50_ms": round(latencies_ms[len(latencies_ms) // 2], 2),
        "p95_ms": round(latencies_ms[int(len(latencies_ms) * 0.95) - 1], 2),
        "fps": round(1000 / mean_ms, 2) if mean_ms > 0 else float("inf"),
    }


def main() -> None:
    logging.basicConfig(level=logging.INFO)
    args = build_arg_parser().parse_args()

    all_frames = load_sample_frames(args.video, args.iterations + args.warmup)
    warmup_frames, timed_frames = all_frames[: args.warmup], all_frames[args.warmup :]

    models = {
        "person_detector": PersonDetector(),
        "thermal_detector": ThermalDetector(),
        "hazard_classifier": HazardClassifier(),
    }

    results = []
    for name, model in models.items():
        result = time_model(name, model.predict, warmup_frames, timed_frames)
        if result:
            results.append(result)

    print()
    print(f"{'model':<20} {'mean(ms)':>10} {'p50(ms)':>10} {'p95(ms)':>10} {'fps':>10}")
    for r in results:
        print(f"{r['model']:<20} {r['mean_ms']:>10} {r['p50_ms']:>10} {r['p95_ms']:>10} {r['fps']:>10}")

    if results:
        pipeline_latency_ms = sum(r["mean_ms"] for r in results)
        pipeline_fps = 1000 / pipeline_latency_ms if pipeline_latency_ms > 0 else float("inf")
        print()
        print(f"end-to-end pipeline latency: {pipeline_latency_ms:.2f} ms/frame")
        print(f"end-to-end pipeline throughput: {pipeline_fps:.2f} fps (all 3 models, sequential, single core)")
    else:
        print("\nNo models had weights available to benchmark — run the train_*.py scripts first.")

    if args.json:
        args.json.write_text(json.dumps(results, indent=2))
        print(f"\nWrote results to {args.json}")

    if not results:
        sys.exit(1)


if __name__ == "__main__":
    main()
