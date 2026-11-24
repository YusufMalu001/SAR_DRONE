import { useState } from "react";
import { ackAlert, resolveAlert, resolveEntity } from "../lib/api";
import type { AlertRecord, EntityRecord } from "../lib/types";

interface EntityDetailDrawerProps {
  entity: EntityRecord | null;
  hazards: EntityRecord[];
  alerts: AlertRecord[];
  onClose: () => void;
  onEntityUpdated: (updated: EntityRecord) => void;
}

function haversineMeters(lat1: number, lon1: number, lat2: number, lon2: number): number {
  const R = 6371000;
  const dLat = ((lat2 - lat1) * Math.PI) / 180;
  const dLon = ((lon2 - lon1) * Math.PI) / 180;
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos((lat1 * Math.PI) / 180) *
      Math.cos((lat2 * Math.PI) / 180) *
      Math.sin(dLon / 2) *
      Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

export default function EntityDetailDrawer({
  entity,
  hazards,
  alerts,
  onClose,
  onEntityUpdated,
}: EntityDetailDrawerProps) {
  const [busy, setBusy] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);

  if (!entity) return null;

  // Nearby hazards within 200m
  const nearbyHazards = hazards
    .filter((h) => h.id !== entity.id)
    .map((h) => ({
      hazard: h,
      distanceM: haversineMeters(entity.lat, entity.lon, h.lat, h.lon),
    }))
    .filter((h) => h.distanceM <= 250)
    .sort((a, b) => a.distanceM - b.distanceM);

  // Associated alert(s)
  const relatedAlerts = alerts.filter(
    (a) => a.detection_id === entity.id || entity.track_ids?.includes(a.detection_id)
  );

  const handleResolve = async (status: "rescued" | "cleared") => {
    setBusy(true);
    setFeedback(null);
    try {
      const res = await resolveEntity(entity.id, status);
      onEntityUpdated(res);
      setFeedback(`Entity marked as ${status}`);
    } catch (err) {
      setFeedback(`Failed: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  const handleAckAlert = async (alertId: number) => {
    setBusy(true);
    try {
      await ackAlert(alertId);
      setFeedback("Alert acknowledged");
    } catch (err) {
      setFeedback(`Ack failed: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  const handleResolveAlert = async (alertId: number) => {
    setBusy(true);
    try {
      await resolveAlert(alertId);
      setFeedback("Alert resolved");
    } catch (err) {
      setFeedback(`Resolve failed: ${err instanceof Error ? err.message : String(err)}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="entity-detail-drawer" role="dialog" aria-label="Entity Details">
      <div className="drawer-header">
        <div className="drawer-title-group">
          <h3 className="drawer-title">
            {entity.type.toUpperCase()}: {entity.class.replace("_", " ")}
          </h3>
          <span className={`status-chip status-chip--${entity.status}`}>
            {entity.status}
          </span>
        </div>
        <button
          className="icon-button"
          onClick={onClose}
          aria-label="Close entity drawer"
        >
          ×
        </button>
      </div>

      <div className="drawer-body">
        {feedback && (
          <div className="drawer-feedback mono" role="status">
            {feedback}
          </div>
        )}

        <section className="drawer-section">
          <h4 className="section-heading">Geotag & Uncertainty</h4>
          <div className="drawer-grid">
            <span className="grid-label">Coordinates:</span>
            <span className="grid-val mono">
              {entity.lat.toFixed(5)}, {entity.lon.toFixed(5)}
            </span>
            <span className="grid-label">Uncertainty Radius:</span>
            <span className="grid-val mono">
              ±{entity.uncertainty_radius ? `${entity.uncertainty_radius.toFixed(1)}m` : "N/A"}
            </span>
            <span className="grid-label">Priority Score:</span>
            <span className="grid-val mono">
              {entity.priority_score.toFixed(2)}
            </span>
          </div>
        </section>

        <section className="drawer-section">
          <h4 className="section-heading">Modalities & Confidence Timeline</h4>
          <div className="modalities-row">
            {entity.modalities && entity.modalities.length > 0 ? (
              entity.modalities.map((m) => (
                <span key={m} className="modality-pill">
                  {m.toUpperCase()}
                </span>
              ))
            ) : (
              <span className="modality-pill">RGB</span>
            )}
          </div>
          <div className="drawer-grid">
            <span className="grid-label">Confidence:</span>
            <span className="grid-val mono">{(entity.confidence * 100).toFixed(1)}%</span>
            <span className="grid-label">Observations:</span>
            <span className="grid-val mono">{entity.observation_count} frames</span>
            <span className="grid-label">First Seen:</span>
            <span className="grid-val mono">
              {entity.first_seen ? new Date(entity.first_seen).toLocaleTimeString() : "N/A"}
            </span>
            <span className="grid-label">Last Seen:</span>
            <span className="grid-val mono">
              {entity.last_seen ? new Date(entity.last_seen).toLocaleTimeString() : "N/A"}
            </span>
          </div>
        </section>

        <section className="drawer-section">
          <h4 className="section-heading">Track History</h4>
          {entity.track_ids && entity.track_ids.length > 0 ? (
            <ul className="track-list mono">
              {entity.track_ids.map((tid) => (
                <li key={tid} className="track-list-item">
                  Track {tid}
                </li>
              ))}
            </ul>
          ) : (
            <p className="empty-text">No active track lineage registered.</p>
          )}
        </section>

        <section className="drawer-section">
          <h4 className="section-heading">Nearby Hazards (Proximity ≤ 250m)</h4>
          {nearbyHazards.length === 0 ? (
            <p className="safe-text">Clear zone — no hazards within proximity buffer.</p>
          ) : (
            <ul className="nearby-hazard-list">
              {nearbyHazards.map(({ hazard, distanceM }) => (
                <li key={hazard.id} className="nearby-hazard-item">
                  <span className="hazard-name">{hazard.class.replace("_", " ")}</span>
                  <span className="hazard-dist mono">{Math.round(distanceM)}m away</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        {relatedAlerts.length > 0 && (
          <section className="drawer-section">
            <h4 className="section-heading">Related Alerts</h4>
            <ul className="drawer-alerts-list">
              {relatedAlerts.map((a) => (
                <li key={a.id} className="drawer-alert-item">
                  <div className="alert-meta-row">
                    <span className={`badge badge--${a.priority_tier}`}>{a.priority_tier}</span>
                    <span className="alert-state-tag mono">{a.state ?? "active"}</span>
                  </div>
                  <p className="alert-msg">{a.message}</p>
                  <div className="alert-actions-row">
                    <button
                      className="btn-sm"
                      onClick={() => handleAckAlert(a.id)}
                      disabled={busy}
                    >
                      Acknowledge
                    </button>
                    <button
                      className="btn-sm btn-sm--resolve"
                      onClick={() => handleResolveAlert(a.id)}
                      disabled={busy}
                    >
                      Resolve
                    </button>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        )}

        <section className="drawer-actions-footer">
          {entity.type === "survivor" ? (
            <button
              className="action-btn action-btn--rescue"
              onClick={() => handleResolve("rescued")}
              disabled={busy || entity.status === "rescued"}
            >
              Mark Survivor Rescued
            </button>
          ) : (
            <button
              className="action-btn action-btn--clear"
              onClick={() => handleResolve("cleared")}
              disabled={busy || entity.status === "cleared"}
            >
              Mark Hazard Cleared
            </button>
          )}
        </section>
      </div>
    </div>
  );
}
