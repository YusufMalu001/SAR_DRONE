import L from "leaflet";
import "leaflet/dist/leaflet.css";
import { useEffect, useRef } from "react";
import type { DetectionClass, DetectionRecord, RouteSegment } from "../lib/types";

// Per the spec: person = red, fire = orange, flood = blue,
// structural_damage = amber. "debris" isn't specified — a neutral gray
// keeps it visually distinct without implying a priority it wasn't given.
export const CLASS_COLORS: Record<DetectionClass, string> = {
  person: "#ef4444",
  fire: "#f97316",
  flood: "#3b82f6",
  structural_damage: "#f59e0b",
  debris: "#9ca3af",
};

const DEFAULT_CENTER: L.LatLngTuple = [20, 78];

interface MapViewProps {
  detections: DetectionRecord[];
  routeSegments?: RouteSegment[];
  selectedId?: string | null;
  onSelect: (detection: DetectionRecord) => void;
}

export default function MapView({
  detections,
  routeSegments,
  selectedId,
  onSelect,
}: MapViewProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<L.Map | null>(null);
  const markersRef = useRef<Map<string, L.CircleMarker>>(new Map());
  const routeLayerRef = useRef<L.LayerGroup | null>(null);
  const hasFitRef = useRef(false);
  // Keep a stable reference to the latest onSelect for marker click
  // handlers, without re-creating markers every render.
  const onSelectRef = useRef(onSelect);
  onSelectRef.current = onSelect;

  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;

    const map = L.map(containerRef.current, {
      center: DEFAULT_CENTER,
      zoom: 14,
      preferCanvas: true,
    });
    const fusionUrl = import.meta.env.VITE_FUSION_URL || "http://localhost:8002";
    const tileUrl = `${fusionUrl}/tiles/{z}/{x}/{y}.png`;
    L.tileLayer(tileUrl, {
      attribution: "&copy; Khoj-o-Drone Offline Map / OpenStreetMap contributors",
      maxZoom: 19,
    }).addTo(map);

    routeLayerRef.current = L.layerGroup().addTo(map);
    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
      markersRef.current.clear();
      routeLayerRef.current = null;
    };
  }, []);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const seen = new Set<string>();

    for (const d of detections) {
      seen.add(d.id);
      const color = CLASS_COLORS[d.class] ?? "#9ca3af";
      const isSelected = d.id === selectedId;
      const style: L.CircleMarkerOptions = {
        radius: isSelected ? 10 : 7,
        color: isSelected ? "#ffffff" : color,
        fillColor: color,
        fillOpacity: 0.85,
        weight: isSelected ? 3 : 1.5,
      };

      let marker = markersRef.current.get(d.id);
      if (!marker) {
        marker = L.circleMarker([d.lat, d.lon], style).addTo(map);
        marker.on("click", () => onSelectRef.current(d));
        markersRef.current.set(d.id, marker);
      } else {
        marker.setLatLng([d.lat, d.lon]);
        marker.setStyle(style);
      }
      marker.bindTooltip(
        `${d.class} · ${Math.round(d.confidence * 100)}%`,
        { direction: "top", offset: [0, -6] },
      );
    }

    for (const [id, marker] of markersRef.current) {
      if (!seen.has(id)) {
        marker.remove();
        markersRef.current.delete(id);
      }
    }

    if (!hasFitRef.current && detections.length > 0) {
      hasFitRef.current = true;
      const bounds = L.latLngBounds(
        detections.map((d) => [d.lat, d.lon] as L.LatLngTuple),
      );
      map.fitBounds(bounds, { padding: [40, 40], maxZoom: 16 });
    }
  }, [detections, selectedId]);

  useEffect(() => {
    const layer = routeLayerRef.current;
    if (!layer) return;
    layer.clearLayers();

    for (const seg of routeSegments ?? []) {
      L.polyline(
        [
          [seg.start.lat, seg.start.lon],
          [seg.end.lat, seg.end.lon],
        ],
        {
          color: seg.status === "obstructed" ? "#ef4444" : "#4edea3",
          weight: seg.status === "obstructed" ? 4 : 2,
          opacity: seg.status === "obstructed" ? 0.9 : 0.5,
          dashArray: seg.status === "obstructed" ? undefined : "4,6",
        },
      ).addTo(layer);
    }
  }, [routeSegments]);

  return <div ref={containerRef} className="map-view" />;
}
