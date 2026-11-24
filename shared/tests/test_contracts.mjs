import { execSync } from 'child_process';
import { readFileSync } from 'fs';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const detectionSchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/detection.schema.json'), 'utf8'));
const telemetrySchema = JSON.parse(readFileSync(join(__dirname, '../jsonschema/telemetry.schema.json'), 'utf8'));

// Run Python producer
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
        "timestamp": datetime.now(timezone.utc).isoformat()
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
console.log('Contract validation passed: Python -> TS validated successfully.');
