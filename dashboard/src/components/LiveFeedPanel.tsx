import { useState } from "react";
import type { DetectionRecord, Source } from "../lib/types";

interface LiveFeedPanelProps {
  detections: DetectionRecord[];
}

export default function LiveFeedPanel({ detections }: LiveFeedPanelProps) {
  const [activeTab, setActiveTab] = useState<Source>("rgb");

  // Filter detections matching the selected modality or containing it
  const matchingDetections = detections.filter(
    (d) => d.source === activeTab || d.modalities?.includes(activeTab)
  );

  return (
    <div className="panel live-feed-panel" role="region" aria-label="Camera Feeds">
      <div className="panel__header">
        <h3 className="panel__title">Camera Feed</h3>
        <div className="feed-tabs" role="tablist">
          <button
            role="tab"
            aria-selected={activeTab === "rgb"}
            className={`feed-tab ${activeTab === "rgb" ? "feed-tab--active" : ""}`}
            onClick={() => setActiveTab("rgb")}
          >
            RGB Optical
          </button>
          <button
            role="tab"
            aria-selected={activeTab === "thermal"}
            className={`feed-tab ${activeTab === "thermal" ? "feed-tab--active" : ""}`}
            onClick={() => setActiveTab("thermal")}
          >
            LWIR Thermal
          </button>
        </div>
      </div>

      <div className={`video-container video-container--${activeTab}`}>
        <div className="video-viewport">
          <div className="feed-overlay-hud">
            <span className="hud-badge">{activeTab.toUpperCase()} SENSOR ACTIVE</span>
            <span className="hud-metric">
              Detections in view: {matchingDetections.length}
            </span>
          </div>

          <div className="detections-overlay" aria-hidden="true">
            {matchingDetections.slice(0, 4).map((d, idx) => {
              // Position overlays cleanly across viewport quadrants
              const positions = [
                { top: "25%", left: "30%", width: "24%", height: "35%" },
                { top: "35%", left: "60%", width: "20%", height: "30%" },
                { top: "50%", left: "20%", width: "22%", height: "28%" },
                { top: "15%", left: "55%", width: "26%", height: "32%" },
              ];
              const pos = positions[idx % positions.length];
              const isSurvivor = d.class === "person";
              const boxColor = isSurvivor ? "#ef4444" : "#f97316";

              return (
                <div
                  key={d.id}
                  className="detection-bbox"
                  style={{
                    position: "absolute",
                    top: pos.top,
                    left: pos.left,
                    width: pos.width,
                    height: pos.height,
                    border: `2px solid ${boxColor}`,
                    backgroundColor: `${boxColor}22`,
                  }}
                >
                  <span
                    className="detection-bbox__tag"
                    style={{ backgroundColor: boxColor }}
                  >
                    {d.class} {(d.confidence * 100).toFixed(0)}%
                  </span>
                </div>
              );
            })}
          </div>

          <div className="sensor-reticle">
            <div className="reticle-crosshair" />
          </div>
        </div>
      </div>
    </div>
  );
}
