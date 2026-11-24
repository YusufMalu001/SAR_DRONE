import { useEffect, useState } from "react";
import { fetchFusionHealth } from "../lib/api";
import type { ServiceHealth } from "../lib/types";

interface HealthStripProps {
  detectionsPerMinute: number;
  avgFusionLatencyMs: number | null;
}

export default function HealthStrip({
  detectionsPerMinute,
  avgFusionLatencyMs,
}: HealthStripProps) {
  const [fusionHealth, setFusionHealth] = useState<ServiceHealth | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function update() {
      try {
        const h = await fetchFusionHealth();
        if (!cancelled) setFusionHealth(h);
      } catch {
        // ignore fetch failures in offline mode
      }
    }
    update();
    const timer = setInterval(update, 2000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  const queueDepth =
    typeof fusionHealth?.queue_depth === "number" ? fusionHealth.queue_depth : 0;
  const oldestAge =
    typeof fusionHealth?.oldest_item_age_seconds === "number"
      ? Math.round(fusionHealth.oldest_item_age_seconds)
      : 0;
  const lastSync =
    typeof fusionHealth?.last_successful_sync === "string"
      ? new Date(fusionHealth.last_successful_sync).toLocaleTimeString()
      : "—";

  return (
    <div className="health-strip">
      <div className="health-strip__metric">
        <span className="health-strip__label">Detection throughput</span>
        <span className="health-strip__value">
          {detectionsPerMinute}
          <span className="health-strip__unit">/min</span>
        </span>
      </div>
      <div className="health-strip__divider" />
      <div className="health-strip__metric">
        <span className="health-strip__label">Avg fusion latency</span>
        <span className="health-strip__value">
          {avgFusionLatencyMs === null ? (
            "—"
          ) : (
            <>
              {Math.round(avgFusionLatencyMs)}
              <span className="health-strip__unit">ms</span>
            </>
          )}
        </span>
      </div>
      <div className="health-strip__divider" />
      <div className="health-strip__metric">
        <span className="health-strip__label">Queue Depth</span>
        <span className="health-strip__value" style={{ color: queueDepth > 0 ? "#f97316" : "inherit" }}>
          {queueDepth}
          <span className="health-strip__unit">items</span>
        </span>
      </div>
      <div className="health-strip__divider" />
      <div className="health-strip__metric">
        <span className="health-strip__label">Oldest Queued</span>
        <span className="health-strip__value">
          {oldestAge > 0 ? `${oldestAge}s` : "0s"}
        </span>
      </div>
      <div className="health-strip__divider" />
      <div className="health-strip__metric">
        <span className="health-strip__label">Last Sync</span>
        <span className="health-strip__value" style={{ fontSize: "13px" }}>
          {lastSync}
        </span>
      </div>
    </div>
  );
}
