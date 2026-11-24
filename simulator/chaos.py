"""Connectivity chaos hooks for testing offline queuing and network recovery."""

import json
import logging
import subprocess
from typing import List, Optional
import urllib.error
import urllib.request

from simulator.scenario import ConnectivityEvent

logger = logging.getLogger("simulator.chaos")


class ConnectivityChaosHook:
    """Manages scheduled network and database disconnect/reconnect events during scenario replay."""

    def __init__(
        self,
        fusion_url: str = "http://localhost:8002",
        prefer_docker: bool = False,
    ):
        self.fusion_url = fusion_url.rstrip("/")
        self.prefer_docker = prefer_docker

    def set_offline(self, offline: bool) -> bool:
        """Toggles offline mode via fusion-service debug endpoint."""
        url = f"{self.fusion_url}/debug/set-offline"
        payload = json.dumps({"offline": offline}).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=payload,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=3.0) as resp:
                data = json.loads(resp.read().decode("utf-8"))
                logger.info(
                    "Chaos hook: toggled offline=%s (response: %s)",
                    offline,
                    data,
                )
                return True
        except (urllib.error.URLError, urllib.error.HTTPError, TimeoutError) as e:
            logger.warning("Chaos hook: failed to reach fusion-service at %s: %s", url, e)
            return False

    def toggle_postgres_container(self, pause: bool) -> bool:
        """Pauses or unpauses postgres container via docker compose."""
        cmd = ["docker", "compose", "pause" if pause else "unpause", "postgres"]
        try:
            res = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5.0)
            if res.returncode == 0:
                logger.info("Chaos hook: docker compose %s postgres succeeded", "pause" if pause else "unpause")
                return True
            else:
                logger.warning(
                    "Chaos hook: docker compose failed: %s",
                    res.stderr.decode("utf-8", "replace").strip(),
                )
                return False
        except Exception as e:
            logger.warning("Chaos hook: failed to execute docker compose: %s", e)
            return False

    def cut_link(self, action: str = "set_offline") -> bool:
        if action == "pause_postgres" or self.prefer_docker:
            ok = self.toggle_postgres_container(pause=True)
            if ok:
                return True
        return self.set_offline(offline=True)

    def restore_link(self, action: str = "set_offline") -> bool:
        if action == "pause_postgres" or self.prefer_docker:
            self.toggle_postgres_container(pause=False)
        return self.set_offline(offline=False)


class ChaosScheduler:
    """Evaluates scenario connectivity events against playback elapsed time."""

    def __init__(self, events: List[ConnectivityEvent], hook: Optional[ConnectivityChaosHook] = None):
        self.events = sorted(events, key=lambda e: e.t_start_s)
        self.hook = hook or ConnectivityChaosHook()
        self._triggered_down = set()
        self._triggered_up = set()

    def update(self, elapsed_s: float) -> List[str]:
        """Checks and triggers any pending chaos events for the current elapsed time."""
        logs = []
        for idx, ev in enumerate(self.events):
            if ev.t_start_s <= elapsed_s < ev.t_end_s and idx not in self._triggered_down:
                msg = f"CHAOS [t={elapsed_s:.1f}s]: Cutting connectivity -> {ev.description} ({ev.action})"
                logger.info(msg)
                logs.append(msg)
                self.hook.cut_link(ev.action)
                self._triggered_down.add(idx)

            elif elapsed_s >= ev.t_end_s and idx in self._triggered_down and idx not in self._triggered_up:
                msg = f"CHAOS [t={elapsed_s:.1f}s]: Restoring connectivity -> {ev.description} ({ev.action})"
                logger.info(msg)
                logs.append(msg)
                self.hook.restore_link(ev.action)
                self._triggered_up.add(idx)

        return logs

    def reset(self):
        """Ensures link is restored and state is clean."""
        self.hook.restore_link()
        self._triggered_down.clear()
        self._triggered_up.clear()
