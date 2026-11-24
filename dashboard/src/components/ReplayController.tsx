import { useEffect, useState } from "react";
import type { DetectionRecord } from "../lib/types";

interface ReplayControllerProps {
  detections: DetectionRecord[];
  active: boolean;
  onToggleActive: (active: boolean) => void;
  onFilterTime: (timestampMs: number | null) => void;
}

export default function ReplayController({
  detections,
  active,
  onToggleActive,
  onFilterTime,
}: ReplayControllerProps) {
  const [isPlaying, setIsPlaying] = useState(false);
  const [speed, setSpeed] = useState<number>(1);
  const [currentTimeMs, setCurrentTimeMs] = useState<number | null>(null);

  // Compute time bounds from detections
  const timestamps = detections
    .map((d) => Date.parse(d.detected_at))
    .filter((t) => !Number.isNaN(t));

  const minTimeMs = timestamps.length > 0 ? Math.min(...timestamps) : Date.now() - 60000;
  const maxTimeMs = timestamps.length > 0 ? Math.max(...timestamps) : Date.now();

  useEffect(() => {
    if (!active) {
      setIsPlaying(false);
      setCurrentTimeMs(null);
      onFilterTime(null);
      return;
    }
    if (currentTimeMs === null) {
      setCurrentTimeMs(maxTimeMs);
      onFilterTime(maxTimeMs);
    }
  }, [active, maxTimeMs, onFilterTime]);

  useEffect(() => {
    if (!active || !isPlaying || currentTimeMs === null) return;

    const timer = setInterval(() => {
      setCurrentTimeMs((prev) => {
        if (prev === null) return minTimeMs;
        const next = prev + 1000 * speed;
        if (next >= maxTimeMs) {
          setIsPlaying(false);
          return maxTimeMs;
        }
        onFilterTime(next);
        return next;
      });
    }, 200);

    return () => clearInterval(timer);
  }, [active, isPlaying, speed, minTimeMs, maxTimeMs, onFilterTime]);

  const handleSliderChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = Number(e.target.value);
    setCurrentTimeMs(val);
    onFilterTime(val);
  };

  const progressPct =
    maxTimeMs > minTimeMs && currentTimeMs !== null
      ? Math.min(100, Math.max(0, ((currentTimeMs - minTimeMs) / (maxTimeMs - minTimeMs)) * 100))
      : 100;

  return (
    <div className="replay-controller-bar" role="region" aria-label="Mission Replay Controls">
      <div className="replay-toggle-group">
        <button
          className={`replay-btn ${active ? "replay-btn--active" : ""}`}
          onClick={() => onToggleActive(!active)}
        >
          {active ? "Exit Replay" : "Replay Mode"}
        </button>
      </div>

      {active && (
        <div className="replay-controls-row">
          <button
            className="replay-play-btn"
            onClick={() => setIsPlaying(!isPlaying)}
            aria-label={isPlaying ? "Pause replay" : "Play replay"}
          >
            {isPlaying ? "❚❚ Pause" : "▶ Play"}
          </button>

          <button
            className="replay-speed-btn mono"
            onClick={() => {
              const speeds = [1, 2, 5];
              const idx = speeds.indexOf(speed);
              setSpeed(speeds[(idx + 1) % speeds.length]);
            }}
          >
            {speed}x
          </button>

          <div className="scrubber-wrapper">
            <input
              type="range"
              min={minTimeMs}
              max={maxTimeMs}
              value={currentTimeMs ?? maxTimeMs}
              onChange={handleSliderChange}
              className="replay-slider"
              aria-label="Replay timeline scrubber"
            />
            <div className="scrubber-labels mono">
              <span>{new Date(minTimeMs).toLocaleTimeString()}</span>
              <span className="scrubber-current">
                {currentTimeMs ? new Date(currentTimeMs).toLocaleTimeString() : "Live"} ({progressPct.toFixed(0)}%)
              </span>
              <span>{new Date(maxTimeMs).toLocaleTimeString()}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
