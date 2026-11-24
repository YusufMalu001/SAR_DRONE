"""Runs video ingestion + telemetry replay together against a shared clock."""

import argparse
import logging
import threading

from app.config import settings
from app.logging_config import setup_json_logging
from app.messaging import get_redis
from app.pipeline.clock import PlaybackClock
from app.pipeline.telemetry_publisher import TelemetryPublisher
from app.pipeline.video_ingest import VideoIngestionPipeline

logger = logging.getLogger(__name__)


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--video", default=settings.video_path)
    parser.add_argument("--thermal-video", default=None)
    parser.add_argument("--telemetry-csv", default=settings.telemetry_csv_path)
    parser.add_argument("--fps", type=float, default=settings.target_fps)
    parser.add_argument("--speed", type=float, default=settings.playback_speed)
    parser.add_argument("--max-frames", type=int, default=None)
    parser.add_argument("--loop", action="store_true")
    return parser


def main() -> None:
    setup_json_logging()
    args = build_arg_parser().parse_args()

    redis_client = get_redis()
    clock = PlaybackClock(speed=args.speed)

    video_pipeline = VideoIngestionPipeline(
        redis_client=redis_client,
        video_path=args.video,
        thermal_path=args.thermal_video,
        target_fps=args.fps,
        clock=clock,
    )
    telemetry_publisher = TelemetryPublisher(
        redis_client=redis_client,
        csv_path=args.telemetry_csv,
        clock=clock,
    )

    video_thread = threading.Thread(
        target=video_pipeline.run,
        kwargs={"max_frames": args.max_frames, "loop": args.loop},
        name="video-ingestion",
    )
    telemetry_thread = threading.Thread(
        target=telemetry_publisher.run,
        kwargs={"loop": args.loop},
        name="telemetry-publisher",
    )

    video_thread.start()
    telemetry_thread.start()
    video_thread.join()
    telemetry_thread.join()


if __name__ == "__main__":
    main()
