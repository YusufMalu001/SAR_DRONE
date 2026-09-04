import { useState } from "react";
import AlertFeed from "../components/AlertFeed";
import DebugPanel from "../components/DebugPanel";
import DetailPanel from "../components/DetailPanel";
import HealthStrip from "../components/HealthStrip";
import MapView from "../components/MapView";
import StatusBar from "../components/StatusBar";
import { useFusionData } from "../hooks/useFusionData";
import { useRouteStatus } from "../hooks/useRouteStatus";
import type { DetectionRecord } from "../lib/types";
import "./LiveMap.css";

export default function LiveMap() {
  const {
    socketConnected,
    connectivityStatus,
    detections,
    alerts,
    detectionsPerMinute,
    avgFusionLatencyMs,
  } = useFusionData();
  const routeStatus = useRouteStatus();

  const [selected, setSelected] = useState<DetectionRecord | null>(null);

  return (
    <section className="ops-dashboard">
      <div className="ops-dashboard__top">
        <StatusBar
          socketConnected={socketConnected}
          connectivityStatus={connectivityStatus}
        />
        <HealthStrip
          detectionsPerMinute={detectionsPerMinute}
          avgFusionLatencyMs={avgFusionLatencyMs}
        />
      </div>

      <DebugPanel />

      <div className="ops-dashboard__body">
        <div className="ops-dashboard__map">
          <MapView
            detections={detections}
            routeSegments={routeStatus?.segments}
            selectedId={selected?.id ?? null}
            onSelect={setSelected}
          />
        </div>

        <div className="ops-dashboard__side">
          <DetailPanel detection={selected} onClose={() => setSelected(null)} />
          <AlertFeed alerts={alerts} />
        </div>
      </div>
    </section>
  );
}
