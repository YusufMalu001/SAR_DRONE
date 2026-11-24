"""Bounded frame queue implementing drop-oldest backpressure policy."""

import queue
import threading
from typing import Any, Optional

from app.pipeline.metrics import metrics


class BoundedFrameQueue:
    """Thread-safe queue with bounded capacity and drop-oldest backpressure.

    When new frames arrive and the queue is at maximum capacity, the oldest unconsumed
    frame is evicted and the dropped frames counter is incremented, ensuring real-time freshness.
    """

    def __init__(self, maxsize: int = 10):
        self.maxsize = maxsize
        self._queue = queue.Queue(maxsize=maxsize)
        self._lock = threading.Lock()

    def put(self, item: Any) -> bool:
        """Puts an item into the queue. If full, drops the oldest item first."""
        with self._lock:
            if self._queue.full():
                try:
                    _dropped = self._queue.get_nowait()
                    metrics.record_frame_dropped()
                except queue.Empty:
                    pass

            self._queue.put_nowait(item)
            return True

    def get(self, block: bool = True, timeout: Optional[float] = None) -> Any:
        return self._queue.get(block=block, timeout=timeout)

    def empty(self) -> bool:
        return self._queue.empty()

    def qsize(self) -> int:
        return self._queue.qsize()
