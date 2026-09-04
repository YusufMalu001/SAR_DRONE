import { useCallback, useEffect, useRef, useState } from "react";
import { FUSION_URL, fetchAlerts, fetchDetections } from "../lib/api";
import type { AlertRecord, DetectionRecord, WSMessage } from "../lib/types";

// How far back to keep latency/throughput samples for the health strip.
const METRICS_WINDOW_MS = 60_000;
// How often to recompute health-strip metrics (so they decay to 0 even
// when no new messages are arriving, not just when one shows up).
const METRICS_TICK_MS = 2_000;
// How often to retry the WebSocket after it drops.
const RECONNECT_DELAY_MS = 2_000;

export type ConnectivityStatus = "live" | "offline-queue" | "unknown";

interface LatencySample {
  arrivedAtMs: number;
  latencyMs: number;
}

export interface FusionData {
  /** Whether the /ws/live socket itself is currently open. */
  socketConnected: boolean;
  /** Derived from the most recent "detection" message's status field. */
  connectivityStatus: ConnectivityStatus;
  detections: DetectionRecord[];
  alerts: AlertRecord[];
  detectionsPerMinute: number;
  avgFusionLatencyMs: number | null;
}

function upsertDetection(
  list: DetectionRecord[],
  next: DetectionRecord,
): DetectionRecord[] {
  const idx = list.findIndex((d) => d.id === next.id);
  if (idx === -1) return [next, ...list];
  const copy = list.slice();
  copy[idx] = next;
  return copy;
}

export function useFusionData(): FusionData {
  const [socketConnected, setSocketConnected] = useState(false);
  const [connectivityStatus, setConnectivityStatus] =
    useState<ConnectivityStatus>("unknown");
  const [detections, setDetections] = useState<DetectionRecord[]>([]);
  const [alerts, setAlerts] = useState<AlertRecord[]>([]);
  const [detectionsPerMinute, setDetectionsPerMinute] = useState(0);
  const [avgFusionLatencyMs, setAvgFusionLatencyMs] = useState<number | null>(
    null,
  );

  const samplesRef = useRef<LatencySample[]>([]);

  const recomputeMetrics = useCallback(() => {
    const cutoff = Date.now() - METRICS_WINDOW_MS;
    const fresh = samplesRef.current.filter((s) => s.arrivedAtMs >= cutoff);
    samplesRef.current = fresh;

    setDetectionsPerMinute(
      Math.round(fresh.length * (60_000 / METRICS_WINDOW_MS)),
    );
    setAvgFusionLatencyMs(
      fresh.length === 0
        ? null
        : fresh.reduce((sum, s) => sum + s.latencyMs, 0) / fresh.length,
    );
  }, []);

  useEffect(() => {
    const tick = setInterval(recomputeMetrics, METRICS_TICK_MS);
    return () => clearInterval(tick);
  }, [recomputeMetrics]);

  // Seed from REST so the dashboard isn't empty while the WebSocket
  // connects (and so history from before this page loaded still shows).
  useEffect(() => {
    let cancelled = false;

    fetchDetections()
      .then((rows) => {
        if (!cancelled) setDetections(rows);
      })
      .catch((err) => console.error("fetchDetections failed:", err));

    fetchAlerts()
      .then((rows) => {
        if (!cancelled) setAlerts(rows);
      })
      .catch((err) => console.error("fetchAlerts failed:", err));

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let unmounted = false;

    const wsURL = FUSION_URL.replace(/^http/, "ws") + "/ws/live";

    function connect() {
      if (unmounted) return;

      socket = new WebSocket(wsURL);

      socket.onopen = () => setSocketConnected(true);

      socket.onclose = () => {
        setSocketConnected(false);
        if (!unmounted) {
          reconnectTimer = setTimeout(connect, RECONNECT_DELAY_MS);
        }
      };

      socket.onerror = () => {
        // onclose fires right after; the reconnect loop lives there.
        socket?.close();
      };

      socket.onmessage = (event) => {
        const arrivedAtMs = Date.now();
        let envelope: WSMessage;
        try {
          envelope = JSON.parse(event.data);
        } catch (err) {
          console.error("failed to parse /ws/live message:", err, event.data);
          return;
        }

        if (envelope.type === "detection") {
          const record = envelope.payload as DetectionRecord;
          setDetections((prev) => upsertDetection(prev, record));

          setConnectivityStatus(
            record.status === "queued" ? "offline-queue" : "live",
          );

          const detectedAtMs = Date.parse(record.detected_at);
          if (!Number.isNaN(detectedAtMs)) {
            samplesRef.current.push({
              arrivedAtMs,
              latencyMs: Math.max(0, arrivedAtMs - detectedAtMs),
            });
            recomputeMetrics();
          }
        } else if (envelope.type === "alert") {
          const alert = envelope.payload as AlertRecord;
          setAlerts((prev) => [
            alert,
            ...prev.filter((a) => a.id !== alert.id),
          ]);
        }
      };
    }

    connect();

    return () => {
      unmounted = true;
      if (reconnectTimer) clearTimeout(reconnectTimer);
      socket?.close();
    };
  }, [recomputeMetrics]);

  return {
    socketConnected,
    connectivityStatus,
    detections,
    alerts,
    detectionsPerMinute,
    avgFusionLatencyMs,
  };
}
