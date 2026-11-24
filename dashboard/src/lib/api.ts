import type {
  AlertRecord,
  CoveragePlanResponse,
  DetectionRecord,
  EntityRecord,
  GeoJSONFeatureCollection,
  MissionStatusResponse,
  MissionSummary,
  RouteStatus,
  ServiceHealth,
} from "./types";

export const INFERENCE_URL =
  import.meta.env.VITE_INFERENCE_URL ?? "http://localhost:8001";
export const FUSION_URL =
  import.meta.env.VITE_FUSION_URL ?? "http://localhost:8002";
export const MISSION_URL =
  import.meta.env.VITE_MISSION_URL ?? "http://localhost:8003";

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`${url} responded ${res.status}`);
  }
  return res.json();
}

async function postJSON<T>(url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    throw new Error(`${url} responded ${res.status}`);
  }
  return res.json();
}

export function fetchInferenceHealth(): Promise<ServiceHealth> {
  return getJSON(`${INFERENCE_URL}/health`);
}

export function fetchFusionHealth(): Promise<ServiceHealth> {
  return getJSON(`${FUSION_URL}/health`);
}

export function fetchMissionHealth(): Promise<ServiceHealth> {
  return getJSON(`${MISSION_URL}/health`);
}

export function fetchDetections(limit = 200): Promise<DetectionRecord[]> {
  return getJSON(`${FUSION_URL}/detections?limit=${limit}`);
}

export function fetchEntities(type?: string, status?: string): Promise<EntityRecord[]> {
  const params = new URLSearchParams();
  if (type) params.set("type", type);
  if (status) params.set("status", status);
  const q = params.toString() ? `?${params.toString()}` : "";
  return getJSON(`${FUSION_URL}/entities${q}`);
}

export function fetchEntity(id: string): Promise<EntityRecord> {
  return getJSON(`${FUSION_URL}/entities/${id}`);
}

export function resolveEntity(id: string, status = "rescued"): Promise<EntityRecord> {
  return postJSON(`${FUSION_URL}/entities/${id}/resolve`, { status });
}

export function fetchAlerts(limit = 100): Promise<AlertRecord[]> {
  return getJSON(`${FUSION_URL}/alerts?limit=${limit}`);
}

export function ackAlert(id: number): Promise<{ status: string }> {
  return postJSON(`${FUSION_URL}/alerts/${id}/ack`);
}

export function resolveAlert(id: number): Promise<{ status: string }> {
  return postJSON(`${FUSION_URL}/alerts/${id}/resolve`);
}

export function fetchHazardsGeoJSON(): Promise<GeoJSONFeatureCollection> {
  return getJSON(`${FUSION_URL}/hazards/geojson`);
}

export function fetchCoverageGeoJSON(): Promise<GeoJSONFeatureCollection> {
  return getJSON(`${FUSION_URL}/mission/coverage`);
}

export function fetchSurvivorSafeRoutes(): Promise<GeoJSONFeatureCollection> {
  return getJSON(`${FUSION_URL}/mission/routes/survivors`);
}

export function fetchMissionSummary(): Promise<MissionSummary> {
  return getJSON(`${FUSION_URL}/mission/summary`);
}

export function fetchRouteStatus(): Promise<RouteStatus> {
  return getJSON(`${FUSION_URL}/mission/route-status`);
}

export function getReportDownloadURL(format: "html" | "json", missionId?: string): string {
  const base = missionId ? `${FUSION_URL}/missions/${missionId}/report` : `${FUSION_URL}/mission/report`;
  return `${base}?format=${format}`;
}

// Mission service API calls
export function fetchMissionStatus(): Promise<MissionStatusResponse> {
  return getJSON(`${MISSION_URL}/mission/status`);
}

export function planCoverageMission(
  polygon: Array<{ lat: number; lon: number }>,
  altitudeM = 50.0,
  speedMps = 5.0
): Promise<CoveragePlanResponse> {
  return postJSON(`${MISSION_URL}/mission/plan`, {
    polygon,
    altitude_m: altitudeM,
    speed_mps: speedMps,
  });
}

export function startMission(): Promise<{ status: string; mission_state: string }> {
  return postJSON(`${MISSION_URL}/mission/start`);
}

export function pauseMission(): Promise<{ status: string; mission_state: string }> {
  return postJSON(`${MISSION_URL}/mission/pause`);
}

export function resumeMission(): Promise<{ status: string; mission_state: string }> {
  return postJSON(`${MISSION_URL}/mission/resume`);
}

export function abortMission(): Promise<{ status: string; mission_state: string }> {
  return postJSON(`${MISSION_URL}/mission/abort`);
}

// Debug/demo controls
export async function setOffline(offline: boolean): Promise<{ offline: boolean }> {
  return postJSON(`${FUSION_URL}/debug/set-offline`, { offline });
}

export function fetchOfflineStatus(): Promise<{ offline: boolean }> {
  return getJSON(`${FUSION_URL}/debug/offline-status`);
}
