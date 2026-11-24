import { useEffect, useState } from "react";
import {
  abortMission,
  fetchMissionStatus,
  pauseMission,
  planCoverageMission,
  resumeMission,
  startMission,
} from "../lib/api";
import type { CoveragePlanResponse, MissionStatusResponse } from "../lib/types";

interface MissionControlPanelProps {
  drawnPolygon: Array<{ lat: number; lon: number }>;
  onPlanPreview: (plan: CoveragePlanResponse | null) => void;
  onClearPolygon: () => void;
}

export default function MissionControlPanel({
  drawnPolygon,
  onPlanPreview,
  onClearPolygon,
}: MissionControlPanelProps) {
  const [status, setStatus] = useState<MissionStatusResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [, setPlanResult] = useState<CoveragePlanResponse | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function update() {
      try {
        const s = await fetchMissionStatus();
        if (!cancelled) setStatus(s);
      } catch {
        // mission-service may be offline in some test setups
      }
    }
    update();
    const interval = setInterval(update, 3000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, []);

  const handlePlan = async () => {
    const polygonToUse =
      drawnPolygon.length >= 3
        ? drawnPolygon
        : [
            { lat: 20.0, lon: 78.0 },
            { lat: 20.005, lon: 78.0 },
            { lat: 20.005, lon: 78.005 },
            { lat: 20.0, lon: 78.005 },
          ];

    setLoading(true);
    setActionMessage(null);
    try {
      const plan = await planCoverageMission(polygonToUse);
      setPlanResult(plan);
      onPlanPreview(plan);
      setActionMessage(
        `Planned ${plan.total_waypoints} waypoints (${(plan.coverage_fraction * 100).toFixed(1)}% coverage)`
      );
    } catch (err) {
      setActionMessage(`Planning failed: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setLoading(false);
    }
  };

  const handleAction = async (
    actionFn: () => Promise<{ status: string; mission_state: string }>,
    name: string
  ) => {
    setLoading(true);
    try {
      const res = await actionFn();
      setActionMessage(`Action '${name}' success: State is ${res.mission_state}`);
      const updated = await fetchMissionStatus().catch(() => null);
      if (updated) setStatus(updated);
    } catch (err) {
      setActionMessage(`Action '${name}' failed: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setLoading(false);
    }
  };

  const currentState = status?.state ?? "IDLE";
  const batteryPct = status?.battery_pct !== undefined ? `${status.battery_pct.toFixed(0)}%` : "N/A";
  const waypointsProgress =
    status?.target_waypoints_count
      ? `${status.current_waypoint_idx} / ${status.target_waypoints_count}`
      : "0 / 0";

  return (
    <div className="panel mission-control-panel" role="region" aria-label="Mission Control">
      <div className="panel__header">
        <h3 className="panel__title">Mission Control</h3>
        <span className="state-badge mono">{currentState}</span>
      </div>

      <div className="mission-metrics-grid">
        <div className="metric-box">
          <span className="metric-box__label">Battery</span>
          <span className="metric-box__value mono">{batteryPct}</span>
        </div>
        <div className="metric-box">
          <span className="metric-box__label">Waypoints</span>
          <span className="metric-box__value mono">{waypointsProgress}</span>
        </div>
        <div className="metric-box">
          <span className="metric-box__label">Armed</span>
          <span className="metric-box__value mono">{status?.armed ? "YES" : "NO"}</span>
        </div>
      </div>

      <div className="mission-polygon-section">
        <div className="section-title-row">
          <span className="section-subtitle">Search Area Polygon</span>
          <span className="section-count mono">
            {drawnPolygon.length > 0 ? `${drawnPolygon.length} vertices` : "Default Quad"}
          </span>
        </div>
        {drawnPolygon.length > 0 && (
          <button className="clear-poly-btn" onClick={onClearPolygon}>
            Clear Drawn Polygon
          </button>
        )}
      </div>

      <div className="mission-actions">
        <button
          className="action-btn action-btn--preview"
          onClick={handlePlan}
          disabled={loading}
        >
          {loading ? "Planning..." : "Preview Coverage Plan"}
        </button>

        <div className="button-group">
          <button
            className="action-btn action-btn--start"
            onClick={() => handleAction(startMission, "Start")}
            disabled={loading || currentState === "COVERAGE_SURVEY"}
          >
            Start
          </button>
          <button
            className="action-btn action-btn--pause"
            onClick={() => handleAction(pauseMission, "Pause")}
            disabled={loading || currentState !== "COVERAGE_SURVEY"}
          >
            Pause
          </button>
          <button
            className="action-btn action-btn--resume"
            onClick={() => handleAction(resumeMission, "Resume")}
            disabled={loading || currentState !== "PAUSED"}
          >
            Resume
          </button>
          <button
            className="action-btn action-btn--abort"
            onClick={() => handleAction(abortMission, "Abort")}
            disabled={loading}
          >
            Abort / RTL
          </button>
        </div>
      </div>

      {actionMessage && (
        <div className="mission-action-notice mono" role="status">
          {actionMessage}
        </div>
      )}
    </div>
  );
}
