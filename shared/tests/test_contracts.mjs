import { execSync } from 'child_process';
import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const detectionSchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/detection.schema.json'), 'utf8'));
const telemetrySchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/telemetry.schema.json'), 'utf8'));

// Run Python producer with v2 multi-sensor/tracking fields
const pyScript = `
import json, uuid
from datetime import datetime, timezone

payload = {
    "detection": {
        "id": str(uuid.uuid4()),
        "class": "person",
        "confidence": 0.88,
        "bbox": {"x": 0.15, "y": 0.25, "w": 0.35, "h": 0.45},
        "frame_id": "frame_00000200",
        "source": "rgb",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "track_id": "track_101",
        "modalities": ["rgb", "thermal"],
        "model_version": "yolov8n-v2.0",
        "timings_ms": {
            "decode": 2.5,
            "preprocess": 1.2,
            "inference": 15.4,
            "postprocess": 3.1,
            "publish": 0.8,
            "total": 23.0
        }
    },
    "telemetry": {
        "lat": 19.0760,
        "lon": 72.8777,
        "alt": 60.0,
        "heading": 90.0,
        "timestamp": datetime.now(timezone.utc).isoformat()
    }
}
print(json.dumps(payload))
`;

const output = execSync(`python3 -c '${pyScript}'`).toString();
const { detection, telemetry } = JSON.parse(output);

function validateDetection(d) {
  if (typeof d.id !== 'string' || !d.id) throw new Error('invalid id');
  if (!detectionSchema.properties.class.enum.includes(d.class)) throw new Error('invalid class: ' + d.class);
  if (typeof d.confidence !== 'number' || d.confidence < 0 || d.confidence > 1) throw new Error('invalid confidence');
  if (!d.bbox || typeof d.bbox !== 'object') throw new Error('missing bbox');
  for (const k of ['x', 'y', 'w', 'h']) {
    if (typeof d.bbox[k] !== 'number' || d.bbox[k] < 0 || d.bbox[k] > 1) throw new Error(`invalid bbox ${k}`);
  }
  if (typeof d.frame_id !== 'string' || !d.frame_id) throw new Error('invalid frame_id');
  if (!detectionSchema.properties.source.enum.includes(d.source)) throw new Error('invalid source: ' + d.source);
  if (isNaN(Date.parse(d.timestamp))) throw new Error('invalid timestamp');

  // Validate optional v2 fields
  if (d.track_id !== undefined && typeof d.track_id !== 'string') throw new Error('invalid track_id');
  if (d.modalities !== undefined) {
    if (!Array.isArray(d.modalities)) throw new Error('invalid modalities');
    for (const m of d.modalities) {
      if (!['rgb', 'thermal'].includes(m)) throw new Error('invalid modality: ' + m);
    }
  }
  if (d.model_version !== undefined && typeof d.model_version !== 'string') throw new Error('invalid model_version');
  if (d.timings_ms !== undefined) {
    if (typeof d.timings_ms !== 'object') throw new Error('invalid timings_ms');
    for (const [k, v] of Object.entries(d.timings_ms)) {
      if (typeof v !== 'number') throw new Error(`invalid timing for ${k}`);
    }
  }
}

function validateTelemetry(t) {
  if (typeof t.lat !== 'number' || t.lat < -90 || t.lat > 90) throw new Error('invalid lat');
  if (typeof t.lon !== 'number' || t.lon < -180 || t.lon > 180) throw new Error('invalid lon');
  if (typeof t.alt !== 'number') throw new Error('invalid alt');
  if (typeof t.heading !== 'number' || t.heading < 0 || t.heading > 360) throw new Error('invalid heading');
  if (isNaN(Date.parse(t.timestamp))) throw new Error('invalid timestamp');
}

validateDetection(detection);
validateTelemetry(telemetry);

// Validate Hazard Zones GeoJSON
const hazardZoneSchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/hazard_zone.schema.json'), 'utf8'));
const sampleHazardZones = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      id: "hazard_zone_fire",
      geometry: {
        type: "Polygon",
        coordinates: [[[78.001, 20.001], [78.003, 20.001], [78.003, 20.003], [78.001, 20.003], [78.001, 20.001]]]
      },
      properties: {
        id: "hazard_zone_fire",
        class: "fire",
        severity: "critical",
        buffer_meters: 50.0,
        area_sq_m: 12500.0,
        detection_count: 5,
        feature_type: "polygon"
      }
    }
  ]
};

function validateHazardZones(hz) {
  if (hz.type !== 'FeatureCollection') throw new Error('invalid FeatureCollection type');
  if (!Array.isArray(hz.features)) throw new Error('missing features array');
  for (const f of hz.features) {
    if (f.type !== 'Feature') throw new Error('invalid feature type');
    if (!f.geometry || !['Polygon', 'MultiPolygon', 'Point'].includes(f.geometry.type)) throw new Error('invalid geometry');
    const p = f.properties;
    if (!p.id || !hazardZoneSchema.properties.features.items.properties.properties.properties.class.enum.includes(p.class)) {
      throw new Error('invalid class: ' + p.class);
    }
    if (!['critical', 'high', 'medium', 'low'].includes(p.severity)) throw new Error('invalid severity');
    if (typeof p.buffer_meters !== 'number' || p.buffer_meters < 0) throw new Error('invalid buffer_meters');
  }
}
validateHazardZones(sampleHazardZones);

// Validate Safe Route GeoJSON
const safeRouteSchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/safe_route.schema.json'), 'utf8'));
const sampleSafeRoute = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      geometry: {
        type: "LineString",
        coordinates: [[78.000, 20.000], [78.0005, 20.0015], [78.002, 20.004]]
      },
      properties: {
        route_id: "route_survivor_1",
        safe: true,
        reason: null,
        length_meters: 450.5,
        risk_score: 0.0,
        avoided_hazards: ["hazard_zone_fire"],
        intersected_hazards: [],
        start: [78.000, 20.000],
        destination: [78.002, 20.004]
      }
    }
  ]
};

function validateSafeRoute(sr) {
  if (sr.type !== 'FeatureCollection') throw new Error('invalid FeatureCollection type');
  for (const f of sr.features) {
    if (f.type !== 'Feature') throw new Error('invalid feature type');
    if (!f.geometry || f.geometry.type !== 'LineString') throw new Error('invalid route geometry');
    const p = f.properties;
    if (typeof p.safe !== 'boolean') throw new Error('missing safe flag');
    if (typeof p.length_meters !== 'number' || p.length_meters < 0) throw new Error('invalid length_meters');
    if (!Array.isArray(p.avoided_hazards) || !Array.isArray(p.intersected_hazards)) throw new Error('invalid hazard lists');
  }
}
validateSafeRoute(sampleSafeRoute);

// Validate Coverage GeoJSON
const coverageSchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/coverage.schema.json'), 'utf8'));
const sampleCoverage = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      geometry: {
        type: "Polygon",
        coordinates: [[[78.000, 20.000], [78.001, 20.000], [78.001, 20.001], [78.000, 20.001], [78.000, 20.000]]]
      },
      properties: {
        cell_id: "cell_0_0",
        observation_count: 3,
        last_observed_at: new Date().toISOString(),
        best_confidence: 0.92,
        searched: true,
        staleness_seconds: 15.2
      }
    }
  ]
};

function validateCoverage(c) {
  if (c.type !== 'FeatureCollection') throw new Error('invalid FeatureCollection type');
  for (const f of c.features) {
    if (f.type !== 'Feature') throw new Error('invalid feature type');
    if (!f.geometry || f.geometry.type !== 'Polygon') throw new Error('invalid coverage geometry');
    const p = f.properties;
    if (typeof p.cell_id !== 'string') throw new Error('invalid cell_id');
    if (typeof p.observation_count !== 'number' || p.observation_count < 0) throw new Error('invalid count');
    if (typeof p.best_confidence !== 'number' || p.best_confidence < 0 || p.best_confidence > 1) throw new Error('invalid confidence');
    if (typeof p.searched !== 'boolean') throw new Error('invalid searched flag');
  }
}
validateCoverage(sampleCoverage);

console.log('Contract validation passed: Python -> TS validated successfully.');
