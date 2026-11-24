import type { DetectionRecord } from "../lib/types";
import { CLASS_COLORS } from "./MapView";

interface DetailPanelProps {
  detection: DetectionRecord | null;
  onClose: () => void;
}

export default function DetailPanel({ detection, onClose }: DetailPanelProps) {
  return (
    <div className="panel detail-panel">
      <div className="panel__header">
        <h3 className="panel__title">Detection Detail</h3>
        {detection && (
          <button
            className="icon-button"
            onClick={onClose}
            aria-label="Close detail panel"
          >
            ×
          </button>
        )}
      </div>

      {!detection ? (
        <p className="detail-panel__hint">
          Click a marker on the map to see its details here.
        </p>
      ) : (
        <dl className="detail-panel__grid">
          <dt>Class</dt>
          <dd>
            <span
              className="class-dot"
              style={{ backgroundColor: CLASS_COLORS[detection.class] }}
            />
            {detection.class}
          </dd>

          {detection.track_id && (
            <>
              <dt>Track ID</dt>
              <dd className="mono">{detection.track_id}</dd>
            </>
          )}

          <dt>Confidence</dt>
          <dd>{(detection.confidence * 100).toFixed(1)}%</dd>

          <dt>Timestamp</dt>
          <dd>{new Date(detection.detected_at).toLocaleString()}</dd>

          <dt>Coordinates</dt>
          <dd className="mono">
            {detection.lat.toFixed(5)}, {detection.lon.toFixed(5)}
          </dd>

          <dt>Priority score</dt>
          <dd>{detection.priority_score.toFixed(2)}</dd>

          <dt>Status</dt>
          <dd>
            <span className={`status-chip status-chip--${detection.status}`}>
              {detection.status}
            </span>
          </dd>
        </dl>
      )}
    </div>
  );
}
