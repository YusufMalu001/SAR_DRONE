import { useState } from "react";
import { ackAlert, resolveAlert } from "../lib/api";
import type { AlertRecord, PriorityTier } from "../lib/types";

const TIER_LABEL: Record<PriorityTier, string> = {
  critical: "Critical",
  high: "High",
  medium: "Medium",
};

interface AlertFeedProps {
  alerts: AlertRecord[];
  onAlertUpdated?: () => void;
}

export default function AlertFeed({ alerts, onAlertUpdated }: AlertFeedProps) {
  const [busyId, setBusyId] = useState<number | null>(null);

  const sorted = [...alerts].sort(
    (a, b) => Date.parse(b.created_at) - Date.parse(a.created_at),
  );

  const handleAck = async (id: number) => {
    setBusyId(id);
    try {
      await ackAlert(id);
      onAlertUpdated?.();
    } catch (err) {
      console.error("Ack alert failed:", err);
    } finally {
      setBusyId(null);
    }
  };

  const handleResolve = async (id: number) => {
    setBusyId(id);
    try {
      await resolveAlert(id);
      onAlertUpdated?.();
    } catch (err) {
      console.error("Resolve alert failed:", err);
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div className="panel alert-feed" role="region" aria-label="Alert Feed">
      <div className="panel__header">
        <h3 className="panel__title">Alerts</h3>
        <span className="panel__count">{alerts.length}</span>
      </div>

      <ul className="alert-feed__list" tabIndex={0} aria-label="System Alerts">
        {sorted.length === 0 && (
          <li className="alert-feed__empty">No alerts yet.</li>
        )}
        {sorted.map((alert) => (
          <li key={alert.id} className="alert-feed__item">
            <span className={`badge badge--${alert.priority_tier}`}>
              {TIER_LABEL[alert.priority_tier] ?? alert.priority_tier}
            </span>
            <div className="alert-feed__body">
              <p className="alert-feed__message">{alert.message}</p>
              <div className="alert-feed__meta">
                <time className="alert-feed__time">
                  {new Date(alert.created_at).toLocaleTimeString()}
                </time>
                {alert.state && (
                  <span className="alert-state mono">{alert.state}</span>
                )}
              </div>
              <div className="alert-feed__actions">
                <button
                  className="btn-xs"
                  onClick={() => handleAck(alert.id)}
                  disabled={busyId === alert.id || alert.state === "acknowledged" || alert.state === "resolved"}
                >
                  Ack
                </button>
                <button
                  className="btn-xs btn-xs--resolve"
                  onClick={() => handleResolve(alert.id)}
                  disabled={busyId === alert.id || alert.state === "resolved"}
                >
                  Resolve
                </button>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
