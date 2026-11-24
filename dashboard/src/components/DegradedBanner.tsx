import type { ServiceHealth } from "../lib/types";

interface DegradedBannerProps {
  fusionHealth: ServiceHealth | null;
  inferenceHealth: ServiceHealth | null;
  missionHealth: ServiceHealth | null;
}

export default function DegradedBanner({
  fusionHealth,
  inferenceHealth,
  missionHealth,
}: DegradedBannerProps) {
  const isFusionOffline = !fusionHealth || fusionHealth.status !== "ok";
  const isDegradedMode = fusionHealth?.mode === "degraded" || fusionHealth?.mode === "offline";
  const queueDepth = typeof fusionHealth?.queue_depth === "number" ? fusionHealth.queue_depth : null;
  const oldestAge = typeof fusionHealth?.oldest_item_age_seconds === "number" ? Math.round(fusionHealth.oldest_item_age_seconds) : null;
  const lastSync = typeof fusionHealth?.last_successful_sync === "string" && fusionHealth.last_successful_sync
    ? new Date(fusionHealth.last_successful_sync).toLocaleTimeString()
    : "N/A";

  const isInferenceOffline = !inferenceHealth || inferenceHealth.status !== "ok";
  const isMissionOffline = !missionHealth || missionHealth.status !== "ok";

  const showBanner = isFusionOffline || isDegradedMode || (queueDepth !== null && queueDepth > 0) || isInferenceOffline || isMissionOffline;

  if (!showBanner) {
    return null;
  }

  let bannerClass = "degraded-banner--warning";
  let title = "Degraded Operational State";

  if (isFusionOffline || isDegradedMode) {
    bannerClass = "degraded-banner--offline";
    title = fusionHealth?.mode === "offline" ? "Autonomous Offline Mode (Store-and-Forward Active)" : "Degraded Connectivity Mode";
  }

  return (
    <div className={`degraded-banner ${bannerClass}`} role="region" aria-label="System Connectivity Banner">
      <div className="degraded-banner__indicator" aria-hidden="true" />
      <div className="degraded-banner__content">
        <span className="degraded-banner__title">{title}</span>
        <div className="degraded-banner__metrics">
          <span className="degraded-metric">
            Queue Depth: <strong>{queueDepth !== null ? queueDepth : "N/A"}</strong>
          </span>
          <span className="degraded-metric">
            Oldest Unsynced: <strong>{oldestAge !== null ? `${oldestAge}s` : "N/A"}</strong>
          </span>
          <span className="degraded-metric">
            Last Sync: <strong>{lastSync}</strong>
          </span>
          {isInferenceOffline && <span className="degraded-tag">Inference Offline</span>}
          {isMissionOffline && <span className="degraded-tag">Mission Control Offline</span>}
        </div>
      </div>
    </div>
  );
}
