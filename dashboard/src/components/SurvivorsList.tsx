import type { EntityRecord } from "../lib/types";

interface SurvivorsListProps {
  survivors: EntityRecord[];
  selectedId: string | null;
  onSelect: (survivor: EntityRecord) => void;
}

export default function SurvivorsList({
  survivors,
  selectedId,
  onSelect,
}: SurvivorsListProps) {
  const sorted = [...survivors].sort((a, b) => b.priority_score - a.priority_score);

  return (
    <div className="panel survivors-panel" role="region" aria-label="Survivors List">
      <div className="panel__header">
        <h3 className="panel__title">Survivors</h3>
        <span className="panel__count">{survivors.length}</span>
      </div>

      <ul className="entity-list" tabIndex={0} aria-label="Detected survivors">
        {sorted.length === 0 ? (
          <li className="entity-list__empty">No survivors detected yet.</li>
        ) : (
          sorted.map((s) => {
            const isSelected = s.id === selectedId;
            const tier = s.priority_score >= 0.7 ? "critical" : s.priority_score >= 0.4 ? "high" : "medium";
            return (
              <li
                key={s.id}
                className={`entity-item ${isSelected ? "entity-item--selected" : ""}`}
                onClick={() => onSelect(s)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect(s);
                  }
                }}
                tabIndex={0}
                role="button"
                aria-pressed={isSelected}
              >
                <div className="entity-item__top">
                  <span className={`badge badge--${tier}`}>
                    Score: {s.priority_score.toFixed(2)}
                  </span>
                  <span className={`status-chip status-chip--${s.status}`}>
                    {s.status}
                  </span>
                </div>
                <div className="entity-item__body">
                  <span className="mono entity-coords">
                    {s.lat.toFixed(5)}, {s.lon.toFixed(5)}
                  </span>
                  <span className="entity-meta">
                    Uncertainty: ±{s.uncertainty_radius ? `${Math.round(s.uncertainty_radius)}m` : "N/A"} · Obs: {s.observation_count}
                  </span>
                  {s.modalities && s.modalities.length > 0 && (
                    <div className="entity-modalities">
                      {s.modalities.map((m) => (
                        <span key={m} className="modality-tag">{m.toUpperCase()}</span>
                      ))}
                    </div>
                  )}
                </div>
              </li>
            );
          })
        )}
      </ul>
    </div>
  );
}
