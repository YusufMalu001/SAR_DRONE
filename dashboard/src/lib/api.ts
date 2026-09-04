import type {
  AlertRecord,
  DetectionRecord,
  MissionSummary,
  RouteStatus,
  ServiceHealth,
} from "./types";

const INFERENCE_URL =
  import.meta.env.VITE_INFERENCE_URL ?? "http://localhost:8001";
export const FUSION_URL =
  import.meta.env.VITE_FUSION_URL ?? "http://localhost:8002";

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`${url} responded ${res.status}`);
  }
  return res.json();
}

async function fetchHealth(url: string): Promise<ServiceHealth> {
  return getJSON(`${url}/health`);
}

export function fetchInferenceHealth() {
  return fetchHealth(INFERENCE_URL);
}

export function fetchFusionHealth() {
  return fetchHealth(FUSION_URL);
}

export function fetchDetections(limit = 200): Promise<DetectionRecord[]> {
  return getJSON(`${FUSION_URL}/detections?limit=${limit}`);
}

export function fetchAlerts(limit = 100): Promise<AlertRecord[]> {
  return getJSON(`${FUSION_URL}/alerts?limit=${limit}`);
}

export function fetchMissionSummary(): Promise<MissionSummary> {
  return getJSON(`${FUSION_URL}/mission/summary`);
}

export function fetchRouteStatus(): Promise<RouteStatus> {
  return getJSON(`${FUSION_URL}/mission/route-status`);
}

// Demo/debug control: forces fusion-service's connectivity monitor into
// (or out of) offline mode, so the offline-queue-then-sync behavior can be
// demonstrated on demand. See fusion-service's internal/connectivity.
export async function setOffline(offline: boolean): Promise<{ offline: boolean }> {
  const res = await fetch(`${FUSION_URL}/debug/set-offline`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ offline }),
  });
  if (!res.ok) {
    throw new Error(`POST /debug/set-offline responded ${res.status}`);
  }
  return res.json();
}

export function fetchOfflineStatus(): Promise<{ offline: boolean }> {
  return getJSON(`${FUSION_URL}/debug/offline-status`);
}
