import { useEffect, useState } from "react";
import AlertFeed from "../components/AlertFeed";
import DebugPanel from "../components/DebugPanel";
import DegradedBanner from "../components/DegradedBanner";
import EntityDetailDrawer from "../components/EntityDetailDrawer";
import { ErrorBoundary } from "../components/ErrorBoundary";
import HazardsList from "../components/HazardsList";
import HealthStrip from "../components/HealthStrip";
import LiveFeedPanel from "../components/LiveFeedPanel";
import MapView from "../components/MapView";
import MissionControlPanel from "../components/MissionControlPanel";
import ReplayController from "../components/ReplayController";
import StatusBar from "../components/StatusBar";
import SurvivorsList from "../components/SurvivorsList";
import { useFusionData } from "../hooks/useFusionData";
import { useRouteStatus } from "../hooks/useRouteStatus";
import {
  fetchFusionHealth,
  fetchInferenceHealth,
  fetchMissionHealth,
  getReportDownloadURL,
} from "../lib/api";
import type {
  CoveragePlanResponse,
  EntityRecord,
  ServiceHealth,
} from "../lib/types";
import "./LiveMap.css";

export default function LiveMap() {
  const {
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
  } = useFusionData();

  const routeStatus = useRouteStatus();

  // Health states for degraded banner
  const [fusionHealth, setFusionHealth] = useState<ServiceHealth | null>(null);
  const [inferenceHealth, setInferenceHealth] = useState<ServiceHealth | null>(null);
  const [missionHealth, setMissionHealth] = useState<ServiceHealth | null>(null);

  // UI Selections & Replay
  const [selectedEntity, setSelectedEntity] = useState<EntityRecord | null>(null);
  const [drawnPolygon, setDrawnPolygon] = useState<Array<{ lat: number; lon: number }>>([]);
  const [plannedPath, setPlannedPath] = useState<CoveragePlanResponse | null>(null);
  const [replayActive, setReplayActive] = useState(false);
  const [replayFilterTime, setReplayFilterTime] = useState<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function checkHealth() {
      const [f, i, m] = await Promise.allSettled([
        fetchFusionHealth(),
        fetchInferenceHealth(),
        fetchMissionHealth(),
      ]);
      if (cancelled) return;
      if (f.status === "fulfilled") setFusionHealth(f.value);
      if (i.status === "fulfilled") setInferenceHealth(i.value);
      if (m.status === "fulfilled") setMissionHealth(m.value);
    }
    checkHealth();
    const timer = setInterval(checkHealth, 3000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, []);

  // Filter entities and detections when replay scrubber is active
  const displayedDetections = replayActive && replayFilterTime !== null
    ? detections.filter((d) => Date.parse(d.detected_at) <= replayFilterTime)
    : detections;

  const displayedEntities = replayActive && replayFilterTime !== null
    ? entities.filter((e) => Date.parse(e.first_seen) <= replayFilterTime)
    : entities;

  const survivors = displayedEntities.filter((e) => e.type === "survivor");
  const hazards = displayedEntities.filter((e) => e.type === "hazard");

  return (
    <ErrorBoundary fallbackTitle="Command Centre Error">
      <div className="ops-dashboard">
        {/* Top Header & Health Bar */}
        <header className="ops-dashboard__top">
          <div className="ops-top-left">
            <StatusBar
              socketConnected={socketConnected}
              connectivityStatus={connectivityStatus}
            />
            <div className="report-buttons-group">
              <a
                href={getReportDownloadURL("html")}
                target="_blank"
                rel="noreferrer"
                className="report-download-btn report-download-btn--html"
                title="Download or print SitRep HTML report"
              >
                SitRep (HTML)
              </a>
              <a
                href={getReportDownloadURL("json")}
                target="_blank"
                rel="noreferrer"
                className="report-download-btn report-download-btn--json"
                title="Download raw SitRep JSON"
              >
                SitRep (JSON)
              </a>
            </div>
          </div>

          <HealthStrip
            detectionsPerMinute={detectionsPerMinute}
            avgFusionLatencyMs={avgFusionLatencyMs}
          />
        </header>

        {/* Degraded & Offline Mode Banner */}
        <DegradedBanner
          fusionHealth={fusionHealth}
          inferenceHealth={inferenceHealth}
          missionHealth={missionHealth}
        />

        <DebugPanel />

        {/* Main Multi-Pane Command Center Grid */}
        <main className="ops-dashboard__body">
          {/* Main Map Column */}
          <section className="ops-dashboard__map-section">
            <MapView
              survivors={survivors}
              hazards={hazards}
              hazardsGeoJSON={hazardsGeoJSON}
              coverageGeoJSON={coverageGeoJSON}
              safeRoutesGeoJSON={safeRoutesGeoJSON}
              routeSegments={routeStatus?.segments}
              telemetryHistory={telemetryHistory}
              plannedWaypoints={plannedPath?.waypoints}
              selectedId={selectedEntity?.id}
              onSelectEntity={(ent) => setSelectedEntity(ent)}
              drawnPolygon={drawnPolygon}
              onPolygonChange={setDrawnPolygon}
            />

            {/* Replay Mode Scrubber Bar */}
            <ReplayController
              detections={detections}
              active={replayActive}
              onToggleActive={setReplayActive}
              onFilterTime={setReplayFilterTime}
            />
          </section>

          {/* Side Command Panels */}
          <aside className="ops-dashboard__side" aria-label="Command Panels">
            {/* Live Camera Feed with Overlays (RGB & Thermal tabs) */}
            <LiveFeedPanel detections={displayedDetections} />

            {/* Mission Control Panel */}
            <MissionControlPanel
              drawnPolygon={drawnPolygon}
              onPlanPreview={setPlannedPath}
              onClearPolygon={() => setDrawnPolygon([])}
            />

            {/* Survivors List (Priority Coloured) */}
            <SurvivorsList
              survivors={survivors}
              selectedId={selectedEntity?.id ?? null}
              onSelect={(s) => setSelectedEntity(s)}
            />

            {/* Hazards List */}
            <HazardsList
              hazards={hazards}
              selectedId={selectedEntity?.id ?? null}
              onSelect={(h) => setSelectedEntity(h)}
            />

            {/* Alert Feed */}
            <AlertFeed alerts={alerts} onAlertUpdated={refreshData} />
          </aside>
        </main>

        {/* Entity Detail Drawer */}
        {selectedEntity && (
          <EntityDetailDrawer
            entity={selectedEntity}
            hazards={hazards}
            alerts={alerts}
            onClose={() => setSelectedEntity(null)}
            onEntityUpdated={(updated) => {
              setSelectedEntity(updated);
              updateEntity(updated);
            }}
          />
        )}
      </div>
    </ErrorBoundary>
  );
}
