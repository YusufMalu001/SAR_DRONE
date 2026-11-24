import { useCallback, useEffect, useRef, useState } from "react";
import {
  FUSION_URL,
  fetchAlerts,
  fetchCoverageGeoJSON,
  fetchDetections,
  fetchEntities,
  fetchHazardsGeoJSON,
  fetchSurvivorSafeRoutes,
} from "../lib/api";
import type {
  AlertRecord,
  DetectionRecord,
  EntityRecord,
  GeoJSONFeatureCollection,
  Telemetry,
  WSMessage,
} from "../lib/types";

const METRICS_WINDOW_MS = 60_000;
const METRICS_TICK_MS = 2_000;

export type ConnectivityStatus = "live" | "offline-queue" | "unknown";

interface LatencySample {
  arrivedAtMs: number;
  latencyMs: number;
}

export interface FusionData {
  socketConnected: boolean;
  connectivityStatus: ConnectivityStatus;
  detections: DetectionRecord[];
  entities: EntityRecord[];
  alerts: AlertRecord[];
  hazardsGeoJSON: GeoJSONFeatureCollection | null;
  coverageGeoJSON: GeoJSONFeatureCollection | null;
  safeRoutesGeoJSON: GeoJSONFeatureCollection | null;
  telemetryHistory: Telemetry[];
  detectionsPerMinute: number;
  avgFusionLatencyMs: number | null;
  refreshData: () => Promise<void>;
  updateEntity: (entity: EntityRecord) => void;
}

function upsertDetection(
  list: DetectionRecord[],
  next: DetectionRecord
): DetectionRecord[] {
  const idx = list.findIndex((d) => d.id === next.id);
  if (idx === -1) return [next, ...list];
  const copy = list.slice();
  copy[idx] = next;
  return copy;
}

function upsertEntityFromDetection(
  entities: EntityRecord[],
  det: DetectionRecord
): EntityRecord[] {
  const existingIdx = entities.findIndex(
    (e) => e.id === det.id || (e.track_ids && det.track_id && e.track_ids.includes(det.track_id))
  );

  if (existingIdx !== -1) {
    const copy = [...entities];
    const prev = copy[existingIdx];
    copy[existingIdx] = {
      ...prev,
      lat: det.lat,
      lon: det.lon,
      confidence: Math.max(prev.confidence, det.confidence),
      priority_score: Math.max(prev.priority_score, det.priority_score),
      last_seen: det.detected_at,
      observation_count: prev.observation_count + 1,
    };
    return copy;
  }

  const isSurvivor = det.class === "person";
  const newEntity: EntityRecord = {
    id: det.id,
    type: isSurvivor ? "survivor" : "hazard",
    class: det.class,
    lat: det.lat,
    lon: det.lon,
    uncertainty_radius: 5.0,
    track_ids: det.track_id ? [det.track_id] : [],
    observation_count: 1,
    confidence: det.confidence,
    modalities: det.source ? [det.source] : ["rgb"],
    priority_score: det.priority_score,
    status: "active",
    first_seen: det.detected_at,
    last_seen: det.detected_at,
  };

  return [newEntity, ...entities];
}

export function useFusionData(): FusionData {
  const [socketConnected, setSocketConnected] = useState(false);
  const [connectivityStatus, setConnectivityStatus] =
    useState<ConnectivityStatus>("unknown");
  const [detections, setDetections] = useState<DetectionRecord[]>([]);
  const [entities, setEntities] = useState<EntityRecord[]>([]);
  const [alerts, setAlerts] = useState<AlertRecord[]>([]);
  const [hazardsGeoJSON, setHazardsGeoJSON] = useState<GeoJSONFeatureCollection | null>(null);
  const [coverageGeoJSON, setCoverageGeoJSON] = useState<GeoJSONFeatureCollection | null>(null);
  const [safeRoutesGeoJSON, setSafeRoutesGeoJSON] = useState<GeoJSONFeatureCollection | null>(null);
  const [telemetryHistory, setTelemetryHistory] = useState<Telemetry[]>([]);
  const [detectionsPerMinute, setDetectionsPerMinute] = useState(0);
  const [avgFusionLatencyMs, setAvgFusionLatencyMs] = useState<number | null>(null);

  const samplesRef = useRef<LatencySample[]>([]);
  const backoffRef = useRef<number>(1000);

  const recomputeMetrics = useCallback(() => {
    const cutoff = Date.now() - METRICS_WINDOW_MS;
    const fresh = samplesRef.current.filter((s) => s.arrivedAtMs >= cutoff);
    samplesRef.current = fresh;

    setDetectionsPerMinute(
      Math.round(fresh.length * (60_000 / METRICS_WINDOW_MS))
    );
    setAvgFusionLatencyMs(
      fresh.length === 0
        ? null
        : fresh.reduce((sum, s) => sum + s.latencyMs, 0) / fresh.length
    );
  }, []);

  useEffect(() => {
    const tick = setInterval(recomputeMetrics, METRICS_TICK_MS);
    return () => clearInterval(tick);
  }, [recomputeMetrics]);

  const refreshData = useCallback(async () => {
    try {
      const [dets, ents, alrts, hazGeo, covGeo, safeGeo] = await Promise.allSettled([
        fetchDetections(200),
        fetchEntities(),
        fetchAlerts(100),
        fetchHazardsGeoJSON(),
        fetchCoverageGeoJSON(),
        fetchSurvivorSafeRoutes(),
      ]);

      if (dets.status === "fulfilled") setDetections(dets.value);
      if (ents.status === "fulfilled") setEntities(ents.value);
      if (alrts.status === "fulfilled") setAlerts(alrts.value);
      if (hazGeo.status === "fulfilled") setHazardsGeoJSON(hazGeo.value);
      if (covGeo.status === "fulfilled") setCoverageGeoJSON(covGeo.value);
      if (safeGeo.status === "fulfilled") setSafeRoutesGeoJSON(safeGeo.value);
    } catch (err) {
      console.error("refreshData failed:", err);
    }
  }, []);

  const updateEntity = useCallback((updated: EntityRecord) => {
    setEntities((prev) =>
      prev.map((e) => (e.id === updated.id ? updated : e))
    );
  }, []);

  useEffect(() => {
    refreshData();
    const interval = setInterval(refreshData, 10000);
    return () => clearInterval(interval);
  }, [refreshData]);

  // WebSocket with exponential backoff & jitter
  useEffect(() => {
    let socket: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
    let unmounted = false;

    const wsURL = FUSION_URL.replace(/^http/, "ws") + "/ws/live";

    function scheduleReconnect() {
      if (unmounted) return;
      const currentDelay = backoffRef.current;
      const jitter = Math.random() * 500;
      const nextDelay = Math.min(16000, currentDelay * 2);
      backoffRef.current = nextDelay;

      reconnectTimer = setTimeout(connect, currentDelay + jitter);
    }

    function connect() {
      if (unmounted) return;

      socket = new WebSocket(wsURL);

      socket.onopen = () => {
        setSocketConnected(true);
        backoffRef.current = 1000; // Reset backoff on successful connect
      };

      socket.onclose = () => {
        setSocketConnected(false);
        scheduleReconnect();
      };

      socket.onerror = () => {
        socket?.close();
      };

      socket.onmessage = (event) => {
        const arrivedAtMs = Date.now();
        let envelope: WSMessage;
        try {
          envelope = JSON.parse(event.data);
        } catch (err) {
          console.error("failed to parse /ws/live message:", err);
          return;
        }

        if (envelope.type === "detection") {
          const record = envelope.payload as DetectionRecord;
          setDetections((prev) => upsertDetection(prev, record));
          setEntities((prev) => upsertEntityFromDetection(prev, record));

          setConnectivityStatus(
            record.status === "queued" ? "offline-queue" : "live"
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
        } else if (envelope.type === "telemetry") {
          const telem = envelope.payload as Telemetry;
          setTelemetryHistory((prev) => [...prev.slice(-100), telem]);
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
    entities,
    alerts,
    hazardsGeoJSON,
    coverageGeoJSON,
    safeRoutesGeoJSON,
    telemetryHistory,
    detectionsPerMinute,
    avgFusionLatencyMs,
    refreshData,
    updateEntity,
  };
}
