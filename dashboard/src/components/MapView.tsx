import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { useEffect, useRef, useState } from "react";
import type {
  CoveragePlanWaypoint,
  DetectionClass,
  EntityRecord,
  GeoJSONFeatureCollection,
  RouteSegment,
  Telemetry,
} from "../lib/types";

export const CLASS_COLORS: Record<DetectionClass, string> = {
  person: "#ef4444",
  fire: "#f97316",
  flood: "#3b82f6",
  structural_damage: "#f59e0b",
  debris: "#9ca3af",
};

export const PRIORITY_COLORS: Record<string, string> = {
  critical: "#ef4444",
  high: "#f97316",
  medium: "#f59e0b",
  low: "#10b981",
};

const DEFAULT_CENTER: L.LatLngTuple = [20, 78];

interface MapViewProps {
  survivors: EntityRecord[];
  hazards: EntityRecord[];
  hazardsGeoJSON?: GeoJSONFeatureCollection | null;
  coverageGeoJSON?: GeoJSONFeatureCollection | null;
  safeRoutesGeoJSON?: GeoJSONFeatureCollection | null;
  routeSegments?: RouteSegment[];
  telemetryHistory?: Telemetry[];
  plannedWaypoints?: CoveragePlanWaypoint[];
  selectedId?: string | null;
  onSelectEntity: (entity: EntityRecord) => void;
  drawnPolygon?: Array<{ lat: number; lon: number }>;
  onPolygonChange?: (polygon: Array<{ lat: number; lon: number }>) => void;
}

export default function MapView({
  survivors,
  hazards,
  hazardsGeoJSON,
  coverageGeoJSON,
  safeRoutesGeoJSON,
  routeSegments,
  telemetryHistory,
  plannedWaypoints,
  selectedId,
  onSelectEntity,
  drawnPolygon = [],
  onPolygonChange,
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<L.Map | null>(null);

  // Layer groups
  const survivorsLayerRef = useRef<L.LayerGroup | null>(null);
  const hazardsLayerRef = useRef<L.GeoJSON | null>(null);
  const safeRoutesLayerRef = useRef<L.GeoJSON | null>(null);
  const coverageLayerRef = useRef<L.GeoJSON | null>(null);
  const droneTrackLayerRef = useRef<L.LayerGroup | null>(null);
  const plannedPathLayerRef = useRef<L.LayerGroup | null>(null);
  const drawLayerRef = useRef<L.LayerGroup | null>(null);

  // Layer toggle states
  const [showSurvivors, setShowSurvivors] = useState(true);
  const [showHazards, setShowHazards] = useState(true);
  const [showSafeRoutes, setShowSafeRoutes] = useState(true);
  const [showCoverage, setShowCoverage] = useState(true);
  const [showDroneTrack, setShowDroneTrack] = useState(true);
  const [showPlannedPath, setShowPlannedPath] = useState(true);
  const [isDrawing, setIsDrawing] = useState(false);
  const [tileServerOnline, setTileServerOnline] = useState(true);

  const hasFitRef = useRef(false);
  const onSelectEntityRef = useRef(onSelectEntity);
  onSelectEntityRef.current = onSelectEntity;

  // Initialize Map
  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = L.map(containerRef.current, {
      center: DEFAULT_CENTER,
      zoom: 14,
      preferCanvas: true,
    });

    const fusionUrl = import.meta.env.VITE_FUSION_URL || "http://localhost:8002";
    const tileUrl = `${fusionUrl}/tiles/{z}/{x}/{y}.png`;
    const tileLayer = L.tileLayer(tileUrl, {
      attribution: "&copy; Khoj-o-Drone Offline Map / OpenStreetMap contributors",
      maxZoom: 19,
    }).addTo(map);

    tileLayer.on("tileerror", () => {
      setTileServerOnline(false);
    });
    tileLayer.on("tileload", () => {
      setTileServerOnline(true);
    });

    survivorsLayerRef.current = L.layerGroup().addTo(map);
    droneTrackLayerRef.current = L.layerGroup().addTo(map);
    plannedPathLayerRef.current = L.layerGroup().addTo(map);
    drawLayerRef.current = L.layerGroup().addTo(map);

    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // Handle map clicks for polygon drawing
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    function handleMapClick(e: L.LeafletMouseEvent) {
      if (!isDrawing || !onPolygonChange) return;
      const next = [...drawnPolygon, { lat: e.latlng.lat, lon: e.latlng.lng }];
      onPolygonChange(next);
    }

    map.on("click", handleMapClick);
    return () => {
      map.off("click", handleMapClick);
    };
  }, [isDrawing, drawnPolygon, onPolygonChange]);

  // Render drawn polygon
  useEffect(() => {
    const layer = drawLayerRef.current;
    if (!layer) return;
    layer.clearLayers();

    if (drawnPolygon.length > 0) {
      const latlngs = drawnPolygon.map((p) => [p.lat, p.lon] as L.LatLngTuple);
      for (const p of drawnPolygon) {
        L.circleMarker([p.lat, p.lon], {
          radius: 5,
          color: "#38bdf8",
          fillColor: "#0284c7",
          fillOpacity: 1,
        }).addTo(layer);
      }
      if (drawnPolygon.length >= 3) {
        L.polygon(latlngs, {
          color: "#38bdf8",
          fillColor: "#38bdf8",
          fillOpacity: 0.15,
          weight: 2,
          dashArray: "5,5",
        }).addTo(layer);
      } else if (drawnPolygon.length === 2) {
        L.polyline(latlngs, {
          color: "#38bdf8",
          weight: 2,
          dashArray: "5,5",
        }).addTo(layer);
      }
    }
  }, [drawnPolygon]);

  // Render Survivors Layer
  useEffect(() => {
    const layer = survivorsLayerRef.current;
    const map = mapRef.current;
    if (!layer || !map) return;
    layer.clearLayers();

    if (!showSurvivors) return;

    for (const s of survivors) {
      const isSelected = s.id === selectedId;
      const tier = s.priority_score >= 0.7 ? "critical" : s.priority_score >= 0.4 ? "high" : "medium";
      const color = PRIORITY_COLORS[tier] ?? "#ef4444";

      // Uncertainty circle
      if (s.uncertainty_radius && s.uncertainty_radius > 0) {
        L.circle([s.lat, s.lon], {
          radius: s.uncertainty_radius,
          color,
          fillColor: color,
          fillOpacity: 0.15,
          weight: 1,
          dashArray: "3,3",
        }).addTo(layer);
      }

      // Marker pin
      const marker = L.circleMarker([s.lat, s.lon], {
        radius: isSelected ? 11 : 8,
        color: isSelected ? "#ffffff" : color,
        fillColor: color,
        fillOpacity: 0.9,
        weight: isSelected ? 3 : 2,
      }).addTo(layer);

      marker.on("click", () => onSelectEntityRef.current(s));
      marker.bindTooltip(
        `Survivor (${tier.toUpperCase()}) · Score: ${s.priority_score.toFixed(2)}`,
        { direction: "top", offset: [0, -6] }
      );
    }

    if (!hasFitRef.current && survivors.length > 0) {
      hasFitRef.current = true;
      const bounds = L.latLngBounds(
        survivors.map((s) => [s.lat, s.lon] as L.LatLngTuple)
      );
      map.fitBounds(bounds, { padding: [40, 40], maxZoom: 16 });
    }
  }, [survivors, selectedId, showSurvivors]);

  // Render Hazards GeoJSON Layer
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    if (hazardsLayerRef.current) {
      map.removeLayer(hazardsLayerRef.current);
      hazardsLayerRef.current = null;
    }

    if (!showHazards) return;

    if (hazardsGeoJSON && hazardsGeoJSON.features && hazardsGeoJSON.features.length > 0) {
      hazardsLayerRef.current = L.geoJSON(hazardsGeoJSON as any, {
        style: (feature) => {
          const hazardClass = feature?.properties?.class ?? "fire";
          const color = CLASS_COLORS[hazardClass as DetectionClass] ?? "#f97316";
          return {
            color,
            fillColor: color,
            fillOpacity: 0.25,
            weight: 2,
          };
        },
        onEachFeature: (feature, layer) => {
          const props = feature.properties || {};
          layer.bindTooltip(
            `Hazard Zone: ${props.class ?? "Hazard"} (Buffer: ${props.buffer_meters ?? 50}m)`,
            { direction: "center" }
          );
        },
      }).addTo(map);
    } else {
      // Fallback: render markers for hazards list
      const fallbackGroup = L.layerGroup();
      for (const h of hazards) {
        const color = CLASS_COLORS[h.class as DetectionClass] ?? "#f97316";
        L.circleMarker([h.lat, h.lon], {
          radius: 7,
          color,
          fillColor: color,
          fillOpacity: 0.8,
          weight: 2,
        })
          .addTo(fallbackGroup)
          .bindTooltip(`Hazard: ${h.class}`, { direction: "top" })
          .on("click", () => onSelectEntityRef.current(h));
      }
      hazardsLayerRef.current = fallbackGroup as any;
      fallbackGroup.addTo(map);
    }
  }, [hazards, hazardsGeoJSON, showHazards]);

  // Render Safe Routes Layer
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    if (safeRoutesLayerRef.current) {
      map.removeLayer(safeRoutesLayerRef.current);
      safeRoutesLayerRef.current = null;
    }

    if (!showSafeRoutes) return;

    if (safeRoutesGeoJSON && safeRoutesGeoJSON.features && safeRoutesGeoJSON.features.length > 0) {
      safeRoutesLayerRef.current = L.geoJSON(safeRoutesGeoJSON as any, {
        style: {
          color: "#10b981",
          weight: 4,
          opacity: 0.85,
          dashArray: "6,6",
        },
        onEachFeature: (feature, layer) => {
          const dist = feature.properties?.distance_meters;
          layer.bindTooltip(
            `Safe Rescue Route${dist ? `: ${Math.round(dist)}m` : ""}`,
            { direction: "center" }
          );
        },
      }).addTo(map);
    } else if (routeSegments && routeSegments.length > 0) {
      const group = L.layerGroup();
      for (const seg of routeSegments) {
        L.polyline(
          [
            [seg.start.lat, seg.start.lon],
            [seg.end.lat, seg.end.lon],
          ],
          {
            color: seg.status === "obstructed" ? "#ef4444" : "#10b981",
            weight: seg.status === "obstructed" ? 4 : 3,
            opacity: 0.85,
            dashArray: seg.status === "obstructed" ? undefined : "6,6",
          }
        ).addTo(group);
      }
      safeRoutesLayerRef.current = group as any;
      group.addTo(map);
    }
  }, [safeRoutesGeoJSON, routeSegments, showSafeRoutes]);

  // Render Coverage Heatmap Layer
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    if (coverageLayerRef.current) {
      map.removeLayer(coverageLayerRef.current);
      coverageLayerRef.current = null;
    }

    if (!showCoverage || !coverageGeoJSON || !coverageGeoJSON.features) return;

    coverageLayerRef.current = L.geoJSON(coverageGeoJSON as any, {
      style: (feature) => {
        const passes = feature?.properties?.passes ?? 1;
        const opacity = Math.min(0.6, 0.15 + passes * 0.15);
        return {
          color: "#06b6d4",
          fillColor: "#06b6d4",
          fillOpacity: opacity,
          weight: 1,
        };
      },
    }).addTo(map);
  }, [coverageGeoJSON, showCoverage]);

  // Render Drone Track & Current Position
  useEffect(() => {
    const layer = droneTrackLayerRef.current;
    if (!layer) return;
    layer.clearLayers();

    if (!showDroneTrack || !telemetryHistory || telemetryHistory.length === 0) return;

    const latlngs = telemetryHistory.map((t) => [t.lat, t.lon] as L.LatLngTuple);

    // Track breadcrumb line
    L.polyline(latlngs, {
      color: "#eab308",
      weight: 2,
      opacity: 0.7,
      dashArray: "3,5",
    }).addTo(layer);

    // Current position icon
    const current = telemetryHistory[telemetryHistory.length - 1];
    const droneMarker = L.circleMarker([current.lat, current.lon], {
      radius: 9,
      color: "#ffffff",
      fillColor: "#eab308",
      fillOpacity: 1,
      weight: 2,
    }).addTo(layer);

    droneMarker.bindTooltip(
      `Drone Position · Alt: ${current.alt.toFixed(1)}m · Heading: ${current.heading.toFixed(0)}°`,
      { direction: "top", offset: [0, -6] }
    );
  }, [telemetryHistory, showDroneTrack]);

  // Render Planned Coverage Path
  useEffect(() => {
    const layer = plannedPathLayerRef.current;
    if (!layer) return;
    layer.clearLayers();

    if (!showPlannedPath || !plannedWaypoints || plannedWaypoints.length === 0) return;

    const latlngs = plannedWaypoints.map((w) => [w.lat, w.lon] as L.LatLngTuple);

    L.polyline(latlngs, {
      color: "#38bdf8",
      weight: 3,
      opacity: 0.9,
      dashArray: "4,8",
    }).addTo(layer);

    for (const wp of plannedWaypoints) {
      L.circleMarker([wp.lat, wp.lon], {
        radius: 4,
        color: "#38bdf8",
        fillColor: "#0284c7",
        fillOpacity: 0.8,
        weight: 1,
      })
        .addTo(layer)
        .bindTooltip(`WP #${wp.index} (${wp.alt_m}m)`, { direction: "top" });
    }
  }, [plannedWaypoints, showPlannedPath]);

  return (
    <div className="map-view-wrapper">
      {/* Map Layer Toggles Header */}
      <div className="map-layer-controls" role="toolbar" aria-label="Map Layer Controls">
        <label className="layer-checkbox">
          <input
            type="checkbox"
            checked={showSurvivors}
            onChange={(e) => setShowSurvivors(e.target.checked)}
          />
          Survivors ({survivors.length})
        </label>
        <label className="layer-checkbox">
          <input
            type="checkbox"
            checked={showHazards}
            onChange={(e) => setShowHazards(e.target.checked)}
          />
          Hazards ({hazards.length})
        </label>
        <label className="layer-checkbox">
          <input
            type="checkbox"
            checked={showSafeRoutes}
            onChange={(e) => setShowSafeRoutes(e.target.checked)}
          />
          Safe Routes
        </label>
        <label className="layer-checkbox">
          <input
            type="checkbox"
            checked={showCoverage}
            onChange={(e) => setShowCoverage(e.target.checked)}
          />
          Coverage Heatmap
        </label>
        <label className="layer-checkbox">
          <input
            type="checkbox"
            checked={showDroneTrack}
            onChange={(e) => setShowDroneTrack(e.target.checked)}
          />
          Drone Track
        </label>
        <label className="layer-checkbox">
          <input
            type="checkbox"
            checked={showPlannedPath}
            onChange={(e) => setShowPlannedPath(e.target.checked)}
          />
          Planned Path
        </label>

        <div className="map-controls-divider" />

        <button
          className={`draw-mode-btn ${isDrawing ? "draw-mode-btn--active" : ""}`}
          onClick={() => setIsDrawing(!isDrawing)}
          title="Click to toggle area polygon drawing on the map"
        >
          {isDrawing ? "Finish Drawing" : "Draw Area Polygon"}
        </button>

        <span
          className={`tile-status-indicator ${
            tileServerOnline ? "tile-status--online" : "tile-status--offline"
          }`}
          title={
            tileServerOnline
              ? "Serving local offline MBTiles cache"
              : "Tile server unreachable"
          }
        >
          {tileServerOnline ? "Offline Tiles: OK" : "Offline Tiles: Disconnected"}
        </span>
      </div>

      <div ref={containerRef} className="map-view" />
    </div>
  );
}
