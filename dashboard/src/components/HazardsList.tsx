import type { EntityRecord } from "../lib/types";

interface HazardsListProps {
  hazards: EntityRecord[];
  selectedId: string | null;
  onSelect: (hazard: EntityRecord) => void;
}

export default function HazardsList({
  hazards,
  selectedId,
  onSelect,
}: HazardsListProps) {
  return (
    <div className="panel hazards-panel" role="region" aria-label="Hazards List">
      <div className="panel__header">
        <h3 className="panel__title">Hazards</h3>
        <span className="panel__count">{hazards.length}</span>
      </div>

      <ul className="entity-list" tabIndex={0} aria-label="Detected hazards">
        {hazards.length === 0 ? (
          <li className="entity-list__empty">No hazards detected in zone.</li>
        ) : (
          hazards.map((h) => {
            const isSelected = h.id === selectedId;
            return (
              <li
                key={h.id}
                className={`entity-item ${isSelected ? "entity-item--selected" : ""}`}
                onClick={() => onSelect(h)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect(h);
                  }
                }}
                tabIndex={0}
                role="button"
                aria-pressed={isSelected}
              >
                <div className="entity-item__top">
                  <span className="hazard-type-tag">{h.class.replace("_", " ")}</span>
                  <span className={`status-chip status-chip--${h.status}`}>
                    {h.status}
                  </span>
                </div>
                <div className="entity-item__body">
                  <span className="mono entity-coords">
                    {h.lat.toFixed(5)}, {h.lon.toFixed(5)}
                  </span>
                  <span className="entity-meta">
                    Buffer: {h.uncertainty_radius ? `${Math.round(h.uncertainty_radius)}m` : "50m"} · Confidence: {(h.confidence * 100).toFixed(0)}%
                  </span>
                </div>
              </li>
            );
          })
        )}
      </ul>
    </div>
  );
}
