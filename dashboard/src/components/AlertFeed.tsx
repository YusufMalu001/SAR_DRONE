import type { AlertRecord, PriorityTier } from "../lib/types";

const TIER_LABEL: Record<PriorityTier, string> = {
  critical: "Critical",
  high: "High",
  medium: "Medium",
};

interface AlertFeedProps {
  alerts: AlertRecord[];
}

export default function AlertFeed({ alerts }: AlertFeedProps) {
  const sorted = [...alerts].sort(
    (a, b) => Date.parse(b.created_at) - Date.parse(a.created_at),
  );

  return (
    <div className="panel alert-feed">
      <div className="panel__header">
        <h3 className="panel__title">Alerts</h3>
        <span className="panel__count">{alerts.length}</span>
      </div>

      <ul className="alert-feed__list">
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
              <time className="alert-feed__time">
                {new Date(alert.created_at).toLocaleTimeString()}
              </time>
            </div>
          </li>
        ))}
      </ul>
    </div>
  );
}
