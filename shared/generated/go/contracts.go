// Package contracts contains types generated from shared/jsonschema -- DO NOT EDIT MANUALLY.
package contracts

import (
	"fmt"
	time "time"
)

type DetectionClass string

const (
	DetectionClassPerson           DetectionClass = "person"
	DetectionClassFire             DetectionClass = "fire"
	DetectionClassFlood            DetectionClass = "flood"
	DetectionClassDebris           DetectionClass = "debris"
	DetectionClassStructuralDamage DetectionClass = "structural_damage"
)

type Source string

const (
	SourceRGB     Source = "rgb"
	SourceThermal Source = "thermal"
)

type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type StageTimings struct {
	Decode      float64 `json:"decode,omitempty"`
	Preprocess  float64 `json:"preprocess,omitempty"`
	Inference   float64 `json:"inference,omitempty"`
	Postprocess float64 `json:"postprocess,omitempty"`
	Publish     float64 `json:"publish,omitempty"`
	Total       float64 `json:"total,omitempty"`
}

type Detection struct {
	ID           string        `json:"id"`
	Class        DetectionClass `json:"class"`
	Confidence   float64       `json:"confidence"`
	BBox         BBox          `json:"bbox"`
	FrameID      string        `json:"frame_id"`
	Source       Source        `json:"source"`
	Timestamp    string        `json:"timestamp"`
	TrackID      string        `json:"track_id,omitempty"`
	Modalities   []Source      `json:"modalities,omitempty"`
	ModelVersion string        `json:"model_version,omitempty"`
	TimingsMS    *StageTimings `json:"timings_ms,omitempty"`
}

func (d *Detection) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("id is required")
	}
	switch d.Class {
	case DetectionClassPerson, DetectionClassFire, DetectionClassFlood, DetectionClassDebris, DetectionClassStructuralDamage:
	default:
		return fmt.Errorf("invalid class %q", d.Class)
	}
	if d.Confidence < 0 || d.Confidence > 1 {
		return fmt.Errorf("confidence out of range [0, 1]: %f", d.Confidence)
	}
	if d.BBox.X < 0 || d.BBox.X > 1 || d.BBox.Y < 0 || d.BBox.Y > 1 || d.BBox.W < 0 || d.BBox.W > 1 || d.BBox.H < 0 || d.BBox.H > 1 {
		return fmt.Errorf("bbox coords out of range [0, 1]")
	}
	if d.FrameID == "" {
		return fmt.Errorf("frame_id is required")
	}
	switch d.Source {
	case SourceRGB, SourceThermal:
	default:
		return fmt.Errorf("invalid source %q", d.Source)
	}
	if _, err := time.Parse(time.RFC3339, d.Timestamp); err != nil {
		if _, err := time.Parse(time.RFC3339Nano, d.Timestamp); err != nil {
			return fmt.Errorf("timestamp is not valid RFC3339: %w", err)
		}
	}
	for _, m := range d.Modalities {
		switch m {
		case SourceRGB, SourceThermal:
		default:
			return fmt.Errorf("invalid modality %q", m)
		}
	}
	return nil
}

type Telemetry struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Alt       float64 `json:"alt"`
	Heading   float64 `json:"heading"`
	Pitch     float64 `json:"pitch,omitempty"`
	Roll      float64 `json:"roll,omitempty"`
	Yaw       float64 `json:"yaw,omitempty"`
	Timestamp string  `json:"timestamp"`
}

func (t *Telemetry) Validate() error {
	if t.Lat < -90 || t.Lat > 90 {
		return fmt.Errorf("lat out of range [-90, 90]: %f", t.Lat)
	}
	if t.Lon < -180 || t.Lon > 180 {
		return fmt.Errorf("lon out of range [-180, 180]: %f", t.Lon)
	}
	if t.Heading < 0 || t.Heading > 360 {
		return fmt.Errorf("heading out of range [0, 360]: %f", t.Heading)
	}
	if _, err := time.Parse(time.RFC3339, t.Timestamp); err != nil {
		if _, err := time.Parse(time.RFC3339Nano, t.Timestamp); err != nil {
			return fmt.Errorf("timestamp is not valid RFC3339: %w", err)
		}
	}
	return nil
}

type HazardZoneProperties struct {
	ID             string  `json:"id"`
	Class          string  `json:"class"`
	Severity       string  `json:"severity"`
	BufferMeters   float64 `json:"buffer_meters"`
	AreaSqM        float64 `json:"area_sq_m"`
	DetectionCount int     `json:"detection_count"`
	FeatureType    string  `json:"feature_type,omitempty"`
}

type HazardZoneFeature struct {
	Type       string                 `json:"type"`
	ID         string                 `json:"id"`
	Geometry   map[string]interface{} `json:"geometry"`
	Properties HazardZoneProperties   `json:"properties"`
}

type HazardZonesGeoJSON struct {
	Type     string              `json:"type"`
	Features []HazardZoneFeature `json:"features"`
}

func (h *HazardZonesGeoJSON) Validate() error {
	if h.Type != "FeatureCollection" {
		return fmt.Errorf("invalid type %q", h.Type)
	}
	for _, f := range h.Features {
		if f.Type != "Feature" {
			return fmt.Errorf("invalid feature type %q", f.Type)
		}
		if f.Properties.ID == "" {
			return fmt.Errorf("hazard zone ID is required")
		}
		if f.Properties.BufferMeters < 0 {
			return fmt.Errorf("buffer meters must be >= 0")
		}
	}
	return nil
}

type SafeRouteProperties struct {
	RouteID            string     `json:"route_id"`
	Safe               bool       `json:"safe"`
	Reason             *string    `json:"reason,omitempty"`
	LengthMeters       float64    `json:"length_meters"`
	RiskScore          float64    `json:"risk_score"`
	AvoidedHazards     []string   `json:"avoided_hazards"`
	IntersectedHazards []string   `json:"intersected_hazards"`
	Start              [2]float64 `json:"start"`
	Destination        [2]float64 `json:"destination"`
}

type SafeRouteFeature struct {
	Type       string                 `json:"type"`
	Geometry   map[string]interface{} `json:"geometry"`
	Properties SafeRouteProperties    `json:"properties"`
}

type SafeRouteGeoJSON struct {
	Type     string             `json:"type"`
	Features []SafeRouteFeature `json:"features"`
}

func (r *SafeRouteGeoJSON) Validate() error {
	if r.Type != "FeatureCollection" {
		return fmt.Errorf("invalid type %q", r.Type)
	}
	for _, f := range r.Features {
		if f.Type != "Feature" {
			return fmt.Errorf("invalid feature type %q", f.Type)
		}
		if f.Properties.LengthMeters < 0 {
			return fmt.Errorf("length meters must be >= 0")
		}
	}
	return nil
}

type CoverageCellProperties struct {
	CellID           string   `json:"cell_id"`
	ObservationCount int      `json:"observation_count"`
	LastObservedAt   *string  `json:"last_observed_at,omitempty"`
	BestConfidence   float64  `json:"best_confidence"`
	Searched         bool     `json:"searched"`
	StalenessSeconds *float64 `json:"staleness_seconds,omitempty"`
}

type CoverageCellFeature struct {
	Type       string                 `json:"type"`
	Geometry   map[string]interface{} `json:"geometry"`
	Properties CoverageCellProperties `json:"properties"`
}

type CoverageGeoJSON struct {
	Type     string                `json:"type"`
	Features []CoverageCellFeature `json:"features"`
}

func (c *CoverageGeoJSON) Validate() error {
	if c.Type != "FeatureCollection" {
		return fmt.Errorf("invalid type %q", c.Type)
	}
	for _, f := range c.Features {
		if f.Type != "Feature" {
			return fmt.Errorf("invalid feature type %q", f.Type)
		}
		if f.Properties.CellID == "" {
			return fmt.Errorf("cell ID is required")
		}
		if f.Properties.BestConfidence < 0 || f.Properties.BestConfidence > 1 {
			return fmt.Errorf("best confidence out of range [0, 1]")
		}
	}
	return nil
}
