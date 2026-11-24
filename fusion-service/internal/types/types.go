package types

// BBox is a normalized [0,1] axis-aligned bounding box.
type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// StageTimings records per-stage processing latency in milliseconds.
type StageTimings struct {
	Decode      float64 `json:"decode,omitempty"`
	Preprocess  float64 `json:"preprocess,omitempty"`
	Inference   float64 `json:"inference,omitempty"`
	Postprocess float64 `json:"postprocess,omitempty"`
	Publish     float64 `json:"publish,omitempty"`
	Total       float64 `json:"total,omitempty"`
}

// Detection mirrors shared/jsonschema/detection.schema.json.
type Detection struct {
	ID           string        `json:"id"`
	Class        string        `json:"class"` // person | fire | flood | debris | structural_damage
	Confidence   float64       `json:"confidence"`
	BBox         BBox          `json:"bbox"`
	FrameID      string        `json:"frame_id"`
	Source       string        `json:"source"` // rgb | thermal
	Timestamp    string        `json:"timestamp"`
	TrackID      string        `json:"track_id,omitempty"`
	Modalities   []string      `json:"modalities,omitempty"`
	ModelVersion string        `json:"model_version,omitempty"`
	TimingsMS    *StageTimings `json:"timings_ms,omitempty"`
}

// Telemetry mirrors shared/jsonschema/telemetry.schema.json.
type Telemetry struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Alt       float64 `json:"alt"`
	Heading   float64 `json:"heading"`
	Timestamp string  `json:"timestamp"`
}
