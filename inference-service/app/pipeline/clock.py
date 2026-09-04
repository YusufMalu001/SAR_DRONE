import time
from datetime import datetime, timedelta, timezone
from typing import Optional


class PlaybackClock:
    """A shared "video timeline" clock so independent producers (frame
    inference, telemetry replay) can pace themselves against the same
    origin and stamp messages with timestamps that line up.

    `elapsed_seconds` is always measured from the same `start_time`, so a
    consumer joining the `detections` and `telemetry` Redis streams by
    timestamp sees values on a consistent timeline regardless of which
    process is slightly ahead or behind in wall-clock terms.
    """

    def __init__(self, start_time: Optional[datetime] = None, speed: float = 1.0):
        self.start_time = start_time or datetime.now(timezone.utc)
        self.speed = speed
        self._t0 = time.monotonic()

    def timestamp_at(self, elapsed_seconds: float) -> str:
        moment = self.start_time + timedelta(seconds=elapsed_seconds)
        return moment.isoformat().replace("+00:00", "Z")

    def sleep_until(self, elapsed_seconds: float) -> None:
        """Block until `elapsed_seconds` of playback time have passed."""
        target = self._t0 + elapsed_seconds / self.speed
        delay = target - time.monotonic()
        if delay > 0:
            time.sleep(delay)
