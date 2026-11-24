package contracts

import (
	"encoding/json"
	"os/exec"
	"testing"
)

type PayloadWrapper struct {
	Detection Detection `json:"detection"`
	Telemetry Telemetry `json:"telemetry"`
}

func TestPythonGoContract(t *testing.T) {
	cmd := exec.Command("python3", "-c", `
import json, uuid
from datetime import datetime, timezone

payload = {
    "detection": {
        "id": str(uuid.uuid4()),
        "class": "person",
        "confidence": 0.92,
        "bbox": {"x": 0.1, "y": 0.2, "w": 0.3, "h": 0.4},
        "frame_id": "frame_00000100",
        "source": "thermal",
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "track_id": "track_42",
        "modalities": ["rgb", "thermal"],
        "model_version": "v2.0",
        "timings_ms": {
            "inference": 14.5,
            "total": 21.0
        }
    },
    "telemetry": {
        "lat": 19.0760,
        "lon": 72.8777,
        "alt": 55.2,
        "heading": 135.0,
        "timestamp": datetime.now(timezone.utc).isoformat()
    }
}
print(json.dumps(payload))
`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("python producer failed: %v", err)
	}

	var parsed PayloadWrapper
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON from python into Go types: %v", err)
	}

	// Validate Detection
	if err := parsed.Detection.Validate(); err != nil {
		t.Fatalf("detection validation failed: %v", err)
	}
	if parsed.Detection.Class != DetectionClassPerson {
		t.Errorf("expected class 'person', got %q", parsed.Detection.Class)
	}
	if parsed.Detection.Source != SourceThermal {
		t.Errorf("expected source 'thermal', got %q", parsed.Detection.Source)
	}
	if parsed.Detection.TrackID != "track_42" {
		t.Errorf("expected track_id 'track_42', got %q", parsed.Detection.TrackID)
	}
	if len(parsed.Detection.Modalities) != 2 {
		t.Errorf("expected 2 modalities, got %v", parsed.Detection.Modalities)
	}
	if parsed.Detection.TimingsMS == nil || parsed.Detection.TimingsMS.Inference != 14.5 {
		t.Errorf("expected timings_ms.inference 14.5, got %v", parsed.Detection.TimingsMS)
	}

	// Validate Telemetry
	if err := parsed.Telemetry.Validate(); err != nil {
		t.Fatalf("telemetry validation failed: %v", err)
	}
	if parsed.Telemetry.Lat != 19.0760 {
		t.Errorf("expected lat 19.0760, got %f", parsed.Telemetry.Lat)
	}
}

func TestInvalidPayloadsRejected(t *testing.T) {
	invalidDet := Detection{
		ID:         "",
		Class:      "alien",
		Confidence: 1.5,
		BBox:       BBox{X: -1, Y: 0, W: 1, H: 1},
	}
	if err := invalidDet.Validate(); err == nil {
		t.Errorf("expected invalid detection to fail validation, but it passed")
	}

	invalidTel := Telemetry{
		Lat: 190.0,
		Lon: -200.0,
	}
	if err := invalidTel.Validate(); err == nil {
		t.Errorf("expected invalid telemetry to fail validation, but it passed")
	}
}
