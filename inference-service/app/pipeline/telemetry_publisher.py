import csv
import logging
from pathlib import Path
from typing import List, Optional, TypedDict

import redis

from app.config import settings
from app.messaging import xadd_json
from app.pipeline.clock import PlaybackClock
from app.schemas import Telemetry

logger = logging.getLogger(__name__)


class TelemetryRow(TypedDict):
    lat: float
    lon: float
    alt: float
    heading: float
    elapsed_s: float


class TelemetryPublisher:
    """Reads a pre-recorded lat/lon/alt/heading CSV and replays it onto the
    `telemetry` Redis stream, paced against the same `PlaybackClock` used by
    `VideoIngestionPipeline` so detections and telemetry can be joined by
    timestamp downstream.

    Expected CSV columns: lat, lon, alt, heading, and either:
      - elapsed_s: seconds since flight start (preferred), or
      - no time column at all, in which case rows are assumed to be evenly
        spaced at `settings.telemetry_sample_rate_hz`.
    """

    def __init__(
        self,
        redis_client: redis.Redis,
        csv_path: Optional[str] = None,
        stream_name: Optional[str] = None,
        clock: Optional[PlaybackClock] = None,
        sample_rate_hz: Optional[float] = None,
    ):
        self.redis_client = redis_client
        self.csv_path = csv_path or settings.telemetry_csv_path
        self.stream_name = stream_name or settings.telemetry_channel
        self.clock = clock or PlaybackClock()
        self.sample_rate_hz = sample_rate_hz or settings.telemetry_sample_rate_hz

    def _load_rows(self) -> List[TelemetryRow]:
        path = Path(self.csv_path)
        if not path.exists():
            raise FileNotFoundError(
                f"Telemetry CSV not found at '{self.csv_path}'. Place a demo CSV there "
                "(see inference-service/README.md) before running the telemetry publisher."
            )

        rows: List[TelemetryRow] = []
        with path.open(newline="") as f:
            reader = csv.DictReader(f)
            for idx, raw_row in enumerate(reader):
                if "elapsed_s" in raw_row and raw_row["elapsed_s"] not in (None, ""):
                    elapsed_s = float(raw_row["elapsed_s"])
                else:
                    elapsed_s = idx / self.sample_rate_hz

                rows.append(
                    TelemetryRow(
                        lat=float(raw_row["lat"]),
                        lon=float(raw_row["lon"]),
                        alt=float(raw_row["alt"]),
                        heading=float(raw_row["heading"]),
                        elapsed_s=elapsed_s,
                    )
                )
        return rows

    def run(self, loop: bool = False) -> int:
        rows = self._load_rows()
        published = 0

        while True:
            for row in rows:
                self.clock.sleep_until(row["elapsed_s"])
                timestamp = self.clock.timestamp_at(row["elapsed_s"])
                telemetry = Telemetry(
                    lat=row["lat"],
                    lon=row["lon"],
                    alt=row["alt"],
                    heading=row["heading"],
                    timestamp=timestamp,
                )
                xadd_json(self.redis_client, self.stream_name, telemetry.model_dump())
                published += 1

            if not loop:
                break

        logger.info("telemetry publisher finished: %d rows published", published)
        return published
