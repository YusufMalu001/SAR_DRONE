import { useEffect, useState } from "react";
import { fetchRouteStatus } from "../lib/api";
import type { RouteStatus } from "../lib/types";

const POLL_INTERVAL_MS = 15_000;

export function useRouteStatus(): RouteStatus | null {
  const [status, setStatus] = useState<RouteStatus | null>(null);

  useEffect(() => {
    let cancelled = false;

    function load() {
      fetchRouteStatus()
        .then((data) => {
          if (!cancelled) setStatus(data);
        })
        .catch((err) => console.error("fetchRouteStatus failed:", err));
    }

    load();
    const interval = setInterval(load, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  return status;
}
