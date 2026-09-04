interface HealthStripProps {
  detectionsPerMinute: number;
  avgFusionLatencyMs: number | null;
}

export default function HealthStrip({
  detectionsPerMinute,
  avgFusionLatencyMs,
}: HealthStripProps) {
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
    </div>
  );
}
